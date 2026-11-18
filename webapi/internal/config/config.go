// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package config reads the service's settings from the environment.
//
// The database settings are the Drogon web's (web/src/Config.hpp), with
// the same names, defaults and fallbacks, so one .env serves both web
// services while they run side by side; so are the key, session, login
// and daemon settings, which must agree for sessions and tokens to work
// in both. Only the listening address is the service's own (WEBAPI_*),
// since both listen at the same time.
package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/settings"
)

// DB is one database pool's settings.
type DB struct {
	Name     string // pool name in logs and health output: "ro" or "app"
	Host     string
	Port     uint16
	DBName   string
	User     string
	Password string
	Conns    int // most open connections in the pool
}

// Config is the whole service configuration.
type Config struct {
	ListenAddr string
	ListenPort uint16
	LogLevel   slog.Level

	RO  DB // SELECT-only over the logger's schema
	App DB // read-write over the web's own schema (accounts, audit)

	// AppKey is WEB_APP_KEY, decoded: it signs sessions and CSRF
	// tokens, encrypts file tokens and keys the daemon API.
	AppKey []byte
	// SessionTimeout is WEB_SESSION_TIMEOUT in seconds; <= 0 = never.
	SessionTimeout int
	// SecureCookie is WEB_SECURE_COOKIE != "0".
	SecureCookie bool
	// LoginMaxAttempts failed logins per (ip, username) are allowed
	// within LoginWindow (WEB_LOGIN_MAX_ATTEMPTS, WEB_LOGIN_WINDOW).
	LoginMaxAttempts int
	LoginWindow      time.Duration
	// LoginMaxPerAccount failed logins per account, from all addresses
	// together, are allowed within LoginWindow
	// (WEB_LOGIN_MAX_ATTEMPTS_PER_ACCOUNT); 0 turns the cap off.
	LoginMaxPerAccount int

	// DaemonAPI is WEB_TGLOGGERD_API, "" when the daemon API is off;
	// FetchTimeout is how long a fetch through it may take.
	DaemonAPI    string
	FetchTimeout time.Duration

	// StorageDir is WEB_STORAGE_DIR, the file store's root; it must be
	// the daemon's TG_STORAGE_DIR.
	StorageDir string
	// DataDir is the daemon's TG_DATA_DIR, whose tdlib/ holds the
	// partial files of big fetches; "" when unset.
	DataDir string
	// StoreFileCap is the daemon's TG_MAX_STORE_FILE_SIZE: files under
	// it are fetched whole into the store. 0 = no cap.
	StoreFileCap uint64

	// TrustedProxies is WEBAPI_TRUSTED_PROXIES: the reverse proxies
	// (and the networks of the ones in front of them) whose
	// X-Forwarded-For hops are skipped to find a request's client.
	// Empty: the connection's peer is the client.
	TrustedProxies []netip.Prefix

	// Live is WEBAPI_LIVE != "0": GET /api/events, fed by the daemon's
	// event stream (so only with DaemonAPI set).
	Live bool
	// LiveMaxSubscribers caps the open event streams
	// (WEBAPI_LIVE_MAX_SUBSCRIBERS); LiveBuffer is how many events may
	// wait for one (WEBAPI_LIVE_BUFFER); LiveHistory how many recent
	// events are kept for streams that reconnect (WEBAPI_LIVE_HISTORY).
	LiveMaxSubscribers int
	LiveBuffer         int
	LiveHistory        int
	// LiveRecheck is how often an open stream checks its session again
	// (WEBAPI_LIVE_RECHECK, seconds).
	LiveRecheck time.Duration
	// RenderSocket is the render service's Unix socket (RENDER_SOCKET);
	// RenderTimeout bounds a render, connect and answer together
	// (RENDER_TIMEOUT), and RenderBackoff is how long the service is
	// skipped after it failed to answer (RENDER_BACKOFF).
	RenderSocket  string
	RenderTimeout time.Duration
	RenderBackoff time.Duration
	// AppDist is the client bundle vite build wrote (WEBAPI_APP_DIST),
	// with .vite/manifest.json and assets/; AppBuild is its build id
	// (WEBAPI_APP_BUILD), empty for the one the build recorded in the
	// bundle (render.ReadBuild).
	AppDist  string
	AppBuild string
	// Defaults are the site's settings for accounts that did not choose
	// their own (WEBAPI_DEFAULT_TIMEZONE, _DATE_FORMAT, _LANG, _THEME).
	Defaults settings.Defaults

	// PublicURL is the site's public base URL, for absolute
	// /files/<token> links in MCP answers (WEB_PUBLIC_URL, falling back
	// to the daemon's TG_DISCORD_PUBLIC_URL, which builds the same
	// links), trailing slashes and blanks dropped; "" when neither is
	// set.
	PublicURL string
	// MCPThreads is how many MCP tool calls run at once (MCP_THREADS,
	// default 2, 1 to 16).
	MCPThreads int
	// MCPAllowedOrigins is MCP_ALLOWED_ORIGINS: browser origins allowed
	// to call /mcp, comma-separated, or "*"; empty allows any.
	MCPAllowedOrigins string
	// MCPLogRequests is MCP_LOG_REQUESTS == "1": log the shape of the
	// requests to /mcp.
	MCPLogRequests bool
}

// env returns the variable's value, or def when it is unset or empty.
// An empty value counts as unset, as in the daemon and the Drogon web.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func port(key, def string) (uint16, error) {
	s := env(key, def)
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%s: %q is not a TCP port", key, s)
	}
	return uint16(n), nil
}

func integer(key, def string) (int, error) {
	s := env(key, def)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, s)
	}
	return n, nil
}

func positive(key, def string) (int, error) {
	s := env(key, def)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive number", key, s)
	}
	return n, nil
}

// millis reads a duration given in milliseconds.
func millis(key, def string) (time.Duration, error) {
	s := env(key, def)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive number of milliseconds", key, s)
	}
	return time.Duration(n) * time.Millisecond, nil
}

func level(key, def string) (slog.Level, error) {
	s := env(key, def)
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return 0, fmt.Errorf("%s: %q is not one of debug, info, warn, error", key, s)
	}
	return l, nil
}

// trustedProxies reads a comma- or space-separated list of addresses
// and networks: "10.0.88.1", "173.245.48.0/20", "2400:cb00::/32".
func trustedProxies(key string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.FieldsFunc(os.Getenv(key), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		if strings.Contains(f, "/") {
			p, err := netip.ParsePrefix(f)
			if err != nil {
				return nil, fmt.Errorf("%s: %q is not an address or network", key, f)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not an address or network", key, f)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// FromEnv builds the configuration from the environment. A malformed
// number is an error rather than a silent zero, so a typo in .env stops
// the service at start instead of misconfiguring it.
func FromEnv() (*Config, error) {
	var (
		c   Config
		err error
	)

	c.ListenAddr = env("WEBAPI_LISTEN_ADDR", "127.0.0.1")
	if c.ListenPort, err = port("WEBAPI_LISTEN_PORT", "8081"); err != nil {
		return nil, err
	}
	if c.LogLevel, err = level("WEBAPI_LOG_LEVEL", "info"); err != nil {
		return nil, err
	}

	// WEB_DB_HOST/PORT set the server for both pools and fall back to
	// the daemon's TG_DB_HOST/PORT, as in the Drogon web.
	host := env("WEB_DB_HOST", env("TG_DB_HOST", "127.0.0.1"))
	dbPort, err := port("WEB_DB_PORT", env("TG_DB_PORT", "3306"))
	if err != nil {
		return nil, err
	}

	c.RO = DB{
		Name:     "ro",
		Host:     host,
		Port:     dbPort,
		DBName:   env("WEB_DB_RO_NAME", "tgloggerd"),
		User:     env("WEB_DB_RO_USER", "web_ro"),
		Password: env("WEB_DB_RO_PASSWORD", ""),
	}
	if c.RO.Conns, err = positive("WEB_DB_RO_CONNS", "4"); err != nil {
		return nil, err
	}

	c.App = DB{
		Name:     "app",
		Host:     host,
		Port:     dbPort,
		DBName:   env("WEB_DB_APP_NAME", "tgloggerd_web"),
		User:     env("WEB_DB_APP_USER", "web_app"),
		Password: env("WEB_DB_APP_PASSWORD", ""),
	}
	if c.App.Conns, err = positive("WEB_DB_APP_CONNS", "2"); err != nil {
		return nil, err
	}

	// Without the key nothing can be signed in or served, so a missing
	// or unusable one stops the service, as it stops the Drogon web.
	if c.AppKey, err = auth.DecodeAppKey(os.Getenv("WEB_APP_KEY")); err != nil {
		return nil, err
	}
	if c.SessionTimeout, err = integer("WEB_SESSION_TIMEOUT", "43200"); err != nil {
		return nil, err
	}
	c.SecureCookie = env("WEB_SECURE_COOKIE", "1") != "0"
	if c.LoginMaxAttempts, err = integer("WEB_LOGIN_MAX_ATTEMPTS", "5"); err != nil {
		return nil, err
	}
	window, err := integer("WEB_LOGIN_WINDOW", "900")
	if err != nil {
		return nil, err
	}
	c.LoginWindow = time.Duration(window) * time.Second
	if c.LoginMaxPerAccount, err = integer("WEB_LOGIN_MAX_ATTEMPTS_PER_ACCOUNT", "20"); err != nil {
		return nil, err
	}

	// A malformed WEB_TGLOGGERD_API only turns the daemon client off
	// (cmd/webapi logs it), as in the Drogon web, which works without
	// the daemon.
	c.DaemonAPI = env("WEB_TGLOGGERD_API", "")
	c.FetchTimeout = daemon.FetchTimeout(os.Getenv("TG_FETCH_TIMEOUT_SECS"))

	// The store and TDLib's directory are shared with the daemon, and
	// so are their settings, read from the same .env.
	c.StorageDir = env("WEB_STORAGE_DIR", "data/storage/files")
	c.DataDir = env("TG_DATA_DIR", "")
	capStr := env("TG_MAX_STORE_FILE_SIZE", "1073741824")
	if c.StoreFileCap, err = strconv.ParseUint(capStr, 10, 64); err != nil {
		return nil, fmt.Errorf("TG_MAX_STORE_FILE_SIZE: %q is not a size in bytes", capStr)
	}

	if c.TrustedProxies, err = trustedProxies("WEBAPI_TRUSTED_PROXIES"); err != nil {
		return nil, err
	}

	c.Live = env("WEBAPI_LIVE", "1") != "0"
	if c.LiveMaxSubscribers, err = positive("WEBAPI_LIVE_MAX_SUBSCRIBERS", "256"); err != nil {
		return nil, err
	}
	if c.LiveBuffer, err = positive("WEBAPI_LIVE_BUFFER", "256"); err != nil {
		return nil, err
	}
	if c.LiveHistory, err = positive("WEBAPI_LIVE_HISTORY", "4096"); err != nil {
		return nil, err
	}
	recheck, err := positive("WEBAPI_LIVE_RECHECK", "30")
	if err != nil {
		return nil, err
	}
	c.LiveRecheck = time.Duration(recheck) * time.Second
	// The render service and the bundle it was built with
	// (docs/design/web-ssr.md sections 4.5 and 6).
	c.RenderSocket = env("RENDER_SOCKET", "/run/tgloggerd-render/render.sock")
	if c.RenderTimeout, err = millis("RENDER_TIMEOUT", "500"); err != nil {
		return nil, err
	}
	if c.RenderBackoff, err = millis("RENDER_BACKOFF", "2000"); err != nil {
		return nil, err
	}
	c.AppDist = env("WEBAPI_APP_DIST", "web/app/dist/client")
	c.AppBuild = env("WEBAPI_APP_BUILD", "")

	c.Defaults = settings.Defaults{
		Timezone:   env("WEBAPI_DEFAULT_TIMEZONE", settings.SiteDefaults.Timezone),
		DateFormat: env("WEBAPI_DEFAULT_DATE_FORMAT", settings.SiteDefaults.DateFormat),
		Lang:       env("WEBAPI_DEFAULT_LANG", settings.SiteDefaults.Lang),
		Theme:      env("WEBAPI_DEFAULT_THEME", settings.SiteDefaults.Theme),
	}
	if err := c.Defaults.Check(); err != nil {
		return nil, fmt.Errorf("WEBAPI_DEFAULT_*: %w", err)
	}

	// The MCP endpoint's settings are the Drogon web's.
	c.PublicURL = strings.TrimRight(env("WEB_PUBLIC_URL", env("TG_DISCORD_PUBLIC_URL", "")), "/ ")
	if c.MCPThreads, err = integer("MCP_THREADS", "2"); err != nil {
		return nil, err
	}
	c.MCPThreads = min(max(c.MCPThreads, 1), 16)
	c.MCPAllowedOrigins = env("MCP_ALLOWED_ORIGINS", "")
	c.MCPLogRequests = env("MCP_LOG_REQUESTS", "0") == "1"

	return &c, nil
}

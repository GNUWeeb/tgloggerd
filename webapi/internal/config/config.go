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

	return &c, nil
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Command webapi is the Go web service of tgloggerd. It serves the JSON
// API under /api/ beside the Drogon web (docs/design/web-go.md).
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/config"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/live"
	"tgloggerd/webapi/internal/mail"
	"tgloggerd/webapi/internal/media"
	"tgloggerd/webapi/internal/render"
	"tgloggerd/webapi/internal/server"
)

// shutdownTimeout bounds how long requests in flight may run after
// SIGTERM. It stays under the 10 seconds docker stop waits before it
// kills the container.
const shutdownTimeout = 8 * time.Second

// openPool opens one database pool. sql.Open connects lazily, so the
// service starts even while MySQL is still coming up; the health
// endpoint reports it until it answers.
func openPool(log *slog.Logger, c config.DB) (*sql.DB, error) {
	mc := mysql.NewConfig()
	// The driver's own messages (a connection dropped mid-query, ...)
	// go to the service log, tagged with the pool.
	mc.Logger = slog.NewLogLogger(log.With("pool", c.Name).Handler(), slog.LevelWarn)
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port)))
	mc.User = c.User
	mc.Passwd = c.Password
	mc.DBName = c.DBName
	// Values are put into the statement by the driver, as the Drogon
	// web's client does, instead of preparing it on the server. The
	// search's relevance sort needs that: MySQL orders by the fulltext
	// index only when the MATCH of ORDER BY is the one of WHERE, which
	// two placeholders of a prepared statement are not, so it would
	// sort every hit instead. It also saves the prepare and close round
	// trips of every query with values.
	mc.InterpolateParams = true

	connector, err := mysql.NewConnector(mc)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(c.Conns)
	db.SetMaxIdleConns(c.Conns)
	// Retire connections before the server's wait_timeout drops them,
	// so a request rarely meets a dead connection after a quiet spell.
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func run(log *slog.Logger, cfg *config.Config) error {
	ro, err := openPool(log, cfg.RO)
	if err != nil {
		return err
	}
	defer ro.Close()
	app, err := openPool(log, cfg.App)
	if err != nil {
		return err
	}
	defer app.Close()

	dc, err := daemon.New(cfg.DaemonAPI, cfg.AppKey, cfg.FetchTimeout)
	switch {
	case err != nil:
		log.Warn("the daemon API stays off", "err", err)
	case dc.Enabled():
		log.Info("daemon API", "url", dc.BaseURL())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	// Served files are reported to the daemon once a minute, for its
	// eviction order; the last minute's are dropped on shutdown, as a
	// failed report's are.
	access := daemon.NewAccessReporter(dc, log)
	go access.Run(ctx)

	tdlib := media.TDLibDir(cfg.DataDir)
	if tdlib == "" {
		log.Warn("TG_DATA_DIR is not set; big files cannot be streamed from TDLib")
	}

	// One subscription to the daemon's events feeds every browser's
	// GET /api/events. The hub stops with ctx, before the server's
	// shutdown, so open streams do not hold the shutdown up.
	var hub *live.Hub
	switch {
	case !cfg.Live:
		log.Info("live updates are off (WEBAPI_LIVE=0)")
	case !dc.Enabled():
		log.Warn("live updates are off: they need WEB_TGLOGGERD_API")
	default:
		hub = live.NewHub(live.DaemonSource{Client: dc, Log: log}, live.Options{
			History:        cfg.LiveHistory,
			Buffer:         cfg.LiveBuffer,
			MaxSubscribers: cfg.LiveMaxSubscribers,
			Log:            log,
		})
		go hub.Run(ctx)
	}
	// Pages link the client bundle vite build wrote; without it they
	// still answer, with their data, but no script to show it.
	assets, err := render.LoadAssets(cfg.AppDist, cfg.AppBuild)
	if err != nil {
		log.Warn("no client bundle; pages link no assets", "err", err)
	}
	var assetFiles *os.Root
	if root, err := os.OpenRoot(filepath.Join(cfg.AppDist, "assets")); err == nil {
		defer root.Close()
		assetFiles = root
	}
	renderer := render.NewClient(cfg.RenderSocket, cfg.RenderTimeout, cfg.RenderBackoff)
	log.Info("render service", "socket", renderer.Socket(), "build", assets.Build)

	// Account mail: links and notices go out from a queue of their own,
	// so no request waits for the relay. Messages still queued at
	// shutdown are lost (internal/mail says why).
	var mailer *mail.Mailer
	if cfg.Mail == nil {
		log.Warn("mail is off (no WEBAPI_SMTP_HOST): no links or notices are sent, " +
			"new accounts and resets cannot be made, and the email gate stays open")
	} else {
		if mailer, err = mail.New(*cfg.Mail, log.With("component", "mail")); err != nil {
			return err
		}
		go mailer.Run(ctx)
		log.Info("mail", "relay", net.JoinHostPort(cfg.Mail.Host, strconv.Itoa(cfg.Mail.Port)),
			"security", cfg.Mail.Security, "site", cfg.PublicURL)
	}

	signer := auth.NewSigner(cfg.AppKey)
	var acctLimiter *auth.RateLimiter
	if cfg.LoginMaxPerAccount > 0 {
		acctLimiter = auth.NewRateLimiter(cfg.LoginMaxPerAccount, cfg.LoginWindow)
	}
	handler := server.New(server.Deps{
		Log:    log,
		RO:     ro,
		App:    app,
		Signer: signer,
		Cookies: &auth.Cookies{
			Signer:  signer,
			Timeout: cfg.SessionTimeout,
			Secure:  cfg.SecureCookie,
		},
		Limiter:     auth.NewRateLimiter(cfg.LoginMaxAttempts, cfg.LoginWindow),
		AcctLimiter: acctLimiter,
		Daemon:      dc,
		Tokens:      auth.NewFileTokens(cfg.AppKey),
		Access:      access,
		Media: server.MediaConfig{
			StorageDir:   cfg.StorageDir,
			TDLibDir:     tdlib,
			StoreFileCap: cfg.StoreFileCap,
		},
		Proxies:    server.TrustedProxies(cfg.TrustedProxies),
		Live:       hub,
		LiveCfg:    server.LiveConfig{Recheck: cfg.LiveRecheck},
		Render:     renderer,
		Assets:     assets,
		AssetFiles: assetFiles,
		Defaults:   cfg.Defaults,
		MCP: server.MCPConfig{
			PublicURL:      cfg.PublicURL,
			Threads:        cfg.MCPThreads,
			AllowedOrigins: cfg.MCPAllowedOrigins,
			LogRequests:    cfg.MCPLogRequests,
		},
		Mail: mailer,
		MailCfg: server.MailConfig{
			Site:      cfg.PublicURL,
			InviteTTL: cfg.InviteLinkTTL,
			ResetTTL:  cfg.ResetLinkTTL,
			VerifyTTL: cfg.VerifyLinkTTL,
		},
	})

	addr := net.JoinHostPort(cfg.ListenAddr, strconv.Itoa(int(cfg.ListenPort)))
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		// The listener failed (address in use, ...) before any signal.
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("stopped")
	return nil
}

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	if err := run(log, cfg); err != nil {
		log.Error("exiting", "err", err)
		os.Exit(1)
	}
}

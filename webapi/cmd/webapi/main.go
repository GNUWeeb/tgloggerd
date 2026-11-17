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
	"strconv"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/config"
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

	addr := net.JoinHostPort(cfg.ListenAddr, strconv.Itoa(int(cfg.ListenPort)))
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(log, ro, app),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

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

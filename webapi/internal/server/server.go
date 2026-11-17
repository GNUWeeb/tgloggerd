// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package server holds the HTTP routes of the Go web service.
//
// Routing uses the standard library's ServeMux with method and path
// patterns ("GET /api/health"), which is all the API needs.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
)

// Deps is what the service is built from.
type Deps struct {
	Log     *slog.Logger
	RO      *sql.DB // SELECT-only over the logger's schema
	App     *sql.DB // read-write over the web's own schema
	Signer  *auth.Signer
	Cookies *auth.Cookies
	Limiter *auth.RateLimiter
	Daemon  *daemon.Client // nil when the daemon API is off
	// Tokens opens the /files/<token> tokens.
	Tokens *auth.FileTokens
	// Access collects the ids of the files served for the daemon; nil
	// when the daemon API is off.
	Access *daemon.AccessReporter
	Media  MediaConfig
}

// Server carries what the handlers share.
type Server struct {
	log      *slog.Logger
	ro       *sql.DB
	app      *sql.DB
	signer   *auth.Signer
	cookies  *auth.Cookies
	limiter  *auth.RateLimiter
	verifier *auth.Verifier
	daemon   *daemon.Client
	tokens   *auth.FileTokens
	access   *daemon.AccessReporter
	media    MediaConfig
}

// New returns the service's handler: the routes wrapped in request
// logging.
func New(d Deps) http.Handler {
	s := &Server{
		log:      d.Log,
		ro:       d.RO,
		app:      d.App,
		signer:   d.Signer,
		cookies:  d.Cookies,
		limiter:  d.Limiter,
		verifier: &auth.Verifier{Cookies: d.Cookies, Epoch: auth.DBEpoch(d.App)},
		daemon:   d.Daemon,
		tokens:   d.Tokens,
		access:   d.Access,
		media:    d.Media,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/login", s.getLogin)
	mux.HandleFunc("POST /api/login", s.postLogin)
	mux.HandleFunc("POST /api/logout", s.postLogout)
	mux.HandleFunc("GET /api/session", s.RequireSession(s.session))
	mux.HandleFunc("GET /files/{token}", s.serveFile)

	return s.logRequests(mux)
}

// healthTimeout bounds each pool's check, so a database that does not
// answer turns into a 503 rather than a hanging probe.
const healthTimeout = 3 * time.Second

// health checks both database pools. The answer says only which pool
// failed; the reason goes to the log, since the endpoint needs no login.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	status := http.StatusOK

	for _, p := range []struct {
		name string
		db   *sql.DB
	}{{"ro", s.ro}, {"app", s.app}} {
		ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		err := p.db.PingContext(ctx)
		cancel()
		if err != nil {
			s.log.Warn("health check failed", "pool", p.name, "err", err)
			checks[p.name] = "unavailable"
			status = http.StatusServiceUnavailable
			continue
		}
		checks[p.name] = "ok"
	}

	body := map[string]any{"status": "ok", "checks": checks}
	if status != http.StatusOK {
		body["status"] = "unavailable"
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// statusWriter remembers the status code a handler sent, for the log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// ReadFrom passes io.Copy through to the underlying writer, which
// sends files with sendfile(2); without it every file served would be
// copied through a buffer.
func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(w.ResponseWriter, r)
}

// Unwrap lets http.ResponseController reach the underlying writer
// (flushing, deadlines) through this wrapper.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start),
			"remote", r.RemoteAddr)
	})
}

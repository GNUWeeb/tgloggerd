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
	"os"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/live"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/render"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
)

// Deps is what the service is built from.
type Deps struct {
	Log     *slog.Logger
	RO      *sql.DB // SELECT-only over the logger's schema
	App     *sql.DB // read-write over the web's own schema
	Signer  *auth.Signer
	Cookies *auth.Cookies
	Limiter *auth.RateLimiter
	// AcctLimiter caps failed logins per account from all addresses;
	// nil leaves only Limiter, per address and name.
	AcctLimiter *auth.RateLimiter
	Daemon      *daemon.Client // nil when the daemon API is off
	// Tokens opens the /files/<token> tokens.
	Tokens *auth.FileTokens
	// Access collects the ids of the files served for the daemon; nil
	// when the daemon API is off.
	Access *daemon.AccessReporter
	Media  MediaConfig
	// Proxies are the reverse proxies whose X-Forwarded-For hops are
	// skipped to find a request's client (WEBAPI_TRUSTED_PROXIES).
	Proxies TrustedProxies
	// Live fans the daemon's events out to GET /api/events; nil when
	// live updates are off.
	Live    *live.Hub
	LiveCfg LiveConfig
	// Render is the render service's client; nil renders every page
	// in the browser (the fallback).
	Render *render.Client
	// Assets are the client bundle's files pages link, and its build.
	Assets render.Assets
	// AssetFiles is the bundle's assets/ directory, served under
	// /app/assets/; nil answers 404 there.
	AssetFiles *os.Root
	// Defaults are the site's settings for what an account did not
	// set; empty fields are settings.SiteDefaults.
	Defaults settings.Defaults

	// pages replaces the page table, state the database's account
	// state behind a session, clock the time, prefs the accounts'
	// stored preferences and pins their pinned groups, for tests.
	pages []pageRoute
	state auth.StateFunc
	clock func() time.Time
	prefs func(ctx context.Context, uid uint64) (settings.Prefs, error)
	pins  func(ctx context.Context, uid uint64, sc scope.Scope) ([]logdb.PinnedGroup, error)
	// discord replaces the client that sends webhook test messages.
	discord *http.Client
}

// Server carries what the handlers share.
type Server struct {
	log         *slog.Logger
	ro          *sql.DB
	app         *sql.DB
	signer      *auth.Signer
	cookies     *auth.Cookies
	limiter     *auth.RateLimiter
	acctLimiter *auth.RateLimiter
	verifier    *auth.Verifier
	daemon      *daemon.Client
	tokens      *auth.FileTokens
	access      *daemon.AccessReporter
	media       MediaConfig
	proxies     TrustedProxies
	live        *live.Hub
	liveCfg     LiveConfig
	// liveFilter replaces eventInScope in tests.
	liveFilter func(scope.Scope, live.Event) (live.Event, bool)
	render     *render.Client
	assets     render.Assets
	assetFiles *os.Root
	defaults   settings.Defaults
	// viewerPrefs reads an account's preferences (settings.Load),
	// viewerPins its pinned groups in the scope (pinnedGroups).
	viewerPrefs func(ctx context.Context, uid uint64) (settings.Prefs, error)
	viewerPins  func(ctx context.Context, uid uint64, sc scope.Scope) ([]logdb.PinnedGroup, error)
	pages       []pageRoute
	clock       func() time.Time
	// discord sends the webhook test messages.
	discord *http.Client
}

// New returns the service's handler: the routes wrapped in request
// logging.
func New(d Deps) http.Handler {
	s := &Server{
		log:         d.Log,
		ro:          d.RO,
		app:         d.App,
		signer:      d.Signer,
		cookies:     d.Cookies,
		limiter:     d.Limiter,
		acctLimiter: d.AcctLimiter,
		verifier:    &auth.Verifier{Cookies: d.Cookies, State: auth.DBState(d.App)},
		daemon:      d.Daemon,
		tokens:      d.Tokens,
		access:      d.Access,
		media:       d.Media,
		proxies:     d.Proxies,
		live:        d.Live,
		liveCfg:     d.LiveCfg,
		render:      d.Render,
		assets:      d.Assets,
		assetFiles:  d.AssetFiles,
		defaults:    d.Defaults.OrSite(),
		pages:       d.pages,
		clock:       time.Now,
	}
	if s.pages == nil {
		s.pages = defaultPages()
	}
	if d.state != nil {
		s.verifier.State = d.state
	}
	if d.clock != nil {
		s.clock = d.clock
	}
	s.viewerPrefs = func(ctx context.Context, uid uint64) (settings.Prefs, error) {
		return settings.Load(ctx, s.app, uid)
	}
	if d.prefs != nil {
		s.viewerPrefs = d.prefs
	}
	s.viewerPins = s.pinnedGroups
	if d.pins != nil {
		s.viewerPins = d.pins
	}
	s.discord = d.discord
	if s.discord == nil {
		s.discord = &http.Client{
			Timeout: discordTimeout,
			// A redirect could lead away from Discord's hosts.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	mux := http.NewServeMux()
	for _, rt := range s.routes() {
		mux.HandleFunc(rt.pattern, rt.handler)
	}
	s.registerPages(mux)

	return s.withClient(s.logRequests(mux))
}

// route is one entry of the route table.
type route struct {
	pattern string // ServeMux pattern, "METHOD /path"
	handler http.HandlerFunc
}

// routes is the service's route table. It only builds handler values
// and touches none of s's fields, so the OpenAPI test lists it from a
// zero Server; every operation served must be in webapi/openapi.yaml.
func (s *Server) routes() []route {
	return []route{
		{"GET /api/health", s.health},
		{"GET /api/login", s.getLogin},
		{"POST /api/login", s.postLogin},
		{"POST /api/logout", s.postLogout},
		{"GET /api/session", s.RequireSession(s.session, allGates...)},
		{"GET /files/{token}", s.serveFile},
		{"GET /api/events", s.RequireSession(s.events)},
		{"GET /api/page/{path...}", s.getPage},
		{"GET /app/assets/{file...}", s.serveAsset},
		{"POST /api/account/password", s.RequireSession(s.changePassword, scope.PasswordChange)},
		{"GET /api/accounts", s.RequireSession(s.listAccounts)},
		{"POST /api/accounts", s.RequireSession(s.createAccount)},
		{"GET /api/accounts/{id}", s.RequireSession(s.getAccount)},
		{"PATCH /api/accounts/{id}", s.RequireSession(s.updateAccount)},
		{"POST /api/accounts/{id}/disable", s.RequireSession(s.disableAccount)},
		{"POST /api/accounts/{id}/enable", s.RequireSession(s.enableAccount)},
		{"POST /api/accounts/{id}/password-reset", s.RequireSession(s.resetAccountPassword)},
		{"POST /api/accounts/{id}/sign-out-everywhere", s.RequireSession(s.signOutAccountEverywhere)},
		{"GET /api/chats/{scope}/{id}/messages", s.getChatMessages},
		{"GET /api/entities", s.RequireSession(s.getEntities)},
		{"GET /api/search/{entity}", s.RequireSession(s.searchEntity)},
		{"GET /api/chats/search", s.RequireSession(s.searchChats)},
		{"PUT /api/account/preferences", s.RequireSession(s.putPreferences)},
		{"GET /api/account/preferences", s.RequireSession(s.getPreferences)},
		{"PUT /api/account/pinned/{group_id}", s.RequireSession(s.pinGroup)},
		{"DELETE /api/account/pinned/{group_id}", s.RequireSession(s.unpinGroup)},
		{"POST /api/account/saved-searches", s.RequireSession(s.saveSearch)},
		{"DELETE /api/account/saved-searches/{id}", s.RequireSession(s.deleteSavedSearch)},
		{"GET /api/account/sessions", s.RequireSession(s.listSessions)},
		{"DELETE /api/account/sessions/{id}", s.RequireSession(s.endSession)},
		{"POST /api/account/sessions/sign-out-others", s.RequireSession(s.endOtherSessions)},
		{"POST /api/federations", s.RequireSession(s.createFederation)},
		{"PATCH /api/federations/{id}", s.RequireSession(s.updateFederation)},
		{"DELETE /api/federations/{id}", s.RequireSession(s.deleteFederation)},
		{"POST /api/forwarding/telegram-discord", s.RequireSession(s.createT2D)},
		{"POST /api/forwarding/telegram-discord/test", s.RequireSession(s.testT2D)},
		{"PATCH /api/forwarding/telegram-discord/{id}", s.RequireSession(s.updateT2D)},
		{"DELETE /api/forwarding/telegram-discord/{id}", s.RequireSession(s.deleteT2D)},
		{"POST /api/forwarding/discord-telegram", s.RequireSession(s.createD2T)},
		{"PATCH /api/forwarding/discord-telegram/{id}", s.RequireSession(s.updateD2T)},
		{"DELETE /api/forwarding/discord-telegram/{id}", s.RequireSession(s.deleteD2T)},
		{"POST /api/join-chat/preview", s.RequireSession(s.previewJoin)},
		{"POST /api/join-chat/join", s.RequireSession(s.joinChat)},
		{"POST /api/users/{id}/lookup-name", s.RequireSession(s.lookupName)},
		{"POST /api/mcp/groups", s.RequireSession(s.exposeMcpGroup)},
		{"DELETE /api/mcp/groups/{group_id}", s.RequireSession(s.unexposeMcpGroup)},
		{"POST /api/mcp/tokens", s.RequireSession(s.createMcpToken)},
		{"POST /api/mcp/tokens/{id}/revoke", s.RequireSession(s.revokeMcpToken)},
	}
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
			"client", clientIP(r),
			"remote", r.RemoteAddr)
	})
}

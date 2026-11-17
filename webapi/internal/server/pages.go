// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/render"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
)

// Pages of the React application (docs/design/web-ssr.md): a page path
// resolves to a route id and its loader, whose plain values are the
// page's props. A document request answers the HTML document, rendered
// by the render service or, when it does not answer, the same document
// with empty markup for the browser to render; GET /api/page/{path}
// answers the same page data as JSON for navigation in the browser.
//
// A route without a loader is a page whose data this service does not
// produce yet. Its document is not served here (nginx sends the path to
// the Drogon web), and /api/page answers {"reload": true} for it, so
// the browser loads it in full from there.

// pageRoute is one entry of the page table.
type pageRoute struct {
	id string
	// pattern is the page path: literal segments, {id} for an integer
	// (a Telegram id, negative for chats) and {scope} for group or
	// private. It is also the ServeMux pattern of the document.
	pattern string
	// public pages are served to signed-out visitors as well; every
	// other page sends them to the sign-in page.
	public bool
	// link pages are the email link pages, exempt from the missing
	// email address gate.
	link bool
	load pageLoader
}

// pageRequest is what a loader gets: the request, its session (nil for
// a signed-out visitor), its scope and the path's parameters.
type pageRequest struct {
	r      *http.Request
	sess   *auth.Session
	scope  scope.Scope
	params map[string]string
	query  url.Values
}

// pageLoader returns a page's props, or errPageNotFound for something
// that does not exist or that the scope hides: both answer the
// not_found page, indistinguishably.
type pageLoader func(ctx context.Context, pr *pageRequest) (any, error)

var errPageNotFound = errors.New("page not found")

// defaultPages is the page table. The ids are the React application's
// route registry's (web/app/src/routes/index.tsx) and the paths those of
// the getPage route table in webapi/openapi.yaml. A port adds the
// loader of each page whose data it brings.
func defaultPages() []pageRoute {
	return []pageRoute{
		{id: "login", pattern: "/login", public: true, load: loadLogin},
		{id: "dashboard", pattern: "/"},
		{id: "users", pattern: "/users"},
		{id: "user", pattern: "/users/{id}"},
		{id: "user_history", pattern: "/users/{id}/history"},
		{id: "user_chat", pattern: "/users/{id}/chat"},
		{id: "groups", pattern: "/groups"},
		{id: "group", pattern: "/groups/{id}"},
		{id: "group_admins", pattern: "/groups/{id}/admins"},
		{id: "group_history", pattern: "/groups/{id}/history"},
		// The chats of groups exposed through MCP are public; the
		// loader decides which group is exposed.
		{id: "group_chat", pattern: "/groups/{id}/chat", public: true},
		{id: "files", pattern: "/files"},
		{id: "group_messages", pattern: "/group_messages"},
		{id: "private_messages", pattern: "/private_messages"},
		{id: "message", pattern: "/messages/{scope}/{id}"},
		{id: "settings", pattern: "/settings"},
		{id: "settings_preferences", pattern: "/settings/preferences"},
		{id: "settings_sessions", pattern: "/settings/sessions"},
		{id: "accounts", pattern: "/settings/accounts"},
		{id: "federations", pattern: "/settings/federations"},
		{id: "platform_fwd", pattern: "/platform-fwd"},
		{id: "fwd_telegram_discord", pattern: "/platform-fwd/telegram-discord"},
		{id: "fwd_discord_telegram", pattern: "/platform-fwd/discord-telegram"},
		{id: "join_chat", pattern: "/join-chat"},
		{id: "mcp", pattern: "/mcp-admin"},
		{id: "mcp_groups", pattern: "/mcp-admin/groups"},
		{id: "mcp_tokens", pattern: "/mcp-admin/tokens"},
		// The link pages change nothing on GET (the token is in the
		// URL's fragment); their actions are not served here yet.
		{id: "password_forgot", pattern: "/password/forgot", public: true, link: true},
		{id: "password_set", pattern: "/password/set", public: true, link: true},
		{id: "email_verify", pattern: "/account/email/verify", public: true, link: true},
	}
}

// pageNav lists the pages the menu shows, from the scope: hidden
// pages are absent, never disabled.
func pageNav(sc scope.Scope) []string {
	return sc.Nav()
}

// registerPages serves the document of every page that has a loader at
// its own path. The API's route table (routes) holds only the JSON
// operations; the page paths are listed in getPage's description.
func (s *Server) registerPages(mux *http.ServeMux) {
	for _, p := range s.pages {
		if p.load == nil {
			continue
		}
		pat := p.pattern
		if pat == "/" {
			pat = "/{$}"
		}
		mux.HandleFunc("GET "+pat, s.servePage)
	}
}

// matchPage resolves a page path (unescaped, with its leading slash).
// A trailing slash is ignored; empty, "." and ".." segments match
// nothing.
func (s *Server) matchPage(path string) (*pageRoute, map[string]string) {
	if !strings.HasPrefix(path, "/") {
		return nil, nil
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	segs := strings.Split(path[1:], "/")
	if path == "/" {
		segs = nil
	}
	for _, sg := range segs {
		if sg == "" || sg == "." || sg == ".." {
			return nil, nil
		}
	}
	for i := range s.pages {
		p := &s.pages[i]
		if params, ok := matchSegments(p.pattern, segs); ok {
			return p, params
		}
	}
	return nil, nil
}

func matchSegments(pattern string, segs []string) (map[string]string, bool) {
	var want []string
	if pattern != "/" {
		want = strings.Split(pattern[1:], "/")
	}
	if len(want) != len(segs) {
		return nil, false
	}
	params := map[string]string{}
	for i, w := range want {
		sg := segs[i]
		switch w {
		case "{id}":
			if _, err := strconv.ParseInt(sg, 10, 64); err != nil || sg[0] == '+' {
				return nil, false
			}
			params["id"] = sg
		case "{scope}":
			if sg != "group" && sg != "private" {
				return nil, false
			}
			params["scope"] = sg
		default:
			if w != sg {
				return nil, false
			}
		}
	}
	return params, true
}

// pageOutcome is what a page request comes to besides page data.
type pageOutcome int

const (
	pageServed pageOutcome = iota // the page data, 200 or 404
	pageReload                    // not served here yet: load it in full
	pageSignIn                    // the page needs a session
)

// pageNotFoundProps are the not_found page's props: none.
type pageNotFoundProps struct{}

// buildPage runs the page request for path: the sign-in rule, the
// email address gate, the route's loader with the request's scope.
func (s *Server) buildPage(r *http.Request, sess *auth.Session, path string) (render.Page, *pageRoute, pageOutcome, error) {
	rt, params := s.matchPage(path)
	pageURL := path
	if r.URL.RawQuery != "" {
		pageURL += "?" + r.URL.RawQuery
	}

	// A signed-out visitor sees the public pages only; for anything
	// else, existing or not, it is asked to sign in first.
	if sess == nil && (rt == nil || !rt.public) {
		return render.Page{URL: pageURL}, rt, pageSignIn, nil
	}

	sc := s.requestScope(r, sess)
	viewer, err := s.pageViewer(r.Context(), sess, sc)
	if err != nil {
		return render.Page{}, rt, pageServed, err
	}
	page := render.Page{
		V: render.Protocol, URL: pageURL, Status: http.StatusOK,
		Viewer: viewer,
	}

	// A signed-in account without a verified email address gets the
	// email_required page at the URL it asked for, without a redirect,
	// for every page but the link pages (web-accounts.md section 8).
	if sess != nil && (rt == nil || !rt.link) {
		pending, required, err := s.emailRequired(r.Context(), sess)
		if err != nil {
			return render.Page{}, rt, pageServed, err
		}
		if required {
			page.Route = "email_required"
			page.Props = emailRequiredProps{Pending: pending, Next: pageURL}
			return page, rt, pageServed, nil
		}
	}

	if rt != nil && rt.load == nil {
		return render.Page{URL: pageURL}, rt, pageReload, nil
	}
	if rt == nil {
		return s.notFoundPage(page), nil, pageServed, nil
	}
	props, err := rt.load(r.Context(), &pageRequest{
		r: r, sess: sess, scope: sc, params: params, query: r.URL.Query(),
	})
	if errors.Is(err, errPageNotFound) {
		return s.notFoundPage(page), rt, pageServed, nil
	}
	if err != nil {
		return render.Page{}, rt, pageServed, err
	}
	page.Route = rt.id
	page.Props = props
	return page, rt, pageServed, nil
}

// emailRequiredProps are the email_required page's
// (EmailRequiredProps).
type emailRequiredProps struct {
	// Pending is an address added but not verified yet.
	Pending *string `json:"pending"`
	// Next is the page asked for, to go on to once verified.
	Next string `json:"next"`
}

// emailRequired is the hook of the missing email address gate: whether
// the account must verify an address before anything else, and the
// address waiting for verification, if any. Accounts have no addresses
// until web-email adds them, so nothing is required yet; web-email
// fills this in.
func (s *Server) emailRequired(_ context.Context, _ *auth.Session) (*string, bool, error) {
	return nil, false, nil
}

func (s *Server) notFoundPage(p render.Page) render.Page {
	p.Route = "not_found"
	p.Status = http.StatusNotFound
	p.Props = pageNotFoundProps{}
	return p
}

// pageViewer is the viewer of the page data: the account and its menu,
// and every display setting in effect, the account's own or the site's
// default, so the server's render and the browser's agree. A
// signed-out visitor gets the site's.
func (s *Server) pageViewer(ctx context.Context, sess *auth.Session, sc scope.Scope) (render.Viewer, error) {
	v := render.Viewer{Now: s.clock().UTC().Format(time.RFC3339)}
	var prefs settings.Prefs
	if sess != nil {
		var err error
		if prefs, err = s.viewerPrefs(ctx, sess.UID); err != nil {
			return render.Viewer{}, err
		}
	}
	e := settings.Resolve(prefs, s.defaults)
	v.Timezone, v.DateFormat, v.Lang, v.Theme = e.Timezone, e.DateFormat, e.Lang, e.Theme
	v.RowsPerPage = e.RowsPerPage
	v.Chat = &e.Chat
	if sess == nil {
		return v, nil
	}
	v.SignedIn = true
	v.Account = &render.Account{
		ID:       sess.UID,
		Username: sess.Username,
		// Accounts get a display name with web-roles; until then
		// the name is the login name.
		Name: sess.Username,
		Role: sess.Role,
	}
	v.Nav = pageNav(sc)
	return v, nil
}

// pageSession checks the request's session for a page: a valid one is
// returned; none or a revoked one (whose cookie is cleared) makes the
// visitor a signed-out one. A session the database could not check
// gets false: the caller answers 503 and keeps the cookie.
func (s *Server) pageSession(w http.ResponseWriter, r *http.Request) (*auth.Session, bool) {
	sess, c := s.verifier.Verify(r.Context(), r)
	switch c {
	case auth.Valid:
		s.noteSessionUse(r, &sess)
		return &sess, true
	case auth.Revoked:
		s.cookies.Clear(w)
	case auth.Unverified:
		s.log.Warn("session not verified", "uid", sess.UID)
		return nil, false
	}
	return nil, true
}

// pageCache sets the caching of a page answer: a public page asked for
// without a session cookie may be cached for a minute by anyone; every
// other answer is the viewer's own.
func pageCache(w http.ResponseWriter, r *http.Request, rt *pageRoute) {
	h := w.Header()
	h.Add("Vary", "Cookie")
	if rt != nil && rt.public && auth.RawSession(r) == "" {
		h.Set("Cache-Control", "public, max-age=60")
		return
	}
	h.Set("Cache-Control", "private, no-store")
}

// signInURL is the sign-in page that comes back to pageURL.
func signInURL(pageURL string) string {
	next := safeNext(pageURL)
	if next == nil {
		return "/login"
	}
	return "/login?next=" + url.QueryEscape(*next)
}

// safeNext keeps a path to go on to only when it stays on this site: a
// path with one leading slash, no backslash and no control character
// (a browser reads "/\" as "//", another host).
func safeNext(next string) *string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsRune(next, '\\') {
		return nil
	}
	for _, c := range next {
		if c < 0x20 || c == 0x7f {
			return nil
		}
	}
	return &next
}

// loginProps are the sign-in page's (LoginProps).
type loginProps struct {
	// Next is where to go once signed in; null when absent or not a
	// same-site path.
	Next *string `json:"next"`
}

func loadLogin(_ context.Context, pr *pageRequest) (any, error) {
	return loginProps{Next: safeNext(pr.query.Get("next"))}, nil
}

// getPage is GET /api/page/{path}: the page data for navigation in the
// browser. ServeMux has unescaped the path, so the slashes the client
// encodes as %2F are slashes here; an empty path is the dashboard.
func (s *Server) getPage(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.pageSession(w, r)
	if !ok {
		noStore(w)
		writeError(w, http.StatusServiceUnavailable, "unavailable", nil)
		return
	}
	path := "/" + r.PathValue("path")
	page, rt, outcome, err := s.buildPage(r, sess, path)
	switch {
	case err != nil:
		noStore(w)
		s.internalError(w, "page "+path, err)
	case outcome == pageSignIn:
		noStore(w)
		writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
	case outcome == pageReload:
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]bool{"reload": true})
	default:
		pageCache(w, r, rt)
		writeJSON(w, page.Status, page)
	}
}

// newRequestID names one render in both services' logs.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// servePage answers a page's document.
func (s *Server) servePage(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.pageSession(w, r)
	if !ok {
		noStore(w)
		statusPage(w, http.StatusServiceUnavailable, "Service unavailable",
			"Your session could not be checked. Please try again shortly.")
		return
	}
	page, rt, outcome, err := s.buildPage(r, sess, r.URL.Path)
	switch {
	case err != nil:
		s.log.Error("page", "path", r.URL.Path, "err", err)
		noStore(w)
		statusPage(w, http.StatusInternalServerError, "Server error",
			"The page could not be loaded.")
		return
	case outcome == pageSignIn:
		noStore(w)
		http.Redirect(w, r, signInURL(page.URL), http.StatusFound)
		return
	case outcome == pageReload:
		// Only pages with a loader are routed here.
		s.log.Error("page without a loader routed here", "path", r.URL.Path)
		noStore(w)
		statusPage(w, http.StatusNotFound, "Not found", "This page is not served here.")
		return
	}

	id := newRequestID()
	var res *render.Result
	if out, err := s.render.Render(r.Context(), page, id, s.assets.Build); err == nil {
		res = &out
		s.log.Debug("rendered", "request_id", id, "route", page.Route, "ms", out.MS)
	} else if !errors.Is(err, render.ErrSkipped) {
		s.log.Warn("render failed, serving the fallback",
			"request_id", id, "route", page.Route, "err", err)
	}

	// The CSRF token of the session cookie this request carries, so a
	// document after a password change, which reissued the cookie,
	// has the new one.
	csrf := ""
	if sess != nil {
		csrf = s.signer.SessionCSRF(auth.RawSession(r))
	}
	doc, err := render.Document(page, res, csrf, s.assets)
	if err != nil {
		s.log.Error("page document", "route", page.Route, "err", err)
		noStore(w)
		statusPage(w, http.StatusInternalServerError, "Server error",
			"The page could not be loaded.")
		return
	}
	pageCache(w, r, rt)
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(page.Status)
	_, _ = w.Write(doc)
}

// assetCacheControl: the bundle's file names carry a content hash.
const assetCacheControl = "public, max-age=31536000, immutable"

// serveAsset serves the client bundle's files under /app/assets/
// (web-ssr.md section 6), from the bundle's assets/ directory only.
func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if s.assetFiles == nil || name == "" {
		http.NotFound(w, r)
		return
	}
	f, err := s.assetFiles.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", assetCacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

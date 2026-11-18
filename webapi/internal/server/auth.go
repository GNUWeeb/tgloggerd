// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// Sign-in for the JSON API, done as web/src/controllers/AuthController.cpp
// does it for the Drogon pages: the same CSRF scheme, throttle, audit
// rows and session cookie, so a session started here is valid there
// and the other way round. The contract is in webapi/README.

// CSRFHeader carries the CSRF token of every state-changing API
// request. The Drogon forms send it as the "csrf" field instead; the
// token is the same.
const CSRFHeader = "X-CSRF-Token"

// loginPath scopes the pre-login CSRF cookie to the login endpoint, as
// the Drogon web scopes its own to /login.
const loginPath = "/api/login"

// maxLoginBody bounds a login request's body.
const maxLoginBody = 64 << 10

// maxUsername is the length of web_users.username, in characters.
const maxUsername = 64

// The messages are the Drogon login page's.
const (
	msgExpired  = "Your session expired. Please try again."
	msgThrottle = "Too many attempts. Please wait and try again."
	msgInvalid  = "Invalid username or password."
)

type userJSON struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	// MustChangePassword: the account signed in with a temporary
	// password; everything but session, logout and POST
	// /api/account/password answers 403 password_change_required.
	MustChangePassword bool `json:"must_change_password"`
}

type ctxKey int

const (
	sessionKey ctxKey = iota
	scopeKey
)

// SessionFrom is the verified session RequireSession put in the
// request's context.
func SessionFrom(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(sessionKey).(auth.Session)
	return s, ok
}

// ScopeFrom is the request scope RequireSession built for the request
// (Server.requestScope); without one, the anonymous scope.
func ScopeFrom(ctx context.Context) scope.Scope {
	sc, _ := ctx.Value(scopeKey).(scope.Scope)
	return sc
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func writeError(w http.ResponseWriter, status int, code string, extra map[string]any) {
	body := map[string]any{"error": code}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, body)
}

// RequireSession lets only requests with a valid session through, as
// AuthFilter does. Where the Drogon filter redirects to /login, the API
// answers 401, clearing a revoked cookie; a session that could not be
// checked for a database error gets 503 and keeps its cookie.
//
// It puts the session and the request scope in the request's context
// (SessionFrom, ScopeFrom). While the scope has a gate (a forced
// password change, later a missing address), the request answers 403
// with the gate as its error code, unless the route passes that gate:
// the operations that take the step, and the session itself.
func (s *Server) RequireSession(next http.HandlerFunc, passes ...scope.Gate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, c := s.verifier.Verify(r.Context(), r)
		switch c {
		case auth.Valid:
			s.noteSessionUse(r, &sess)
			sc := s.requestScope(r, &sess)
			if g := sc.Gate(); g != scope.None && !slices.Contains(passes, g) {
				noStore(w)
				writeError(w, http.StatusForbidden, string(g),
					map[string]any{"message": gateMessages[g]})
				return
			}
			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next(w, r.WithContext(context.WithValue(ctx, scopeKey, sc)))
			return
		case auth.Revoked:
			s.cookies.Clear(w)
			noStore(w)
			writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
		case auth.Unverified:
			s.log.Warn("session not verified", "uid", sess.UID)
			noStore(w)
			writeError(w, http.StatusServiceUnavailable, "unavailable", nil)
		default:
			noStore(w)
			writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
		}
	}
}

// gateMessages are the sentences of the gates' 403 answers.
var gateMessages = map[scope.Gate]string{
	scope.PasswordChange: "Choose a new password before going on.",
	scope.EmailRequired:  "Add and verify an email address before going on.",
}

// allGates is what a route that every signed-in account may use passes.
var allGates = []scope.Gate{scope.PasswordChange, scope.EmailRequired}

// session answers who is signed in and the CSRF token to send.
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":    userJSON{sess.UID, sess.Username, sess.Role, sess.MustChangePassword},
		"csrf":    s.signer.SessionCSRF(auth.RawSession(r)),
		"landing": s.landing(r.Context(), sess.UID),
	})
}

// newLoginToken sets a fresh pre-login CSRF cookie and returns the
// token to echo, as the Drogon login page does on every render.
func (s *Server) newLoginToken(w http.ResponseWriter) string {
	tok := s.signer.NewLoginToken()
	s.cookies.SetLoginCookie(w, tok, loginPath)
	return tok
}

// getLogin starts a sign-in. A signed-in caller (by the cookie's
// signature alone, as GET /login checks it) gets no token.
func (s *Server) getLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if _, ok := s.cookies.Current(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{"signed_in": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"signed_in": false,
		"csrf":      s.newLoginToken(w),
	})
}

// loginFailure answers a refused sign-in with a fresh login token, so
// the client can try again.
func (s *Server) loginFailure(w http.ResponseWriter, status int, code, msg string) {
	tok := s.newLoginToken(w)
	writeError(w, status, code, map[string]any{"message": msg, "csrf": tok})
}

func (s *Server) internalError(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", nil)
}

// postLogin checks the credentials in the order AuthController does:
// CSRF, then the throttle, then the account and its password.
func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	ctx := r.Context()

	if !s.signer.CheckLogin(r, r.Header.Get(CSRFHeader)) {
		s.loginFailure(w, http.StatusForbidden, "csrf", msgExpired)
		return
	}

	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody))
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			map[string]any{"message": "The body must be a JSON object."})
		return
	}

	// No account has a longer name (web_users.username), so a longer
	// one fails without a lookup, and only its first maxUsername
	// characters key the throttle.
	name, tooLong := cutRunes(in.Username, maxUsername)
	ip := clientIP(r)
	key := ip + "\n" + name
	if !s.limiter.Allowed(key) {
		if err := webdb.Audit(ctx, s.app, nil, "login_ratelimited", ip, in.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		s.loginFailure(w, http.StatusTooManyRequests, "rate_limited", msgThrottle)
		return
	}

	u, err := webdb.User{}, webdb.ErrNoUser
	if !tooLong {
		u, err = webdb.FindByUsername(ctx, s.app, in.Username)
	}
	if err != nil && !errors.Is(err, webdb.ErrNoUser) {
		s.internalError(w, "account lookup", err)
		return
	}

	// The account cap counts failures per account from every address,
	// so guesses spread over many addresses still stop. It keys on the
	// account the name found, since the name column matches without
	// regard to case or accents; a name that finds none keys on itself.
	acctKey := "name\n" + strings.ToLower(name)
	if err == nil {
		acctKey = "id\n" + strconv.FormatUint(u.ID, 10)
	}
	if s.acctLimiter != nil && !s.acctLimiter.Allowed(acctKey) {
		if err := webdb.Audit(ctx, s.app, nil, "login_ratelimited", ip, in.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		s.loginFailure(w, http.StatusTooManyRequests, "rate_limited", msgThrottle)
		return
	}

	ok := err == nil && u.IsActive && auth.VerifyPassword(u.PasswordHash, in.Password)
	if !ok {
		s.limiter.RecordFailure(key)
		if s.acctLimiter != nil {
			s.acctLimiter.RecordFailure(acctKey)
		}
		if err := webdb.Audit(ctx, s.app, nil, "login_fail", ip, in.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		s.loginFailure(w, http.StatusUnauthorized, "invalid_credentials", msgInvalid)
		return
	}

	s.limiter.Reset(key)
	if s.acctLimiter != nil {
		s.acctLimiter.Reset(acctKey)
	}
	if err := webdb.Audit(ctx, s.app, &u.ID, "login_ok", ip, u.Username); err != nil {
		s.internalError(w, "audit", err)
		return
	}

	raw, err := s.startSession(ctx, w, r, u.ID, u.Username, u.Role, u.Epoch, nil)
	if err != nil {
		s.internalError(w, "start session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":    userJSON{u.ID, u.Username, u.Role, u.MustChangePassword},
		"csrf":    s.signer.SessionCSRF(raw),
		"landing": s.landing(ctx, u.ID),
	})
}

// postLogout ends the session on this browser, as POST /logout does:
// the CSRF token must match the session cookie, the logout is audited
// when the cookie opens, and the cookie is cleared. The session's
// record is ended too, so a copy of the cookie stops working on both
// services. The account's other sessions stay; the sessions page or a
// password change ends those.
func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.signer.CheckSessionCSRF(r, r.Header.Get(CSRFHeader)) {
		writeError(w, http.StatusForbidden, "csrf", nil)
		return
	}
	if sess, ok := s.cookies.Current(r); ok {
		uid := sess.UID
		if err := webdb.EndSessionKey(r.Context(), s.app, webdb.NewSession{
			UID: uid, Key: auth.SessionKey(auth.RawSession(r)), Epoch: sess.Epoch,
			Expires: sess.Exp, UserAgent: r.UserAgent(), IP: clientIP(r),
		}); err != nil {
			s.internalError(w, "end session", err)
			return
		}
		if err := webdb.Audit(r.Context(), s.app, &uid, "logout", clientIP(r), sess.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
	}
	s.cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

// cutRunes returns s cut to at most n characters, and whether it was
// longer.
func cutRunes(s string, n int) (string, bool) {
	c := 0
	for i := range s {
		if c == n {
			return s[:i], true
		}
		c++
	}
	return s, false
}

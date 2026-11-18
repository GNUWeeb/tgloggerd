// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/webdb"
)

// The record of each session (web_sessions,
// decisions/2026-11-18-sessions-list.md): the sessions this service
// signs in are recorded with their device and address, sessions it
// only sees (signed in by the Drogon web, or before the records) are
// recorded when first seen, and each request keeps the last use.

// sessionTries bounds the search for a free cookie at sign-in. Two
// sign-ins of one account in the same second would make the same
// cookie; each try takes one second off the expiry, so this many
// sign-ins in one second still get a session each.
const sessionTries = 64

// startSession signs a session in: it records the session and sets
// its cookie, which it returns. from is the key of the session the new
// one replaces (a password change reissues the cookie), nil for a new
// sign-in.
func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request,
	uid uint64, username, role string, epoch uint32, from *[32]byte) (string, error) {
	sess := s.cookies.NewSession(uid, username, role, epoch)
	if sess.Exp == 0 {
		// Every session would get the same cookie otherwise.
		sess.Exp = auth.NeverExpires
	}
	for range sessionTries {
		raw := s.signer.EncodeSession(sess)
		err := webdb.RecordSession(ctx, s.app, webdb.NewSession{
			UID: uid, Key: auth.SessionKey(raw), Epoch: epoch, Expires: sess.Exp,
			UserAgent: r.UserAgent(), IP: clientIP(r), From: from,
		})
		if errors.Is(err, webdb.ErrSessionExists) {
			sess.Exp--
			continue
		}
		if err != nil {
			return "", err
		}
		if err := webdb.PruneSessions(ctx, s.app, uid, epoch); err != nil {
			s.log.Warn("prune sessions", "uid", uid, "err", err)
		}
		return s.cookies.Issue(w, sess), nil
	}
	return "", errors.New("no free session cookie")
}

// noteSessionUse records a valid session's use: its row the first time
// it is seen, else its last use once TouchInterval has passed. A
// failure only goes to the log; the request goes on.
func (s *Server) noteSessionUse(r *http.Request, sess *auth.Session) {
	var err error
	switch {
	case sess.Row == 0:
		n := webdb.NewSession{
			UID: sess.UID, Key: sess.Key, Epoch: sess.Epoch, Expires: sess.Exp,
			UserAgent: r.UserAgent(), IP: clientIP(r),
		}
		// A cookie with an expiry was issued a session timeout before
		// it (a start in the future is taken as now); one that never
		// expires tells nothing, so it starts now.
		if t := s.cookies.Timeout; t > 0 && sess.Exp != 0 {
			n.SignedIn = time.Unix(sess.Exp-int64(t), 0)
		}
		err = webdb.SeenSession(r.Context(), s.app, n)
	case sess.Stale:
		err = webdb.TouchSession(r.Context(), s.app, sess.Row, clientIP(r))
	}
	if err != nil {
		s.log.Warn("record session use", "uid", sess.UID, "err", err)
	}
}

// sessionJSON is a session as the sessions page lists it (the
// SessionsProps schema).
type sessionJSON struct {
	ID      string `json:"id"`
	Device  string `json:"device"`
	IP      string `json:"ip"`
	Place   string `json:"place"`
	Started string `json:"started"`
	Last    string `json:"last"`
	Current bool   `json:"current"`
}

// accountSessions are the account's live sessions, this one first.
func (s *Server) accountSessions(ctx context.Context, sess *auth.Session) ([]sessionJSON, error) {
	rows, err := webdb.ListSessions(ctx, s.app, sess.UID)
	if err != nil {
		return nil, err
	}
	lang := s.textLang(ctx, sess.UID)
	out := []sessionJSON{}
	for _, row := range rows {
		j := sessionJSON{
			ID:      strconv.FormatUint(row.ID, 10),
			Device:  deviceName(lang, row.UserAgent),
			IP:      row.IP,
			Started: row.SignedIn.Format(time.RFC3339),
			Last:    row.LastUsed.Format(time.RFC3339),
			Current: row.Key == sess.Key,
		}
		if j.Current {
			out = append([]sessionJSON{j}, out...)
		} else {
			out = append(out, j)
		}
	}
	return out, nil
}

// endSession answers DELETE /api/account/sessions/{id}: that session
// ends on both services. Ending this browser's own session signs it
// out, as a logout does.
func (s *Server) endSession(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	ok := err == nil && id != 0
	if ok {
		if ok, err = webdb.EndSession(ctx, s.app, sess.UID, id); err != nil {
			s.internalError(w, "end session", err)
			return
		}
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", nil)
		return
	}
	uid := sess.UID
	if err := webdb.Audit(ctx, s.app, &uid, "session_end", clientIP(r),
		"session "+strconv.FormatUint(id, 10)); err != nil {
		s.internalError(w, "audit", err)
		return
	}
	if id == sess.Row {
		s.cookies.Clear(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

// endOtherSessions answers POST /api/account/sessions/sign-out-others:
// every other session of the account ends on both services, whether
// this service ever saw it or not, as a password change ends them
// (the epoch moves on). This session gets a new cookie, keeping its
// record, and the answer its new CSRF token.
func (s *Server) endOtherSessions(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	epoch, err := webdb.EndOtherSessions(ctx, s.app, sess.UID)
	if err != nil {
		s.internalError(w, "end sessions", err)
		return
	}
	uid := sess.UID
	if err := webdb.Audit(ctx, s.app, &uid, "sessions_signout_others", clientIP(r),
		sess.Username); err != nil {
		s.internalError(w, "audit", err)
		return
	}
	raw, err := s.startSession(ctx, w, r, sess.UID, sess.Username, sess.Role, epoch, &sess.Key)
	if err != nil {
		s.internalError(w, "start session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"csrf": s.signer.SessionCSRF(raw)})
}

// The browsers and systems deviceName tells apart, in the order they
// are tried: a token several User-Agents share (Chrome's "Safari/")
// comes after the ones that carry it too.
var (
	uaBrowsers = []struct{ token, name string }{
		{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"Firefox/", "Firefox"},
		{"FxiOS/", "Firefox"}, {"CriOS/", "Chrome"}, {"Chrome/", "Chrome"},
		{"Safari/", "Safari"}, {"curl/", "curl"},
	}
	uaSystems = []struct{ token, name string }{
		{"Android", "Android"}, {"iPhone", "iPhone"}, {"iPad", "iPad"},
		{"Windows", "Windows"}, {"Mac OS X", "macOS"}, {"CrOS", "ChromeOS"},
		{"Linux", "Linux"},
	}
)

// deviceName describes a User-Agent briefly ("Firefox on Linux").
func deviceName(lang, ua string) string {
	find := func(list []struct{ token, name string }) string {
		for _, e := range list {
			if strings.Contains(ua, e.token) {
				return e.name
			}
		}
		return ""
	}
	b, o := find(uaBrowsers), find(uaSystems)
	switch {
	case b != "" && o != "":
		return i18n.T(lang, "session.device", "browser", b, "system", o)
	case b != "":
		return b
	case o != "":
		return o
	}
	return i18n.T(lang, "session.unknown_device")
}

// sessionsProps are the sessions page's (SessionsProps), and the
// answer of GET /api/account/sessions.
type sessionsProps struct {
	Sessions []sessionJSON `json:"sessions"`
}

// loadSessions loads the settings_sessions page.
func loadSessions(ctx context.Context, pr *pageRequest) (any, error) {
	list, err := pr.s.accountSessions(ctx, pr.sess)
	if err != nil {
		return nil, err
	}
	return sessionsProps{Sessions: list}, nil
}

// listSessions answers GET /api/account/sessions.
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sess, _ := SessionFrom(r.Context())
	list, err := s.accountSessions(r.Context(), &sess)
	if err != nil {
		s.internalError(w, "list sessions", err)
		return
	}
	writeJSON(w, http.StatusOK, sessionsProps{Sessions: list})
}

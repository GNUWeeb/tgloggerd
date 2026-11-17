// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"tgloggerd/webapi/internal/auth"
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
// its cookie, which it returns. from is the row of the session the new
// one replaces (a password change reissues the cookie), 0 for a new
// sign-in.
func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request,
	uid uint64, username, role string, epoch uint32, from uint64) (string, error) {
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

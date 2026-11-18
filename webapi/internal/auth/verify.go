// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"tgloggerd/webapi/internal/webdb"
)

// Check is the outcome of checking a request's session
// (tgweb::auth::SessionCheck).
type Check int

const (
	// Valid: the signature is good and the account's epoch matches.
	Valid Check = iota
	// None: no session cookie, or one that does not open or expired.
	None
	// Revoked: signed, but issued before the account's last epoch bump
	// (a password change), the session was ended, or the account is
	// gone.
	Revoked
	// Unverified: signed, but the epoch could not be read (a database
	// error). The cookie is not at fault and is kept.
	Unverified
)

// StateFunc reads an account's session state, with the record of the
// session whose key is given; webdb.ErrNoUser means the account is
// gone.
type StateFunc func(ctx context.Context, uid uint64, key [32]byte) (webdb.SessionState, error)

// DBState reads the state from the web's own database.
func DBState(db *sql.DB) StateFunc {
	return func(ctx context.Context, uid uint64, key [32]byte) (webdb.SessionState, error) {
		return webdb.LoadSessionState(ctx, db, uid, key)
	}
}

// Verifier decides whether a request is signed in, as verifySession in
// web/src/auth/AuthFilter.cpp does: the cookie's signature and expiry,
// then its epoch against web_users.session_epoch, so a password change
// on either service signs the account out of both, and the session's
// record in web_sessions, so a session ended by itself (signed out
// from the sessions page) ends on both as well.
//
// Unlike the Drogon filter it also refuses a disabled account, and it
// takes the role and the forced password change from the account, not
// from the cookie, so a role change or a disable applies to the very
// next request.
type Verifier struct {
	Cookies *Cookies
	State   StateFunc
}

// Verify checks the request's session. The session is returned for
// every outcome but None; when Valid, its Role and MustChangePassword
// are the account's own.
func (v *Verifier) Verify(ctx context.Context, r *http.Request) (Session, Check) {
	s, ok := v.Cookies.Current(r)
	if !ok {
		return Session{}, None
	}
	s.Key = SessionKey(RawSession(r))
	st, err := v.State(ctx, s.UID, s.Key)
	switch {
	case errors.Is(err, webdb.ErrNoUser):
		return s, Revoked
	case err != nil:
		return s, Unverified
	case st.Epoch != s.Epoch || !st.Active || st.SessionEnded:
		return s, Revoked
	}
	s.Role = st.Role
	s.MustChangePassword = st.MustChangePassword
	s.AdminOf = webdb.JoinIDs(st.AdminOf)
	s.HidePrivate = st.HidePrivate
	s.Row = st.SessionRow
	s.Stale = st.SessionStale
	return s, Valid
}

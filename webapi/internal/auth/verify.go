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
	// (a password change), or the account is gone.
	Revoked
	// Unverified: signed, but the epoch could not be read (a database
	// error). The cookie is not at fault and is kept.
	Unverified
)

// EpochFunc reads an account's session_epoch; webdb.ErrNoUser means
// the account is gone.
type EpochFunc func(ctx context.Context, uid uint64) (uint32, error)

// DBEpoch reads the epoch from the web's own database.
func DBEpoch(db *sql.DB) EpochFunc {
	return func(ctx context.Context, uid uint64) (uint32, error) {
		return webdb.SessionEpoch(ctx, db, uid)
	}
}

// Verifier decides whether a request is signed in, as verifySession in
// web/src/auth/AuthFilter.cpp does: the cookie's signature and expiry,
// then its epoch against web_users.session_epoch, so a password change
// on either service signs the account out of both.
type Verifier struct {
	Cookies *Cookies
	Epoch   EpochFunc
}

// Verify checks the request's session. The session is returned for
// every outcome but None.
func (v *Verifier) Verify(ctx context.Context, r *http.Request) (Session, Check) {
	s, ok := v.Cookies.Current(r)
	if !ok {
		return Session{}, None
	}
	epoch, err := v.Epoch(ctx, s.UID)
	switch {
	case errors.Is(err, webdb.ErrNoUser):
		return s, Revoked
	case err != nil:
		return s, Unverified
	case epoch != s.Epoch:
		return s, Revoked
	}
	return s, Valid
}

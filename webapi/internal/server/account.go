// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/webdb"
)

// The signed-in account's own operations. The contract is in
// webapi/openapi.yaml (tag account); web/docs/accounts.md explains it.

// MinPasswordLen is the shortest password accepted, in bytes, as the
// Drogon settings page counts it. There is no maximum below the body
// limit (decisions/2026-11-18-password-hashing.md).
const MinPasswordLen = 8

// maxBody bounds the body of an account operation. It is the only
// limit on a password's length.
const maxBody = 64 << 10

// decodeBody reads a JSON object body into v, answering 400
// bad_request itself when it is not one.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			map[string]any{"message": "The body must be a JSON object of the documented fields."})
		return false
	}
	return true
}

// checkCSRF answers 403 csrf itself when the session's token is
// missing or wrong.
func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	if !s.signer.CheckSessionCSRF(r, r.Header.Get(CSRFHeader)) {
		writeError(w, http.StatusForbidden, "csrf", nil)
		return false
	}
	return true
}

// changePassword answers POST /api/account/password, as the Drogon
// settings page changes a password: the current one must match, the
// new one must be long enough and different, every other session ends
// (the epoch moves on) and this one gets a new cookie, and the account
// is told at its verified address. It also ends a forced change, which
// is why it passes the password gate; the scope puts that gate before
// the email gate, so it is let through while both are up.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	var in struct {
		Current *string `json:"current_password"`
		New     *string `json:"new_password"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Current == nil || in.New == nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			map[string]any{"message": "current_password and new_password are required."})
		return
	}
	ctx := r.Context()
	ip := clientIP(r)

	// Guesses at the current password through a stolen session are
	// throttled like sign-ins.
	key := "password\n" + strconv.FormatUint(sess.UID, 10)
	if !s.limiter.Allowed(key) {
		writeError(w, http.StatusTooManyRequests, "rate_limited",
			map[string]any{"message": msgThrottle})
		return
	}

	u, err := webdb.FindByID(ctx, s.app, sess.UID)
	if errors.Is(err, webdb.ErrNoUser) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
		return
	}
	if err != nil {
		s.internalError(w, "account lookup", err)
		return
	}

	code, msg := "", ""
	switch {
	case !auth.VerifyPassword(u.PasswordHash, *in.Current):
		code, msg = "wrong_password", "Your current password is incorrect."
		s.limiter.RecordFailure(key)
	case len(*in.New) < MinPasswordLen:
		code, msg = "weak_password", "The new password must be at least 8 characters."
	case *in.New == *in.Current:
		code, msg = "weak_password", "The new password must be different from the current one."
	}
	if code != "" {
		if err := webdb.Audit(ctx, s.app, &u.ID, "password_change_fail", ip, u.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		writeError(w, http.StatusBadRequest, code, map[string]any{"message": msg})
		return
	}
	s.limiter.Reset(key)

	hash, err := auth.HashPassword(*in.New)
	if err != nil {
		s.internalError(w, "hash", err)
		return
	}
	epoch, err := webdb.SetOwnPassword(ctx, s.app, u.ID, hash)
	if err != nil {
		s.internalError(w, "set password", err)
		return
	}
	if err := webdb.Audit(ctx, s.app, &u.ID, "password_change", ip, u.Username); err != nil {
		s.internalError(w, "audit", err)
		return
	}
	raw, err := s.startSession(ctx, w, r, u.ID, u.Username, u.Role, epoch, &sess.Key)
	if err != nil {
		s.internalError(w, "start session", err)
		return
	}
	s.noticeByID(ctx, u.ID, "password")
	writeJSON(w, http.StatusOK, map[string]any{"csrf": s.signer.SessionCSRF(raw)})
}

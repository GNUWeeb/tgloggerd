// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package webdb reads and writes the web's own schema (tgloggerd_web,
// the "app" pool): accounts and the audit log. The statements are the
// Drogon web's (web/src/dao/Accounts.cpp and Audit.cpp), so both
// services see and leave the same rows.
package webdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// User is a web_users row.
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
	Role         string // "superadmin", "admin" or "member"
	IsActive     bool
	Epoch        uint32 // session_epoch: bumped to end every session
	// MustChangePassword: the password was set for the account (a
	// temporary one); it must choose its own before anything else.
	MustChangePassword bool
}

// ErrNoUser is returned when no account matches.
var ErrNoUser = errors.New("no such account")

const userColumns = "SELECT id, username, password_hash, role, is_active, " +
	"session_epoch, must_change_password FROM web_users "

func scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.IsActive,
		&u.Epoch, &u.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// FindByUsername looks an account up by name; the column's collation
// decides how names compare. The caller checks IsActive and the
// password.
func FindByUsername(ctx context.Context, db *sql.DB, username string) (User, error) {
	return scanUser(db.QueryRowContext(ctx, userColumns+"WHERE username = ?", username))
}

// FindByID looks an account up by id.
func FindByID(ctx context.Context, db *sql.DB, id uint64) (User, error) {
	return scanUser(db.QueryRowContext(ctx, userColumns+"WHERE id = ?", id))
}

// SessionState is what every signed-in request reads of its account:
// the cookie's epoch is checked against Epoch, and the role and flags
// come from here, not from the cookie, so a change applies at once.
type SessionState struct {
	Epoch              uint32
	Role               string
	Active             bool
	MustChangePassword bool
}

// LoadSessionState is an account's SessionState, or ErrNoUser when the
// account is gone. One primary-key lookup per request.
func LoadSessionState(ctx context.Context, db *sql.DB, id uint64) (SessionState, error) {
	var st SessionState
	err := db.QueryRowContext(ctx,
		"SELECT session_epoch, role, is_active, must_change_password "+
			"FROM web_users WHERE id = ?", id).
		Scan(&st.Epoch, &st.Role, &st.Active, &st.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionState{}, ErrNoUser
	}
	return st, err
}

// Execer is a pool or a transaction.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Audit appends a security event to web_audit. uid is nil for an
// anonymous or failed action, such as a failed login.
func Audit(ctx context.Context, db Execer, uid *uint64, action, ip, detail string) error {
	return audit(ctx, db, uid, nil, action, ip, detail)
}

// AuditAccount appends an account change to web_audit: who made it
// (actor) and the account it applied to (target).
func AuditAccount(ctx context.Context, db Execer, actor, target uint64, action, ip, detail string) error {
	return audit(ctx, db, &actor, &target, action, ip, detail)
}

func audit(ctx context.Context, db Execer, uid, target *uint64, action, ip, detail string) error {
	detail = auditDetail(detail)
	var u, t any
	if uid != nil {
		u = *uid
	}
	if target != nil {
		t = *target
	}
	_, err := db.ExecContext(ctx,
		"INSERT INTO web_audit (web_user_id, target_user_id, action, ip, detail) "+
			"VALUES (?, ?, ?, ?, ?)", u, t, action, ip, detail)
	return err
}

// MaxAuditDetail is the length of web_audit.detail, in characters.
const MaxAuditDetail = 255

// auditDetail fits detail into web_audit.detail: invalid UTF-8 is
// replaced and the text is cut to MaxAuditDetail characters, so an
// over-long value (a username typed at the login form) is recorded
// rather than failing the insert.
func auditDetail(detail string) string {
	detail = strings.ToValidUTF8(detail, "\uFFFD")
	n := 0
	for i := range detail {
		if n == MaxAuditDetail {
			return detail[:i]
		}
		n++
	}
	return detail
}

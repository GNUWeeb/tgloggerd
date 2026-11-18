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
// The Session fields are the cookie's own record (web_sessions).
type SessionState struct {
	Epoch              uint32
	Role               string
	Active             bool
	MustChangePassword bool
	// SessionRow is the session's web_sessions.id, 0 when it has no
	// row yet; SessionEnded: it was signed out; SessionStale: its last
	// use was recorded more than TouchInterval ago.
	SessionRow   uint64
	SessionEnded bool
	SessionStale bool
	// AdminOf is the federations the account is an admin of
	// (web_federation_admins), nil for none.
	AdminOf []uint64
}

// LoadSessionState is an account's SessionState with the record of
// the session whose cookie key is given, or ErrNoUser when the account
// is gone. One primary-key lookup per request, with the session's row
// by its unique key.
func LoadSessionState(ctx context.Context, db *sql.DB, id uint64, key [32]byte) (SessionState, error) {
	var (
		st      SessionState
		adminOf sql.NullString
	)
	err := db.QueryRowContext(ctx,
		"SELECT u.session_epoch, u.role, u.is_active, u.must_change_password, "+
			"COALESCE(s.id, 0), s.revoked_at IS NOT NULL, "+
			"COALESCE(s.last_used_at < NOW() - INTERVAL ? SECOND, 0), "+
			adminOfColumn+
			"FROM web_users u LEFT JOIN web_sessions s "+
			"ON s.token_hash = ? AND s.web_user_id = u.id WHERE u.id = ?",
		int(TouchInterval.Seconds()), key[:], id).
		Scan(&st.Epoch, &st.Role, &st.Active, &st.MustChangePassword,
			&st.SessionRow, &st.SessionEnded, &st.SessionStale, &adminOf)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionState{}, ErrNoUser
	}
	if err != nil {
		return SessionState{}, err
	}
	if st.AdminOf, err = idList(adminOf); err != nil {
		return SessionState{}, err
	}
	return st, nil
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

// SetOwnPassword stores a password the account chose itself: it ends
// a forced change, and bumps session_epoch in the same statement, so
// every session issued before ends on both services. It returns the
// new epoch, for the caller's new cookie.
func SetOwnPassword(ctx context.Context, db *sql.DB, id uint64, hash string) (uint32, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		"UPDATE web_users SET password_hash = ?, must_change_password = 0, "+
			"session_epoch = session_epoch + 1 WHERE id = ?", hash, id); err != nil {
		return 0, err
	}
	var epoch uint32
	if err := tx.QueryRowContext(ctx,
		"SELECT session_epoch FROM web_users WHERE id = ?", id).Scan(&epoch); err != nil {
		return 0, err
	}
	return epoch, tx.Commit()
}

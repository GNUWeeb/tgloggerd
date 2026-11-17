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
	Role         string // "admin" or "viewer"
	IsActive     bool
	Epoch        uint32 // session_epoch: bumped to end every session
}

// ErrNoUser is returned when no account matches.
var ErrNoUser = errors.New("no such account")

const userColumns = "SELECT id, username, password_hash, role, is_active, " +
	"session_epoch FROM web_users "

func scanUser(row *sql.Row) (User, error) {
	var u User
	var active int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &active, &u.Epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, err
	}
	u.IsActive = active != 0
	return u, nil
}

// FindByUsername looks an account up by name; the column's collation
// decides how names compare. The caller checks IsActive and the
// password.
func FindByUsername(ctx context.Context, db *sql.DB, username string) (User, error) {
	return scanUser(db.QueryRowContext(ctx, userColumns+"WHERE username = ?", username))
}

// SessionEpoch is an account's session_epoch, or ErrNoUser when the
// account is gone.
func SessionEpoch(ctx context.Context, db *sql.DB, id uint64) (uint32, error) {
	var epoch uint32
	err := db.QueryRowContext(ctx,
		"SELECT session_epoch FROM web_users WHERE id = ?", id).Scan(&epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoUser
	}
	return epoch, err
}

// Audit appends a security event to web_audit. uid is nil for an
// anonymous or failed action, such as a failed login.
func Audit(ctx context.Context, db *sql.DB, uid *uint64, action, ip, detail string) error {
	detail = auditDetail(detail)
	var u any
	if uid != nil {
		u = *uid
	}
	_, err := db.ExecContext(ctx,
		"INSERT INTO web_audit (web_user_id, action, ip, detail) "+
			"VALUES (?, ?, ?, ?)", u, action, ip, detail)
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

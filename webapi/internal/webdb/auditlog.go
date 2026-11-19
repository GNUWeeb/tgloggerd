// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"time"
)

// AuditEntry is a web_audit row as the administration pages list one.
type AuditEntry struct {
	By     string // the account's username, "" if none
	Detail string
	At     time.Time
}

// AuditOf is the newest rows of one action, newest first.
func AuditOf(ctx context.Context, db *sql.DB, action string, limit int) ([]AuditEntry, error) {
	return AuditOfBy(ctx, db, action, nil, limit)
}

// AuditOfBy is AuditOf narrowed to the rows of one account when by is
// not nil, read through idx_web_audit_user_action.
func AuditOfBy(ctx context.Context, db *sql.DB, action string, by *uint64, limit int) ([]AuditEntry, error) {
	where, args := "a.action = ?", []any{action}
	if by != nil {
		where += " AND a.web_user_id = ?"
		args = append(args, *by)
	}
	rows, err := db.QueryContext(ctx, "SELECT COALESCE(u.username, ''), COALESCE(a.detail, ''), "+
		"UNIX_TIMESTAMP(a.created_at) FROM web_audit a "+
		"LEFT JOIN web_users u ON u.id = a.web_user_id "+
		"WHERE "+where+" ORDER BY a.id DESC LIMIT ?", append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var at float64
		if err := rows.Scan(&e.By, &e.Detail, &at); err != nil {
			return nil, err
		}
		e.At = time.Unix(int64(at), 0).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

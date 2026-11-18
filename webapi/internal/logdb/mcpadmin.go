// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"

	"tgloggerd/webapi/internal/scope"
)

// The MCP exposure allowlist, telegram_public_groups (web/docs/mcp.md,
// migration 000022), as the Drogon dao::mcp keeps it. The web's account
// is granted INSERT and DELETE on it, not UPDATE: a row is a decision
// to expose a group, added or withdrawn, never quietly edited. Every
// read takes the scope, so a group hidden from the viewer is neither
// listed nor counted nor changed (web-accounts.md section 4).

// ExposedGroup is an exposed group the scope sees.
type ExposedGroup struct {
	Group Group
	Note  string
	Added int64 // unix seconds
	// PublicNow: the group owns an active public username. A hint
	// for the admin, never the gate.
	PublicNow bool
}

// ExposedGroups lists the exposed groups the scope sees, newest first.
func ExposedGroups(ctx context.Context, db *sql.DB, sc scope.Scope) ([]ExposedGroup, error) {
	gf, ga := sc.GroupFilter("p.group_id")
	rows, err := db.QueryContext(ctx, "SELECT p.group_id, p.note, UNIX_TIMESTAMP(p.created_at), "+
		"EXISTS (SELECT 1 FROM telegram_group_usernames gu WHERE gu.group_id = p.group_id "+
		"AND gu.kind = 'active') FROM telegram_public_groups p "+
		"JOIN telegram_groups g ON g.id = p.group_id WHERE "+gf+
		" ORDER BY p.created_at DESC, p.group_id", ga...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var (
		out []ExposedGroup
		ids []int64
	)
	for rows.Next() {
		var e ExposedGroup
		if err := rows.Scan(&e.Group.ID, &e.Note, &e.Added, &e.PublicNow); err != nil {
			return nil, err
		}
		out = append(out, e)
		ids = append(ids, e.Group.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	groups, err := groupsByID(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if g, ok := groups[out[i].Group.ID]; ok {
			out[i].Group = g
		}
	}
	if out == nil {
		out = []ExposedGroup{}
	}
	return out, nil
}

// ExposedStats counts the exposed groups the scope sees and their
// messages.
func ExposedStats(ctx context.Context, db *sql.DB, sc scope.Scope) (groups int, messages int64, err error) {
	gf, ga := sc.GroupFilter("p.group_id")
	err = db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(g.msg_count), 0) "+
		"FROM telegram_public_groups p JOIN telegram_groups g ON g.id = p.group_id WHERE "+gf,
		ga...).Scan(&groups, &messages)
	return groups, messages, err
}

// IsExposed is whether a group is on the allowlist.
func IsExposed(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegram_public_groups "+
		"WHERE group_id = ?", id).Scan(&n)
	return n > 0, err
}

// ExposeGroup puts a group the caller has checked on the allowlist;
// false when it was there already (a race between two admins, in
// which doing nothing is right). note is at most 255 characters.
func ExposeGroup(ctx context.Context, db *sql.DB, id int64, note string, by uint64) (bool, error) {
	res, err := db.ExecContext(ctx, "INSERT IGNORE INTO telegram_public_groups "+
		"(group_id, note, added_by) VALUES (?, ?, ?)", id, note, by)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// UnexposeGroup takes a group off the allowlist; false when it was not
// on it.
func UnexposeGroup(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	res, err := db.ExecContext(ctx, "DELETE FROM telegram_public_groups WHERE group_id = ?", id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

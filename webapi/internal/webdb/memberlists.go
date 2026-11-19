// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
)

// Hidden member lists (web migration 000013): channels whose member
// list a superadmin hid, which the web lists for superadmins only.

// MemberListHidden is whether the group's member list is hidden.
func MemberListHidden(ctx context.Context, db *sql.DB, groupID int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM web_hidden_member_lists "+
		"WHERE group_id = ?", groupID).Scan(&n)
	return n > 0, err
}

// SetMemberListHidden hides the group's member list (by superadmin
// by) or shows it again. changed is false when it already was so.
func SetMemberListHidden(ctx context.Context, db *sql.DB, groupID int64, hidden bool,
	by uint64) (changed bool, err error) {
	var res sql.Result
	if hidden {
		res, err = db.ExecContext(ctx, "INSERT IGNORE INTO web_hidden_member_lists "+
			"(group_id, hidden_by) VALUES (?, ?)", groupID, by)
	} else {
		res, err = db.ExecContext(ctx, "DELETE FROM web_hidden_member_lists WHERE group_id = ?",
			groupID)
	}
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

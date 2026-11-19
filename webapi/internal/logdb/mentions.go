// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strings"
)

// UsernameOwners maps the usernames among names (without the leading
// @) to the users that hold them as an active username now
// (telegram_user_usernames), for the @username mentions of a page of
// messages; one query for all of them. Keys are lowercased: Telegram
// usernames are case-insensitive, as the table's collation compares
// them. Released, disabled and unknown ones are left out. It does not
// check the scope: the caller passes the owners through
// VisibleUserIDs.
func UsernameOwners(ctx context.Context, db *sql.DB, names []string) (map[string]int64, error) {
	out := map[string]int64{}
	seen := map[string]bool{}
	var args []any
	for _, n := range names {
		k := strings.ToLower(n)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		args = append(args, k)
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT username, user_id FROM telegram_user_usernames "+
		"WHERE username IN ("+placeholders(len(args))+") AND kind = 'active' "+
		"AND user_id IS NOT NULL", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name string
			id   int64
		)
		if err := rows.Scan(&name, &id); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = id
	}
	return out, rows.Err()
}

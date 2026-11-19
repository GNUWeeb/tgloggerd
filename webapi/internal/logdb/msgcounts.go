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

// GroupMsgCounts is the stored message counter (telegram_groups
// .msg_count) of each of the groups ids the logger has a row for.
// The callers pass ids they hold themselves, not a viewer's request,
// so no scope applies.
func GroupMsgCounts(ctx context.Context, db *sql.DB, ids []int64) (map[int64]uint64, error) {
	out := map[int64]uint64{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, "SELECT id, msg_count FROM telegram_groups WHERE id IN ("+
		strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n uint64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

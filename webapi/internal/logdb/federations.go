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

// The logger's side of the federations: which groups exist. The
// federations themselves live in the web's own schema, which hands
// their groups over as a GroupSet.

// MissingGroups is the ids among ids that are no group the scope sees,
// in the order given; nil when every one is.
func MissingGroups(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	in, inArgs := GroupSet{IDs: ids}.Cond("id")
	cond, args := sc.GroupFilter("id")
	rows, err := db.QueryContext(ctx, "SELECT id FROM telegram_groups WHERE "+in+" AND "+cond,
		append(inArgs, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []int64
	for _, id := range ids {
		if !found[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

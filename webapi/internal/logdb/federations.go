// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strconv"

	"tgloggerd/webapi/internal/scope"
)

// The logger's side of the federations: which groups exist, how many
// of a federation's the scope sees, and a preview of them. The
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

// CountGroups is how many groups of the set the scope sees.
func CountGroups(ctx context.Context, db *sql.DB, sc scope.Scope, g GroupSet) (int, error) {
	in, inArgs := g.Cond("id")
	cond, args := sc.GroupFilter("id")
	var n int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegram_groups WHERE "+in+" AND "+cond,
		append(inArgs, args...)...).Scan(&n)
	return n, err
}

// PreviewGroups is at most limit groups of the set that the scope
// sees, busiest first, as a federation's card shows them.
func PreviewGroups(ctx context.Context, db *sql.DB, sc scope.Scope, g GroupSet, limit int) ([]Group, error) {
	in, inArgs := g.Cond("g.id")
	cond, args := sc.GroupFilter("g.id")
	rows, err := db.QueryContext(ctx, groupQuery+"WHERE "+in+" AND "+cond+
		" ORDER BY g.msg_count DESC, g.id DESC LIMIT "+strconv.Itoa(limit),
		append(inArgs, args...)...)
	if err != nil {
		return nil, err
	}
	gs, err := scanGroups(rows)
	if gs == nil {
		gs = []Group{}
	}
	return gs, err
}

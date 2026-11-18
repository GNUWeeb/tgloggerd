// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strings"

	"tgloggerd/webapi/internal/scope"
)

// PinnedGroup is a pinned group as the menu and the preferences page
// show it.
type PinnedGroup struct {
	ID      int64
	Title   string
	PhotoID uint64 // telegram_files.id of its photo, 0 if none
}

// PinnedGroups are the groups of ids, in that order, that exist and
// that the scope sees; the others are left out, so a pin of a group
// the viewer may no longer see shows nowhere.
func PinnedGroups(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) ([]PinnedGroup, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cond, args := sc.GroupFilter("id")
	q := "SELECT id, title, COALESCE(photo_file_id, 0) FROM telegram_groups WHERE id IN (?" +
		strings.Repeat(", ?", len(ids)-1) + ") AND " + cond
	all := make([]any, 0, len(ids)+len(args))
	for _, id := range ids {
		all = append(all, id)
	}
	all = append(all, args...)
	rows, err := db.QueryContext(ctx, q, all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[int64]PinnedGroup{}
	for rows.Next() {
		var g PinnedGroup
		if err := rows.Scan(&g.ID, &g.Title, &g.PhotoID); err != nil {
			return nil, err
		}
		found[g.ID] = g
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []PinnedGroup{}
	for _, id := range ids {
		if g, ok := found[id]; ok {
			out = append(out, g)
		}
	}
	return out, nil
}

// GroupVisible is whether a group exists and the scope sees it.
func GroupVisible(ctx context.Context, db *sql.DB, sc scope.Scope, id int64) (bool, error) {
	g, err := PinnedGroups(ctx, db, sc, []int64{id})
	return len(g) == 1, err
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

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

// FedGroupKey is where a page of a federation's groups ends: its last
// group's message count and id, in the order busiest first, then by
// id descending.
type FedGroupKey struct {
	MsgCount uint64
	ID       int64
}

// FedGroupQuery is a page of a federation's groups: those of Set the
// scope sees whose title or active username contains Q (without
// regard to case; a leading @ is dropped), after the key After (nil
// for the first page), at most Limit of them.
type FedGroupQuery struct {
	Set   GroupSet
	Q     string
	After *FedGroupKey
	Limit int
}

// fedGroupCond is the condition of q's groups, without the page's key,
// over the groups g.
func fedGroupCond(sc scope.Scope, q FedGroupQuery) (string, []any) {
	in, args := q.Set.Cond("g.id")
	cond, cargs := sc.GroupFilter("g.id")
	where := in + " AND " + cond
	args = append(args, cargs...)
	if t := strings.TrimPrefix(strings.TrimSpace(q.Q), "@"); t != "" {
		where += " AND (g.title LIKE ? OR EXISTS (SELECT 1 FROM telegram_group_usernames x " +
			"WHERE x.group_id = g.id AND x.kind = 'active' AND x.username LIKE ?))"
		args = append(args, likeContains(t), likeContains(t))
	}
	return where, args
}

// FederationGroupPage is a page of a federation's groups and how many
// groups match the query in all (every page's). It returns one group
// more than Limit when more follow.
func FederationGroupPage(ctx context.Context, db *sql.DB, sc scope.Scope, q FedGroupQuery) ([]Group, int, error) {
	where, args := fedGroupCond(sc, q)
	var total int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegram_groups g WHERE "+where,
		args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if q.After != nil {
		where += " AND (g.msg_count < ? OR (g.msg_count = ? AND g.id < ?))"
		args = append(args, q.After.MsgCount, q.After.MsgCount, q.After.ID)
	}
	rows, err := db.QueryContext(ctx, groupQuery+"WHERE "+where+
		" ORDER BY g.msg_count DESC, g.id DESC LIMIT "+strconv.Itoa(q.Limit+1), args...)
	if err != nil {
		return nil, 0, err
	}
	gs, err := scanGroups(rows)
	if gs == nil {
		gs = []Group{}
	}
	return gs, total, err
}

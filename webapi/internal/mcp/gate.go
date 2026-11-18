// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"context"
	"strings"
)

// THE EXPOSURE GATE.
//
// Nothing is readable through MCP until an administrator adds the group
// to telegram_public_groups, a curated list; whether a group has a
// public username is never consulted (migrations/000022 says why).
// Where it is enforced, so the whole set is in one place:
//
//   - Every tool that returns MESSAGES goes through msgGate: search,
//     recent, get_messages, message history, popular words.
//   - The tools that return a GROUP or a per-group aggregate carry the
//     list as a condition in their own statement (groupGate, or a
//     group_id IN (SELECT group_id FROM telegram_public_groups) on the
//     table's group column): list_groups, get_group,
//     get_group_history, list_group_admins, list_group_senders,
//     count_user_messages.
//   - The user tools (get_users, get_user, get_user_history) are not
//     gated: they describe accounts, not conversations, as in the
//     Drogon web.
//
// Direct messages are out of reach: no file of this package names their
// table, which a grep for its name checks (mcp_test.go).
//
// The gate binds the allowed ids as constants rather than a semi-join
// on the list. With the subquery MySQL drives the join from the list
// and never picks the (chat_id, <timestamp>) indexes; with constants an
// ordered scan is a reverse range scan of one index (web/docs/mcp.md,
// Performance notes). The list is curated, so it is small. An empty
// list compiles to 1=0, which matches nothing.

// groupGate is the gate for a statement whose telegram_groups alias is
// g.
const groupGate = " AND g.id IN (SELECT group_id FROM telegram_public_groups)"

type gate struct {
	sql     string // " AND m.chat_id IN (?,?)" or " AND 1=0"
	binds   []any
	nothing bool // nothing readable: the statement can be skipped
}

// exposedGroups lists the curated groups.
func (t *telegram) exposedGroups(ctx context.Context) ([]int64, error) {
	rows, err := execTool(ctx, t.db, "SELECT group_id FROM telegram_public_groups", nil, queryFailed)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.i64("group_id"))
	}
	return out, nil
}

// msgGate builds the gate over m.chat_id, narrowed to args[key] when
// that is an integer. A named group that is not exposed gives a gate
// that matches nothing rather than an error: refusing would confirm the
// group exists.
func (t *telegram) msgGate(ctx context.Context, args map[string]any) (gate, error) {
	groups, err := t.exposedGroups(ctx)
	if err != nil {
		return gate{}, err
	}
	if want, ok := intValue(args["group_id"]); ok {
		found := false
		for _, g := range groups {
			if g == want {
				found = true
				break
			}
		}
		groups = groups[:0]
		if found {
			groups = append(groups, want)
		}
	}
	if len(groups) == 0 {
		return gate{sql: " AND 1=0", nothing: true}, nil
	}
	g := gate{sql: " AND m.chat_id IN (" + strings.Repeat("?,", len(groups)-1) + "?)"}
	for _, id := range groups {
		g.binds = append(g.binds, id)
	}
	return g, nil
}

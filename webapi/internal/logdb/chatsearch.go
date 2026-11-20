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

// The chat search of the pickers.

// ChatPick is a chat as the pickers list it.
type ChatPick struct {
	ID       int64
	Kind     string // "group" or "user"
	Title    string
	Type     string // a group's type; "" for a user
	Username string
	Photo    uint64 // 0 = none
}

// ChatSearch is a chat search: q matched against titles, names and
// usernames (and an exact id), kinds "group", "user" or both (""),
// Groups the groups of one federation only (nil for all).
type ChatSearch struct {
	Q      string
	Kind   string
	Groups *GroupSet
	Limit  int
	Offset int
}

// GroupSet is a set of groups by chat id, as the web's own schema
// gives it (the logger's schema knows no federations): the groups
// listed, or with Not every group but those, which is how the Default
// federation's derived groups are named (every group in no other
// federation).
type GroupSet struct {
	IDs []int64
	Not bool
}

// Cond is the condition, with its arguments, that keeps the rows of
// the set's groups; column is the query's own name for the group's
// chat id, a constant of the caller's.
func (g GroupSet) Cond(column string) (string, []any) {
	if len(g.IDs) == 0 {
		if g.Not {
			return "1 = 1", nil
		}
		return "1 = 0", nil
	}
	args := make([]any, len(g.IDs))
	for i, id := range g.IDs {
		args[i] = id
	}
	op := " IN ("
	if g.Not {
		op = " NOT IN ("
	}
	return column + op + placeholders(len(g.IDs)) + ")", args
}

// SearchChats lists the groups and private chats the scope sees that
// match, busiest first: groups by their message count, private chats
// by theirs. It returns one row more than Limit when more follow.
//
// Private chats are only for a scope that sees them. Groups limits the
// answer to those groups (a federation's), and so to groups only.
func SearchChats(ctx context.Context, db *sql.DB, sc scope.Scope, s ChatSearch) ([]ChatPick, error) {
	q := strings.TrimPrefix(strings.TrimSpace(s.Q), "@")
	wantGroups := s.Kind == "" || s.Kind == "group"
	wantUsers := (s.Kind == "" || s.Kind == "user") && sc.SeesPrivate() && s.Groups == nil
	var (
		parts []string
		args  []any
	)
	if wantGroups {
		gf, ga := sc.GroupFilter("g.id")
		part := "SELECT g.id, 'group' AS kind, g.title, g.type, g.photo_file_id AS photo, " +
			"g.msg_count AS n, (SELECT gu.username FROM telegram_group_usernames gu " +
			"WHERE gu.group_id = g.id AND gu.kind='active' ORDER BY gu.position LIMIT 1) " +
			"AS username FROM `telegram_groups` g WHERE " + gf
		args = append(args, ga...)
		if s.Groups != nil {
			fc, fa := s.Groups.Cond("g.id")
			part += " AND " + fc
			args = append(args, fa...)
		}
		if q != "" {
			part += " AND (g.title LIKE ? OR EXISTS (SELECT 1 FROM telegram_group_usernames x " +
				"WHERE x.group_id = g.id AND x.kind='active' AND x.username LIKE ?)"
			args = append(args, likeContains(q), likeContains(q))
			if isIntStr(q) {
				part += " OR g.id = ?"
				args = append(args, q)
			}
			part += ")"
		}
		parts = append(parts, part)
	}
	if wantUsers {
		part := "SELECT u.id, 'user' AS kind, " +
			"TRIM(CONCAT_WS(' ', u.first_name, u.last_name)) AS title, '' AS type, " +
			"u.profile_photo_file_id AS photo, pc.n, (SELECT un.username " +
			"FROM telegram_user_usernames un WHERE un.user_id = u.id AND un.kind='active' " +
			"ORDER BY un.position LIMIT 1) AS username " +
			"FROM (SELECT chat_id, COUNT(*) AS n FROM telegram_private_messages " +
			"GROUP BY chat_id) pc JOIN telegram_users u ON u.id = pc.chat_id"
		if q != "" {
			part += " WHERE (CONCAT_WS(' ', u.first_name, u.last_name, " +
				"COALESCE(u.contact_first_name,''), COALESCE(u.contact_last_name,'')) LIKE ? " +
				"OR EXISTS (SELECT 1 FROM telegram_user_usernames x WHERE x.user_id = u.id " +
				"AND x.kind='active' AND x.username LIKE ?)"
			args = append(args, likeContains(q), likeContains(q))
			if isIntStr(q) {
				part += " OR u.id = ?"
				args = append(args, q)
			}
			part += ")"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	sqlText := "SELECT /*+ MAX_EXECUTION_TIME(" + strconv.Itoa(statementMS) + ") */ * FROM ((" +
		strings.Join(parts, ") UNION ALL (") + ")) c ORDER BY c.n DESC, c.kind ASC, c.id DESC " +
		"LIMIT " + strconv.Itoa(s.Limit+1) + " OFFSET " + strconv.Itoa(s.Offset)
	rs, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		if isTimeout(err) {
			return nil, ErrSearchTimeout
		}
		return nil, err
	}
	rows, err := scanRows(rs)
	if err != nil {
		return nil, err
	}
	out := make([]ChatPick, 0, len(rows))
	for _, r := range rows {
		p := ChatPick{ID: r.Int("id"), Kind: r.Str("kind"), Type: r.Str("type"),
			Username: r.Str("username"), Photo: r.Uint("photo")}
		if p.Kind == "group" {
			p.Title = GroupTitle(r.Str("title"))
		} else {
			p.Title = DisplayName(r.Str("title"), "")
		}
		out = append(out, p)
	}
	return out, nil
}

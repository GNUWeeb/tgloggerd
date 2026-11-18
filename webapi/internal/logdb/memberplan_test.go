// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"database/sql"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/testdb"
)

// planOf is the plan of q (EXPLAIN): each row's table alias, the
// index it uses ("" for none, "range checked" for an index chosen per
// outer row) and its select type, in plan order.
func planOf(t *testing.T, db *sql.DB, q string, args ...any) [][3]string {
	t.Helper()
	rows, err := db.Query("EXPLAIN FORMAT=TRADITIONAL "+q, args...)
	if err != nil {
		t.Fatalf("EXPLAIN %s: %v", q, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out [][3]string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		var table, key, extra, kind string
		for i, c := range cols {
			switch c {
			case "table":
				table = vals[i].String
			case "key":
				key = vals[i].String
			case "Extra":
				extra = vals[i].String
			case "select_type":
				kind = vals[i].String
			}
		}
		// A dependent subquery whose index the server picks for each
		// outer row (the user's id, then the IN list of chats) reads
		// by index too: name it so.
		if key == "" && strings.Contains(extra, "Range checked for each record") {
			key = "range checked"
		}
		out = append(out, [3]string{table, key, kind})
	}
	return out
}

// keyOf is the index the plan uses for the table aliased alias.
func keyOf(plan [][3]string, alias string) (string, bool) {
	for _, r := range plan {
		if r[0] == alias {
			return r[1], true
		}
	}
	return "", false
}

// A member's filters read only its groups' rows through the chat_id
// and (sender_user_id, chat_id) indexes, not by scanning the messages
// table (docs/todo/web-member-access.md, design notes). The archive
// is filled with enough messages of a group the member does not see
// for the optimizer to tell the plans apart.
func TestMemberFilterPlans(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample").DB
	// 6000 messages in GNU/Weeb and 200 in Rust, from the sample's
	// people, a day apart by the second.
	if _, err := db.Exec("INSERT INTO telegram_group_messages (chat_id, message_id, " +
		"sender_user_id, date, content_type, text) " +
		"WITH RECURSIVE d (x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM d WHERE x < 99), " +
		"n (i) AS (SELECT a.x * 100 + b.x + 1 FROM d a, d b) " +
		"SELECT IF(i <= 6000, -1001000000001, -1001000000003), (900000 + i) << 20, " +
		"9299000001 + (i % 8), 1794000000 + i, 'text', 'filler' FROM n WHERE i <= 6200"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("ANALYZE TABLE telegram_group_messages, telegram_group_admins, " +
		"telegram_users, telegram_files"); err != nil {
		t.Fatal(err)
	}
	member := scope.For(scope.Viewer{ID: 5, Role: scope.Member,
		GrantedGroups: []int64{-1001000000002, -1001000000003}})
	chatKey := func(k string) bool {
		return strings.HasPrefix(k, "idx_group_messages_chat") || k == "uq_group_messages_chat_msg" ||
			k == "idx_group_messages_album"
	}
	senderKey := func(k string) bool {
		return k == "idx_group_messages_sender_chat" || k == "idx_group_messages_sender_user_id" ||
			k == "idx_group_messages_sender_date" || k == "range checked"
	}

	// The group messages list and its total.
	b, err := GroupMessages.build(member, SearchRequest{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]string{"page": b.pageSQL, "count": b.countSQL} {
		plan := planOf(t, db, q, b.pageArgs()...)
		t.Logf("group messages %s: %v", name, plan)
		if k, _ := keyOf(plan, "m"); !chatKey(k) {
			t.Errorf("group messages %s reads m by %q, not a chat_id index: %v", name, k, plan)
		}
	}

	// The users list: the user filter's two subqueries by their
	// indexes, and a member's totals by (sender_user_id, chat_id).
	b, err = Users.build(member, SearchRequest{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	plan := planOf(t, db, b.pageSQL, b.pageArgs()...)
	t.Logf("users page: %v", plan)
	if k, ok := keyOf(plan, "uf_m"); !ok || !senderKey(k) && !chatKey(k) {
		t.Errorf("user filter reads messages by %q: %v", k, plan)
	}
	if k, ok := keyOf(plan, "uf_a"); !ok || k == "" {
		t.Errorf("user filter reads admins by %q: %v", k, plan)
	}
	if k, ok := keyOf(plan, "gm"); !ok || !senderKey(k) && !chatKey(k) {
		t.Errorf("a member's totals read messages by %q: %v", k, plan)
	}

	// One user's check (VisibleUserIDs): a lookup per id.
	mcond, margs := member.GroupFilter("m.chat_id")
	acond, aargs := member.GroupFilter("a.group_id")
	plan = planOf(t, db, "SELECT DISTINCT m.sender_user_id FROM telegram_group_messages m "+
		"WHERE m.sender_user_id IN (?) AND "+mcond+" UNION SELECT a.user_id FROM telegram_group_admins a "+
		"WHERE a.user_id IN (?) AND "+acond,
		append(append(append([]any{int64(9299000001)}, margs...), int64(9299000001)), aargs...)...)
	t.Logf("user check: %v", plan)
	if k, ok := keyOf(plan, "m"); !ok || k != "idx_group_messages_sender_chat" && k != "idx_group_messages_chat_sender" {
		t.Errorf("user check reads messages by %q: %v", k, plan)
	}
	if k, ok := keyOf(plan, "a"); !ok || k == "" {
		t.Errorf("user check reads admins by %q: %v", k, plan)
	}

	// The files list: the messages a file came in, by file and chat.
	b, err = Files.build(member, SearchRequest{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	plan = planOf(t, db, b.pageSQL, b.pageArgs()...)
	t.Logf("files page: %v", plan)
	if k, ok := keyOf(plan, "fm"); !ok || k == "" {
		t.Errorf("files filter reads messages by %q: %v", k, plan)
	}
}

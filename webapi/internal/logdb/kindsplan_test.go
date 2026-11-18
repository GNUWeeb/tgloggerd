// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"strings"
	"testing"

	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/testdb"
)

// The searches by tdlib_type and service_type, which have no index of
// their own, and a forum topic's chat window read one chat's rows
// through a chat_id index, never the whole messages table
// (docs/todo/web-message-types.md). The archive is filled with
// thousands of service messages of other chats, so a plan that scanned
// the table would show.
func TestKindFilterPlans(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample", "kinds").DB
	if _, err := db.Exec("INSERT INTO telegram_group_messages (chat_id, message_id, " +
		"sender_user_id, date, content_type, text, service_type, tdlib_type) " +
		"WITH RECURSIVE d (x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM d WHERE x < 99), " +
		"n (i) AS (SELECT a.x * 100 + b.x + 1 FROM d a, d b) " +
		"SELECT -1001000000001, 900000 + i, 9299000001 + (i % 8), 1794000000 + i, 'service', " +
		"'pinned a message', 'pin_message', 'messagePinMessage' FROM n WHERE i <= 6000"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO telegram_private_messages (chat_id, message_id, " +
		"sender_id, is_outgoing, date, content_type, text, service_type, tdlib_type) " +
		"WITH RECURSIVE d (x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM d WHERE x < 99), " +
		"n (i) AS (SELECT a.x * 100 + b.x + 1 FROM d a, d b) " +
		"SELECT 9299000002 + (i % 6), 900000 + i, 9299000002 + (i % 6), 0, 1794000000 + i, " +
		"'service', 'took a screenshot', 'screenshot_taken', 'messageScreenshotTaken' " +
		"FROM n WHERE i <= 6000"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("ANALYZE TABLE telegram_group_messages, telegram_private_messages"); err != nil {
		t.Fatal(err)
	}
	chatKey := func(k string) bool {
		return strings.HasPrefix(k, "idx_group_messages_chat") || k == "uq_group_messages_chat_msg" ||
			k == "idx_group_messages_album" || k == "uq_private_messages_chat_msg" ||
			k == "idx_private_messages_album"
	}
	all := scope.For(scope.Viewer{ID: 1, Role: scope.Superadmin})
	for _, c := range []struct {
		e     *Entity
		conds []Cond
	}{
		{GroupMessages, []Cond{{C: "chat_id", O: "=", V: "-1001000000040", N: "AND", HasV: true},
			{C: "service_type", O: "=", V: "pin_message", HasV: true}}},
		{GroupMessages, []Cond{{C: "tdlib_type", O: "!=", V: "messageText", N: "AND", HasV: true},
			{C: "chat_id", O: "=", V: "-1001000000040", HasV: true}}},
		{PrivateMessages, []Cond{{C: "chat_id", O: "=", V: "9299000001", N: "AND", HasV: true},
			{C: "service_type", O: "=", V: "screenshot_taken", HasV: true}}},
	} {
		b, err := c.e.build(all, SearchRequest{Conds: c.conds, Limit: 50})
		if err != nil {
			t.Fatalf("%s %v: %v", c.e.Name, c.conds, err)
		}
		for name, q := range map[string]string{"page": b.pageSQL, "count": b.countSQL} {
			plan := planOf(t, db, q, b.pageArgs()...)
			t.Logf("%s %s: %v", c.e.Name, name, plan)
			if k, _ := keyOf(plan, "m"); !chatKey(k) {
				t.Errorf("%s %s reads m by %q, not a chat_id index: %v", c.e.Name, name, k, plan)
			}
		}
	}

	// Without a chat_id = condition every row must meet, they are
	// refused.
	for _, conds := range [][]Cond{
		{{C: "service_type", O: "=", V: "pin_message", HasV: true}},
		{{C: "chat_id", O: "=", V: "-1001000000040", N: "OR", HasV: true},
			{C: "service_type", O: "=", V: "pin_message", HasV: true}},
		{{C: "chat_id", O: ">", V: "-1001000000040", N: "AND", HasV: true},
			{C: "tdlib_type", O: "=", V: "messagePoll", HasV: true}},
	} {
		if _, err := GroupMessages.build(all, SearchRequest{Conds: conds}); err == nil ||
			!strings.Contains(err.Error(), "requires a 'chat_id =' condition") {
			t.Errorf("%v: %v", conds, err)
		}
	}
	// A chat_id = condition in another chain bounds them too.
	if _, err := GroupMessages.build(all, SearchRequest{
		Conds: []Cond{{C: "service_type", O: "=", V: "pin_message", N: "OR", HasV: true},
			{C: "tdlib_type", O: "=", V: "messagePoll", HasV: true}},
		And: [][]Cond{{{C: "chat_id", O: "=", V: "-1001000000040", HasV: true}}}}); err != nil {
		t.Errorf("chat in another chain: %v", err)
	}

	// A forum topic's window and its edges.
	c := Chat{Group: true, ID: -1001000000040, Topic: 5}
	tc, targs := c.topicCond("m.")
	q := tablesOf(true).messageSelect() + "WHERE m.chat_id = ? " + tc + "ORDER BY m.message_id DESC LIMIT 60"
	plan := planOf(t, db, q, append([]any{c.ID}, targs...)...)
	t.Logf("topic window: %v", plan)
	if k, _ := keyOf(plan, "m"); !chatKey(k) {
		t.Errorf("topic window reads m by %q: %v", k, plan)
	}
	tc, targs = c.topicCond("")
	plan = planOf(t, db, "SELECT 1 FROM telegram_group_messages WHERE chat_id = ? "+tc+
		"AND message_id < ? LIMIT 1", append(append([]any{c.ID}, targs...), 700003)...)
	if k, _ := keyOf(plan, "telegram_group_messages"); !chatKey(k) {
		t.Errorf("topic edge reads by %q: %v", k, plan)
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"strings"
	"testing"
)

// The message page carries what the chat shows beside a message and the
// page has no other source for: its link preview card, and its forum
// topic's name and colour (the chat page's topic list).

// pageMessage is the message page's message as role sees it.
func pageMessage(t *testing.T, e *adminEnv, role, scope string, row int) map[string]any {
	t.Helper()
	b := e.want(200, role, "GET", fmt.Sprintf("/api/page/messages/%s/%d", scope, row), nil)
	return b["props"].(map[string]any)["message"].(map[string]any)
}

// The link preview of Paging Lab's message 110 (the "chats" fixture),
// as the chat window gives it, and none for a message without one.
func TestMessagePageLinkPreview(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "chats")
	m := pageMessage(t, e, "superadmin", "group", 1110)
	p, ok := m["link_preview"].(map[string]any)
	if !ok || p["site"] != "Example" || p["title"] != "Kernel notes" ||
		p["desc"] != "Notes on the kernel." || p["url"] != "https://example.org/kernel" {
		t.Fatalf("link preview %v", m["link_preview"])
	}
	md, _ := p["media"].(map[string]any)
	if u, _ := md["url"].(string); md["stored"] != true || !strings.HasPrefix(u, "/files/") {
		t.Errorf("link preview media %v", p["media"])
	}
	// The same card as the chat window's.
	b := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?around=%d&limit=3",
		pagingLab, labMsg(110)), nil)
	found := false
	for _, w := range asList(b["messages"]) {
		if w := w.(map[string]any); num(w["id"]) == 1110 {
			found = true
			if js(w["link_preview"]) != js(m["link_preview"]) {
				t.Errorf("chat %s, page %s", js(w["link_preview"]), js(m["link_preview"]))
			}
		}
	}
	if !found {
		t.Errorf("row 1110 not in the chat window %v", b)
	}
	// None: the key is there, null.
	m = pageMessage(t, e, "superadmin", "group", 1112)
	if v, ok := m["link_preview"]; !ok || v != nil {
		t.Errorf("no preview: %v %v", v, ok)
	}
	if v, ok := m["topic"]; !ok || v != nil {
		t.Errorf("no topic: %v %v", v, ok)
	}
}

// The topic of Kinds Lab's forum messages (the "kinds" fixture): in
// Releases, in General (hidden), in Off-topic (closed), none for a
// thread's, and none for a topic never seen even when another group
// has one of that id.
func TestMessagePageTopic(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	for _, q := range []string{
		"UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 9 WHERE id = 4016",
		"UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 77 WHERE id = 4017",
		"INSERT INTO telegram_forum_topics (chat_id, topic_id, name, icon_color, creation_date, " +
			"creator_user_id, is_general, is_closed, is_hidden, is_pinned) VALUES " +
			"(-1001000000004, 77, 'Elsewhere', 1, 1794900000, 9299000001, 0, 0, 0, 0)",
	} {
		if _, err := e.ro.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		role  string
		row   int
		topic string
	}{
		{"superadmin", 4001, `{"closed":false,"hidden":false,"icon_color":7322096,"id":5,"name":"Releases"}`},
		{"member", 4002, `{"closed":false,"hidden":false,"icon_color":7322096,"id":5,"name":"Releases"}`},
		{"superadmin", 4006, `{"closed":false,"hidden":true,"icon_color":0,"id":1,"name":"General"}`},
		{"superadmin", 4016, `{"closed":true,"hidden":false,"icon_color":16749490,"id":9,"name":"Off-topic"}`},
		{"superadmin", 4007, `null`},
		{"superadmin", 4021, `null`},
	} {
		m := pageMessage(t, e, c.role, "group", c.row)
		if got := js(m["topic"]); got != c.topic {
			t.Errorf("%s row %d: topic %s, want %s", c.role, c.row, got, c.topic)
		}
		if c.topic != "null" && num(m["topic_id"]) != num(m["topic"].(map[string]any)["id"]) {
			t.Errorf("row %d: topic_id %v", c.row, m["topic_id"])
		}
	}
	// A topic never seen: the id stays, the topic is null.
	m := pageMessage(t, e, "superadmin", "group", 4017)
	if num(m["topic_id"]) != 77 || m["topic"] != nil {
		t.Errorf("unseen topic: %v %v", m["topic_id"], m["topic"])
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"net/url"
	"testing"
)

// A deletion the daemon found on a recheck against Telegram carries
// meta.deleted_on_recheck; every place a message's deletion is sent
// gives it as deleted_on_recheck: true, and leaves the key out for a
// deletion seen live and for a message not deleted (a stray flag
// counts only with deleted_at). Over the "chats" fixture.
func TestDeletedOnRecheck(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	for _, q := range []string{
		// 1020 (deleted in the fixture) found so; 1030 has the flag but
		// is not deleted. Album 7001 (1040-1042) is deleted whole, 1041
		// found on recheck. Rina's 1003 found on recheck.
		`UPDATE telegram_group_messages SET meta = '{"deleted_on_recheck": true}',
			deleted_at = '2026-11-17 15:59:00' WHERE id = 1020`,
		`UPDATE telegram_group_messages SET meta = '{"deleted_on_recheck": true}' WHERE id = 1030`,
		`UPDATE telegram_group_messages SET deleted_at = '2026-11-17 13:00:00'
			WHERE id IN (1040, 1041, 1042)`,
		`UPDATE telegram_group_messages SET meta = '{"effect_id": "1", "deleted_on_recheck": true}'
			WHERE id = 1041`,
		// A false flag is no flag.
		`UPDATE telegram_group_messages SET meta = '{"deleted_on_recheck": false}',
			deleted_at = '2026-11-17 13:00:00' WHERE id = 1050`,
		`UPDATE telegram_private_messages SET meta = '{"deleted_on_recheck": true}',
			deleted_at = '2026-11-17 13:00:00' WHERE id = 1003`,
	} {
		if _, err := e.roExec(q); err != nil {
			t.Fatal(err)
		}
	}
	admin := e.as("admin")
	get := func(path string) map[string]any {
		t.Helper()
		var b map[string]any
		e.decode(path, admin, 200, &b)
		return b
	}
	flag := func(m map[string]any) any { return m["deleted_on_recheck"] }

	// The chat window.
	b := get(fmt.Sprintf("/api/chats/group/%d/messages?after=%d&limit=60", pagingLab, labMsg(1)))
	byID := map[int64]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		byID[num(m["id"])] = m
	}
	if m := byID[1020]; m == nil || m["deleted_at"] == nil || flag(m) != true {
		t.Errorf("1020: %v", m)
	}
	for _, id := range []int64{1030, 1050, 1010} {
		if m := byID[id]; m == nil {
			t.Errorf("%d missing", id)
		} else if _, ok := m["deleted_on_recheck"]; ok {
			t.Errorf("%d has the flag: %v", id, flag(m))
		}
	}
	album := byID[1040]
	if album == nil || album["deleted_at"] == nil || flag(album) != true {
		t.Fatalf("album: %v", album)
	}
	if md := album["media"].(map[string]any); md["deleted_at"] == nil || md["deleted_on_recheck"] != nil {
		t.Errorf("album item 1040: %v", md)
	}
	items := asList(album["album"])
	if len(items) != 2 || items[0].(map[string]any)["deleted_on_recheck"] != true ||
		items[1].(map[string]any)["deleted_on_recheck"] != nil {
		t.Errorf("album items: %s", js(items))
	}

	// A private chat.
	b = get(fmt.Sprintf("/api/chats/private/%d/messages", rina))
	found := false
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		if num(m["id"]) == 1003 {
			found = true
			if flag(m) != true {
				t.Errorf("private 1003: %v", m)
			}
		} else if _, ok := m["deleted_on_recheck"]; ok {
			t.Errorf("private %v has the flag", m["id"])
		}
	}
	if !found {
		t.Error("private 1003 missing")
	}

	// The message page, and an album item there.
	pm := get("/api/page/messages/group/1020")["props"].(map[string]any)["message"].(map[string]any)
	if flag(pm) != true {
		t.Errorf("message page 1020: %v", flag(pm))
	}
	pm = get("/api/page/messages/group/1040")["props"].(map[string]any)["message"].(map[string]any)
	if _, ok := pm["deleted_on_recheck"]; ok {
		t.Errorf("message page 1040: %v", flag(pm))
	}
	if it := asList(pm["album"]); len(it) != 2 || it[0].(map[string]any)["deleted_on_recheck"] != true {
		t.Errorf("message page album: %s", js(pm["album"]))
	}
	pm = get("/api/page/messages/group/1030")["props"].(map[string]any)["message"].(map[string]any)
	if _, ok := pm["deleted_on_recheck"]; ok {
		t.Errorf("message page 1030: %v", flag(pm))
	}

	// The message lists.
	f := url.QueryEscape(fmt.Sprintf(`[{"field":"chat_id","op":"=","value":"%d"},`+
		`{"field":"is_deleted","op":"is","value":"yes"}]`, pagingLab))
	rows := asList(get("/api/page/group_messages?f=" + f)["props"].(map[string]any)["rows"])
	seen := 0
	for _, r := range rows {
		r := r.(map[string]any)
		switch id := num(r["id"]); id {
		case 1020, 1041:
			seen++
			if flag(r) != true {
				t.Errorf("list %d: %v", id, r)
			}
		default:
			if _, ok := r["deleted_on_recheck"]; ok {
				t.Errorf("list %d has the flag", id)
			}
		}
	}
	if seen != 2 || len(rows) != 5 {
		t.Errorf("list: %d of the flagged rows in %s", seen, js(rows))
	}
	rows = asList(get("/api/page/private_messages")["props"].(map[string]any)["rows"])
	seen = 0
	for _, r := range rows {
		if r := r.(map[string]any); num(r["id"]) == 1003 && flag(r) == true {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("private list: %s", js(rows))
	}

	// The dashboard's deleted messages: 1020 is the latest deletion.
	del := asList(get("/api/page/")["props"].(map[string]any)["deleted"])
	if len(del) == 0 || num(del[0].(map[string]any)["id"]) != 1020 || flag(del[0].(map[string]any)) != true {
		t.Fatalf("dashboard deleted: %s", js(del))
	}
	for _, d := range del[1:] {
		if d := d.(map[string]any); d["deleted_on_recheck"] != nil && num(d["id"]) != 1041 {
			t.Errorf("dashboard %v has the flag", d["id"])
		}
	}
}

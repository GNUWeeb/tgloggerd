// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"testing"
	"time"
)

// The groups of the "group_members" fixture.
const (
	gmFresh   = int64(-1001000000001) // GNU/Weeb, 1234 members, fresh
	gmStale   = int64(-1001000000002) // Linux Kernel Indonesia, 500, stale
	gmNone    = int64(-1001000000003) // Rust Programming, no count
	gmChannel = int64(-1001000000005) // Assembly Lounge, 880, a channel
)

const groupMembersFixture = "group_members"

// memberCountOf reads a group summary's member_count and checks that
// member_count_at goes with it.
func memberCountOf(t *testing.T, what string, g any) any {
	t.Helper()
	m, ok := g.(map[string]any)
	if !ok {
		t.Fatalf("%s: no group in %v", what, g)
	}
	n, ok := m["member_count"]
	if !ok {
		t.Fatalf("%s: no member_count in %v", what, m)
	}
	at, ok := m["member_count_at"]
	if !ok {
		t.Fatalf("%s: no member_count_at in %v", what, m)
	}
	if (n == nil) != (at == nil) {
		t.Errorf("%s: member_count %v at %v", what, n, at)
	}
	if s, ok := at.(string); ok {
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Errorf("%s: member_count_at %q", what, s)
		}
	}
	if f, ok := n.(float64); ok {
		return int64(f)
	}
	return n
}

// Telegram's member count, as the daemon stored it, is on the chat
// head, the group page and its tabs, an exposed group's public page
// and the groups list; null where none is stored.
func TestMemberCountShown(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	for _, c := range []struct {
		id   int64
		want any
	}{{gmFresh, int64(1234)}, {gmStale, int64(500)}, {gmNone, nil}, {gmChannel, int64(880)}} {
		for _, path := range []string{"/groups/%d/chat", "/groups/%d", "/groups/%d/admins", "/groups/%d/history"} {
			p := fmt.Sprintf(path, c.id)
			st, b := e.page("admin", p)
			if st != 200 {
				t.Fatalf("%s: %d %v", p, st, b)
			}
			props := b["props"].(map[string]any)
			g := props["group"]
			if chat, ok := props["chat"].(map[string]any); ok {
				g = chat["group"]
			}
			if n := memberCountOf(t, p, g); n != c.want {
				t.Errorf("%s: member_count %v, want %v", p, n, c.want)
			}
		}
	}

	// A member sees the exposed GNU/Weeb's public page, with the count.
	st, b := e.page("member", fmt.Sprintf("/groups/%d", gmFresh))
	if st != 200 || b["props"].(map[string]any)["public"] != true {
		t.Fatalf("public page: %d %v", st, b)
	}
	if n := memberCountOf(t, "public", b["props"].(map[string]any)["group"]); n != int64(1234) {
		t.Errorf("public page: member_count %v", n)
	}

	// The groups list, sorted and filtered by the count.
	st, b = e.page("admin", `/groups?sort=member_count&dir=desc&per=10`)
	if st != 200 {
		t.Fatalf("list: %d %v", st, b)
	}
	rows := b["props"].(map[string]any)["rows"].([]any)
	got := map[int64]any{}
	var order []int64
	for _, r := range rows {
		m := r.(map[string]any)
		id := num(m["id"])
		got[id] = memberCountOf(t, "list", m)
		order = append(order, id)
	}
	if got[gmFresh] != int64(1234) || got[gmStale] != int64(500) || got[gmNone] != nil ||
		len(order) < 3 || order[0] != gmFresh || order[1] != gmChannel || order[2] != gmStale {
		t.Errorf("list: %v in order %v", got, order)
	}
	st, b = e.page("admin", `/groups?f=[{"field":"member_count","op":">","value":"600"}]`)
	if rows := b["props"].(map[string]any)["rows"].([]any); st != 200 || len(rows) != 2 {
		t.Errorf("list filtered: %d %v", st, b)
	}
}

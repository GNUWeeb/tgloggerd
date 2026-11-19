// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
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

// refreshCalls counts the daemon's refresh calls for a group.
func refreshCalls(e *adminEnv, id int64) int {
	n := 0
	for _, c := range e.daemon.calls() {
		if strings.HasPrefix(c, fmt.Sprintf("/v1/chats/%d/refresh ", id)) {
			n++
		}
	}
	return n
}

// A page showing a missing or stale count asks the daemon for a fresh
// one in the background, once while the call is in flight, without
// waiting for it; the next page load shows what the daemon stored. A
// fresh count asks nothing, and a failed refresh keeps the stored count.
func TestMemberCountRefresh(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	release := make(chan struct{})
	e.daemon.mu.Lock()
	e.daemon.answers[fmt.Sprintf("/v1/chats/%d/refresh", gmStale)] = func(w http.ResponseWriter) {
		<-release
		// The daemon stores what Telegram said, then answers.
		if _, err := e.ro.Exec("UPDATE telegram_groups SET member_count = 777, "+
			"member_count_at = NOW() WHERE id = ?", gmStale); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"member_count":777,"can_get_members":true,"is_channel":false,`+
			`"account_is_admin":false,"fetched_at":1763540000}`)
	}
	e.daemon.mu.Unlock()
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/refresh", gmNone), 504, "", `{"error":"timeout"}`)

	count := func(path string) any {
		t.Helper()
		st, b := e.page("admin", path)
		if st != 200 {
			t.Fatalf("%s: %d %v", path, st, b)
		}
		props := b["props"].(map[string]any)
		if chat, ok := props["chat"].(map[string]any); ok {
			return memberCountOf(t, path, chat["group"])
		}
		return memberCountOf(t, path, props["group"])
	}

	// The daemon holds the call; the pages answer at once, with the
	// stored count, and ask only once.
	start := time.Now()
	for _, p := range []string{"/groups/%d/chat", "/groups/%d", "/groups/%d/chat", "/groups/%d/history"} {
		if n := count(fmt.Sprintf(p, gmStale)); n != int64(500) {
			t.Errorf("%s: %v while the refresh runs", p, n)
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the pages waited for the refresh: %v", d)
	}
	waitFor(t, "the refresh call", func() bool { return refreshCalls(e, gmStale) == 1 })
	close(release)
	waitFor(t, "the stored count", func() bool {
		return e.one("SELECT member_count FROM telegram_groups WHERE id = ?", gmStale) == "777"
	})
	if n := count(fmt.Sprintf("/groups/%d/chat", gmStale)); n != int64(777) {
		t.Errorf("after the refresh: %v", n)
	}
	if n := refreshCalls(e, gmStale); n != 1 {
		t.Errorf("%d refresh calls for the stale group", n)
	}

	// A fresh count asks nothing.
	for _, p := range []string{"/groups/%d/chat", "/groups/%d"} {
		count(fmt.Sprintf(p, gmFresh))
	}

	// No count: asked; the daemon fails, the count stays unknown, and
	// the next pages do not ask again right away.
	if n := count(fmt.Sprintf("/groups/%d", gmNone)); n != nil {
		t.Errorf("no count: %v", n)
	}
	waitFor(t, "the failed refresh", func() bool { return refreshCalls(e, gmNone) == 1 })
	for range 3 {
		if n := count(fmt.Sprintf("/groups/%d/chat", gmNone)); n != nil {
			t.Errorf("after the failed refresh: %v", n)
		}
	}
	// An exposed group's public page asks too (the member's view of the
	// fresh GNU/Weeb asks nothing).
	e.page("member", fmt.Sprintf("/groups/%d", gmFresh))
	time.Sleep(100 * time.Millisecond)
	if n, m := refreshCalls(e, gmNone), refreshCalls(e, gmFresh); n != 1 || m != 0 {
		t.Errorf("refresh calls: %d for the group without a count, %d for the fresh one", n, m)
	}
}

// Without the daemon API the pages ask nothing and still show the
// stored count.
func TestMemberCountWithoutDaemon(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	e.with(func(d *Deps) { d.Daemon = nil })
	st, b := e.page("admin", fmt.Sprintf("/groups/%d/chat", gmStale))
	if st != 200 {
		t.Fatalf("%d %v", st, b)
	}
	g := b["props"].(map[string]any)["chat"].(map[string]any)["group"]
	if n := memberCountOf(t, "chat", g); n != int64(500) {
		t.Errorf("member_count %v", n)
	}
	if c := e.daemon.calls(); len(c) != 0 {
		t.Errorf("asked the daemon: %v", c)
	}
}

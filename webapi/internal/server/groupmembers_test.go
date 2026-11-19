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
	"sync/atomic"
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

// groupFlags are a group page's member list flags as role sees them.
func groupFlags(t *testing.T, e *adminEnv, role string, id int64) (hidden, can bool) {
	t.Helper()
	st, b := e.page(role, fmt.Sprintf("/groups/%d", id))
	if st != 200 {
		t.Fatalf("%s /groups/%d: %d %v", role, id, st, b)
	}
	p := b["props"].(map[string]any)
	h, ok1 := p["hide_member_list"].(bool)
	c, ok2 := p["can_hide_member_list"].(bool)
	if !ok1 || !ok2 {
		t.Fatalf("%s /groups/%d: flags %v %v", role, id, p["hide_member_list"], p["can_hide_member_list"])
	}
	return h, c
}

// A superadmin hides a channel's member list and shows it again, each
// change audited; an admin may not (403), a member does not see the
// channel (404), and a group that is not a channel cannot be hidden.
func TestHideMemberList(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	path := fmt.Sprintf("/api/groups/%d", gmChannel)

	if h, c := groupFlags(t, e, "superadmin", gmChannel); h || !c {
		t.Errorf("superadmin, channel: hidden %v can %v", h, c)
	}
	if h, c := groupFlags(t, e, "admin", gmChannel); h || c {
		t.Errorf("admin, channel: hidden %v can %v", h, c)
	}
	if _, c := groupFlags(t, e, "superadmin", gmFresh); c {
		t.Error("superadmin may hide a supergroup's list")
	}

	if st, _, _ := e.call("superadmin", "PATCH", path, map[string]any{"hide_member_list": true}, false); st != 403 {
		t.Errorf("without CSRF: %d", st)
	}
	b := e.want(200, "superadmin", "PATCH", path, map[string]any{"hide_member_list": true})
	if js(b) != fmt.Sprintf(`{"can_hide_member_list":true,"hide_member_list":true,"id":%d}`, gmChannel) {
		t.Errorf("hide: %v", b)
	}
	// Again: no change, no audit.
	e.want(200, "superadmin", "PATCH", path, map[string]any{"hide_member_list": true})
	if h, _ := groupFlags(t, e, "admin", gmChannel); !h {
		t.Error("admin does not see the list as hidden")
	}
	if h, _ := groupFlags(t, e, "superadmin", gmChannel); !h {
		t.Error("superadmin does not see the list as hidden")
	}
	if n := e.appOne("SELECT hidden_by FROM web_hidden_member_lists WHERE group_id = ?", gmChannel); n != "1" {
		t.Errorf("hidden_by %q", n)
	}

	if b := e.want(403, "admin", "PATCH", path, map[string]any{"hide_member_list": false}); b["error"] != "forbidden" {
		t.Errorf("admin: %v", b)
	}
	e.want(404, "member", "PATCH", path, map[string]any{"hide_member_list": false})
	e.want(404, "superadmin", "PATCH", "/api/groups/-1009999999999", map[string]any{"hide_member_list": true})
	e.want(404, "superadmin", "PATCH", "/api/groups/42", map[string]any{"hide_member_list": true})
	e.want(400, "superadmin", "PATCH", fmt.Sprintf("/api/groups/%d", gmFresh),
		map[string]any{"hide_member_list": true})
	e.want(400, "superadmin", "PATCH", path, map[string]any{})
	e.want(400, "superadmin", "PATCH", path, map[string]any{"hide_member_list": "yes"})
	if st, _, _ := e.call("", "PATCH", path, map[string]any{"hide_member_list": false}); st != 401 {
		t.Errorf("signed out: %d", st)
	}

	e.want(200, "superadmin", "PATCH", path, map[string]any{"hide_member_list": false})
	if h, _ := groupFlags(t, e, "admin", gmChannel); h {
		t.Error("still hidden")
	}
	want := []string{
		fmt.Sprintf("1 member_list_hide group %d", gmChannel),
		fmt.Sprintf("1 member_list_show group %d", gmChannel),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audits\n%s", strings.Join(got, "\n"))
	}
}

// memberCalls are the daemon's member list calls for a group, with
// their query.
func memberCalls(e *adminEnv, id int64) []string {
	var out []string
	for _, c := range e.daemon.calls() {
		if p := fmt.Sprintf("/v1/chats/%d/members?", id); strings.HasPrefix(c, p) {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(c, p)))
		}
	}
	return out
}

const gmUnstored = int64(9299555555) // a member the archive has no row of

// A superadmin and an admin get a page of the members, named from
// telegram_users (one not stored named by its id); a member and a
// signed-out visitor get 404, as for an unknown group.
func TestGroupMembersList(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmFresh), 200, "",
		fmt.Sprintf(`{"available":true,"reason":"","total":1300,"members":[`+
			`{"user_id":9299000003,"status":"creator","joined_at":null},`+
			`{"user_id":9299000001,"status":"administrator","joined_at":1700000000},`+
			`{"user_id":%d,"status":"restricted","joined_at":1700000100}]}`, gmUnstored))
	path := fmt.Sprintf("/api/groups/%d/members", gmFresh)

	for _, role := range []string{"superadmin", "admin"} {
		b := e.want(200, role, "GET", path, nil)
		if b["available"] != true || b["reason"] != "" || num(b["total"]) != 1300 ||
			num(b["offset"]) != 0 || num(b["limit"]) != 50 {
			t.Errorf("%s: %v", role, b)
		}
		ms := b["members"].([]any)
		if len(ms) != 3 {
			t.Fatalf("%s: members %v", role, ms)
		}
		m0, m1, m2 := ms[0].(map[string]any), ms[1].(map[string]any), ms[2].(map[string]any)
		u0, u1, u2 := m0["user"].(map[string]any), m1["user"].(map[string]any), m2["user"].(map[string]any)
		if num(u0["id"]) != 9299000003 || u0["name"] != "Sari Wulandari" || m0["status"] != "creator" ||
			m0["joined_at"] != nil {
			t.Errorf("%s: first %v", role, m0)
		}
		if num(u1["id"]) != 9299000001 || u1["name"] != "Dewi Kusuma" || u1["photo"] == nil ||
			m1["status"] != "administrator" || m1["joined_at"] != "2023-11-14T22:13:20Z" {
			t.Errorf("%s: second %v", role, m1)
		}
		if num(u2["id"]) != gmUnstored || u2["name"] != fmt.Sprintf("#%d", gmUnstored) ||
			u2["type"] != "unknown" || m2["status"] != "restricted" {
			t.Errorf("%s: unstored %v", role, m2)
		}
	}
	for _, role := range []string{"member", ""} {
		if st, b, _ := e.call(role, "GET", path, nil); st != 404 {
			t.Errorf("%q: %d %v", role, st, b)
		}
	}
	e.want(404, "admin", "GET", "/api/groups/-1009999999999/members", nil)
	e.want(404, "admin", "GET", "/api/groups/42/members", nil)
	for _, q := range []string{"?limit=0", "?limit=201", "?offset=-1", "?offset=1000001", "?limit=x"} {
		e.want(400, "admin", "GET", path+q, nil)
	}
	// The daemon was asked once for the first page (the first page is
	// kept), with the page's offset and limit.
	if c := memberCalls(e, gmFresh); len(c) != 1 || c[0] != "offset=0&limit=50" {
		t.Errorf("daemon calls %v", c)
	}
}

// A group hidden from an admin has no members list for it.
func TestGroupMembersHiddenGroup(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) VALUES (?, ?)",
		e.uids["admin"], gmFresh); err != nil {
		t.Fatal(err)
	}
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmFresh), 200, "",
		`{"available":true,"reason":"","total":1,"members":[]}`)
	e.want(404, "admin", "GET", fmt.Sprintf("/api/groups/%d/members", gmFresh), nil)
	if st, _ := e.page("admin", fmt.Sprintf("/groups/%d/members", gmFresh)); st != 404 {
		t.Errorf("page: %d", st)
	}
	if c := memberCalls(e, gmFresh); len(c) != 0 {
		t.Errorf("daemon asked: %v", c)
	}
}

// The first page of a group's members is kept for two minutes per
// group and limit; other pages are asked every time.
func TestGroupMembersFirstPageCache(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmFresh), 200, "",
		`{"available":true,"reason":"","total":2,"members":[{"user_id":9299000001,"status":"member","joined_at":null}]}`)
	now := time.Now()
	var later atomic.Int64
	e.with(func(d *Deps) {
		d.clock = func() time.Time { return now.Add(time.Duration(later.Load())) }
	})
	path := fmt.Sprintf("/api/groups/%d/members", gmFresh)
	for range 3 {
		e.want(200, "superadmin", "GET", path, nil)
		e.want(200, "admin", "GET", path, nil)
	}
	e.want(200, "admin", "GET", path+"?limit=10", nil)
	e.want(200, "admin", "GET", path+"?offset=50", nil)
	e.want(200, "admin", "GET", path+"?offset=50", nil)
	// The group_members page uses the same first page.
	if st, b := e.page("admin", fmt.Sprintf("/groups/%d/members", gmFresh)); st != 200 {
		t.Fatalf("page: %d %v", st, b)
	}
	want := []string{"offset=0&limit=50", "offset=0&limit=10", "offset=50&limit=50", "offset=50&limit=50"}
	if c := memberCalls(e, gmFresh); strings.Join(c, " ") != strings.Join(want, " ") {
		t.Errorf("daemon calls %v", c)
	}
	// Two minutes on, the first page is asked again.
	later.Store(int64(memberPageTTL - time.Second))
	e.want(200, "admin", "GET", path, nil)
	later.Store(int64(memberPageTTL))
	e.want(200, "admin", "GET", path, nil)
	e.want(200, "admin", "GET", path, nil)
	if c := memberCalls(e, gmFresh); len(c) != 5 || c[4] != "offset=0&limit=50" {
		t.Errorf("after two minutes: %v", c)
	}
}

// When the list is not available, the answer says why, with the
// stored count as its total and no members.
func TestGroupMembersUnavailable(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	set := func(id int64, status int, body string) {
		e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", id), status, "", body)
	}
	get := func(id int64, q string) map[string]any {
		t.Helper()
		b := e.want(200, "admin", "GET", fmt.Sprintf("/api/groups/%d/members%s", id, q), nil)
		if b["available"] != false || len(b["members"].([]any)) != 0 {
			t.Errorf("%d%s: %v", id, q, b)
		}
		return b
	}
	set(gmChannel, 200, `{"available":false,"reason":"not_allowed","total":880,"members":[]}`)
	if b := get(gmChannel, ""); b["reason"] != "not_allowed" || num(b["total"]) != 880 {
		t.Errorf("not allowed: %v", b)
	}
	set(gmStale, 200, `{"available":false,"reason":"flood_wait","retry_after":42,"total":null,"members":[]}`)
	if b := get(gmStale, "?offset=50"); b["reason"] != "flood_wait" || num(b["retry_after"]) != 42 ||
		num(b["total"]) != 500 {
		t.Errorf("flood wait: %v", b)
	}
	set(gmStale, 200, `{"available":false,"reason":"error","message":"CHAT_ADMIN_REQUIRED","total":null,"members":[]}`)
	if b := get(gmStale, "?offset=100"); b["reason"] != "error" || b["retry_after"] != nil {
		t.Errorf("error: %v", b)
	}
	// A reason this service does not know yet passes through.
	set(gmStale, 200, `{"available":false,"reason":"something_new","total":null,"members":[]}`)
	if b := get(gmStale, "?offset=150"); b["reason"] != "something_new" {
		t.Errorf("new reason: %v", b)
	}
	set(gmStale, 504, `{"error":"timeout"}`)
	if b := get(gmStale, "?offset=200"); b["reason"] != "unavailable" || num(b["total"]) != 500 {
		t.Errorf("daemon timeout: %v", b)
	}
	// No count stored and no list: total null.
	if b := get(gmNone, ""); b["reason"] != "unavailable" || b["total"] != nil {
		t.Errorf("unknown chat: %v", b)
	}
	e.with(func(d *Deps) { d.Daemon = nil })
	if b := get(gmFresh, ""); b["reason"] != "unavailable" || num(b["total"]) != 1234 {
		t.Errorf("daemon off: %v", b)
	}
}

// A hidden channel list answers reason hidden to admins, without
// asking the daemon, and is listed for superadmins.
func TestGroupMembersHiddenList(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmChannel), 200, "",
		`{"available":true,"reason":"","total":880,"members":[{"user_id":9299000002,"status":"member","joined_at":null}]}`)
	e.want(200, "superadmin", "PATCH", fmt.Sprintf("/api/groups/%d", gmChannel),
		map[string]any{"hide_member_list": true})
	path := fmt.Sprintf("/api/groups/%d/members", gmChannel)
	b := e.want(200, "admin", "GET", path, nil)
	if b["available"] != false || b["reason"] != "hidden" || num(b["total"]) != 880 ||
		len(b["members"].([]any)) != 0 {
		t.Errorf("admin: %v", b)
	}
	if c := memberCalls(e, gmChannel); len(c) != 0 {
		t.Errorf("asked the daemon for the admin: %v", c)
	}
	b = e.want(200, "superadmin", "GET", path, nil)
	if b["available"] != true || len(b["members"].([]any)) != 1 {
		t.Errorf("superadmin: %v", b)
	}
	// Cached for the superadmin: the admin still gets hidden.
	if b := e.want(200, "admin", "GET", path, nil); b["reason"] != "hidden" {
		t.Errorf("admin after the superadmin: %v", b)
	}
	st, p := e.page("admin", fmt.Sprintf("/groups/%d/members", gmChannel))
	if props := p["props"].(map[string]any); st != 200 || props["reason"] != "hidden" ||
		props["hide_member_list"] != true || props["can_hide_member_list"] != false {
		t.Errorf("admin page: %d %v", st, p)
	}
	st, p = e.page("superadmin", fmt.Sprintf("/groups/%d/members", gmChannel))
	if props := p["props"].(map[string]any); st != 200 || props["available"] != true ||
		props["hide_member_list"] != true || props["can_hide_member_list"] != true {
		t.Errorf("superadmin page: %d %v", st, p)
	}
}

// The group_members page: the group's page as its other tabs have it
// and a page of members; not_found for a member, sign in for a
// signed-out visitor.
func TestGroupMembersPage(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmFresh), 200, "",
		`{"available":true,"reason":"","total":1300,"members":[{"user_id":9299000001,"status":"member","joined_at":null}]}`)
	st, b := e.page("admin", fmt.Sprintf("/groups/%d/members?offset=50", gmFresh))
	if st != 200 || b["route"] != "group_members" {
		t.Fatalf("%d %v", st, b)
	}
	p := b["props"].(map[string]any)
	g := p["group"].(map[string]any)
	if g["title"] != "GNU/Weeb" || memberCountOf(t, "page", g) != int64(1234) || p["admins"] == nil ||
		p["available"] != true || num(p["total"]) != 1300 || num(p["offset"]) != 50 ||
		len(p["members"].([]any)) != 1 {
		t.Errorf("props %v", p)
	}
	if c := memberCalls(e, gmFresh); len(c) != 1 || c[0] != "offset=50&limit=50" {
		t.Errorf("daemon calls %v", c)
	}
	if st, _ := e.page("member", fmt.Sprintf("/groups/%d/members", gmFresh)); st != 404 {
		t.Errorf("member: %d", st)
	}
	if st, _ := e.page("", fmt.Sprintf("/groups/%d/members", gmFresh)); st != 401 {
		t.Errorf("signed out: %d", st)
	}
	if st, _ := e.page("admin", "/groups/-1009999999999/members"); st != 404 {
		t.Errorf("unknown group: %d", st)
	}
}

// The members are named in one query, however many there are.
func TestGroupMembersBatched(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, groupMembersFixture)
	n := countTable(t, e, "telegram_users")
	list := func(k int) string {
		var ms []string
		for i := range k {
			ms = append(ms, fmt.Sprintf(`{"user_id":%d,"status":"member","joined_at":null}`, 9299000001+i))
		}
		return `{"available":true,"reason":"","total":40,"members":[` + strings.Join(ms, ",") + `]}`
	}
	cost := func(k, offset int) int64 {
		t.Helper()
		e.daemon.answer(fmt.Sprintf("/v1/chats/%d/members", gmFresh), 200, "", list(k))
		before := n.Load()
		b := e.want(200, "admin", "GET", fmt.Sprintf("/api/groups/%d/members?offset=%d", gmFresh, offset), nil)
		if len(b["members"].([]any)) != k {
			t.Fatalf("%d members: %v", k, b)
		}
		return n.Load() - before
	}
	few, many := cost(2, 0), cost(40, 50)
	if few == 0 || few != many {
		t.Errorf("statements on telegram_users: %d for 2 members, %d for 40", few, many)
	}
}

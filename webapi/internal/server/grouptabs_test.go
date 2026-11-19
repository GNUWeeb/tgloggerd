// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"testing"
)

// The group page's Files and Polls tabs, on the "group_tabs" fixture
// (Tabs Lab: 61 messages with files, 55 polls).

const (
	tabsLab     = int64(-1001000000060)
	tabsFixture = "group_tabs"
	asmLounge   = int64(-1001000000005) // the sample's channel
)

// tabsMsgID is Tabs Lab's message n's id.
func tabsMsgID(n int) float64 { return float64(int64(800000+n) << 20) }

func tabsPath(tab string, query string) string {
	p := fmt.Sprintf("/groups/%d/%s", tabsLab, tab)
	if query != "" {
		p += "?" + query
	}
	return p
}

// tabProps is a tab page's props as role sees it, which must answer
// 200.
func tabProps(t *testing.T, e *adminEnv, role, path string) map[string]any {
	t.Helper()
	st, b := e.page(role, path)
	if st != 200 {
		t.Fatalf("%s %s: %d %v", role, path, st, b)
	}
	if b["route"] != "group_files" && b["route"] != "group_polls" {
		t.Fatalf("%s: route %v", path, b["route"])
	}
	return b["props"].(map[string]any)
}

func tabRows(p map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range p["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

// Each kind lists only its own messages, the kinds list names only the
// kinds present with their counts, and the props carry the group.
func TestGroupFilesKinds(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)

	p := tabProps(t, e, "admin", tabsPath("files", ""))
	if g := p["group"].(map[string]any); g["id"] != float64(tabsLab) || g["title"] != "Tabs Lab" ||
		g["admin_count"] != 1.0 {
		t.Errorf("group %v", g)
	}
	// The group page's own props, beside the tab's.
	if a := p["admins"].([]any); len(a) != 1 || p["history"] == nil || p["top_posters"] == nil {
		t.Errorf("group props: admins %v, history %v", a, p["history"])
	}
	if p["kind"] != "all" {
		t.Errorf("kind %v", p["kind"])
	}
	want := `[{"count":53,"kind":"photo"},{"count":1,"kind":"video"},{"count":1,"kind":"animation"},` +
		`{"count":1,"kind":"audio"},{"count":1,"kind":"voice"},{"count":2,"kind":"document"},` +
		`{"count":1,"kind":"sticker"},{"count":1,"kind":"video_note"}]`
	if got := js(p["kinds"]); got != want {
		t.Errorf("kinds\n got %s\nwant %s", got, want)
	}

	byKind := map[string][]int{
		"photo": nil, "video": {53}, "animation": {54}, "audio": {55}, "voice": {56},
		"document": {60, 57}, "sticker": {58}, "video_note": {59},
	}
	fileType := map[string]string{"photo": "photo", "video": "video", "animation": "animation",
		"audio": "audio", "voice": "voice", "document": "document", "sticker": "sticker",
		"video_note": "video_note"}
	for kind, ns := range byKind {
		p := tabProps(t, e, "admin", tabsPath("files", "kind="+kind))
		if p["kind"] != kind {
			t.Errorf("%s: kind %v", kind, p["kind"])
		}
		rows := tabRows(p)
		if kind == "photo" {
			// 53 photos: a full page, newest (61, the channel's) first.
			if len(rows) != 50 || rows[0]["message_id"] != tabsMsgID(61) ||
				rows[1]["message_id"] != tabsMsgID(52) || p["next_before"] != tabsMsgID(4) {
				t.Errorf("photos: %d rows, first %v, next %v", len(rows), rows[0]["message_id"],
					p["next_before"])
			}
			continue
		}
		if len(rows) != len(ns) || p["next_before"] != nil {
			t.Errorf("%s: %d rows, next %v", kind, len(rows), p["next_before"])
			continue
		}
		for i, n := range ns {
			if rows[i]["message_id"] != tabsMsgID(n) || rows[i]["file_type"] != fileType[kind] ||
				rows[i]["id"] != float64(840000+n) {
				t.Errorf("%s row %d: %v", kind, i, rows[i])
			}
		}
	}
	if p := tabProps(t, e, "admin", tabsPath("files", "kind=document")); tabRows(p)[1]["name"] != "notes.pdf" {
		t.Errorf("document name %v", tabRows(p)[1]["name"])
	}

	// A text and a game message are no file of the tab; another
	// group's kinds are its own.
	all := tabRows(tabProps(t, e, "admin", tabsPath("files", "")))
	for _, r := range all {
		if r["message_id"] == tabsMsgID(62) || r["message_id"] == tabsMsgID(63) {
			t.Errorf("listed %v", r)
		}
	}
	g := tabProps(t, e, "admin", fmt.Sprintf("/groups/%d/files", gnuWeeb))
	if got := js(g["kinds"]); got != `[{"count":1,"kind":"photo"},{"count":1,"kind":"document"},{"count":1,"kind":"sticker"}]` {
		t.Errorf("GNU/Weeb kinds %s", got)
	}
	// An unknown kind is every kind on the page, 400 from the API.
	if p := tabProps(t, e, "admin", tabsPath("files", "kind=nope")); p["kind"] != "all" || len(tabRows(p)) != 50 {
		t.Errorf("unknown kind: %v, %d rows", p["kind"], len(tabRows(p)))
	}
	for _, q := range []string{"kind=nope", "kind=text", "before=x", "before=-5", "before=0"} {
		if st, b, _ := e.call("admin", "GET", fmt.Sprintf("/api/groups/%d/files?%s", tabsLab, q), nil); st != 400 {
			t.Errorf("API %s: %d %v", q, st, b)
		}
	}
}

// Newest first, 50 a page, and ?before= the next: the page and the API
// answer the same lists.
func TestGroupFilesPaging(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)

	p := tabProps(t, e, "superadmin", tabsPath("files", ""))
	rows := tabRows(p)
	if len(rows) != 50 {
		t.Fatalf("%d rows", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i]["message_id"].(float64) >= rows[i-1]["message_id"].(float64) {
			t.Fatalf("not newest first at %d: %v", i, rows[i]["message_id"])
		}
	}
	// 61, 60, 59 ... 12.
	if rows[0]["message_id"] != tabsMsgID(61) || rows[49]["message_id"] != tabsMsgID(12) ||
		p["next_before"] != tabsMsgID(12) {
		t.Fatalf("first %v, last %v, next %v", rows[0]["message_id"], rows[49]["message_id"], p["next_before"])
	}
	next := fmt.Sprint(int64(tabsMsgID(12)))
	p2 := tabProps(t, e, "superadmin", tabsPath("files", "before="+next))
	rows2 := tabRows(p2)
	if len(rows2) != 11 || rows2[0]["message_id"] != tabsMsgID(11) || rows2[10]["message_id"] != tabsMsgID(1) ||
		p2["next_before"] != nil {
		t.Fatalf("second page: %d rows, next %v", len(rows2), p2["next_before"])
	}

	api := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/groups/%d/files?before=%s", tabsLab, next), nil)
	for _, k := range []string{"group", "admins", "history"} {
		if _, ok := api[k]; ok {
			t.Errorf("the API answers the group's %s", k)
		}
	}
	for _, k := range []string{"kinds", "kind", "rows", "next_before"} {
		if js(api[k]) != js(p2[k]) {
			t.Errorf("API %s\n got %s\nwant %s", k, js(api[k]), js(p2[k]))
		}
	}
	first := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/groups/%d/files", tabsLab), nil)
	if js(first["rows"]) != js(p["rows"]) || js(first["next_before"]) != js(p["next_before"]) {
		t.Errorf("API first page differs")
	}
	// Paging by kind.
	ph := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/groups/%d/files?kind=photo&before=%d",
		tabsLab, int64(tabsMsgID(4))), nil)
	if r := tabRows(ph); len(r) != 3 || r[0]["message_id"] != tabsMsgID(3) || ph["next_before"] != nil {
		t.Errorf("photos page 2: %v", js(ph))
	}
}

// Each row names its sender: a user, the group posting as itself, a
// channel the viewer sees, and one it does not, which is not named.
func TestGroupFilesSenders(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	grantMember(t, e, []int64{tabsLab})

	row := func(role string, n int) map[string]any {
		t.Helper()
		for _, r := range tabRows(tabProps(t, e, role, tabsPath("files", ""))) {
			if r["message_id"] == tabsMsgID(n) {
				return r
			}
		}
		t.Fatalf("%s: no row %d", role, n)
		return nil
	}
	sender := func(role string, n int) string {
		t.Helper()
		return js(row(role, n)["sender"])
	}

	if got := sender("admin", 61); got != `{"id":-1001000000005,"name":"Assembly Lounge","photo":null,"username":"asm_lounge"}` {
		t.Errorf("admin, channel: %s", got)
	}
	if got := sender("member", 61); got != `{"id":null,"name":"(hidden chat)","photo":null,"username":""}` {
		t.Errorf("member, hidden channel: %s", got)
	}
	for _, role := range []string{"admin", "member"} {
		if got := sender(role, 60); got != `{"id":-1001000000060,"name":"Tabs Lab","photo":null,"username":""}` {
			t.Errorf("%s, the group itself: %s", role, got)
		}
		if got := sender(role, 58); got != `{"id":9299000004,"name":"Andi Santoso","photo":null,"username":""}` {
			t.Errorf("%s, Andi: %s", role, got)
		}
	}
	if got := sender("admin", 59); got != fmt.Sprintf(`{"id":9299000001,"name":"Dewi Kusuma","photo":%q,"username":"dewikusuma"}`, photoOf(dewiPhoto)) {
		t.Errorf("admin, Dewi: %s", got)
	}
}

// Each row links to its message in the chat and on its message page,
// and carries its file as the files list does.
func TestGroupFilesLinks(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	rows := tabRows(tabProps(t, e, "admin", tabsPath("files", "")))
	r := rows[0] // message 61, a stored photo
	mid := int64(tabsMsgID(61))
	if r["chat_link"] != fmt.Sprintf("/groups/%d/chat?at=%d", tabsLab, mid) ||
		r["message_row_id"] != 6061.0 || r["date"] != iso(1794931200-86400+60*61) {
		t.Errorf("links %v", r)
	}
	if r["url"] != photoOf(840061) || r["thumb"] != photoOf(840061) || r["stored"] != true ||
		r["file_type"] != "photo" || r["name"] != "840061.jpg" || r["size"] != 1061.0 {
		t.Errorf("file %v", r)
	}
	// The message page the row links to is the message.
	_, m := e.page("admin", "/messages/group/6061")
	if mm, _ := m["props"].(map[string]any)["message"].(map[string]any); mm == nil ||
		mm["message_id"] != float64(mid) {
		t.Errorf("message page %v", m["props"])
	}
	// An unstored photo has no thumbnail.
	if r := rows[9]; r["message_id"] != tabsMsgID(52) || r["stored"] != false || r["thumb"] != nil {
		t.Errorf("unstored %v", r)
	}
}

// The polls tab: newest first, paged, each with the chat's poll detail
// (the past-close-date rule, at most 20 options), its sender and links.
func TestGroupPolls(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	grantMember(t, e, []int64{tabsLab})

	p := tabProps(t, e, "admin", tabsPath("polls", ""))
	if g := p["group"].(map[string]any); g["id"] != float64(tabsLab) {
		t.Errorf("group %v", g)
	}
	rows := tabRows(p)
	if len(rows) != 50 || rows[0]["message_id"] != tabsMsgID(150) || rows[49]["message_id"] != tabsMsgID(101) ||
		p["next_before"] != tabsMsgID(101) {
		t.Fatalf("%d rows, first %v, next %v", len(rows), rows[0]["message_id"], p["next_before"])
	}
	next := fmt.Sprint(int64(tabsMsgID(101)))
	p2 := tabProps(t, e, "admin", tabsPath("polls", "before="+next))
	rows2 := tabRows(p2)
	if len(rows2) != 5 || p2["next_before"] != nil {
		t.Fatalf("second page: %d rows, next %v", len(rows2), p2["next_before"])
	}
	at := map[int]map[string]any{}
	for i, n := range []int{74, 73, 72, 71, 70} {
		if rows2[i]["message_id"] != tabsMsgID(n) {
			t.Fatalf("row %d: %v", i, rows2[i]["message_id"])
		}
		at[n] = rows2[i]
	}
	poll := func(n int) map[string]any {
		t.Helper()
		pl, _ := at[n]["poll"].(map[string]any)
		if pl == nil {
			t.Fatalf("poll %d: %v", n, at[n]["poll"])
		}
		return pl
	}

	open := poll(70)
	if open["question"] != "Lunch?" || open["type"] != "regular" || open["anonymous"] != false ||
		open["multiple"] != true || open["closed"] != false || open["total_voters"] != 7.0 ||
		open["options_total"] != 2.0 || js(open["options"]) != `[{"correct":false,"percent":57,"text":"Rice","voters":4},{"correct":false,"percent":43,"text":"Noodles","voters":3}]` {
		t.Errorf("open poll %s", js(open))
	}
	// Stored open, but past its close date: closed.
	if pl := poll(71); pl["closed"] != true || pl["close_date"] != iso(1794931200-100) {
		t.Errorf("past close date %s", js(pl))
	}
	quiz := poll(72)
	opts := quiz["options"].([]any)
	if quiz["type"] != "quiz" || quiz["closed"] != true || len(opts) != 20 || quiz["options_total"] != 25.0 ||
		quiz["explanation"] != "The second one." {
		t.Errorf("quiz %s", js(quiz))
	}
	for i, o := range opts {
		if c := o.(map[string]any)["correct"]; c != (i == 1) {
			t.Errorf("quiz option %d correct %v", i, c)
		}
	}
	if at[73]["poll"] != nil {
		t.Errorf("unstored poll %v", at[73]["poll"])
	}
	if got := js(at[74]["sender"]); got != `{"id":-1001000000005,"name":"Assembly Lounge","photo":null,"username":"asm_lounge"}` {
		t.Errorf("admin, channel poll: %s", got)
	}
	if at[70]["chat_link"] != fmt.Sprintf("/groups/%d/chat?at=%d", tabsLab, int64(tabsMsgID(70))) ||
		at[70]["message_row_id"] != 6070.0 || at[70]["date"] != iso(1794931200-86400+60*70) ||
		js(at[70]["sender"]) != `{"id":9299000004,"name":"Andi Santoso","photo":null,"username":""}` {
		t.Errorf("row 70 %v", at[70])
	}

	// The member sees the same polls, the channel unnamed.
	m := e.want(200, "member", "GET", fmt.Sprintf("/api/groups/%d/polls?before=%s", tabsLab, next), nil)
	mr := tabRows(m)
	if len(mr) != 5 || js(mr[0]["sender"]) != `{"id":null,"name":"(hidden chat)","photo":null,"username":""}` ||
		js(mr[2]["poll"]) != js(at[72]["poll"]) {
		t.Errorf("member polls %s", js(m))
	}
	if st, b, _ := e.call("admin", "GET", fmt.Sprintf("/api/groups/%d/polls?before=abc", tabsLab), nil); st != 400 {
		t.Errorf("bad before: %d %v", st, b)
	}
	// Another group's poll is its own.
	g := tabRows(tabProps(t, e, "admin", fmt.Sprintf("/groups/%d/polls", gnuWeeb)))
	if len(g) != 1 || g[0]["message_row_id"] != 6201.0 {
		t.Errorf("GNU/Weeb polls %v", g)
	}
}

// Both tabs, page and API, for every role: 200 for a viewer that sees
// the group, 404 for one that does not (a member not granted it, an
// admin it is hidden from), even for an exposed group, and a session
// asked of a signed-out visitor.
func TestGroupTabsScope(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	grantMember(t, e, []int64{tabsLab})
	paths := func(g int64) []string {
		return []string{fmt.Sprintf("/groups/%d/files", g), fmt.Sprintf("/groups/%d/polls", g)}
	}
	check := func(role string, g int64, want int) {
		t.Helper()
		for _, p := range paths(g) {
			if st, b := e.page(role, p); st != want {
				t.Errorf("%q page %s: %d %v, want %d", role, p, st, b, want)
			}
			if st, b, _ := e.call(role, "GET", "/api"+p, nil); st != want {
				t.Errorf("%q API %s: %d %v, want %d", role, p, st, b, want)
			}
		}
	}
	for _, role := range []string{"superadmin", "admin", "member"} {
		check(role, tabsLab, 200)
	}
	// Not granted to the member; GNU/Weeb is exposed, its tabs are not.
	check("member", gnuWeeb, 404)
	check("member", kernelChat, 404)
	check("admin", gnuWeeb, 200)
	check("", tabsLab, 401)
	check("", gnuWeeb, 401)
	check("admin", -1009999999999, 404)

	e.want(200, "superadmin", "PATCH", "/api/accounts/3", map[string]any{"restrict": map[string]any{
		"hide_private": false, "hide_groups": []any{tabsLab}, "join_chat": false}})
	check("admin", tabsLab, 404)
	check("superadmin", tabsLab, 200)
	check("member", tabsLab, 200)
}

// A page costs as many statements with many rows as with one: the
// senders, the files' pictures and the polls come in batches.
func TestGroupTabsBatched(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	n := countTable(t, e, "")
	cost := func(path string) int64 {
		t.Helper()
		before := n.Load()
		if st, b := e.page("admin", path); st != 200 {
			t.Fatalf("%s: %d %v", path, st, b)
		}
		return n.Load() - before
	}
	// The first page counts the kinds, which are kept a while. The
	// photos and the documents both name users and chats.
	cost(tabsPath("files", ""))
	many, few := cost(tabsPath("files", "kind=photo")), cost(tabsPath("files", "kind=document"))
	if many == 0 || many != few {
		t.Errorf("files: %d statements for 50 rows, %d for 2", many, few)
	}
	videos, video := cost(tabsPath("files", "")), cost(tabsPath("files", "kind=video"))
	if videos != video+1 {
		// The video's row names no chat: one statement less.
		t.Errorf("files with videos: %d statements for 50 rows, %d for one", videos, video)
	}
	// Beyond the group page's own, a handful: the rows, the senders,
	// the chats, the pictures, the polls and their options.
	page := cost(fmt.Sprintf("/groups/%d", tabsLab))
	if many > page+6 {
		t.Errorf("files: %d statements, the group page %d", many, page)
	}
	next := fmt.Sprint(int64(tabsMsgID(101)))
	polls, five := cost(tabsPath("polls", "")), cost(tabsPath("polls", "before="+next))
	if polls == 0 || polls != five {
		t.Errorf("polls: %d statements for 50 rows, %d for 5", polls, five)
	}
	if polls > page+6 {
		t.Errorf("polls: %d statements, the group page %d", polls, page)
	}
}

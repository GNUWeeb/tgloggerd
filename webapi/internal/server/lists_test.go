// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/url"
	"strings"
	"testing"
)

// The list pages' data through GET /api/page/<path>, on the fixtures
// of the search tests (searchEnv), for each role.

// page asks for a list page's data as role and returns its props.
func (e *searchEnv) page(role, path string, query ...string) (int, map[string]any) {
	e.t.Helper()
	q := url.Values{}
	for i := 0; i+1 < len(query); i += 2 {
		q.Set(query[i], query[i+1])
	}
	st, b := e.get(role, "/api/page/"+path+"?"+q.Encode())
	if st == 200 && b["route"] != strings.TrimPrefix(path, "/") {
		e.t.Errorf("%s: route %v", path, b["route"])
	}
	props, _ := b["props"].(map[string]any)
	return st, props
}

func (e *searchEnv) props(role, path string, query ...string) map[string]any {
	e.t.Helper()
	st, p := e.page(role, path, query...)
	if st != 200 {
		e.t.Fatalf("%s %s %v: %d", role, path, query, st)
	}
	return p
}

func listIDs(p map[string]any) string {
	var ids []string
	for _, r := range p["rows"].([]any) {
		ids = append(ids, js(r.(map[string]any)["id"]))
	}
	return strings.Join(ids, ",")
}

func wantList(t *testing.T, what string, p map[string]any, total int64, ids string) {
	t.Helper()
	if num(p["total"]) != total || (ids != "*" && listIDs(p) != ids) {
		t.Errorf("%s: total %v rows %s, want %d %s", what, p["total"], listIDs(p), total, ids)
	}
}

func TestListUsers(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	p := e.props("superadmin", "users")
	wantList(t, "users", p, 9, "9299000001,9299000003,9299000002,9299000007,9299000004,"+
		"9299100001,9299000008,9299000006,9299000005")
	if p["sort"] != "msg_count" || p["dir"] != "desc" || num(p["page"]) != 1 ||
		num(p["pages"]) != 1 || num(p["per"]) != 25 || num(p["archive_total"]) != 9 ||
		js(p["conds"]) != "[]" || p["q"] != "" {
		t.Errorf("state %v", p)
	}
	dewi := map[string]any{"id": 9299000001, "name": "Dewi Kusuma", "username": "dewikusuma",
		"type": "regular", "photo": e.file(880007), "contact_name": "Dewi (kernel)",
		"msg_count": 6, "is_verified": false, "is_premium": true, "is_scam": false,
		"is_fake": false, "is_support": false, "language": "id",
		"created_at": "2023-02-11T07:30:00Z", "updated_at": "2026-11-17T15:31:00Z",
		"phone": "+62 812 5550 1234", "bio": "Kernel hacker. Coffee first. She/her.",
		"restriction_reason": "", "has_sensitive_content": false, "restricts_new_chats": false,
		"paid_message_star_count": 0, "personal_chat_id": nil, "emoji_status": nil}
	if js(p["rows"].([]any)[0]) != js(dewi) {
		t.Errorf("Dewi:\n got %s\nwant %s", js(p["rows"].([]any)[0]), js(dewi))
	}
	tom := p["rows"].([]any)[5].(map[string]any)
	if tom["name"] != `<i>Tom & "Jerry"</i> O'Neil` || tom["photo"] != nil {
		t.Errorf("unescaped for the page: %v", tom)
	}

	for _, c := range []struct {
		query []string
		total int64
		ids   string
	}{
		{[]string{"f", `[{"field":"is_premium","op":"is","value":"yes"}]`}, 3, "9299000001,9299000003,9299000007"},
		{[]string{"f", `[{"field":"type","op":"=","value":"bot"}]`}, 1, "9299000005"},
		{[]string{"f", `[{"field":"name","op":"contains","value":"(kernel)"}]`}, 1, "9299000001"},
		{[]string{"f", `[{"field":"username","op":"empty","value":""}]`}, 3, "9299000004,9299000008,9299000006"},
		{[]string{"f", `[{"field":"language","op":"=","value":"id"}]`}, 2, "9299000001,9299000003"},
		{[]string{"f", `[{"field":"created_at","op":">","value":"2026-01-01"}]`}, 2, "9299100001,9299000008"},
		{[]string{"f", `[{"field":"msg_count","op":">","value":"2"},{"field":"is_premium","op":"is","value":"no"}]`}, 1, "9299000002"},
		{[]string{"q", "@sariw"}, 1, "9299000003"},
		{[]string{"q", "9299000005"}, 1, "9299000005"},
		{[]string{"q", "jerry"}, 1, "9299100001"},
		{[]string{"q", "a", "f", `[{"field":"is_premium","op":"is","value":"yes"}]`}, 3, "*"},
		{[]string{"sort", "name", "dir", "asc", "per", "3"}, 9, "9299000006,9299100001,9299000004"},
		{[]string{"sort", "msg_count", "per", "2", "page", "2"}, 9, "9299000002,9299000007"},
		// A page past the end shows the last one.
		{[]string{"per", "4", "page", "9"}, 9, "9299000005"},
		{[]string{"per", "4", "page", "99999999999"}, 9, "9299000005"},
	} {
		p := e.props("superadmin", "users", c.query...)
		wantList(t, strings.Join(c.query, " "), p, c.total, c.ids)
	}

	// A filter the list does not take, or a value that does not
	// parse, shows the whole list with the reason, as Drogon's lists
	// do; an unknown sort or direction is the default.
	p = e.props("superadmin", "users", "f", `[{"field":"nope","op":"=","value":"1"},`+
		`{"field":"type","op":"=","value":"bot"}]`)
	if js(p["conds"]) != `[]` || num(p["total"]) != 9 ||
		js(p["error"]) != `{"code":"bad_search","message":"unknown field: 'nope'"}` {
		t.Errorf("refused filters: %v %v %v", p["conds"], p["total"], p["error"])
	}
	p = e.props("superadmin", "users", "f", "not json", "sort", "bio", "dir", "sideways")
	if num(p["total"]) != 9 || p["sort"] != "msg_count" || p["dir"] != "desc" ||
		js(p["error"]) != `{"code":"bad_search","message":"f must be a JSON array of conditions"}` {
		t.Errorf("malformed: %v %v %v %v", p["total"], p["sort"], p["dir"], p["error"])
	}

	// An admin sees what a superadmin sees; a member sees nobody until
	// it has groups, and nothing but names and photos even then.
	if p := e.props("admin", "users"); num(p["total"]) != 9 || num(p["archive_total"]) != 9 {
		t.Errorf("admin: %v %v", p["total"], p["archive_total"])
	}
	p = e.props("member", "users", "f", `[{"field":"bio","op":"contains","value":"kernel"}]`,
		"sort", "msg_count")
	wantList(t, "member", p, 0, "")
	if _, ok := p["archive_total"]; ok || p["sort"] != "name" || js(p["conds"]) != "[]" {
		t.Errorf("member state: %v", p)
	}
}

func TestListGroups(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	p := e.props("admin", "groups")
	wantList(t, "groups", p, 6, "-1001000000001,-1001000000002,-1001000000003,"+
		"-1001000000005,-1001000000004,-1001000000010")
	gnu := map[string]any{"id": -1001000000001, "title": "GNU/Weeb", "username": "gnuweeb",
		"type": "supergroup", "photo": e.file(880008), "msg_count": 8, "exposed": true,
		"admin_count": 2, "description": "Low-level programming, Linux and free software.",
		"created_at": "2022-03-14T09:00:00Z", "updated_at": "2026-11-17T15:40:00Z",
		"member_count": nil, "member_count_at": nil}
	if js(p["rows"].([]any)[0]) != js(gnu) || num(p["archive_total"]) != 6 {
		t.Errorf("GNU/Weeb:\n got %s\nwant %s", js(p["rows"].([]any)[0]), js(gnu))
	}
	for _, c := range []struct {
		query []string
		total int64
		ids   string
	}{
		{[]string{"f", `[{"field":"exposed","op":"is","value":"yes"}]`}, 3, "-1001000000001,-1001000000003,-1001000000010"},
		{[]string{"f", `[{"field":"type","op":"=","value":"channel"}]`}, 1, "-1001000000005"},
		{[]string{"f", `[{"field":"username","op":"empty","value":""}]`}, 1, "-1001000000004"},
		{[]string{"f", `[{"field":"description","op":"contains","value":"kernel"}]`}, 1, "-1001000000002"},
		{[]string{"q", "@rustlang"}, 1, "-1001000000003"},
		{[]string{"q", "-1001000000010"}, 1, "-1001000000010"},
		{[]string{"sort", "admin_count", "per", "2"}, 6, "-1001000000001,-1001000000002"},
		{[]string{"sort", "title", "dir", "asc", "per", "2"}, 6, "-1001000000005,-1001000000004"},
	} {
		p := e.props("superadmin", "groups", c.query...)
		wantList(t, strings.Join(c.query, " "), p, c.total, c.ids)
	}
	p = e.props("member", "groups")
	wantList(t, "member", p, 0, "")
	if _, ok := p["archive_total"]; ok {
		t.Errorf("member archive total %v", p["archive_total"])
	}
}

func TestListFiles(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	p := e.props("superadmin", "files", "file", "880005")
	wantList(t, "files", p, 8, "880006,880004,880001,880002,880005,880003,880007,880008")
	// The page size is the viewer's rows_per_page, the site's 25 here.
	if num(p["per"]) != 25 || num(p["archive_total"]) != 8 || p["can_fetch"] != true ||
		num(p["stored_bytes"]) != 183406+3145728+24576+40960+51200 {
		t.Errorf("state: %v %v %v %v", p["per"], p["archive_total"], p["can_fetch"], p["stored_bytes"])
	}
	rs := p["rows"].([]any)
	byID := map[string]map[string]any{}
	for _, r := range rs {
		byID[js(r.(map[string]any)["id"])] = r.(map[string]any)
	}
	photo := map[string]any{"id": 880001, "file_type": "photo", "name": "880001.jpg",
		"ext": "jpg", "size": 183406, "hits": 12, "stored": true, "fetchable": false,
		"created_at": "2026-11-17T14:05:00Z", "accessed_at": "2026-11-17T15:30:00Z",
		"tg_file_id": "AgACAgUAAxkBAAIfixture01",
		"sha256":     "4a0886fd32d3b6fe7983dcfcb3f5948184e9751e308f0ddf34ca3076a7844913",
		"url":        e.file(880001), "thumb": e.file(880001),
		"chat":    map[string]any{"id": -1001000000001, "title": "GNU/Weeb", "kind": "group"},
		"sender":  map[string]any{"id": 9299000001, "name": "Dewi Kusuma"},
		"message": map[string]any{"scope": "group", "id": 4}}
	if js(byID["880001"]) != js(photo) {
		t.Errorf("photo:\n got %s\nwant %s", js(byID["880001"]), js(photo))
	}
	if f := byID["880006"]; f["url"] != nil || f["name"] != "io_uring-talk.pdf" ||
		js(f["chat"]) != `{"id":-1001000000002,"kind":"group","title":"Linux Kernel Indonesia"}` ||
		js(f["sender"]) != `{"id":9299000007,"name":"Putri Lestari"}` {
		t.Errorf("unstored document: %v", f)
	}
	if f := byID["880005"]; js(f["chat"]) != `{"id":9299000002,"kind":"user","title":"Bima Pratama"}` ||
		js(f["message"]) != `{"id":5,"scope":"private"}` || f["thumb"] != nil {
		t.Errorf("voice in a private chat: %v", f)
	}
	// A profile photo was sent in no message.
	if f := byID["880007"]; f["chat"] != nil || f["message"] != nil || f["sender"] != nil {
		t.Errorf("profile photo: %v", f)
	}
	if pv := p["preview"].(map[string]any); pv["id"] != float64(880005) {
		t.Errorf("preview %v", pv)
	}

	for _, c := range []struct {
		query []string
		total int64
		ids   string
	}{
		{[]string{"f", `[{"field":"stored","op":"is","value":"no"}]`}, 3, "880006,880005,880003"},
		{[]string{"f", `[{"field":"file_type","op":"=","value":"document"}]`}, 2, "880006,880002"},
		{[]string{"f", `[{"field":"size","op":">","value":"1000000"}]`}, 3, "880006,880002,880003"},
		{[]string{"q", "slides"}, 1, "880002"},
		{[]string{"q", "880004"}, 1, "880004"},
		{[]string{"sort", "size", "per", "2"}, 8, "880003,880006"},
	} {
		p := e.props("admin", "files", c.query...)
		wantList(t, strings.Join(c.query, " "), p, c.total, c.ids)
	}

	p = e.props("member", "files", "file", "880001")
	wantList(t, "member", p, 0, "")
	if p["preview"] != nil || p["can_fetch"] != false || p["stored_bytes"] != nil ||
		p["archive_total"] != nil {
		t.Errorf("member: %v", p)
	}
}

func TestListMessages(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	p := e.props("superadmin", "group_messages", "per", "3")
	if listIDs(p) != "100,8,7" || p["scope"] != "group" || p["sort"] != "date" ||
		num(p["total"]) < 17 || num(p["archive_total"]) < 0 || p["archive_total"] == nil {
		t.Errorf("state: %s %v %v %v", listIDs(p), p["sort"], p["total"], p["archive_total"])
	}
	tom := map[string]any{"id": 100, "scope": "group", "message_id": 440001 << 20,
		"date": "2026-11-17T15:50:00Z",
		"chat": map[string]any{"id": -1001000000004, "title": "C/C++ Study Group", "kind": "group",
			"photo": nil},
		"sender": map[string]any{"id": 9299100001, "name": `<i>Tom & "Jerry"</i> O'Neil`,
			"username": "tom_jerry", "type": "regular", "photo": nil},
		"content_type": "text", "snippet": "templates & <generics> in C++",
		"forwarded": "Secret Admirer", "edited": false, "deleted_at": nil, "service_type": nil}
	if js(p["rows"].([]any)[0]) != js(tom) {
		t.Errorf("newest:\n got %s\nwant %s", js(p["rows"].([]any)[0]), js(tom))
	}
	if r := p["rows"].([]any)[1].(map[string]any); r["snippet"] != "[service]" || r["forwarded"] != nil {
		t.Errorf("service message: %v", r)
	}

	for _, c := range []struct {
		query []string
		total int64
		ids   string
	}{
		{[]string{"f", `[{"field":"is_deleted","op":"is","value":"yes"}]`}, 1, "6"},
		{[]string{"f", `[{"field":"is_edited","op":"is","value":"yes"}]`}, 1, "3"},
		{[]string{"f", `[{"field":"is_forwarded","op":"is","value":"yes"}]`}, 2, "100,11"},
		{[]string{"f", `[{"field":"date","op":"<","value":"2026-11-17"}]`}, 3, "13,12,16"},
		{[]string{"f", `[{"field":"chat_id","op":"=","value":"-1001000000005"}]`}, 2, "15,14"},
		{[]string{"f", `[{"field":"sender_id","op":"=","value":"9299000007"}]`}, 2, "11,10"},
		{[]string{"f", `[{"field":"content_type","op":"!=","value":"text"}]`}, 6, "8,7,5,4,11,13"},
		{[]string{"f", `[{"field":"text","op":"contains","value":"kernel"}]`}, 1, "2"},
	} {
		p := e.props("superadmin", "group_messages", c.query...)
		wantList(t, strings.Join(c.query, " "), p, c.total, c.ids)
	}
	// The search text is a fulltext match, ordered by relevance.
	p = e.props("superadmin", "group_messages", "q", "build")
	if p["sort"] != "relevance" || num(p["total"]) != 2 || p["q"] != "build" {
		t.Errorf("relevance: %v %v", p["sort"], p["total"])
	}
	if p = e.props("superadmin", "group_messages", "q", "build", "sort", "date"); p["sort"] != "date" ||
		listIDs(p) != "3,15" {
		t.Errorf("by date: %v %s", p["sort"], listIDs(p))
	}
	p = e.props("admin", "group_messages", "f", `[{"field":"sender_id","op":"=","value":"9299000002"}]`)
	if r := p["rows"].([]any); len(r) != 2 {
		t.Errorf("Bima: %v", r)
	}
	// Forwarded from Bima; a channel's own post names the channel.
	p = e.props("admin", "group_messages", "f", `[{"field":"chat_id","op":"=","value":"-1001000000002"}]`)
	if r := p["rows"].([]any)[0].(map[string]any); r["forwarded"] != "Bima Pratama" {
		t.Errorf("forwarded: %v", r)
	}
	p = e.props("admin", "group_messages", "f", `[{"field":"chat_id","op":"=","value":"-1001000000005"}]`)
	if s := p["rows"].([]any)[1].(map[string]any)["sender"].(map[string]any); s["name"] != "Assembly Lounge" ||
		s["username"] != "asm_lounge" || s["id"] != float64(-1001000000005) {
		t.Errorf("channel post: %v", s)
	}

	p = e.props("member", "group_messages")
	wantList(t, "member", p, 0, "")
	if p["archive_total"] != nil {
		t.Errorf("member archive total %v", p["archive_total"])
	}

	p = e.props("superadmin", "private_messages")
	wantList(t, "private", p, 6, "4,3,2,1,6,5")
	deleted := map[string]any{"id": 4, "scope": "private", "message_id": 610004 << 20,
		"date": "2026-11-17T15:31:40Z",
		"chat": map[string]any{"id": 9299000001, "title": "Dewi Kusuma", "kind": "user",
			"photo": e.file(880007)},
		"sender": map[string]any{"id": 9299000001, "name": "Dewi Kusuma", "username": "dewikusuma",
			"type": "regular", "photo": e.file(880007)},
		"content_type": "text", "snippet": "never mind", "forwarded": nil, "edited": false,
		"deleted_at": "2026-11-17T15:32:00Z", "service_type": nil}
	if js(p["rows"].([]any)[0]) != js(deleted) || num(p["archive_total"]) != 6 {
		t.Errorf("private:\n got %s\nwant %s", js(p["rows"].([]any)[0]), js(deleted))
	}
	if s := p["rows"].([]any)[2].(map[string]any)["sender"]; s != nil {
		t.Errorf("outgoing has a sender: %v", s)
	}
	p = e.props("admin", "private_messages", "f", `[{"field":"is_outgoing","op":"is","value":"yes"}]`)
	wantList(t, "outgoing", p, 2, "2,6")

	// Private messages do not exist for a member: the not_found page.
	st, b := e.get("member", "/api/page/private_messages")
	if st != 404 || b["route"] != "not_found" {
		t.Errorf("member private messages: %d %v", st, b["route"])
	}
}

// A message list's row carries a service message's action, from the
// stored service_type, so the list can label it; every other row has
// null.
func TestListMessagesServiceType(t *testing.T) {
	t.Parallel()
	e := newSearchEnvOpts(t, searchOpts{fixtures: []string{"list_service"}})
	for _, c := range []struct{ list, chat, want, ids string }{
		{"group_messages", "-1001000000003", "pin_message", "601,600,13,12"},
		{"private_messages", "9299000002", "contact_registered", "601,600,6,5"},
	} {
		p := e.props("superadmin", c.list, "f",
			`[{"field":"chat_id","op":"=","value":"`+c.chat+`"}]`)
		if listIDs(p) != c.ids {
			t.Fatalf("%s: %s", c.list, listIDs(p))
		}
		rows := p["rows"].([]any)
		plain, svc := rows[0].(map[string]any), rows[1].(map[string]any)
		if v, ok := svc["service_type"]; !ok || v != c.want || svc["content_type"] != "service" {
			t.Errorf("%s service row: %v", c.list, svc)
		}
		if v, ok := plain["service_type"]; !ok || v != nil {
			t.Errorf("%s plain row: %v", c.list, plain)
		}
		for _, r := range rows[2:] {
			if r := r.(map[string]any); r["service_type"] != nil {
				t.Errorf("%s row %v: %v", c.list, r["id"], r["service_type"])
			}
		}
	}
}

// A video or animated sticker's bytes are no image, so it has no
// thumbnail: an <img> of a .webm showed a broken picture in the grid.
func TestFileThumbOnlyImages(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		stored        bool
		fileType, ext string
		want          bool
	}{
		{true, "photo", "jpg", true},
		{true, "sticker", "webp", true},
		{true, "sticker", "webm", false},
		{true, "sticker", "WEBM", false},
		{true, "sticker", "tgs", false},
		{false, "photo", "jpg", false},
		{true, "video", "mp4", false},
		{true, "animation", "mp4", false},
	} {
		if got := hasThumb(c.stored, c.fileType, c.ext); got != c.want {
			t.Errorf("hasThumb(%v, %s, %s) = %v, want %v", c.stored, c.fileType, c.ext, got, c.want)
		}
	}
}

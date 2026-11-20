// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// The sample as a member granted Linux Kernel Indonesia and Rust
// Programming sees it (web-member-access): those two groups, their
// messages (rows 9 - 13) and files (880006, 880003), and the people
// who appear in them: Dewi (a sender and an admin in Kernel), Putri
// (a sender in Kernel) and Andi (a sender in Rust). Nothing else.
const (
	memberUID  = 5 // adminEnv's "member"
	kernelChat = int64(-1001000000002)
)

var (
	memberGroups   = []int64{kernelChat, rustChat}
	hiddenGroups   = []int64{gnuWeeb, -1001000000004, -1001000000005, -1001000000010}
	memberUsers    = []int64{9299000001, 9299000004, 9299000007}
	hiddenUsers    = []int64{9299000002, 9299000003, 9299000005, 9299000006, 9299000008}
	memberMessages = []int{9, 10, 11, 12, 13}
	hiddenMessages = []int{1, 4, 5, 8, 14, 15, 16}
	memberFiles    = []string{"880003", "880006"}
	hiddenFiles    = []string{"880001", "880002", "880004", "880005"}
)

// grantMember grants the adminEnv's member groups of Default directly
// in the web's database, and abilities.
func grantMember(t *testing.T, e *adminEnv, groups []int64, abilities ...string) {
	t.Helper()
	if _, err := e.app.Exec("DELETE FROM web_member_grant_groups WHERE web_user_id = ?", memberUID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Exec("DELETE FROM web_member_abilities WHERE web_user_id = ?", memberUID); err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if _, err := e.app.Exec("INSERT INTO web_member_grant_groups (web_user_id, federation_id, "+
			"group_id) VALUES (?, 0, ?)", memberUID, g); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range abilities {
		if _, err := e.app.Exec("INSERT INTO web_member_abilities (web_user_id, ability) VALUES (?, ?)",
			memberUID, a); err != nil {
			t.Fatal(err)
		}
	}
}

func idText(v int64) string { return fmt.Sprint(v) }

// groupHidden is whether a group page answer shows a group the viewer
// does not see: 404, or for an exposed group the public details that
// anyone gets (props.public), without its admins, history or posters.
func groupHidden(status int, page map[string]any) bool {
	if status == 404 {
		return true
	}
	props, _ := page["props"].(map[string]any)
	if status != 200 || props["public"] != true {
		return false
	}
	_, admins := props["admins"]
	_, history := props["history"]
	_, posters := props["top_posters"]
	return !admins && !history && !posters
}

// mentions reports which of ids the text names.
func mentions[T int64 | int](text string, ids []T) []T {
	var out []T
	for _, id := range ids {
		if strings.Contains(text, fmt.Sprint(id)) {
			out = append(out, id)
		}
	}
	return out
}

// A member granted two groups sees them, their people, messages and
// files, on every page, list, search, count, chat window, message
// page, user page, chat search and entity lookup; every other group,
// user, message and file answers 404 or is simply absent.
func TestMemberSeesOnlyItsGroups(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	grantMember(t, e, memberGroups)

	pageText := func(path string) string {
		t.Helper()
		st, p := e.page("member", path)
		if st != 200 {
			t.Fatalf("%s: %d %v", path, st, p)
		}
		return js(p["props"])
	}
	apiText := func(path string) string {
		t.Helper()
		st, b, _ := e.call("member", "GET", path, nil)
		if st != 200 {
			t.Fatalf("%s: %d %v", path, st, b)
		}
		return js(b)
	}
	// Each list names the granted groups and nothing hidden.
	sees := func(what, text string, want, hidden []int64) {
		t.Helper()
		if got := mentions(text, want); !slices.Equal(got, want) {
			t.Errorf("%s: names %v of %v", what, got, want)
		}
		if got := mentions(text, hidden); len(got) != 0 {
			t.Errorf("%s: names hidden %v", what, got)
		}
	}

	groups := pageText("/groups")
	sees("groups list", groups, memberGroups, hiddenGroups)
	users := pageText("/users")
	sees("users list", users, memberUsers, hiddenUsers)
	if strings.Contains(users, `"bio"`) || strings.Contains(users, `"phone"`) ||
		strings.Contains(users, `"msg_count"`) {
		t.Errorf("users list shows the full columns: %s", users)
	}
	sees("groups search", apiText("/api/search/groups"), memberGroups, hiddenGroups)
	sees("users search", apiText("/api/search/users"), memberUsers, hiddenUsers)
	// A search for a hidden person finds no one.
	if s := apiText("/api/search/users?q=Bima"); strings.Contains(s, "9299000002") {
		t.Errorf("found a hidden user: %s", s)
	}

	_, lp := e.page("member", "/group_messages")
	gids := strings.Split(listIDs(lp["props"].(map[string]any)), ",")
	sort.Strings(gids)
	if got := strings.Join(gids, ","); got != "10,11,12,13,9" {
		t.Errorf("group messages %s", got)
	}
	if n := num(lp["props"].(map[string]any)["total"]); n != 5 {
		t.Errorf("group messages total %d", n)
	}
	gm := apiText("/api/search/group_messages")
	if !strings.Contains(gm, "can someone review my patch") ||
		strings.Contains(gm, "good morning everyone") || strings.Contains(gm, "x86-64 calling") {
		t.Errorf("group messages search %s", gm)
	}

	_, fp := e.page("member", "/files")
	files := fp["props"].(map[string]any)
	ids := strings.Split(listIDs(files), ",")
	sort.Strings(ids)
	if !slices.Equal(ids, memberFiles) || num(files["total"]) != 2 {
		t.Errorf("files %v total %v", ids, files["total"])
	}
	if f := apiText("/api/search/files"); len(mentions(f, []int{880001, 880002, 880004, 880005})) != 0 {
		t.Errorf("files search names hidden files: %s", f)
	}

	sees("chat search", apiText("/api/chats/search"), memberGroups, append(hiddenGroups, 9299000001, 9299000002))
	sees("entities", apiText("/api/entities?users=9299000001,9299000002,9299000003,9299000007&groups="+
		"-1001000000001,-1001000000002,-1001000000005"), []int64{9299000001, 9299000007, kernelChat},
		[]int64{9299000002, 9299000003, gnuWeeb, -1001000000005})

	// The dashboard counts and lists only the granted groups.
	dash := pageText("/")
	sees("dashboard", dash, nil, hiddenGroups)
	if strings.Contains(dash, `"private_messages"`) || strings.Contains(dash, `"files"`) {
		t.Errorf("dashboard shows private or file totals: %s", dash)
	}
	for _, k := range []struct {
		key  string
		want float64
	}{{"groups", 2}, {"group_messages", 5}, {"users", 3}} {
		_, p := e.page("member", "/")
		found := false
		for _, kp := range p["props"].(map[string]any)["kpis"].([]any) {
			m := kp.(map[string]any)
			if m["key"] == k.key {
				found = true
				if m["total"] != k.want {
					t.Errorf("dashboard %s total %v, want %v", k.key, m["total"], k.want)
				}
			}
		}
		if !found {
			t.Errorf("dashboard has no %s", k.key)
		}
	}

	// Pages and APIs of what is granted open; everything else is 404.
	ok := func(what string, st int) {
		t.Helper()
		if st != 200 {
			t.Errorf("%s: %d, want 200", what, st)
		}
	}
	hidden := func(what string, st int) {
		t.Helper()
		if st != 404 {
			t.Errorf("%s: %d, want 404", what, st)
		}
	}
	pageSt := func(path string) int { st, _ := e.page("member", path); return st }
	apiSt := func(path string) int { st, _, _ := e.call("member", "GET", path, nil); return st }
	for _, g := range memberGroups {
		id := idText(g)
		ok("group page "+id, pageSt("/groups/"+id))
		ok("group admins "+id, pageSt("/groups/"+id+"/admins"))
		ok("group chat "+id, pageSt("/groups/"+id+"/chat"))
		ok("chat window "+id, apiSt("/api/chats/group/"+id+"/messages"))
		ok("group history "+id, apiSt("/api/groups/"+id+"/history"))
		ok("group photos "+id, apiSt("/api/groups/"+id+"/photos"))
	}
	for _, g := range hiddenGroups {
		id := idText(g)
		if st, p := e.page("member", "/groups/"+id); !groupHidden(st, p) {
			t.Errorf("group page %s: %d %v, want 404 or the public page", id, st, p)
		}
		hidden("group history "+id, apiSt("/api/groups/"+id+"/history"))
		hidden("group photos "+id, apiSt("/api/groups/"+id+"/photos"))
	}
	// Exposed groups stay public: their chat is reachable signed out,
	// so a member reaches it as well. A hidden one that is not
	// exposed answers 404.
	for _, g := range []int64{-1001000000004, -1001000000005} {
		hidden("group chat", pageSt("/groups/"+idText(g)+"/chat"))
		hidden("chat window", apiSt("/api/chats/group/"+idText(g)+"/messages"))
	}
	for _, m := range memberMessages {
		ok(fmt.Sprint("message ", m), pageSt(fmt.Sprint("/messages/group/", m)))
	}
	for _, m := range hiddenMessages {
		hidden(fmt.Sprint("message ", m), pageSt(fmt.Sprint("/messages/group/", m)))
	}
	hidden("private message", pageSt("/messages/private/1"))
	hidden("private messages list", pageSt("/private_messages"))
	hidden("private search", apiSt("/api/search/private_messages"))
	hidden("private chat", pageSt("/users/9299000001/chat"))
	hidden("private chat window", apiSt("/api/chats/private/9299000001/messages"))

	// A user in the groups: the limited page, its groups only those
	// granted; no history, photos or other groups.
	for _, u := range memberUsers {
		st, p := e.page("member", "/users/"+idText(u))
		ok(fmt.Sprint("user ", u), st)
		props := p["props"].(map[string]any)
		if props["limited"] != true {
			t.Errorf("user %d: not the limited page: %v", u, props)
		}
		if got := mentions(js(props), hiddenGroups); len(got) != 0 {
			t.Errorf("user %d: names hidden groups %v", u, got)
		}
		if s := js(props); strings.Contains(s, "Kernel hacker") || strings.Contains(s, "+62") ||
			strings.Contains(s, "msg_count") {
			t.Errorf("user %d: shows more than the limited view: %s", u, s)
		}
		hidden(fmt.Sprint("user history ", u), pageSt("/users/"+idText(u)+"/history"))
		hidden(fmt.Sprint("user history api ", u), apiSt("/api/users/"+idText(u)+"/history"))
		hidden(fmt.Sprint("user photos ", u), apiSt("/api/users/"+idText(u)+"/photos"))
	}
	for _, u := range hiddenUsers {
		hidden(fmt.Sprint("user ", u), pageSt("/users/"+idText(u)))
	}
	// Dewi's shared groups are Kernel only (she is in GNU/Weeb too).
	_, dp := e.page("member", "/users/9299000001")
	if got := mentions(js(dp["props"]), memberGroups); !slices.Equal(got, []int64{kernelChat}) {
		t.Errorf("Dewi's groups %v", got)
	}

	// A chat window names only its own people, without their counts.
	cw := apiText("/api/chats/group/" + idText(kernelChat) + "/messages")
	if strings.Contains(cw, "msg_count") || strings.Contains(cw, "contact_name") {
		t.Errorf("chat window shows counts: %s", cw)
	}
	// Reachable chats of the member only: the federation and
	// administration pages do not exist for it.
	for _, p := range []string{"/settings/federations", "/settings/federations/0",
		"/settings/accounts", "/mcp-admin", "/mcp-admin/groups", "/join-chat", "/platform-fwd"} {
		hidden(p, pageSt(p))
	}
	for _, p := range []string{"/api/federations/0", "/api/federations/0/groups", "/api/accounts", "/api/bots"} {
		hidden(p, apiSt(p))
	}
}

// A member with no grants sees nothing: no group, person, message or
// file, and empty totals.
func TestMemberWithoutGrantsSeesNothing(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	all := append(append([]int64{}, memberGroups...), hiddenGroups...)
	people := append(append([]int64{}, memberUsers...), hiddenUsers...)
	for _, path := range []string{"/groups", "/users", "/group_messages", "/files", "/"} {
		st, p := e.page("member", path)
		if st != 200 {
			t.Fatalf("%s: %d", path, st)
		}
		s := js(p["props"])
		if got := mentions(s, all); len(got) != 0 {
			t.Errorf("%s names groups %v", path, got)
		}
		if got := mentions(s, people); len(got) != 0 {
			t.Errorf("%s names people %v", path, got)
		}
		if t2, ok := p["props"].(map[string]any)["total"]; ok && num(t2) != 0 {
			t.Errorf("%s total %v", path, t2)
		}
	}
	for _, path := range []string{"/groups/" + idText(kernelChat), "/users/9299000001", "/messages/group/9"} {
		if st, _ := e.page("member", path); st != 404 {
			t.Errorf("%s: %d", path, st)
		}
	}
	if st, b, _ := e.call("member", "GET", "/api/chats/search", nil); st != 200 || len(mentions(js(b), all)) != 0 {
		t.Errorf("chat search %d %v", st, b)
	}
}

// fetch_files gates having an unstored file fetched: without it a
// member gets no link to one and its link does not reach the daemon;
// with it, both work. The stored file's link works either way.
func TestMemberFetchFiles(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	grantMember(t, e, memberGroups)

	video := func() map[string]any {
		t.Helper()
		_, b, _ := e.call("member", "GET", "/api/chats/group/"+idText(rustChat)+"/messages", nil)
		for _, m := range b["messages"].([]any) {
			mm := m.(map[string]any)
			if mm["id"] == float64(13) {
				return mm["media"].(map[string]any)
			}
		}
		t.Fatalf("no video message: %v", b)
		return nil
	}
	if m := video(); m["fetchable"] != false || m["url"] != nil {
		t.Errorf("without fetch_files: %v", m)
	}
	_, fp := e.page("member", "/files")
	if fp["props"].(map[string]any)["can_fetch"] != false {
		t.Errorf("files page offers fetching: %v", fp["props"])
	}
	// A link it holds anyway (minted for someone else) fetches nothing.
	token := e.deps.Tokens.Encrypt(880003)
	get := func() int {
		req := httptest.NewRequest("GET", "/files/"+token, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: e.cookies["member"]})
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		_, _ = io.Copy(io.Discard, rec.Body)
		return rec.Code
	}
	if st := get(); st != 404 {
		t.Errorf("fetch without fetch_files: %d", st)
	}
	for _, c := range e.daemon.calls() {
		if strings.Contains(c, "/v1/files/880003") {
			t.Errorf("the daemon was asked: %s", c)
		}
	}

	grantMember(t, e, memberGroups, "fetch_files")
	if m := video(); m["fetchable"] != true || m["url"] == nil {
		t.Errorf("with fetch_files: %v", m)
	}
	_, fp = e.page("member", "/files")
	if fp["props"].(map[string]any)["can_fetch"] != true {
		t.Errorf("files page does not offer fetching: %v", fp["props"])
	}
	get()
	asked := false
	for _, c := range e.daemon.calls() {
		asked = asked || strings.Contains(c, "/v1/files/880003")
	}
	if !asked {
		t.Errorf("the daemon was not asked: %v", e.daemon.calls())
	}
}

// forward opens the forwarding pages to a member, which there sees and
// changes only its own configurations, of its own groups.
func TestMemberForward(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	grantMember(t, e, memberGroups)
	if st, _ := e.page("member", "/platform-fwd"); st != 404 {
		t.Fatalf("forwarding without forward: %d", st)
	}
	create := map[string]any{"chat_id": kernelChat, "chat_kind": "group", "webhook_url": newHook,
		"types": []string{"text"}, "enabled": true}
	if st, b, _ := e.call("member", "POST", t2dBase, create); st != 404 {
		t.Fatalf("create without forward: %d %v", st, b)
	}

	grantMember(t, e, memberGroups, "forward")
	if st, _ := e.page("member", "/platform-fwd"); st != 200 {
		t.Fatalf("forwarding with forward: %d", st)
	}
	// The fixture's webhook 3 is on Rust, a granted group, but not the
	// member's: it does not exist for it.
	routes := func() string {
		t.Helper()
		_, p := e.page("member", "/platform-fwd/telegram-discord")
		return js(p["props"].(map[string]any)["routes"])
	}
	if r := routes(); r != "[]" {
		t.Errorf("sees others' webhooks: %s", r)
	}
	if st, _, _ := e.call("member", "PATCH", t2dBase+"/3", map[string]any{"enabled": true}); st != 404 {
		t.Errorf("patch another's webhook: %d", st)
	}
	if st, _, _ := e.call("member", "DELETE", t2dBase+"/3", nil); st != 404 {
		t.Errorf("delete another's webhook: %d", st)
	}
	if st, _, _ := e.call("member", "POST", t2dBase+"/test", map[string]any{"id": 3}); st != 404 {
		t.Errorf("test another's webhook: %d", st)
	}
	_, ip := e.page("member", "/platform-fwd")
	if s := js(ip["props"]); s != `{"d2t":{"active":0,"sent":0,"total":0},"t2d":{"active":0,"sent":0,"total":0}}` {
		t.Errorf("index counts others' configurations: %s", s)
	}
	// Not in a group it does not see.
	hide := map[string]any{"chat_id": gnuWeeb, "chat_kind": "group", "webhook_url": newHook,
		"types": []string{"text"}, "enabled": true}
	if st, _, _ := e.call("member", "POST", t2dBase, hide); st != 404 {
		t.Errorf("create in a hidden group: %d", st)
	}
	b := e.want(201, "member", "POST", t2dBase, create)
	id := fmt.Sprint(b["id"])
	if r := routes(); !strings.Contains(r, `"id":`+id) || strings.Contains(r, `"id":3,`) {
		t.Errorf("own webhooks %s", r)
	}
	e.want(204, "member", "PATCH", t2dBase+"/"+id, map[string]any{"enabled": false})
	// The admin still sees both.
	_, ap := e.page("admin", "/platform-fwd/telegram-discord")
	if r := js(ap["props"]); !strings.Contains(r, `"id":`+id) || !strings.Contains(r, `"id":3,`) {
		t.Errorf("admin's webhooks %s", r)
	}
	e.want(204, "member", "DELETE", t2dBase+"/"+id, nil)
}

// The accounts API stores a member's grants and abilities
// (PATCH /api/accounts/{id}): the groups of a grant must be its
// federation's, an admin changes only the federations it administers,
// and grants go when their group leaves the federation, when the
// member leaves the federation, and when the account stops being a
// member. Every change is audited.
func TestMemberGrantsAPI(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	path := fmt.Sprintf("/api/accounts/%d", memberUID)
	grant := func(fed any, groups ...int64) map[string]any {
		if groups == nil {
			groups = []int64{}
		}
		return map[string]any{"fed": fed, "group_ids": groups}
	}
	patch := func(role string, body map[string]any) (int, map[string]any) {
		t.Helper()
		st, b, _ := e.call(role, "PATCH", path, body)
		return st, b
	}
	granted := func(b map[string]any) string {
		t.Helper()
		var out []string
		for _, g := range b["grants"].([]any) {
			gm := g.(map[string]any)
			var ids []string
			for _, p := range gm["groups"].([]any) {
				ids = append(ids, fmt.Sprint(num(p.(map[string]any)["id"])))
			}
			out = append(out, fmt.Sprintf("%v:%s", gm["fed"], strings.Join(ids, ",")))
		}
		return strings.Join(out, " ")
	}
	sees := func(g int64) bool {
		st, p := e.page("member", "/groups/"+idText(g))
		return st == 200 && !groupHidden(st, p)
	}

	_, ap := e.page("superadmin", "/settings/accounts")
	if ed := ap["props"].(map[string]any)["editable"].(map[string]any); ed["group_grants"] != true ||
		ed["abilities"] != true {
		t.Errorf("editable %v", ed)
	}

	st, b := patch("superadmin", map[string]any{"grants": []any{grant(0, kernelChat, rustChat)},
		"abilities": []string{"forward"}})
	if st != 200 || granted(b) != fmt.Sprintf("0:%d,%d", rustChat, kernelChat) ||
		js(b["abilities"]) != `["forward"]` {
		t.Fatalf("grant: %d %v", st, b)
	}
	if g := b["grants"].([]any)[0].(map[string]any)["groups"].([]any)[1].(map[string]any); g["title"] != "Linux Kernel Indonesia" ||
		g["kind"] != "group" {
		t.Errorf("pick item %v", g)
	}
	if !sees(kernelChat) || !sees(rustChat) || sees(gnuWeeb) {
		t.Error("the member does not see what it was granted")
	}
	if st, _ := e.page("member", "/platform-fwd"); st != 200 {
		t.Errorf("forward: %d", st)
	}

	// Refusals: a group outside the federation, a federation that does
	// not exist, an ability members cannot have, abilities for an admin.
	fb := e.want(201, "superadmin", "POST", "/api/federations",
		map[string]any{"name": "Kernel", "group_ids": []int64{kernelChat}})
	kern := uint64(fb["id"].(float64))
	// Kernel left Default when it joined the federation: so did its grant.
	if sees(kernelChat) || !sees(rustChat) {
		t.Error("a group that left Default stays granted there")
	}
	for _, tc := range []struct {
		name  string
		body  map[string]any
		code  string
		field string
	}{
		{"group of another federation", map[string]any{"grants": []any{grant(0, kernelChat)}}, "bad_request", "grants"},
		{"group not in the federation", map[string]any{"grants": []any{grant(kern, rustChat)}}, "bad_request", "grants"},
		{"group the archive lacks", map[string]any{"grants": []any{grant(0, -1009999999999)}}, "bad_request", "grants"},
		{"no such federation", map[string]any{"grants": []any{grant(99, rustChat)}}, "bad_federation", ""},
		{"unknown ability", map[string]any{"abilities": []string{"join_chat"}}, "bad_request", "abilities"},
	} {
		if st, b := patch("superadmin", tc.body); st != 400 || b["error"] != tc.code ||
			(tc.field != "" && b["field"] != tc.field) {
			t.Errorf("%s: %d %v", tc.name, st, b)
		}
	}

	// A superadmin files the member in Kernel with its group.
	st, b = patch("superadmin", map[string]any{"grants": []any{grant(0, rustChat), grant(kern, kernelChat)}})
	if st != 200 || granted(b) != fmt.Sprintf("0:%d %d:%d", rustChat, kern, kernelChat) {
		t.Fatalf("two federations: %d %v", st, b)
	}
	if !sees(kernelChat) || !sees(rustChat) {
		t.Error("grants in two federations")
	}
	// The admin of Default sends the member's whole list back with a
	// change in Default: Kernel's grant stays as it was. Naming
	// Kernel's groups changes nothing there; adding a federation it
	// does not administer is 403.
	st, b = patch("admin", map[string]any{"grants": []any{grant(0), grant(kern)}})
	// The admin is shown no group of Kernel, which it does not
	// administer.
	if st != 200 || granted(b) != fmt.Sprintf("0: %d:", kern) {
		t.Fatalf("admin's change: %d %v", st, b)
	}
	if sees(rustChat) || !sees(kernelChat) {
		t.Error("after the admin's change")
	}
	if st, b := patch("admin", map[string]any{"grants": []any{grant(0), grant(kern), grant(99)}}); st != 403 {
		t.Errorf("admin names a federation it does not administer: %d %v", st, b)
	}
	if st, b := patch("superadmin", map[string]any{"role": "admin", "abilities": []string{"forward"}}); st != 400 {
		t.Errorf("abilities for an admin: %d %v", st, b)
	}

	// The group leaves Kernel: its grant goes.
	e.want(200, "superadmin", "PATCH", fmt.Sprintf("/api/federations/%d", kern),
		map[string]any{"group_ids": []int64{}})
	_, b, _ = e.call("superadmin", "GET", path, nil)
	if granted(b) != fmt.Sprintf("0: %d:", kern) || sees(kernelChat) {
		t.Errorf("after the group left the federation: %v", b["grants"])
	}
	e.want(200, "superadmin", "PATCH", fmt.Sprintf("/api/federations/%d", kern),
		map[string]any{"group_ids": []int64{kernelChat}})
	_, b = patch("superadmin", map[string]any{"grants": []any{grant(0, rustChat), grant(kern, kernelChat)}})
	// The member leaves Kernel: its grant there goes.
	st, b = patch("superadmin", map[string]any{"grants": []any{grant(0, rustChat)}})
	if st != 200 || granted(b) != fmt.Sprintf("0:%d", rustChat) || sees(kernelChat) {
		t.Errorf("after leaving the federation: %d %v", st, b["grants"])
	}
	if n := e.appOne("SELECT COUNT(*) FROM web_member_grant_groups WHERE web_user_id = ? AND federation_id = ?",
		memberUID, kern); n != "0" {
		t.Errorf("%s grants left in the federation", n)
	}
	// A role change away from member drops grants and abilities; back
	// as a member it is in Default with nothing granted.
	e.want(200, "superadmin", "PATCH", path, map[string]any{"role": "admin"})
	b = e.want(200, "superadmin", "PATCH", path, map[string]any{"role": "member"})
	if granted(b) != "0:" || js(b["abilities"]) != "[]" {
		t.Errorf("after admin and back: %v %v", b["grants"], b["abilities"])
	}
	if n := e.appOne("SELECT COUNT(*) FROM web_member_grant_groups WHERE web_user_id = ?", memberUID) +
		e.appOne("SELECT COUNT(*) FROM web_member_abilities WHERE web_user_id = ?", memberUID); n != "00" {
		t.Errorf("rows left after the role change: %s", n)
	}
	if sees(rustChat) {
		t.Error("sees a group after the role change")
	}

	// Creating a member with groups in its grant.
	cb := e.want(201, "admin", "POST", "/api/accounts", map[string]any{"username": "granted",
		"name": "Granted", "role": "member", "grants": []any{grant(0, rustChat)}})
	_, b, _ = e.call("admin", "GET", fmt.Sprintf("/api/accounts/%v", cb["id"]), nil)
	if granted(b) != fmt.Sprintf("0:%d", rustChat) {
		t.Errorf("created with grants: %v", b["grants"])
	}

	if !auditHas(e, []string{
		fmt.Sprintf("grants 0: none->%d,%d", rustChat, kernelChat),
		"abilities none->forward",
		fmt.Sprintf("grants %d: none->%d", kern, kernelChat),
		"abilities forward->none",
	}) {
		t.Errorf("audit %v", e.audits())
	}
}

// mcp_tokens is a member ability like the others: an admin managing
// the member gives it, it is stored and read back, and it reaches the
// member's account.
func TestMemberMCPTokensAbility(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	path := fmt.Sprintf("/api/accounts/%d", memberUID)
	b := e.want(200, "admin", "PATCH", path, map[string]any{"abilities": []string{"mcp_tokens", "forward"}})
	if js(b["abilities"]) != `["forward","mcp_tokens"]` {
		t.Fatalf("abilities %v", b["abilities"])
	}
	if !auditHas(e, []string{"abilities none->forward,mcp_tokens"}) {
		t.Errorf("audit %v", e.audits())
	}
	if got := e.appOne("SELECT GROUP_CONCAT(ability ORDER BY ability) FROM web_member_abilities "+
		"WHERE web_user_id = ?", memberUID); got != "forward,mcp_tokens" {
		t.Errorf("stored %s", got)
	}
	b = e.want(200, "superadmin", "PATCH", path, map[string]any{"abilities": []string{}})
	if js(b["abilities"]) != `[]` {
		t.Errorf("abilities after removing %v", b["abilities"])
	}
}

// An admin sees, in a member's row, only the granted groups it may see
// and change: none hidden from it (by title, id or audit line) and none
// of a federation it does not administer. Its PATCH from that row keeps
// the grants it was not shown; a superadmin sees and changes them all.
func TestMemberGrantsHiddenFromAdmin(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	path := fmt.Sprintf("/api/accounts/%d", memberUID)
	grant := func(fed any, groups ...int64) map[string]any {
		return map[string]any{"fed": fed, "group_ids": append([]int64{}, groups...)}
	}
	fb := e.want(201, "superadmin", "POST", "/api/federations",
		map[string]any{"name": "CTF", "group_ids": []int64{-1001000000010}})
	ctf := fb["id"]
	// Rust and Assembly Lounge are hidden from the admin, which
	// administers Default only.
	e.want(200, "superadmin", "PATCH", "/api/accounts/3", map[string]any{"restrict": map[string]any{
		"hide_private": false, "hide_groups": []int64{rustChat, -1001000000005}, "join_chat": false}})
	b := e.want(200, "superadmin", "PATCH", path, map[string]any{"grants": []any{
		grant(0, kernelChat, rustChat), grant(ctf, -1001000000010)}})
	full := js(b)
	for _, want := range []string{"Rust Programming", "CTF Practice", "Linux Kernel Indonesia"} {
		if !strings.Contains(full, want) {
			t.Errorf("superadmin's row lacks %s: %s", want, full)
		}
	}

	rowOf := func(role string) string {
		t.Helper()
		st, b, _ := e.call(role, "GET", path, nil)
		if st != 200 {
			t.Fatalf("%s GET: %d %v", role, st, b)
		}
		return js(b)
	}
	listed := func(role string) string {
		t.Helper()
		_, b, _ := e.call(role, "GET", "/api/accounts", nil)
		return js(b)
	}
	for what, s := range map[string]string{"row": rowOf("admin"), "list": listed("admin")} {
		if strings.Contains(s, "Rust Programming") || strings.Contains(s, idText(rustChat)) {
			t.Errorf("admin's %s names the hidden group: %s", what, s)
		}
		if strings.Contains(s, "CTF Practice") || strings.Contains(s, "-1001000000010") {
			t.Errorf("admin's %s names another federation's group: %s", what, s)
		}
		if !strings.Contains(s, "Linux Kernel Indonesia") {
			t.Errorf("admin's %s lacks its own grant: %s", what, s)
		}
	}

	// The admin sends back what it was shown, and removes nothing it
	// could not see; a hidden group it names anyway is refused.
	e.want(200, "admin", "PATCH", path, map[string]any{"grants": []any{grant(0, kernelChat), grant(ctf)}})
	if st, b, _ := e.call("admin", "PATCH", path, map[string]any{"grants": []any{grant(0)}}); st != 200 {
		t.Fatalf("admin clears its grants: %d %v", st, b)
	}
	if got := e.appOne("SELECT GROUP_CONCAT(CONCAT(federation_id, ':', group_id) ORDER BY federation_id, group_id) "+
		"FROM web_member_grant_groups WHERE web_user_id = ?", memberUID); got != fmt.Sprintf("0:%d,%v:-1001000000010", rustChat, ctf) {
		t.Errorf("grants after the admin's changes: %s", got)
	}
	if st, b, _ := e.call("admin", "PATCH", path, map[string]any{"grants": []any{grant(0, -1001000000005)}}); st != 400 ||
		b["field"] != "grants" {
		t.Errorf("admin grants a hidden group: %d %v", st, b)
	}

	// The superadmin sees them all, and removes the hidden one.
	if s := rowOf("superadmin"); !strings.Contains(s, "Rust Programming") || !strings.Contains(s, "CTF Practice") {
		t.Errorf("superadmin's row %s", s)
	}
	e.want(200, "superadmin", "PATCH", path, map[string]any{"grants": []any{grant(0), grant(ctf, -1001000000010)}})
	if got := e.appOne("SELECT COUNT(*) FROM web_member_grant_groups WHERE web_user_id = ? AND federation_id = 0",
		memberUID); got != "0" {
		t.Errorf("superadmin's removal left %s", got)
	}
}

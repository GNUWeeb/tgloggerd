// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"slices"
	"testing"
)

// A superadmin grants an admin Join chat (restrict.join_chat,
// web-admin-restrictions): from the next request on the page, its menu
// entry and both operations are the admin's; without the grant they
// answer 404, an admin cannot grant it, a member cannot hold it, and a
// role change away from admin withdraws it.
func TestAdminJoinChatGrant(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer("/v1/chats/resolve", 200, "", previewChat)
	e.daemon.answer("/v1/chats/join", 200, "", previewChat)
	const adminPath = "/api/accounts/3"
	grant := func(on bool) map[string]any {
		return map[string]any{"restrict": map[string]any{"hide_private": false, "hide_groups": []any{}, "join_chat": on}}
	}
	ref := map[string]any{"ref": "@gnuweeb"}

	// sees reports, for each surface, whether role may join chats.
	sees := func(role string) map[string]bool {
		_, p := e.page(role, "/")
		var nav []string
		for _, k := range p["viewer"].(map[string]any)["nav"].([]any) {
			nav = append(nav, k.(string))
		}
		pst, _ := e.page(role, "/join-chat")
		prev, _, _ := e.call(role, "POST", "/api/join-chat/preview", ref)
		join, _, _ := e.call(role, "POST", "/api/join-chat/join", ref)
		return map[string]bool{"menu": slices.Contains(nav, "join_chat"), "page": pst == 200,
			"preview": prev == 200, "join": join == 200}
	}
	expect := func(when, role string, want bool) {
		t.Helper()
		for k, got := range sees(role) {
			if got != want {
				t.Errorf("%s: %s: %s %v, want %v", when, role, k, got, want)
			}
		}
	}

	expect("default", "admin", false)
	expect("default", "member", false)
	expect("default", "superadmin", true)

	// Only a superadmin grants it, and only to an admin.
	if st, b, _ := e.call("admin", "PATCH", adminPath, grant(true)); st != 403 && st != 404 {
		t.Errorf("by the admin itself: %d %v", st, b)
	}
	if st, b, _ := e.call("superadmin", "PATCH", "/api/accounts/5", grant(true)); st != 400 || b["error"] != "bad_request" {
		t.Errorf("a member: %d %v", st, b)
	}
	if st, b, _ := e.call("superadmin", "PATCH", "/api/accounts/1", grant(true)); st != 400 || b["error"] != "bad_request" {
		t.Errorf("a superadmin: %d %v", st, b)
	}
	expect("after refusals", "admin", false)

	b := e.want(200, "superadmin", "PATCH", adminPath, grant(true))
	if r, _ := b["restrict"].(map[string]any); r == nil || r["join_chat"] != true {
		t.Fatalf("answer %v", b)
	}
	expect("granted", "admin", true)
	expect("granted to the admin", "member", false)
	// The accounts page shows the grant and offers every restriction.
	_, p := e.page("superadmin", "/settings/accounts")
	props := p["props"].(map[string]any)
	if ed := props["editable"].(map[string]any); ed["restrictions"] != true {
		t.Errorf("editable %v", ed)
	}
	for _, a := range props["accounts"].([]any) {
		a := a.(map[string]any)
		if a["id"] == 3.0 && a["restrict"].(map[string]any)["join_chat"] != true {
			t.Errorf("row %v", a)
		}
	}
	// The granted admin files a joined group into its own federations
	// only.
	if _, p := e.page("admin", "/join-chat"); js(p["props"].(map[string]any)["federations"]) !=
		`[{"id":0,"name":"Default"}]` {
		t.Errorf("admin's federations %s", js(p["props"]))
	}

	e.want(200, "superadmin", "PATCH", adminPath, grant(false))
	expect("withdrawn", "admin", false)

	// A role change away from admin withdraws it.
	e.want(200, "superadmin", "PATCH", adminPath, grant(true))
	e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "member"})
	b = e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "admin"})
	if r := b["restrict"].(map[string]any); r["join_chat"] != false {
		t.Errorf("after member and back: %v", r)
	}
	if want := []string{"join_chat on", "join_chat off", "role admin->member; join_chat off"}; !auditHas(e, want) {
		t.Errorf("audit %v, want entries %v", e.audits(), want)
	}
}

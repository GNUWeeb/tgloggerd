// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"slices"
	"strings"
	"testing"
)

// A superadmin hides private chats from an admin (restrict.hide_private,
// web-admin-restrictions): from the next request on, every private page,
// chat window, search and count answers 404 or leaves them out, and
// lifting it brings them back.
func TestAdminHidePrivate(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	const peer = "9299000001" // the sample's private chat
	adminPath := "/api/accounts/3"

	type check struct {
		name string
		see  func() bool
	}
	nav := func() []string {
		_, p := e.page("admin", "/")
		var out []string
		for _, k := range p["viewer"].(map[string]any)["nav"].([]any) {
			out = append(out, k.(string))
		}
		return out
	}
	status := func(path string) int {
		st, _, _ := e.call("admin", "GET", path, nil)
		return st
	}
	pageStatus := func(path string) int {
		st, _ := e.page("admin", path)
		return st
	}
	kpis := func() string {
		_, p := e.page("admin", "/")
		return js(p["props"].(map[string]any)["kpis"])
	}
	checks := []check{
		{"menu", func() bool { return slices.Contains(nav(), "private_messages") }},
		{"list page", func() bool { return pageStatus("/private_messages") == 200 }},
		{"chat page", func() bool { return pageStatus("/users/"+peer+"/chat") == 200 }},
		{"message page", func() bool { return pageStatus("/messages/private/1") == 200 }},
		{"chat window", func() bool { return status("/api/chats/private/"+peer+"/messages") == 200 }},
		{"search", func() bool { return status("/api/search/private_messages") == 200 }},
		{"user page", func() bool {
			_, p := e.page("admin", "/users/"+peer)
			return p["props"].(map[string]any)["can_private"] == true
		}},
		{"dashboard", func() bool { return strings.Contains(kpis(), "private") }},
	}
	expect := func(when string, want bool) {
		t.Helper()
		for _, c := range checks {
			if got := c.see(); got != want {
				t.Errorf("%s: %s: sees private %v, want %v", when, c.name, got, want)
			}
		}
	}

	expect("unrestricted", true)
	restrict := func(hide bool) map[string]any {
		return map[string]any{"restrict": map[string]any{"hide_private": hide, "hide_groups": []any{}, "join_chat": false}}
	}

	// Only a superadmin restricts, only an admin, and join_chat is not
	// built.
	// An admin does not manage admins: the account is not found.
	if st, b, _ := e.call("admin", "PATCH", adminPath, restrict(true)); st != 403 && st != 404 {
		t.Errorf("by an admin: %d %v", st, b)
	}
	if st, b, _ := e.call("superadmin", "PATCH", "/api/accounts/5", restrict(true)); st != 400 || b["error"] != "bad_request" {
		t.Errorf("a member: %d %v", st, b)
	}
	for _, body := range []map[string]any{
		{"restrict": map[string]any{"hide_private": false, "hide_groups": []any{}, "join_chat": true}},
	} {
		if st, b, _ := e.call("superadmin", "PATCH", adminPath, body); st != 400 || b["error"] != "not_supported" {
			t.Errorf("%v: %d %v", body, st, b)
		}
	}
	expect("after refusals", true)

	b := e.want(200, "superadmin", "PATCH", adminPath, restrict(true))
	if r, _ := b["restrict"].(map[string]any); r == nil || r["hide_private"] != true {
		t.Fatalf("answer %v", b)
	}
	expect("hidden", false)
	// The accounts page shows it, and offers it.
	_, p := e.page("superadmin", "/settings/accounts")
	props := p["props"].(map[string]any)
	if ed := props["editable"].(map[string]any); ed["restrict_private"] != true || ed["restrictions"] != false {
		t.Errorf("editable %v", ed)
	}
	for _, a := range props["accounts"].([]any) {
		a := a.(map[string]any)
		if a["id"] == 3.0 && a["restrict"].(map[string]any)["hide_private"] != true {
			t.Errorf("row %v", a)
		}
	}
	// The superadmin itself still sees everything.
	if st, _ := e.page("superadmin", "/private_messages"); st != 200 {
		t.Errorf("superadmin list page %d", st)
	}

	e.want(200, "superadmin", "PATCH", adminPath, restrict(false))
	expect("lifted", true)

	// A role change away from admin clears it.
	e.want(200, "superadmin", "PATCH", adminPath, restrict(true))
	e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "member"})
	b = e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "admin"})
	if r := b["restrict"].(map[string]any); r["hide_private"] != false {
		t.Errorf("after member and back: %v", r)
	}
	if want := []string{"hide_private on", "role admin->member; hide_private off"}; !auditHas(e, want) {
		t.Errorf("audit %v, want entries %v", e.audits(), want)
	}
}

// auditHas is whether every detail in want appears in the account
// audit entries of the run.
func auditHas(e *adminEnv, want []string) bool {
	got := strings.Join(e.audits(), "\n")
	for _, w := range want {
		if !strings.Contains(got, w) {
			return false
		}
	}
	return true
}

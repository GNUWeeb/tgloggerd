// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// kernelID is the sample's "Linux Kernel Indonesia": not exposed, three
// messages, 9299000007 among its senders.
const kernelID = -1001000000002

// A superadmin hides a group from an admin (restrict.hide_groups,
// web-admin-restrictions): from the next request on it is in no list,
// search, count, chat search, dashboard or user page, its pages and
// APIs answer 404, and lifting it brings it all back.
func TestAdminHideGroups(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	gid := fmt.Sprint(kernelID)
	adminPath := "/api/accounts/3"

	pageJSON := func(who, path string) (int, string) {
		st, p := e.page(who, path)
		return st, js(p)
	}
	total := func(who string) float64 {
		_, p := e.page(who, "/group_messages")
		return p["props"].(map[string]any)["total"].(float64)
	}
	type check struct {
		name string
		see  func() bool
	}
	checks := []check{
		{"groups list", func() bool { _, b := pageJSON("admin", "/groups"); return strings.Contains(b, gid) }},
		{"group page", func() bool { st, _ := e.page("admin", "/groups/"+gid); return st == 200 }},
		{"chat page", func() bool { st, _ := e.page("admin", "/groups/"+gid+"/chat"); return st == 200 }},
		{"chat window", func() bool {
			st, _, _ := e.call("admin", "GET", "/api/chats/group/"+gid+"/messages", nil)
			return st == 200
		}},
		{"message page", func() bool { st, _ := e.page("admin", "/messages/group/9"); return st == 200 }},
		{"message count", func() bool { return total("admin") == total("superadmin") }},
		{"chat search", func() bool {
			_, b, _ := e.call("admin", "GET", "/api/chats/search?q=Kernel", nil)
			return strings.Contains(js(b), gid)
		}},
		{"dashboard", func() bool { _, b := pageJSON("admin", "/"); return strings.Contains(b, gid) }},
		{"user page", func() bool {
			_, p := e.page("admin", "/users/9299000007")
			return strings.Contains(js(p["props"].(map[string]any)["groups"]), gid)
		}},
	}
	expect := func(when string, want bool) {
		t.Helper()
		for _, c := range checks {
			if got := c.see(); got != want {
				t.Errorf("%s: %s: sees the group %v, want %v", when, c.name, got, want)
			}
		}
	}
	restrict := func(groups ...int64) map[string]any {
		ids := []any{}
		for _, g := range groups {
			ids = append(ids, g)
		}
		return map[string]any{"restrict": map[string]any{"hide_private": false, "hide_groups": ids, "join_chat": false}}
	}

	expect("unrestricted", true)

	// Only a superadmin restricts, only an admin, only groups the
	// archive has.
	if st, b, _ := e.call("admin", "PATCH", adminPath, restrict(kernelID)); st != 403 && st != 404 {
		t.Errorf("by an admin: %d %v", st, b)
	}
	if st, b, _ := e.call("superadmin", "PATCH", "/api/accounts/5", restrict(kernelID)); st != 400 || b["error"] != "bad_request" {
		t.Errorf("a member: %d %v", st, b)
	}
	if st, b, _ := e.call("superadmin", "PATCH", adminPath, restrict(-1009999999999)); st != 400 || b["field"] != "hide_groups" {
		t.Errorf("an unknown group: %d %v", st, b)
	}
	expect("after refusals", true)

	b := e.want(200, "superadmin", "PATCH", adminPath, restrict(kernelID))
	hg := b["restrict"].(map[string]any)["hide_groups"].([]any)
	if len(hg) != 1 || hg[0].(map[string]any)["id"] != float64(kernelID) ||
		hg[0].(map[string]any)["title"] != "Linux Kernel Indonesia" {
		t.Fatalf("answer %v", b["restrict"])
	}
	expect("hidden", false)
	// Three of its messages are gone from the admin's count, not more.
	if a, s := total("admin"), total("superadmin"); s-a != 3 {
		t.Errorf("counts: admin %v, superadmin %v", a, s)
	}
	// The superadmin still sees it, and the page offers the picker.
	if st, _ := e.page("superadmin", "/groups/"+gid); st != 200 {
		t.Errorf("superadmin group page %d", st)
	}
	_, p := e.page("superadmin", "/settings/accounts")
	if ed := p["props"].(map[string]any)["editable"].(map[string]any); ed["restrict_groups"] != true {
		t.Errorf("editable %v", ed)
	}

	e.want(200, "superadmin", "PATCH", adminPath, restrict())
	expect("lifted", true)

	// A role change away from admin drops the list.
	e.want(200, "superadmin", "PATCH", adminPath, restrict(kernelID))
	e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "member"})
	b = e.want(200, "superadmin", "PATCH", adminPath, map[string]any{"role": "admin"})
	if hg := b["restrict"].(map[string]any)["hide_groups"].([]any); len(hg) != 0 {
		t.Errorf("after member and back: %v", hg)
	}
	if !auditHas(e, []string{"hide_groups none->" + gid, "hide_groups " + gid + "->none"}) {
		t.Errorf("audit %v", e.audits())
	}
}

// A message elsewhere that refers to a hidden group does not name it:
// posted as it, forwarded from it, or replying from it.
func TestHiddenGroupReferences(t *testing.T) {
	t.Parallel()
	v := view{sc: scope.For(scope.Viewer{ID: 3, Role: scope.Admin, HideGroups: []int64{kernelID}})}
	n := names{groups: map[int64]logdb.Group{kernelID: {ID: kernelID, Title: "Linux Kernel Indonesia"}}}
	m := logdb.Message{ChatID: gnuWeeb, SenderChat: kernelID,
		Forward: &logdb.Forward{Type: "channel", ChatID: kernelID, MessageID: 9},
		Reply:   &logdb.Reply{ChatID: gnuWeeb, MessageID: 1, Resolved: true, SenderChat: kernelID}}
	if j := v.messageExtra(n, m); j.SenderChat != nil {
		t.Errorf("posted as %+v", j.SenderChat)
	}
	if s := v.sender(n, m); s.ID != 0 || s.Name != hiddenChat {
		t.Errorf("sender %+v", s)
	}
	if f := v.forwardFrom(n, m); f.ChatID != nil || f.MessageID != nil || f.Name != hiddenChat {
		t.Errorf("forward %+v", f)
	}
	if f := v.forwardName(n, m); *f != hiddenChat {
		t.Errorf("forward name %q", *f)
	}
	if r := v.reply(n, kinds{}, gnuWeeb, m); r.SenderID != 0 || r.SenderName != hiddenChat {
		t.Errorf("reply %+v", r)
	}
	// Visible to a superadmin.
	v.sc = scope.For(scope.Viewer{ID: 1, Role: scope.Superadmin})
	if f := v.forwardFrom(n, m); f.ChatID == nil || f.Name != "Linux Kernel Indonesia" {
		t.Errorf("superadmin forward %+v", f)
	}
}

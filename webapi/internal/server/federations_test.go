// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/testdb"
)

// The tests of federations run on the "federations" fixture of the
// web (testdb/fixtures/web/federations.sql has who is where), each in
// a database of its own.

func fedAccountID(name string) uint64 {
	return map[string]uint64{"fed_root": 101, "kern_admin": 102, "sec_admin": 103,
		"idle_admin": 104, "kern_member": 110, "both_member": 111, "sec_member": 112,
		"default_member": 113, "loose_member": 114}[name]
}

// feds is an account row's federations: admin_of for an admin, the
// grants' federations for a member.
func feds(row map[string]any) string {
	var out []string
	for _, f := range row["admin_of"].([]any) {
		out = append(out, fmt.Sprint(f))
	}
	for _, g := range row["grants"].([]any) {
		out = append(out, fmt.Sprint(g.(map[string]any)["fed"]))
	}
	return strings.Join(out, ",")
}

// Federations limit which accounts an admin manages: the members filed
// in a federation it administers, whichever other federations they are
// in; a member in no federation is the superadmins' alone.
func TestFederationsLimitAccounts(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "federations").DB
	h := newHandler(db, 50)
	all := "both_member,default_member,fed_root,idle_admin,kern_admin,kern_member," +
		"loose_member,sec_admin,sec_member"
	for _, tc := range []struct{ viewer, sees string }{
		{"fed_root", all},
		{"kern_admin", "both_member,kern_member"},
		{"sec_admin", "both_member,default_member,sec_member"},
		{"idle_admin", ""},
	} {
		s := newSession(t, h, tc.viewer, "password")
		a := s.call("GET", "/api/accounts", nil)
		if a.status != 200 || strings.Join(ids(a), ",") != tc.sees {
			t.Errorf("%s lists %d %v, want %s", tc.viewer, a.status, ids(a), tc.sees)
		}
		// Every other account answers 404 to it, as a missing one.
		for _, name := range strings.Split(all, ",") {
			want := 404
			if slices.Contains(strings.Split(tc.sees, ","), name) {
				want = 200
			}
			if a := s.call("GET", fmt.Sprintf("/api/accounts/%d", fedAccountID(name)), nil); a.status != want {
				t.Errorf("%s GET %s: %d, want %d", tc.viewer, name, a.status, want)
			}
		}
	}
	// To a member the administration does not exist.
	m := newSession(t, h, "kern_member", "password")
	if a := m.call("GET", "/api/accounts", nil); a.status != 404 {
		t.Errorf("member lists accounts: %d", a.status)
	}

	// Rows show every federation of the account, also those the viewer
	// does not administer.
	root := newSession(t, h, "fed_root", "password")
	for name, want := range map[string]string{"kern_admin": "1", "sec_admin": "0,2",
		"idle_admin": "", "both_member": "1,2", "default_member": "0", "loose_member": "",
		"fed_root": ""} {
		a := root.call("GET", fmt.Sprintf("/api/accounts/%d", fedAccountID(name)), nil)
		if a.status != 200 || feds(a.body) != want {
			t.Errorf("%s's federations: %d %q, want %q", name, a.status, feds(a.body), want)
		}
	}
	kern := newSession(t, h, "kern_admin", "password")
	if a := kern.call("GET", fmt.Sprintf("/api/accounts/%d", fedAccountID("both_member")), nil); feds(a.body) != "1,2" {
		t.Errorf("both_member as kern_admin sees it: %v", a.body)
	}
}

// An admin files the members it creates and changes in the federations
// it administers, and only there; a superadmin anywhere.
func TestFederationsGrants(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "federations").DB
	h := newHandler(db, 50)
	root := newSession(t, h, "fed_root", "password")
	kern := newSession(t, h, "kern_admin", "password")
	grant := func(f int) map[string]any { return map[string]any{"fed": f, "group_ids": []int{}} }
	path := func(name string) string { return fmt.Sprintf("/api/accounts/%d", fedAccountID(name)) }

	// Creating.
	for _, tc := range []struct {
		s      session
		fed    int
		status int
		code   string
	}{
		{kern, 2, 403, "forbidden"}, // not its federation
		{kern, 0, 403, "forbidden"}, // nor is Default
		{kern, 99, 403, "forbidden"},
		{root, 99, 400, "bad_federation"},
	} {
		a := tc.s.call("POST", "/api/accounts", account("newbie", "member", grant(tc.fed)))
		if a.status != tc.status || a.body["error"] != tc.code {
			t.Errorf("create in %d: %d %v", tc.fed, a.status, a.body)
		}
	}
	id, _ := create(t, kern, account("newbie", "member", grant(1)))
	if a := kern.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.status != 200 || feds(a.body) != "1" {
		t.Fatalf("kern_admin's new member: %d %v", a.status, a.body)
	}
	id2, _ := create(t, root, account("rootie", "member", grant(1), grant(2)))
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id2), nil); feds(a.body) != "1,2" {
		t.Fatalf("fed_root's new member: %v", a.body)
	}
	id3, _ := create(t, root, account("plain", "member"))
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id3), nil); feds(a.body) != "0" {
		t.Fatalf("a superadmin's member without grants: %v", a.body)
	}
	id4, _ := create(t, root, map[string]any{"username": "nowhere", "name": "", "email": "nowhere@example.org",
		"role": "member", "grants": []any{}})
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id4), nil); feds(a.body) != "" {
		t.Fatalf("a member with no grants asked for: %v", a.body)
	}

	// The accounts page sends the member's whole list back: the grant
	// of Security, which kern_admin does not administer, is kept as it
	// is; adding one is refused.
	if a := kern.call("PATCH", path("both_member"), map[string]any{"grants": []any{grant(1), grant(2)}}); a.status != 200 ||
		feds(a.body) != "1,2" {
		t.Fatalf("both_member sent back unchanged: %d %v", a.status, a.body)
	}
	if a := kern.call("PATCH", path("kern_member"), map[string]any{"grants": []any{grant(1), grant(2)}}); a.status != 403 {
		t.Fatalf("kern_admin files a member in Security: %d %v", a.status, a.body)
	}
	// Taking the member out of Kernel leaves it in Security, and out of
	// kern_admin's hands.
	if a := kern.call("PATCH", path("both_member"), map[string]any{"grants": []any{grant(2)}}); a.status != 200 ||
		feds(a.body) != "2" {
		t.Fatalf("both_member out of Kernel: %d %v", a.status, a.body)
	}
	if a := kern.call("GET", path("both_member"), nil); a.status != 404 {
		t.Fatalf("both_member after leaving Kernel: %d", a.status)
	}
	sec := newSession(t, h, "sec_admin", "password")
	if a := sec.call("PATCH", path("sec_member"), map[string]any{"grants": []any{grant(0), grant(2)}}); a.status != 200 ||
		feds(a.body) != "0,2" {
		t.Fatalf("sec_member into Default too: %d %v", a.status, a.body)
	}
	// Leaving grants out changes nothing.
	if a := sec.call("PATCH", path("sec_member"), map[string]any{"name": "Sec"}); feds(a.body) != "0,2" {
		t.Fatalf("a rename changes the federations: %v", a.body)
	}
	// A superadmin replaces them all.
	if a := root.call("PATCH", path("sec_member"), map[string]any{"grants": []any{grant(1)}}); feds(a.body) != "1" {
		t.Fatalf("fed_root sets sec_member's federations: %v", a.body)
	}
	if a := root.call("PATCH", path("sec_member"), map[string]any{"grants": []any{grant(98)}}); a.status != 400 ||
		a.body["error"] != "bad_federation" {
		t.Fatalf("an unknown federation: %d %v", a.status, a.body)
	}
	// Grants are for members.
	if a := root.call("PATCH", path("kern_admin"), map[string]any{"grants": []any{grant(1)}}); a.status != 400 {
		t.Fatalf("grants for an admin: %d %v", a.status, a.body)
	}

	// Audited with what changed.
	var detail string
	if err := db.QueryRow("SELECT detail FROM web_audit WHERE action = 'account_update' " +
		"AND web_user_id = 102 AND target_user_id = 111 ORDER BY id DESC LIMIT 1").Scan(&detail); err != nil ||
		detail != "federations 1,2->2" {
		t.Fatalf("audit of taking both_member out of Kernel: %q %v", detail, err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'account_update' " +
		"AND target_user_id = 111").Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d audit rows of both_member, %v (an unchanged list is not audited)", n, err)
	}
}

// A superadmin says which federations an admin administers, and the
// admin's scope follows on its next request; a role change drops the
// rows of the old role.
func TestFederationsAdminOf(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "federations").DB
	h := newHandler(db, 50)
	root := newSession(t, h, "fed_root", "password")
	kern := newSession(t, h, "kern_admin", "password")
	path := func(name string) string { return fmt.Sprintf("/api/accounts/%d", fedAccountID(name)) }

	if a := root.call("PATCH", path("kern_admin"), map[string]any{"admin_of": []int{2}}); a.status != 200 ||
		feds(a.body) != "2" {
		t.Fatalf("kern_admin to Security: %d %v", a.status, a.body)
	}
	// Same session, new scope.
	if a := kern.call("GET", "/api/accounts", nil); a.status != 200 ||
		strings.Join(ids(a), ",") != "both_member,sec_member" {
		t.Fatalf("kern_admin's accounts after the change: %d %v", a.status, ids(a))
	}
	for _, tc := range []struct {
		name string
		body map[string]any
		code string
	}{
		{"kern_admin", map[string]any{"admin_of": []int{99}}, "bad_federation"},
		{"kern_admin", map[string]any{"admin_of": []int{-1}}, "bad_federation"},
		{"kern_member", map[string]any{"admin_of": []int{1}}, "bad_request"},
		{"fed_root", map[string]any{"admin_of": []int{1}}, "bad_request"},
	} {
		if a := root.call("PATCH", path(tc.name), tc.body); a.status != 400 || a.body["error"] != tc.code {
			t.Errorf("%s %v: %d %v", tc.name, tc.body, a.status, a.body)
		}
	}
	// An empty list is fine for anyone, and admin_of is a superadmin's.
	if a := root.call("PATCH", path("kern_member"), map[string]any{"admin_of": []int{}}); a.status != 200 {
		t.Errorf("admin_of [] for a member: %d %v", a.status, a.body)
	}
	sec := newSession(t, h, "sec_admin", "password")
	if a := sec.call("PATCH", path("sec_member"), map[string]any{"admin_of": []int{}}); a.status != 403 {
		t.Errorf("an admin sends admin_of: %d %v", a.status, a.body)
	}

	// A member made admin leaves its federations; made a member again
	// without grants, it is filed in Default.
	if a := root.call("PATCH", path("both_member"), map[string]any{"role": "admin"}); a.status != 200 ||
		feds(a.body) != "" {
		t.Fatalf("both_member made admin: %d %v", a.status, a.body)
	}
	if a := root.call("PATCH", path("both_member"), map[string]any{"role": "admin", "admin_of": []int{1}}); feds(a.body) != "1" {
		t.Fatalf("both_member made admin of Kernel: %v", a.body)
	}
	if a := root.call("PATCH", path("both_member"), map[string]any{"role": "member"}); feds(a.body) != "0" {
		t.Fatalf("both_member made member again: %v", a.body)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_federation_admins WHERE web_user_id = 111").Scan(&n); err != nil ||
		n != 0 {
		t.Fatalf("%d admin rows left of a member, %v", n, err)
	}
	if a := root.call("PATCH", path("both_member"), map[string]any{"role": "member",
		"grants": []any{map[string]any{"fed": 2, "group_ids": []int{}}}}); feds(a.body) != "2" {
		t.Fatalf("both_member's grants: %v", a.body)
	}
	var detail string
	if err := db.QueryRow("SELECT detail FROM web_audit WHERE action = 'account_update' " +
		"AND target_user_id = 102 ORDER BY id LIMIT 1").Scan(&detail); err != nil || detail != "admin_of 1->2" {
		t.Fatalf("audit of kern_admin's change: %q %v", detail, err)
	}
}

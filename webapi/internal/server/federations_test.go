// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/scope"
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

// newFedHandler is a service over the logger's "sample" fixture and
// the web's "federations" one, with the web database for checks.
func newFedHandler(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	ro := interpolating(t, testdb.New(t, testdb.Logger, "sample"))
	app := testdb.New(t, testdb.Web, "federations").DB
	signer := auth.NewSigner(testKey)
	return New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      ro,
		App:     app,
		Signer:  signer,
		Cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		Limiter: auth.NewRateLimiter(50, time.Hour),
		Tokens:  auth.NewFileTokens(testKey),
	}), app
}

func pickIDs(b map[string]any) string {
	var out []string
	for _, it := range b["items"].([]any) {
		out = append(out, fmt.Sprint(int64(it.(map[string]any)["id"].(float64))))
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

// The chat search of a federation lists its groups; Default's are
// derived, the groups in no other federation, and follow as groups
// move in and out of the others.
func TestFederationsChatSearch(t *testing.T) {
	t.Parallel()
	h, app := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")
	kern := newSession(t, h, "kern_admin", "password")
	search := func(s session, fed string) string {
		t.Helper()
		a := s.call("GET", "/api/chats/search?limit=200&federation="+fed, nil)
		if a.status != 200 {
			t.Fatalf("federation %s: %d %v", fed, a.status, a.body)
		}
		if a.body["next_cursor"] != nil {
			t.Fatalf("federation %s: more than a page", fed)
		}
		return pickIDs(a.body)
	}
	g := func(n ...int) string {
		var out []string
		for _, i := range n {
			out = append(out, fmt.Sprint(-1001000000000-int64(i)))
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	check := func(step string, want map[string]string) {
		t.Helper()
		for fed, w := range want {
			// Federations change no visibility: an admin of none of
			// them gets the same lists.
			for _, s := range []session{root, kern} {
				if got := search(s, fed); got != w {
					t.Errorf("%s: federation %s: %s, want %s", step, fed, got, w)
				}
			}
		}
	}
	check("fixture", map[string]string{
		"1": g(1, 2), "2": g(2, 3), "0": g(4, 5, 10), "99": ""})

	move := func(q string, args ...any) {
		t.Helper()
		if _, err := app.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Out of Security, -1001000000003 is in no federation: Default.
	move("DELETE FROM web_federation_groups WHERE federation_id = 2 AND group_id = ?", -1001000000003)
	check("3 out of Security", map[string]string{"2": g(2), "0": g(3, 4, 5, 10)})
	// Out of Kernel only, -1001000000002 stays out of Default.
	move("DELETE FROM web_federation_groups WHERE federation_id = 1 AND group_id = ?", -1001000000002)
	check("2 out of Kernel", map[string]string{"1": g(1), "2": g(2), "0": g(3, 4, 5, 10)})
	// Into a federation, a Default group leaves Default.
	move("INSERT INTO web_federation_groups (federation_id, group_id) VALUES (1, ?), (1, ?)",
		-1001000000004, -1001000000010)
	check("4 and 10 into Kernel", map[string]string{"1": g(1, 4, 10), "0": g(3, 5)})
	// A federation filters with the scope and the text as before.
	if a := root.call("GET", "/api/chats/search?federation=1&q=ctf", nil); pickIDs(a.body) != g(10) {
		t.Errorf("federation 1, q=ctf: %v", a.body)
	}
	// A federation lists groups only.
	if a := root.call("GET", "/api/chats/search?federation=0&kind=user", nil); pickIDs(a.body) != "" {
		t.Errorf("Default's users: %v", a.body)
	}
}

// Only superadmins create, change and delete federations; to everyone
// else the operations do not exist. Groups move in and out of Default
// as federations take and drop them.
func TestFederationsEditor(t *testing.T) {
	t.Parallel()
	h, app := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")
	groupsOf := func(fed int) string {
		t.Helper()
		a := root.call("GET", fmt.Sprintf("/api/chats/search?limit=200&federation=%d", fed), nil)
		if a.status != 200 {
			t.Fatalf("groups of %d: %d %v", fed, a.status, a.body)
		}
		return pickIDs(a.body)
	}
	g := func(n ...int) string {
		var out []string
		for _, i := range n {
			out = append(out, fmt.Sprint(-1001000000000-int64(i)))
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	gid := func(n int) int64 { return -1001000000000 - int64(n) }

	// Nobody but a superadmin.
	for _, name := range []string{"kern_admin", "sec_admin", "idle_admin", "kern_member"} {
		s := newSession(t, h, name, "password")
		for _, op := range [][2]string{{"POST", "/api/federations"}, {"PATCH", "/api/federations/1"},
			{"PATCH", "/api/federations/0"}, {"DELETE", "/api/federations/1"}} {
			if a := s.call(op[0], op[1], map[string]any{"name": "Mine"}); a.status != 404 {
				t.Errorf("%s %s %s: %d %v", name, op[0], op[1], a.status, a.body)
			}
		}
	}
	anon := &client{t: t, h: h, cookies: map[string]string{}}
	if a := anon.do("POST", "/api/federations", "", map[string]any{"name": "x"}); a.status != 401 {
		t.Errorf("signed out: %d", a.status)
	}
	if a := root.do("POST", "/api/federations", "", map[string]any{"name": "x"}); a.status != 403 ||
		a.body["error"] != "csrf" {
		t.Errorf("without the CSRF token: %d %v", a.status, a.body)
	}

	// Refused writes.
	for _, tc := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{map[string]any{}, 400, "bad_name"},
		{map[string]any{"name": "  "}, 400, "bad_name"},
		{map[string]any{"name": strings.Repeat("x", 65)}, 400, "bad_name"},
		{map[string]any{"name": "a\nb"}, 400, "bad_name"},
		{map[string]any{"name": "kernel"}, 409, "taken"}, // names compare without case
		{map[string]any{"name": "R", "group_ids": []int64{-999}}, 400, "bad_group"},
		{map[string]any{"name": "R", "admin_ids": []int{110}}, 400, "bad_admin"},
		{map[string]any{"name": "R", "admin_ids": []int{101}}, 400, "bad_admin"},
		{map[string]any{"name": "R", "admin_ids": []int{9999}}, 400, "bad_admin"},
		{map[string]any{"name": 5}, 400, "bad_request"},
	} {
		if a := root.call("POST", "/api/federations", tc.body); a.status != tc.status || a.body["error"] != tc.code {
			t.Errorf("create %v: %d %v", tc.body, a.status, a.body)
		}
	}

	// A new federation takes its groups out of Default.
	a := root.call("POST", "/api/federations", map[string]any{"name": " Rust ",
		"group_ids": []int64{gid(3), gid(5), gid(5)}, "admin_ids": []int{104}})
	if a.status != 201 || a.body["id"] != float64(3) {
		t.Fatalf("create Rust: %d %v", a.status, a.body)
	}
	if groupsOf(3) != g(3, 5) || groupsOf(0) != g(4, 10) || groupsOf(2) != g(2, 3) {
		t.Errorf("after Rust: Rust %s, Default %s, Security %s", groupsOf(3), groupsOf(0), groupsOf(2))
	}
	// Its admin manages the members filed in it.
	idle := newSession(t, h, "idle_admin", "password")
	if a := idle.call("GET", "/api/accounts", nil); len(ids(a)) != 0 {
		t.Errorf("idle_admin's accounts before: %v", ids(a))
	}
	if a := root.call("PATCH", fmt.Sprintf("/api/accounts/%d", fedAccountID("loose_member")),
		map[string]any{"grants": []any{map[string]any{"fed": 3, "group_ids": []int{}}}}); a.status != 200 {
		t.Fatalf("loose_member into Rust: %d %v", a.status, a.body)
	}
	if a := idle.call("GET", "/api/accounts", nil); strings.Join(ids(a), ",") != "loose_member" {
		t.Errorf("idle_admin's accounts after: %v", ids(a))
	}

	// Changing one: the fields given replace, the others stay.
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"name": "Linux",
		"group_ids": []int64{gid(1)}}); a.status != 200 || a.body["id"] != float64(1) {
		t.Fatalf("change Kernel: %d %v", a.status, a.body)
	}
	// -1001000000002 is still in Security, so not in Default.
	if groupsOf(1) != g(1) || groupsOf(0) != g(4, 10) {
		t.Errorf("after Kernel: %s, Default %s", groupsOf(1), groupsOf(0))
	}
	var kernAdmins int
	if err := app.QueryRow("SELECT COUNT(*) FROM web_federation_admins WHERE federation_id = 1 " +
		"AND web_user_id = 102").Scan(&kernAdmins); err != nil || kernAdmins != 1 {
		t.Errorf("Kernel's admin after a change without admin_ids: %d %v", kernAdmins, err)
	}
	for _, tc := range []struct {
		path   string
		body   map[string]any
		status int
		code   string
	}{
		{"/api/federations/0", map[string]any{"group_ids": []int64{gid(4)}}, 400, "bad_request"},
		{"/api/federations/0", map[string]any{"name": "Everything"}, 400, "bad_request"},
		{"/api/federations/1", map[string]any{"name": "Security"}, 409, "taken"},
		{"/api/federations/99", map[string]any{"name": "Nowhere"}, 404, "not_found"},
		{"/api/federations/x", map[string]any{"name": "Nowhere"}, 404, "not_found"},
	} {
		if a := root.call("PATCH", tc.path, tc.body); a.status != tc.status || a.body["error"] != tc.code {
			t.Errorf("PATCH %s %v: %d %v", tc.path, tc.body, a.status, a.body)
		}
	}
	// Default takes admins, with its own name sent back as the page
	// does.
	kern := newSession(t, h, "kern_admin", "password")
	if a := root.call("PATCH", "/api/federations/0", map[string]any{"name": "Default",
		"admin_ids": []int{102, 103}}); a.status != 200 {
		t.Fatalf("Default's admins: %d %v", a.status, a.body)
	}
	if a := kern.call("GET", "/api/accounts", nil); strings.Join(ids(a), ",") != "both_member,default_member,kern_member" {
		t.Errorf("kern_admin's accounts as an admin of Default: %v", ids(a))
	}

	// Deleting Security: -1001000000002 is in no federation now, so
	// back in Default; -1001000000003 stays in Rust. sec_member, in
	// Security only, is filed in Default; both_member keeps Kernel.
	if a := root.call("DELETE", "/api/federations/2", nil); a.status != 204 {
		t.Fatalf("delete Security: %d %v", a.status, a.body)
	}
	if groupsOf(0) != g(2, 4, 10) || groupsOf(2) != "" || groupsOf(3) != g(3, 5) {
		t.Errorf("after deleting Security: Default %s, Security %s, Rust %s", groupsOf(0), groupsOf(2), groupsOf(3))
	}
	for name, want := range map[string]string{"sec_member": "0", "both_member": "1", "sec_admin": "0"} {
		if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", fedAccountID(name)), nil); feds(a.body) != want {
			t.Errorf("%s after deleting Security: %q, want %q", name, feds(a.body), want)
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/federations/0", 400}, {"/api/federations/2", 404}, {"/api/federations/99", 404}} {
		if a := root.call("DELETE", tc.path, nil); a.status != tc.status {
			t.Errorf("DELETE %s: %d %v", tc.path, a.status, a.body)
		}
	}

	// Every change is audited with who made it.
	rows, err := app.Query("SELECT action, detail FROM web_audit WHERE action LIKE 'federation_%' " +
		"AND web_user_id = 101 ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, detail string
		if err := rows.Scan(&action, &detail); err != nil {
			t.Fatal(err)
		}
		got = append(got, action+": "+detail)
	}
	want := []string{
		"federation_create: federation 3 Rust; groups +2 -0; admins none->104",
		"federation_update: federation 1; name Kernel->Linux; groups +0 -1",
		"federation_update: federation 0; admins 103->102,103",
		"federation_delete: federation 2 Security; members to Default 112",
	}
	if !slices.Equal(got, want) {
		t.Errorf("audit:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Federations made at once get ids of their own.
func TestFederationsCreateAtOnce(t *testing.T) {
	t.Parallel()
	h, _ := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")
	const n = 6
	got := make([]answer, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = root.call("POST", "/api/federations", map[string]any{"name": fmt.Sprintf("F%d", i)})
		}()
	}
	wg.Wait()
	var fids []int
	for i, a := range got {
		if a.status != 201 {
			t.Fatalf("create F%d: %d %v", i, a.status, a.body)
		}
		fids = append(fids, int(a.body["id"].(float64)))
	}
	slices.Sort(fids)
	if !slices.Equal(fids, []int{3, 4, 5, 6, 7, 8}) {
		t.Errorf("ids %v", fids)
	}
}

// propsServer is a Server over the logger's "sample" fixture and the
// web's "federations" one, for the page data helpers.
func propsServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		ro:     interpolating(t, testdb.New(t, testdb.Logger, "sample")),
		app:    testdb.New(t, testdb.Web, "federations").DB,
		tokens: auth.NewFileTokens(testKey),
	}
}

func fedScope(name string) scope.Scope {
	role := map[string]scope.Role{"fed_root": scope.Superadmin, "kern_admin": scope.Admin,
		"sec_admin": scope.Admin, "idle_admin": scope.Admin}[name]
	adminOf := map[string][]uint64{"kern_admin": {1}, "sec_admin": {0, 2}}[name]
	return scope.For(scope.Viewer{ID: fedAccountID(name), Role: role, EmailVerified: true,
		AdminOf: adminOf})
}

// keys is a JSON object's keys, sorted.
func keys(t *testing.T, v any) string {
	t.Helper()
	b, _ := json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return strings.Join(ks, ",")
}

// The accounts page's federations: those the viewer administers, with
// how many groups it sees in each, and every federation by name.
func TestFederationsAccountsProps(t *testing.T) {
	t.Parallel()
	s := propsServer(t)
	ctx := context.Background()
	if e := accountsEditable(); !e.Federations || e.GroupGrants || e.Restrictions || e.Abilities {
		t.Errorf("editable %+v", e)
	}
	for name, want := range map[string]string{
		"fed_root":   "0 Default 3,1 Kernel 2,2 Security 2",
		"kern_admin": "1 Kernel 2",
		"sec_admin":  "0 Default 3,2 Security 2",
		"idle_admin": "",
	} {
		managed, all, err := s.accountsFederations(ctx, fedScope(name))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range managed {
			got = append(got, fmt.Sprintf("%d %s %d", f.ID, f.Name, f.GroupCount))
		}
		if strings.Join(got, ",") != want || len(all) != 3 || !all[0].Default || all[1].Name != "Kernel" ||
			all[2].Name != "Security" || all[1].Default {
			t.Errorf("%s: %v, all %+v", name, got, all)
		}
		if len(managed) > 0 && keys(t, managed[0]) != "default,group_count,id,name" {
			t.Errorf("Fed with group_count: %s", keys(t, managed[0]))
		}
	}
}

// The federations page: a superadmin's every federation, editable, with
// the admins it may assign; an admin's own, read-only.
func TestFederationsPageProps(t *testing.T) {
	t.Parallel()
	s := propsServer(t)
	ctx := context.Background()
	row := func(r federationRowJSON) string {
		var gs, as, ms []string
		for _, g := range r.Groups {
			gs = append(gs, fmt.Sprint(g.ID+1001000000000))
		}
		for _, a := range r.Admins {
			as = append(as, a.Username)
		}
		for _, m := range r.Members {
			ms = append(ms, m.Username)
		}
		return fmt.Sprintf("%d %s %v groups %s of %d admins %s members %s", r.ID, r.Name, r.Default,
			strings.Join(gs, " "), r.GroupCount, strings.Join(as, " "), strings.Join(ms, " "))
	}
	def := "0 Default true groups -5 -10 -4 of 3 admins sec_admin members default_member"
	kern := "1 Kernel false groups -1 -2 of 2 admins kern_admin members both_member kern_member"
	sec := "2 Security false groups -2 -3 of 2 admins sec_admin members both_member sec_member"
	for _, tc := range []struct {
		name     string
		rows     []string
		editable bool
		options  string
	}{
		{"fed_root", []string{def, kern, sec}, true, "idle_admin kern_admin sec_admin"},
		{"kern_admin", []string{kern}, false, ""},
		{"sec_admin", []string{def, sec}, false, ""},
		{"idle_admin", nil, false, ""},
	} {
		p, err := s.federationsProps(ctx, fedScope(tc.name))
		if err != nil {
			t.Fatal(err)
		}
		var rows []string
		for _, r := range p.Federations {
			rows = append(rows, row(r))
		}
		var opts []string
		for _, o := range p.AdminOptions {
			opts = append(opts, o.Username)
		}
		if !slices.Equal(rows, tc.rows) || p.Editable != tc.editable || strings.Join(opts, " ") != tc.options {
			t.Errorf("%s:\n%s\neditable %v, options %v", tc.name, strings.Join(rows, "\n"), p.Editable, opts)
		}
		if keys(t, p) != "admin_options,editable,federations" {
			t.Errorf("FederationsProps keys %s", keys(t, p))
		}
		for _, r := range p.Federations {
			if keys(t, r) != "admins,created,default,group_count,groups,id,members,name" {
				t.Errorf("FederationRow keys %s", keys(t, r))
			}
			if (r.Created == nil) != r.Default {
				t.Errorf("%s created %v", r.Name, r.Created)
			}
		}
	}
	// A preview shows at most federationPreview groups, the count all.
	p, err := s.federationsProps(ctx, fedScope("fed_root"))
	if err != nil || p.Federations[1].Groups[0].Title != "GNU/Weeb" || p.Federations[1].Groups[0].Photo == nil {
		t.Errorf("Kernel's first group: %+v %v", p.Federations[1].Groups, err)
	}
}

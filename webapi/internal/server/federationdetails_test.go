// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// The tests of a federation's description and details, on the
// "federations" fixture of the web and the "sample" one of the logger
// (see federations_test.go), each in databases of their own.

// fedAudits is the federation changes audited as made by the account,
// "action: detail", oldest first.
func fedAudits(t *testing.T, app *sql.DB, actor uint64) []string {
	t.Helper()
	rows, err := app.Query("SELECT action, detail FROM web_audit WHERE action LIKE 'federation_%' "+
		"AND web_user_id = ? ORDER BY id", actor)
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
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

// fedDescription is the description stored for a federation.
func fedDescription(t *testing.T, app *sql.DB, id int) string {
	t.Helper()
	var d string
	if err := app.QueryRow("SELECT description FROM web_federations WHERE id = ?", id).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

// listedDescriptions is the federations page's rows as "name:
// description".
func listedDescriptions(t *testing.T, s session) []string {
	t.Helper()
	a := s.call("GET", "/api/page/settings/federations", nil)
	if a.status != 200 || a.body["route"] != "federations" {
		t.Fatalf("federations page: %d %v", a.status, a.body)
	}
	var out []string
	for _, r := range a.body["props"].(map[string]any)["federations"].([]any) {
		row := r.(map[string]any)
		out = append(out, row["name"].(string)+": "+row["description"].(string))
	}
	return out
}

// A superadmin describes a federation when it creates or changes one:
// trimmed, at most 2000 characters, newlines and tabs but no other
// control characters; "" clears it. The federations page shows it.
func TestFederationDescription(t *testing.T) {
	t.Parallel()
	h, app := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")

	for _, d := range []string{strings.Repeat("x", 2001), "a\x00b", "bell\a", "a\rb", "\x7f",
		"esc\x1b[1m"} {
		for _, op := range [][2]string{{"POST", "/api/federations"}, {"PATCH", "/api/federations/1"}} {
			a := root.call(op[0], op[1], map[string]any{"name": "Described", "description": d})
			if a.status != 400 || a.body["error"] != "bad_description" {
				t.Errorf("%s %q: %d %v", op[0], d, a.status, a.body)
			}
		}
	}
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": 7}); a.status != 400 ||
		a.body["error"] != "bad_request" {
		t.Errorf("a number: %d %v", a.status, a.body)
	}
	if got := fedDescription(t, app, 1); got != "" {
		t.Errorf("after refusals: %q", got)
	}

	// 2000 characters fit, counted as characters, not bytes.
	long := strings.Repeat("é", 2000)
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": long}); a.status != 200 {
		t.Fatalf("2000 characters: %d %v", a.status, a.body)
	}
	if got := fedDescription(t, app, 1); got != long {
		t.Errorf("stored %d characters", len([]rune(got)))
	}
	text := "Linux kernel groups.\r\n\tAsk in the main one first. "
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": text}); a.status != 200 {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	if got := fedDescription(t, app, 1); got != "Linux kernel groups.\n\tAsk in the main one first." {
		t.Errorf("stored %q", got)
	}
	// The same text again changes nothing and is not audited.
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": text}); a.status != 200 {
		t.Fatalf("again: %d %v", a.status, a.body)
	}
	a := root.call("POST", "/api/federations", map[string]any{"name": "Rust", "description": " Crabs "})
	if a.status != 201 || a.body["id"] != float64(3) {
		t.Fatalf("create: %d %v", a.status, a.body)
	}
	// Default takes one too.
	if a := root.call("PATCH", "/api/federations/0", map[string]any{"name": "Default",
		"description": "Everything else."}); a.status != 200 {
		t.Fatalf("Default: %d %v", a.status, a.body)
	}
	want := []string{"Default: Everything else.", "Kernel: Linux kernel groups.\n\tAsk in the main one first.",
		"Rust: Crabs", "Security: "}
	if got := listedDescriptions(t, root); !slices.Equal(got, want) {
		t.Errorf("page rows %q, want %q", got, want)
	}
	// An admin's page shows its federations' descriptions as well.
	kern := newSession(t, h, "kern_admin", "password")
	if got := listedDescriptions(t, kern); !slices.Equal(got, want[1:2]) {
		t.Errorf("kern_admin's rows %q", got)
	}
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": "  "}); a.status != 200 {
		t.Fatalf("clear: %d %v", a.status, a.body)
	}
	if got := fedDescription(t, app, 1); got != "" {
		t.Errorf("after clearing: %q", got)
	}

	wantAudit := []string{
		"federation_update: federation 1; description set",
		"federation_update: federation 1; description set",
		"federation_create: federation 3 Rust; description set",
		"federation_update: federation 0; description set",
		"federation_update: federation 1; description cleared",
	}
	if got := fedAudits(t, app, fedAccountID("fed_root")); !slices.Equal(got, wantAudit) {
		t.Errorf("audit:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantAudit, "\n"))
	}
}

// An admin renames and describes the federations it administers, and
// changes nothing else of them: groups or admins answer 403, another
// federation 404 as if it did not exist, Default's name 400 as for a
// superadmin. Its changes are audited as its own.
func TestFederationAdminRename(t *testing.T) {
	t.Parallel()
	h, app := newFedHandler(t)
	kern := newSession(t, h, "kern_admin", "password")
	sec := newSession(t, h, "sec_admin", "password")
	member := newSession(t, h, "kern_member", "password")
	name := func(id int) string {
		t.Helper()
		var n string
		if err := app.QueryRow("SELECT name FROM web_federations WHERE id = ?", id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	for _, tc := range []struct {
		s      session
		who    string
		path   string
		body   map[string]any
		status int
		code   string
	}{
		{kern, "kern_admin", "/api/federations/1", map[string]any{"group_ids": []int64{-1001000000001}}, 403, "forbidden"},
		{kern, "kern_admin", "/api/federations/1", map[string]any{"name": "Linux", "admin_ids": []int{102}}, 403, "forbidden"},
		{kern, "kern_admin", "/api/federations/1", map[string]any{"group_ids": []int64{}}, 403, "forbidden"},
		{kern, "kern_admin", "/api/federations/2", map[string]any{"name": "Mine"}, 404, "not_found"},
		{kern, "kern_admin", "/api/federations/0", map[string]any{"description": "x"}, 404, "not_found"},
		{kern, "kern_admin", "/api/federations/99", map[string]any{"name": "Mine"}, 404, "not_found"},
		{sec, "sec_admin", "/api/federations/1", map[string]any{"description": "x"}, 404, "not_found"},
		{sec, "sec_admin", "/api/federations/0", map[string]any{"name": "Everything"}, 400, "bad_request"},
		{sec, "sec_admin", "/api/federations/2", map[string]any{"name": "Kernel"}, 409, "taken"},
		{sec, "sec_admin", "/api/federations/2", map[string]any{"name": " "}, 400, "bad_name"},
		{sec, "sec_admin", "/api/federations/2", map[string]any{"description": strings.Repeat("d", 2001)}, 400, "bad_description"},
		{sec, "sec_admin", "/api/federations/2", map[string]any{"description": "tab\tok but not\x00"}, 400, "bad_description"},
		{member, "kern_member", "/api/federations/1", map[string]any{"name": "Mine"}, 404, "not_found"},
	} {
		if a := tc.s.call("PATCH", tc.path, tc.body); a.status != tc.status || a.body["error"] != tc.code {
			t.Errorf("%s PATCH %s %v: %d %v", tc.who, tc.path, tc.body, a.status, a.body)
		}
	}
	if name(1) != "Kernel" || name(2) != "Security" || fedDescription(t, app, 2) != "" {
		t.Fatalf("after refusals: %s, %s, %q", name(1), name(2), fedDescription(t, app, 2))
	}
	// Creating and deleting stay the superadmins'.
	if a := kern.call("POST", "/api/federations", map[string]any{"name": "Mine"}); a.status != 404 {
		t.Errorf("kern_admin creates: %d", a.status)
	}
	if a := kern.call("DELETE", "/api/federations/1", nil); a.status != 404 {
		t.Errorf("kern_admin deletes: %d", a.status)
	}

	if a := kern.call("PATCH", "/api/federations/1", map[string]any{"name": " Linux ",
		"description": "Kernel hackers."}); a.status != 200 || a.body["id"] != float64(1) {
		t.Fatalf("kern_admin renames: %d %v", a.status, a.body)
	}
	if name(1) != "Linux" || fedDescription(t, app, 1) != "Kernel hackers." {
		t.Errorf("after the rename: %s %q", name(1), fedDescription(t, app, 1))
	}
	// Default's admin describes it, sending its name back unchanged.
	if a := sec.call("PATCH", "/api/federations/0", map[string]any{"name": "Default",
		"description": "The rest."}); a.status != 200 {
		t.Fatalf("sec_admin describes Default: %d %v", a.status, a.body)
	}
	if got := fedAudits(t, app, fedAccountID("kern_admin")); !slices.Equal(got,
		[]string{"federation_update: federation 1; name Kernel->Linux; description set"}) {
		t.Errorf("kern_admin's audit %q", got)
	}
	if got := fedAudits(t, app, fedAccountID("sec_admin")); !slices.Equal(got,
		[]string{"federation_update: federation 0; description set"}) {
		t.Errorf("sec_admin's audit %q", got)
	}
	// The groups and admins stayed.
	var n int
	if err := app.QueryRow("SELECT (SELECT COUNT(*) FROM web_federation_groups WHERE federation_id = 1) + " +
		"(SELECT COUNT(*) FROM web_federation_admins WHERE federation_id = 1)").Scan(&n); err != nil || n != 3 {
		t.Errorf("Kernel's groups and admins: %d %v", n, err)
	}
}

// fedGroupIDs is a groups page's items, as their ids less
// -1001000000000 (so -1 is -1001000000001), in the order given.
func fedGroupIDs(t *testing.T, page any) string {
	t.Helper()
	var out []string
	for _, it := range page.(map[string]any)["items"].([]any) {
		g := it.(map[string]any)
		if _, ok := g["msg_count"]; !ok {
			t.Errorf("group without msg_count: %v", g)
		}
		out = append(out, fmt.Sprint(int64(g["id"].(float64))+1001000000000))
	}
	return strings.Join(out, " ")
}

// A federation's details: a superadmin sees every federation, an admin
// those it administers, anyone else none (404, as for one that does not
// exist); the same data as the page federation. can_edit says what the
// viewer may change.
func TestFederationDetails(t *testing.T) {
	t.Parallel()
	h, app := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")
	kern := newSession(t, h, "kern_admin", "password")
	sec := newSession(t, h, "sec_admin", "password")
	idle := newSession(t, h, "idle_admin", "password")
	member := newSession(t, h, "kern_member", "password")
	if a := root.call("PATCH", "/api/federations/1", map[string]any{"description": "Kernel hackers."}); a.status != 200 {
		t.Fatalf("describe: %d %v", a.status, a.body)
	}

	a := kern.call("GET", "/api/federations/1", nil)
	if a.status != 200 {
		t.Fatalf("kern_admin's federation: %d %v", a.status, a.body)
	}
	if keys(t, a.body) != "admins,can_edit,federation,groups,members" {
		t.Errorf("FederationDetailProps keys %s", keys(t, a.body))
	}
	f := a.body["federation"].(map[string]any)
	if keys(t, f) != "created,created_by,default,description,group_count,id,name" {
		t.Errorf("FederationDetail keys %s", keys(t, f))
	}
	created, _ := f["created"].(string)
	if f["id"] != float64(1) || f["name"] != "Kernel" || f["default"] != false ||
		f["description"] != "Kernel hackers." || f["group_count"] != float64(2) ||
		!strings.HasPrefix(created, "2026-04-0") {
		t.Errorf("federation %v", f)
	}
	if by := js(f["created_by"]); by != `{"id":101,"name":"Fed Root","username":"fed_root"}` {
		t.Errorf("created_by %s", by)
	}
	if got := js(a.body["admins"]); got != `[{"id":102,"name":"Kernel Admin","username":"kern_admin"}]` {
		t.Errorf("admins %s", got)
	}
	if got := js(a.body["members"]); got != `[{"id":111,"name":"","username":"both_member"},`+
		`{"id":110,"name":"","username":"kern_member"}]` {
		t.Errorf("members %s", got)
	}
	g := a.body["groups"].(map[string]any)
	if fedGroupIDs(t, g) != "-1 -2" || g["total"] != float64(2) || g["next_cursor"] != nil {
		t.Errorf("groups %v", g)
	}
	if got := js(a.body["can_edit"]); got != `{"admins":false,"delete":false,"description":true,"groups":false,"name":true}` {
		t.Errorf("kern_admin's can_edit %s", got)
	}
	// The page carries the same data.
	p := kern.call("GET", "/api/page/settings/federations/1", nil)
	if p.status != 200 || p.body["route"] != "federation" || js(p.body["props"]) != js(a.body) {
		t.Errorf("page: %d %v", p.status, p.body)
	}

	// What each viewer may change.
	for _, tc := range []struct {
		s       session
		who, id string
		want    string
	}{
		{root, "fed_root", "1", `{"admins":true,"delete":true,"description":true,"groups":true,"name":true}`},
		{root, "fed_root", "0", `{"admins":true,"delete":false,"description":true,"groups":false,"name":false}`},
		{sec, "sec_admin", "0", `{"admins":false,"delete":false,"description":true,"groups":false,"name":false}`},
		{sec, "sec_admin", "2", `{"admins":false,"delete":false,"description":true,"groups":false,"name":true}`},
	} {
		a := tc.s.call("GET", "/api/federations/"+tc.id, nil)
		if a.status != 200 || js(a.body["can_edit"]) != tc.want {
			t.Errorf("%s on %s: %d can_edit %s", tc.who, tc.id, a.status, js(a.body["can_edit"]))
		}
	}
	// Default has no creator or date; its groups are those in no other
	// federation.
	a = root.call("GET", "/api/federations/0", nil)
	f = a.body["federation"].(map[string]any)
	if f["default"] != true || f["created"] != nil || f["created_by"] != nil || f["group_count"] != float64(3) ||
		fedGroupIDs(t, a.body["groups"]) != "-5 -10 -4" {
		t.Errorf("Default: %v %v", f, a.body["groups"])
	}

	// Federations the viewer does not see do not exist for it.
	for _, tc := range []struct {
		s       session
		who, id string
	}{
		{kern, "kern_admin", "2"}, {kern, "kern_admin", "0"}, {kern, "kern_admin", "99"},
		{sec, "sec_admin", "1"}, {idle, "idle_admin", "1"}, {idle, "idle_admin", "0"},
		{member, "kern_member", "1"}, {member, "kern_member", "0"}, {root, "fed_root", "99"},
	} {
		for _, path := range []string{"/api/federations/" + tc.id, "/api/federations/" + tc.id + "/groups"} {
			if a := tc.s.call("GET", path, nil); a.status != 404 || a.body["error"] != "not_found" {
				t.Errorf("%s GET %s: %d %v", tc.who, path, a.status, a.body)
			}
		}
		if a := tc.s.call("GET", "/api/page/settings/federations/"+tc.id, nil); a.status != 404 ||
			a.body["route"] != "not_found" {
			t.Errorf("%s page %s: %d %v", tc.who, tc.id, a.status, a.body)
		}
	}
	if a := root.call("GET", "/api/page/settings/federations/-1", nil); a.status != 404 {
		t.Errorf("page -1: %d", a.status)
	}
	if a := root.call("GET", "/api/federations/x", nil); a.status != 404 {
		t.Errorf("GET x: %d", a.status)
	}
	anon := &client{t: t, h: h, cookies: map[string]string{}}
	if a := anon.do("GET", "/api/federations/1", "", nil); a.status != 401 {
		t.Errorf("signed out: %d", a.status)
	}

	// Groups hidden from an admin are neither listed nor counted.
	if _, err := app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) VALUES (102, -1001000000001)"); err != nil {
		t.Fatal(err)
	}
	a = kern.call("GET", "/api/federations/1", nil)
	if a.body["federation"].(map[string]any)["group_count"] != float64(1) ||
		fedGroupIDs(t, a.body["groups"]) != "-2" || a.body["groups"].(map[string]any)["total"] != float64(1) {
		t.Errorf("with a hidden group: %v %v", a.body["federation"], a.body["groups"])
	}
	for _, q := range []string{"", "?q=weeb", "?q=gnuweeb"} {
		a := kern.call("GET", "/api/federations/1/groups"+q, nil)
		want, total := "-2", float64(1)
		if q != "" {
			want, total = "", 0
		}
		if a.status != 200 || fedGroupIDs(t, a.body) != want || a.body["total"] != total {
			t.Errorf("kern_admin's groups%s with a hidden group: %d %v", q, a.status, a.body)
		}
	}
	if a := root.call("GET", "/api/federations/1", nil); a.body["federation"].(map[string]any)["group_count"] != float64(2) {
		t.Errorf("fed_root with kern_admin's hidden group: %v", a.body["federation"])
	}
}

// A federation's groups page by page: busiest first, then by id
// descending, with a cursor that holds across equal counts, q on title
// and username, and total counting every match.
func TestFederationGroups(t *testing.T) {
	t.Parallel()
	h, _ := newFedHandler(t)
	root := newSession(t, h, "fed_root", "password")
	all := []int64{-1001000000001, -1001000000002, -1001000000003, -1001000000004, -1001000000005,
		-1001000000010}
	if a := root.call("POST", "/api/federations", map[string]any{"name": "All", "group_ids": all}); a.status != 201 ||
		a.body["id"] != float64(3) {
		t.Fatalf("create: %d %v", a.status, a.body)
	}
	pages := func(query string) []string {
		t.Helper()
		var out []string
		cursor := ""
		for range 10 {
			path := "/api/federations/3/groups?" + query
			if cursor != "" {
				path += "&cursor=" + url.QueryEscape(cursor)
			}
			a := root.call("GET", path, nil)
			if a.status != 200 {
				t.Fatalf("GET %s: %d %v", path, a.status, a.body)
			}
			out = append(out, fmt.Sprintf("%s (%v)", fedGroupIDs(t, a.body), a.body["total"]))
			next, ok := a.body["next_cursor"].(string)
			if !ok {
				return out
			}
			cursor = next
		}
		t.Fatalf("%s: no last page", query)
		return nil
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"", []string{"-1 -2 -3 -5 -10 -4 (6)"}},
		{"limit=2", []string{"-1 -2 (6)", "-3 -5 (6)", "-10 -4 (6)"}},
		// The two groups of 2 messages fall on two pages.
		{"limit=3", []string{"-1 -2 -3 (6)", "-5 -10 -4 (6)"}},
		{"limit=6", []string{"-1 -2 -3 -5 -10 -4 (6)"}},
		{"limit=200", []string{"-1 -2 -3 -5 -10 -4 (6)"}},
		{"q=LINUX", []string{"-2 (1)"}},
		{"q=%40rust", []string{"-3 (1)"}},
		{"q=ctfpr", []string{"-10 (1)"}}, // its username only
		{"q=" + url.QueryEscape("%"), []string{" (0)"}},
		{"q=o&limit=1", []string{"-2 (4)", "-3 (4)", "-5 (4)", "-4 (4)"}},
	} {
		if got := pages(tc.query); !slices.Equal(got, tc.want) {
			t.Errorf("%q: %q, want %q", tc.query, got, tc.want)
		}
	}
	for _, q := range []string{"limit=0", "limit=201", "limit=x", "cursor=x", "cursor=" + fedGroupsCursorText("o5"),
		"cursor=" + fedGroupsCursorText("k1"), "cursor=" + fedGroupsCursorText("k1:x"),
		"q=" + strings.Repeat("q", 513)} {
		if a := root.call("GET", "/api/federations/3/groups?"+q, nil); a.status != 400 || a.body["error"] != "bad_request" {
			t.Errorf("%.40s: %d %v", q, a.status, a.body)
		}
	}
}

// fedGroupsCursorText encodes a cursor's text as the service does, for
// cursors it would never give.
func fedGroupsCursorText(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

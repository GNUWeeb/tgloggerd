// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
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

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The MCP administration (adminEnv): the sample exposes GNU/Weeb, Rust
// Programming and CTF Practice; the web's "admin" fixture has a live
// and a revoked token.

func TestMcpPages(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)

	st, b := e.page("admin", "/mcp-admin")
	if st != 200 || b["route"] != "mcp" {
		t.Fatalf("index %d %v", st, b)
	}
	// GNU/Weeb 8 + Rust 2 + CTF Practice's messages.
	msgs := e.one("SELECT SUM(g.msg_count) FROM telegram_public_groups p " +
		"JOIN telegram_groups g ON g.id = p.group_id")
	if js(b["props"]) != canon(`{"exposed_groups": 3, "exposed_messages": `+msgs+`,
		"active_tokens": 1, "last_used": "2026-11-17T15:00:00Z",
		"endpoint": "http://example.com/mcp"}`) {
		t.Errorf("index %s", js(b["props"]))
	}

	_, b = e.page("superadmin", "/mcp-admin/groups")
	gs := b["props"].(map[string]any)["groups"].([]any)
	if len(gs) != 3 {
		t.Fatalf("groups %v", gs)
	}
	var titles []string
	for _, g := range gs {
		m := g.(map[string]any)
		titles = append(titles, m["group"].(map[string]any)["title"].(string))
		if m["group"].(map[string]any)["msg_count"] == nil || m["added"] == "" {
			t.Errorf("group %v", m)
		}
	}
	if !strings.Contains(js(gs), `"note":"community"`) || !strings.Contains(js(gs), `"public_now":true`) {
		t.Errorf("groups %s", js(gs))
	}

	_, b = e.page("admin", "/mcp-admin/tokens")
	if js(b["props"]) != canon(`{"tokens": [
		{"id": 2, "name": "old laptop", "owner": "ops", "created": "2026-03-01T08:00:00Z",
		 "last_used": "2026-04-01T08:00:00Z", "revoked": "2026-05-01T08:00:00Z"},
		{"id": 1, "name": "claude-desktop", "owner": "admin", "created": "2026-10-01T08:00:00Z",
		 "last_used": "2026-11-17T15:00:00Z", "revoked": null}]}`) {
		t.Errorf("tokens %s", js(b["props"]))
	}

	for _, p := range []string{"/mcp-admin", "/mcp-admin/groups", "/mcp-admin/tokens"} {
		if st, b := e.page("member", p); st != 404 || b["route"] != "not_found" {
			t.Errorf("member %s: %d %v", p, st, b)
		}
	}
}

func TestMcpGroups(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	const kernel = -1001000000002 // Linux Kernel Indonesia, not exposed

	b := e.want(201, "admin", "POST", "/api/mcp/groups", map[string]any{"group_id": kernel,
		"note": "  " + strings.Repeat("n", 300) + "  "})
	if b["group_id"] != float64(kernel) {
		t.Errorf("expose %v", b)
	}
	if got := e.one("SELECT CONCAT_WS(' ', CHAR_LENGTH(note), added_by) FROM telegram_public_groups "+
		"WHERE group_id = ?", kernel); got != "255 3" {
		t.Errorf("stored %q", got)
	}
	if st, b, _ := e.call("superadmin", "POST", "/api/mcp/groups", map[string]any{"group_id": kernel}); st != 409 ||
		b["error"] != "already_exposed" {
		t.Errorf("again %d %v", st, b)
	}
	e.want(204, "superadmin", "DELETE", fmt.Sprintf("/api/mcp/groups/%d", kernel), nil)
	if e.one("SELECT COUNT(*) FROM telegram_public_groups WHERE group_id = ?", kernel) != "0" {
		t.Error("not withdrawn")
	}
	e.want(404, "superadmin", "DELETE", fmt.Sprintf("/api/mcp/groups/%d", kernel), nil)
	e.want(201, "superadmin", "POST", "/api/mcp/groups", map[string]any{"group_id": kernel, "note": "again"})

	want := []string{
		fmt.Sprintf("3 mcp_expose group %d %s", kernel, strings.Repeat("n", 255-len(fmt.Sprintf("group %d ", kernel)))),
		fmt.Sprintf("3 owner_set mcp_group %d", kernel),
		fmt.Sprintf("1 mcp_unexpose group %d", kernel),
		fmt.Sprintf("1 resource_drop mcp_group %d", kernel),
		fmt.Sprintf("1 mcp_expose group %d again", kernel),
		fmt.Sprintf("1 owner_set mcp_group %d", kernel),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	for _, c := range []struct {
		role   string
		method string
		path   string
		body   map[string]any
		status int
	}{
		{"superadmin", "POST", "/api/mcp/groups", map[string]any{"group_id": -1009999999999}, 404},
		{"superadmin", "POST", "/api/mcp/groups", map[string]any{"group_id": 9299000001}, 400},
		{"superadmin", "POST", "/api/mcp/groups", map[string]any{}, 400},
		{"superadmin", "DELETE", "/api/mcp/groups/-1009999999999", nil, 404},
		{"superadmin", "DELETE", "/api/mcp/groups/x", nil, 404},
		{"member", "POST", "/api/mcp/groups", map[string]any{"group_id": -1001000000004}, 404},
		{"member", "DELETE", "/api/mcp/groups/-1001000000001", nil, 404},
		{"", "POST", "/api/mcp/groups", map[string]any{"group_id": -1001000000004}, 401},
	} {
		if st, b, _ := e.call(c.role, c.method, c.path, c.body); st != c.status {
			t.Errorf("%s %s %s %v: %d %v", c.role, c.method, c.path, c.body, st, b)
		}
	}
	if st, _, _ := e.call("admin", "POST", "/api/mcp/groups", map[string]any{"group_id": -1001000000004}, false); st != 403 {
		t.Errorf("no csrf %d", st)
	}
	if n := e.one("SELECT COUNT(*) FROM telegram_public_groups"); n != "4" {
		t.Errorf("%s exposed after refusals", n)
	}
	if len(e.audits()) != 6 {
		t.Errorf("refusals audited: %v", e.audits())
	}
}

// The note of an exposed group is reworded in place: who exposed it
// and when stay, and the audit row keeps the old and the new note.
func TestMcpGroupNote(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	const gnu = -1001000000001 // GNU/Weeb, exposed with "community"
	path := fmt.Sprintf("/api/mcp/groups/%d", gnu)
	row := func() string {
		return e.one("SELECT CONCAT_WS(' ', added_by, created_at) FROM telegram_public_groups "+
			"WHERE group_id = ?", gnu)
	}
	note := func() string {
		return e.one("SELECT note FROM telegram_public_groups WHERE group_id = ?", gnu)
	}
	if _, err := e.ro.Exec("UPDATE telegram_public_groups SET added_by = 1, "+
		"created_at = '2026-01-01 00:00:00' WHERE group_id = ?", gnu); err != nil {
		t.Fatal(err)
	}
	before := row()

	e.want(204, "admin", "PATCH", path, map[string]any{"note": "  kernel hackers  "})
	if got := note(); got != "kernel hackers" {
		t.Errorf("note %q", got)
	}
	if got := row(); got != before {
		t.Errorf("exposer and date %q, want %q", got, before)
	}
	_, b := e.page("admin", "/mcp-admin/groups")
	if !strings.Contains(js(b["props"]), `"note":"kernel hackers"`) {
		t.Errorf("page %s", js(b["props"]))
	}
	// Unchanged: nothing to audit.
	e.want(204, "superadmin", "PATCH", path, map[string]any{"note": "kernel hackers"})
	// Clipped to the column.
	e.want(204, "superadmin", "PATCH", path, map[string]any{"note": strings.Repeat("n", 300)})
	if got := note(); got != strings.Repeat("n", 255) {
		t.Errorf("clipped note %d characters", len(got))
	}
	// An empty note clears it.
	e.want(204, "admin", "PATCH", path, map[string]any{"note": ""})
	if got := note(); got != "" {
		t.Errorf("cleared note %q", got)
	}
	if got := row(); got != before {
		t.Errorf("exposer and date %q, want %q", got, before)
	}

	prefix := fmt.Sprintf("group %d ", gnu)
	room := 255 - len(prefix) - len(" -> ")
	want := []string{
		fmt.Sprintf("3 mcp_note %scommunity -> kernel hackers", prefix),
		fmt.Sprintf("1 mcp_note %skernel hackers -> %s", prefix, strings.Repeat("n", room-len("kernel hackers"))),
		fmt.Sprintf("3 mcp_note %s%s -> ", prefix, strings.Repeat("n", room)),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if _, err := e.ro.Exec("UPDATE telegram_public_groups SET note = 'community' WHERE group_id = ?",
		gnu); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		role   string
		path   string
		body   any
		status int
	}{
		{"superadmin", "/api/mcp/groups/-1009999999999", map[string]any{"note": "x"}, 404},
		{"superadmin", "/api/mcp/groups/-1001000000002", map[string]any{"note": "x"}, 404}, // not exposed
		{"superadmin", "/api/mcp/groups/x", map[string]any{"note": "x"}, 404},
		{"superadmin", "/api/mcp/groups/1001000000001", map[string]any{"note": "x"}, 404},
		{"superadmin", path, map[string]any{}, 400},
		{"superadmin", path, map[string]any{"note": nil}, 400},
		{"superadmin", path, map[string]any{"note": 5}, 400},
		{"member", path, map[string]any{"note": "x"}, 404},
		{"", path, map[string]any{"note": "x"}, 401},
	} {
		if st, b, _ := e.call(c.role, "PATCH", c.path, c.body); st != c.status {
			t.Errorf("%s %s %v: %d %v", c.role, c.path, c.body, st, b)
		}
	}
	if st, _, _ := e.call("admin", "PATCH", path, map[string]any{"note": "x"}, false); st != 403 {
		t.Errorf("no csrf %d", st)
	}
	if got := note(); got != "community" {
		t.Errorf("note after refusals %q", got)
	}
	if e.one("SELECT COUNT(*) FROM telegram_public_groups WHERE group_id = -1001000000002") != "0" {
		t.Error("a refused note exposed a group")
	}
	if len(e.audits()) != len(want) {
		t.Errorf("refusals audited: %v", e.audits())
	}
}

// noteChange gives each note its share of the audit detail.
func TestMcpNoteChange(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("o", 200)
	got := noteChange("group -1 ", long, strings.Repeat("n", 200))
	if utf8.RuneCountInString(got) != 255 || !strings.Contains(got, strings.Repeat("o", 121)+" -> "+strings.Repeat("n", 121)) {
		t.Errorf("both long: %d %q", len(got), got)
	}
	if got := noteChange("group -1 ", "a", "b"); got != "group -1 a -> b" {
		t.Errorf("short %q", got)
	}
	got = noteChange("group -1 ", strings.Repeat("\u00e9", 250), "b")
	if utf8.RuneCountInString(got) != 255 || !strings.HasSuffix(got, "\u00e9 -> b") {
		t.Errorf("runes %d %q", utf8.RuneCountInString(got), got)
	}
}

// A group the scope hides is not exposed, listed or counted.
func TestMcpHiddenGroups(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	ctx := context.Background()
	member := scope.For(scope.Viewer{ID: 5, Role: scope.Member})
	if ok, err := logdb.GroupVisible(ctx, e.ro, member, -1001000000002); ok || err != nil {
		t.Errorf("member sees the group: %v %v", ok, err)
	}
	gs, err := logdb.ExposedGroups(ctx, e.ro, member)
	if err != nil || len(gs) != 0 {
		t.Errorf("member lists %v %v", gs, err)
	}
	if n, m, err := logdb.ExposedStats(ctx, e.ro, member); n != 0 || m != 0 || err != nil {
		t.Errorf("member counts %d %d %v", n, m, err)
	}
}

var mcpTokenRe = regexp.MustCompile(`^tgmcp_[0-9a-f]{64}$`)

func TestMcpTokens(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	b := e.want(201, "admin", "POST", "/api/mcp/tokens", map[string]any{"name": "  ci runner  "})
	tok, _ := b["token"].(string)
	id := uint64(b["id"].(float64))
	if !mcpTokenRe.MatchString(tok) {
		t.Fatalf("token %q", tok)
	}
	// Stored as its SHA-256 only, owned by the viewer.
	sum := sha256.Sum256([]byte(tok))
	if got := e.appOne("SELECT CONCAT_WS(' ', web_user_id, name, HEX(token_sha256)) "+
		"FROM web_mcp_tokens WHERE id = ?", id); got != "3 ci runner "+strings.ToUpper(hex.EncodeToString(sum[:])) {
		t.Errorf("stored %q", got)
	}
	// Never shown again.
	_, p := e.page("admin", "/mcp-admin/tokens")
	noSecrets(t, "tokens page", p, tok, tok[6:])
	if !strings.Contains(js(p), `"name":"ci runner"`) {
		t.Errorf("tokens page %s", js(p))
	}

	e.want(204, "admin", "POST", fmt.Sprintf("/api/mcp/tokens/%d/revoke", id), nil)
	e.want(204, "admin", "POST", fmt.Sprintf("/api/mcp/tokens/%d/revoke", id), nil)
	if e.appOne("SELECT revoked_at IS NOT NULL FROM web_mcp_tokens WHERE id = ?", id) != "1" {
		t.Error("not revoked")
	}
	e.want(404, "admin", "POST", "/api/mcp/tokens/999/revoke", nil)

	want := []string{
		fmt.Sprintf("3 mcp_token_create token %d ci runner", id),
		fmt.Sprintf("3 owner_set mcp_token %d", id),
		fmt.Sprintf("3 mcp_token_revoke token %d ci runner", id),
	}
	got := e.audits()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	noSecrets(t, "audit", got, tok[6:])

	for _, c := range []struct {
		role   string
		path   string
		body   map[string]any
		status int
	}{
		{"admin", "/api/mcp/tokens", map[string]any{"name": "   "}, 400},
		{"admin", "/api/mcp/tokens", map[string]any{}, 400},
		{"member", "/api/mcp/tokens", map[string]any{"name": "x"}, 404},
		{"member", "/api/mcp/tokens/1/revoke", nil, 404},
		{"", "/api/mcp/tokens", map[string]any{"name": "x"}, 401},
	} {
		if st, b, _ := e.call(c.role, "POST", c.path, c.body); st != c.status {
			t.Errorf("%s %s %v: %d %v", c.role, c.path, c.body, st, b)
		}
	}
	if st, _, _ := e.call("admin", "POST", "/api/mcp/tokens/1/revoke", nil, false); st != 403 {
		t.Errorf("no csrf %d", st)
	}
	if e.appOne("SELECT revoked_at IS NULL FROM web_mcp_tokens WHERE id = 1") != "1" {
		t.Error("token 1 revoked by a refused request")
	}
	// A long name is cut to the column's 64 characters.
	b = e.want(201, "admin", "POST", "/api/mcp/tokens", map[string]any{"name": strings.Repeat("e", 70)})
	if n := e.appOne("SELECT CHAR_LENGTH(name) FROM web_mcp_tokens WHERE id = ?", b["id"]); n != "64" {
		t.Errorf("name of %s characters", n)
	}
}

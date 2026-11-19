// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Filing joined groups into federations (join-chat-federations).

// joinFilingEnv is an adminEnv where admin gw_admin (3) holds
// join_chat and administers federations 9 and 10 but not 11.
func joinFilingEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t)
	for _, q := range []string{
		"UPDATE web_users SET join_chat = 1 WHERE id = 3",
		"INSERT INTO web_federations (id, name) VALUES (9, 'Lab'), (10, 'Ops'), (11, 'Other')",
		"INSERT INTO web_federation_admins (federation_id, web_user_id) VALUES (9, 3), (10, 3)",
	} {
		if _, err := e.app.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// fedGroups is the stored groups of federation fed, as "id,id".
func (e *adminEnv) fedGroups(fed int) string {
	e.t.Helper()
	rows, err := e.app.Query("SELECT group_id FROM web_federation_groups WHERE federation_id = ? "+
		"ORDER BY group_id", fed)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, g)
	}
	return strings.Join(out, ",")
}

// sweep runs one pass of the background filer.
func (e *adminEnv) sweep() {
	e.t.Helper()
	f := NewJoinFiler(slog.New(slog.NewTextHandler(io.Discard, nil)), e.ro, e.app)
	if err := f.pass(context.Background()); err != nil {
		e.t.Fatal(err)
	}
}

// pending is the held filings on role's Join chat page, as
// "ref fed by chat_id".
func (e *adminEnv) pending(role string) []string {
	e.t.Helper()
	st, p := e.page(role, "/join-chat")
	if st != 200 {
		e.t.Fatalf("%s page %d", role, st)
	}
	var out []string
	for _, j := range p["props"].(map[string]any)["pending"].([]any) {
		j := j.(map[string]any)
		out = append(out, j["ref"].(string)+" "+j["federation"].(map[string]any)["name"].(string)+
			" "+j["by"].(string)+" "+js(j["chat_id"]))
	}
	return out
}

// An admin with join_chat joins a group and ticks two of its
// federations: the group is in both, and only there.
func TestJoinFilingJoined(t *testing.T) {
	t.Parallel()
	e := joinFilingEnv(t)
	e.daemon.answer("/v1/chats/join", 200, "", strings.Replace(previewChat, `"ok"`, `"joined"`, 1))
	b := e.want(200, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{10, 0, 9, 9}})
	if js(b["federation_ids"]) != "[9,10]" || b["filing_pending"] != false {
		t.Errorf("answer %v", b)
	}
	for fed, want := range map[int]string{9: "-1001000000001", 10: "-1001000000001", 11: ""} {
		if got := e.fedGroups(fed); got != want {
			t.Errorf("federation %d holds %q, want %q", fed, got, want)
		}
	}
	if !auditHas(e, []string{"federation 9; groups +1 -0; joined -1001000000001",
		"federation 10; groups +1 -0; joined -1001000000001"}) {
		t.Errorf("audit %v", e.audits())
	}
	// A federation not the admin's is refused before anything is
	// joined.
	asked := len(e.daemon.calls())
	e.want(404, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{9, 11}})
	if c := e.daemon.calls(); len(c) != asked {
		t.Errorf("the daemon was asked: %q", c)
	}

	// None chosen, or a group the account was in already: nothing is
	// filed.
	e.daemon.answer("/v1/chats/join", 200, "", strings.NewReplacer(`"ok"`, `"already_member"`,
		"-1001000000001", "-1001000000002").Replace(previewChat))
	b = e.want(200, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{9}})
	if js(b["federation_ids"]) != "[]" || b["filing_pending"] != false {
		t.Errorf("already a member %v", b)
	}
	if got := e.fedGroups(9); got != "-1001000000001" {
		t.Errorf("federation 9 holds %q", got)
	}
	// A preview says nothing of filing.
	e.daemon.answer("/v1/chats/resolve", 200, "", previewChat)
	b = e.want(200, "admin", "POST", "/api/join-chat/preview", map[string]any{"ref": "@gnuweeb"})
	if _, ok := b["federation_ids"]; ok {
		t.Errorf("preview %v", b)
	}
	if _, ok := b["filing_pending"]; ok {
		t.Errorf("preview %v", b)
	}
}

// A join that waits for approval holds the choice; the sweep files the
// group once the logger counts messages of it, and a later look at a
// chat behind an invite link files that one.
func TestJoinFilingHeld(t *testing.T) {
	t.Parallel()
	e := joinFilingEnv(t)
	// A public group the logger has no row of yet.
	e.daemon.answer("/v1/chats/join", 200, "", strings.NewReplacer(`"ok"`, `"request_sent"`,
		"-1001000000001", "-1001000000099", "@gnuweeb", "@closedlab", "GNU/Weeb", "Closed Lab").
		Replace(previewChat))
	b := e.want(200, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@closedlab", "federation_ids": []int{9}})
	if js(b["federation_ids"]) != "[9]" || b["filing_pending"] != true {
		t.Errorf("held %v", b)
	}
	// A private group behind a link, whose id the account cannot see.
	invite := `{"outcome": "approval_required", "message": "", "ref": "https://t.me/+AbCdEfGhIjKl",
		"kind": "invite", "chat": {"chat_id": null, "title": "Hidden", "type": "supergroup",
		"member_count": 3, "is_member": false, "banned": false, "needs_approval": true,
		"paid": false, "username": null}}`
	e.daemon.answer("/v1/chats/join", 200, "", invite)
	b = e.want(200, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "t.me/+AbCdEfGhIjKl", "federation_ids": []int{10}})
	if js(b["federation_ids"]) != "[10]" || b["filing_pending"] != true {
		t.Errorf("held invite %v", b)
	}
	// One the superadmin holds, which the admin neither sees nor drops.
	e.daemon.answer("/v1/chats/join", 200, "", strings.NewReplacer(`"ok"`, `"request_sent"`,
		"-1001000000001", "-1001000000098", "@gnuweeb", "@otherlab").Replace(previewChat))
	e.want(200, "superadmin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@otherlab", "federation_ids": []int{11}})

	want := []string{"@closedlab Lab gw_admin -1001000000099", "https://t.me/+AbCdEfGhIjKl Ops gw_admin 0"}
	if got := e.pending("admin"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("admin's pending %q", got)
	}
	if got := e.pending("superadmin"); len(got) != 3 || got[2] != "@otherlab Other admin -1001000000098" {
		t.Errorf("superadmin's pending %q", got)
	}
	if e.fedGroups(9) != "" || e.fedGroups(10) != "" {
		t.Errorf("filed before approval: %q %q", e.fedGroups(9), e.fedGroups(10))
	}

	// No message yet: the sweep leaves it held. The account starts
	// receiving the group's messages: the sweep files it.
	e.sweep()
	if e.fedGroups(9) != "" {
		t.Errorf("filed without a message")
	}
	if _, err := e.ro.Exec("INSERT INTO telegram_groups (id, type, title, msg_count, created_at, " +
		"updated_at) VALUES (-1001000000099, 'supergroup', 'Closed Lab', 2, NOW(), NOW())"); err != nil {
		t.Fatal(err)
	}
	e.sweep()
	if got := e.fedGroups(9); got != "-1001000000099" {
		t.Errorf("after the sweep federation 9 holds %q", got)
	}
	if !auditHas(e, []string{"federation 9; groups +1 -0; approved -1001000000099"}) {
		t.Errorf("audit %v", e.audits())
	}

	// The private group: approved, and previewed again.
	e.daemon.answer("/v1/chats/resolve", 200, "", strings.NewReplacer(`"approval_required"`, `"ok"`,
		`"chat_id": null`, `"chat_id": -1001000000097`, `"is_member": false`, `"is_member": true`).
		Replace(invite))
	e.want(200, "admin", "POST", "/api/join-chat/preview", map[string]any{"ref": "t.me/+AbCdEfGhIjKl"})
	if got := e.fedGroups(10); got != "-1001000000097" {
		t.Errorf("after the preview federation 10 holds %q", got)
	}
	if got := e.pending("admin"); len(got) != 0 {
		t.Errorf("still held %q", got)
	}

	// Dropping: not the admin's, then the superadmin's own.
	if got := e.pending("superadmin"); len(got) != 1 {
		t.Fatalf("superadmin's pending %q", got)
	}
	_, p := e.page("superadmin", "/join-chat")
	id := int(p["props"].(map[string]any)["pending"].([]any)[0].(map[string]any)["id"].(float64))
	path := "/api/join-chat/pending/" + strconv.Itoa(id)
	if st, _, _ := e.call("admin", "DELETE", path, nil); st != 404 {
		t.Errorf("admin drops the superadmin's: %d", st)
	}
	if st, _, _ := e.call("superadmin", "DELETE", path, nil, false); st != 403 {
		t.Errorf("without csrf: %d", st)
	}
	if st, _, _ := e.call("member", "DELETE", path, nil); st != 404 {
		t.Errorf("member: %d", st)
	}
	if st, _, _ := e.call("superadmin", "DELETE", path, nil); st != http.StatusNoContent {
		t.Errorf("drop: %d", st)
	}
	if st, _, _ := e.call("superadmin", "DELETE", path, nil); st != 404 {
		t.Errorf("drop again: %d", st)
	}
	if got := e.pending("superadmin"); len(got) != 0 {
		t.Errorf("still held %q", got)
	}
}

// A held filing whose account lost the federation meanwhile is dropped
// unfiled.
func TestJoinFilingLostFederation(t *testing.T) {
	t.Parallel()
	e := joinFilingEnv(t)
	e.daemon.answer("/v1/chats/join", 200, "", strings.NewReplacer(`"ok"`, `"request_sent"`,
		"-1001000000001", "-1001000000002").Replace(previewChat))
	e.want(200, "admin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{9}})
	if _, err := e.app.Exec("DELETE FROM web_federation_admins WHERE federation_id = 9 AND web_user_id = 3"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ro.Exec("UPDATE telegram_groups SET msg_count = msg_count + 1 WHERE id = -1001000000002"); err != nil {
		t.Fatal(err)
	}
	e.sweep()
	if got := e.fedGroups(9); got != "" {
		t.Errorf("filed into a lost federation: %q", got)
	}
	if got := e.pending("superadmin"); len(got) != 0 {
		t.Errorf("still held %q", got)
	}
	if !auditHas(e, []string{"@gnuweeb federation 9 chat=-1001000000002 no longer the account's"}) {
		t.Errorf("audit %v", e.audits())
	}
}

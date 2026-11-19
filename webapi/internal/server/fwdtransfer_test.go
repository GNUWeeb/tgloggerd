// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/webdb"
)

// fwdRoute is configuration id of a forwarding page's routes as role
// sees it, nil when it does not see it.
func fwdRoute(t *testing.T, e *adminEnv, role, page string, id float64) map[string]any {
	t.Helper()
	_, p := e.page(role, page)
	for _, r := range p["props"].(map[string]any)["routes"].([]any) {
		if m := r.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

// candidateIDs is the ids of a configuration's transfer_candidates,
// ascending.
func candidateIDs(r map[string]any) string {
	var ids []string
	for _, c := range r["transfer_candidates"].([]any) {
		ids = append(ids, fmt.Sprint(c.(map[string]any)["id"]))
	}
	slices.Sort(ids)
	return strings.Join(ids, ",")
}

// An admin gives a configuration of its federation to the federation's
// admins and to its members holding forward that see the group; a
// superadmin to any active account. The owner it is given to sees it,
// and disabling that owner flags it without stopping it.
func TestForwardingTransfer(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	const page = "/platform-fwd/telegram-discord"
	tr := func(role, base string, id, to any) (int, map[string]any) {
		st, b, _ := e.call(role, "POST", fmt.Sprintf("%s/%v/transfer", base, id), map[string]any{"owner_id": to})
		return st, b
	}

	// Webhook 1 (GNU/Weeb, Default) is shared. intern2025 (7) is
	// disabled; the members (5, 6) hold no forward yet.
	if c := candidateIDs(fwdRoute(t, e, "superadmin", page, 1)); c != "1,2,3,4,5,6" {
		t.Errorf("superadmin's candidates %s", c)
	}
	if c := candidateIDs(fwdRoute(t, e, "admin", page, 1)); c != "3,4" {
		t.Errorf("admin's candidates %s", c)
	}
	grantMember(t, e, []int64{gnuWeeb}, "forward")
	if c := candidateIDs(fwdRoute(t, e, "admin", page, 1)); c != "3,4,5" {
		t.Errorf("admin's candidates with a forwarding member %s", c)
	}
	// Not to an account outside the candidates.
	for _, to := range []any{1, 6, 7, 99, nil} {
		if st, b := tr("admin", t2dBase, 1, to); st != 400 || b["error"] != "bad_request" {
			t.Errorf("to %v: %d %v", to, st, b)
		}
	}
	if st, b := tr("admin", t2dBase, 1, 4); st != 204 {
		t.Fatalf("shared to an admin: %d %v", st, b)
	}
	if o := e.ownerOf(webdb.ResT2DWebhook, 1); o != 4 {
		t.Errorf("owner %d", o)
	}
	if st, b := tr("admin", t2dBase, 1, 5); st != 204 {
		t.Fatalf("to the member: %d %v", st, b)
	}
	// The member sees it now, and cannot give it on.
	if r := fwdRoute(t, e, "member", page, 1); r == nil || candidateIDs(r) != "" {
		t.Errorf("member's view %v", r)
	}
	if st, _ := tr("member", t2dBase, 1, 3); st != 404 {
		t.Errorf("member transfers: %d", st)
	}
	if st, _ := tr("admin", t2dBase, 99, 3); st != 404 {
		t.Errorf("missing webhook: %d", st)
	}

	// Disabling the owner flags the webhook; it keeps forwarding.
	if _, err := e.app.Exec("UPDATE web_users SET is_active = 0 WHERE id = 5"); err != nil {
		t.Fatal(err)
	}
	r := fwdRoute(t, e, "admin", page, 1)
	if js(r["owner"]) != canon(`{"id":5,"name":"researcher","role":"member","active":false}`) ||
		r["enabled"] != true {
		t.Errorf("disabled owner's webhook %s %v", js(r["owner"]), r["enabled"])
	}
	_, ip := e.page("admin", "/platform-fwd")
	if a := ip["props"].(map[string]any)["t2d"].(map[string]any)["active"]; a != float64(2) {
		t.Errorf("active webhooks %v", a)
	}
	if e.one("SELECT enabled FROM telegram_discord_webhooks WHERE id = 1") != "1" {
		t.Error("webhook disabled")
	}
	if c := candidateIDs(r); c != "3,4" {
		t.Errorf("candidates beside a disabled owner %s", c)
	}
	if st, b := tr("admin", t2dBase, 1, 3); st != 204 {
		t.Fatalf("from the disabled owner: %d %v", st, b)
	}

	// A configuration of a federation the admin does not administer:
	// it sees it, and may give it to nobody.
	if _, err := e.app.Exec("INSERT INTO web_federations (id, name) VALUES (9, 'Lab')"); err != nil {
		t.Fatal(err)
	}
	if err := webdb.FileResource(context.Background(), e.app,
		webdb.Resource{Type: webdb.ResT2DWebhook, ID: 3}, 9); err != nil {
		t.Fatal(err)
	}
	if r := fwdRoute(t, e, "admin", page, 3); r == nil || candidateIDs(r) != "" {
		t.Errorf("other federation's webhook %v", r)
	}
	if st, _ := tr("admin", t2dBase, 3, 4); st != 400 {
		t.Errorf("transfer outside its federations: %d", st)
	}
	if c := candidateIDs(fwdRoute(t, e, "superadmin", page, 3)); c != "1,2,3,4,6" {
		t.Errorf("superadmin's candidates for it %s", c)
	}

	// A route goes to a member only with both its bots.
	if _, err := e.app.Exec("UPDATE web_users SET is_active = 1 WHERE id = 5"); err != nil {
		t.Fatal(err)
	}
	if st, b := tr("admin", d2tBase, 1, 5); st != 409 || b["error"] != "bot_not_owned" {
		t.Errorf("route without its bots: %d %v", st, b)
	}
	ctx := context.Background()
	if err := webdb.SetOwner(ctx, e.app, webdb.Resource{Type: webdb.ResTelegramBot, ID: 1}, 5,
		webdb.Deployment); err != nil {
		t.Fatal(err)
	}
	if st, b := tr("admin", d2tBase, 1, 5); st != 409 || b["error"] != "bot_not_owned" {
		t.Errorf("route with one of its bots: %d %v", st, b)
	}
	if err := webdb.SetOwner(ctx, e.app, webdb.Resource{Type: webdb.ResDiscordBot,
		ID: int64(discordBot)}, 5, webdb.Deployment); err != nil {
		t.Fatal(err)
	}
	if st, b := tr("admin", d2tBase, 1, 5); st != 204 {
		t.Errorf("route with both bots: %d %v", st, b)
	}
	if fwdRoute(t, e, "member", "/platform-fwd/discord-telegram", 1) == nil {
		t.Error("member does not see the route given to it")
	}
	// To an admin, the bots stay whose they are.
	if st, b := tr("superadmin", d2tBase, 1, 4); st != 204 {
		t.Errorf("route to an admin: %d %v", st, b)
	}

	want := []string{
		"3 owner_set t2d_webhook 1",
		fmt.Sprintf("3 fwd_t2d_transfer webhook 1 chat %d from 0 to 4", gnuWeeb),
		"3 owner_transfer t2d_webhook 1 from 4 to 5",
		fmt.Sprintf("3 fwd_t2d_transfer webhook 1 chat %d from 4 to 5", gnuWeeb),
		"3 owner_transfer t2d_webhook 1 from 5 to 3",
		fmt.Sprintf("3 fwd_t2d_transfer webhook 1 chat %d from 5 to 3", gnuWeeb),
		"0 owner_set telegram_bot 1",
		fmt.Sprintf("0 owner_set discord_bot %d", discordBot),
		"3 owner_set d2t_route 1",
		fmt.Sprintf("3 fwd_d2t_transfer route 1 chat %d from 0 to 5", gnuWeeb),
		"1 owner_transfer d2t_route 1 from 5 to 4",
		fmt.Sprintf("1 fwd_d2t_transfer route 1 chat %d from 5 to 4", gnuWeeb),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A configuration touching a group hidden from an admin does not exist
// for it: not listed, counted, changed or transferred.
func TestForwardingHiddenGroup(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) VALUES (3, ?)",
		gnuWeeb); err != nil {
		t.Fatal(err)
	}
	if fwdRoute(t, e, "admin", "/platform-fwd/telegram-discord", 1) != nil ||
		fwdRoute(t, e, "admin", "/platform-fwd/discord-telegram", 1) != nil {
		t.Error("admin sees a configuration of a hidden group")
	}
	_, ip := e.page("admin", "/platform-fwd")
	if s := js(ip["props"]); s != `{"d2t":{"active":0,"sent":0,"total":1},"t2d":{"active":1,"sent":0,"total":2}}` {
		t.Errorf("index counts a hidden group's configurations: %s", s)
	}
	for _, c := range []struct{ method, path string }{
		{"PATCH", t2dBase + "/1"},
		{"DELETE", t2dBase + "/1"},
		{"POST", t2dBase + "/1/transfer"},
		{"PATCH", d2tBase + "/1"},
		{"POST", d2tBase + "/1/transfer"},
	} {
		if st, _, _ := e.call("admin", c.method, c.path, map[string]any{"owner_id": 4}); st != 404 {
			t.Errorf("%s %s: %d", c.method, c.path, st)
		}
	}
	if fwdRoute(t, e, "superadmin", "/platform-fwd/telegram-discord", 1) == nil {
		t.Error("superadmin does not see it")
	}
}

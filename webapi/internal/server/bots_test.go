// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The bots operations and page on the "forwarding" and "bots" fixtures
// (adminEnv). The tokens are made up.

const (
	botsBase = "/api/bots"
	// Shaped like Discord's: the first part is base64url of a snowflake.
	newDiscordToken   = "MTAwMDAwMDAwMDAwMDAwMDA5OQ.GtestA.testSecretDiscordNew"
	otherDiscordToken = "MTAwMDAwMDAwMDAwMDAwMDA5OA.GtestB.testSecretDiscordOther"
)

// fakeTelegramToken has a Telegram bot token's shape and is plainly not
// a real one, so secret scanners leave it alone.
var fakeTelegramToken = "7000000001:AA" + strings.Repeat("x", 33)

func TestBotTokenShapes(t *testing.T) {
	t.Parallel()
	for tok, want := range map[string]bool{
		fakeTelegramToken:                    true,
		"1:" + strings.Repeat("a", 30):       true,
		"1:" + strings.Repeat("a", 29):       false,
		"12ab:" + strings.Repeat("a", 30):    false,
		":" + strings.Repeat("a", 30):        false,
		"1:" + strings.Repeat("a", 29) + "!": false,
		"1:" + strings.Repeat("a", 29) + " ": false,
		"1:" + strings.Repeat("a", 300):      false,
		fakeTelegramToken + ":x":             false,
	} {
		if got := logdb.ValidTelegramToken(tok); got != want {
			t.Errorf("telegram %q: %v, want %v", tok, got, want)
		}
	}
	for tok, want := range map[string]bool{
		newDiscordToken:                    true,
		"MTAwMDAwMDAwMDAwMDAwMDA5OQ.a.b":   true,
		"MTAwMDAwMDAwMDAwMDAwMDA5OQ.a":     false,
		"MTAwMDAwMDAwMDAwMDAwMDA5OQ..b":    false,
		"MTAwMDAwMDAwMDAwMDAwMDA5OQ.a.b.c": false,
		"aGVsbG8.a.b":                      false, // "hello", not an id
		"MA.a.b":                           false, // "0"
		"MTAwMDAwMDAwMDAwMDAwMDA5OQ.a+.b":  false,
		fakeTelegramToken:                  false,
	} {
		if got := logdb.ValidDiscordToken(tok); got != want {
			t.Errorf("discord %q: %v, want %v", tok, got, want)
		}
	}
	if logdb.KindOf(1) != logdb.KindTelegram || logdb.KindOf(logdb.DiscordBotIDBase) != logdb.KindDiscord {
		t.Error("KindOf")
	}
}

func botByID(t *testing.T, b map[string]any, id uint64) map[string]any {
	t.Helper()
	for _, x := range b["props"].(map[string]any)["bots"].([]any) {
		if m := x.(map[string]any); m["id"] == float64(id) {
			return m
		}
	}
	return nil
}

func TestBotsPage(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, role := range []string{"superadmin", "admin"} {
		st, b := e.page(role, "/platform-fwd/bots")
		if st != 200 || b["route"] != "bots" {
			t.Fatalf("%s: %d %v", role, st, b)
		}
		noSecrets(t, role+" bots page", b)
		props := b["props"].(map[string]any)
		if props["show_owner"] != true || len(props["bots"].([]any)) != 4 {
			t.Errorf("%s props %s", role, js(props))
		}
	}
	_, b := e.page("admin", "/platform-fwd/bots")
	// The admin's candidates, by username: the superadmins, the
	// admins and the members of its federation; not the disabled one.
	cands := `[{"id":1,"name":"admin","role":"superadmin","active":true},` +
		`{"id":3,"name":"gw_admin","role":"admin","active":true},` +
		`{"id":6,"name":"mod_kernel","role":"member","active":true},` +
		`{"id":2,"name":"ops","role":"superadmin","active":true},` +
		`{"id":5,"name":"researcher","role":"member","active":true},` +
		`{"id":4,"name":"sec_lead","role":"admin","active":true}]`
	if got := js(botByID(t, b, 1000000002)); got != canon(`{"id":1000000002,"kind":"discord",`+
		`"name":"Old reader","identity":null,"status":"token_refused","enabled":false,"photo":null,"profile":null,`+
		`"owner":{"id":0,"name":"Shared","active":true},"transfer_candidates":`+cands+`,`+
		`"status_at":"2026-11-18T02:00:00Z",`+
		`"status_detail":"Discord refused the token (close code 4004).","routes":0}`) {
		t.Errorf("refused bot %s", got)
	}
	if got := botByID(t, b, discordBot); got["routes"] != float64(1) || got["status"] != "connected" ||
		got["status_detail"] != nil {
		t.Errorf("connected bot %s", js(got))
	}
	if got := botByID(t, b, 2); got["name"] != "Bot 2" || got["status"] != "disabled" {
		t.Errorf("telegram bot 2 %s", js(got))
	}
	if st, b := e.page("member", "/platform-fwd/bots"); st != 404 || b["route"] != "not_found" {
		t.Errorf("member: %d %v", st, b)
	}

	// The pickers' list, by kind.
	lb := e.want(200, "admin", "GET", botsBase+"?kind=discord", nil)
	noSecrets(t, "list", lb)
	if got := js(lb); got != canon(`{"bots":[`+
		`{"id":1000000001,"kind":"discord","name":"Main reader","identity":{"user_id":"1000000000000000001",`+
		`"username":"gw_reader"},"status":"connected","enabled":true,"photo":null,"profile":null},`+
		`{"id":1000000002,"kind":"discord","name":"Old reader","identity":null,`+
		`"status":"token_refused","enabled":false,"photo":null,"profile":null}]}`) {
		t.Errorf("list %s", got)
	}
	if lb := e.want(200, "superadmin", "GET", botsBase, nil); len(lb["bots"].([]any)) != 4 {
		t.Errorf("list all %v", lb)
	}
	if st, b, _ := e.call("admin", "GET", botsBase+"?kind=slack", nil); st != 400 || b["error"] != "bad_request" {
		t.Errorf("bad kind %d %v", st, b)
	}
}

func TestBotOperations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	admin := e.uids["admin"]

	b := e.want(201, "admin", "POST", botsBase, map[string]any{"kind": "discord",
		"name": "  Second reader ", "token": newDiscordToken})
	noSecrets(t, "create", b, "testSecretDiscord")
	id := uint64(b["id"].(float64))
	if id < logdb.DiscordBotIDBase || b["name"] != "Second reader" || b["status"] != "connecting" ||
		b["enabled"] != true || b["identity"] != nil || b["owner"].(map[string]any)["id"] != float64(admin) {
		t.Errorf("created %s", js(b))
	}
	if got := e.one("SELECT CONCAT_WS(' ', name, token, enabled, status) FROM discord_bots WHERE id = ?",
		id); got != "Second reader "+newDiscordToken+" 1 connecting" {
		t.Errorf("stored %q", got)
	}
	if o := e.ownerOf(webdb.ResDiscordBot, int64(id)); o != admin {
		t.Errorf("owner %d", o)
	}

	// Its owner adding it again gets it back; another account is
	// refused without being told whose it is.
	again := e.want(201, "admin", "POST", botsBase, map[string]any{"kind": "discord",
		"name": "x", "token": newDiscordToken})
	if again["id"] != float64(id) || again["name"] != "Second reader" {
		t.Errorf("re-add %s", js(again))
	}
	st, rb, _ := e.call("superadmin", "POST", botsBase, map[string]any{"kind": "discord",
		"name": "x", "token": newDiscordToken})
	if st != 409 || rb["error"] != "token_in_use" || strings.Contains(js(rb), "gw_admin") {
		t.Errorf("other's token %d %v", st, rb)
	}
	// An unowned (shared) bot is the superadmins'.
	tok1 := "7000000001:fixtureBotSecretOne" + strings.Repeat("A", 20)
	if _, err := e.ro.Exec("UPDATE telegram_bots SET token = ? WHERE id = 1", tok1); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := e.call("admin", "POST", botsBase, map[string]any{"kind": "telegram",
		"name": "x", "token": tok1}); st != 409 {
		t.Errorf("admin re-adds a shared bot: %d", st)
	}
	if b := e.want(201, "superadmin", "POST", botsBase, map[string]any{"kind": "telegram",
		"name": "x", "token": tok1}); b["id"] != float64(1) {
		t.Errorf("superadmin re-adds the shared bot %v", b)
	}

	path := fmt.Sprintf("%s/%d", botsBase, id)
	// Off, then on: on is connecting, one more try for discordd.
	b = e.want(200, "admin", "PATCH", path, map[string]any{"enabled": false, "name": "Renamed"})
	if b["status"] != "disabled" || b["enabled"] != false || b["name"] != "Renamed" {
		t.Errorf("disabled %s", js(b))
	}
	if _, err := e.ro.Exec("UPDATE discord_bots SET status = 'intent_missing', "+
		"status_detail = 'no intent', bot_user_id = 42, username = 'r' WHERE id = ?", id); err != nil {
		t.Fatal(err)
	}
	b = e.want(200, "admin", "PATCH", path, map[string]any{"enabled": true})
	if b["status"] != "connecting" || b["enabled"] != true || b["status_detail"] != nil {
		t.Errorf("enabled again %s", js(b))
	}
	// A new token forgets the identity and connects again.
	b = e.want(200, "admin", "PATCH", path, map[string]any{"token": otherDiscordToken})
	noSecrets(t, "patch", b, "testSecretDiscord")
	if b["identity"] != nil || b["status"] != "connecting" ||
		e.one("SELECT token FROM discord_bots WHERE id = ?", id) != otherDiscordToken {
		t.Errorf("new token %s", js(b))
	}
	if st, b, _ := e.call("admin", "PATCH", path, map[string]any{
		"token": "MTAwMDAwMDAwMDAwMDAwMDAwMQ.GfixA.fixtureBotSecretDiscordOne"}); st != 409 ||
		b["error"] != "token_in_use" {
		t.Errorf("another bot's token %d %v", st, b)
	}

	// A bot routes use cannot go; an unused one can.
	if st, b, _ := e.call("admin", "DELETE", fmt.Sprintf("%s/%d", botsBase, discordBot), nil); st != 409 ||
		b["error"] != "bot_in_use" || !strings.Contains(b["message"].(string), "1 route") {
		t.Errorf("delete used %d %v", st, b)
	}
	e.want(204, "admin", "DELETE", path, nil)
	e.want(404, "admin", "DELETE", path, nil)
	if o := e.ownerOf(webdb.ResDiscordBot, int64(id)); o != 0 {
		t.Errorf("owner after delete %d", o)
	}

	want := []string{
		fmt.Sprintf(`3 bot_create bot %d kind discord name "Second reader"`, id),
		fmt.Sprintf("3 owner_set discord_bot %d", id),
		fmt.Sprintf(`3 bot_update bot %d name "Renamed" enabled=false`, id),
		fmt.Sprintf("3 bot_update bot %d enabled=true", id),
		fmt.Sprintf("3 bot_update bot %d token", id),
		fmt.Sprintf("3 bot_delete bot %d kind discord", id),
		fmt.Sprintf("3 resource_drop discord_bot %d", id),
	}
	got := e.audits()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	noSecrets(t, "audit", got, "testSecretDiscord")
}

func TestBotRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, c := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{map[string]any{"kind": "discord", "name": "x"}, 400, "bad_request"},
		{map[string]any{"kind": "slack", "name": "x", "token": newDiscordToken}, 400, "bad_request"},
		{map[string]any{"kind": "discord", "name": " ", "token": newDiscordToken}, 400, "bad_request"},
		{map[string]any{"kind": "discord", "name": strings.Repeat("é", 65), "token": newDiscordToken}, 400, "bad_request"},
		{map[string]any{"kind": "discord", "name": "x", "token": newToken}, 400, "bad_token"},
		{map[string]any{"kind": "telegram", "name": "x", "token": newDiscordToken}, 400, "bad_token"},
		{map[string]any{"kind": "telegram", "name": "x", "token": "7000000099:short"}, 400, "bad_token"},
	} {
		st, b, _ := e.call("admin", "POST", botsBase, c.body)
		if st != c.status || b["error"] != c.code || b["message"] == "" {
			t.Errorf("%v: %d %v, want %d %s", c.body, st, b, c.status, c.code)
		}
		noSecrets(t, "refusal", b, "testSecretDiscord", "testSecretNewBot")
	}
	if st, b, _ := e.call("admin", "PATCH", botsBase+"/1", map[string]any{"token": newDiscordToken}); st != 400 ||
		b["error"] != "bad_token" {
		t.Errorf("a Discord token for a Telegram bot %d %v", st, b)
	}
	// A 64-character name is fine.
	e.want(200, "admin", "PATCH", botsBase+"/1", map[string]any{"name": strings.Repeat("é", 64)})

	ok := map[string]any{"kind": "discord", "name": "x", "token": newDiscordToken}
	ops := [][2]string{{"GET", botsBase}, {"POST", botsBase}, {"PATCH", botsBase + "/1"},
		{"DELETE", botsBase + "/2"}, {"POST", botsBase + "/2/transfer"}}
	for _, op := range ops {
		if st, b, _ := e.call("member", op[0], op[1], ok); st != 404 || b["error"] != "not_found" {
			t.Errorf("member %s %s: %d %v", op[0], op[1], st, b)
		}
		if st, _, _ := e.call("", op[0], op[1], ok); st != 401 {
			t.Errorf("anonymous %s %s: %d", op[0], op[1], st)
		}
		if op[0] != "GET" {
			if st, _, _ := e.call("admin", op[0], op[1], ok, false); st != 403 {
				t.Errorf("no csrf %s %s: %d", op[0], op[1], st)
			}
		}
	}
	for _, p := range []string{"/999", "/1000000099", "/0", "/x"} {
		if st, _, _ := e.call("admin", "PATCH", botsBase+p, map[string]any{}); st != 404 {
			t.Errorf("patch %s: %d", p, st)
		}
	}
	if n := e.one("SELECT COUNT(*) FROM discord_bots"); n != "2" {
		t.Errorf("%s Discord bots after refusals", n)
	}
	got := e.audits()
	if len(got) != 1 || !strings.HasPrefix(got[0], "3 bot_update bot 1 name") {
		t.Errorf("audits %v", got)
	}
}

func TestBotTransfer(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	tr := func(role string, id uint64, to any) (int, map[string]any) {
		st, b, _ := e.call(role, "POST", fmt.Sprintf("%s/%d/transfer", botsBase, id), map[string]any{"owner_id": to})
		return st, b
	}
	// A shared bot gets its first owner; then it moves on.
	if st, b := tr("superadmin", discordBot, 3); st != 204 {
		t.Fatalf("first transfer %d %v", st, b)
	}
	if o := e.ownerOf(webdb.ResDiscordBot, int64(discordBot)); o != 3 {
		t.Errorf("owner %d", o)
	}
	if st, b := tr("admin", discordBot, 5); st != 204 {
		t.Fatalf("admin to its member %d %v", st, b)
	}
	if o := e.ownerOf(webdb.ResDiscordBot, int64(discordBot)); o != 5 {
		t.Errorf("owner %d", o)
	}
	// Not to an account outside the candidates: disabled, unknown.
	for _, to := range []any{7, 99, nil} {
		if st, b := tr("admin", 1, to); st != 400 || b["error"] != "bad_request" {
			t.Errorf("to %v: %d %v", to, st, b)
		}
	}
	if st, _ := tr("admin", 1000000099, 3); st != 404 {
		t.Errorf("missing bot %d", st)
	}
	want := []string{
		fmt.Sprintf("1 owner_set discord_bot %d", discordBot),
		fmt.Sprintf("1 bot_transfer bot %d from 0 to 3", discordBot),
		fmt.Sprintf("3 owner_transfer discord_bot %d from 3 to 5", discordBot),
		fmt.Sprintf("3 bot_transfer bot %d from 3 to 5", discordBot),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	_, b := e.page("superadmin", "/platform-fwd/bots")
	if got := botByID(t, b, discordBot)["owner"]; js(got) !=
		canon(`{"id":5,"name":"researcher","role":"member","active":true}`) {
		t.Errorf("owner answered %s", js(got))
	}
}

// A member with forward sees and uses only its own bots: here a member
// that owns one bot.
func TestBotAccessOwnBotsOnly(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	ctx := context.Background()
	if err := webdb.SetOwner(ctx, e.app, webdb.Resource{Type: webdb.ResDiscordBot,
		ID: int64(discordBot)}, 5, webdb.Deployment); err != nil {
		t.Fatal(err)
	}
	srv := &Server{ro: e.ro, app: e.app}
	for role, all := range map[scope.Role]bool{scope.Superadmin: true, scope.Admin: true} {
		a, err := srv.botAccessOf(ctx, scope.For(scope.Viewer{ID: 3, Role: role}))
		if err != nil || a.all != all {
			t.Errorf("%s: %v %v", role, a, err)
		}
	}
	// Without forward a member sees nothing, its own bot included.
	a, err := srv.botAccessOf(ctx, scope.For(scope.Viewer{ID: 5, Role: scope.Member}))
	if err != nil || a.all || a.sees(discordBot) || a.sees(1) {
		t.Errorf("member without forward %v %v", a, err)
	}
	// Given forward, it sees its own bot only.
	own, err := srv.botAccessOf(ctx, scope.For(scope.Viewer{ID: 5, Role: scope.Member,
		Abilities: []scope.Ability{scope.Forward}}))
	if err != nil || own.all || !own.sees(discordBot) || own.sees(1000000002) || own.sees(1) {
		t.Errorf("own %v %v", own, err)
	}
}

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
)

// The forwarding operations on the "forwarding" fixture (adminEnv).

const (
	t2dBase  = "/api/forwarding/telegram-discord"
	d2tBase  = "/api/forwarding/discord-telegram"
	rustChat = int64(-1001000000003)
	newHook  = "https://discord.com/api/webhooks/1100000000000000099/testSecretNewHook"
	newToken = "7000000099:testSecretNewBot"
)

func TestParseWebhook(t *testing.T) {
	t.Parallel()
	for u, want := range map[string]bool{
		"https://discord.com/api/webhooks/1/abc":                         true,
		"https://discordapp.com/api/webhooks/1/abc?wait=true":            true,
		"https://ptb.discord.com/api/webhooks/1/abc":                     true,
		"https://canary.discord.com/api/webhooks/1/abc":                  true,
		"http://discord.com/api/webhooks/1/abc":                          false,
		"https://discord.com.evil.example/api/webhooks/1/abc":            false,
		"https://evil.example/api/webhooks/1/abc":                        false,
		"https://discord.com@evil.example/api/webhooks/1/a":              false,
		"https://discord.com/api/webhooks/1/":                            false,
		"https://discord.com/api/webhooks//abc":                          false,
		"https://discord.com/api/webhooks/1":                             false,
		"https://discord.com/api/channels/1/abc":                         false,
		"https://discord.com":                                            false,
		"":                                                               false,
		"https://discord.com/api/webhooks/1/" + strings.Repeat("a", 500): false,
	} {
		if _, _, ok := parseWebhook(u); ok != want {
			t.Errorf("%q: %v, want %v", u, ok, want)
		}
	}
	if m := maskWebhook("https://discord.com/api/webhooks/123/sEcReT"); m !=
		"https://discord.com/api/webhooks/123/****" {
		t.Errorf("mask %q", m)
	}
}

func TestForwardingPages(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)

	for _, role := range []string{"superadmin", "admin"} {
		st, b := e.page(role, "/platform-fwd")
		if st != 200 || b["route"] != "platform_fwd" || js(b["props"]) !=
			`{"d2t":{"active":1,"sent":2,"total":2},"t2d":{"active":2,"sent":3,"total":3}}` {
			t.Fatalf("%s index: %d %v", role, st, b)
		}
	}

	_, b := e.page("admin", "/platform-fwd/telegram-discord")
	noSecrets(t, "webhooks page", b)
	routes := b["props"].(map[string]any)["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("webhooks %v", routes)
	}
	if got := js(routes[2]); got != canon(`{"id":1,"owner":{"id":0,"name":"Shared","active":true},`+
		`"federation":{"id":0,"name":"Default"},"transfer_candidates":[],`+
		`"chat":{"id":-1001000000001,"title":"GNU/Weeb","kind":"group","sub":"supergroup","photo":null},`+
		`"url_masked":"https://discord.com/api/webhooks/1100000000000000001/****",`+
		`"types":["sticker","text","photo","video","document","other"],"enabled":true,`+
		`"sent":3,"last_sent":"2026-11-17T11:00:01Z"}`) {
		t.Errorf("webhook 1 %s", got)
	}
	if got := js(routes[1].(map[string]any)["chat"]); got !=
		canon(`{"id":9299000001,"title":"Dewi Kusuma","kind":"user","photo":null}`) {
		t.Errorf("webhook 2 chat %s", got)
	}
	if got := js(routes[0].(map[string]any)["types"]); got != `["text"]` ||
		routes[0].(map[string]any)["enabled"] != false || routes[0].(map[string]any)["last_sent"] != nil {
		t.Errorf("webhook 3 %s", js(routes[0]))
	}
	if js(b["props"].(map[string]any)["federations"]) != `[{"id":0,"name":"Default"}]` {
		t.Errorf("federations %v", b["props"])
	}

	_, b = e.page("superadmin", "/platform-fwd/discord-telegram")
	noSecrets(t, "routes page", b)
	routes = b["props"].(map[string]any)["routes"].([]any)
	if len(routes) != 2 {
		t.Fatalf("routes %v", routes)
	}
	r1, r2 := routes[1].(map[string]any), routes[0].(map[string]any)
	if r1["channel_id"] != "710000000000000001" || r1["channel_name"] != "general" ||
		r1["bot"] != "@gw_bridge_bot" || r1["sent"] != float64(2) ||
		js(r1["chat"]) != canon(`{"id":-1001000000001,"title":"GNU/Weeb","kind":"group","sub":"supergroup","photo":null}`) {
		t.Errorf("route 1 %s", js(r1))
	}
	if r2["channel_name"] != "" || r2["bot"] != "bot #2 (not connected yet)" ||
		js(r2["types"]) != `["sticker","text","document"]` {
		t.Errorf("route 2 %s", js(r2))
	}

	// To a member the pages do not exist.
	for _, p := range []string{"/platform-fwd", "/platform-fwd/telegram-discord",
		"/platform-fwd/discord-telegram"} {
		if st, b := e.page("member", p); st != 404 || b["route"] != "not_found" {
			t.Errorf("member %s: %d %v", p, st, b)
		}
	}
}

// The lists apply the scope: a scope that sees no group sees only the
// private chats' configurations when it sees those, and none here.
func TestForwardingListsScope(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	ctx := context.Background()
	member := scope.For(scope.Viewer{ID: 5, Role: scope.Member, EmailVerified: true})
	ws, err := logdb.ListT2D(ctx, e.ro, member)
	if err != nil || len(ws) != 0 {
		t.Errorf("member webhooks %v %v", ws, err)
	}
	rs, err := logdb.ListD2T(ctx, e.ro, member)
	if err != nil || len(rs) != 0 {
		t.Errorf("member routes %v %v", rs, err)
	}
	if _, ok, err := logdb.GetT2D(ctx, e.ro, member, 1); ok || err != nil {
		t.Errorf("member webhook 1 %v %v", ok, err)
	}
	if _, ok, _ := logdb.ResolveChat(ctx, e.ro, member, gnuWeeb); ok {
		t.Error("member resolves a group it does not see")
	}
	if _, ok, _ := logdb.ResolveChat(ctx, e.ro, member, 9299000001); ok {
		t.Error("member resolves a private chat")
	}
	admin := scope.For(scope.Viewer{ID: 3, Role: scope.Admin, EmailVerified: true})
	if c, ok, _ := logdb.ResolveChat(ctx, e.ro, admin, 9299000001); !ok || c.Title != "Dewi Kusuma" {
		t.Errorf("admin resolves %v %v", c, ok)
	}
}

func TestT2DOperations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)

	create := map[string]any{"chat_id": rustChat, "chat_kind": "group", "webhook_url": newHook,
		"types": []string{"text", "photo"}, "enabled": true}
	b := e.want(201, "admin", "POST", t2dBase, create)
	noSecrets(t, "create", b, "testSecretNewHook")
	id := uint64(b["id"].(float64))
	if got := e.one("SELECT CONCAT_WS(' ', telegram_chat_id, telegram_chat_type, webhook_url, "+
		"enabled, fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document, fwd_other) "+
		"FROM telegram_discord_webhooks WHERE id = ?", id); got !=
		fmt.Sprintf("%d group %s 1 0 1 1 0 0 0", rustChat, newHook) {
		t.Errorf("stored %q", got)
	}

	// Change only what is given; the URL is write-only and stays.
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", t2dBase, id), map[string]any{"enabled": false})
	e.want(204, "superadmin", "PATCH", fmt.Sprintf("%s/%d", t2dBase, id),
		map[string]any{"types": []string{"other"}})
	if got := e.one("SELECT CONCAT_WS(' ', webhook_url, enabled, fwd_text, fwd_other) "+
		"FROM telegram_discord_webhooks WHERE id = ?", id); got != newHook+" 0 0 1" {
		t.Errorf("after patch %q", got)
	}
	other := "https://ptb.discord.com/api/webhooks/1100000000000000098/testSecretOther"
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", t2dBase, id), map[string]any{"webhook_url": other})
	if got := e.one("SELECT webhook_url FROM telegram_discord_webhooks WHERE id = ?", id); got != other {
		t.Errorf("url %q", got)
	}
	_, page := e.page("admin", "/platform-fwd/telegram-discord")
	noSecrets(t, "page", page, "testSecretNewHook", "testSecretOther")

	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", t2dBase, id), nil)
	if e.one("SELECT COUNT(*) FROM telegram_discord_webhooks WHERE id = ?", id) != "0" {
		t.Error("not deleted")
	}
	e.want(404, "admin", "DELETE", fmt.Sprintf("%s/%d", t2dBase, id), nil)

	// A private chat, as a user.
	b = e.want(201, "superadmin", "POST", t2dBase, map[string]any{"chat_id": 9299000002,
		"chat_kind": "user", "webhook_url": newHook, "types": []string{}, "enabled": false})
	priv := uint64(b["id"].(float64))

	want := []string{
		fmt.Sprintf("3 fwd_t2d_create webhook %d chat %d enabled=true types=text,photo", id, rustChat),
		fmt.Sprintf("3 owner_set t2d_webhook %d", id),
		fmt.Sprintf("3 fwd_t2d_update webhook %d chat %d enabled=false", id, rustChat),
		fmt.Sprintf("1 fwd_t2d_update webhook %d chat %d types=other", id, rustChat),
		fmt.Sprintf("3 fwd_t2d_update webhook %d chat %d url", id, rustChat),
		fmt.Sprintf("3 fwd_t2d_delete webhook %d chat %d", id, rustChat),
		fmt.Sprintf("3 resource_drop t2d_webhook %d", id),
		fmt.Sprintf("1 fwd_t2d_create webhook %d chat 9299000002 enabled=false types=", priv),
		fmt.Sprintf("1 owner_set t2d_webhook %d", priv),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	noSecrets(t, "audit", e.audits(), "testSecretNewHook", "testSecretOther")
}

func TestT2DRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	ok := map[string]any{"chat_id": gnuWeeb, "chat_kind": "group", "webhook_url": newHook,
		"types": []string{"text"}, "enabled": true}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range ok {
			m[a] = b
		}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, c := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{with("webhook_url", "https://evil.example/api/webhooks/1/x"), 400, "bad_webhook"},
		{with("webhook_url", "http://discord.com/api/webhooks/1/x"), 400, "bad_webhook"},
		{with("webhook_url", "https://discord.com/api/channels/1/x"), 400, "bad_webhook"},
		{with("types", []string{"text", "sound"}), 400, "bad_request"},
		{with("types", nil), 400, "bad_request"},
		{with("enabled", nil), 400, "bad_request"},
		{with("chat_kind", "user"), 400, "bad_request"},
		// A chat the logger never saw, and a federation the viewer
		// does not administer, do not exist.
		{with("chat_id", int64(-1009999999999)), 404, "not_found"},
		{with("federation_id", 7), 404, "not_found"},
	} {
		st, b, _ := e.call("admin", "POST", t2dBase, c.body)
		if st != c.status || b["error"] != c.code {
			t.Errorf("%v: %d %v, want %d %s", c.body, st, b, c.status, c.code)
		}
		if c.status == 400 && b["message"] == "" {
			t.Errorf("%v: no message", c.body)
		}
	}
	if st, b, _ := e.call("admin", "PATCH", t2dBase+"/1",
		map[string]any{"webhook_url": "https://example.org/hook"}); st != 400 || b["error"] != "bad_webhook" {
		t.Errorf("patch bad url %d %v", st, b)
	}
	// Without the CSRF token, nothing happens.
	if st, b, _ := e.call("admin", "POST", t2dBase, ok, false); st != 403 || b["error"] != "csrf" {
		t.Errorf("no csrf %d %v", st, b)
	}
	if st, _, _ := e.call("admin", "DELETE", t2dBase+"/1", nil, false); st != 403 {
		t.Errorf("delete without csrf %d", st)
	}
	// To a member the operations do not exist; without a session 401.
	for _, op := range [][2]string{{"POST", t2dBase}, {"PATCH", t2dBase + "/1"},
		{"DELETE", t2dBase + "/1"}, {"POST", t2dBase + "/test"}} {
		if st, b, _ := e.call("member", op[0], op[1], ok); st != 404 || b["error"] != "not_found" {
			t.Errorf("member %s %s: %d %v", op[0], op[1], st, b)
		}
		if st, _, _ := e.call("", op[0], op[1], ok); st != 401 {
			t.Errorf("anonymous %s %s: %d", op[0], op[1], st)
		}
	}
	for _, p := range []string{"/999", "/0", "/x"} {
		if st, _, _ := e.call("admin", "PATCH", t2dBase+p, map[string]any{}); st != 404 {
			t.Errorf("patch %s: %d", p, st)
		}
	}
	if n := e.one("SELECT COUNT(*) FROM telegram_discord_webhooks"); n != "3" {
		t.Errorf("%s webhooks after refusals", n)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("refusals audited: %v", a)
	}
}

func TestT2DTest(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.want(204, "admin", "POST", t2dBase+"/test", map[string]any{"id": 1})
	e.want(204, "admin", "POST", t2dBase+"/test", map[string]any{"webhook_url": newHook})
	sent := e.discord.sent()
	if len(sent) != 2 ||
		!strings.HasPrefix(sent[0], "discord.com/api/webhooks/1100000000000000001/fixtureSecretTokenOne {") ||
		!strings.HasPrefix(sent[1], "discord.com/api/webhooks/1100000000000000099/testSecretNewHook {") ||
		!strings.Contains(sent[0], `"username":"tgloggerd"`) {
		t.Errorf("posts %q", sent)
	}

	// Discord refuses: 502 with its answer, never the URL.
	e.discord.mu.Lock()
	e.discord.status = 404
	e.discord.mu.Unlock()
	st, b, _ := e.call("admin", "POST", t2dBase+"/test", map[string]any{"id": 1})
	if st != 502 || b["error"] != "send_failed" || !strings.Contains(b["message"].(string), "404") {
		t.Errorf("refused %d %v", st, b)
	}
	noSecrets(t, "refused", b)

	// Only Discord's URLs are posted to; a hidden or missing webhook
	// does not exist.
	for _, c := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{map[string]any{"webhook_url": "https://127.0.0.1/api/webhooks/1/x"}, 400, "bad_webhook"},
		{map[string]any{"webhook_url": "https://discord.com.evil.example/api/webhooks/1/x"}, 400, "bad_webhook"},
		{map[string]any{"id": 999}, 404, "not_found"},
		{map[string]any{}, 400, "bad_request"},
	} {
		if st, b, _ := e.call("admin", "POST", t2dBase+"/test", c.body); st != c.status || b["error"] != c.code {
			t.Errorf("%v: %d %v", c.body, st, b)
		}
	}
	// A stored URL that is not Discord's is not posted to either.
	if _, err := e.ro.Exec("UPDATE telegram_discord_webhooks SET webhook_url = " +
		"'https://evil.example/api/webhooks/1/x' WHERE id = 2"); err != nil {
		t.Fatal(err)
	}
	if st, b, _ := e.call("admin", "POST", t2dBase+"/test", map[string]any{"id": 2}); st != 400 ||
		b["error"] != "bad_webhook" {
		t.Errorf("stored bad url %d %v", st, b)
	}
	if n := len(e.discord.sent()); n != 3 {
		t.Errorf("%d posts, want 3", n)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("tests audited: %v", a)
	}
}

func TestD2TOperations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)

	create := map[string]any{"discord_channel_id": "710000000000000003", "chat_id": rustChat,
		"bot_token": newToken, "types": []string{"text", "document"}, "enabled": true}
	b := e.want(201, "admin", "POST", d2tBase, create)
	noSecrets(t, "create", b, "testSecretNewBot")
	id := uint64(b["id"].(float64))
	bot := e.one("SELECT telegram_bot_id FROM discord_telegram_routes WHERE id = ?", id)
	if got := e.one("SELECT CONCAT_WS(' ', r.discord_channel_id, r.telegram_chat_id, r.enabled, "+
		"r.fwd_sticker, r.fwd_text, r.fwd_photo, r.fwd_video, r.fwd_document, b.token) "+
		"FROM discord_telegram_routes r JOIN telegram_bots b ON b.id = r.telegram_bot_id "+
		"WHERE r.id = ?", id); got != fmt.Sprintf("710000000000000003 %d 1 0 1 0 0 1 %s", rustChat, newToken) {
		t.Errorf("stored %q", got)
	}

	// The same channel and chat again: refused.
	if st, b, _ := e.call("admin", "POST", d2tBase, create); st != 409 || b["error"] != "duplicate" {
		t.Errorf("duplicate %d %v", st, b)
	}
	// The same token again reuses its bot.
	create["chat_id"] = gnuWeeb
	b = e.want(201, "superadmin", "POST", d2tBase, create)
	id2 := uint64(b["id"].(float64))
	if got := e.one("SELECT telegram_bot_id FROM discord_telegram_routes WHERE id = ?", id2); got != bot {
		t.Errorf("bot %s, want %s", got, bot)
	}

	// Change what is given; a token replaces the bot, the old one stays.
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, id), map[string]any{
		"enabled": false, "types": []string{"sticker"}})
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, id), map[string]any{
		"bot_token": "7000000001:fixtureBotSecretOne"})
	if got := e.one("SELECT CONCAT_WS(' ', enabled, fwd_sticker, fwd_text, telegram_bot_id) "+
		"FROM discord_telegram_routes WHERE id = ?", id); got != "0 1 0 1" {
		t.Errorf("after patch %q", got)
	}
	_, page := e.page("admin", "/platform-fwd/discord-telegram")
	noSecrets(t, "page", page, "testSecretNewBot")

	// A route that has sent messages is refused; one that has not goes.
	if st, b, _ := e.call("admin", "DELETE", d2tBase+"/1", nil); st != 409 || b["error"] != "in_use" ||
		!strings.Contains(b["message"].(string), "2 message") {
		t.Errorf("delete used %d %v", st, b)
	}
	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", d2tBase, id), nil)
	e.want(404, "admin", "DELETE", fmt.Sprintf("%s/%d", d2tBase, id), nil)

	want := []string{
		fmt.Sprintf("3 fwd_d2t_create route %d channel 710000000000000003 chat %d enabled=true types=text,document", id, rustChat),
		fmt.Sprintf("3 owner_set d2t_route %d", id),
		"3 owner_set telegram_bot " + bot,
		// The same token again reuses the bot, which keeps its owner.
		fmt.Sprintf("1 fwd_d2t_create route %d channel 710000000000000003 chat %d enabled=true types=text,document", id2, gnuWeeb),
		fmt.Sprintf("1 owner_set d2t_route %d", id2),
		fmt.Sprintf("3 fwd_d2t_update route %d chat %d types=sticker enabled=false", id, rustChat),
		fmt.Sprintf("3 fwd_d2t_update route %d chat %d bot", id, rustChat),
		fmt.Sprintf("3 fwd_d2t_delete route %d channel 710000000000000003 chat %d", id, rustChat),
		fmt.Sprintf("3 resource_drop d2t_route %d", id),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	noSecrets(t, "audit", e.audits(), "testSecretNewBot")
}

func TestD2TRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	base := map[string]any{"discord_channel_id": "710000000000000009", "chat_id": gnuWeeb,
		"bot_token": newToken, "types": []string{"text"}, "enabled": true}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range base {
			m[a] = b
		}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, c := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{with("discord_channel_id", "abc"), 400, "bad_channel"},
		{with("discord_channel_id", "0"), 400, "bad_channel"},
		{with("discord_channel_id", "123456789012345678901"), 400, "bad_channel"},
		{with("bot_token", "not-a-token"), 400, "bad_token"},
		{with("bot_token", ":secret"), 400, "bad_token"},
		{with("bot_token", "12ab:secret"), 400, "bad_token"},
		{with("types", []string{"other"}), 400, "bad_request"},
		{with("bot_token", nil), 400, "bad_request"},
		{with("chat_id", int64(-1009999999999)), 404, "not_found"},
		{with("federation_id", 3), 404, "not_found"},
	} {
		st, b, _ := e.call("admin", "POST", d2tBase, c.body)
		if st != c.status || b["error"] != c.code {
			t.Errorf("%v: %d %v, want %d %s", c.body, st, b, c.status, c.code)
		}
		noSecrets(t, "refusal", b, "testSecretNewBot")
	}
	if st, b, _ := e.call("admin", "PATCH", d2tBase+"/2", map[string]any{"bot_token": "x"}); st != 400 ||
		b["error"] != "bad_token" {
		t.Errorf("patch bad token %d %v", st, b)
	}
	for _, op := range [][2]string{{"POST", d2tBase}, {"PATCH", d2tBase + "/2"}, {"DELETE", d2tBase + "/2"}} {
		if st, _, _ := e.call("member", op[0], op[1], base); st != 404 {
			t.Errorf("member %s %s: %d", op[0], op[1], st)
		}
		if st, _, _ := e.call("admin", op[0], op[1], base, false); st != 403 {
			t.Errorf("no csrf %s %s: %d", op[0], op[1], st)
		}
	}
	if n := e.one("SELECT COUNT(*) FROM discord_telegram_routes"); n != "2" {
		t.Errorf("%s routes after refusals", n)
	}
	if n := e.one("SELECT COUNT(*) FROM telegram_bots"); n != "2" {
		t.Errorf("%s bots after refusals", n)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("refusals audited: %v", a)
	}
}

// A new configuration's federation must exist and be one the viewer
// administers; the pages offer exactly those.
func TestForwardingFederations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, q := range []string{
		"INSERT INTO web_federations (id, name) VALUES (8, 'Other'), (9, 'Lab')",
		"INSERT INTO web_federation_admins (federation_id, web_user_id) VALUES (9, 3)",
	} {
		if _, err := e.app.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for role, want := range map[string]string{
		"admin":      `[{"id":0,"name":"Default"},{"id":9,"name":"Lab"}]`,
		"superadmin": `[{"id":0,"name":"Default"},{"id":9,"name":"Lab"},{"id":8,"name":"Other"}]`,
	} {
		for _, p := range []string{"/platform-fwd/telegram-discord", "/platform-fwd/discord-telegram"} {
			if _, b := e.page(role, p); js(b["props"].(map[string]any)["federations"]) != want {
				t.Errorf("%s %s: %s", role, p, js(b["props"].(map[string]any)["federations"]))
			}
		}
	}
	body := func(fed int) map[string]any {
		return map[string]any{"chat_id": rustChat, "chat_kind": "group", "webhook_url": newHook,
			"types": []string{"text"}, "enabled": true, "federation_id": fed}
	}
	e.want(201, "admin", "POST", t2dBase, body(9))
	e.want(404, "admin", "POST", t2dBase, body(8))
	e.want(201, "superadmin", "POST", t2dBase, body(8))
	e.want(404, "superadmin", "POST", t2dBase, body(7))
}

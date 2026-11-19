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
	newToken = "7000000099:testSecretNewBotAAAAAAAAAAAAAAAAAAAAAA"
	// The "bots" fixture's connected Discord bot.
	discordBot = uint64(1000000001)
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
		`"chat":{"id":-1001000000001,"title":"GNU/Weeb","kind":"group","sub":"supergroup",`+
		`"photo":"`+photoOf(gnuWeebPhoto)+`"},"url_masked":"https://discord.com/api/webhooks/1100000000000000001/****",`+
		`"types":["text","photo","video","animation","video_note","document","sticker","voice","audio",`+
		`"poll","checklist","dice","game","location","contact","story","paid_media","invoice","service","other"],`+
		`"enabled":true,`+
		`"sent":3,"last_sent":"2026-11-17T11:00:01Z"}`) {
		t.Errorf("webhook 1 %s", got)
	}
	if got := js(routes[1].(map[string]any)["chat"]); got !=
		canon(`{"id":9299000001,"title":"Dewi Kusuma","kind":"user","photo":"`+photoOf(dewiPhoto)+`"}`) {
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
		r1["sent"] != float64(2) ||
		js(r1["chat"]) != canon(`{"id":-1001000000001,"title":"GNU/Weeb","kind":"group","sub":"supergroup",`+
			`"photo":"`+photoOf(gnuWeebPhoto)+`"}`) {
		t.Errorf("route 1 %s", js(r1))
	}
	if got := js(r1["telegram_bot"]); got != canon(`{"id":1,"kind":"telegram","name":"@gw_bridge_bot",`+
		`"identity":{"user_id":"7000000001","username":"gw_bridge_bot"},"status":"connecting","enabled":true,`+
		`"photo":null,"profile":null}`) {
		t.Errorf("route 1 telegram bot %s", got)
	}
	if got := js(r1["discord_bot"]); got != canon(`{"id":1000000001,"kind":"discord","name":"Main reader",`+
		`"identity":{"user_id":"1000000000000000001","username":"gw_reader"},"status":"connected","enabled":true,`+
		`"photo":null,"profile":null}`) {
		t.Errorf("route 1 discord bot %s", got)
	}
	// Route 2: its Telegram bot is switched off, and it names no
	// Discord bot yet; the page can tell both.
	if r2["channel_name"] != "" || r2["discord_bot"] != nil ||
		js(r2["telegram_bot"]) != canon(`{"id":2,"kind":"telegram","name":"Bot 2","identity":null,`+
			`"status":"disabled","enabled":false,"photo":null,"profile":null}`) ||
		js(r2["types"]) != `["text","document","sticker","audio","embed","system"]` {
		t.Errorf("route 2 %s", js(r2))
	}
	if _, ok := r1["bot"]; ok {
		t.Error("the route still answers bot")
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
	member := scope.For(scope.Viewer{ID: 5, Role: scope.Member})
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
	admin := scope.For(scope.Viewer{ID: 3, Role: scope.Admin})
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
		"chat_kind": "user", "webhook_url": newHook, "types": []string{"poll"}, "enabled": false})
	priv := uint64(b["id"].(float64))

	want := []string{
		fmt.Sprintf("3 fwd_t2d_create webhook %d chat %d enabled=true types=text,photo", id, rustChat),
		fmt.Sprintf("3 owner_set t2d_webhook %d", id),
		fmt.Sprintf("3 fwd_t2d_update webhook %d chat %d enabled=false", id, rustChat),
		fmt.Sprintf("1 fwd_t2d_update webhook %d chat %d types=other", id, rustChat),
		fmt.Sprintf("3 fwd_t2d_update webhook %d chat %d url", id, rustChat),
		fmt.Sprintf("3 fwd_t2d_delete webhook %d chat %d", id, rustChat),
		fmt.Sprintf("3 resource_drop t2d_webhook %d", id),
		fmt.Sprintf("1 fwd_t2d_create webhook %d chat 9299000002 enabled=false types=poll", priv),
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
		{with("types", []string{}), 400, "bad_request"},
		{with("types", nil), 400, "bad_request"},
		{with("enabled", nil), 400, "bad_request"},
		{with("chat_kind", "user"), 400, "bad_request"},
		// A chat the logger never saw does not exist; a federation
		// that does not hold the chat is no choice.
		{with("chat_id", int64(-1009999999999)), 404, "not_found"},
		{with("federation_id", 7), 400, "bad_federation"},
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
		"discord_bot_id": discordBot, "telegram_bot_id": 1,
		"types": []string{"text", "document"}, "enabled": true}
	b := e.want(201, "admin", "POST", d2tBase, create)
	id := uint64(b["id"].(float64))
	if got := e.one("SELECT CONCAT_WS(' ', discord_channel_id, telegram_chat_id, enabled, "+
		"fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document, telegram_bot_id, discord_bot_id) "+
		"FROM discord_telegram_routes WHERE id = ?", id); got !=
		fmt.Sprintf("710000000000000003 %d 1 0 1 0 0 1 1 %d", rustChat, discordBot) {
		t.Errorf("stored %q", got)
	}
	// The same channel and chat again: refused.
	if st, b, _ := e.call("admin", "POST", d2tBase, create); st != 409 || b["error"] != "duplicate" {
		t.Errorf("duplicate %d %v", st, b)
	}

	// A private chat is a target too, as on the Drogon page.
	create["chat_id"] = 9299000001
	b = e.want(201, "superadmin", "POST", d2tBase, create)
	priv := uint64(b["id"].(float64))
	if got := e.one("SELECT telegram_chat_id FROM discord_telegram_routes WHERE id = ?", priv); got != "9299000001" {
		t.Errorf("private route chat %s", got)
	}

	// Change what is given: state, types, the bots, the channel and the
	// chat.
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, id), map[string]any{
		"enabled": false, "types": []string{"sticker"}})
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, id), map[string]any{
		"telegram_bot_id": 2, "discord_bot_id": 1000000002,
		"discord_channel_id": "710000000000000005", "chat_id": gnuWeeb})
	if got := e.one("SELECT CONCAT_WS(' ', enabled, fwd_sticker, fwd_text, telegram_bot_id, "+
		"discord_bot_id, discord_channel_id, telegram_chat_id) FROM discord_telegram_routes "+
		"WHERE id = ?", id); got != fmt.Sprintf("0 1 0 2 1000000002 710000000000000005 %d", gnuWeeb) {
		t.Errorf("after patch %q", got)
	}
	// Onto a channel and chat another route has: refused, unchanged.
	if st, b, _ := e.call("admin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, id), map[string]any{
		"discord_channel_id": "710000000000000001"}); st != 409 || b["error"] != "duplicate" {
		t.Errorf("patch onto route 1 %d %v", st, b)
	}
	_, page := e.page("admin", "/platform-fwd/discord-telegram")
	noSecrets(t, "page", page)

	// A route that has sent messages is refused; one that has not goes.
	if st, b, _ := e.call("admin", "DELETE", d2tBase+"/1", nil); st != 409 || b["error"] != "in_use" ||
		!strings.Contains(b["message"].(string), "2 message") {
		t.Errorf("delete used %d %v", st, b)
	}
	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", d2tBase, id), nil)
	e.want(404, "admin", "DELETE", fmt.Sprintf("%s/%d", d2tBase, id), nil)

	want := []string{
		fmt.Sprintf("3 fwd_d2t_create route %d channel 710000000000000003 chat %d discord_bot %d "+
			"telegram_bot 1 enabled=true types=text,document", id, rustChat, discordBot),
		fmt.Sprintf("3 owner_set d2t_route %d", id),
		fmt.Sprintf("1 fwd_d2t_create route %d channel 710000000000000003 chat 9299000001 discord_bot %d "+
			"telegram_bot 1 enabled=true types=text,document", priv, discordBot),
		fmt.Sprintf("1 owner_set d2t_route %d", priv),
		fmt.Sprintf("3 fwd_d2t_update route %d chat %d types=sticker enabled=false", id, rustChat),
		fmt.Sprintf("3 fwd_d2t_update route %d chat %d channel=710000000000000005 "+
			"discord_bot=1000000002 telegram_bot=2 chat=%d", id, gnuWeeb, gnuWeeb),
		fmt.Sprintf("3 fwd_d2t_delete route %d channel 710000000000000005 chat %d", id, gnuWeeb),
		fmt.Sprintf("3 resource_drop d2t_route %d", id),
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestD2TRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	base := map[string]any{"discord_channel_id": "710000000000000009", "chat_id": gnuWeeb,
		"discord_bot_id": discordBot, "telegram_bot_id": 1, "types": []string{"text"}, "enabled": true}
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
		{with("types", []string{"other"}), 400, "bad_request"},
		{with("types", []string{"video_note"}), 400, "bad_request"},
		{with("types", []string{}), 400, "bad_request"},
		{with("discord_bot_id", nil), 400, "bad_request"},
		{with("telegram_bot_id", nil), 400, "bad_request"},
		// A bot of the other kind in either place.
		{with("discord_bot_id", 1), 400, "bad_request"},
		{with("telegram_bot_id", discordBot), 400, "bad_request"},
		// Bots and chats that do not exist.
		{with("telegram_bot_id", 99), 404, "not_found"},
		{with("discord_bot_id", 1000000099), 404, "not_found"},
		{with("chat_id", int64(-1009999999999)), 404, "not_found"},
		{with("federation_id", 3), 400, "bad_federation"},
	} {
		st, b, _ := e.call("admin", "POST", d2tBase, c.body)
		if st != c.status || b["error"] != c.code {
			t.Errorf("%v: %d %v, want %d %s", c.body, st, b, c.status, c.code)
		}
	}
	// A token is not taken on a route any more.
	noBot := with("telegram_bot_id", nil)
	noBot["bot_token"] = newToken
	if st, b, _ := e.call("admin", "POST", d2tBase, noBot); st != 400 || b["error"] != "bad_request" {
		t.Errorf("route with a token: %d %v", st, b)
	}
	for _, c := range []struct {
		body   map[string]any
		status int
	}{
		{map[string]any{"discord_channel_id": "x"}, 400},
		{map[string]any{"chat_id": -1009999999999}, 404},
		{map[string]any{"telegram_bot_id": discordBot}, 400},
		{map[string]any{"discord_bot_id": 1000000099}, 404},
	} {
		if st, _, _ := e.call("admin", "PATCH", d2tBase+"/2", c.body); st != c.status {
			t.Errorf("patch %v: %d, want %d", c.body, st, c.status)
		}
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

// The chat of a webhook changes with its kind, as on creation.
func TestT2DChangeChat(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.want(204, "admin", "PATCH", t2dBase+"/3", map[string]any{"chat_id": 9299000002, "chat_kind": "user"})
	if got := e.one("SELECT CONCAT_WS(' ', telegram_chat_id, telegram_chat_type, enabled) " +
		"FROM telegram_discord_webhooks WHERE id = 3"); got != "9299000002 private 0" {
		t.Errorf("after patch %q", got)
	}
	for _, c := range []struct {
		body   map[string]any
		status int
	}{
		{map[string]any{"chat_id": gnuWeeb}, 400},
		{map[string]any{"chat_id": gnuWeeb, "chat_kind": "user"}, 400},
		{map[string]any{"chat_id": -1009999999999, "chat_kind": "group"}, 404},
	} {
		if st, _, _ := e.call("admin", "PATCH", t2dBase+"/3", c.body); st != c.status {
			t.Errorf("%v: %d, want %d", c.body, st, c.status)
		}
	}
	want := []string{"3 fwd_t2d_update webhook 3 chat 9299000002 chat=9299000002"}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit %v", got)
	}
}

// A new configuration's federation must exist and be one the viewer
// administers; the pages offer exactly those.
// A configuration goes into a federation holding its chat: the one
// choice there is, or the one the form asked for among several.
func TestForwardingFederations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	// Rust is in Other and Lab, the admin administers Lab (and
	// Default); Kernel is in Other alone, GNU/Weeb in Default.
	for _, q := range []string{
		"INSERT INTO web_federations (id, name) VALUES (8, 'Other'), (9, 'Lab')",
		"INSERT INTO web_federation_admins (federation_id, web_user_id) VALUES (9, 3)",
		fmt.Sprintf("INSERT INTO web_federation_groups (federation_id, group_id) VALUES "+
			"(8, %d), (9, %d), (8, %d)", rustChat, rustChat, kernelChat),
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
	choices := func(role string, chat int64) string {
		t.Helper()
		return js(e.want(200, role, "GET", fmt.Sprintf("/api/forwarding/federations?chat_id=%d", chat),
			nil)["federations"])
	}
	for _, c := range []struct {
		role string
		chat int64
		want string
	}{
		{"admin", rustChat, `[{"id":9,"name":"Lab"}]`},
		{"superadmin", rustChat, `[{"id":8,"name":"Other"},{"id":9,"name":"Lab"}]`},
		// None of the admin's own: every federation holding it.
		{"admin", kernelChat, `[{"id":8,"name":"Other"}]`},
		{"admin", gnuWeeb, `[{"id":0,"name":"Default"}]`},
		{"superadmin", 9299000002, `[{"id":0,"name":"Default"}]`},
	} {
		if got := choices(c.role, c.chat); got != c.want {
			t.Errorf("%s %d: %s, want %s", c.role, c.chat, got, c.want)
		}
	}
	if st, _, _ := e.call("admin", "GET", "/api/forwarding/federations?chat_id=-1009999999999", nil); st != 404 {
		t.Errorf("unseen chat %d", st)
	}
	if st, b, _ := e.call("admin", "GET", "/api/forwarding/federations", nil); st != 400 ||
		b["error"] != "bad_request" {
		t.Errorf("no chat %d %v", st, b)
	}
	if st, _, _ := e.call("member", "GET", fmt.Sprintf("/api/forwarding/federations?chat_id=%d",
		rustChat), nil); st != 404 {
		t.Errorf("member without forward %d", st)
	}

	body := func(chat int64, fed any) map[string]any {
		m := map[string]any{"chat_id": chat, "chat_kind": "group", "webhook_url": newHook,
			"types": []string{"text"}, "enabled": true}
		if fed != nil {
			m["federation_id"] = fed
		}
		return m
	}
	fedOf := func(id any) string {
		t.Helper()
		f := e.appOne("SELECT federation_id FROM web_resource_federations WHERE "+
			"resource_type = 't2d_webhook' AND resource_id = ?", uint64(id.(float64)))
		if f == "" {
			return "Default"
		}
		return f
	}
	refuse := func(role string, b map[string]any, code string) {
		t.Helper()
		if st, a, _ := e.call(role, "POST", t2dBase, b); st != 400 || a["error"] != code {
			t.Errorf("%s %v: %d %v, want %s", role, b, st, a, code)
		}
	}
	// One choice: filed there with or without asking.
	lab := e.want(201, "admin", "POST", t2dBase, body(rustChat, nil))["id"]
	if f := fedOf(lab); f != "9" {
		t.Errorf("admin's Rust webhook in %s", f)
	}
	if f := fedOf(e.want(201, "admin", "POST", t2dBase, body(rustChat, 9))["id"]); f != "9" {
		t.Errorf("admin's Rust webhook, asked, in %s", f)
	}
	refuse("admin", body(rustChat, 8), "bad_federation")
	refuse("admin", body(rustChat, 0), "bad_federation")
	if f := fedOf(e.want(201, "admin", "POST", t2dBase, body(kernelChat, nil))["id"]); f != "8" {
		t.Errorf("admin's Kernel webhook in %s", f)
	}
	// Several choices: asked for.
	refuse("superadmin", body(rustChat, nil), "federation_required")
	refuse("superadmin", body(rustChat, 7), "bad_federation")
	other := e.want(201, "superadmin", "POST", t2dBase, body(rustChat, 8))["id"]
	if f := fedOf(other); f != "8" {
		t.Errorf("superadmin's Rust webhook in %s", f)
	}
	if f := fedOf(e.want(201, "superadmin", "POST", t2dBase, body(gnuWeeb, nil))["id"]); f != "Default" {
		t.Errorf("GNU/Weeb webhook in %s", f)
	}
	// Discord to Telegram asks the same.
	route := map[string]any{"discord_channel_id": "1200000000000000077", "chat_id": rustChat,
		"discord_bot_id": discordBot, "telegram_bot_id": 1, "types": []string{"text"}, "enabled": true}
	if st, b, _ := e.call("superadmin", "POST", d2tBase, route); st != 400 || b["error"] != "federation_required" {
		t.Errorf("route without a federation: %d %v", st, b)
	}
	route["federation_id"] = 9
	rid := e.want(201, "superadmin", "POST", d2tBase, route)["id"]
	if f := e.appOne("SELECT federation_id FROM web_resource_federations WHERE "+
		"resource_type = 'd2t_route' AND resource_id = ?", uint64(rid.(float64))); f != "9" {
		t.Errorf("route in %q", f)
	}

	// The lists answer the federation; the fixture's are in Default.
	_, p := e.page("superadmin", "/platform-fwd/telegram-discord")
	feds := map[float64]string{}
	for _, r := range p["props"].(map[string]any)["routes"].([]any) {
		m := r.(map[string]any)
		feds[m["id"].(float64)] = js(m["federation"])
	}
	if feds[lab.(float64)] != `{"id":9,"name":"Lab"}` || feds[other.(float64)] != `{"id":8,"name":"Other"}` ||
		feds[1] != `{"id":0,"name":"Default"}` {
		t.Errorf("federations answered %v", feds)
	}
	_, p = e.page("superadmin", "/platform-fwd/discord-telegram")
	for _, r := range p["props"].(map[string]any)["routes"].([]any) {
		m := r.(map[string]any)
		if m["id"] == rid && js(m["federation"]) != `{"id":9,"name":"Lab"}` {
			t.Errorf("route answered %s", js(m["federation"]))
		}
	}

	// Deleting a configuration drops its federation.
	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", t2dBase, uint64(lab.(float64))), nil)
	if f := fedOf(lab); f != "Default" {
		t.Errorf("deleted webhook still in %s", f)
	}
	if got := e.audits(); !strings.Contains(strings.Join(got, "\n"),
		fmt.Sprintf("fwd_t2d_create webhook %d chat %d enabled=true types=text federation=8",
			uint64(other.(float64)), rustChat)) {
		t.Errorf("audit\n%s", strings.Join(got, "\n"))
	}
}

// A member files a configuration into the federation in which its
// group is granted to it.
func TestMemberForwardFederation(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, q := range []string{
		"INSERT INTO web_federations (id, name) VALUES (8, 'Other'), (9, 'Lab')",
		fmt.Sprintf("INSERT INTO web_federation_groups (federation_id, group_id) VALUES "+
			"(8, %d), (9, %d)", rustChat, rustChat),
		"INSERT INTO web_federation_members (federation_id, web_user_id) VALUES (9, 5)",
		fmt.Sprintf("INSERT INTO web_member_grant_groups (web_user_id, federation_id, group_id) "+
			"VALUES (5, 9, %d)", rustChat),
		"INSERT INTO web_member_abilities (web_user_id, ability) VALUES (5, 'forward')",
	} {
		if _, err := e.app.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if got := js(e.want(200, "member", "GET", fmt.Sprintf("/api/forwarding/federations?chat_id=%d",
		rustChat), nil)["federations"]); got != `[{"id":9,"name":"Lab"}]` {
		t.Errorf("member's choices %s", got)
	}
	id := e.want(201, "member", "POST", t2dBase, map[string]any{"chat_id": rustChat, "chat_kind": "group",
		"webhook_url": newHook, "types": []string{"text"}, "enabled": true})["id"]
	if f := e.appOne("SELECT federation_id FROM web_resource_federations WHERE "+
		"resource_type = 't2d_webhook' AND resource_id = ?", uint64(id.(float64))); f != "9" {
		t.Errorf("member's webhook in %q", f)
	}
}

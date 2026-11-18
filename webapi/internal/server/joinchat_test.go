// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"
)

// Join chat against the fake daemon (adminEnv).

const previewChat = `{"outcome": "ok", "message": "", "ref": "@gnuweeb", "kind": "username",
	"chat": {"chat_id": -1001000000001, "title": "GNU/Weeb", "type": "supergroup",
	"member_count": 1234, "is_member": false, "banned": false, "needs_approval": false,
	"paid": false, "username": "gnuweeb"}}`

func TestJoinChatPreview(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer("/v1/chats/resolve", 200, "", previewChat)

	b := e.want(200, "superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "  @gnuweeb \n"})
	if b["outcome"] != "ok" || b["message"] != "" || b["ref"] != "@gnuweeb" ||
		js(b["chat"]) != canon(`{"chat_id": -1001000000001, "title": "GNU/Weeb", "type": "supergroup",
			"member_count": 1234, "is_member": false, "banned": false, "needs_approval": false,
			"paid": false, "username": "gnuweeb"}`) {
		t.Errorf("preview %v", b)
	}
	if c := e.daemon.calls(); len(c) != 1 || c[0] != `/v1/chats/resolve {"ref":"@gnuweeb"}` {
		t.Errorf("daemon asked %q", c)
	}

	// A preview that found nothing is still 200, with the reason; a
	// private group behind a link has no chat id yet (0).
	e.daemon.answer("/v1/chats/resolve", 200, "", `{"outcome": "not_found", "message": "",
		"ref": "@nobody_here", "kind": "username", "chat": null}`)
	b = e.want(200, "superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "@nobody_here"})
	if b["outcome"] != "not_found" || b["chat"] != nil || b["message"] != "No public chat has that username." {
		t.Errorf("not found %v", b)
	}
	e.daemon.answer("/v1/chats/resolve", 200, "", `{"outcome": "ok", "ref": "https://t.me/+AbCdEfGhIjKl",
		"kind": "invite", "chat": {"chat_id": null, "title": "Hidden", "type": "supergroup",
		"member_count": 3, "is_member": false, "banned": false, "needs_approval": true,
		"paid": false, "username": null}}`)
	b = e.want(200, "superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "t.me/+AbCdEfGhIjKl"})
	if c := b["chat"].(map[string]any); c["chat_id"] != float64(0) || c["username"] != "" ||
		c["needs_approval"] != true {
		t.Errorf("invite %v", b)
	}

	// The daemon refuses the input: 400 with its reason. It is away:
	// 502 daemon.
	e.daemon.answer("/v1/chats/resolve", 400, "", `{"error": "bad_request", "message": "not an invite link or a username"}`)
	if st, b, _ := e.call("superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "tg://x"}); st != 400 ||
		b["error"] != "bad_request" || !strings.Contains(b["message"].(string), "not an invite link") {
		t.Errorf("refused %d %v", st, b)
	}
	e.daemon.answer("/v1/chats/resolve", 503, "", `{"error": "unavailable", "message": "TDLib is not authorized"}`)
	if st, b, _ := e.call("superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "@gnuweeb"}); st != 502 ||
		b["error"] != "daemon" || !strings.Contains(b["message"].(string), "TDLib is not authorized") {
		t.Errorf("not ready %d %v", st, b)
	}
	// Previews change nothing and are not audited.
	if a := e.audits(); len(a) != 0 {
		t.Errorf("previews audited: %v", a)
	}
}

func TestJoinChatJoin(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer("/v1/chats/join", 200, "", strings.Replace(previewChat, `"ok"`, `"joined"`, 1))
	b := e.want(200, "superadmin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{0}})
	if b["outcome"] != "joined" || b["message"] != "Joined. The chat is now logged, media included." ||
		b["retry_after"] != nil {
		t.Errorf("join %v", b)
	}

	e.daemon.answer("/v1/chats/join", 200, "", `{"outcome": "flood_wait", "message": "", "ref": "@gnuweeb",
		"retry_after": 300, "chat": null}`)
	b = e.want(200, "superadmin", "POST", "/api/join-chat/join", map[string]any{"ref": "@gnuweeb"})
	if b["outcome"] != "flood_wait" || b["retry_after"] != float64(300) ||
		b["message"] != "Telegram is rate-limiting the account. Try again in 5 minutes." {
		t.Errorf("flood wait %v", b)
	}
	e.daemon.answer("/v1/chats/join", 200, "", `{"outcome": "error", "message": "CHANNELS_TOO_MUCH", "ref": "@x1234", "chat": null}`)
	b = e.want(200, "superadmin", "POST", "/api/join-chat/join", map[string]any{"ref": "@x1234"})
	if b["message"] != "Telegram refused: CHANNELS_TOO_MUCH" {
		t.Errorf("error %v", b)
	}
	// The daemon gives no outcome: audited all the same, 502.
	e.daemon.answer("/v1/chats/join", 504, "", `{"error": "timeout"}`)
	st, b, _ := e.call("superadmin", "POST", "/api/join-chat/join", map[string]any{"ref": "@slowgroup"})
	if st != 502 || b["error"] != "daemon" || !strings.Contains(b["message"].(string), "may still have happened") {
		t.Errorf("timeout %d %v", st, b)
	}

	want := []string{
		`1 chat_join joined @gnuweeb "GNU/Weeb"`,
		`1 chat_join flood_wait @gnuweeb retry_after=300`,
		`1 chat_join error @x1234`,
		`1 chat_join http_504 @slowgroup`,
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The page lists the attempts, these and the Drogon page's.
	st, p := e.page("superadmin", "/join-chat")
	if st != 200 || p["route"] != "join_chat" {
		t.Fatalf("page %d %v", st, p)
	}
	props := p["props"].(map[string]any)
	recent := props["recent"].([]any)
	if js(props["federations"]) != `[{"id":0,"name":"Default"}]` || len(recent) != 6 ||
		js(recent[0]) != canon(`{"at": "`+recent[0].(map[string]any)["at"].(string)+
			`", "by": "admin", "ref": "@slowgroup", "title": "", "outcome": "http_504"}`) ||
		js(recent[3]) != canon(`{"at": "`+recent[3].(map[string]any)["at"].(string)+
			`", "by": "admin", "ref": "@gnuweeb", "title": "GNU/Weeb", "outcome": "joined"}`) ||
		js(recent[4]) != canon(`{"at": "2026-11-11T09:00:00Z", "by": "admin",
			"ref": "https://t.me/+AbCdEfGhIjKl", "title": "Some Group", "outcome": "flood_wait"}`) {
		t.Errorf("recent %s", js(recent))
	}
}

func TestJoinChatRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer("/v1/chats/join", 200, "", previewChat)
	e.daemon.answer("/v1/chats/resolve", 200, "", previewChat)
	for _, c := range []struct {
		role   string
		body   map[string]any
		status int
		code   string
	}{
		{"superadmin", map[string]any{"ref": "   "}, 400, "bad_request"},
		{"superadmin", map[string]any{}, 400, "bad_request"},
		{"superadmin", map[string]any{"ref": "@" + strings.Repeat("a", 256)}, 400, "bad_request"},
		// A superadmin administers every federation, but one that
		// does not exist is not found.
		{"superadmin", map[string]any{"ref": "@gnuweeb", "federation_ids": []int{0, 5}}, 404, "not_found"},
		// Admins hold join_chat only when granted it; members never.
		{"admin", map[string]any{"ref": "@gnuweeb"}, 404, "not_found"},
		{"member", map[string]any{"ref": "@gnuweeb"}, 404, "not_found"},
	} {
		st, b, _ := e.call(c.role, "POST", "/api/join-chat/join", c.body)
		if st != c.status || b["error"] != c.code {
			t.Errorf("%s %v: %d %v, want %d %s", c.role, c.body, st, b, c.status, c.code)
		}
	}
	for _, role := range []string{"admin", "member"} {
		if st, _, _ := e.call(role, "POST", "/api/join-chat/preview", map[string]any{"ref": "@gnuweeb"}); st != 404 {
			t.Errorf("%s preview: %d", role, st)
		}
		if st, b := e.page(role, "/join-chat"); st != 404 || b["route"] != "not_found" {
			t.Errorf("%s page: %d %v", role, st, b)
		}
	}
	if st, b, _ := e.call("superadmin", "POST", "/api/join-chat/join", map[string]any{"ref": "@gnuweeb"}, false); st != 403 ||
		b["error"] != "csrf" {
		t.Errorf("no csrf %d %v", st, b)
	}
	if st, _, _ := e.call("superadmin", "POST", "/api/join-chat/preview", map[string]any{"ref": "@gnuweeb"}, false); st != 403 {
		t.Errorf("preview without csrf %d", st)
	}
	if c := e.daemon.calls(); len(c) != 0 {
		t.Errorf("the daemon was asked: %q", c)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("refusals audited: %v", a)
	}
}

// Without the daemon's API every join fails with 502 and is audited
// as api_off.
func TestJoinChatDaemonOff(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.with(func(d *Deps) { d.Daemon = nil })
	st, b, _ := e.call("superadmin", "POST", "/api/join-chat/join", map[string]any{"ref": "@gnuweeb"})
	if st != 502 || b["error"] != "daemon" || !strings.Contains(b["message"].(string), "WEB_TGLOGGERD_API") {
		t.Errorf("off %d %v", st, b)
	}
	if a := e.audits(); len(a) != 1 || a[0] != "1 chat_join api_off @gnuweeb" {
		t.Errorf("audit %v", a)
	}
}

// The federations offered and accepted are the existing ones the
// viewer administers.
func TestJoinChatFederations(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	if _, err := e.app.Exec("INSERT INTO web_federations (id, name) VALUES (9, 'Lab')"); err != nil {
		t.Fatal(err)
	}
	e.daemon.answer("/v1/chats/join", 200, "", strings.Replace(previewChat, `"ok"`, `"joined"`, 1))
	if _, p := e.page("superadmin", "/join-chat"); js(p["props"].(map[string]any)["federations"]) !=
		`[{"id":0,"name":"Default"},{"id":9,"name":"Lab"}]` {
		t.Errorf("federations %s", js(p["props"]))
	}
	e.want(200, "superadmin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{9}})
	e.want(404, "superadmin", "POST", "/api/join-chat/join",
		map[string]any{"ref": "@gnuweeb", "federation_ids": []int{10}})
}

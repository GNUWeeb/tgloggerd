// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"fmt"
	"testing"

	"tgloggerd/webapi/internal/webdb"
)

// ownerOf is the owner of a resource in the env's web database, 0 for
// none.
func (e *adminEnv) ownerOf(t webdb.ResourceType, id int64) uint64 {
	e.t.Helper()
	o, ok, err := webdb.OwnerOf(context.Background(), e.app, webdb.Resource{Type: t, ID: id})
	if err != nil {
		e.t.Fatal(err)
	}
	if !ok {
		return 0
	}
	return o.ID
}

// Every create operation makes the account creating the resource its
// owner: webhooks, routes, the bot a new token adds, exposed groups and
// MCP tokens. A token naming a stored bot reuses it without touching
// its owner, and deleting a resource drops its owner.
func TestOwnerOnCreation(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	admin := e.uids["admin"]

	b := e.want(201, "admin", "POST", t2dBase, map[string]any{"chat_id": rustChat,
		"chat_kind": "group", "webhook_url": newHook, "types": []string{"text"}, "enabled": true})
	hook := int64(b["id"].(float64))
	if o := e.ownerOf(webdb.ResT2DWebhook, hook); o != admin {
		t.Errorf("webhook owner %d", o)
	}

	b = e.want(201, "admin", "POST", d2tBase, map[string]any{"discord_channel_id": "710000000000000003",
		"chat_id": rustChat, "bot_token": newToken, "types": []string{"text"}, "enabled": true})
	route := int64(b["id"].(float64))
	var bot int64
	fmt.Sscan(e.one("SELECT telegram_bot_id FROM discord_telegram_routes WHERE id = ?", route), &bot)
	if o := e.ownerOf(webdb.ResD2TRoute, route); o != admin {
		t.Errorf("route owner %d", o)
	}
	if o := e.ownerOf(webdb.ResTelegramBot, bot); o != admin {
		t.Errorf("new bot owner %d", o)
	}

	// The superadmin routes through the admin's bot: the bot stays the
	// admin's. A token of the fixture's (unowned) bot leaves it unowned.
	b = e.want(201, "superadmin", "POST", d2tBase, map[string]any{"discord_channel_id": "710000000000000004",
		"chat_id": rustChat, "bot_token": newToken, "types": []string{"text"}, "enabled": true})
	route2 := int64(b["id"].(float64))
	if o := e.ownerOf(webdb.ResD2TRoute, route2); o != e.uids["superadmin"] {
		t.Errorf("second route owner %d", o)
	}
	if o := e.ownerOf(webdb.ResTelegramBot, bot); o != admin {
		t.Errorf("reused bot owner %d", o)
	}
	e.want(204, "superadmin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, route2),
		map[string]any{"bot_token": "7000000001:fixtureBotSecretOne"})
	if o := e.ownerOf(webdb.ResTelegramBot, 1); o != 0 {
		t.Errorf("fixture bot owner %d", o)
	}
	// A new token on a route adds a bot owned by who gave it.
	e.want(204, "superadmin", "PATCH", fmt.Sprintf("%s/%d", d2tBase, route2),
		map[string]any{"bot_token": "7000000098:testSecretPatchedBot"})
	var bot2 int64
	fmt.Sscan(e.one("SELECT telegram_bot_id FROM discord_telegram_routes WHERE id = ?", route2), &bot2)
	if o := e.ownerOf(webdb.ResTelegramBot, bot2); bot2 == bot || o != e.uids["superadmin"] {
		t.Errorf("patched bot %d owner %d", bot2, o)
	}

	const kernel = -1001000000002
	e.want(201, "admin", "POST", "/api/mcp/groups", map[string]any{"group_id": kernel})
	if o := e.ownerOf(webdb.ResMCPGroup, kernel); o != admin {
		t.Errorf("exposed group owner %d", o)
	}
	b = e.want(201, "admin", "POST", "/api/mcp/tokens", map[string]any{"name": "mine"})
	token := int64(b["id"].(float64))
	if o := e.ownerOf(webdb.ResMCPToken, token); o != admin {
		t.Errorf("token owner %d", o)
	}

	// Deleting drops the owner; revoking a token keeps it (the row
	// stays).
	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", t2dBase, hook), nil)
	e.want(204, "admin", "DELETE", fmt.Sprintf("%s/%d", d2tBase, route), nil)
	e.want(204, "admin", "DELETE", fmt.Sprintf("/api/mcp/groups/%d", kernel), nil)
	e.want(204, "admin", "POST", fmt.Sprintf("/api/mcp/tokens/%d/revoke", token), nil)
	for _, c := range []struct {
		t    webdb.ResourceType
		id   int64
		want uint64
	}{
		{webdb.ResT2DWebhook, hook, 0}, {webdb.ResD2TRoute, route, 0},
		{webdb.ResMCPGroup, kernel, 0}, {webdb.ResMCPToken, token, admin},
		{webdb.ResTelegramBot, bot, admin},
	} {
		if o := e.ownerOf(c.t, c.id); o != c.want {
			t.Errorf("%s %d after delete: owner %d, want %d", c.t, c.id, o, c.want)
		}
	}
}

// The forwarding lists and the MCP tokens list answer the recorded
// owners: name (display name, else username), role and whether the
// account is active, so a disabled owner shows. Without an owner a
// configuration is shared and a token names the account it was made
// by.
func TestOwnersInAnswers(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	ctx := context.Background()
	if _, err := e.app.Exec("UPDATE web_users SET display_name = 'GW Admin' WHERE id = 3"); err != nil {
		t.Fatal(err)
	}
	by := webdb.By(1, "")
	for _, c := range []struct {
		t     webdb.ResourceType
		id    int64
		owner uint64
	}{
		{webdb.ResT2DWebhook, 1, 3}, {webdb.ResD2TRoute, 1, 7}, {webdb.ResMCPToken, 2, 1},
	} {
		if err := webdb.SetOwner(ctx, e.app, webdb.Resource{Type: c.t, ID: c.id}, c.owner, by); err != nil {
			t.Fatal(err)
		}
	}

	_, b := e.page("admin", "/platform-fwd/telegram-discord")
	routes := b["props"].(map[string]any)["routes"].([]any)
	owners := map[float64]string{}
	for _, r := range routes {
		m := r.(map[string]any)
		owners[m["id"].(float64)] = js(m["owner"])
	}
	if owners[1] != canon(`{"id":3,"name":"GW Admin","role":"admin","active":true}`) ||
		owners[2] != canon(`{"id":0,"name":"Shared","active":true}`) {
		t.Errorf("webhook owners %v", owners)
	}

	_, b = e.page("superadmin", "/platform-fwd/discord-telegram")
	routes = b["props"].(map[string]any)["routes"].([]any)
	owners = map[float64]string{}
	for _, r := range routes {
		m := r.(map[string]any)
		owners[m["id"].(float64)] = js(m["owner"])
	}
	if owners[1] != canon(`{"id":7,"name":"intern2025","role":"member","active":false}`) ||
		owners[2] != canon(`{"id":0,"name":"Shared","active":true}`) {
		t.Errorf("route owners %v", owners)
	}

	_, b = e.page("admin", "/mcp-admin/tokens")
	tokens := b["props"].(map[string]any)["tokens"].([]any)
	got := map[float64]any{}
	for _, tk := range tokens {
		m := tk.(map[string]any)
		got[m["id"].(float64)] = m["owner"]
	}
	// Token 2 was made by ops and is owned by admin; token 1 has no
	// owner row and names the account it was made by.
	if got[2] != "admin" || got[1] != "admin" {
		t.Errorf("token owners %v", got)
	}
	if err := webdb.SetOwner(ctx, e.app, webdb.Resource{Type: webdb.ResMCPToken, ID: 1}, 2, by); err != nil {
		t.Fatal(err)
	}
	_, b = e.page("admin", "/mcp-admin/tokens")
	for _, tk := range b["props"].(map[string]any)["tokens"].([]any) {
		if m := tk.(map[string]any); m["id"] == float64(1) && m["owner"] != "ops" {
			t.Errorf("token 1 owner %v", m["owner"])
		}
	}
}

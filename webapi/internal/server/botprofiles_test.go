// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"sync/atomic"
	"testing"
)

// A bot's profile, the link to its user page, on the "fwd_photos"
// fixture: Telegram bot 1 (@gw_bridge_bot) has its user row, bot 2 has
// not logged in, and a Telegram user has the Discord bot gw_reader's
// user id, which must never link it.

// profilesOf is each bot's profile by id, from a list of bots, with
// whether the field is there.
func profilesOf(t *testing.T, bots any) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, b := range bots.([]any) {
		m := b.(map[string]any)
		p, ok := m["profile"]
		if !ok {
			t.Errorf("bot %v has no profile field", m["id"])
		}
		out[fmt.Sprint(int64(m["id"].(float64)))] = p
	}
	return out
}

func sameProfiles(t *testing.T, where string, got, want map[string]any) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: bots %v, want %v", where, got, want)
	}
	for id, w := range want {
		if g, ok := got[id]; !ok || g != w {
			t.Errorf("%s: bot %s profile %v, want %v", where, id, g, w)
		}
	}
}

func TestBotProfiles(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	// A Telegram bot logged in whose user the archive lacks (yet).
	addTelegramBot(t, e.ro, 100, 7000000100, "unresolved_bot")
	want := map[string]any{"1": float64(7000000001), "2": nil, "100": nil,
		"1000000001": nil, "1000000002": nil}

	for _, role := range []string{"superadmin", "admin"} {
		_, p := e.page(role, "/platform-fwd/bots")
		sameProfiles(t, role+" bots page", profilesOf(t, p["props"].(map[string]any)["bots"]), want)
		_, l, _ := e.call(role, "GET", "/api/bots", nil)
		sameProfiles(t, role+" GET /api/bots", profilesOf(t, l["bots"]), want)

		d2t := routesOf(t, e, role, "/platform-fwd/discord-telegram")
		if got := d2t["1"]["telegram_bot"].(map[string]any)["profile"]; got != float64(7000000001) {
			t.Errorf("%s: route 1 telegram bot profile %v", role, got)
		}
		if b := d2t["1"]["discord_bot"].(map[string]any); b["profile"] != nil {
			t.Errorf("%s: route 1 discord bot profile %v", role, b["profile"])
		}
		if p, ok := d2t["2"]["telegram_bot"].(map[string]any)["profile"]; !ok || p != nil {
			t.Errorf("%s: route 2 telegram bot profile %v (present %t)", role, p, ok)
		}
	}

	// Once the user is in the archive, the bot links to it.
	if _, err := e.ro.Exec("INSERT INTO telegram_users (id, first_name, last_name, type, msg_count, " +
		"created_at, updated_at) VALUES (7000000100, 'Unresolved', '', 'bot', 0, NOW(), NOW())"); err != nil {
		t.Fatal(err)
	}
	_, l, _ := e.call("admin", "GET", "/api/bots", nil)
	if got := profilesOf(t, l["bots"])["100"]; got != float64(7000000100) {
		t.Errorf("a resolved bot's profile %v", got)
	}
}

// A member sees the profile of its own bots only where it may open the
// user's page: 9299000001 administers Linux Kernel Indonesia, one of
// its groups; 9299000003 is in GNU/Weeb only.
func TestBotProfilesForMember(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	grantMember(t, e, memberGroups, "forward")
	mk := func(kind, name, token string) uint64 {
		t.Helper()
		b := e.want(201, "member", "POST", "/api/bots", map[string]any{"kind": kind, "name": name, "token": token})
		return uint64(b["id"].(float64))
	}
	seen := mk("telegram", "Seen", "9299000001:memberProfileFixtureSecretNo001")
	hidden := mk("telegram", "Hidden", "9299000003:memberProfileFixtureSecretNo002")
	reader := mk("discord", "Reader", "MTAwMDAwMDAwMDAwMDAwMDMwMA.GfixD.memberProfileDiscordBot")
	// Their identities, as discordd records them at login.
	for id, uid := range map[uint64]int64{seen: 9299000001, hidden: 9299000003} {
		if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = ?, username = ? WHERE id = ?",
			uid, fmt.Sprintf("bot_%d", uid), id); err != nil {
			t.Fatal(err)
		}
	}
	// The Discord bot's id is a Telegram user the member may see.
	if _, err := e.ro.Exec("UPDATE discord_bots SET bot_user_id = 9299000001, username = 'reader' WHERE id = ?",
		reader); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.page("member", "/users/9299000003"); st != 404 {
		t.Fatalf("the member opens 9299000003's page: %d", st)
	}
	if st, _ := e.page("member", "/users/9299000001"); st != 200 {
		t.Fatalf("the member cannot open 9299000001's page: %d", st)
	}

	want := map[string]any{fmt.Sprint(seen): float64(9299000001), fmt.Sprint(hidden): nil,
		fmt.Sprint(reader): nil}
	_, p := e.page("member", "/platform-fwd/bots")
	sameProfiles(t, "member bots page", profilesOf(t, p["props"].(map[string]any)["bots"]), want)
	_, l, _ := e.call("member", "GET", "/api/bots", nil)
	sameProfiles(t, "member GET /api/bots", profilesOf(t, l["bots"]), want)

	// The same on the member's routes.
	for i, tb := range []uint64{seen, hidden} {
		e.want(201, "member", "POST", d2tBase, map[string]any{
			"discord_channel_id": fmt.Sprint(710000000000000300 + i), "chat_id": kernelChat,
			"discord_bot_id": reader, "telegram_bot_id": tb, "types": []string{"text"}, "enabled": true})
	}
	got := map[string]any{}
	for _, r := range routesOf(t, e, "member", "/platform-fwd/discord-telegram") {
		tb := r["telegram_bot"].(map[string]any)
		got[fmt.Sprint(int64(tb["id"].(float64)))] = tb["profile"]
		if db := r["discord_bot"].(map[string]any); db["profile"] != nil {
			t.Errorf("member's route discord bot profile %v", db["profile"])
		}
	}
	delete(want, fmt.Sprint(reader))
	sameProfiles(t, "member routes", got, want)

	// An admin sees both users, so both bots link.
	_, l, _ = e.call("admin", "GET", "/api/bots", nil)
	if a := profilesOf(t, l["bots"]); a[fmt.Sprint(hidden)] != float64(9299000003) ||
		a[fmt.Sprint(seen)] != float64(9299000001) || a[fmt.Sprint(reader)] != nil {
		t.Errorf("admin's profiles %v", a)
	}
}

// The profiles are asked for in one query per page: as many
// statements on the member's visible users (telegram_group_admins,
// which only that query reads here) with many bots as with one, and
// as many naming telegram_users for an admin.
func TestBotProfilesAreBatched(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	grantMember(t, e, memberGroups, "forward")
	cost := func(n *atomic.Int64, role, path string) int64 {
		t.Helper()
		before := n.Load()
		if st, b := e.page(role, path); st != 200 {
			t.Fatalf("%s %s: %d %v", role, path, st, b)
		}
		return n.Load() - before
	}
	var bots []uint64
	mk := func(i int) {
		t.Helper()
		b := e.want(201, "member", "POST", "/api/bots", map[string]any{"kind": "telegram",
			"name": fmt.Sprint("Bot ", i), "token": fmt.Sprintf("%d:batchProfileFixtureSecret%05d", 9299000001+i, i)})
		id := uint64(b["id"].(float64))
		if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = ?, username = ? WHERE id = ?",
			9299000001+i, fmt.Sprint("bot_", i), id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.ro.Exec("INSERT INTO discord_telegram_routes (id, discord_channel_id, "+
			"telegram_chat_id, telegram_bot_id, enabled) VALUES (?, ?, ?, ?, 1)",
			100+i, 710000000000000400+i, kernelChat, id); err != nil {
			t.Fatal(err)
		}
		bots = append(bots, id)
	}
	mk(0)
	users := countTable(t, e, "telegram_users")
	admin, d2t := cost(users, "admin", "/platform-fwd/bots"), cost(users, "admin", "/platform-fwd/discord-telegram")
	admins := countTable(t, e, "telegram_group_admins")
	member := cost(admins, "member", "/platform-fwd/bots")
	if member == 0 || admin == 0 || d2t == 0 {
		t.Fatalf("the profiles are not asked for: %d %d %d", member, admin, d2t)
	}
	for i := 1; i < 6; i++ {
		mk(i)
	}
	if now := cost(admins, "member", "/platform-fwd/bots"); now != member {
		t.Errorf("member's bots page: %d statements on its users, %d with one bot", now, member)
	}
	users = countTable(t, e, "telegram_users")
	if now := cost(users, "admin", "/platform-fwd/bots"); now != admin {
		t.Errorf("admin's bots page: %d statements on telegram_users, %d with fewer bots", now, admin)
	}
	if now := cost(users, "admin", "/platform-fwd/discord-telegram"); now != d2t {
		t.Errorf("routes page: %d statements on telegram_users, %d with fewer routes", now, d2t)
	}
	// And the profiles are all there.
	_, l, _ := e.call("admin", "GET", "/api/bots", nil)
	got := profilesOf(t, l["bots"])
	for i, id := range bots {
		if got[fmt.Sprint(id)] != float64(9299000001+i) {
			t.Errorf("bot %d profile %v", id, got[fmt.Sprint(id)])
		}
	}
}

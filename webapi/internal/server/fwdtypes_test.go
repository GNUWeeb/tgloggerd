// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/testdb"
)

// The content types of the forwarding routes (decision
// 2026-11-20-fwd-more-types): every flag on its own, type_options, the
// refusals, and the migration that added the flags beyond 000031's.

// The types of each direction, in the form's order. Written out rather
// than read from logdb, so a change to the order or the set shows here.
var (
	t2dTypes = []string{"text", "photo", "video", "animation", "video_note", "document",
		"sticker", "voice", "audio", "poll", "checklist", "dice", "game", "location", "contact",
		"story", "paid_media", "invoice", "service", "other"}
	d2tTypes = []string{"text", "photo", "video", "animation", "document", "sticker", "audio"}
)

// flagsOf is the CONCAT_WS of the fwd_<type> columns of types.
func flagsOf(types []string) string {
	cols := make([]string, len(types))
	for i, k := range types {
		cols[i] = "fwd_" + k
	}
	return "CONCAT_WS(' ', " + strings.Join(cols, ", ") + ")"
}

// oneHot is the flags of all with only on set, as flagsOf reads them.
func oneHot(all []string, on ...string) string {
	out := make([]string, len(all))
	for i, k := range all {
		out[i] = "0"
		for _, o := range on {
			if o == k {
				out[i] = "1"
			}
		}
	}
	return strings.Join(out, " ")
}

func TestFwdTypeOptions(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for path, want := range map[string][]string{
		"/platform-fwd/telegram-discord": t2dTypes,
		"/platform-fwd/discord-telegram": d2tTypes,
	} {
		st, b := e.page("admin", path)
		if st != 200 {
			t.Fatalf("%s: %d %v", path, st, b)
		}
		if got := js(b["props"].(map[string]any)["type_options"]); got != js(want) {
			t.Errorf("%s type_options %s, want %s", path, got, js(want))
		}
	}
}

// routeTypes is the types the page answers for configuration id.
func routeTypes(t *testing.T, e *adminEnv, path string, id uint64) string {
	t.Helper()
	_, b := e.page("admin", path)
	for _, r := range b["props"].(map[string]any)["routes"].([]any) {
		if m := r.(map[string]any); uint64(m["id"].(float64)) == id {
			return js(m["types"])
		}
	}
	t.Fatalf("%s: no route %d", path, id)
	return ""
}

// Each type is stored in its own column and read back on its own, so
// a route can carry polls and drop service messages, and the whole set
// round-trips.
func TestFwdTypesRoundTrip(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	b := e.want(201, "admin", "POST", t2dBase, map[string]any{"chat_id": rustChat,
		"chat_kind": "group", "webhook_url": newHook, "types": []string{"poll"}, "enabled": true})
	hook := uint64(b["id"].(float64))
	b = e.want(201, "admin", "POST", d2tBase, map[string]any{
		"discord_channel_id": "710000000000000003", "chat_id": rustChat,
		"discord_bot_id": discordBot, "telegram_bot_id": 1, "types": []string{"audio"}, "enabled": true})
	route := uint64(b["id"].(float64))

	for _, c := range []struct {
		base, page, table string
		id                uint64
		all               []string
	}{
		{t2dBase, "/platform-fwd/telegram-discord", "telegram_discord_webhooks", hook, t2dTypes},
		{d2tBase, "/platform-fwd/discord-telegram", "discord_telegram_routes", route, d2tTypes},
	} {
		q := "SELECT " + flagsOf(c.all) + " FROM " + c.table + " WHERE id = ?"
		for _, k := range c.all {
			e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", c.base, c.id),
				map[string]any{"types": []string{k}})
			if got := e.one(q, c.id); got != oneHot(c.all, k) {
				t.Errorf("%s %s stored %q, want %q", c.table, k, got, oneHot(c.all, k))
			}
			if got := routeTypes(t, e, c.page, c.id); got != js([]string{k}) {
				t.Errorf("%s %s answered %s", c.table, k, got)
			}
		}
		// The whole set, given out of order and with a repeat, is
		// answered in the form's order.
		rev := make([]string, 0, len(c.all)+1)
		for i := len(c.all) - 1; i >= 0; i-- {
			rev = append(rev, c.all[i])
		}
		rev = append(rev, c.all[0])
		e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", c.base, c.id), map[string]any{"types": rev})
		if got := e.one(q, c.id); got != oneHot(c.all, c.all...) {
			t.Errorf("%s all stored %q", c.table, got)
		}
		if got := routeTypes(t, e, c.page, c.id); got != js(c.all) {
			t.Errorf("%s all answered %s", c.table, got)
		}
	}

	// Polls without service messages, the case the item names.
	e.want(204, "admin", "PATCH", fmt.Sprintf("%s/%d", t2dBase, hook),
		map[string]any{"types": []string{"text", "poll"}})
	if got := e.one("SELECT CONCAT_WS(' ', fwd_text, fwd_poll, fwd_service, fwd_other) "+
		"FROM telegram_discord_webhooks WHERE id = ?", hook); got != "1 1 0 0" {
		t.Errorf("polls without service messages: %q", got)
	}
}

// Writes refuse an empty set and a type the direction does not have,
// and leave the route as it was.
func TestFwdTypesRefused(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, c := range []struct {
		path  string
		types []string
	}{
		{t2dBase + "/1", []string{}},
		{t2dBase + "/1", []string{"text", "giveaway"}},
		{t2dBase + "/1", []string{"Text"}},
		{d2tBase + "/1", []string{}},
		{d2tBase + "/1", []string{"voice"}},
		{d2tBase + "/1", []string{"text", "other"}},
		{d2tBase + "/1", []string{"service"}},
	} {
		st, b, _ := e.call("admin", "PATCH", c.path, map[string]any{"types": c.types})
		if st != 400 || b["error"] != "bad_request" {
			t.Errorf("%s %v: %d %v", c.path, c.types, st, b)
		}
	}
	if got := e.one("SELECT " + flagsOf(t2dTypes) + " FROM telegram_discord_webhooks WHERE id = 1"); got !=
		oneHot(t2dTypes, t2dTypes...) {
		t.Errorf("webhook 1 changed: %q", got)
	}
	if got := e.one("SELECT " + flagsOf(d2tTypes) + " FROM discord_telegram_routes WHERE id = 1"); got !=
		oneHot(d2tTypes, d2tTypes...) {
		t.Errorf("route 1 changed: %q", got)
	}
}

// migrationFile is the forwarding types migration's up or down file,
// found by name so that renumbering it does not break the test.
func migrationFile(t *testing.T, dir string) string {
	t.Helper()
	m, err := filepath.Glob("../../../migrations/*_forward_more_content_types." + dir + ".sql")
	if err != nil || len(m) != 1 {
		t.Fatalf("forward_more_content_types.%s.sql: %v %v", dir, m, err)
	}
	b, err := os.ReadFile(m[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The migration gives each new flag the value of the flag its type
// followed until then, so no existing route changes what it forwards,
// and its down migration undoes it.
func TestFwdTypesMigrationCopiesFlags(t *testing.T) {
	t.Parallel()
	d := testdb.New(t, testdb.Logger)
	cfg, err := mysql.ParseDSN(d.DSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	one := func(q string) string {
		t.Helper()
		var v string
		if err := db.QueryRow(q).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	up, down := migrationFile(t, "up"), migrationFile(t, "down")

	// A later migration adds Discord route flags after these; undo it
	// first, so this one's down and up run on the schema they expect.
	if later, _ := filepath.Glob("../../../migrations/*_discord_forward_more_kinds.down.sql"); len(later) == 1 {
		b, err := os.ReadFile(later[0])
		if err != nil {
			t.Fatal(err)
		}
		exec(string(b))
	}

	// Back to the schema before it, with routes as they were then.
	exec(down)
	exec("INSERT INTO telegram_discord_webhooks (id, telegram_chat_id, telegram_chat_type, " +
		"webhook_url, fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document, fwd_other) VALUES " +
		"(1, -1001, 'group', 'https://discord.com/api/webhooks/1/a', 1, 1, 1, 1, 1, 0), " +
		"(2, -1002, 'group', 'https://discord.com/api/webhooks/2/b', 0, 1, 0, 0, 0, 1)")
	exec("INSERT INTO telegram_bots (id, token) VALUES (1, '1:x')")
	exec("INSERT INTO discord_telegram_routes (id, discord_channel_id, telegram_chat_id, " +
		"telegram_bot_id, fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document) VALUES " +
		"(1, 11, -1001, 1, 1, 1, 0, 1, 1), (2, 12, -1001, 1, 1, 1, 1, 1, 0), " +
		"(3, 13, -1001, 1, 0, 1, 0, 0, 0)")

	check := func(when string) {
		t.Helper()
		newT2D := t2dTypes[:0:0]
		for _, k := range t2dTypes {
			switch k {
			case "text", "photo", "video", "document", "sticker", "other":
			default:
				newT2D = append(newT2D, k)
			}
		}
		for id, want := range map[int]string{1: "0", 2: "1"} {
			got := one(fmt.Sprintf("SELECT %s FROM telegram_discord_webhooks WHERE id = %d",
				flagsOf(newT2D), id))
			if got != strings.TrimSpace(strings.Repeat(want+" ", len(newT2D))) {
				t.Errorf("%s: webhook %d new flags %q, want all %s (fwd_other)", when, id, got, want)
			}
		}
		// fwd_audio follows fwd_document, fwd_animation fwd_photo.
		for id, want := range map[int]string{1: "1 0", 2: "0 1", 3: "0 0"} {
			got := one(fmt.Sprintf("SELECT CONCAT_WS(' ', fwd_audio, fwd_animation) "+
				"FROM discord_telegram_routes WHERE id = %d", id))
			if got != want {
				t.Errorf("%s: route %d audio, animation %q, want %q", when, id, got, want)
			}
		}
		// A route made afterwards forwards everything by default.
		exec("INSERT INTO telegram_discord_webhooks (id, telegram_chat_id, telegram_chat_type, " +
			"webhook_url) VALUES (9, -1009, 'group', 'https://discord.com/api/webhooks/9/z')")
		if got := one("SELECT " + flagsOf(t2dTypes) + " FROM telegram_discord_webhooks WHERE id = 9"); got !=
			oneHot(t2dTypes, t2dTypes...) {
			t.Errorf("%s: new webhook %q", when, got)
		}
		exec("DELETE FROM telegram_discord_webhooks WHERE id = 9")
	}
	exec(up)
	check("up")
	// Down drops exactly the new columns; up again copies again.
	exec(down)
	if got := one("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() " +
		"AND TABLE_NAME IN ('telegram_discord_webhooks', 'discord_telegram_routes') " +
		"AND COLUMN_NAME LIKE 'fwd\\_%'"); got != "11" {
		t.Errorf("down left %s fwd_ columns, want 11 (000031's)", got)
	}
	exec(up)
	check("up again")
}

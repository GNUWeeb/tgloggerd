// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A made-up token shaped like Discord's.
const importToken = "MTAwMDAwMDAwMDAwMDAwMDAwNw.GimpA.testSecretImported"

// The import stores the environment's bot owned by the account named,
// points the routes without a Discord bot at it, and is a no-op the
// second time.
func TestImportDiscordBot(t *testing.T) {
	t.Parallel()
	ro, app := assignDBs(t)
	ctx := context.Background()

	var out strings.Builder
	if err := importDiscordBot(ctx, ro.DB, app.DB, "admin", importToken, &out); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := ro.QueryRow("SELECT id FROM discord_bots WHERE token = ?", importToken).Scan(&id); err != nil {
		t.Fatal(err)
	}
	want := "Discord bot " + id + " stored as \"Discord bot\"\n" +
		"Owner: admin (id 1), given now\n" +
		"Routes pointed at it: 2\n"
	if out.String() != want {
		t.Errorf("output\n%s\nwant\n%s", out.String(), want)
	}
	if strings.Contains(out.String(), "testSecretImported") {
		t.Error("the token was printed")
	}
	var n int
	if err := ro.QueryRow("SELECT COUNT(*) FROM discord_telegram_routes WHERE discord_bot_id = ?",
		id).Scan(&n); err != nil || n != 2 {
		t.Errorf("%d routes, %v", n, err)
	}
	if rel := relations(t, app); !strings.Contains(rel, "discord_bot "+id+" 1 cmd") {
		t.Errorf("relations\n%s", rel)
	}
	var audits int
	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set' "+
		"AND web_user_id IS NULL AND target_user_id = 1 AND detail = ?", "discord_bot "+id).Scan(&audits); err != nil ||
		audits != 1 {
		t.Errorf("%d audit rows, %v", audits, err)
	}

	// Twice: nothing changes, whoever it names.
	out.Reset()
	if err := importDiscordBot(ctx, ro.DB, app.DB, "ops", importToken, &out); err != nil {
		t.Fatal(err)
	}
	want = "Discord bot " + id + " stored already\n" +
		"Owner: admin (id 1), as before\n" +
		"Routes pointed at it: 0\n" +
		"Nothing to do.\n"
	if out.String() != want {
		t.Errorf("second run\n%s\nwant\n%s", out.String(), want)
	}
	if err := ro.QueryRow("SELECT COUNT(*) FROM discord_bots").Scan(&n); err != nil || n != 1 {
		t.Errorf("%d bots, %v", n, err)
	}
	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set' AND detail = ?",
		"discord_bot "+id).Scan(&audits); err != nil || audits != 1 {
		t.Errorf("%d audit rows after the second run, %v", audits, err)
	}
}

// Without a usable token or an active superadmin nothing is stored.
func TestImportDiscordBotRefuses(t *testing.T) {
	t.Parallel()
	ro, app := assignDBs(t)
	ctx := context.Background()
	for _, c := range []struct{ user, token string }{
		{"admin", ""},
		{"admin", "not-a-token"},
		{"admin", "7000000001:AA" + strings.Repeat("x", 33)},
		{"gw_admin", importToken},
		{"intern2025", importToken},
		{"nobody", importToken},
	} {
		var out strings.Builder
		err := importDiscordBot(ctx, ro.DB, app.DB, c.user, c.token, &out)
		var refused errRefused
		if !errors.As(err, &refused) || out.Len() != 0 {
			t.Errorf("%s %q: %v %q", c.user, c.token, err, out.String())
		}
		if err != nil && strings.Contains(err.Error(), "testSecretImported") {
			t.Errorf("the refusal names the token: %v", err)
		}
	}
	var n int
	if err := ro.QueryRow("SELECT COUNT(*) FROM discord_bots").Scan(&n); err != nil || n != 0 {
		t.Errorf("%d bots after refusals, %v", n, err)
	}
	if err := ro.QueryRow("SELECT COUNT(*) FROM discord_telegram_routes WHERE discord_bot_id IS NOT NULL").
		Scan(&n); err != nil || n != 0 {
		t.Errorf("%d routes pointed after refusals, %v", n, err)
	}
}

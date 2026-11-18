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

	"tgloggerd/webapi/internal/testdb"
)

// The logger's "sample" and "forwarding" fixtures and the web's
// "accounts", "admin" and "ownership": four of the resources have an
// owner, the others none.
func assignDBs(t *testing.T) (ro, app *testdb.DB) {
	t.Helper()
	ro = testdb.New(t, testdb.Logger, "sample", "forwarding")
	app = testdb.New(t, testdb.Web, "accounts", "admin", "ownership")
	return ro, app
}

// relations is every owner row, as "<type> <id> <owner> <created_by>".
func relations(t *testing.T, app *testdb.DB) string {
	t.Helper()
	rows, err := app.Query("SELECT CONCAT_WS(' ', resource_type, resource_id, user_id, " +
		"COALESCE(created_by, 'cmd')) FROM web_resource_relations WHERE relation = 'owner' " +
		"ORDER BY resource_type, resource_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}

func TestAssignOwner(t *testing.T) {
	t.Parallel()
	ro, app := assignDBs(t)
	ctx := context.Background()

	var out strings.Builder
	if err := assignOwner(ctx, ro.DB, app.DB, "admin", &out); err != nil {
		t.Fatal(err)
	}
	want := `Owner: admin (id 1)
mcp_group     2 assigned: -1001000000010 -1001000000003; 1 owned already
mcp_token     1 assigned to their creators: 2; 1 owned already
telegram_bot  2 assigned: 1 2; 0 owned already
t2d_webhook   2 assigned: 2 3; 1 owned already
d2t_route     1 assigned: 2; 1 owned already
discord_bot   0 assigned; 0 owned already
Assigned 8 resources.
`
	if out.String() != want {
		t.Errorf("output\n%s\nwant\n%s", out.String(), want)
	}
	// Only the unowned ones changed: the fixture's owners stay. MCP
	// token 2 goes to ops, which minted it, not to the account named.
	wantRel := `d2t_route 1 7 1
d2t_route 2 1 cmd
mcp_group -1001000000010 1 cmd
mcp_group -1001000000003 1 cmd
mcp_group -1001000000001 2 2
mcp_token 1 1 1
mcp_token 2 2 cmd
t2d_webhook 1 3 1
t2d_webhook 2 1 cmd
t2d_webhook 3 1 cmd
telegram_bot 1 1 cmd
telegram_bot 2 1 cmd`
	if got := relations(t, app); got != wantRel {
		t.Errorf("relations\n%s\nwant\n%s", got, wantRel)
	}
	// Audited, one row per resource, by no account, with the owner as
	// the target.
	var n int
	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set' " +
		"AND web_user_id IS NULL AND target_user_id = 1 AND detail LIKE '% assign-owner'").Scan(&n); err != nil ||
		n != 7 {
		t.Errorf("%d audit rows, %v", n, err)
	}

	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set' " +
		"AND web_user_id IS NULL AND target_user_id = 2 AND detail LIKE '% assign-owner'").Scan(&n); err != nil ||
		n != 1 {
		t.Errorf("%d audit rows for the token's creator, %v", n, err)
	}

	// Twice is a no-op, whoever it names.
	out.Reset()
	if err := assignOwner(ctx, ro.DB, app.DB, "ops", &out); err != nil {
		t.Fatal(err)
	}
	want = `Owner: ops (id 2)
mcp_group     0 assigned; 3 owned already
mcp_token     0 assigned to their creators; 2 owned already
telegram_bot  0 assigned; 2 owned already
t2d_webhook   0 assigned; 3 owned already
d2t_route     0 assigned; 2 owned already
discord_bot   0 assigned; 0 owned already
Assigned 0 resources.
`
	if out.String() != want {
		t.Errorf("second run\n%s\nwant\n%s", out.String(), want)
	}
	if got := relations(t, app); got != wantRel {
		t.Errorf("relations after the second run\n%s", got)
	}
	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set'").Scan(&n); err != nil ||
		n != 8 {
		t.Errorf("%d audit rows after the second run, %v", n, err)
	}

	// A resource made since is assigned by the next run, and only it.
	if _, err := ro.Exec("INSERT INTO telegram_bots (id, token) VALUES (100, '7000000100:testSecretLater')"); err != nil {
		t.Fatal(err)
	}
	if _, err := ro.Exec("INSERT INTO discord_bots (id, token) VALUES (1000000100, 'MTAw.a.testSecretLater')"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := assignOwner(ctx, ro.DB, app.DB, "admin", &out); err != nil ||
		!strings.Contains(out.String(), "telegram_bot  1 assigned: 100; 2 owned already\n") ||
		!strings.Contains(out.String(), "discord_bot   1 assigned: 1000000100; 0 owned already\n") ||
		!strings.HasSuffix(out.String(), "Assigned 2 resources.\n") {
		t.Errorf("later run %v\n%s", err, out.String())
	}
}

// Only an active superadmin is assigned to; a refusal assigns nothing.
func TestAssignOwnerRefuses(t *testing.T) {
	t.Parallel()
	ro, app := assignDBs(t)
	ctx := context.Background()
	if _, err := app.Exec("INSERT INTO web_users (id, username, password_hash, role, is_active) " +
		"VALUES (100, 'old_boss', 'x', 'superadmin', 0)"); err != nil {
		t.Fatal(err)
	}
	before := relations(t, app)
	for name, reason := range map[string]string{
		"nobody":     `no account is named "nobody"`,
		"old_boss":   `the account "old_boss" is disabled`,
		"intern2025": `the account "intern2025" is disabled`,
		"gw_admin":   `the account "gw_admin" is not a superadmin (role admin)`,
		"researcher": `the account "researcher" is not a superadmin (role member)`,
		"":           `no account is named ""`,
	} {
		var out strings.Builder
		err := assignOwner(ctx, ro.DB, app.DB, name, &out)
		var refused errRefused
		if !errors.As(err, &refused) || err.Error() != reason {
			t.Errorf("%q: %v, want %q", name, err, reason)
		}
		if out.Len() != 0 {
			t.Errorf("%q printed %q", name, out.String())
		}
	}
	if got := relations(t, app); got != before {
		t.Errorf("relations changed by refusals:\n%s", got)
	}
	var n int
	if err := app.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'owner_set'").Scan(&n); err != nil ||
		n != 0 {
		t.Errorf("%d audit rows after refusals, %v", n, err)
	}
}

func TestCommandUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"assign-owner"}, {"assign-owner", ""},
		{"assign-owner", "a", "b"}, {"serve"}} {
		if st := command(args); st != 2 {
			t.Errorf("%v: exit %d", args, st)
		}
	}
}

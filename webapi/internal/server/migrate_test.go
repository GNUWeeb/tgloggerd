// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"os"
	"slices"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// Web migration 000004 upgrades today's accounts in place: admin
// becomes superadmin and viewer member, with their passwords and
// session epochs kept, so they sign in and stay signed in.
func TestMigrationUpgradesRoles(t *testing.T) {
	t.Parallel()
	db := emptyAppDB(t)
	migrateAppDB(t, db, "000003")
	hash, _ := auth.HashPassword("before-the-upgrade")
	if _, err := db.Exec("INSERT INTO web_users (username, password_hash, role, "+
		"session_epoch) VALUES ('boss', ?, 'admin', 3), ('peek', ?, 'viewer', 1)",
		hash, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO web_audit (web_user_id, action) VALUES (1, 'login_ok')"); err != nil {
		t.Fatal(err)
	}
	migrateAppDB(t, db, "000004")

	rows, err := db.Query("SELECT username, role, password_hash, session_epoch, " +
		"is_active, must_change_password, display_name, email IS NULL, " +
		"created_by IS NULL FROM web_users ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := map[string][2]any{"boss": {"superadmin", uint32(3)}, "peek": {"member", uint32(1)}}
	n := 0
	for rows.Next() {
		var (
			name, role, h, display     string
			epoch                      uint32
			active, must, noMail, noBy bool
		)
		if err := rows.Scan(&name, &role, &h, &epoch, &active, &must, &display,
			&noMail, &noBy); err != nil {
			t.Fatal(err)
		}
		w := want[name]
		if role != w[0] || epoch != w[1] || h != hash || !active || must || display != "" ||
			!noMail || !noBy {
			t.Errorf("%s after the upgrade: %s %d %v %v %q", name, role, epoch, active, must, display)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("%d accounts after the upgrade", n)
	}
	var target any
	if err := db.QueryRow("SELECT target_user_id FROM web_audit").Scan(&target); err != nil || target != nil {
		t.Fatalf("audit row after the upgrade: %v %v", target, err)
	}

	// Both sign in with their old passwords, with the new roles, once
	// the later migrations are in as well.
	migrateAppDB(t, db, "")
	h := newHandler(db, 5)
	for name, role := range map[string]string{"boss": "superadmin", "peek": "member"} {
		c := &client{t: t, h: h, cookies: map[string]string{}}
		a := c.login(name, "before-the-upgrade")
		if a.status != 200 || a.body["user"].(map[string]any)["role"] != role {
			t.Fatalf("%s signs in: %d %v", name, a.status, a.body)
		}
	}
}

// Web migration 000005 only adds tables: the accounts stay as they
// were, signed-in sessions with them, and its down migration leaves
// the schema of 000004.
func TestMigrationAddsSettings(t *testing.T) {
	t.Parallel()
	db := emptyAppDB(t)
	migrateAppDB(t, db, "000004")
	hash, _ := auth.HashPassword("before-the-upgrade")
	if _, err := db.Exec("INSERT INTO web_users (username, password_hash, role, "+
		"session_epoch) VALUES ('boss', ?, 'superadmin', 2)", hash); err != nil {
		t.Fatal(err)
	}
	tables := func() int {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables " +
			"WHERE table_schema = DATABASE() AND table_name IN ('web_user_settings', " +
			"'web_pinned_groups', 'web_saved_searches', 'web_sessions')").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	migrateAppDB(t, db, "000005")
	if n := tables(); n != 4 {
		t.Fatalf("%d of the 4 tables after the upgrade", n)
	}
	var epoch uint32
	if err := db.QueryRow("SELECT session_epoch FROM web_users WHERE username = 'boss'").
		Scan(&epoch); err != nil || epoch != 2 {
		t.Fatalf("account after the upgrade: epoch %d, %v", epoch, err)
	}

	down, err := os.ReadFile("../../../web/migrations/000005_create_account_settings.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down: %v", err)
	}
	if n := tables(); n != 0 {
		t.Fatalf("%d tables left after the down migration", n)
	}
}

// Web migration 000006 adds the federations and keeps today's rights:
// every admin administers the Default federation (id 0) and every
// member, disabled ones too, is filed in it, so an admin still manages
// every member. Superadmins get no rows (they administer everything).
// Its down migration drops the tables again.
func TestMigrationAddsFederations(t *testing.T) {
	t.Parallel()
	db := emptyAppDB(t)
	migrateAppDB(t, db, "000005")
	hash, _ := auth.HashPassword("before-the-upgrade")
	if _, err := db.Exec("INSERT INTO web_users (id, username, password_hash, role, is_active) "+
		"VALUES (1, 'boss', ?, 'superadmin', 1), (2, 'lead', ?, 'admin', 1), "+
		"(3, 'old_lead', ?, 'admin', 0), (4, 'peek', ?, 'member', 1), "+
		"(5, 'gone', ?, 'member', 0)", hash, hash, hash, hash, hash); err != nil {
		t.Fatal(err)
	}
	migrateAppDB(t, db, "000006")

	var (
		id      uint64
		name    string
		by, at  any
		nGroups int
	)
	if err := db.QueryRow("SELECT id, name, created_by, created_at FROM web_federations").
		Scan(&id, &name, &by, &at); err != nil || id != 0 || name != "Default" || by != nil || at != nil {
		t.Fatalf("Default after the upgrade: %d %q %v %v, %v", id, name, by, at, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM web_federation_groups").Scan(&nGroups); err != nil ||
		nGroups != 0 {
		t.Fatalf("%d stored groups after the upgrade, %v", nGroups, err)
	}
	usersIn := func(table string) []uint64 {
		rows, err := db.Query("SELECT web_user_id FROM " + table +
			" WHERE federation_id = 0 ORDER BY web_user_id")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []uint64
		for rows.Next() {
			var u uint64
			if err := rows.Scan(&u); err != nil {
				t.Fatal(err)
			}
			out = append(out, u)
		}
		return out
	}
	if got := usersIn("web_federation_admins"); !slices.Equal(got, []uint64{2, 3}) {
		t.Errorf("admins of Default after the upgrade: %v", got)
	}
	if got := usersIn("web_federation_members"); !slices.Equal(got, []uint64{4, 5}) {
		t.Errorf("members of Default after the upgrade: %v", got)
	}

	// The admin still manages both members once the service reads the
	// federations.
	migrateAppDB(t, db, "")
	h := newHandler(db, 5)
	lead := newSession(t, h, "lead", "before-the-upgrade")
	if got := lead.call("GET", "/api/accounts", nil); got.status != 200 ||
		!slices.Equal(ids(got), []string{"gone", "peek"}) {
		t.Fatalf("lead's accounts after the upgrade: %d %v", got.status, got.body)
	}

	down, err := os.ReadFile("../../../web/migrations/000006_create_federations.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down: %v", err)
	}
	var left int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables " +
		"WHERE table_schema = DATABASE() AND table_name LIKE 'web_federation%'").Scan(&left); err != nil ||
		left != 0 {
		t.Fatalf("%d tables left after the down migration, %v", left, err)
	}
}

// Web migration 000008 only adds web_resource_relations, empty: every
// resource starts with no owner (the superadmins'), and the accounts
// stay as they were. One owner per resource, any number of other
// relations. Its down migration drops the table again.
func TestMigrationAddsResourceRelations(t *testing.T) {
	t.Parallel()
	db := emptyAppDB(t)
	migrateAppDB(t, db, "000006")
	hash, _ := auth.HashPassword("before-the-upgrade")
	if _, err := db.Exec("INSERT INTO web_users (id, username, password_hash, role, "+
		"session_epoch) VALUES (1, 'boss', ?, 'superadmin', 2), (2, 'peek', ?, 'member', 1)",
		hash, hash); err != nil {
		t.Fatal(err)
	}
	migrateAppDB(t, db, "000008")
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_resource_relations").Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d relations after the upgrade, %v", n, err)
	}
	var epoch uint32
	if err := db.QueryRow("SELECT session_epoch FROM web_users WHERE username = 'boss'").
		Scan(&epoch); err != nil || epoch != 2 {
		t.Fatalf("account after the upgrade: epoch %d, %v", epoch, err)
	}
	ins := func(rel string, uid int) error {
		_, err := db.Exec("INSERT INTO web_resource_relations (resource_type, resource_id, "+
			"relation, user_id) VALUES ('mcp_group', -1001, ?, ?)", rel, uid)
		return err
	}
	if err := ins("owner", 1); err != nil {
		t.Fatal(err)
	}
	if err := ins("owner", 2); err == nil {
		t.Error("a second owner")
	}
	if err := ins("viewer", 1); err != nil {
		t.Error(err)
	}
	if err := ins("viewer", 2); err != nil {
		t.Error(err)
	}
	if err := ins("owner", 99); err == nil {
		t.Error("an owner that is no account")
	}
	if _, err := db.Exec("DELETE FROM web_users WHERE id = 1"); err == nil {
		t.Error("deleted an account that owns a resource")
	}

	down, err := os.ReadFile("../../../web/migrations/000008_create_resource_relations.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables " +
		"WHERE table_schema = DATABASE() AND table_name = 'web_resource_relations'").Scan(&n); err != nil ||
		n != 0 {
		t.Fatalf("%d tables left after the down migration, %v", n, err)
	}
}

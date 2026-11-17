// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
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

	// Both sign in with their old passwords, with the new roles.
	h := newHandler(db, 5)
	for name, role := range map[string]string{"boss": "superadmin", "peek": "member"} {
		c := &client{t: t, h: h, cookies: map[string]string{}}
		a := c.login(name, "before-the-upgrade")
		if a.status != 200 || a.body["user"].(map[string]any)["role"] != role {
			t.Fatalf("%s signs in: %d %v", name, a.status, a.body)
		}
	}
}

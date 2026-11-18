// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package testdb

import (
	"fmt"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

func TestSkipsWithoutServer(t *testing.T) {
	t.Setenv(EnvDSN, "")
	var inner *testing.T
	t.Run("new", func(t *testing.T) {
		inner = t
		New(t, Web)
		t.Error("New did not skip")
	})
	if !inner.Skipped() {
		t.Fatal("not skipped")
	}
}

func count(t *testing.T, d *DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// A test's database has the template's tables, indexes and foreign
// keys, and is a database of its own.
func TestMatchesTemplate(t *testing.T) {
	t.Parallel()
	for _, s := range []Schema{Logger, Web} {
		d := New(t, s)
		tpl, err := srv.template(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME <> '" + readyTable + "'",
			"SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME <> '" + readyTable + "'",
			"SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = ?",
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME <> '" + readyTable + "'",
		} {
			got, want := count(t, d, q, d.Name), count(t, d, q, tpl.name)
			if got != want || want == 0 {
				t.Errorf("%s: %s: %d in the copy, %d in the template", s.name, q, got, want)
			}
		}
		if n := count(t, d, "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS "+
			"WHERE CONSTRAINT_SCHEMA = ? AND UNIQUE_CONSTRAINT_SCHEMA <> ?", d.Name, d.Name); n != 0 {
			t.Errorf("%s: %d foreign keys point out of the database", s.name, n)
		}
	}
}

func TestIsolated(t *testing.T) {
	t.Parallel()
	a, b := New(t, Web), New(t, Web)
	if a.Name == b.Name {
		t.Fatalf("both are %s", a.Name)
	}
	if _, err := a.Exec("INSERT INTO web_users (username, password_hash) VALUES ('only_a', 'x')"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, b, "SELECT COUNT(*) FROM web_users"); n != 0 {
		t.Fatalf("%d rows in the other database", n)
	}
	if n := count(t, a, "SELECT COUNT(*) FROM web_users"); n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// Many tests at once, each with its own rows under the same ids.
func TestParallel(t *testing.T) {
	t.Parallel()
	for i := 0; i < 12; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			d := New(t, Logger)
			if _, err := d.Exec("INSERT INTO telegram_files (id, tg_file_id) VALUES (1, ?)",
				fmt.Sprint("tg-", i)); err != nil {
				t.Fatal(err)
			}
			var got string
			if err := d.QueryRow("SELECT tg_file_id FROM telegram_files").Scan(&got); err != nil ||
				got != fmt.Sprint("tg-", i) {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestDropped(t *testing.T) {
	t.Parallel()
	var name string
	t.Run("inner", func(t *testing.T) {
		name = New(t, Web).Name
	})
	if name == "" {
		t.Skip("no database was made")
	}
	d := New(t, Web)
	if n := count(t, d, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?",
		name); n != 0 {
		t.Fatalf("%s is still there", name)
	}
}

// The fixtures load, and their counters agree with their rows the way
// the daemon keeps them.
func TestFixtures(t *testing.T) {
	t.Parallel()
	lg := New(t, Logger, "sample")
	for q, want := range map[string]int{
		"SELECT COUNT(*) FROM telegram_users":            8,
		"SELECT COUNT(*) FROM telegram_groups":           6,
		"SELECT COUNT(*) FROM telegram_group_messages":   16,
		"SELECT COUNT(*) FROM telegram_private_messages": 6,
		"SELECT COUNT(*) FROM telegram_files":            8,
		"SELECT COUNT(*) FROM telegram_public_groups":    3,
		// msg_count as scripts/prefill_msg_count.py computes it.
		"SELECT COUNT(*) FROM telegram_groups g WHERE msg_count <> " +
			"(SELECT COUNT(1) FROM telegram_group_messages m WHERE m.chat_id = g.id)": 0,
		"SELECT COUNT(*) FROM telegram_users u WHERE msg_count <> " +
			"(SELECT COUNT(1) FROM telegram_group_messages m WHERE m.sender_user_id = u.id) + " +
			"(SELECT COUNT(1) FROM telegram_private_messages m WHERE m.sender_id = u.id)": 0,
		// Telegram message ids are server ids shifted by 20.
		"SELECT COUNT(*) FROM telegram_group_messages WHERE message_id % 1048576 <> 0": 0,
	} {
		if n := count(t, lg, q); n != want {
			t.Errorf("%s: %d, want %d", q, n, want)
		}
	}

	web := New(t, Web, "accounts")
	rows, err := web.Query("SELECT username, password_hash, is_active FROM web_users ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name, hash string
		var active bool
		if err := rows.Scan(&name, &hash, &active); err != nil {
			t.Fatal(err)
		}
		if !auth.VerifyPassword(hash, "password") {
			t.Errorf("%s: the password is not \"password\"", name)
		}
		if active != (name != "intern2025") {
			t.Errorf("%s: active %v", name, active)
		}
		names = append(names, name)
	}
	if fmt.Sprint(names) != "[admin ops gw_admin sec_lead researcher mod_kernel intern2025]" {
		t.Errorf("accounts %v", names)
	}
	// The rows a migration made come along with id 0 kept (the
	// Default federation), and the fixture files its admins and
	// members there as the migration would.
	if n := count(t, web, "SELECT COUNT(*) FROM web_federations WHERE id = 0 AND name = 'Default'"); n != 1 {
		t.Errorf("%d Default federations", n)
	}
	if n := count(t, web, "SELECT COUNT(*) FROM web_federation_admins a JOIN web_users u "+
		"ON u.id = a.web_user_id AND u.role = 'admin' WHERE a.federation_id = 0"); n != 2 {
		t.Errorf("%d admins of Default", n)
	}
	if n := count(t, web, "SELECT COUNT(*) FROM web_federation_members m JOIN web_users u "+
		"ON u.id = m.web_user_id AND u.role = 'member' WHERE m.federation_id = 0"); n != 3 {
		t.Errorf("%d members in Default", n)
	}

	// Load adds to a database later, and a test's own rows follow the
	// fixture's ids.
	d := New(t, Web)
	d.Load(t, "accounts")
	res, err := d.Exec("INSERT INTO web_users (username, password_hash) VALUES ('mine', 'x')")
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := res.LastInsertId(); id != 8 {
		t.Errorf("the next account id is %d", id)
	}
}

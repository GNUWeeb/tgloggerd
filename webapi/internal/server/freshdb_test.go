// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/testdb"
)

// freshAppDB is a web database of the test's own, with every web
// migration applied (internal/testdb). Tests on it create every row
// they read and run in parallel.
func freshAppDB(t *testing.T) *sql.DB {
	t.Helper()
	return testdb.New(t, testdb.Web).DB
}

// migrateAppDB applies the web migrations not applied to db yet, up
// to and including the one numbered upto ("000004"; all with ""). It
// remembers the last one in a table of its own, test_migrated.
func migrateAppDB(t *testing.T, db *sql.DB, upto string) {
	t.Helper()
	ups, err := filepath.Glob("../../../web/migrations/*.up.sql")
	if err != nil || len(ups) == 0 {
		t.Fatalf("no web migrations found: %v", err)
	}
	sort.Strings(ups)
	var applied string
	_ = db.QueryRow("SELECT v FROM test_migrated").Scan(&applied)
	for _, f := range ups {
		n := filepath.Base(f)[:6]
		if n <= applied || (upto != "" && n > upto) {
			continue
		}
		q, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(q)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		applied = n
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS test_migrated (v CHAR(6) NOT NULL); " +
		"DELETE FROM test_migrated; INSERT INTO test_migrated VALUES ('" + applied + "')"); err != nil {
		t.Fatal(err)
	}
}

// downAppDB applies the down migrations of the web migrations after
// the one numbered downto ("000006"), newest first, as golang-migrate
// steps down to it.
func downAppDB(t *testing.T, db *sql.DB, downto string) {
	t.Helper()
	downs, err := filepath.Glob("../../../web/migrations/*.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(downs)))
	for _, f := range downs {
		if filepath.Base(f)[:6] <= downto {
			continue
		}
		q, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(q)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
	}
}

// emptyAppDB is an empty database of the test's own on the test
// server (testdb.EnvDSN), for the tests that apply the migrations
// themselves; it is dropped when the test ends.
func emptyAppDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(testdb.EnvDSN)
	if dsn == "" {
		t.Skip(testdb.EnvDSN + " is not set")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "test_upgrade_" + hex.EncodeToString(b)

	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})

	cfg.DBName = name
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

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
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// freshAppDB is a web database of the test's own: created on the
// server WEBAPI_TEST_APP_DSN names, as "<its database>_<random>", with
// every web migration (web/migrations) applied, and dropped when the
// test ends. Tests on it create every row they read and can run in
// parallel with each other and with the tests on the shared database.
func freshAppDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("WEBAPI_TEST_APP_DSN")
	if dsn == "" {
		t.Skip("WEBAPI_TEST_APP_DSN is not set")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.DBName, "test") {
		t.Fatalf("refusing to create databases beside %q: not a test database", cfg.DBName)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := cfg.DBName + "_" + hex.EncodeToString(b)

	admin, err := sql.Open("mysql", dsn)
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
	ups, err := filepath.Glob("../../../web/migrations/*.up.sql")
	if err != nil || len(ups) == 0 {
		t.Fatalf("no web migrations found: %v", err)
	}
	sort.Strings(ups)
	for _, f := range ups {
		q, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(q)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
	}
	return db
}

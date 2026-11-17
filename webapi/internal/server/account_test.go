// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// signIn signs a new client in and returns it with its CSRF token.
func signIn(t *testing.T, c *client, name, password string) string {
	t.Helper()
	a := c.login(name, password)
	if a.status != 200 {
		t.Fatalf("login %s: %d %v", name, a.status, a.body)
	}
	return a.body["csrf"].(string)
}

func TestChangePassword(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("old-secret")
	id := addUser(t, db, "carol", hash, "member", true)
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	other := &client{t: t, h: h, cookies: map[string]string{}}
	tok := signIn(t, c, "carol", "old-secret")
	signIn(t, other, "carol", "old-secret")

	if a := c.do("POST", "/api/account/password", "", map[string]string{
		"current_password": "old-secret", "new_password": "new-secret"}); a.status != 403 ||
		a.body["error"] != "csrf" {
		t.Fatalf("without CSRF: %d %v", a.status, a.body)
	}
	for _, tc := range []struct {
		cur, next, code string
	}{
		{"wrong", "new-secret", "wrong_password"},
		{"old-secret", "short", "weak_password"},
		{"old-secret", "old-secret", "weak_password"},
	} {
		a := c.do("POST", "/api/account/password", tok, map[string]string{
			"current_password": tc.cur, "new_password": tc.next})
		if a.status != 400 || a.body["error"] != tc.code {
			t.Fatalf("%s -> %s: %d %v", tc.cur, tc.next, a.status, a.body)
		}
	}
	if a := c.do("POST", "/api/account/password", tok, map[string]string{}); a.status != 400 ||
		a.body["error"] != "bad_request" {
		t.Fatalf("empty body: %d %v", a.status, a.body)
	}

	// No upper limit below the body cap: a long passphrase counts in
	// full (bcrypt would cut it at 72 bytes).
	long := strings.Repeat("correct horse battery staple ", 20)
	a := c.do("POST", "/api/account/password", tok, map[string]string{
		"current_password": "old-secret", "new_password": long})
	if a.status != 200 || a.body["csrf"] == "" || a.body["csrf"] == tok {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	// This session goes on with its new cookie; the other one ended.
	if a := c.do("GET", "/api/session", "", nil); a.status != 200 {
		t.Fatalf("session after the change: %d", a.status)
	}
	if a := other.do("GET", "/api/session", "", nil); a.status != 401 {
		t.Fatalf("other session after the change: %d", a.status)
	}
	var stored string
	if err := db.QueryRow("SELECT password_hash FROM web_users WHERE id = ?", id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(stored, long) || auth.VerifyPassword(stored, long[:72]) ||
		!strings.HasPrefix(stored, "$argon2id$") {
		t.Fatalf("stored hash %q", stored)
	}
	rows := audits(t, db)
	fails, ok := 0, 0
	for _, r := range rows {
		switch r.action {
		case "password_change_fail":
			fails++
		case "password_change":
			ok++
		}
	}
	if fails != 3 || ok != 1 {
		t.Fatalf("audit rows %+v", rows)
	}
}

// An account with a temporary password reaches nothing but its
// session, logout and the password change until it chose its own.
func TestPasswordGate(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("temporary-1")
	id := addUser(t, db, "dave", hash, "superadmin", true)
	if _, err := db.Exec("UPDATE web_users SET must_change_password = 1 WHERE id = ?", id); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, h: newHandler(db, 5), cookies: map[string]string{}}
	a := c.login("dave", "temporary-1")
	if a.status != 200 || a.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("login: %d %v", a.status, a.body)
	}
	tok := a.body["csrf"].(string)
	if a := c.do("GET", "/api/session", "", nil); a.status != 200 ||
		a.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("session: %d %v", a.status, a.body)
	}
	a = c.do("POST", "/api/account/password", tok, map[string]string{
		"current_password": "temporary-1", "new_password": "my-own-password"})
	if a.status != 200 {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	if a := c.do("GET", "/api/session", "", nil); a.status != 200 ||
		a.body["user"].(map[string]any)["must_change_password"] != false {
		t.Fatalf("session after the change: %d %v", a.status, a.body)
	}
}

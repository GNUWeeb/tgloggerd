// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// These tests run on a database of their own (freshAppDB), in
// parallel, with accounts they create.

// session is a signed-in client and its CSRF token.
type session struct {
	*client
	csrf string
}

func newSession(t *testing.T, h http.Handler, name, password string) session {
	t.Helper()
	c := &client{t: t, h: h, cookies: map[string]string{}}
	return session{c, signIn(t, c, name, password)}
}

func (s session) call(method, path string, body any) answer {
	return s.do(method, path, s.csrf, body)
}

// firstPassword signs a new account in with its temporary password and
// replaces it, as its owner would at the first sign-in.
func firstPassword(t *testing.T, h http.Handler, name, temp, own string) session {
	t.Helper()
	s := newSession(t, h, name, temp)
	a := s.call("POST", "/api/account/password",
		map[string]string{"current_password": temp, "new_password": own})
	if a.status != 200 {
		t.Fatalf("%s: first password change: %d %v", name, a.status, a.body)
	}
	s.csrf = a.body["csrf"].(string)
	return s
}

func create(t *testing.T, s session, body map[string]any) (uint64, string) {
	t.Helper()
	a := s.call("POST", "/api/accounts", body)
	if a.status != 201 {
		t.Fatalf("create %v: %d %v", body["username"], a.status, a.body)
	}
	pw, _ := a.body["temporary_password"].(string)
	if len(pw) != 20 {
		t.Fatalf("temporary password %q", pw)
	}
	return uint64(a.body["id"].(float64)), pw
}

func account(name, role string, grants ...map[string]any) map[string]any {
	b := map[string]any{"username": name, "name": strings.ToUpper(name),
		"email": name + "@example.org", "role": role}
	if grants != nil {
		b["grants"] = grants
	}
	return b
}

var defaultGrant = map[string]any{"fed": 0, "group_ids": []int{}}

func ids(a answer) []string {
	var out []string
	for _, r := range a.body["accounts"].([]any) {
		out = append(out, r.(map[string]any)["username"].(string))
	}
	return out
}

// A superadmin creates an admin and a member; each signs in with its
// temporary password, must change it first, and then sees and does
// what its role allows (web-roles "Done when").
func TestAccountsRoles(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("root-password")
	rootID := addUser(t, db, "root", hash, "superadmin", true)
	h := newHandler(db, 50)
	root := newSession(t, h, "root", "root-password")

	adminID, adminTemp := create(t, root, account("ann", "admin"))
	memberID, memberTemp := create(t, root, account("max", "member"))

	// The temporary password works once, behind the gate.
	ann := newSession(t, h, "ann", adminTemp)
	if a := ann.call("GET", "/api/accounts", nil); a.status != 403 ||
		a.body["error"] != "password_change_required" {
		t.Fatalf("admin behind the gate: %d %v", a.status, a.body)
	}
	if a := ann.call("POST", "/api/accounts", account("x", "member", defaultGrant)); a.status != 403 ||
		a.body["error"] != "password_change_required" {
		t.Fatalf("create behind the gate: %d %v", a.status, a.body)
	}
	ann = firstPassword(t, h, "ann", adminTemp, "ann-own-password")
	maxs := firstPassword(t, h, "max", memberTemp, "max-own-password")
	if a := ann.call("GET", "/api/session", nil); a.body["user"].(map[string]any)["role"] != "admin" {
		t.Fatalf("admin session %v", a.body)
	}

	// The superadmin sees everyone; the admin only the members.
	if a := root.call("GET", "/api/accounts", nil); a.status != 200 ||
		strings.Join(ids(a), ",") != "ann,max,root" {
		t.Fatalf("superadmin list: %d %v", a.status, a.body)
	}
	if a := root.call("GET", "/api/accounts?role=admin", nil); strings.Join(ids(a), ",") != "ann" {
		t.Fatalf("superadmin list of admins: %v", a.body)
	}
	if a := ann.call("GET", "/api/accounts", nil); a.status != 200 ||
		strings.Join(ids(a), ",") != "max" {
		t.Fatalf("admin list: %d %v", a.status, a.body)
	}
	// Accounts the admin does not manage do not exist for it.
	for _, id := range []uint64{rootID, adminID, 999999} {
		if a := ann.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.status != 404 {
			t.Fatalf("admin GET %d: %d", id, a.status)
		}
		if a := ann.call("POST", fmt.Sprintf("/api/accounts/%d/disable", id), nil); a.status != 404 {
			t.Fatalf("admin disable %d: %d", id, a.status)
		}
	}
	// To a member the administration does not exist at all.
	for _, op := range [][2]string{{"GET", "/api/accounts"}, {"POST", "/api/accounts"},
		{"GET", fmt.Sprintf("/api/accounts/%d", memberID)},
		{"POST", fmt.Sprintf("/api/accounts/%d/sign-out-everywhere", memberID)}} {
		if a := maxs.call(op[0], op[1], account("y", "member")); a.status != 404 {
			t.Fatalf("member %s %s: %d", op[0], op[1], a.status)
		}
	}

	// An admin creates members only, filed in a federation it
	// administers.
	if a := ann.call("POST", "/api/accounts", account("bea", "admin", defaultGrant)); a.status != 403 ||
		a.body["error"] != "forbidden" {
		t.Fatalf("admin creates an admin: %d %v", a.status, a.body)
	}
	if a := ann.call("POST", "/api/accounts", account("bea", "member")); a.status != 400 ||
		a.body["error"] != "grants_required" {
		t.Fatalf("admin creates a member without grants: %d %v", a.status, a.body)
	}
	if a := ann.call("POST", "/api/accounts",
		account("bea", "member", map[string]any{"fed": 7, "group_ids": []int{}})); a.status != 403 {
		t.Fatalf("admin grants another federation: %d %v", a.status, a.body)
	}
	if a := ann.call("POST", "/api/accounts",
		account("bea", "member", map[string]any{"fed": 0, "group_ids": []int{-1001}})); a.status != 400 ||
		a.body["error"] != "not_supported" {
		t.Fatalf("admin grants a group: %d %v", a.status, a.body)
	}
	beaID, _ := create(t, ann, account("bea", "member", defaultGrant))
	if a := ann.call("PATCH", fmt.Sprintf("/api/accounts/%d", beaID),
		map[string]any{"role": "admin"}); a.status != 403 {
		t.Fatalf("admin promotes a member: %d %v", a.status, a.body)
	}
	a := ann.call("PATCH", fmt.Sprintf("/api/accounts/%d", beaID),
		map[string]any{"name": "Bea B.", "grants": []any{defaultGrant}, "abilities": []string{}})
	if a.status != 200 || a.body["name"] != "Bea B." || a.body["role"] != "member" ||
		a.body["invited"] != true {
		t.Fatalf("admin renames a member: %d %v", a.status, a.body)
	}
	if a := ann.call("PATCH", fmt.Sprintf("/api/accounts/%d", beaID),
		map[string]any{"restrict": map[string]any{"hide_private": false,
			"hide_groups": []int{}, "join_chat": false}}); a.status != 403 {
		t.Fatalf("admin sets restrictions: %d %v", a.status, a.body)
	}

	// Taken names and addresses.
	if a := root.call("POST", "/api/accounts", account("max", "member")); a.status != 409 ||
		a.body["error"] != "taken" {
		t.Fatalf("taken username: %d %v", a.status, a.body)
	}
	dup := account("maxi", "member")
	dup["email"] = "max@example.org"
	if a := root.call("POST", "/api/accounts", dup); a.status != 409 || a.body["error"] != "email_taken" {
		t.Fatalf("taken address: %d %v", a.status, a.body)
	}
	for _, bad := range []map[string]any{
		{"username": "", "name": "", "email": "a@example.org", "role": "member"},
		{"username": "a b", "name": "", "email": "a@example.org", "role": "member"},
		{"username": "ab", "name": "", "email": "not an address", "role": "member"},
		{"username": "ab", "name": "", "email": "a@example.org", "role": "viewer"},
		{"username": "ab", "name": "x\x01", "email": "a@example.org", "role": "member"},
		{"username": "ab"},
	} {
		if a := root.call("POST", "/api/accounts", bad); a.status != 400 {
			t.Fatalf("bad create %v: %d %v", bad, a.status, a.body)
		}
	}

	// The admin disables the member: its sessions end and it cannot
	// sign in; enabled again, it can.
	if a := ann.call("POST", fmt.Sprintf("/api/accounts/%d/disable", memberID), nil); a.status != 204 {
		t.Fatalf("disable: %d %v", a.status, a.body)
	}
	if a := maxs.call("GET", "/api/session", nil); a.status != 401 {
		t.Fatalf("disabled member's session: %d", a.status)
	}
	if a := (&client{t: t, h: h, cookies: map[string]string{}}).login("max", "max-own-password"); a.status != 401 {
		t.Fatalf("disabled member signs in: %d", a.status)
	}
	if a := ann.call("POST", fmt.Sprintf("/api/accounts/%d/enable", memberID), nil); a.status != 204 {
		t.Fatalf("enable: %d %v", a.status, a.body)
	}
	maxs = newSession(t, h, "max", "max-own-password")

	// Every change is audited with who made it.
	row := root.call("GET", fmt.Sprintf("/api/accounts/%d", memberID), nil)
	var whats []string
	for _, e := range row.body["audit"].([]any) {
		m := e.(map[string]any)
		whats = append(whats, m["what"].(string)+" by "+m["by"].(string))
	}
	if strings.Join(whats, "|") != "account_enable by ann|account_disable by ann|"+
		"account_create: max role=member by root" || row.body["invited"] != false ||
		row.body["last_signin"] == nil || row.body["email"] != "max@example.org" {
		t.Fatalf("member row %v", row.body)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action LIKE 'account_%' " +
		"AND (web_user_id IS NULL OR target_user_id IS NULL)").Scan(&n); err != nil || n != 0 {
		t.Fatalf("account audit rows without actor or target: %d %v", n, err)
	}
	// No password is ever written to the audit log.
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE detail LIKE ? OR detail LIKE ?",
		"%"+adminTemp+"%", "%"+memberTemp+"%").Scan(&n); err != nil || n != 0 {
		t.Fatalf("a temporary password in the audit log: %d %v", n, err)
	}
}

// The last active superadmin cannot be disabled or demoted.
func TestLastSuperadmin(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("root-password")
	rootID := addUser(t, db, "root", hash, "superadmin", true)
	// A disabled superadmin does not count.
	addUser(t, db, "old", hash, "superadmin", false)
	h := newHandler(db, 50)
	root := newSession(t, h, "root", "root-password")
	self := fmt.Sprintf("/api/accounts/%d", rootID)

	if a := root.call("GET", self, nil); a.body["last_superadmin"] != true {
		t.Fatalf("row %v", a.body)
	}
	if a := root.call("POST", self+"/disable", nil); a.status != 409 || a.body["error"] != "last_superadmin" {
		t.Fatalf("disable the last superadmin: %d %v", a.status, a.body)
	}
	for _, role := range []string{"admin", "member"} {
		if a := root.call("PATCH", self, map[string]any{"role": role}); a.status != 409 {
			t.Fatalf("demote the last superadmin to %s: %d %v", role, a.status, a.body)
		}
	}
	// A rename is fine and keeps the session.
	if a := root.call("PATCH", self, map[string]any{"name": "Root"}); a.status != 200 {
		t.Fatalf("rename: %d %v", a.status, a.body)
	}

	// With a second one, either may go, but not both: two superadmins
	// disabling each other at once leave one. The one disabled first
	// loses its session, so its own request answers 401 when it comes
	// second, or 409 when it reached the last-superadmin check first.
	secondID, temp := create(t, root, account("sue", "superadmin"))
	sue := firstPassword(t, h, "sue", temp, "sue-own-password")
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, pair := range []struct {
		s  session
		id uint64
	}{{root, secondID}, {sue, rootID}} {
		wg.Add(1)
		go func(i int, s session, id uint64) {
			defer wg.Done()
			codes[i] = s.call("POST", fmt.Sprintf("/api/accounts/%d/disable", id), nil).status
		}(i, pair.s, pair.id)
	}
	wg.Wait()
	var active int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_users WHERE role = 'superadmin' " +
		"AND is_active = 1").Scan(&active); err != nil {
		t.Fatal(err)
	}
	ok := (codes[0] == 204) != (codes[1] == 204)
	for _, c := range codes {
		ok = ok && (c == 204 || c == 401 || c == 409)
	}
	if active != 1 || !ok {
		t.Fatalf("disabling each other: %v, %d active", codes, active)
	}
}

// Sign-out-everywhere, a role change and a password reset end every
// session of the account; a reset hands out a new temporary password.
func TestEndSessions(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("root-password")
	addUser(t, db, "root", hash, "superadmin", true)
	h := newHandler(db, 50)
	root := newSession(t, h, "root", "root-password")
	id, temp := create(t, root, account("eve", "admin"))
	path := fmt.Sprintf("/api/accounts/%d", id)

	eve := firstPassword(t, h, "eve", temp, "eve-own-password")
	if a := root.call("POST", path+"/sign-out-everywhere", nil); a.status != 204 {
		t.Fatalf("sign out everywhere: %d %v", a.status, a.body)
	}
	if a := eve.call("GET", "/api/session", nil); a.status != 401 {
		t.Fatalf("session after sign-out-everywhere: %d", a.status)
	}

	eve = newSession(t, h, "eve", "eve-own-password")
	if a := root.call("PATCH", path, map[string]any{"role": "member"}); a.status != 200 ||
		a.body["role"] != "member" {
		t.Fatalf("demote: %d %v", a.status, a.body)
	}
	if a := eve.call("GET", "/api/session", nil); a.status != 401 {
		t.Fatalf("session after a role change: %d", a.status)
	}

	eve = newSession(t, h, "eve", "eve-own-password")
	a := root.call("POST", path+"/password-reset", nil)
	reset, _ := a.body["temporary_password"].(string)
	if a.status != 200 || len(reset) != 20 || reset == temp {
		t.Fatalf("reset: %d %v", a.status, a.body)
	}
	if a := eve.call("GET", "/api/session", nil); a.status != 401 {
		t.Fatalf("session after a reset: %d", a.status)
	}
	if a := (&client{t: t, h: h, cookies: map[string]string{}}).login("eve", "eve-own-password"); a.status != 401 {
		t.Fatalf("old password after a reset: %d", a.status)
	}
	eve = newSession(t, h, "eve", reset)
	if a := eve.call("GET", "/api/session", nil); a.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("after a reset: %v", a.body)
	}
	var stored string
	if err := db.QueryRow("SELECT password_hash FROM web_users WHERE id = ?", id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$argon2id$") || strings.Contains(stored, reset) {
		t.Fatalf("stored %q", stored)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'account_password_reset' "+
		"AND target_user_id = ?", id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("reset audit rows: %d %v", n, err)
	}
}

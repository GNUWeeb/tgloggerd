// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/testdb"
)

// settingsEnv is a handler over the sample archive and the sample
// accounts, each a database of the test's own.
type settingsEnv struct {
	t       *testing.T
	ro, app *sql.DB
	h       http.Handler
}

func newSettingsEnv(t *testing.T) *settingsEnv {
	t.Helper()
	ro := testdb.New(t, testdb.Logger, "sample").DB
	app := testdb.New(t, testdb.Web, "accounts").DB
	signer := auth.NewSigner(testKey)
	h := New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      ro,
		App:     app,
		Signer:  signer,
		Cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		Limiter: auth.NewRateLimiter(50, time.Hour),
		Tokens:  auth.NewFileTokens(testKey),
	})
	return &settingsEnv{t: t, ro: ro, app: app, h: h}
}

// as signs a sample account in ("password" for all of them).
func (e *settingsEnv) as(name string) (*client, string) {
	c := &client{t: e.t, h: e.h, cookies: map[string]string{}}
	return c, signIn(e.t, c, name, "password")
}

const (
	gnuWeeb   = -1001000000001
	rustGroup = -1001000000003
)

func pinned(t *testing.T, c *client) []any {
	t.Helper()
	p, _ := viewerOf(t, c)["pinned"].([]any)
	return p
}

func TestPins(t *testing.T) {
	t.Parallel()
	e := newSettingsEnv(t)
	admin, tok := e.as("gw_admin")
	pin := func(c *client, tok, method string, id int64) int {
		return c.do(method, fmt.Sprintf("/api/account/pinned/%d", id), tok, nil).status
	}

	if st := pin(admin, "", "PUT", gnuWeeb); st != 403 {
		t.Fatalf("pin without CSRF: %d", st)
	}
	for _, id := range []int64{rustGroup, gnuWeeb, rustGroup} {
		if st := pin(admin, tok, "PUT", id); st != 204 {
			t.Fatalf("pin %d: %d", id, st)
		}
	}
	// No such group, or not a group id: 404.
	for _, id := range []int64{-1001999999999, 9299000001} {
		if st := pin(admin, tok, "PUT", id); st != 404 {
			t.Fatalf("pin %d: %d", id, st)
		}
	}
	if a := admin.do("PUT", "/api/account/pinned/abc", tok, nil); a.status != 404 {
		t.Fatalf("pin abc: %d", a.status)
	}
	// In the order pinned, once each.
	p := pinned(t, admin)
	if len(p) != 2 || p[0].(map[string]any)["title"] != "Rust Programming" ||
		p[1].(map[string]any)["id"] != float64(gnuWeeb) ||
		p[1].(map[string]any)["title"] != "GNU/Weeb" {
		t.Fatalf("pinned %v", p)
	}

	// A member sees no group yet: it cannot pin one, and a pin it holds
	// (made while it could see the group) is hidden, not shown.
	member, mtok := e.as("researcher")
	if st := pin(member, mtok, "PUT", gnuWeeb); st != 404 {
		t.Fatalf("member pins a hidden group: %d", st)
	}
	if _, err := e.app.Exec("INSERT INTO web_pinned_groups (web_user_id, group_id) "+
		"VALUES (5, ?)", gnuWeeb); err != nil {
		t.Fatal(err)
	}
	if p := pinned(t, member); len(p) != 0 {
		t.Fatalf("a hidden pin shows: %v", p)
	}
	if st := pin(member, mtok, "DELETE", gnuWeeb); st != 404 {
		t.Fatalf("member unpins a hidden group: %d", st)
	}

	if st := pin(admin, tok, "DELETE", rustGroup); st != 204 {
		t.Fatalf("unpin: %d", st)
	}
	if st := pin(admin, tok, "DELETE", rustGroup); st != 404 {
		t.Fatalf("unpin again: %d", st)
	}
	if p := pinned(t, admin); len(p) != 1 {
		t.Fatalf("after unpinning %v", p)
	}

	// The limit counts every pin, hidden ones too.
	vals := []string{}
	for i := range 49 {
		vals = append(vals, fmt.Sprintf("(3, %d)", -1002000000000-int64(i)))
	}
	if _, err := e.app.Exec("INSERT INTO web_pinned_groups (web_user_id, group_id) VALUES " +
		strings.Join(vals, ", ")); err != nil {
		t.Fatal(err)
	}
	a := admin.do("PUT", fmt.Sprintf("/api/account/pinned/%d", rustGroup), tok, nil)
	if a.status != 400 || a.body["error"] != "too_many" || !strings.Contains(a.body["message"].(string), "50") {
		t.Fatalf("pin over the limit: %d %v", a.status, a.body)
	}
	if st := pin(admin, tok, "PUT", gnuWeeb); st != 204 {
		t.Fatalf("pin of a pinned group at the limit: %d", st)
	}
}

func TestSavedSearches(t *testing.T) {
	t.Parallel()
	e := newSettingsEnv(t)
	c, tok := e.as("gw_admin")
	other, otok := e.as("sec_lead")
	save := func(c *client, tok string, body any) answer {
		return c.do("POST", "/api/account/saved-searches", tok, body)
	}
	good := map[string]any{"title": "  Deleted messages ",
		"url": "/group_messages?f=%5B%7B%22field%22%3A%22is_deleted%22%7D%5D"}
	if a := save(c, "", good); a.status != 403 {
		t.Fatalf("without CSRF: %d", a.status)
	}
	a := save(c, tok, good)
	id, _ := a.body["id"].(float64)
	if a.status != 201 || id == 0 {
		t.Fatalf("save: %d %v", a.status, a.body)
	}
	var title string
	if err := e.app.QueryRow("SELECT title FROM web_saved_searches WHERE id = ?", id).
		Scan(&title); err != nil || title != "Deleted messages" {
		t.Fatalf("stored title %q %v", title, err)
	}
	for _, tc := range []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"url": "/users"}, "title"},
		{map[string]any{"title": "   ", "url": "/users"}, "title"},
		{map[string]any{"title": strings.Repeat("x", 129), "url": "/users"}, "title"},
		{map[string]any{"title": "x"}, "url"},
		{map[string]any{"title": "x", "url": "https://evil.example/users"}, "url"},
		{map[string]any{"title": "x", "url": "//evil.example/users"}, "url"},
		{map[string]any{"title": "x", "url": "/\\evil.example"}, "url"},
		{map[string]any{"title": "x", "url": "users"}, "url"},
		{map[string]any{"title": "x", "url": "/" + strings.Repeat("a", 2048)}, "url"},
	} {
		if a := save(c, tok, tc.body); a.status != 400 || a.body["field"] != tc.field {
			t.Errorf("%v: %d %v", tc.body, a.status, a.body)
		}
	}
	if a := save(c, tok, map[string]any{"title": strings.Repeat("x", 128), "url": "/users"}); a.status != 201 {
		t.Fatalf("a 128-character title: %d %v", a.status, a.body)
	}

	del := func(c *client, tok string, id float64) int {
		return c.do("DELETE", fmt.Sprintf("/api/account/saved-searches/%d", int64(id)), tok, nil).status
	}
	if st := del(other, otok, id); st != 404 {
		t.Fatalf("another account deletes it: %d", st)
	}
	if st := del(c, tok, id); st != 204 {
		t.Fatalf("delete: %d", st)
	}
	if st := del(c, tok, id); st != 404 {
		t.Fatalf("delete again: %d", st)
	}

	vals := []string{}
	for i := range 99 {
		vals = append(vals, fmt.Sprintf("(4, 's%d', '/users')", i))
	}
	if _, err := e.app.Exec("INSERT INTO web_saved_searches (web_user_id, title, url) VALUES " +
		strings.Join(vals, ", ")); err != nil {
		t.Fatal(err)
	}
	if a := save(other, otok, good); a.status != 201 {
		t.Fatalf("the 100th: %d", a.status)
	}
	if a := save(other, otok, good); a.status != 400 || a.body["error"] != "too_many" {
		t.Fatalf("the 101st: %d %v", a.status, a.body)
	}
}

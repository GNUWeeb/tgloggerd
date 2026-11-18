// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/testdb"
)

// viewerOf is the viewer of the client's page data.
func viewerOf(t *testing.T, c *client) map[string]any {
	t.Helper()
	a := c.do("GET", "/api/page/login", "", nil)
	if a.status != 200 {
		t.Fatalf("page: %d %v", a.status, a.body)
	}
	return a.body["viewer"].(map[string]any)
}

func TestPreferences(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts").DB
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	tok := signIn(t, c, "gw_admin", "password")

	// Nothing set: the site's defaults.
	if a := c.do("GET", "/api/session", "", nil); a.body["landing"] != "/" {
		t.Fatalf("default landing %v", a.body)
	}
	v := viewerOf(t, c)
	if v["timezone"] != "Asia/Jakarta" || v["date_format"] != "24h" || v["lang"] != "en" ||
		v["theme"] != "system" || v["rows_per_page"] != 25.0 {
		t.Fatalf("default viewer %v", v)
	}

	prefs := map[string]any{
		"timezone": "Europe/Berlin", "date_format": "iso", "lang": "en", "theme": "dark",
		"rows_per_page": 50, "landing": "/groups", "chat_auto_load": false, "chat_live": false,
		"chat_media": "click", "chat_show_deleted": false, "chat_show_edited": false,
		"chat_show_service": false,
	}
	if a := c.do("PUT", "/api/account/preferences", "", prefs); a.status != 403 ||
		a.body["error"] != "csrf" {
		t.Fatalf("without CSRF: %d %v", a.status, a.body)
	}
	a := c.do("PUT", "/api/account/preferences", tok, prefs)
	if a.status != 200 {
		t.Fatalf("put: %d %v", a.status, a.body)
	}
	got := a.body["prefs"].(map[string]any)
	for k, want := range prefs {
		if w, ok := want.(int); ok {
			want = float64(w)
		}
		if got[k] != want {
			t.Errorf("answered %s = %v, want %v", k, got[k], want)
		}
	}
	// Where signing in goes: the landing page, in the session too.
	if a := c.do("GET", "/api/session", "", nil); a.body["landing"] != "/groups" {
		t.Fatalf("session landing %v", a.body)
	}
	if a := (&client{t: t, h: h, cookies: map[string]string{}}).login("gw_admin", "password"); a.body["landing"] != "/groups" {
		t.Fatalf("login landing %v", a.body)
	}
	v = viewerOf(t, c)
	chat := v["chat"].(map[string]any)
	if v["timezone"] != "Europe/Berlin" || v["date_format"] != "iso" || v["theme"] != "dark" ||
		v["rows_per_page"] != 50.0 || chat["live"] != false || chat["media"] != "click" ||
		chat["auto_load"] != false || chat["show_service"] != false {
		t.Fatalf("viewer after the change %v", v)
	}

	// Null and a missing key both go back to the site default; the
	// rest is replaced, not merged.
	a = c.do("PUT", "/api/account/preferences", tok, map[string]any{"timezone": nil, "theme": "light"})
	if a.status != 200 {
		t.Fatalf("reset: %d %v", a.status, a.body)
	}
	v = viewerOf(t, c)
	if v["timezone"] != "Asia/Jakarta" || v["theme"] != "light" || v["date_format"] != "24h" ||
		v["chat"].(map[string]any)["live"] != true {
		t.Fatalf("viewer after the reset %v", v)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_user_settings WHERE web_user_id = 3").
		Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows stored, want 1 (%v)", n, err)
	}

	// Every key refuses a value it does not take, and the body as a
	// whole: nothing changes.
	for _, tc := range []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"timezone": "Mars/Olympus"}, "timezone"},
		{map[string]any{"timezone": "Local"}, "timezone"},
		{map[string]any{"timezone": 7}, "timezone"},
		{map[string]any{"date_format": "24H"}, "date_format"},
		{map[string]any{"lang": "fr"}, "lang"},
		{map[string]any{"theme": "blue"}, "theme"},
		{map[string]any{"rows_per_page": 30}, "rows_per_page"},
		{map[string]any{"rows_per_page": "50"}, "rows_per_page"},
		{map[string]any{"landing": "https://evil.example/"}, "landing"},
		{map[string]any{"landing": "//evil.example"}, "landing"},
		{map[string]any{"chat_media": "never"}, "chat_media"},
		{map[string]any{"chat_live": "yes"}, "chat_live"},
		{map[string]any{"theme": "dark", "sidebar": "closed"}, "sidebar"},
		{map[string]any{"theme": "dark", "colour": "red"}, "colour"},
	} {
		a := c.do("PUT", "/api/account/preferences", tok, tc.body)
		if a.status != 400 || a.body["error"] != "bad_request" || a.body["field"] != tc.field ||
			!strings.Contains(a.body["message"].(string), tc.field) {
			t.Errorf("%v: %d %v", tc.body, a.status, a.body)
		}
	}
	if v := viewerOf(t, c); v["theme"] != "light" {
		t.Fatalf("a refused body changed the theme: %v", v)
	}
	if a := c.do("PUT", "/api/account/preferences", tok, []int{1}); a.status != 400 {
		t.Fatalf("not an object: %d", a.status)
	}

	// The document carries the language and the theme.
	r := httptest.NewRequest("GET", "/login", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: c.cookies[auth.SessionCookie]})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), `<html lang="en" data-theme="light">`) {
		t.Fatalf("document head: %.200s", body)
	}
}

// Two accounts with different time zones get the same UTC times with
// their own zone to show them in.
func TestPreferencesPerAccount(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts").DB
	h := newHandler(db, 5)
	berlin := &client{t: t, h: h, cookies: map[string]string{}}
	tokyo := &client{t: t, h: h, cookies: map[string]string{}}
	tb := signIn(t, berlin, "admin", "password")
	tt := signIn(t, tokyo, "researcher", "password")
	berlin.do("PUT", "/api/account/preferences", tb, map[string]any{"timezone": "Europe/Berlin"})
	tokyo.do("PUT", "/api/account/preferences", tt, map[string]any{"timezone": "Asia/Tokyo",
		"date_format": "12h"})
	vb, vt := viewerOf(t, berlin), viewerOf(t, tokyo)
	if vb["timezone"] != "Europe/Berlin" || vb["date_format"] != "24h" ||
		vt["timezone"] != "Asia/Tokyo" || vt["date_format"] != "12h" {
		t.Fatalf("berlin %v, tokyo %v", vb, vt)
	}
	// A signed-out visitor sees the site's.
	anon := &client{t: t, h: h, cookies: map[string]string{}}
	if v := viewerOf(t, anon); v["timezone"] != "Asia/Jakarta" || v["signed_in"] != false {
		t.Fatalf("signed-out viewer %v", v)
	}

	// The site's defaults are configuration (WEBAPI_DEFAULT_*); an
	// account's own choice still wins.
	signer := auth.NewSigner(testKey)
	h2 := New(Deps{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), RO: db, App: db, Signer: signer,
		Cookies:  &auth.Cookies{Signer: signer, Timeout: 3600},
		Limiter:  auth.NewRateLimiter(5, time.Hour),
		Defaults: settings.Defaults{Timezone: "UTC", Theme: "dark"},
	})
	anon.h, tokyo.h = h2, h2
	if v := viewerOf(t, anon); v["timezone"] != "UTC" || v["theme"] != "dark" ||
		v["date_format"] != "24h" {
		t.Fatalf("configured defaults %v", v)
	}
	if v := viewerOf(t, tokyo); v["timezone"] != "Asia/Tokyo" || v["theme"] != "dark" {
		t.Fatalf("own choice under configured defaults %v", v)
	}
}

// PATCH changes only the keys it names: the others keep what a PUT
// stored, null puts a key back to the site default, and the viewer
// carries the sidebar.
func TestPatchPreferences(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts").DB
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	tok := signIn(t, c, "gw_admin", "password")

	if v := viewerOf(t, c); v["sidebar"] != "open" {
		t.Fatalf("default sidebar %v", v["sidebar"])
	}
	if a := c.do("PUT", "/api/account/preferences", tok,
		map[string]any{"timezone": "UTC", "theme": "light", "landing": "/groups"}); a.status != 200 {
		t.Fatalf("put: %d %v", a.status, a.body)
	}
	if a := c.do("PATCH", "/api/account/preferences", "", map[string]any{"theme": "dark"}); a.status != 403 {
		t.Fatalf("without CSRF: %d %v", a.status, a.body)
	}
	a := c.do("PATCH", "/api/account/preferences", tok,
		map[string]any{"theme": "dark", "sidebar": "collapsed", "landing": nil})
	if a.status != 200 {
		t.Fatalf("patch: %d %v", a.status, a.body)
	}
	p := a.body["prefs"].(map[string]any)
	if p["theme"] != "dark" || p["sidebar"] != "collapsed" || p["timezone"] != "UTC" ||
		p["landing"] != nil || p["lang"] != nil {
		t.Fatalf("after patch %v", p)
	}
	if v := viewerOf(t, c); v["sidebar"] != "collapsed" || v["theme"] != "dark" || v["timezone"] != "UTC" {
		t.Fatalf("viewer after patch %v", v)
	}
	if a := c.do("GET", "/api/account/preferences", "", nil); a.body["prefs"].(map[string]any)["sidebar"] != "collapsed" {
		t.Fatalf("stored %v", a.body["prefs"])
	}

	// Refusals are PUT's, and leave everything as it was.
	for _, body := range []map[string]any{
		{"sidebar": "closed"}, {"sidebar": 1}, {"colour": "red"}, {"theme": "light", "rows_per_page": 7},
	} {
		if a := c.do("PATCH", "/api/account/preferences", tok, body); a.status != 400 || a.body["error"] != "bad_request" {
			t.Errorf("%v: %d %v", body, a.status, a.body)
		}
	}
	// An empty body changes nothing.
	a = c.do("PATCH", "/api/account/preferences", tok, map[string]any{})
	if p := a.body["prefs"].(map[string]any); a.status != 200 || p["theme"] != "dark" || p["sidebar"] != "collapsed" {
		t.Fatalf("empty patch: %d %v", a.status, a.body)
	}
}

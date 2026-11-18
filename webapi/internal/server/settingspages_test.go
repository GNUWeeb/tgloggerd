// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"

	"tgloggerd/webapi/internal/i18n"
)

func TestSettingsPage(t *testing.T) {
	t.Parallel()
	e := newSettingsEnv(t)
	if _, err := e.app.Exec("UPDATE web_users SET display_name = 'Gita', " +
		"email = 'gita@example.org' WHERE id = 3"); err != nil {
		t.Fatal(err)
	}
	c, _ := e.as("gw_admin")
	a := c.do("GET", "/api/page/settings", "", nil)
	if a.status != 200 || a.body["route"] != "settings" {
		t.Fatalf("page: %d %v", a.status, a.body)
	}
	p := a.body["props"].(map[string]any)
	acct := p["account"].(map[string]any)
	if acct["username"] != "gw_admin" || acct["name"] != "Gita" || acct["role"] != "admin" ||
		acct["email"] != "gita@example.org" || acct["email_verified"] != false {
		t.Fatalf("account %v", acct)
	}
	if n := p["notices"].([]any); len(n) != len(noticeKeys) || !strings.Contains(n[0].(string), "password") {
		t.Fatalf("notices %v", n)
	}
	// No display name and no address: the login name, and null.
	m, _ := e.as("researcher")
	acct = m.do("GET", "/api/page/settings", "", nil).body["props"].(map[string]any)["account"].(map[string]any)
	if acct["name"] != "researcher" || acct["email"] != nil {
		t.Fatalf("member account %v", acct)
	}
}

func TestPreferencesPage(t *testing.T) {
	t.Parallel()
	e := newSettingsEnv(t)
	c, tok := e.as("gw_admin")
	c.do("PUT", "/api/account/pinned/-1001000000001", tok, nil)
	c.do("PUT", "/api/account/pinned/-1001000000003", tok, nil)
	c.do("POST", "/api/account/saved-searches", tok, map[string]any{"title": "Premium", "url": "/users?f=x"})
	c.do("PUT", "/api/account/preferences", tok, map[string]any{"timezone": "America/Argentina/Salta",
		"theme": "dark"})

	page := c.do("GET", "/api/page/settings/preferences", "", nil)
	api := c.do("GET", "/api/account/preferences", "", nil)
	if page.status != 200 || page.body["route"] != "settings_preferences" || api.status != 200 {
		t.Fatalf("page %d %v, api %d", page.status, page.body, api.status)
	}
	p := page.body["props"].(map[string]any)
	if len(api.body) != len(p) || len(api.body["pinned"].([]any)) != 2 {
		t.Fatalf("the API answers other data: %v", api.body)
	}
	prefs := p["prefs"].(map[string]any)
	if prefs["timezone"] != "America/Argentina/Salta" || prefs["theme"] != "dark" ||
		prefs["lang"] != nil {
		t.Fatalf("prefs %v", prefs)
	}
	tz := p["timezones"].([]any)
	if tz[0] != "Asia/Jakarta" || tz[len(tz)-1] != "America/Argentina/Salta" {
		t.Fatalf("timezones %v", tz)
	}
	pins := p["pinned"].([]any)
	gw := pins[0].(map[string]any)
	if gw["title"] != "GNU/Weeb" || !strings.HasPrefix(gw["photo"].(string), "/files/") ||
		pins[1].(map[string]any)["photo"] != nil {
		t.Fatalf("pinned %v", pins)
	}
	ss := p["saved_searches"].([]any)
	if len(ss) != 1 || ss[0].(map[string]any)["url"] != "/users?f=x" {
		t.Fatalf("saved %v", ss)
	}
	// The landing options: the admin's pages, then its pinned chats.
	opts := map[string]string{}
	for _, o := range p["landing_options"].([]any) {
		pair := o.([]any)
		opts[pair[0].(string)] = pair[1].(string)
	}
	if opts["/"] != "Dashboard" || opts["/groups/-1001000000001/chat"] != "GNU/Weeb chat" ||
		opts["/settings/accounts"] != "Accounts" {
		t.Fatalf("landing options %v", opts)
	}
	if _, ok := opts["/join-chat"]; ok {
		t.Fatal("an admin may not join chats, yet it is offered")
	}

	// A member: none of its pins show, and only its pages are offered.
	m, _ := e.as("researcher")
	if _, err := e.app.Exec("INSERT INTO web_pinned_groups (web_user_id, group_id) " +
		"VALUES (5, -1001000000001)"); err != nil {
		t.Fatal(err)
	}
	mp := m.do("GET", "/api/page/settings/preferences", "", nil).body["props"].(map[string]any)
	if len(mp["pinned"].([]any)) != 0 || len(mp["saved_searches"].([]any)) != 0 {
		t.Fatalf("member props %v", mp)
	}
	for _, o := range mp["landing_options"].([]any) {
		if path := o.([]any)[0]; path == "/private_messages" || path == "/settings/accounts" {
			t.Fatalf("member offered %v", path)
		}
	}
}

// Every landing page and notice has an English text.
func TestSettingsTexts(t *testing.T) {
	t.Parallel()
	for _, lp := range landingPages {
		if got := i18n.T("en", "page."+lp.key); got == "page."+lp.key {
			t.Errorf("no text for page.%s", lp.key)
		}
	}
	for _, k := range noticeKeys {
		if i18n.T("en", k) == k {
			t.Errorf("no text for %s", k)
		}
	}
}

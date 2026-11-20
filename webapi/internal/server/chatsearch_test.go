// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/url"
	"strings"
	"testing"
)

// The chat search of the pickers, on the search tests' fixtures
// (searchEnv).

func pickTitles(b map[string]any) string {
	var ts []string
	for _, it := range b["items"].([]any) {
		ts = append(ts, it.(map[string]any)["title"].(string))
	}
	return strings.Join(ts, "|")
}

func TestSearchChats(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	// Busiest first across groups and private chats, followed by
	// the cursor to the end.
	var got []string
	path := "/api/chats/search?limit=3"
	for pages := 0; path != ""; pages++ {
		b := e.ok("superadmin", path)
		got = append(got, pickTitles(b))
		path = ""
		if c, ok := b["next_cursor"].(string); ok {
			path = "/api/chats/search?limit=3&cursor=" + url.QueryEscape(c)
		}
		if pages > 5 {
			t.Fatal("the cursor does not end")
		}
	}
	if s := strings.Join(got, " / "); s != "GNU/Weeb|Dewi Kusuma|Linux Kernel Indonesia / "+
		"Rust Programming|Assembly Lounge|Bima Pratama / C/C++ Study Group|CTF Practice" {
		t.Errorf("pages: %s", s)
	}
	b := e.ok("admin", "/api/chats/search?limit=2")
	if js(b["items"].([]any)[0]) != js(map[string]any{"id": -1001000000001, "kind": "group",
		"photo": e.file(880008), "sub": "supergroup", "title": "GNU/Weeb"}) ||
		js(b["items"].([]any)[1]) != js(map[string]any{"id": 9299000001, "kind": "user",
			"photo": e.file(880007), "sub": "@dewikusuma", "title": "Dewi Kusuma"}) {
		t.Errorf("items %s", js(b["items"]))
	}
	for q, want := range map[string]string{
		"q=rust":                   "Rust Programming",
		"q=%40gnuweeb":             "GNU/Weeb",
		"q=bima":                   "Bima Pratama",
		"q=-1001000000010":         "CTF Practice",
		"q=kernel&kind=user":       "Dewi Kusuma",
		"kind=user":                "Dewi Kusuma|Bima Pratama",
		"q=a&kind=group&limit=200": "Linux Kernel Indonesia|Rust Programming|Assembly Lounge|CTF Practice",
		"federation=0&q=e":         "GNU/Weeb|Linux Kernel Indonesia|Assembly Lounge|CTF Practice",
		"federation=7":             "",
		"q=nothing-like-it":        "",
	} {
		if b := e.ok("superadmin", "/api/chats/search?"+q); pickTitles(b) != want || b["next_cursor"] != nil {
			t.Errorf("%s: %q %v, want %q", q, pickTitles(b), b["next_cursor"], want)
		}
	}
	// A member sees no group yet and never a private chat.
	for _, q := range []string{"", "?kind=user", "?q=dewi"} {
		if b := e.ok("member", "/api/chats/search"+q); len(b["items"].([]any)) != 0 {
			t.Errorf("member %s: %v", q, b["items"])
		}
	}
	for _, q := range []string{"kind=channel", "limit=0", "limit=201", "limit=x",
		"cursor=nope", "federation=-1"} {
		if st, b := e.get("superadmin", "/api/chats/search?"+q); st != 400 || b["error"] != "bad_request" {
			t.Errorf("%s: %d %v", q, st, b)
		}
	}
	if st, _ := e.get("", "/api/chats/search"); st != 401 {
		t.Errorf("signed out: %d", st)
	}
}

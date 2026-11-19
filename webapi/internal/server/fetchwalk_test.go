// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Every media object a chat window or a message page nests (ChatMedia,
// MessageFull.media) links a big unstored file the same way: url null,
// fetchable true and fetch the "Fetch from Telegram" action's path
// (web/docs/media.md). The test makes every file of the fixtures big
// and unstored, walks the chat windows and message pages, and checks
// each media object it finds, by where it sits.

// walkMedia calls visit for every object under v that is a media
// object (it has stored and url), with its path: the keys from the
// root, without the array indices.
func walkMedia(v any, path string, visit func(path string, m map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		_, s := x["stored"]
		_, u := x["url"]
		if s && u {
			visit(path, x)
		}
		for k, c := range x {
			walkMedia(c, path+"."+k, visit)
		}
	case []any:
		for _, c := range x {
			walkMedia(c, path, visit)
		}
	}
}

// fetchWalk walks the chat windows and message pages of every chat of
// the fixtures as a superadmin, checks each media object, and returns
// the paths where a fetchable one was found.
func fetchWalk(t *testing.T, fixtures ...string) map[string]bool {
	t.Helper()
	e := newAdminEnv(t, fixtures...)
	e.deps.Media.StoreFileCap = 1000
	e.h = New(e.deps)
	if _, err := e.ro.Exec("UPDATE telegram_files SET on_disk = 0, alias_of = NULL, " +
		"file_size = 1 << 31"); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	check := func(where, path string, m map[string]any) {
		fetch, _ := m["fetch"].(string)
		switch {
		case m["stored"] != false:
			t.Errorf("%s %s: stored: %v", where, path, m)
		case m["fetchable"] == true:
			if m["url"] != nil || !strings.HasPrefix(fetch, "/api/files/") ||
				!strings.HasSuffix(fetch, "/fetch") {
				t.Errorf("%s %s: big fetchable file: %v", where, path, m)
			}
			seen[path] = true
		case m["fetch"] != nil || m["url"] != nil:
			// Not fetchable: a reference to a file row that is gone.
			t.Errorf("%s %s: not fetchable but linked: %v", where, path, m)
		}
	}
	for _, c := range []struct{ scope, q string }{
		{"group", "SELECT DISTINCT chat_id FROM telegram_group_messages"},
		{"private", "SELECT DISTINCT chat_id FROM telegram_private_messages"},
	} {
		var chats []int64
		rs, err := e.ro.Query(c.q)
		if err != nil {
			t.Fatal(err)
		}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				t.Fatal(err)
			}
			chats = append(chats, id)
		}
		rs.Close()
		for _, chat := range chats {
			before := ""
			for {
				st, b, _ := e.call("superadmin", "GET", fmt.Sprintf("/api/chats/%s/%d/messages?limit=100%s",
					c.scope, chat, before), nil)
				if st != 200 {
					// A chat the fixtures log no peer of, or the like.
					break
				}
				msgs := asList(b["messages"])
				walkMedia(b, "chat", func(p string, m map[string]any) { check(fmt.Sprint(chat), p, m) })
				for _, x := range msgs {
					id := num(x.(map[string]any)["id"])
					st, p := e.page("superadmin", fmt.Sprintf("/messages/%s/%d", c.scope, id))
					if st == 200 {
						walkMedia(p["props"], "page", func(p string, m map[string]any) {
							check(fmt.Sprintf("%s message %d", c.scope, id), p, m)
						})
					}
				}
				if b["has_older"] != true || len(msgs) == 0 {
					break
				}
				before = fmt.Sprintf("&before=%d", num(msgs[0].(map[string]any)["id"]))
			}
		}
	}
	return seen
}

// TestFetchEverywhere walks the fixtures that hold each kind of nested
// media, and fails unless a big fetchable file was found, and linked
// through fetch only, in every place the pages nest one.
func TestFetchEverywhere(t *testing.T) {
	t.Parallel()
	all := map[string]bool{}
	for _, f := range [][]string{{"chats"}, {"chats", "link_preview_media"}, {"parity_chat"},
		{"service_detail"}, {"gifts"}, {"rich"}, {"kinds"}, {"video_stills"}} {
		for p := range fetchWalk(t, f...) {
			all[p] = true
		}
	}
	for _, p := range []string{
		"messages.media",                            // the message's own
		"messages.album",                            // album items
		"messages.edits.media",                      // edit snapshots
		"messages.link_preview.media",               // link previews
		"messages.detail.blocks.media",              // rich blocks
		"messages.detail.blocks.items.blocks.media", // nested rich blocks
		"messages.detail.photo",                     // game, invoice
		"messages.detail.items.media",               // paid media
		"messages.detail.sticker",                   // giveaway
		"messages.service.detail.photo",             // chat photo changes
		"messages.service.detail.animation",         // animated chat photos
		"messages.service.detail.image",             // chat backgrounds
		"messages.service.detail.sticker",           // gifts
		"messages.service.detail.symbol_sticker",    // upgraded gifts' symbols
	} {
		for _, where := range []string{"chat." + p, "page." + strings.Replace(p, "messages", "message", 1)} {
			if !all[where] {
				t.Errorf("no big fetchable file found at %s", where)
			}
		}
	}
	if t.Failed() {
		var ps []string
		for p := range all {
			ps = append(ps, p)
		}
		sort.Strings(ps)
		t.Logf("found at:\n%s", strings.Join(ps, "\n"))
	}
}

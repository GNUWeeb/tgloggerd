// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"testing"
)

// The link preview card's type, extras, own medium and target, on the
// "link_preview_media" fixture's previews in Open Lab.

// previewCards are the link previews of Open Lab's window, by row, as
// cookie sees them.
func previewCards(t *testing.T, e *chatEnv, cookie string) map[int64]map[string]any {
	t.Helper()
	var w struct {
		Messages []map[string]any `json:"messages"`
	}
	e.decode(fmt.Sprintf("/api/chats/group/%d/messages", openLab), cookie, 200, &w)
	out := map[int64]map[string]any{}
	for _, m := range w.Messages {
		if p, ok := m["link_preview"].(map[string]any); ok {
			out[num(m["id"])] = p
		}
	}
	return out
}

func TestLinkPreviewMedia(t *testing.T) {
	t.Parallel()
	e := newChatEnvFor(t, true, nil, "sample", "chats", "link_preview_media")
	ps := previewCards(t, e, e.as("admin"))
	photo := func(id uint64) string { return "/files/" + e.tokens.Encrypt(id) }

	// A video preview: its medium plays, the photo is its poster, and
	// no minithumbnail is sent.
	p := ps[2101]
	md, _ := p["media"].(map[string]any)
	if p["type"] != "video" || num(p["duration"]) != 42 || md == nil ||
		md["render"] != "video" || md["url"] != photo(871002) || md["stored"] != true ||
		md["poster"] != photo(871001) || md["thumb"] != photo(871001) ||
		num(md["w"]) != 1280 || num(md["h"]) != 720 || num(md["duration"]) != 42 ||
		md["mime"] != "video/mp4" || md["minithumb"] != nil {
		t.Errorf("video preview %s", js(p))
	}
	if _, ok := p["embed"]; ok {
		t.Errorf("video preview has an embed: %s", js(p))
	}

	// An embedded player: its URL and size, the player's duration, and
	// the cached medium's own size and length, fetchable.
	p = ps[2102]
	md, _ = p["media"].(map[string]any)
	em, _ := p["embed"].(map[string]any)
	if p["type"] != "embedded_video_player" || p["author"] != "Some Channel" ||
		num(p["duration"]) != 213 || em == nil || em["url"] != "https://tube.example.org/embed/x" ||
		num(em["w"]) != 1280 || num(em["h"]) != 720 || md == nil || md["render"] != "video" ||
		md["stored"] != false || md["fetchable"] != true || num(md["w"]) != 640 ||
		num(md["duration"]) != 30 || md["poster"] != photo(871003) || md["minithumb"] != nil {
		t.Errorf("embedded player %s", js(p))
	}

	// An embed with neither medium nor photo: no media, its embed.
	p = ps[2103]
	em, _ = p["embed"].(map[string]any)
	if _, ok := p["media"]; ok || em == nil || em["url"] != "https://sound.example.org/player/t" ||
		em["w"] != nil || num(p["duration"]) != 180 {
		t.Errorf("embedded audio player %s", js(p))
	}

	// Audio: the title is the track's (audio_title), the photo its
	// thumb but not a poster.
	p = ps[2104]
	md, _ = p["media"].(map[string]any)
	if md == nil || md["render"] != "audio" || md["title"] != "Song" || md["performer"] != "Band" ||
		md["thumb"] != photo(871006) || md["poster"] != nil || num(md["duration"]) != 200 ||
		p["title"] != "song.mp3" {
		t.Errorf("audio preview %s", js(p))
	}

	// An article: the photo, with its size and no minithumb.
	p = ps[2109]
	md, _ = p["media"].(map[string]any)
	if p["type"] != "article" || md == nil || md["render"] != "image" || md["url"] != photo(871001) ||
		num(md["w"]) != 800 || num(md["h"]) != 418 || md["minithumb"] != nil {
		t.Errorf("article %s", js(p))
	}
}

func TestLinkPreviewTarget(t *testing.T) {
	t.Parallel()
	e := newChatEnvFor(t, true, nil, "sample", "chats", "link_preview_media")
	target := func(p map[string]any) string {
		if v, ok := p["target"]; ok {
			return js(v)
		}
		return "none"
	}
	admin := previewCards(t, e, e.as("admin"))
	for row, want := range map[int64]string{
		2105: fmt.Sprintf(`{"chat_id":%d,"message_id":%d}`, pagingLab, int64(700005)<<20),
		2106: fmt.Sprintf(`{"chat_id":%d,"message_id":%d}`, openLab, int64(800001)<<20),
		2107: "none", // not in the archive
		2108: "none", // a released username
		2101: "none", // not a message preview
	} {
		if got := target(admin[row]); got != want {
			t.Errorf("admin, row %d: target %s, want %s", row, got, want)
		}
	}
	// A visitor of the exposed Open Lab may not see Paging Lab: the
	// card links to its URL alone.
	public := previewCards(t, e, "")
	if got := target(public[2105]); got != "none" {
		t.Errorf("public, a hidden group's message: target %s", got)
	}
	if got, want := target(public[2106]), target(admin[2106]); got != want {
		t.Errorf("public, Open Lab's message: target %s, want %s", got, want)
	}
}

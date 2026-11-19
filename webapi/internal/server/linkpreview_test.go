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

// The types named since unsupported, and the media kept since for the
// older ones, on the "link_preview_types" fixture's previews.
func TestLinkPreviewTypes(t *testing.T) {
	t.Parallel()
	e := newChatEnvFor(t, true, nil, "sample", "chats", "link_preview_types")
	ps := previewCards(t, e, e.as("admin"))
	file := func(id uint64) string { return "/files/" + e.tokens.Encrypt(id) }
	media := func(p map[string]any) map[string]any {
		md, _ := p["media"].(map[string]any)
		return md
	}
	list := func(v any) []any {
		l, _ := v.([]any)
		return l
	}
	obj := func(v any) map[string]any {
		o, _ := v.(map[string]any)
		return o
	}

	// A video's kept cover is its poster; the photo stays its thumb.
	md := media(ps[2111])
	if md == nil || md["render"] != "video" || md["url"] != file(872002) ||
		md["poster"] != file(872003) || md["thumb"] != file(872001) {
		t.Errorf("video with a cover %s", js(ps[2111]))
	}

	// A document's own file, its thumbnail the photo.
	md = media(ps[2112])
	if md == nil || md["render"] != "file" || md["url"] != file(872004) ||
		md["name"] != "paper.pdf" || md["mime"] != "application/pdf" || md["thumb"] != file(872001) {
		t.Errorf("document %s", js(ps[2112]))
	}

	// A sticker, with its emoji, format and size.
	md = media(ps[2113])
	if md == nil || md["render"] != "image" || md["url"] != file(872005) || md["emoji"] != "x" ||
		md["format"] != "webp" || num(md["w"]) != 512 {
		t.Errorf("sticker %s", js(ps[2113]))
	}

	// A video note plays round, sized by its diameter, the photo its
	// poster; no minithumb.
	md = media(ps[2114])
	if md == nil || md["render"] != "video" || md["round"] != true || num(md["w"]) != 384 ||
		num(md["h"]) != 384 || md["poster"] != file(872006) || md["minithumb"] != nil ||
		num(ps[2114]["duration"]) != 9 {
		t.Errorf("video note %s", js(ps[2114]))
	}

	// A voice note, with its waveform.
	md = media(ps[2115])
	if md == nil || md["render"] != "audio" || md["url"] != file(872008) ||
		len(list(md["wave"])) == 0 || md["mime"] != "audio/ogg" {
		t.Errorf("voice note %s", js(ps[2115]))
	}

	// An album: its items by position, an empty one null, and no
	// medium of the card's own.
	p := ps[2116]
	al := obj(p["album"])
	items := list(al["items"])
	if _, ok := p["media"]; ok || al == nil || al["caption"] != "Two of them" || len(items) != 3 ||
		obj(items[0])["render"] != "image" || obj(items[0])["url"] != file(872009) ||
		num(obj(items[0])["w"]) != 800 || obj(items[1])["render"] != "video" ||
		obj(items[1])["url"] != file(872010) || num(obj(items[1])["duration"]) != 3 ||
		items[2] != nil {
		t.Errorf("album %s", js(p))
	}

	// A background: the pattern's image, its fill to 24 bits, the
	// photo as the card's image.
	p = ps[2117]
	bg := obj(p["background"])
	if bg == nil || bg["type"] != "pattern" || obj(bg["image"])["url"] != file(872011) ||
		obj(bg["image"])["render"] != "image" || num(bg["intensity"]) != 50 ||
		bg["is_moving"] != true || bg["is_blurred"] != false || bg["theme_name"] != nil ||
		js(bg["fill"]) != `{"colors":[255,255],"rotation_angle":45,"type":"gradient"}` ||
		media(p)["url"] != file(872012) || bg["dark"] != nil || bg["name"] != nil {
		t.Errorf("background %s", js(p))
	}

	// A sticker set's stickers by position, a missing one null.
	st := list(ps[2118]["stickers"])
	if len(st) != 3 || obj(st[0])["render"] != "lottie" || obj(st[0])["emoji"] != "a" ||
		st[1] != nil || obj(st[2])["render"] != "animation" || obj(st[2])["url"] != file(872014) {
		t.Errorf("sticker set %s", js(ps[2118]))
	}
	if st := list(ps[2119]["stickers"]); len(st) != 1 || obj(st[0])["url"] != file(872015) {
		t.Errorf("gift collection %s", js(ps[2119]))
	}

	// A gift auction's gift and dates.
	g := obj(ps[2120]["gift"])
	au := obj(g["auction"])
	if g == nil || obj(g["sticker"])["url"] != file(872016) || obj(g["sticker"])["emoji"] != "h" ||
		num(g["star_count"]) != 500 || g["upgraded"] != nil || g["symbol_sticker"] != nil ||
		au == nil || au["start_date"] != iso(1794000000) || au["end_date"] != iso(1795000000) ||
		num(au["gifts_per_round"]) != 10 {
		t.Errorf("gift auction %s", js(ps[2120]))
	}

	// An upgraded gift: model and symbol stickers, its owner as a ref.
	g = obj(ps[2121]["gift"])
	up := obj(g["upgraded"])
	if g == nil || obj(g["sticker"])["url"] != file(872017) ||
		obj(g["symbol_sticker"])["url"] != file(872018) || g["auction"] != nil ||
		up == nil || up["name"] != "LabGift-7" || num(up["number"]) != 7 ||
		num(obj(up["owner"])["id"]) != 9299000001 || obj(up["model"])["name"] != "Round" ||
		obj(up["backdrop"])["rarity"] != "rare" || up["symbol"] != nil {
		t.Errorf("upgraded gift %s", js(ps[2121]))
	}

	// A live story's poster and its username.
	s := obj(ps[2122]["story"])
	if s == nil || num(s["story_id"]) != 7 || num(obj(s["poster"])["id"]) != 9299000001 ||
		s["username"] != "dewikusuma" {
		t.Errorf("live story %s", js(ps[2122]))
	}

	// A theme's settings: accent 0 is black, not absent.
	th := obj(ps[2123]["theme"])
	tb := obj(th["background"])
	if th == nil || th["base_theme"] != "night" || th["accent_color"] != float64(0) ||
		num(th["outgoing_message_accent_color"]) != 16711935 ||
		th["animate_outgoing_message_fill"] != false ||
		obj(th["outgoing_message_fill"])["type"] != "gradient" ||
		tb == nil || tb["name"] != "labbg" || tb["dark"] != true || tb["type"] != "fill" ||
		tb["image"] != nil || obj(tb["fill"])["type"] != "solid" {
		t.Errorf("theme %s", js(ps[2123]))
	}

	// A text composition style's custom emoji, resolved.
	p = ps[2124]
	ce := obj(p["custom_emoji"])
	if p["custom_emoji_id"] != "5000000000000000091" || ce == nil || ce["format"] != "webp" ||
		ce["url"] != file(872019) {
		t.Errorf("text composition style %s", js(p))
	}

	// An unsupported row is as before: its photo, nothing else.
	p = ps[2125]
	if p["type"] != "unsupported" || media(p)["url"] != file(872001) || media(p)["render"] != "image" {
		t.Errorf("unsupported %s", js(p))
	}
	for _, k := range []string{"album", "background", "stickers", "gift", "story", "theme",
		"custom_emoji_id", "custom_emoji", "embed", "target", "duration"} {
		if _, ok := p[k]; ok {
			t.Errorf("unsupported has %s: %s", k, js(p))
		}
	}

	// A story album: the video icon plays, the photo icon its poster.
	md = media(ps[2126])
	if md == nil || md["render"] != "video" || md["url"] != file(872021) || num(md["w"]) != 240 ||
		md["poster"] != file(872020) {
		t.Errorf("story album %s", js(ps[2126]))
	}

	// A direct messages chat: its photo.
	if p := ps[2127]; p["type"] != "direct_messages_chat" || media(p)["url"] != file(872001) {
		t.Errorf("direct messages chat %s", js(p))
	}
}

// A visitor of the exposed Open Lab sees no story poster or gift owner
// it may not see.
func TestLinkPreviewTypesPublic(t *testing.T) {
	t.Parallel()
	e := newChatEnvFor(t, true, nil, "sample", "chats", "link_preview_types")
	admin := previewCards(t, e, e.as("admin"))
	public := previewCards(t, e, "")
	for _, row := range []int64{2122, 2121} {
		if public[row] == nil {
			t.Fatalf("public: no card on row %d", row)
		}
	}
	s, _ := public[2122]["story"].(map[string]any)
	as, _ := admin[2122]["story"].(map[string]any)
	if s == nil || (s["poster"] == nil) != (s["username"] == nil) {
		t.Errorf("public live story %s", js(public[2122]))
	}
	if as["poster"] == nil {
		t.Errorf("admin live story without a poster %s", js(admin[2122]))
	}
}

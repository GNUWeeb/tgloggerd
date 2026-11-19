// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/logdb"
)

// A message's link preview card (LinkPreview): the card's columns, its
// type's own fields from extra (docs/design/message-types.md section
// 4.8) that the web shows, its media (the message's preview_media
// files by position, section 4.3), and the archived message a message
// preview points at. The people a story or an upgraded gift names and
// a text composition style's custom emoji are loaded with the page's
// kinds (loadKinds), and go through the request scope as the refs do.

type linkPreviewJSON struct {
	Type       string                 `json:"type"`
	Site       string                 `json:"site"`
	Title      string                 `json:"title"`
	Desc       string                 `json:"desc"`
	URL        string                 `json:"url"`
	Author     string                 `json:"author,omitempty"`
	Media      *chatMediaJSON         `json:"media,omitempty"`
	Duration   int                    `json:"duration,omitempty"`
	Embed      *previewEmbedJSON      `json:"embed,omitempty"`
	Target     *previewTargetJSON     `json:"target,omitempty"`
	Album      *previewAlbumJSON      `json:"album,omitempty"`
	Background *previewBackgroundJSON `json:"background,omitempty"`
	// Stickers holds nil for a sticker whose file is not recorded.
	Stickers      []*chatMediaJSON  `json:"stickers,omitempty"`
	Gift          *previewGiftJSON  `json:"gift,omitempty"`
	Story         *previewStoryJSON `json:"story,omitempty"`
	Theme         *previewThemeJSON `json:"theme,omitempty"`
	CustomEmojiID string            `json:"custom_emoji_id,omitempty"`
	// CustomEmoji is set (maybe to null) with CustomEmojiID only.
	CustomEmoji json.RawMessage `json:"custom_emoji,omitempty"`
}

type (
	previewEmbedJSON struct {
		URL string `json:"url"`
		W   int    `json:"w,omitempty"`
		H   int    `json:"h,omitempty"`
	}
	previewTargetJSON struct {
		ChatID    int64 `json:"chat_id"`
		MessageID int64 `json:"message_id"`
	}
	previewAlbumJSON struct {
		Caption string           `json:"caption"`
		Items   []*chatMediaJSON `json:"items"`
	}
	previewBackgroundJSON struct {
		Image *chatMediaJSON `json:"image"`
		backgroundLookJSON
		Name string `json:"name,omitempty"`
		Dark *bool  `json:"dark,omitempty"`
	}
	previewGiftJSON struct {
		Sticker       *chatMediaJSON      `json:"sticker"`
		SymbolSticker *chatMediaJSON      `json:"symbol_sticker"`
		StarCount     *int64              `json:"star_count"`
		Upgraded      *upgradedGiftJSON   `json:"upgraded"`
		Auction       *previewAuctionJSON `json:"auction"`
	}
	previewAuctionJSON struct {
		StartDate     *string `json:"start_date"`
		EndDate       *string `json:"end_date"`
		GiftsPerRound *int64  `json:"gifts_per_round"`
	}
	previewStoryJSON struct {
		Poster   *chatRefJSON `json:"poster"`
		StoryID  int64        `json:"story_id"`
		Username *string      `json:"username"`
	}
	previewThemeJSON struct {
		BaseTheme      *string                `json:"base_theme"`
		AccentColor    int64                  `json:"accent_color"`
		Background     *previewBackgroundJSON `json:"background"`
		OutFill        *backgroundFill        `json:"outgoing_message_fill"`
		AnimateOutFill bool                   `json:"animate_outgoing_message_fill"`
		OutAccent      *int64                 `json:"outgoing_message_accent_color"`
	}
)

// previewExtra is the part of a preview's extra the card reads; the
// keys of the other types are zero.
type previewExtra struct {
	Duration    int    `json:"duration"`
	EmbedURL    string `json:"embed_url"`
	EmbedWidth  int    `json:"embed_width"`
	EmbedHeight int    `json:"embed_height"`
	// album
	Caption string            `json:"caption"`
	Items   []json.RawMessage `json:"items"`
	// sticker_set, gift_collection
	Stickers []json.RawMessage `json:"stickers"`
	Icons    []json.RawMessage `json:"icons"`
	// background: its look's keys, its type under background_type
	BackgroundType string `json:"background_type"`
	backgroundExtra
	// gift_auction, upgraded_gift
	Gift          *storedGift `json:"gift"`
	AuctionStart  int64       `json:"auction_start_date"`
	AuctionEnd    int64       `json:"auction_end_date"`
	GiftsPerRound int64       `json:"gifts_per_round"`
	// story, live_story
	PosterChatID int64 `json:"poster_chat_id"`
	StoryID      int64 `json:"story_id"`
	// text_composition_style
	CustomEmojiID looseID `json:"custom_emoji_id"`
	// theme
	BaseTheme       string `json:"base_theme"`
	AccentColor     int64  `json:"accent_color"`
	ThemeBackground *struct {
		Name string `json:"name"`
		backgroundExtra
	} `json:"background"`
	OutFill        *backgroundFill `json:"outgoing_message_fill"`
	AnimateOutFill bool            `json:"animate_outgoing_message_fill"`
	OutAccent      *int64          `json:"outgoing_message_accent_color"`
}

// parsePreviewExtra reads a preview's extra; nil or malformed gives
// the zero value.
func parsePreviewExtra(raw []byte) *previewExtra {
	x := &previewExtra{}
	if len(raw) > 0 && json.Unmarshal(raw, x) != nil {
		x = &previewExtra{}
	}
	return x
}

// previewExtraOf is a page message's preview extra, as loadKinds
// parsed it, else parsed now.
func previewExtraOf(k kinds, m logdb.Message) *previewExtra {
	if x := k.previews[m.RowID]; x != nil {
		return x
	}
	return parsePreviewExtra(m.Preview.Extra)
}

// addPreview records the preview of a message of the page with the
// people it names (a story's poster, an upgraded gift's owner and
// original sender and receiver) and the custom emoji it shows.
func (k *kinds) addPreview(m logdb.Message, users, chats, emoji *[]int64) {
	p := m.Preview
	if p == nil {
		return
	}
	x := parsePreviewExtra(p.Extra)
	if k.previews == nil {
		k.previews = map[uint64]*previewExtra{}
	}
	k.previews[m.RowID] = x
	add := func(r *partyRef) {
		if r == nil {
			return
		}
		switch id := r.id(); {
		case id > 0:
			*users = append(*users, id)
		case id < 0:
			*chats = append(*chats, id)
		}
	}
	switch p.Type {
	case "story", "live_story":
		add(&partyRef{ChatID: x.PosterChatID})
	case "upgraded_gift":
		if g := x.Gift; g != nil {
			add(g.Owner)
			if o := g.Original; o != nil {
				add(o.Sender)
				add(o.Receiver)
			}
		}
	case "text_composition_style":
		if x.CustomEmojiID != 0 {
			*emoji = append(*emoji, int64(x.CustomEmojiID))
		}
	}
}

// addPreviewTexts adds the people and custom emoji an upgraded gift
// preview's original text mentions to mc.
func (k *kinds) addPreviewTexts(mc *mentionCollector, m logdb.Message) {
	x := k.previews[m.RowID]
	if x == nil || m.Preview.Type != "upgraded_gift" || x.Gift == nil || x.Gift.Original == nil {
		return
	}
	if t := x.Gift.Original.Text; t != nil {
		mc.add(t.Text, t.Entities, "gift text")
	}
}

// previewMediumKinds are the content types the medium at position 0
// renders as, by preview type.
var previewMediumKinds = map[string]string{
	"video": "video", "animation": "animation", "audio": "audio",
	"document": "document", "sticker": "sticker", "video_note": "video_note",
	"voice_note": "voice", "story_album": "video",
	"embedded_video_player": "video", "embedded_animation_player": "animation",
	"embedded_audio_player": "audio",
}

// previewOwnMedia are the preview types whose files are in fields of
// their own, never the card's medium.
var previewOwnMedia = map[string]bool{
	"album": true, "background": true, "sticker_set": true, "gift_collection": true,
	"gift_auction": true, "upgraded_gift": true,
}

// previewMediaKind is the content type a preview's medium renders as:
// its preview type's, else the file's type when it is a medium.
func previewMediaKind(ptype string, f logdb.FileRef) string {
	if k, ok := previewMediumKinds[ptype]; ok {
		return k
	}
	switch f.Type {
	case "video", "animation", "audio":
		return f.Type
	}
	return "document"
}

// cleanDetail is a medium's keys as mediaDetail reads them: the
// minithumbnail is dropped (a card never shows one), and an audio's
// title is audio_title (title being the card's own). nil when there is
// nothing to read.
func cleanDetail(raw json.RawMessage) []byte {
	var x map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &x) != nil || x == nil {
		return nil
	}
	delete(x, "minithumbnail")
	delete(x, "title")
	if t, ok := x["audio_title"]; ok {
		x["title"] = t
	}
	for k, v := range x {
		if v == nil {
			delete(x, k)
		}
	}
	b, err := json.Marshal(x)
	if err != nil {
		return nil
	}
	return b
}

// previewDetail is the content_extra-shaped object mediaDetail reads
// for a preview's medium or photo: an embedded player's medium has its
// keys under "media", a story album's video icon under "video", and a
// photo has its size only for the types that carry one. nil when there
// is nothing to read.
func previewDetail(ptype string, extra []byte, medium bool) []byte {
	if len(extra) == 0 {
		return nil
	}
	var x map[string]json.RawMessage
	if json.Unmarshal(extra, &x) != nil {
		return nil
	}
	if !medium {
		switch ptype {
		case "article", "photo", "app", "web_app", "story_album":
			b, _ := json.Marshal(map[string]json.RawMessage{"width": x["width"], "height": x["height"]})
			return cleanDetail(b)
		}
		return nil
	}
	switch {
	case strings.HasPrefix(ptype, "embedded_"):
		return cleanDetail(x["media"])
	case ptype == "story_album":
		return cleanDetail(x["video"])
	}
	return cleanDetail(extra)
}

// previewFile is the preview's preview_media file at pos.
func previewFile(p *logdb.LinkPreview, pos int) (logdb.FileRef, bool) {
	for _, f := range p.Files {
		if f.Position == pos {
			return f.File, true
		}
	}
	return logdb.FileRef{}, false
}

// previewList is the media of a list the extra describes (an album's
// items, a set's stickers), item i the file at position i: nil for an
// item without one. kind gives an item's content type from its keys.
func (v view) previewList(p *logdb.LinkPreview, items []json.RawMessage,
	kind func(json.RawMessage) string) []*chatMediaJSON {
	out := make([]*chatMediaJSON, len(items))
	for i, it := range items {
		if f, ok := previewFile(p, i); ok {
			out[i] = v.media(kind(it), f, cleanDetail(it))
		}
	}
	return out
}

// albumItemKind is an album item's content type: its kind, photo or
// video.
func albumItemKind(it json.RawMessage) string {
	var x struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(it, &x)
	if x.Kind == "video" {
		return "video"
	}
	return "photo"
}

// baseThemes are the values of LinkPreviewTheme.base_theme.
var baseThemes = map[string]bool{"classic": true, "day": true, "night": true, "tinted": true,
	"arctic": true}

// previewTheme is a theme preview's settings.
func previewTheme(x *previewExtra) *previewThemeJSON {
	j := &previewThemeJSON{AccentColor: x.AccentColor & 0xffffff, OutFill: cleanFill(x.OutFill),
		AnimateOutFill: x.AnimateOutFill}
	if baseThemes[x.BaseTheme] {
		t := x.BaseTheme
		j.BaseTheme = &t
	}
	if c := x.OutAccent; c != nil {
		n := *c & 0xffffff
		j.OutAccent = &n
	}
	if b := x.ThemeBackground; b != nil {
		dark := b.Dark
		j.Background = &previewBackgroundJSON{backgroundLookJSON: b.backgroundExtra.look(),
			Name: b.Name, Dark: &dark}
	}
	return j
}

// previewGift is a gift auction's or an upgraded gift preview's gift;
// nil when nothing of it is stored.
func (v view) previewGift(k kinds, p *logdb.LinkPreview, x *previewExtra) *previewGiftJSON {
	sticker := func(pos int, emoji string) *chatMediaJSON {
		f, ok := previewFile(p, pos)
		if !ok {
			return nil
		}
		var extra []byte
		if emoji != "" {
			extra, _ = json.Marshal(map[string]string{"emoji": emoji})
		}
		return v.media("sticker", f, extra)
	}
	j := &previewGiftJSON{}
	if p.Type == "upgraded_gift" {
		if x.Gift != nil {
			j.Upgraded = v.upgradedGift(k, x.Gift)
		}
		j.Sticker, j.SymbolSticker = sticker(0, ""), sticker(1, "")
		if j.Upgraded == nil && j.Sticker == nil && j.SymbolSticker == nil {
			return nil
		}
		return j
	}
	var emoji string
	if g := x.Gift; g != nil {
		j.StarCount, emoji = nonZero(g.StarCount), g.Emoji
	}
	j.Sticker = sticker(0, emoji)
	j.Auction = &previewAuctionJSON{StartDate: isoPtr(x.AuctionStart), EndDate: isoPtr(x.AuctionEnd),
		GiftsPerRound: nonZero(x.GiftsPerRound)}
	if x.Gift == nil && j.Sticker == nil && x.AuctionStart == 0 && x.AuctionEnd == 0 &&
		x.GiftsPerRound == 0 {
		return nil
	}
	return j
}

// previewMedium is the card's medium, the preview_media file at
// position 0, with the preview's photo as its thumb and, for a video or
// an animation, its poster; a video preview's own cover (position 1)
// is the poster first.
func (v view) previewMedium(p *logdb.LinkPreview) *chatMediaJSON {
	md := v.media(previewMediaKind(p.Type, p.Media), p.Media, previewDetail(p.Type, p.Extra, true))
	var thumb *string
	if p.Photo.ID != 0 {
		thumb = v.photo(p.Photo)
	}
	poster := thumb
	if f, ok := previewFile(p, 1); ok && p.Type == "video" {
		if c := v.photo(f); c != nil {
			poster = c
			if thumb == nil {
				thumb = c
			}
		}
	}
	if poster != nil && (md.Render == "video" || md.Render == "animation") {
		md.Poster = poster
	}
	if thumb != nil {
		md.Thumb = thumb
	}
	return md
}

// linkPreview is a message's link preview card, nil when it has none.
func (v view) linkPreview(k kinds, m logdb.Message) *linkPreviewJSON {
	p := m.Preview
	if p == nil {
		return nil
	}
	x := previewExtraOf(k, m)
	j := &linkPreviewJSON{Type: p.Type, Site: p.Site, Title: p.Title, Desc: p.Description,
		URL: p.URL, Author: p.Author, Duration: x.Duration}
	if p.Media.ID != 0 && !previewOwnMedia[p.Type] {
		j.Media = v.previewMedium(p)
	} else {
		j.Media = v.media("photo", p.Photo, previewDetail(p.Type, p.Extra, false))
	}
	if x.EmbedURL != "" && strings.HasPrefix(p.Type, "embedded_") {
		j.Embed = &previewEmbedJSON{URL: x.EmbedURL, W: x.EmbedWidth, H: x.EmbedHeight}
	}
	// Shown as the chat itself would be opened (logdb.OpenChat): a
	// group the scope sees, or an exposed one.
	if t := p.Target; t != nil && (t.Exposed || v.sc.SeesGroup(t.ChatID)) {
		j.Target = &previewTargetJSON{ChatID: t.ChatID, MessageID: t.MessageID}
	}
	switch p.Type {
	case "album":
		j.Album = &previewAlbumJSON{Caption: x.Caption,
			Items: v.previewList(p, x.Items, albumItemKind)}
	case "background":
		b := x.backgroundExtra
		b.Type = x.BackgroundType
		j.Background = &previewBackgroundJSON{backgroundLookJSON: b.look()}
		if f, ok := previewFile(p, 0); ok {
			j.Background.Image = v.backgroundImage(f)
		}
	case "sticker_set", "gift_collection":
		items := x.Stickers
		if p.Type == "gift_collection" {
			items = x.Icons
		}
		if len(items) > 0 {
			j.Stickers = v.previewList(p, items, func(json.RawMessage) string { return "sticker" })
		}
	case "gift_auction", "upgraded_gift":
		j.Gift = v.previewGift(k, p, x)
	case "story", "live_story":
		if x.StoryID != 0 {
			s := &previewStoryJSON{StoryID: x.StoryID}
			if x.PosterChatID != 0 {
				s.Poster = v.refPtr(k, x.PosterChatID)
			}
			if s.Poster != nil {
				s.Username = refUsername(k, x.PosterChatID)
			}
			j.Story = s
		}
	case "theme":
		if len(p.Extra) > 0 {
			j.Theme = previewTheme(x)
		}
	case "text_composition_style":
		if id := int64(x.CustomEmojiID); id != 0 {
			j.CustomEmojiID = strconv.FormatInt(id, 10)
			j.CustomEmoji = jsonOrNull(v.customEmojiOf(k, id))
		}
	}
	return j
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"strings"

	"tgloggerd/webapi/internal/logdb"
)

// A message's link preview card (LinkPreview): the card's columns, its
// type's own fields from extra (docs/design/message-types.md section
// 4.8) that the web shows, its own medium, and the archived message a
// message preview points at.

type linkPreviewJSON struct {
	Type     string             `json:"type"`
	Site     string             `json:"site"`
	Title    string             `json:"title"`
	Desc     string             `json:"desc"`
	URL      string             `json:"url"`
	Author   string             `json:"author,omitempty"`
	Media    *chatMediaJSON     `json:"media,omitempty"`
	Duration int                `json:"duration,omitempty"`
	Embed    *previewEmbedJSON  `json:"embed,omitempty"`
	Target   *previewTargetJSON `json:"target,omitempty"`
}

type previewEmbedJSON struct {
	URL string `json:"url"`
	W   int    `json:"w,omitempty"`
	H   int    `json:"h,omitempty"`
}

type previewTargetJSON struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// previewExtra is the part of a preview's extra the card reads.
type previewExtra struct {
	Duration    int    `json:"duration"`
	EmbedURL    string `json:"embed_url"`
	EmbedWidth  int    `json:"embed_width"`
	EmbedHeight int    `json:"embed_height"`
}

// previewMediaKind is the content type a preview's medium renders as:
// a video, animation or audio preview's own, an embedded player's by
// the player; else the file's type when it is one of them.
func previewMediaKind(ptype string, f logdb.FileRef) string {
	switch k := strings.TrimSuffix(strings.TrimPrefix(ptype, "embedded_"), "_player"); k {
	case "video", "animation", "audio":
		return k
	}
	switch f.Type {
	case "video", "animation", "audio":
		return f.Type
	}
	return "document"
}

// previewDetail is the content_extra-shaped object mediaDetail reads
// for a preview's medium or photo: an embedded player's medium has its
// keys under "media", an audio's title is audio_title (title being the
// card's own), and a photo has its size only for the types that carry
// one. The minithumbnail is dropped: the card never shows one. nil
// when there is nothing to read.
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
		case "article", "photo", "app", "web_app":
			x = map[string]json.RawMessage{"width": x["width"], "height": x["height"]}
		default:
			return nil
		}
	} else if strings.HasPrefix(ptype, "embedded_") {
		var m map[string]json.RawMessage
		if json.Unmarshal(x["media"], &m) != nil {
			return nil
		}
		x = m
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

// linkPreview is a message's link preview card, nil when it has none.
func (v view) linkPreview(m logdb.Message) *linkPreviewJSON {
	p := m.Preview
	if p == nil {
		return nil
	}
	j := &linkPreviewJSON{Type: p.Type, Site: p.Site, Title: p.Title, Desc: p.Description,
		URL: p.URL, Author: p.Author}
	if p.Media.ID != 0 {
		j.Media = v.media(previewMediaKind(p.Type, p.Media), p.Media,
			previewDetail(p.Type, p.Extra, true))
		// The preview's photo is the medium's picture, as a
		// video's cover is.
		if u := v.photo(p.Photo); p.Photo.ID != 0 && u != nil {
			if j.Media.Render == "video" || j.Media.Render == "animation" {
				j.Media.Poster = u
			}
			j.Media.Thumb = u
		}
	} else {
		j.Media = v.media("photo", p.Photo, previewDetail(p.Type, p.Extra, false))
	}
	var x previewExtra
	if len(p.Extra) > 0 {
		_ = json.Unmarshal(p.Extra, &x)
	}
	j.Duration = x.Duration
	if x.EmbedURL != "" && strings.HasPrefix(p.Type, "embedded_") {
		j.Embed = &previewEmbedJSON{URL: x.EmbedURL, W: x.EmbedWidth, H: x.EmbedHeight}
	}
	// Shown as the chat itself would be opened (logdb.OpenChat): a
	// group the scope sees, or an exposed one.
	if t := p.Target; t != nil && (t.Exposed || v.sc.SeesGroup(t.ChatID)) {
		j.Target = &previewTargetJSON{ChatID: t.ChatID, MessageID: t.MessageID}
	}
	return j
}

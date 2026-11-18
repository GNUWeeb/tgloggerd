// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/base64"
	"encoding/json"

	"tgloggerd/webapi/internal/logdb"
)

// The detail of a message's media the chat view and the message page
// show: how it renders, its size, length and format, a sticker's emoji,
// a voice note's waveform, an audio file's title and performer. Rows
// logged since migration 000034 carry it in content_extra
// (docs/design/message-types.md section 4.2.1); for older rows it
// falls back to what the Drogon chat view reads, the content type and
// the file's extension (web/src/dao/Browse.cpp, mediaRender).

// contentExtra is the part of content_extra the media detail reads.
type contentExtra struct {
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Length        int    `json:"length"` // a video note's diameter
	Duration      int    `json:"duration"`
	Mime          string `json:"mime"`
	Emoji         string `json:"emoji"`
	Format        string `json:"format"`
	Waveform      string `json:"waveform"`
	Title         string `json:"title"`
	Performer     string `json:"performer"`
	Minithumbnail *struct {
		Data string `json:"data"`
	} `json:"minithumbnail"`
}

// parseExtra reads content_extra; nil, malformed or a class without
// curated keys ({"tdlib": ...}) gives the zero value.
func parseExtra(raw []byte) contentExtra {
	var x contentExtra
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &x)
	}
	return x
}

// mediaDetailJSON is the part of ChatMedia, and of MessageFull's
// media, that describes the media beyond its file.
type mediaDetailJSON struct {
	Render    string `json:"render,omitempty"`
	Ext       string `json:"ext,omitempty"`
	Format    string `json:"format,omitempty"`
	Mime      string `json:"mime,omitempty"`
	W         int    `json:"w,omitempty"`
	H         int    `json:"h,omitempty"`
	Duration  int    `json:"duration,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
	Wave      []int  `json:"wave,omitempty"`
	Title     string `json:"title,omitempty"`
	Performer string `json:"performer,omitempty"`
	Minithumb string `json:"minithumb,omitempty"`
	// Poster is a video's cover photo, its /files/<token> URL.
	Poster *string `json:"poster,omitempty"`
}

// stickerFormat is a sticker's file format: content_extra's, else the
// file's extension; "" when neither says.
func stickerFormat(x contentExtra, f logdb.FileRef) string {
	for _, v := range []string{x.Format, f.Ext} {
		switch v {
		case "webp", "webm", "tgs":
			return v
		}
	}
	return ""
}

// renderOf is how the media renders, as Drogon's mediaRender decides:
// image, video, animation (loops, muted), lottie (a .tgs sticker),
// audio, or file.
func renderOf(ctype, format string, f logdb.FileRef) string {
	switch ctype {
	case "sticker":
		switch format {
		case "webm":
			return "animation"
		case "tgs":
			return "lottie"
		}
		return "image"
	case "photo":
		return "image"
	case "video", "video_note":
		return "video"
	case "animation":
		return "animation"
	case "audio", "voice":
		return "audio"
	}
	if f.Type == "photo" {
		return "image"
	}
	return "file"
}

// decodeWaveform unpacks Telegram's waveform: 5-bit samples packed
// little-endian, one per bar. Malformed base64 gives nil.
func decodeWaveform(b64 string) []int {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(b) == 0 {
		return nil
	}
	n := len(b) * 8 / 5
	out := make([]int, n)
	for i := range n {
		bit := i * 5
		v := int(b[bit/8])
		if bit/8+1 < len(b) {
			v |= int(b[bit/8+1]) << 8
		}
		out[i] = (v >> (bit % 8)) & 31
	}
	return out
}

// mediaDetail describes a message's (or an edit version's) media.
func mediaDetail(ctype string, f logdb.FileRef, extra []byte) mediaDetailJSON {
	x := parseExtra(extra)
	d := mediaDetailJSON{Ext: f.Ext, Mime: x.Mime, W: x.Width, H: x.Height,
		Duration: x.Duration, Title: x.Title, Performer: x.Performer}
	if ctype == "sticker" {
		d.Format = stickerFormat(x, f)
		d.Emoji = x.Emoji
	}
	d.Render = renderOf(ctype, d.Format, f)
	if x.Length > 0 && d.W == 0 && d.H == 0 {
		d.W, d.H = x.Length, x.Length
	}
	if x.Waveform != "" {
		d.Wave = decodeWaveform(x.Waveform)
	}
	if m := x.Minithumbnail; m != nil && m.Data != "" {
		if _, err := base64.StdEncoding.DecodeString(m.Data); err == nil {
			d.Minithumb = "data:image/jpeg;base64," + m.Data
		}
	}
	return d
}

// chatTypes are the content types of the API's ContentType: the kinds
// the daemon gained with migration 000034 (video_note, location, ...)
// are shown as unknown until the chat view renders them
// (docs/todo/web-message-types.md), as the Drogon web shows them, and
// tdlib_type says what they are.
var chatTypes = map[string]bool{"text": true, "photo": true, "video": true,
	"document": true, "audio": true, "voice": true, "sticker": true, "animation": true,
	"service": true, "unknown": true}

func chatType(ctype string) string {
	if chatTypes[ctype] {
		return ctype
	}
	return "unknown"
}

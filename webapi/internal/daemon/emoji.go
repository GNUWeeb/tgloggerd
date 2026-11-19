// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// The most ids one POST /v1/custom-emoji or /v1/message-effects takes
// (the daemon refuses more): getCustomEmojiStickers's limit, and one
// getMessageEffect call per effect.
const (
	CustomEmojiLimit   = 200
	MessageEffectLimit = 50
)

// emojiTimeout bounds a custom emoji or effect lookup: the daemon
// gives Telegram up to 15 s (20 s for all of a request's effects); a
// daemon slower than this counts as away.
const emojiTimeout = 25 * time.Second

// The outcomes of POST /v1/custom-emoji and /v1/message-effects. The
// rows already stored come with every one of them.
const (
	EmojiOK          = "ok"
	EmojiFloodWait   = "flood_wait"
	EmojiUnavailable = "unavailable"
	EmojiTimeout     = "timeout"
	EmojiError       = "error"
)

// CustomEmoji is one telegram_custom_emoji row: the sticker behind a
// custom emoji id. Ids are strings in the daemon's answer, as they do
// not fit a JavaScript number, and int64 here. SetID is 0 when unknown,
// Format ("webp", "tgs", "webm") "" when unknown, FileID 0 when the
// sticker had no file to record.
type CustomEmoji struct {
	ID              int64
	Emoji           string
	SetID           int64
	Format          string
	NeedsRepainting bool
	FileID          uint64
}

// MessageEffect is one telegram_message_effects row. Type is
// "emoji_reaction", "premium_sticker" or "" (a type the daemon does not
// know); a file id is 0 when the effect has no such file.
type MessageEffect struct {
	ID                    int64
	Emoji                 string
	IsPremium             bool
	Type                  string
	StaticIconFileID      uint64
	SelectAnimationFileID uint64
	EffectAnimationFileID uint64
	StickerFileID         uint64
	StickerFormat         string
}

// EmojiLookup is the daemon's answer to POST /v1/custom-emoji or
// /v1/message-effects. Emoji or Effects hold the rows known, in the
// order asked; Missing the ids Telegram does not know. An id in
// neither was not resolved this time (Outcome says why) and may be
// asked for again. Asked is how many ids the daemon sent to Telegram.
// RetryAfter is in seconds, for EmojiFloodWait only; Message is
// Telegram's.
type EmojiLookup struct {
	Outcome    string
	Message    string
	RetryAfter int64
	Asked      int
	Emoji      []CustomEmoji
	Effects    []MessageEffect
	Missing    []int64
}

// CustomEmojiStickers is POST /v1/custom-emoji: the stickers of up to
// CustomEmojiLimit custom emoji ids, which the daemon asks Telegram for
// only when it has no row for them. ok is false when there is no
// answer to go by: the client is off, ids is empty or over the limit
// (nothing is sent), the daemon is away, slow or refuses the request,
// or its answer is not a lookup's. reply is the call itself, for logs.
func (c *Client) CustomEmojiStickers(ctx context.Context, ids []int64) (EmojiLookup, bool, Reply) {
	return c.emojiLookup(ctx, "/v1/custom-emoji", ids, false, emojiTimeout)
}

// MessageEffects is POST /v1/message-effects: up to MessageEffectLimit
// message effects, answered as CustomEmojiStickers is.
func (c *Client) MessageEffects(ctx context.Context, ids []int64) (EmojiLookup, bool, Reply) {
	return c.emojiLookup(ctx, "/v1/message-effects", ids, true, emojiTimeout)
}

// idString is an id the daemon sends as a decimal string.
type idString int64

func (v *idString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*v = idString(n)
	return nil
}

// optID is a decimal string id that may be null (0).
type optID struct{ v int64 }

func (o *optID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		o.v = 0
		return nil
	}
	var s idString
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.v = int64(s)
	return nil
}

type rawEmoji struct {
	ID              *idString `json:"custom_emoji_id"`
	Emoji           string    `json:"emoji"`
	SetID           optID     `json:"set_id"`
	Format          *string   `json:"format"`
	NeedsRepainting bool      `json:"needs_repainting"`
	FileID          *uint64   `json:"file_id"`
}

type rawEffect struct {
	ID                    *idString `json:"effect_id"`
	Emoji                 string    `json:"emoji"`
	IsPremium             bool      `json:"is_premium"`
	Type                  *string   `json:"type"`
	StaticIconFileID      *uint64   `json:"static_icon_file_id"`
	SelectAnimationFileID *uint64   `json:"select_animation_file_id"`
	EffectAnimationFileID *uint64   `json:"effect_animation_file_id"`
	StickerFileID         *uint64   `json:"sticker_file_id"`
	StickerFormat         *string   `json:"sticker_format"`
}

func deref[T any](p *T) T {
	var z T
	if p == nil {
		return z
	}
	return *p
}

// emojiLookup sends ids to path; effects says which answer it is
// (/v1/message-effects), as each carries its own list.
func (c *Client) emojiLookup(ctx context.Context, path string, ids []int64, effects bool,
	timeout time.Duration) (EmojiLookup, bool, Reply) {
	if c == nil {
		return EmojiLookup{}, false, c.Call(ctx, http.MethodPost, path, nil, timeout)
	}
	limit := CustomEmojiLimit
	if effects {
		limit = MessageEffectLimit
	}
	if len(ids) == 0 || len(ids) > limit {
		return EmojiLookup{}, false, Reply{Error: "want 1 to " + strconv.Itoa(limit) + " ids"}
	}
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: make([]string, 0, len(ids))}
	for _, id := range ids {
		body.IDs = append(body.IDs, strconv.FormatInt(id, 10))
	}
	r := c.Call(ctx, http.MethodPost, path, body, timeout)
	var raw struct {
		Outcome    string       `json:"outcome"`
		Message    string       `json:"message"`
		RetryAfter int64        `json:"retry_after"`
		Asked      int          `json:"asked"`
		Emoji      *[]rawEmoji  `json:"emoji"`
		Effects    *[]rawEffect `json:"effects"`
		Missing    *[]idString  `json:"missing"`
	}
	if !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &raw) != nil {
		return EmojiLookup{}, false, r
	}
	switch raw.Outcome {
	case EmojiOK, EmojiFloodWait, EmojiUnavailable, EmojiTimeout, EmojiError:
	default:
		return EmojiLookup{}, false, r
	}
	if raw.Missing == nil || (effects && raw.Effects == nil) ||
		(!effects && raw.Emoji == nil) {
		return EmojiLookup{}, false, r
	}
	l := EmojiLookup{Outcome: raw.Outcome, Message: raw.Message, Asked: max(raw.Asked, 0)}
	if raw.Outcome == EmojiFloodWait {
		l.RetryAfter = max(raw.RetryAfter, 0)
	}
	if !effects {
		for _, e := range *raw.Emoji {
			if e.ID == nil {
				return EmojiLookup{}, false, r
			}
			l.Emoji = append(l.Emoji, CustomEmoji{
				ID: int64(*e.ID), Emoji: e.Emoji, SetID: e.SetID.v,
				Format: deref(e.Format), NeedsRepainting: e.NeedsRepainting,
				FileID: deref(e.FileID),
			})
		}
	}
	if effects {
		for _, e := range *raw.Effects {
			if e.ID == nil {
				return EmojiLookup{}, false, r
			}
			l.Effects = append(l.Effects, MessageEffect{
				ID: int64(*e.ID), Emoji: e.Emoji, IsPremium: e.IsPremium,
				Type:                  deref(e.Type),
				StaticIconFileID:      deref(e.StaticIconFileID),
				SelectAnimationFileID: deref(e.SelectAnimationFileID),
				EffectAnimationFileID: deref(e.EffectAnimationFileID),
				StickerFileID:         deref(e.StickerFileID),
				StickerFormat:         deref(e.StickerFormat),
			})
		}
	}
	for _, id := range *raw.Missing {
		l.Missing = append(l.Missing, int64(id))
	}
	return l, true, r
}

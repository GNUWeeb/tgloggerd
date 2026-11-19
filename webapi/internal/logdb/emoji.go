// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"fmt"
)

// The stickers behind custom emoji ids (telegram_custom_emoji) and the
// message effects (telegram_message_effects), which the daemon fills on
// demand (docs/design/message-types.md sections 4.11 and 4.12).

// CustomEmoji is a telegram_custom_emoji row with its sticker's file.
// Format is "webp", "tgs", "webm" or "" when unknown; File.ID is 0
// when no file was recorded.
type CustomEmoji struct {
	ID              int64
	Emoji           string
	Format          string
	NeedsRepainting bool
	File            FileRef
}

// MessageEffect is a telegram_message_effects row with the file of its
// small picture: the static icon (webp), else a premium sticker
// effect's sticker (in its sticker_format), else an emoji reaction
// effect's select animation (tgs). Format is "" and File.ID 0 when it
// has none of them.
type MessageEffect struct {
	ID     int64
	Emoji  string
	Format string
	File   FileRef
}

// CustomEmojiByID are the custom emoji among ids the table has, by id.
func CustomEmojiByID(ctx context.Context, db *sql.DB, ids []int64) (map[int64]CustomEmoji, error) {
	ids = uniq(ids)
	out := map[int64]CustomEmoji{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT e.custom_emoji_id, e.emoji, COALESCE(e.format, ''), "+
		"e.needs_repainting, COALESCE(e.file_id, 0), "+fileColumnsOf("f")+
		" FROM telegram_custom_emoji e LEFT JOIN telegram_files f ON f.id = e.file_id "+
		"WHERE e.custom_emoji_id IN ("+placeholders(len(ids))+")", int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			e      CustomEmoji
			fileID uint64
			f      nullFile
		)
		dest := append([]any{&e.ID, &e.Emoji, &e.Format, &e.NeedsRepainting, &fileID}, f.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if fileID != 0 {
			e.File = f.ref(fileID)
		}
		out[e.ID] = e
	}
	return out, rows.Err()
}

// effectPicture and effectFormat pick an effect's small picture and
// its format, in the order MessageEffect gives; NULL for none.
const effectPicked = "CASE WHEN e.static_icon_file_id IS NOT NULL THEN %s " +
	"WHEN e.effect_type = 'premium_sticker' AND e.sticker_file_id IS NOT NULL " +
	"AND e.sticker_format IS NOT NULL THEN %s " +
	"WHEN e.effect_type = 'emoji_reaction' AND e.select_animation_file_id IS NOT NULL " +
	"THEN %s END"

var (
	effectPicture = fmt.Sprintf(effectPicked, "e.static_icon_file_id", "e.sticker_file_id",
		"e.select_animation_file_id")
	effectFormat = fmt.Sprintf(effectPicked, "'webp'", "e.sticker_format", "'tgs'")
)

// MessageEffectsByID are the message effects among ids the table has,
// by id.
func MessageEffectsByID(ctx context.Context, db *sql.DB, ids []int64) (map[int64]MessageEffect, error) {
	ids = uniq(ids)
	out := map[int64]MessageEffect{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT e.effect_id, e.emoji, COALESCE("+effectFormat+", ''), "+
		"COALESCE("+effectPicture+", 0), "+fileColumnsOf("f")+
		" FROM telegram_message_effects e LEFT JOIN telegram_files f ON f.id = "+effectPicture+
		" WHERE e.effect_id IN ("+placeholders(len(ids))+")", int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			e      MessageEffect
			fileID uint64
			f      nullFile
		)
		dest := append([]any{&e.ID, &e.Emoji, &e.Format, &fileID}, f.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if fileID != 0 {
			e.File = f.ref(fileID)
		}
		out[e.ID] = e
	}
	return out, rows.Err()
}

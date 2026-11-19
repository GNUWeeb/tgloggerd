// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
)

// What a service message's detail reads beyond its row
// (docs/design/message-types.md section 4.2.3): the messages a page's
// pins are about and the forum topics its topic lines name, each in
// one batch per page of messages.

// Target is a message another message of its chat is about (a pin's
// message), as a one-line quote shows it.
type Target struct {
	RowID       uint64
	MessageID   int64
	ContentType string
	// Snippet is the start of its text, as much as a reply quotes.
	Snippet string
	Deleted bool
	// Extra and FileName are its content_extra and its file's name,
	// read only when it has no text, for its summary.
	Extra    []byte
	FileName string
}

// TargetsByID are the logged messages of a chat among the message ids
// (server ids, as stored); missing ones are left out. A group's chat
// id is negative, a private chat's is its user's.
func TargetsByID(ctx context.Context, db *sql.DB, chat int64, ids []int64) (map[int64]Target, error) {
	ids = uniq(ids)
	out := map[int64]Target{}
	if len(ids) == 0 || chat == 0 {
		return out, nil
	}
	t := tablesOf(chat < 0)
	rows, err := db.QueryContext(ctx, "SELECT m.id, m.message_id, m.content_type, "+
		"LEFT(COALESCE(m.text, ''), 120), m.deleted_at IS NOT NULL, "+
		"IF(COALESCE(m.text, '') = '', m.content_extra, NULL), "+
		"IF(COALESCE(m.text, '') = '', (SELECT f.orig_file_name FROM telegram_files f "+
		"WHERE f.id = m.file_id), NULL) "+
		"FROM "+t.msgs+" m WHERE m.chat_id = ? AND m.message_id IN ("+placeholders(len(ids))+")",
		append([]any{chat}, int64Args(ids)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			m    Target
			name sql.NullString
		)
		if err := rows.Scan(&m.RowID, &m.MessageID, &m.ContentType, &m.Snippet, &m.Deleted,
			&m.Extra, &name); err != nil {
			return nil, err
		}
		m.FileName = name.String
		out[m.MessageID] = m
	}
	return out, rows.Err()
}

// TopicsByID are a group's forum topics among ids, as last seen;
// missing ones are left out, and a private chat has none.
func TopicsByID(ctx context.Context, db *sql.DB, chat int64, ids []int64) (map[int64]Topic, error) {
	ids = uniq(ids)
	out := map[int64]Topic{}
	if len(ids) == 0 || chat >= 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT topic_id, name, icon_color, is_closed, is_hidden, "+
		"COALESCE(icon_custom_emoji_id, 0) "+
		"FROM telegram_forum_topics WHERE chat_id = ? AND topic_id IN ("+
		placeholders(len(ids))+")", append([]any{chat}, int64Args(ids)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Closed, &t.Hidden, &t.IconEmoji); err != nil {
			return nil, err
		}
		out[t.ID] = t
	}
	return out, rows.Err()
}

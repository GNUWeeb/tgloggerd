// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
)

// MessageExtras are the content_extra of the messages of a chat among
// these message ids (server ids, as stored), by message id: what a
// service action about another message reads of it, such as the tasks
// of the checklist whose tasks were marked done, or the prize of the
// giveaway that was completed. A group chat's id is negative, a private
// chat's is its user's. Messages not in the archive, or without a
// content_extra, are left out. The caller reads the chat, so its
// messages are the viewer's to see.
func MessageExtras(ctx context.Context, db *sql.DB, chat int64, ids []int64) (map[int64][]byte, error) {
	ids = uniq(ids)
	out := map[int64][]byte{}
	if len(ids) == 0 || chat == 0 {
		return out, nil
	}
	t := tablesOf(chat < 0)
	rows, err := db.QueryContext(ctx, "SELECT message_id, content_extra FROM "+t.msgs+
		" WHERE chat_id = ? AND message_id IN ("+placeholders(len(ids))+
		") AND content_extra IS NOT NULL", append([]any{chat}, int64Args(ids)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id    int64
			extra []byte
		)
		if err := rows.Scan(&id, &extra); err != nil {
			return nil, err
		}
		out[id] = extra
	}
	return out, rows.Err()
}

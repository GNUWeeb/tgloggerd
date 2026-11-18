// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strconv"

	"tgloggerd/webapi/internal/scope"
)

// What the list pages read besides the search itself: the archive's
// totals, the message a file came in, the origin of forwarded
// messages, and the chat search of the pickers. Each takes the scope.

// ArchiveTotal is the number of rows of the entity, for a scope that
// sees all of them; ok is false for one that does not, which gets no
// archive total at all. It is exact when counting fits the statement
// time limit, the table's row estimate otherwise (always for entities
// counted by estimate).
func ArchiveTotal(ctx context.Context, db *sql.DB, sc scope.Scope, e *Entity) (int64, bool, error) {
	if !e.visible(sc) {
		return 0, false, nil
	}
	if cond, _ := e.filter(sc); cond != "" {
		return 0, false, nil
	}
	if !e.estimateBrowse {
		n, err := countInt(ctx, db, "SELECT /*+ MAX_EXECUTION_TIME("+
			strconv.Itoa(statementMS)+") */ COUNT(*) FROM "+e.countFromFor(""))
		if err == nil {
			return n, true, nil
		}
		if ctx.Err() != nil {
			return 0, false, ctx.Err()
		}
	}
	var est sql.NullInt64
	err := db.QueryRowContext(ctx, "SELECT table_rows FROM information_schema.tables "+
		"WHERE table_schema = DATABASE() AND table_name = ?", e.countTable()).Scan(&est)
	if err != nil {
		return 0, false, err
	}
	return est.Int64, true, nil
}

// StoredBytes is the size of the files whose bytes are in the store,
// for a scope that sees every file; ok is false otherwise, or when the
// sum does not fit the statement time limit.
func StoredBytes(ctx context.Context, db *sql.DB, sc scope.Scope) (int64, bool) {
	if cond, _ := Files.filter(sc); cond != "" || !Files.visible(sc) {
		return 0, false
	}
	n, err := countInt(ctx, db, "SELECT /*+ MAX_EXECUTION_TIME("+
		strconv.Itoa(statementMS)+") */ SUM(file_size) FROM telegram_files "+
		"WHERE on_disk = 1")
	if err != nil {
		return 0, false
	}
	return n, true
}

// FileOrigin is the first message a file was sent in, among those the
// scope sees.
type FileOrigin struct {
	Scope     string // "group" or "private"
	MessageID uint64 // the message's row id
	ChatID    int64
	ChatTitle string
	ChatKind  string // "group" or "user"
	SenderID  int64  // 0: the logging account, or a chat
	Sender    string // the sender's name, "" with SenderID 0
}

// FileOrigins are the first visible messages of the files ids, by file
// id: a group message when there is one, else a private message. A
// file sent in no visible message has none.
func FileOrigins(ctx context.Context, db *sql.DB, sc scope.Scope, ids []uint64) (map[uint64]FileOrigin, error) {
	out := map[uint64]FileOrigin{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	gf, ga := sc.GroupFilter("fm.chat_id")
	q := "SELECT gm.file_id, gm.id, gm.chat_id, g.title, gm.sender_user_id, " +
		"su.first_name, su.last_name FROM telegram_group_messages gm " +
		"JOIN (SELECT fm.file_id, MIN(fm.id) AS id FROM telegram_group_messages fm " +
		"WHERE fm.file_id IN (" + placeholders(len(ids)) + ") AND " + gf +
		" GROUP BY fm.file_id) x ON x.id = gm.id " +
		"LEFT JOIN `telegram_groups` g ON g.id = gm.chat_id " +
		"LEFT JOIN telegram_users su ON su.id = gm.sender_user_id"
	rs, err := db.QueryContext(ctx, q, both(args, ga)...)
	if err != nil {
		return nil, err
	}
	rows, err := scanRows(rs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		o := FileOrigin{Scope: "group", MessageID: r.Uint("id"), ChatID: r.Int("chat_id"),
			ChatTitle: GroupTitle(r.Str("title")), ChatKind: "group"}
		if !r.Null("sender_user_id") {
			o.SenderID = r.Int("sender_user_id")
			o.Sender = DisplayName(r.Str("first_name"), r.Str("last_name"))
		}
		out[r.Uint("file_id")] = o
	}
	if !sc.SeesPrivate() || len(out) == len(ids) {
		return out, nil
	}
	q = "SELECT pm.file_id, pm.id, pm.chat_id, pu.first_name AS peer_first, " +
		"pu.last_name AS peer_last, pm.sender_id, pm.is_outgoing " +
		"FROM telegram_private_messages pm " +
		"JOIN (SELECT fp.file_id, MIN(fp.id) AS id FROM telegram_private_messages fp " +
		"WHERE fp.file_id IN (" + placeholders(len(ids)) + ") GROUP BY fp.file_id) x " +
		"ON x.id = pm.id LEFT JOIN telegram_users pu ON pu.id = pm.chat_id"
	if rs, err = db.QueryContext(ctx, q, args...); err != nil {
		return nil, err
	}
	if rows, err = scanRows(rs); err != nil {
		return nil, err
	}
	for _, r := range rows {
		id := r.Uint("file_id")
		if _, ok := out[id]; ok {
			continue
		}
		peer := DisplayName(r.Str("peer_first"), r.Str("peer_last"))
		o := FileOrigin{Scope: "private", MessageID: r.Uint("id"), ChatID: r.Int("chat_id"),
			ChatTitle: peer, ChatKind: "user"}
		if !r.Bool("is_outgoing") && !r.Null("sender_id") {
			// Incoming private messages come from the peer.
			o.SenderID, o.Sender = r.Int("sender_id"), peer
		}
		out[id] = o
	}
	return out, nil
}

// ForwardOrigins names where forwarded messages came from, by message
// row id, for the messages of one scope ("group" or "private") the
// caller already read within the request scope. A chat the scope does
// not see is not named.
func ForwardOrigins(ctx context.Context, db *sql.DB, sc scope.Scope, msgScope string, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	table, fk := "telegram_group_message_fwd_info", "group_message_id"
	if msgScope == "private" {
		table, fk = "telegram_private_message_fwd_info", "private_message_id"
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rs, err := db.QueryContext(ctx, "SELECT fi."+fk+" AS mid, fi.origin_sender_name, "+
		"fi.origin_chat_id, ou.first_name, ou.last_name, oc.title "+
		"FROM "+table+" fi LEFT JOIN telegram_users ou ON ou.id = fi.origin_sender_user_id "+
		"LEFT JOIN `telegram_groups` oc ON oc.id = fi.origin_chat_id "+
		"WHERE fi."+fk+" IN ("+placeholders(len(ids))+")", args...)
	if err != nil {
		return nil, err
	}
	rows, err := scanRows(rs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name := r.Str("origin_sender_name")
		switch {
		case name != "":
		case r.Str("first_name") != "" || r.Str("last_name") != "":
			name = DisplayName(r.Str("first_name"), r.Str("last_name"))
		case !r.Null("origin_chat_id") && r.Str("title") != "" &&
			sc.SeesChat(r.Int("origin_chat_id")):
			name = r.Str("title")
		default:
			name = "unknown"
		}
		out[r.Uint("mid")] = name
	}
	return out, nil
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"tgloggerd/webapi/internal/scope"
)

// The change timelines of the user and group pages: the per-user and
// per-group history tables merged newest first, as userHistory in
// web/src/dao/Browse.cpp merges them, and paged by a cursor instead of
// an offset, so a long history costs the same on every page.

// HistoryEntry is one change on a user's or group's timeline.
type HistoryEntry struct {
	Kind   string // name, username, bio, phone, photo; title, description, admin
	At     int64
	Detail string
	// Action is a username event's (added, removed, reordered,
	// kind_changed, collectible_changed) or an admin change's (added,
	// removed, updated).
	Action string
	Photo  FileRef // a photo change; ID 0 = removed
	UserID int64   // an admin change's admin
	// UsernameKind and Collectible are a username's state after the
	// event (active or disabled; "" for a removal).
	UsernameKind string
	Collectible  bool
	// Status is an admin change's status at the event (creator,
	// administrator).
	Status string
	rowID  uint64 // the row's id in its own table, for the cursor
}

// HistoryPage is one page of a timeline.
type HistoryPage struct {
	Entries []HistoryEntry
	// Next is the cursor of the following page, "" at the end.
	Next string
}

// History page sizes: the pages' first page, and the most one request
// may ask for (Drogon's history page took up to 500).
const (
	HistoryPageSize = 100
	MaxHistoryLimit = 500
)

// ErrBadCursor is a history cursor that does not parse.
var ErrBadCursor = errors.New("bad cursor")

var (
	userHistoryKinds  = []string{"name", "bio", "phone", "username", "photo"}
	groupHistoryKinds = []string{"title", "description", "username", "photo", "admin"}
)

// historyCursor is the position just after e: "<unix time>.<kind>.<row
// id>", the timeline's sort key.
func historyCursor(e HistoryEntry) string {
	return strconv.FormatInt(e.At, 10) + "." + e.Kind + "." + strconv.FormatUint(e.rowID, 10)
}

type cursorPos struct {
	at   int64
	kind string
	id   uint64
}

func parseHistoryCursor(s string, kinds []string) (cursorPos, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return cursorPos{}, ErrBadCursor
	}
	at, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseUint(parts[2], 10, 64)
	if err1 != nil || err2 != nil || at < 0 {
		return cursorPos{}, ErrBadCursor
	}
	for _, k := range kinds {
		if k == parts[1] {
			return cursorPos{at: at, kind: k, id: id}, nil
		}
	}
	return cursorPos{}, ErrBadCursor
}

// The union's columns, in order: kind, id, created_at, detail, action,
// file_id, user_id, ukind (a username's kind, or an admin's status),
// collectible.
const userHistoryUnion = "" +
	"  SELECT 'name' AS kind, id, created_at, " +
	"         TRIM(CONCAT_WS(' ', first_name, last_name)) AS detail, " +
	"         NULL AS action, NULL AS file_id, 0 AS user_id, NULL AS ukind, " +
	"         NULL AS collectible " +
	"  FROM telegram_user_hist_name WHERE user_id = ? " +
	"  UNION ALL SELECT 'bio', id, created_at, bio, NULL, NULL, 0, NULL, NULL " +
	"  FROM telegram_user_hist_bio WHERE user_id = ? " +
	"  UNION ALL SELECT 'phone', id, created_at, phone_number, NULL, NULL, 0, NULL, NULL " +
	"  FROM telegram_user_hist_phone_num WHERE user_id = ? " +
	"  UNION ALL SELECT 'username', id, created_at, username, action, NULL, 0, kind, " +
	"         is_collectible " +
	"  FROM telegram_user_hist_usernames_events WHERE user_id = ? " +
	"  UNION ALL SELECT 'photo', id, created_at, NULL, NULL, file_id, 0, NULL, NULL " +
	"  FROM telegram_user_hist_profile_photo WHERE user_id = ? "

const groupHistoryUnion = "" +
	"  SELECT 'title' AS kind, id, created_at, title AS detail, NULL AS action, " +
	"         NULL AS file_id, 0 AS user_id, NULL AS ukind, NULL AS collectible " +
	"  FROM telegram_group_hist_title WHERE group_id = ? " +
	"  UNION ALL SELECT 'description', id, created_at, description, NULL, NULL, 0, " +
	"         NULL, NULL " +
	"  FROM telegram_group_hist_description WHERE group_id = ? " +
	"  UNION ALL SELECT 'username', id, created_at, username, action, NULL, 0, kind, " +
	"         is_collectible " +
	"  FROM telegram_group_hist_usernames_events WHERE group_id = ? " +
	"  UNION ALL SELECT 'photo', id, created_at, NULL, NULL, file_id, 0, NULL, NULL " +
	"  FROM telegram_group_hist_photo WHERE group_id = ? " +
	"  UNION ALL SELECT 'admin', id, created_at, " +
	"         IF(custom_title <> '', custom_title, status), action, NULL, user_id, " +
	"         status, NULL " +
	"  FROM telegram_group_admin_hist WHERE group_id = ? "

// history reads one page of a timeline: union is the merged tables, each
// taking id once; before is the cursor ("" for the newest); limit is
// clamped to 1..MaxHistoryLimit.
func history(ctx context.Context, db *sql.DB, union string, kinds []string, id int64,
	before string, limit int) (HistoryPage, error) {
	limit = min(max(limit, 1), MaxHistoryLimit)
	args := []any{id, id, id, id, id}
	where := ""
	if before != "" {
		c, err := parseHistoryCursor(before, kinds)
		if err != nil {
			return HistoryPage{}, err
		}
		// The sort is created_at DESC, kind, id DESC: what comes
		// after the cursor.
		where = "WHERE t.created_at < FROM_UNIXTIME(?) OR (t.created_at = FROM_UNIXTIME(?) " +
			"AND (t.kind > ? OR (t.kind = ? AND t.id < ?))) "
		args = append(args, c.at, c.at, c.kind, c.kind, c.id)
	}
	q := "SELECT t.kind, t.id, UNIX_TIMESTAMP(t.created_at), COALESCE(t.detail, ''), " +
		"COALESCE(t.action, ''), COALESCE(t.file_id, 0), " + fileColumnsOf("f") +
		", t.user_id, COALESCE(t.ukind, ''), COALESCE(t.collectible, 0) FROM (" + union +
		") t LEFT JOIN telegram_files f ON f.id = t.file_id " + where +
		"ORDER BY t.created_at DESC, t.kind, t.id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	p := HistoryPage{Entries: []HistoryEntry{}}
	for rows.Next() {
		var (
			e    HistoryEntry
			file uint64
			f    nullFile
			uk   string
		)
		dest := append([]any{&e.Kind, &e.rowID, &e.At, &e.Detail, &e.Action, &file}, f.dest()...)
		dest = append(dest, &e.UserID, &uk, &e.Collectible)
		if err := rows.Scan(dest...); err != nil {
			return HistoryPage{}, err
		}
		if file != 0 {
			e.Photo = f.ref(file)
		}
		switch e.Kind {
		case "username":
			e.UsernameKind = uk
		case "admin":
			e.Status = uk
		}
		p.Entries = append(p.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	if len(p.Entries) > limit {
		p.Entries = p.Entries[:limit]
		p.Next = historyCursor(p.Entries[limit-1])
	}
	return p, nil
}

// UserHistory is a page of the user's timeline: names, usernames
// (every event), bios, phone numbers and profile photos. Only for a
// viewer who sees users in full; the caller checks.
func UserHistory(ctx context.Context, db *sql.DB, id int64, before string, limit int) (HistoryPage, error) {
	return history(ctx, db, userHistoryUnion, userHistoryKinds, id, before, limit)
}

// GroupHistory is a page of the group's timeline: titles,
// descriptions, usernames (every event), photos and admin changes.
func GroupHistory(ctx context.Context, db *sql.DB, id int64, before string, limit int) (HistoryPage, error) {
	return history(ctx, db, groupHistoryUnion, groupHistoryKinds, id, before, limit)
}

// ValidUserCursor is whether s is a cursor of a user's timeline.
func ValidUserCursor(s string) bool {
	_, err := parseHistoryCursor(s, userHistoryKinds)
	return err == nil
}

// ValidGroupCursor is whether s is a cursor of a group's timeline.
func ValidGroupCursor(s string) bool {
	_, err := parseHistoryCursor(s, groupHistoryKinds)
	return err == nil
}

// HistoryUsers are the users a group's timeline names (its admin
// changes); they appear in the group, so its viewer sees them.
func HistoryUsers(ctx context.Context, db *sql.DB, sc scope.Scope, h HistoryPage) (map[int64]User, error) {
	var ids []int64
	for _, e := range h.Entries {
		if e.Kind == "admin" {
			ids = append(ids, e.UserID)
		}
	}
	return usersByID(ctx, db, sc, ids)
}

// RowID is the entry's id in its own history table; for a photo
// change, the photo's id in UserPhotos or GroupPhotos.
func (e HistoryEntry) RowID() uint64 {
	return e.rowID
}

// MaxPhotos is the most photos UserPhotos and GroupPhotos read.
const MaxPhotos = 500

// Photo is one recorded profile photo of a user, or photo of a group.
type Photo struct {
	ID   uint64 // the history table's row
	At   int64
	File FileRef
}

// photos are the recorded photos in a photo history table (its owner
// column owner), newest first (as the timeline orders them), at most
// MaxPhotos; the removals (rows without a file) are left out.
func photos(ctx context.Context, db *sql.DB, table, owner string, id int64) ([]Photo, error) {
	rows, err := db.QueryContext(ctx, "SELECT p.id, UNIX_TIMESTAMP(p.created_at), p.file_id, "+
		fileColumnsOf("f")+" FROM "+table+" p "+
		"LEFT JOIN telegram_files f ON f.id = p.file_id "+
		"WHERE p."+owner+" = ? AND p.file_id IS NOT NULL "+
		"ORDER BY p.created_at DESC, p.id DESC LIMIT ?", id, MaxPhotos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		var (
			p    Photo
			file uint64
			f    nullFile
		)
		if err := rows.Scan(append([]any{&p.ID, &p.At, &file}, f.dest()...)...); err != nil {
			return nil, err
		}
		p.File = f.ref(file)
		out = append(out, p)
	}
	return out, rows.Err()
}

// UserPhotos are the user's recorded profile photos (see photos). Only
// for a viewer who sees users in full; the caller checks.
func UserPhotos(ctx context.Context, db *sql.DB, id int64) ([]Photo, error) {
	return photos(ctx, db, "telegram_user_hist_profile_photo", "user_id", id)
}

// GroupPhotos are the group's recorded photos (see photos). Only for a
// group the viewer's scope sees; the caller checks.
func GroupPhotos(ctx context.Context, db *sql.DB, id int64) ([]Photo, error) {
	return photos(ctx, db, "telegram_group_hist_photo", "group_id", id)
}

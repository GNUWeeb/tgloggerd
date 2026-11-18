// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// An account's pinned groups and saved searches (web-accounts.md
// section 7). Which pinned groups it may see is the request scope's
// business (logdb.PinnedGroups); the pins are kept as they were made.

// MaxPins and MaxSavedSearches bound what one account keeps, so its
// menu and its preferences page stay small.
const (
	MaxPins          = 50
	MaxSavedSearches = 100
)

// ErrLimit: the account has as many as it may keep.
var ErrLimit = errors.New("limit reached")

// Pins are the ids of the account's pinned groups, oldest pin first.
func Pins(ctx context.Context, db *sql.DB, uid uint64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, "SELECT group_id FROM web_pinned_groups "+
		"WHERE web_user_id = ? ORDER BY pinned_at, group_id", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Pin pins a group, keeping its place when it is pinned already. The
// account's row is locked first, so two pins at once cannot pass the
// limit together.
func Pin(ctx context.Context, db *sql.DB, uid uint64, group int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT id FROM web_users WHERE id = ? FOR UPDATE", uid).
		Scan(&n); err != nil {
		return err
	}
	var have, pinned int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(group_id = ?), 0) "+
		"FROM web_pinned_groups WHERE web_user_id = ?", group, uid).Scan(&have, &pinned); err != nil {
		return err
	}
	if pinned > 0 {
		return nil
	}
	if have >= MaxPins {
		return ErrLimit
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO web_pinned_groups (web_user_id, group_id) "+
		"VALUES (?, ?)", uid, group); err != nil {
		return err
	}
	return tx.Commit()
}

// Unpin removes a pin; it reports false when there was none.
func Unpin(ctx context.Context, db *sql.DB, uid uint64, group int64) (bool, error) {
	res, err := db.ExecContext(ctx, "DELETE FROM web_pinned_groups "+
		"WHERE web_user_id = ? AND group_id = ?", uid, group)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SavedSearch is one saved search.
type SavedSearch struct {
	ID      uint64    `json:"id"`
	Title   string    `json:"title"`
	URL     string    `json:"url"`
	Created time.Time `json:"-"`
}

// The lengths of the saved searches' columns: the title in characters,
// the URL in bytes (it is a path and query string, mostly ASCII).
const (
	MaxSearchTitle = 128
	MaxSearchURL   = 2048
)

// SavedSearches are the account's saved searches, oldest first.
func SavedSearches(ctx context.Context, db *sql.DB, uid uint64) ([]SavedSearch, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, title, url, UNIX_TIMESTAMP(created_at) "+
		"FROM web_saved_searches WHERE web_user_id = ? ORDER BY id", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedSearch{}
	for rows.Next() {
		var (
			s  SavedSearch
			at float64
		)
		if err := rows.Scan(&s.ID, &s.Title, &s.URL, &at); err != nil {
			return nil, err
		}
		s.Created = time.Unix(int64(at), 0).UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}

// SaveSearch stores a search the caller has checked and returns its
// id; ErrLimit when the account keeps MaxSavedSearches already.
func SaveSearch(ctx context.Context, db *sql.DB, uid uint64, title, url string) (uint64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT id FROM web_users WHERE id = ? FOR UPDATE", uid).
		Scan(&n); err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM web_saved_searches "+
		"WHERE web_user_id = ?", uid).Scan(&n); err != nil {
		return 0, err
	}
	if n >= MaxSavedSearches {
		return 0, ErrLimit
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO web_saved_searches (web_user_id, title, url) "+
		"VALUES (?, ?, ?)", uid, strings.TrimSpace(title), url)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), tx.Commit()
}

// DeleteSearch removes one of the account's saved searches; it reports
// false when the account has none with this id.
func DeleteSearch(ctx context.Context, db *sql.DB, uid, id uint64) (bool, error) {
	res, err := db.ExecContext(ctx, "DELETE FROM web_saved_searches "+
		"WHERE id = ? AND web_user_id = ?", id, uid)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

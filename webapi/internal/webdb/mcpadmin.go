// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The MCP bearer tokens (web_mcp_tokens, migration 000003; only their
// SHA-256 is stored), as the Drogon web's dao::mcp keeps them.

// MCPToken is a web_mcp_tokens row, without its digest.
type MCPToken struct {
	ID       uint64
	Name     string
	Owner    string // the owning account's username
	Created  time.Time
	LastUsed *time.Time
	Revoked  *time.Time
}

func unixPtr(v sql.NullFloat64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(int64(v.Float64), 0).UTC()
	return &t
}

// ListMCPTokens is every token, newest first.
func ListMCPTokens(ctx context.Context, db *sql.DB) ([]MCPToken, error) {
	rows, err := db.QueryContext(ctx, "SELECT t.id, t.name, u.username, "+
		"UNIX_TIMESTAMP(t.created_at), UNIX_TIMESTAMP(t.last_used_at), "+
		"UNIX_TIMESTAMP(t.revoked_at) FROM web_mcp_tokens t "+
		"JOIN web_users u ON u.id = t.web_user_id ORDER BY t.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MCPToken{}
	for rows.Next() {
		var (
			t             MCPToken
			created       float64
			used, revoked sql.NullFloat64
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.Owner, &created, &used, &revoked); err != nil {
			return nil, err
		}
		t.Created = time.Unix(int64(created), 0).UTC()
		t.LastUsed, t.Revoked = unixPtr(used), unixPtr(revoked)
		out = append(out, t)
	}
	return out, rows.Err()
}

// MCPTokenStats counts the tokens not revoked, and the newest use of
// one of them.
func MCPTokenStats(ctx context.Context, db *sql.DB) (active int, lastUsed *time.Time, err error) {
	var used sql.NullFloat64
	err = db.QueryRowContext(ctx, "SELECT COUNT(*), UNIX_TIMESTAMP(MAX(last_used_at)) "+
		"FROM web_mcp_tokens WHERE revoked_at IS NULL").Scan(&active, &used)
	return active, unixPtr(used), err
}

// CreateMCPToken stores a token by its digest; the token itself is
// never passed here, so it cannot be stored by accident.
func CreateMCPToken(ctx context.Context, db *sql.DB, uid uint64, name string, sha256 []byte) (uint64, error) {
	res, err := db.ExecContext(ctx, "INSERT INTO web_mcp_tokens (web_user_id, name, token_sha256) "+
		"VALUES (?, ?, ?)", uid, name, sha256)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

// MCPTokenName is a token's name, or ErrNoToken.
func MCPTokenName(ctx context.Context, db *sql.DB, id uint64) (string, error) {
	var name string
	err := db.QueryRowContext(ctx, "SELECT name FROM web_mcp_tokens WHERE id = ?", id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoToken
	}
	return name, err
}

// ErrNoToken is returned when no token has the id.
var ErrNoToken = errors.New("no such token")

// RevokeMCPToken stamps revoked_at, so the row stays auditable; true
// when it was not revoked before.
func RevokeMCPToken(ctx context.Context, db *sql.DB, id uint64) (bool, error) {
	res, err := db.ExecContext(ctx, "UPDATE web_mcp_tokens SET revoked_at = NOW() "+
		"WHERE id = ? AND revoked_at IS NULL", id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
)

// The ids of every resource of the logger's schema that the web gives
// an owner (web-accounts.md section 6), for the assign-owner command.
// It acts for no request, so unlike every other read here these take
// no scope: they are never answered to a viewer.

func allIDs(ctx context.Context, db *sql.DB, q string) ([]int64, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AllExposedGroupIDs is every group on the MCP allowlist.
func AllExposedGroupIDs(ctx context.Context, db *sql.DB) ([]int64, error) {
	return allIDs(ctx, db, "SELECT group_id FROM telegram_public_groups ORDER BY group_id")
}

// AllTelegramBotIDs is every Telegram bot, disabled ones too.
func AllTelegramBotIDs(ctx context.Context, db *sql.DB) ([]int64, error) {
	return allIDs(ctx, db, "SELECT id FROM telegram_bots ORDER BY id")
}

// AllT2DIDs is every Telegram to Discord webhook.
func AllT2DIDs(ctx context.Context, db *sql.DB) ([]int64, error) {
	return allIDs(ctx, db, "SELECT id FROM telegram_discord_webhooks ORDER BY id")
}

// AllD2TIDs is every Discord to Telegram route.
func AllD2TIDs(ctx context.Context, db *sql.DB) ([]int64, error) {
	return allIDs(ctx, db, "SELECT id FROM discord_telegram_routes ORDER BY id")
}

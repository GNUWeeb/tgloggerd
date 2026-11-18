// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"

	"tgloggerd/webapi/internal/scope"
)

// The numbers of the dashboard's logger status card that come from the
// logger's database. The card is only for a viewer that sees every
// group and private chat, so these read the whole archive.

// LastMessageAt is when the newest stored message (group or private)
// was logged, in Unix seconds; 0 when there is none. Each table's
// newest row is found by primary key: created_at has no index, and a
// scan over it would read the whole table.
func LastMessageAt(ctx context.Context, db *sql.DB) (int64, error) {
	var last int64
	for _, t := range []string{"telegram_group_messages", "telegram_private_messages"} {
		var at sql.NullInt64
		err := db.QueryRowContext(ctx, "SELECT UNIX_TIMESTAMP(created_at) FROM "+t+
			" ORDER BY id DESC LIMIT 1").Scan(&at)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		if at.Int64 > last {
			last = at.Int64
		}
	}
	return last, nil
}

// ActiveForwarding is how many forwarding configurations the scope
// sees that forward now, as the forwarding index counts "active": the
// enabled Telegram-to-Discord webhooks, and the enabled Discord-to-
// Telegram routes whose Telegram and Discord bots are both enabled.
// Through the count cache.
func ActiveForwarding(ctx context.Context, db *sql.DB, sc scope.Scope) (int64, error) {
	wf, wa := ChatFilter(sc, "w.telegram_chat_id")
	t2d, err := countInt(ctx, db, "SELECT COUNT(*) FROM telegram_discord_webhooks w "+
		"WHERE w.enabled = 1 AND "+wf, wa...)
	if err != nil {
		return 0, err
	}
	rf, ra := ChatFilter(sc, "r.telegram_chat_id")
	d2t, err := countInt(ctx, db, "SELECT COUNT(*) FROM discord_telegram_routes r "+
		"JOIN telegram_bots tb ON tb.id = r.telegram_bot_id AND tb.enabled = 1 "+
		"JOIN discord_bots dbt ON dbt.id = r.discord_bot_id AND dbt.enabled = 1 "+
		"WHERE r.enabled = 1 AND "+rf, ra...)
	if err != nil {
		return 0, err
	}
	return t2d + d2t, nil
}

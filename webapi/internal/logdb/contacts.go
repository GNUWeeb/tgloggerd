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

// ContactOf is whether a user the scope sees exists, and whether the
// logging account has it as a contact (telegram_users.is_contact,
// migration 000033; NULL, not seen since, reads as no contact), as
// "Look up real name" checks before asking the daemon.
func ContactOf(ctx context.Context, db *sql.DB, sc scope.Scope, id int64) (exists, contact bool, err error) {
	if id <= 0 || !sc.SeesUser(id) {
		return false, false, nil
	}
	var c sql.NullBool
	err = db.QueryRowContext(ctx, "SELECT is_contact FROM telegram_users WHERE id = ?", id).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, c.Valid && c.Bool, nil
}

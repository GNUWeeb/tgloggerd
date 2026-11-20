// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/webdb"
)

// webapi import-discord-bot <username> (web/docs/platform-forwarding.md,
// Bots): run once at the deployment that moves the Discord bot token
// from discordd's environment into the database
// (decisions/2026-11-18-ownership-and-bots.md). It stores the bot of
// the token in DISCORD_BOT_TOKEN, owned by the account named (an
// active superadmin), and points every Discord to Telegram route that
// names no Discord bot at it. A second run finds the bot stored and
// every route pointed, and changes nothing. The token is never printed.
//
// It is the web service's command rather than discordd's because the
// owner lives in the web's schema, which only the web service reaches,
// and because its pools have the grants on the bots table already.

// importedBotName is the stored bot's name; its owner may rename it.
const importedBotName = "Discord bot"

// importDiscordBot imports token as above and writes what it did to
// out.
func importDiscordBot(ctx context.Context, ro, app *sql.DB, username, token string, out io.Writer) error {
	if token == "" {
		return errRefused{"DISCORD_BOT_TOKEN is not set: nothing to import"}
	}
	if !logdb.ValidDiscordToken(token) {
		return errRefused{"DISCORD_BOT_TOKEN does not look like a Discord bot token"}
	}
	u, err := webdb.FindByUsername(ctx, app, username)
	switch {
	case errors.Is(err, webdb.ErrNoUser):
		return errRefused{fmt.Sprintf("no account is named %q", username)}
	case err != nil:
		return err
	case !u.IsActive:
		return errRefused{fmt.Sprintf("the account %q is disabled", u.Username)}
	case u.Role != "superadmin":
		return errRefused{fmt.Sprintf("the account %q is not a superadmin (role %s)", u.Username, u.Role)}
	}
	id, created, routes, err := logdb.ImportDiscordBot(ctx, ro, importedBotName, token)
	if err != nil {
		return err
	}
	res := webdb.Resource{Type: webdb.ResDiscordBot, ID: int64(id)}
	owner, owned, err := webdb.OwnerOf(ctx, app, res)
	if err != nil {
		return err
	}
	if !owned {
		if err := webdb.SetOwner(ctx, app, res, u.ID, webdb.Deployment); err != nil {
			return err
		}
		owner = webdb.OwnerInfo{ID: u.ID, Username: u.Username}
	}
	if created {
		fmt.Fprintf(out, "Discord bot %d stored as %q\n", id, importedBotName)
	} else {
		fmt.Fprintf(out, "Discord bot %d stored already\n", id)
	}
	if owned {
		fmt.Fprintf(out, "Owner: %s (id %d), as before\n", owner.Username, owner.ID)
	} else {
		fmt.Fprintf(out, "Owner: %s (id %d), given now\n", owner.Username, owner.ID)
	}
	fmt.Fprintf(out, "Routes pointed at it: %d\n", routes)
	if !created && owned && routes == 0 {
		fmt.Fprintln(out, "Nothing to do.")
	}
	return nil
}

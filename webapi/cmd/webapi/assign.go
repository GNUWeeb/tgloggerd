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
	"slices"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/webdb"
)

// webapi assign-owner <username> (web/docs/accounts.md, Ownership):
// gives every resource that has no owner to one account, the first
// superadmin, once, at the deployment that introduces ownership
// (decisions/2026-11-18-ownership-and-bots.md). MCP tokens are the
// exception: each goes to the account that minted it, which every
// token records. Resources that have an
// owner are left alone, so a second run assigns nothing; each resource
// assigned gets its owner_set row in web_audit, by no account (the
// command) and with the owner as its target.

// ownable is where the ids of each resource type come from: the
// logger's schema (ro) or the web's (app). The Discord bots get their
// line with their table (discord-bots-per-user).
// creators, when set, names the owner of each resource instead of the
// account the command is given.
var ownable = []struct {
	t        webdb.ResourceType
	list     func(ctx context.Context, ro, app *sql.DB) ([]int64, error)
	creators func(ctx context.Context, app *sql.DB) (map[int64]uint64, error)
}{
	{webdb.ResMCPGroup, func(ctx context.Context, ro, _ *sql.DB) ([]int64, error) {
		return logdb.AllExposedGroupIDs(ctx, ro)
	}, nil},
	{webdb.ResMCPToken, func(ctx context.Context, _, app *sql.DB) ([]int64, error) {
		return webdb.AllMCPTokenIDs(ctx, app)
	}, webdb.MCPTokenCreators},
	{webdb.ResTelegramBot, func(ctx context.Context, ro, _ *sql.DB) ([]int64, error) {
		return logdb.AllTelegramBotIDs(ctx, ro)
	}, nil},
	{webdb.ResT2DWebhook, func(ctx context.Context, ro, _ *sql.DB) ([]int64, error) {
		return logdb.AllT2DIDs(ctx, ro)
	}, nil},
	{webdb.ResD2TRoute, func(ctx context.Context, ro, _ *sql.DB) ([]int64, error) {
		return logdb.AllD2TIDs(ctx, ro)
	}, nil},
}

// errRefused is an account the command will not assign to; its text is
// the reason.
type errRefused struct{ reason string }

func (e errRefused) Error() string { return e.reason }

// assignOwner gives every unowned resource to the account named
// username, which must be an active superadmin, and writes what it
// assigned, by type, to out.
func assignOwner(ctx context.Context, ro, app *sql.DB, username string, out io.Writer) error {
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
	fmt.Fprintf(out, "Owner: %s (id %d)\n", u.Username, u.ID)
	total := 0
	for _, o := range ownable {
		ids, err := o.list(ctx, ro, app)
		if err != nil {
			return fmt.Errorf("list %s: %w", o.t, err)
		}
		// Each owner's resources are assigned together.
		byOwner := map[uint64][]int64{u.ID: ids}
		label := "assigned"
		if o.creators != nil {
			made, err := o.creators(ctx, app)
			if err != nil {
				return fmt.Errorf("creators of %s: %w", o.t, err)
			}
			byOwner = map[uint64][]int64{}
			for _, id := range ids {
				owner := made[id]
				if owner == 0 {
					owner = u.ID
				}
				byOwner[owner] = append(byOwner[owner], id)
			}
			label = "assigned to their creators"
		}
		owners := make([]uint64, 0, len(byOwner))
		for owner := range byOwner {
			owners = append(owners, owner)
		}
		slices.Sort(owners)
		var given []int64
		for _, owner := range owners {
			g, err := webdb.AssignUnowned(ctx, app, o.t, byOwner[owner], owner, webdb.Deployment)
			if err != nil {
				return fmt.Errorf("assign %s: %w", o.t, err)
			}
			given = append(given, g...)
		}
		slices.Sort(given)
		total += len(given)
		line := fmt.Sprintf("%-13s %d %s", o.t, len(given), label)
		if len(given) > 0 {
			s := make([]string, len(given))
			for i, id := range given {
				s[i] = strconv.FormatInt(id, 10)
			}
			line += ": " + strings.Join(s, " ")
		}
		fmt.Fprintf(out, "%s; %d owned already\n", line, len(ids)-len(given))
	}
	fmt.Fprintf(out, "%-13s not stored yet\n", webdb.ResDiscordBot)
	fmt.Fprintf(out, "Assigned %d resources.\n", total)
	return nil
}

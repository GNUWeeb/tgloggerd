// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"encoding/json"

	"tgloggerd/webapi/internal/scope"
)

// What the chat's message kinds read beyond the message rows
// (docs/design/message-types.md sections 4.2 to 4.5): the polls the
// messages show, and the users and groups their detail names. Each is
// read in one batch per page of messages.

// Poll is a poll as last seen (telegram_polls), with its options that
// are still in it, in order.
type Poll struct {
	ID          int64
	Question    string
	Type        string // regular, quiz
	Anonymous   bool
	Multiple    bool
	Closed      bool
	TotalVoters int64
	CloseDate   int64  // unix; 0 = none
	Explanation string // a quiz's; "" = none
	// Correct are a quiz's correct options, by their 0-based position.
	Correct []int
	Options []PollOption
}

// PollOption is one option of a poll.
type PollOption struct {
	Position int
	Text     string
	Voters   int64
	Percent  int64
}

// PollsByID are the stored polls among ids; missing ones are left
// out. Removed options (deleted_at set) are not read.
func PollsByID(ctx context.Context, db *sql.DB, ids []int64) (map[int64]*Poll, error) {
	ids = uniq(ids)
	out := map[int64]*Poll{}
	if len(ids) == 0 {
		return out, nil
	}
	in := placeholders(len(ids))
	rows, err := db.QueryContext(ctx, "SELECT id, question, type, is_anonymous, "+
		"allows_multiple_answers, is_closed, total_voter_count, close_date, "+
		"COALESCE(explanation, ''), correct_option_ids FROM telegram_polls WHERE id IN ("+in+")",
		int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			p       Poll
			correct []byte
		)
		if err := rows.Scan(&p.ID, &p.Question, &p.Type, &p.Anonymous, &p.Multiple, &p.Closed,
			&p.TotalVoters, &p.CloseDate, &p.Explanation, &correct); err != nil {
			rows.Close()
			return nil, err
		}
		if len(correct) > 0 {
			_ = json.Unmarshal(correct, &p.Correct)
		}
		out[p.ID] = &p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	rows, err = db.QueryContext(ctx, "SELECT poll_id, position, text, voter_count, "+
		"vote_percentage FROM telegram_poll_options WHERE poll_id IN ("+in+") AND "+
		"deleted_at IS NULL ORDER BY poll_id, position", int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id int64
			o  PollOption
		)
		if err := rows.Scan(&id, &o.Position, &o.Text, &o.Voters, &o.Percent); err != nil {
			return nil, err
		}
		if p := out[id]; p != nil {
			p.Options = append(p.Options, o)
		}
	}
	return out, rows.Err()
}

// Refs are the users and groups among these ids that sc sees, for the
// detail of messages that name them (a contact's user, who completed a
// checklist task, a call's participants, a service message's people):
// users as VisibleUsers answers them, groups that exist and sc sees,
// and the group chat itself, which the request is reading. Missing and
// hidden ones are left out.
func Refs(ctx context.Context, db *sql.DB, sc scope.Scope, chat int64, users, groups []int64) (map[int64]User, map[int64]Group, error) {
	us, err := VisibleUsers(ctx, db, sc, users)
	if err != nil {
		return nil, nil, err
	}
	um := make(map[int64]User, len(us))
	for _, u := range us {
		um[u.ID] = u
	}
	var want []int64
	for _, g := range uniq(groups) {
		if g == chat || sc.SeesChat(g) {
			want = append(want, g)
		}
	}
	gm, err := groupsByID(ctx, db, want)
	if err != nil {
		return nil, nil, err
	}
	return um, gm, nil
}

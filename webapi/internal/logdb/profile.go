// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
)

// The profile of the user page and the usernames of the user and group
// pages: every field getUser and getGroup in web/src/dao/Browse.cpp
// read, for a viewer who sees the person in full.

// Username is one of a user's or group's current usernames.
type Username struct {
	Username string
	Active   bool
	// Kind is active or disabled; a username can be collectible
	// (bought on fragment.com) either way.
	Kind        string
	Collectible bool
}

// Birthday is a user's birthday; Year 0 when hidden.
type Birthday struct {
	Day, Month, Year int
}

// Profile is what telegram_users and telegram_user_extra_info hold
// about a person beyond its summary. Zero values are "none", as the
// logger stores them: AccentColor 0 is the default colour,
// ProfileAccentColor -1 none, an emoji id 0 none.
type Profile struct {
	Bio, Phone          string
	RestrictionReason   string
	HasSensitiveContent bool
	RestrictsNewChats   bool
	PaidMessageStars    int64
	PersonalChatID      int64 // 0 = none
	AccentColor         int
	ProfileAccentColor  int
	BackgroundEmoji     int64
	ProfileBgEmoji      int64
	EmojiStatus         int64 // custom emoji id; 0 = none
	EmojiStatusExpires  int64 // unix; 0 = does not expire
	Birthday            *Birthday
	IsContact           bool
	IsMutualContact     bool
	OwnNameKnown        bool
	OwnNameCheckedAt    int64 // unix; 0 = never looked up
	LookupPending       bool  // a real-name lookup has not added the contact back
	LookupPendingSince  int64
	LookupPendingError  string
}

func loadProfile(ctx context.Context, db *sql.DB, id int64) (Profile, error) {
	var (
		p                Profile
		personal, status sql.NullInt64
		expires          sql.NullInt64
		bd, bm, by       sql.NullInt64
		contact          sql.NullBool
		checked          sql.NullInt64
	)
	err := db.QueryRowContext(ctx, "SELECT COALESCE(e.bio, ''), COALESCE(e.phone_number, ''), "+
		"COALESCE(e.restriction_reason, ''), COALESCE(e.has_sensitive_content, 0), "+
		"COALESCE(e.restricts_new_chats, 0), COALESCE(e.paid_message_star_count, 0), "+
		"e.personal_chat_id, u.accent_color_id, COALESCE(e.profile_accent_color_id, -1), "+
		"COALESCE(e.background_custom_emoji_id, 0), "+
		"COALESCE(e.profile_background_custom_emoji_id, 0), "+
		"e.emoji_status_custom_emoji_id, e.emoji_status_expiration_date, "+
		"u.birthday_day, u.birthday_month, u.birthday_year, u.is_contact, "+
		"u.is_mutual_contact, u.own_name_known, UNIX_TIMESTAMP(u.own_name_checked_at) "+
		"FROM telegram_users u LEFT JOIN telegram_user_extra_info e ON e.user_id = u.id "+
		"WHERE u.id = ?", id).Scan(&p.Bio, &p.Phone, &p.RestrictionReason,
		&p.HasSensitiveContent, &p.RestrictsNewChats, &p.PaidMessageStars, &personal,
		&p.AccentColor, &p.ProfileAccentColor, &p.BackgroundEmoji, &p.ProfileBgEmoji,
		&status, &expires, &bd, &bm, &by, &contact, &p.IsMutualContact, &p.OwnNameKnown,
		&checked)
	if err != nil {
		return Profile{}, err
	}
	p.PersonalChatID, p.EmojiStatus = personal.Int64, status.Int64
	if status.Valid && expires.Int64 > 0 {
		p.EmojiStatusExpires = expires.Int64
	}
	if bd.Valid && bm.Valid {
		p.Birthday = &Birthday{Day: int(bd.Int64), Month: int(bm.Int64), Year: int(by.Int64)}
	}
	// NULL until the daemon has seen the user since contacts were
	// recorded apart, which reads as no contact.
	p.IsContact = contact.Valid && contact.Bool
	p.OwnNameCheckedAt = checked.Int64

	err = db.QueryRowContext(ctx, "SELECT UNIX_TIMESTAMP(created_at), last_error "+
		"FROM telegram_contact_lookups WHERE user_id = ?", id).Scan(&p.LookupPendingSince,
		&p.LookupPendingError)
	switch {
	case err == nil:
		p.LookupPending = true
	case err != sql.ErrNoRows:
		return Profile{}, err
	}
	return p, nil
}

// usernames are the current usernames of a user (table
// telegram_user_usernames, column user_id) or a group, active ones
// first, each list in its order.
func usernames(ctx context.Context, db *sql.DB, table, column string, id int64) ([]Username, error) {
	rows, err := db.QueryContext(ctx, "SELECT username, kind, is_collectible FROM "+table+
		" WHERE "+column+" = ? ORDER BY kind, position", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Username{}
	for rows.Next() {
		var u Username
		if err := rows.Scan(&u.Username, &u.Kind, &u.Collectible); err != nil {
			return nil, err
		}
		u.Active = u.Kind == "active"
		out = append(out, u)
	}
	return out, rows.Err()
}

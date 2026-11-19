// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"tgloggerd/webapi/internal/scope"
)

// The data of the user, group, message and dashboard pages, from the
// same tables the Drogon pages read (web/src/dao/Browse.cpp: getUser,
// userHistory, getGroup, getMessage, counts), each through the scope.

// Days is the length of the activity series the pages chart.
const Days = 30

const day = 86400

// dayStart is the start of the UTC day holding the unix time now.
func dayStart(now int64) int64 {
	return now - now%day
}

// perDay turns (day index, count) rows into the Days-long series ending
// with the day of now, oldest first.
func perDay(ctx context.Context, db *sql.DB, now int64, q string, args ...any) ([]int64, error) {
	start := dayStart(now) - (Days-1)*day
	out := make([]int64, Days)
	rows, err := db.QueryContext(ctx, q, append([]any{start, start}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d, n int64
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		if d >= 0 && d < Days {
			out[d] += n
		}
	}
	return out, rows.Err()
}

// localDays are the starts of the Days calendar days in loc that end
// with the day holding now, oldest first, and the start of the day
// after them: Days+1 unix times. A day in a zone with daylight saving
// time can be 23 or 25 hours long.
func localDays(now int64, loc *time.Location) []int64 {
	t := time.Unix(now, 0).In(loc)
	y, m, d := t.Date()
	out := make([]int64, Days+1)
	for i := range out {
		out[i] = time.Date(y, m, d-(Days-1)+i, 0, 0, 0, 0, loc).Unix()
	}
	return out
}

// dayIndex is the SELECT head of a perDay query over a table aliased m
// with a unix date column, whose WHERE starts with "m.date >= ?"; perDay
// passes the series' start to both.
const dayIndex = "SELECT FLOOR((m.date - ?) / 86400), COUNT(*) "

// UserDetail is the user page's person: the summary and the profile a
// full-access viewer sees.
type UserDetail struct {
	User
	Profile
	Usernames []Username
}

// GroupCount is a group with the number of messages of someone in it.
type GroupCount struct {
	Group
	Msgs int64
}

// UserPage is the data of the user pages.
type UserPage struct {
	// Limited: the viewer sees the person only through its groups;
	// only User and Groups (without counts) are filled.
	Limited  bool
	User     UserDetail
	Groups   []GroupCount
	History  HistoryPage
	Activity []int64
}

// LoadUserPage is the user page of id, within the scope; now is the
// server's clock (unix), for the activity series, and before the
// history's cursor ("" for the newest changes).
func LoadUserPage(ctx context.Context, db *sql.DB, sc scope.Scope, id, now int64, before string) (UserPage, error) {
	us, err := VisibleUsers(ctx, db, sc, []int64{id})
	if err != nil {
		return UserPage{}, err
	}
	if len(us) == 0 {
		return UserPage{}, ErrNotFound
	}
	p := UserPage{User: UserDetail{User: us[0]}, Limited: !sc.SeesAllUsers()}

	cond, cargs := sc.GroupFilter("m.chat_id")
	rows, err := db.QueryContext(ctx, "SELECT m.chat_id, COUNT(*) FROM telegram_group_messages m "+
		"WHERE m.sender_user_id = ? AND "+cond+" GROUP BY m.chat_id ORDER BY COUNT(*) DESC, "+
		"m.chat_id LIMIT 100", append([]any{id}, cargs...)...)
	if err != nil {
		return UserPage{}, err
	}
	var ids []int64
	counts := map[int64]int64{}
	for rows.Next() {
		var g, n int64
		if err := rows.Scan(&g, &n); err != nil {
			rows.Close()
			return UserPage{}, err
		}
		ids = append(ids, g)
		counts[g] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return UserPage{}, err
	}
	gs, err := groupsByID(ctx, db, ids)
	if err != nil {
		return UserPage{}, err
	}
	for _, g := range inOrder(ids, gs) {
		p.Groups = append(p.Groups, GroupCount{Group: g, Msgs: counts[g.ID]})
	}
	if p.Limited {
		return p, nil
	}

	if p.User.Profile, err = loadProfile(ctx, db, id); err != nil {
		return UserPage{}, err
	}
	if p.User.Usernames, err = usernames(ctx, db, "telegram_user_usernames", "user_id", id); err != nil {
		return UserPage{}, err
	}

	if p.History, err = UserHistory(ctx, db, id, before, HistoryPageSize); err != nil {
		return UserPage{}, err
	}

	// Activity: the person's messages in visible groups, and in
	// private chats when the viewer sees those.
	q := dayIndex + "FROM telegram_group_messages m WHERE m.date >= ? AND m.sender_user_id = ? AND " +
		cond + " GROUP BY 1"
	if p.Activity, err = perDay(ctx, db, now, q, append([]any{id}, cargs...)...); err != nil {
		return UserPage{}, err
	}
	if sc.SeesPrivate() {
		priv, err := perDay(ctx, db, now, dayIndex+"FROM telegram_private_messages m "+
			"WHERE m.date >= ? AND m.sender_id = ? GROUP BY 1", id)
		if err != nil {
			return UserPage{}, err
		}
		for i := range p.Activity {
			p.Activity[i] += priv[i]
		}
	}
	return p, nil
}

// GroupDetail is the group page's group.
type GroupDetail struct {
	Group
	AdminCount int64
	Usernames  []Username
}

// Admin is one of a group's current administrators.
type Admin struct {
	UserID      int64
	Status      string // creator, administrator
	CustomTitle string
	IsAnonymous bool
	Perms       []string
	Since       int64
}

// Poster is a group member with its message count.
type Poster struct {
	UserID int64
	Msgs   int64
}

// GroupPage is the data of the group pages.
type GroupPage struct {
	Group      GroupDetail
	Admins     []Admin
	History    HistoryPage
	TopPosters []Poster
	Activity   []int64
	// Users are the users the admins, the history and the posters
	// name; they appear in the group, so its viewer sees them.
	Users map[int64]User
}

// adminPerms are the permission columns of telegram_group_admins and
// their labels, as Browse.cpp's kPerms.
var adminPerms = []struct{ col, label string }{
	{"can_manage_chat", "manage chat"},
	{"can_change_info", "change info"},
	{"can_post_messages", "post messages"},
	{"can_edit_messages", "edit messages"},
	{"can_delete_messages", "delete messages"},
	{"can_invite_users", "invite users"},
	{"can_restrict_members", "restrict members"},
	{"can_pin_messages", "pin messages"},
	{"can_manage_topics", "manage topics"},
	{"can_promote_members", "promote members"},
	{"can_manage_video_chats", "manage video chats"},
	{"can_post_stories", "post stories"},
	{"can_edit_stories", "edit stories"},
	{"can_delete_stories", "delete stories"},
	{"can_manage_direct_messages", "manage direct messages"},
	{"can_manage_tags", "manage tags"},
}

// topPosters is how many posters the group page lists.
const topPosters = 8

// LoadGroupPage is the group page of id, within the scope; before is
// the history's cursor ("" for the newest changes).
func LoadGroupPage(ctx context.Context, db *sql.DB, sc scope.Scope, id, now int64, before string) (GroupPage, error) {
	gs, err := VisibleGroups(ctx, db, sc, []int64{id})
	if err != nil {
		return GroupPage{}, err
	}
	if len(gs) == 0 {
		return GroupPage{}, ErrNotFound
	}
	p := GroupPage{Group: GroupDetail{Group: gs[0]}}

	cols := make([]string, len(adminPerms))
	for i, pm := range adminPerms {
		cols[i] = pm.col
	}
	rows, err := db.QueryContext(ctx, "SELECT user_id, status, custom_title, is_anonymous, "+
		"IF(joined_date > 0, joined_date, UNIX_TIMESTAMP(created_at)), "+strings.Join(cols, ", ")+
		" FROM telegram_group_admins WHERE group_id = ? ORDER BY status, user_id", id)
	if err != nil {
		return GroupPage{}, err
	}
	var users []int64
	for rows.Next() {
		var (
			a     Admin
			perms = make([]bool, len(adminPerms))
		)
		dest := []any{&a.UserID, &a.Status, &a.CustomTitle, &a.IsAnonymous, &a.Since}
		for i := range perms {
			dest = append(dest, &perms[i])
		}
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return GroupPage{}, err
		}
		a.Perms = []string{}
		for i, ok := range perms {
			if ok {
				a.Perms = append(a.Perms, adminPerms[i].label)
			}
		}
		p.Admins = append(p.Admins, a)
		users = append(users, a.UserID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return GroupPage{}, err
	}
	p.Group.AdminCount = int64(len(p.Admins))
	if p.Group.Usernames, err = usernames(ctx, db, "telegram_group_usernames", "group_id", id); err != nil {
		return GroupPage{}, err
	}

	if p.History, err = GroupHistory(ctx, db, id, before, HistoryPageSize); err != nil {
		return GroupPage{}, err
	}
	for _, e := range p.History.Entries {
		if e.Kind == "admin" {
			users = append(users, e.UserID)
		}
	}

	// The posters and the activity of the last Days days, which the
	// (chat_id, date, sender_user_id) index serves without reading the
	// group's whole history.
	start := dayStart(now) - (Days-1)*day
	rows, err = db.QueryContext(ctx, "SELECT m.sender_user_id, COUNT(*) FROM telegram_group_messages m "+
		"WHERE m.chat_id = ? AND m.date >= ? AND m.sender_user_id IS NOT NULL "+
		"GROUP BY m.sender_user_id ORDER BY COUNT(*) DESC, m.sender_user_id LIMIT ?",
		id, start, topPosters)
	if err != nil {
		return GroupPage{}, err
	}
	for rows.Next() {
		var pp Poster
		if err := rows.Scan(&pp.UserID, &pp.Msgs); err != nil {
			rows.Close()
			return GroupPage{}, err
		}
		p.TopPosters = append(p.TopPosters, pp)
		users = append(users, pp.UserID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return GroupPage{}, err
	}
	if p.Activity, err = perDay(ctx, db, now, dayIndex+"FROM telegram_group_messages m "+
		"WHERE m.date >= ? AND m.chat_id = ? GROUP BY 1", id); err != nil {
		return GroupPage{}, err
	}
	if p.Users, err = usersByID(ctx, db, sc, users); err != nil {
		return GroupPage{}, err
	}
	return p, nil
}

// MessageDetail is the message page's message.
type MessageDetail struct {
	Message
	Group bool
	// Chat is the group or the private chat's other side.
	ChatGroup Group
	ChatUser  User
	Users     map[int64]User
	Groups    map[int64]Group
	// Album is the other messages of the message's album, in message
	// order (none when it is not in one).
	Album []Message
	// Topic is the forum topic of a group message, nil when it has
	// none or the topic was never seen (telegram_forum_topics).
	Topic *Topic
}

// albumSpan bounds the message ids an album's other items are looked
// for in: an album's items are sent together, so their ids are close,
// and the (chat_id, message_id) index then reads a short range.
const albumSpan = int64(albumEdge*4) << 20

// LoadMessage is the message with this row id, within the scope: a
// group message of a visible group, a private message when the scope
// sees private chats.
func LoadMessage(ctx context.Context, db *sql.DB, sc scope.Scope, group bool, rowID uint64) (MessageDetail, error) {
	if !group && !sc.SeesPrivate() {
		return MessageDetail{}, ErrNotFound
	}
	t := tablesOf(group)
	q := t.messageSelect() + "WHERE m.id = ?"
	args := []any{rowID}
	if group {
		cond, cargs := sc.GroupFilter("m.chat_id")
		q += " AND " + cond
		args = append(args, cargs...)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return MessageDetail{}, err
	}
	ms, err := scanMessages(rows)
	if err != nil {
		return MessageDetail{}, err
	}
	if len(ms) == 0 {
		return MessageDetail{}, ErrNotFound
	}
	if a := ms[0].AlbumID; a != 0 {
		rows, err := db.QueryContext(ctx, t.messageSelect()+"WHERE m.chat_id = ? AND m.message_id BETWEEN ? AND ? "+
			"AND m.media_album_id = ? AND m.id <> ? ORDER BY m.message_id LIMIT ?",
			ms[0].ChatID, ms[0].MessageID-albumSpan, ms[0].MessageID+albumSpan, a, ms[0].RowID, albumEdge)
		if err != nil {
			return MessageDetail{}, err
		}
		more, err := scanMessages(rows)
		if err != nil {
			return MessageDetail{}, err
		}
		ms = append(ms, more...)
	}
	if err := completeMessages(ctx, db, group, ms); err != nil {
		return MessageDetail{}, err
	}
	d := MessageDetail{Message: ms[0], Group: group, Album: ms[1:]}
	c := Chat{Group: group, ID: d.ChatID}
	var w Window
	w.Messages = ms
	if err := c.names(ctx, db, &w); err != nil {
		return MessageDetail{}, err
	}
	d.Users, d.Groups = w.Users, w.Groups
	if group {
		gs, err := groupsByID(ctx, db, []int64{d.ChatID})
		if err != nil {
			return MessageDetail{}, err
		}
		d.ChatGroup = gs[d.ChatID]
		if d.ChatGroup.ID == 0 {
			d.ChatGroup = Group{ID: d.ChatID, Title: "(no title)"}
		}
		if d.TopicID != 0 {
			if d.Topic, err = c.topic(ctx, db, d.TopicID); err != nil {
				return MessageDetail{}, err
			}
		}
	} else {
		us, err := usersByID(ctx, db, sc, []int64{d.ChatID})
		if err != nil {
			return MessageDetail{}, err
		}
		d.ChatUser = us[d.ChatID]
		d.ChatUser.ID = d.ChatID
	}
	return d, nil
}

// KPI is one of the dashboard's counters.
type KPI struct {
	Key   string
	Total int64
	Today int64
}

// Dashboard is the dashboard's data.
type Dashboard struct {
	KPIs    []KPI
	Busiest []Group
	Deleted []Message
	Users   map[int64]User
	Groups  map[int64]Group
}

// dashboardRows is how many busiest groups and deleted messages the
// dashboard lists.
const (
	busiestGroups   = 6
	deletedMessages = 5
)

// LoadKPIs are the dashboard's cards within the scope, as LoadDashboard
// answers them: totals and today's counts, today being the viewer's
// calendar day in loc. A viewer that sees everything gets the
// archive's totals; one that does not, counts over what it sees, and
// no private messages or files. Every count goes through the count
// cache, so asking often costs nothing more than asking once a TTL.
func LoadKPIs(ctx context.Context, db *sql.DB, sc scope.Scope, now int64,
	loc *time.Location) ([]KPI, error) {
	var out []KPI
	days := localDays(now, loc)
	today := days[Days-1]
	cond, cargs := sc.GroupFilter("m.chat_id")
	gcond, gargs := sc.GroupFilter("g.id")
	// The archive's totals: through the count cache (countcache.go).
	count := func(q string, args ...any) (int64, error) {
		return countInt(ctx, db, q, args...)
	}
	type kq struct {
		key          string
		total, today string
		targs, dargs []any
	}
	var ks []kq
	if sc.SeesAllUsers() {
		ks = append(ks, kq{"users", "SELECT COUNT(*) FROM telegram_users", "SELECT COUNT(*) FROM " +
			"telegram_users WHERE created_at >= FROM_UNIXTIME(?)", nil, []any{today}})
	} else {
		ks = append(ks, kq{"users", "SELECT COUNT(DISTINCT m.sender_user_id) FROM telegram_group_messages m " +
			"WHERE " + cond, "SELECT COUNT(DISTINCT m.sender_user_id) FROM telegram_group_messages m " +
			"WHERE m.date >= ? AND " + cond, cargs, append([]any{today}, cargs...)})
	}
	ks = append(ks,
		kq{"groups", "SELECT COUNT(*) FROM telegram_groups g WHERE " + gcond,
			"SELECT COUNT(*) FROM telegram_groups g WHERE g.created_at >= FROM_UNIXTIME(?) AND " + gcond,
			gargs, append([]any{today}, gargs...)})
	if sc.SeesAllGroups() {
		ks = append(ks, kq{"group_messages", "SELECT COUNT(*) FROM telegram_group_messages",
			"SELECT COUNT(*) FROM telegram_group_messages WHERE date >= ?", nil, []any{today}})
	} else {
		ks = append(ks, kq{"group_messages", "SELECT SUM(g.msg_count) FROM telegram_groups g WHERE " + gcond,
			"SELECT COUNT(*) FROM telegram_group_messages m WHERE m.date >= ? AND " + cond,
			gargs, append([]any{today}, cargs...)})
	}
	if sc.SeesPrivate() {
		ks = append(ks, kq{"private_messages", "SELECT COUNT(*) FROM telegram_private_messages",
			"SELECT COUNT(*) FROM telegram_private_messages WHERE date >= ?", nil, []any{today}})
	}
	if sc.SeesAllGroups() && sc.SeesPrivate() {
		ks = append(ks, kq{"files", "SELECT COUNT(*) FROM telegram_files",
			"SELECT COUNT(*) FROM telegram_files WHERE created_at >= FROM_UNIXTIME(?)", nil, []any{today}})
	}
	for _, k := range ks {
		t, err := count(k.total, k.targs...)
		if err != nil {
			return nil, err
		}
		n, err := count(k.today, k.dargs...)
		if err != nil {
			return nil, err
		}
		out = append(out, KPI{Key: k.key, Total: t, Today: n})
	}
	return out, nil
}

// LoadDashboard is the dashboard within the scope: a viewer that sees
// everything gets the archive's totals (as Browse.cpp's counts); one
// that does not gets counts over what it sees, and no private messages
// or files. Its today is the calendar day in loc, the viewer's
// timezone; the chart's series is LoadSeries'.
func LoadDashboard(ctx context.Context, db *sql.DB, sc scope.Scope, now int64,
	loc *time.Location) (Dashboard, error) {
	var d Dashboard
	cond, cargs := sc.GroupFilter("m.chat_id")
	gcond, gargs := sc.GroupFilter("g.id")
	var err error
	if d.KPIs, err = LoadKPIs(ctx, db, sc, now, loc); err != nil {
		return Dashboard{}, err
	}

	rows, err := db.QueryContext(ctx, groupQuery+"WHERE "+gcond+
		" ORDER BY g.msg_count DESC, g.id LIMIT ?", append(gargs, busiestGroups)...)
	if err != nil {
		return Dashboard{}, err
	}
	if d.Busiest, err = scanGroups(rows); err != nil {
		return Dashboard{}, err
	}

	rows, err = db.QueryContext(ctx, groupTables.messageSelect()+"WHERE m.deleted_at IS NOT NULL AND "+
		cond+" ORDER BY m.deleted_at DESC, m.id DESC LIMIT ?", append(cargs, deletedMessages)...)
	if err != nil {
		return Dashboard{}, err
	}
	if d.Deleted, err = scanMessages(rows); err != nil {
		return Dashboard{}, err
	}
	var users, groups []int64
	for _, m := range d.Deleted {
		users = append(users, m.SenderUser)
		groups = append(groups, m.ChatID, m.SenderChat)
	}
	if d.Users, err = usersByID(ctx, db, sc, users); err != nil {
		return Dashboard{}, err
	}
	if d.Groups, err = groupsByID(ctx, db, groups); err != nil {
		return Dashboard{}, err
	}
	return d, nil
}

// GroupUsernames are a group's usernames, as its page lists them. The
// caller decides the group may be shown.
func GroupUsernames(ctx context.Context, db *sql.DB, id int64) ([]Username, error) {
	return usernames(ctx, db, "telegram_group_usernames", "group_id", id)
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strings"

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

// dayIndex is the SELECT head of a perDay query over a table aliased m
// with a unix date column, whose WHERE starts with "m.date >= ?"; perDay
// passes the series' start to both.
const dayIndex = "SELECT FLOOR((m.date - ?) / 86400), COUNT(*) "

// UserDetail is the user page's person: the summary and the profile a
// full-access viewer sees.
type UserDetail struct {
	User
	Bio       string
	Phone     string
	Usernames []Username
}

// Username is one of a user's current usernames.
type Username struct {
	Username string
	Active   bool
}

// GroupCount is a group with the number of messages of someone in it.
type GroupCount struct {
	Group
	Msgs int64
}

// HistoryEntry is one change on a user's or group's timeline.
type HistoryEntry struct {
	Kind   string // name, username, bio, phone, photo; title, description, admin
	At     int64
	Detail string
	Action string
	Photo  FileRef // a photo change; ID 0 = removed
	UserID int64   // an admin change's admin
}

// UserPage is the data of the user pages.
type UserPage struct {
	// Limited: the viewer sees the person only through its groups;
	// only User and Groups (without counts) are filled.
	Limited  bool
	User     UserDetail
	Groups   []GroupCount
	History  []HistoryEntry
	Activity []int64
}

// LoadUserPage is the user page of id, within the scope; now is the
// server's clock (unix), for the activity series.
func LoadUserPage(ctx context.Context, db *sql.DB, sc scope.Scope, id, now int64) (UserPage, error) {
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

	err = db.QueryRowContext(ctx, "SELECT COALESCE(bio, ''), COALESCE(phone_number, '') "+
		"FROM telegram_user_extra_info WHERE user_id = ?", id).Scan(&p.User.Bio, &p.User.Phone)
	if err != nil && err != sql.ErrNoRows {
		return UserPage{}, err
	}
	rows, err = db.QueryContext(ctx, "SELECT username, kind = 'active' FROM telegram_user_usernames "+
		"WHERE user_id = ? ORDER BY kind, position", id)
	if err != nil {
		return UserPage{}, err
	}
	for rows.Next() {
		var u Username
		if err := rows.Scan(&u.Username, &u.Active); err != nil {
			rows.Close()
			return UserPage{}, err
		}
		p.User.Usernames = append(p.User.Usernames, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return UserPage{}, err
	}

	if p.History, err = userHistory(ctx, db, id); err != nil {
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

// historyLimit caps a timeline.
const historyLimit = 200

// userHistory is the timeline of the five per-user history tables,
// newest first, as userHistory in Browse.cpp merges them. Username
// events other than additions and removals are left out.
func userHistory(ctx context.Context, db *sql.DB, id int64) ([]HistoryEntry, error) {
	q := "SELECT t.kind, UNIX_TIMESTAMP(t.created_at), COALESCE(t.detail, ''), " +
		"COALESCE(t.action, ''), COALESCE(t.file_id, 0), " + fileColumnsOf("f") + " FROM (" +
		"  SELECT 'name' AS kind, id, created_at, " +
		"         TRIM(CONCAT_WS(' ', first_name, last_name)) AS detail, " +
		"         NULL AS action, NULL AS file_id " +
		"  FROM telegram_user_hist_name WHERE user_id = ? " +
		"  UNION ALL SELECT 'bio', id, created_at, bio, NULL, NULL " +
		"  FROM telegram_user_hist_bio WHERE user_id = ? " +
		"  UNION ALL SELECT 'phone', id, created_at, phone_number, NULL, NULL " +
		"  FROM telegram_user_hist_phone_num WHERE user_id = ? " +
		"  UNION ALL SELECT 'username', id, created_at, username, action, NULL " +
		"  FROM telegram_user_hist_usernames_events WHERE user_id = ? " +
		"  AND action IN ('added', 'removed') " +
		"  UNION ALL SELECT 'photo', id, created_at, NULL, NULL, file_id " +
		"  FROM telegram_user_hist_profile_photo WHERE user_id = ? " +
		") t LEFT JOIN telegram_files f ON f.id = t.file_id " +
		"ORDER BY t.created_at DESC, t.kind, t.id DESC LIMIT ?"
	return scanHistory(db.QueryContext(ctx, q, id, id, id, id, id, historyLimit))
}

func scanHistory(rows *sql.Rows, err error) ([]HistoryEntry, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var (
			e    HistoryEntry
			file uint64
			f    nullFile
		)
		dest := append([]any{&e.Kind, &e.At, &e.Detail, &e.Action, &file}, f.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if file != 0 {
			e.Photo = f.ref(file)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GroupDetail is the group page's group.
type GroupDetail struct {
	Group
	AdminCount int64
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
	History    []HistoryEntry
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

// LoadGroupPage is the group page of id, within the scope.
func LoadGroupPage(ctx context.Context, db *sql.DB, sc scope.Scope, id, now int64) (GroupPage, error) {
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

	q := "SELECT t.kind, UNIX_TIMESTAMP(t.created_at), COALESCE(t.detail, ''), " +
		"COALESCE(t.action, ''), COALESCE(t.file_id, 0), " + fileColumnsOf("f") + ", t.user_id FROM (" +
		"  SELECT 'title' AS kind, id, created_at, title AS detail, NULL AS action, " +
		"         NULL AS file_id, 0 AS user_id " +
		"  FROM telegram_group_hist_title WHERE group_id = ? " +
		"  UNION ALL SELECT 'description', id, created_at, description, NULL, NULL, 0 " +
		"  FROM telegram_group_hist_description WHERE group_id = ? " +
		"  UNION ALL SELECT 'username', id, created_at, username, action, NULL, 0 " +
		"  FROM telegram_group_hist_usernames_events WHERE group_id = ? " +
		"  AND action IN ('added', 'removed') " +
		"  UNION ALL SELECT 'photo', id, created_at, NULL, NULL, file_id, 0 " +
		"  FROM telegram_group_hist_photo WHERE group_id = ? " +
		"  UNION ALL SELECT 'admin', id, created_at, " +
		"         IF(custom_title <> '', custom_title, status), action, NULL, user_id " +
		"  FROM telegram_group_admin_hist WHERE group_id = ? " +
		") t LEFT JOIN telegram_files f ON f.id = t.file_id " +
		"ORDER BY t.created_at DESC, t.kind, t.id DESC LIMIT ?"
	rows, err = db.QueryContext(ctx, q, id, id, id, id, id, historyLimit)
	if err != nil {
		return GroupPage{}, err
	}
	p.History = []HistoryEntry{}
	for rows.Next() {
		var (
			e    HistoryEntry
			file uint64
			f    nullFile
		)
		dest := append([]any{&e.Kind, &e.At, &e.Detail, &e.Action, &file}, f.dest()...)
		dest = append(dest, &e.UserID)
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return GroupPage{}, err
		}
		if file != 0 {
			e.Photo = f.ref(file)
		}
		p.History = append(p.History, e)
		users = append(users, e.UserID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return GroupPage{}, err
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
	if p.Users, err = usersByID(ctx, db, users); err != nil {
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
}

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
	if err := completeMessages(ctx, db, group, ms); err != nil {
		return MessageDetail{}, err
	}
	d := MessageDetail{Message: ms[0], Group: group}
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
	} else {
		us, err := usersByID(ctx, db, []int64{d.ChatID})
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
	PerDay  []int64
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

// LoadDashboard is the dashboard within the scope: a viewer that sees
// everything gets the archive's totals (as Browse.cpp's counts); one
// that does not gets counts over what it sees, and no private messages
// or files.
func LoadDashboard(ctx context.Context, db *sql.DB, sc scope.Scope, now int64) (Dashboard, error) {
	var d Dashboard
	today := dayStart(now)
	cond, cargs := sc.GroupFilter("m.chat_id")
	gcond, gargs := sc.GroupFilter("g.id")
	count := func(q string, args ...any) (int64, error) {
		var n sql.NullInt64
		err := db.QueryRowContext(ctx, q, args...).Scan(&n)
		return n.Int64, err
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
			return Dashboard{}, err
		}
		n, err := count(k.today, k.dargs...)
		if err != nil {
			return Dashboard{}, err
		}
		d.KPIs = append(d.KPIs, KPI{Key: k.key, Total: t, Today: n})
	}

	var err error
	if d.PerDay, err = perDay(ctx, db, now, dayIndex+"FROM telegram_group_messages m "+
		"WHERE m.date >= ? AND "+cond+" GROUP BY 1", cargs...); err != nil {
		return Dashboard{}, err
	}
	if sc.SeesPrivate() {
		priv, err := perDay(ctx, db, now, dayIndex+"FROM telegram_private_messages m "+
			"WHERE m.date >= ? GROUP BY 1")
		if err != nil {
			return Dashboard{}, err
		}
		for i := range d.PerDay {
			d.PerDay[i] += priv[i]
		}
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
	if d.Users, err = usersByID(ctx, db, users); err != nil {
		return Dashboard{}, err
	}
	if d.Groups, err = groupsByID(ctx, db, groups); err != nil {
		return Dashboard{}, err
	}
	return d, nil
}

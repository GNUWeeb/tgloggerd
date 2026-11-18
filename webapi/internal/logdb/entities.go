// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"slices"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/scope"
)

// Users and groups as the pages and the chat show them, loaded in
// batches by id. The batch loaders are unexported: the ids they get
// come from rows the request may already see (a visible chat's
// senders, a visible group's admins). Users and groups a request asks
// for by id go through VisibleUsers and VisibleGroups, which apply the
// scope.

// FileRef is a file a row refers to, as far as its URL and its media
// object need it.
type FileRef struct {
	ID     uint64
	Exists bool   // the telegram_files row exists
	OnDisk bool   // the bytes are in the store
	Size   uint64 // 0 = unknown
	Ext    string // without the dot, or ""
	Name   string // the name sent through Telegram, or ""
	Type   string // file_type
}

// User is a telegram_users row with what its summary shows.
type User struct {
	ID           int64
	FirstName    string
	LastName     string
	Type         string // regular, bot, deleted, unknown
	Username     string // the first active username, or ""
	Photo        FileRef
	ContactFirst string
	ContactLast  string
	IsContact    bool // contact_first_name is set
	MsgCount     uint64
	IsVerified   bool
	IsPremium    bool
	IsScam       bool
	IsFake       bool
	IsSupport    bool
	Language     string
	CreatedAt    int64 // unix seconds
	UpdatedAt    int64
}

// Name is the user's display name: the own name, else the name the
// logging account saved the contact under, else "(no name)".
func (u User) Name() string {
	if n := joinName(u.FirstName, u.LastName); n != "" {
		return n
	}
	if n := joinName(u.ContactFirst, u.ContactLast); n != "" {
		return n
	}
	if u.Type == "deleted" {
		return "Deleted Account"
	}
	return "(no name)"
}

// ContactName is the name the logging account saved the user under, or
// "" when the user is not a contact.
func (u User) ContactName() string {
	if !u.IsContact {
		return ""
	}
	return joinName(u.ContactFirst, u.ContactLast)
}

func joinName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

// Group is a telegram_groups row with what its summary shows.
type Group struct {
	ID          int64
	Type        string // supergroup, basic_group, channel
	Title       string // "(no title)" when empty
	Username    string
	Photo       FileRef
	Description string
	MsgCount    uint64
	Exposed     bool // in telegram_public_groups
	CreatedAt   int64
	UpdatedAt   int64
}

// placeholders is "?, ?, ..." for n values.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func int64Args(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

// uniq is ids without zeros and repeats, sorted.
func uniq(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != 0 {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// fileColumnsOf selects a FileRef's columns of the files row joined as
// alias.
func fileColumnsOf(alias string) string {
	return alias + ".id, " + alias + ".on_disk, " + alias + ".file_size, " +
		"COALESCE(" + alias + ".file_ext, ''), " + alias + ".orig_file_name, " +
		alias + ".file_type"
}

// nullFile scans the columns of fileColumnsOf.
type nullFile struct {
	id     sql.NullInt64
	onDisk sql.NullBool
	size   sql.NullInt64
	ext    sql.NullString
	name   sql.NullString
	typ    sql.NullString
}

func (n *nullFile) dest() []any {
	return []any{&n.id, &n.onDisk, &n.size, &n.ext, &n.name, &n.typ}
}

// ref is the file, given the id the referring row holds (0 for none).
func (n *nullFile) ref(id uint64) FileRef {
	if !n.id.Valid {
		return FileRef{ID: id}
	}
	return FileRef{ID: uint64(n.id.Int64), Exists: true, OnDisk: n.onDisk.Bool,
		Size: uint64(n.size.Int64), Ext: n.ext.String, Name: n.name.String, Type: n.typ.String}
}

const userSelect = "SELECT u.id, u.first_name, u.last_name, u.type, " +
	"(SELECT un.username FROM telegram_user_usernames un WHERE un.user_id = u.id " +
	" AND un.kind = 'active' ORDER BY un.position LIMIT 1), " +
	"COALESCE(u.profile_photo_file_id, 0), " + "%FILE%, " +
	"u.contact_first_name, u.contact_last_name, u.msg_count, u.is_verified, " +
	"u.is_premium, u.is_scam, u.is_fake, u.is_support, " +
	"COALESCE(e.language_code, ''), UNIX_TIMESTAMP(u.created_at), " +
	"UNIX_TIMESTAMP(u.updated_at) FROM telegram_users u " +
	"LEFT JOIN telegram_files f ON f.id = u.profile_photo_file_id " +
	"LEFT JOIN telegram_user_extra_info e ON e.user_id = u.id "

var userQuery = strings.Replace(userSelect, "%FILE%", fileColumnsOf("f"), 1)

func scanUsers(rows *sql.Rows) (map[int64]User, error) {
	defer rows.Close()
	out := map[int64]User{}
	for rows.Next() {
		var (
			u            User
			username     sql.NullString
			photo        uint64
			f            nullFile
			cFirst, cLst sql.NullString
		)
		dest := []any{&u.ID, &u.FirstName, &u.LastName, &u.Type, &username, &photo}
		dest = append(dest, f.dest()...)
		dest = append(dest, &cFirst, &cLst, &u.MsgCount, &u.IsVerified, &u.IsPremium,
			&u.IsScam, &u.IsFake, &u.IsSupport, &u.Language, &u.CreatedAt, &u.UpdatedAt)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		u.Username = username.String
		if photo != 0 {
			u.Photo = f.ref(photo)
		}
		u.ContactFirst, u.ContactLast, u.IsContact = cFirst.String, cLst.String, cFirst.Valid
		out[u.ID] = u
	}
	return out, rows.Err()
}

// userTotal is the SQL expression of a user's message total as the
// scope may see it, and the joins it reads: telegram_users.msg_count
// counts every group and private message the user sent, so a scope
// that hides groups or private chats gets it less the user's messages
// there. Those are counted once per query, per sender, in derived
// tables joined to the users (hg, hp), not per user row: the users
// list sorts by the total, and a subquery per row over half a million
// users ran past the statement time limit. The hidden groups are
// inlined: they are our own integers, from the account's settings.
func userTotal(sc scope.Scope) (expr, joins string) {
	minus := ""
	if hidden := sc.HiddenGroups(); len(hidden) > 0 {
		ids := make([]string, len(hidden))
		for i, g := range hidden {
			ids[i] = strconv.FormatInt(g, 10)
		}
		joins += " LEFT JOIN (SELECT sender_user_id AS uid, COUNT(*) AS n FROM telegram_group_messages" +
			" WHERE chat_id IN (" + strings.Join(ids, ",") + ") AND sender_user_id IS NOT NULL" +
			" GROUP BY sender_user_id) hg ON hg.uid = u.id"
		minus += " - COALESCE(hg.n, 0)"
	}
	if !sc.SeesPrivate() && sc.SeesAllUsers() {
		joins += " LEFT JOIN (SELECT sender_id AS uid, COUNT(*) AS n FROM telegram_private_messages" +
			" WHERE sender_id IS NOT NULL GROUP BY sender_id) hp ON hp.uid = u.id"
		minus += " - COALESCE(hp.n, 0)"
	}
	if minus == "" {
		return "u.msg_count", ""
	}
	return "GREATEST(CAST(u.msg_count AS SIGNED)" + minus + ", 0)", joins
}

// scopedTotals rewrites a query over users aliased u so it reads the
// scope's userTotal wherever it reads u.msg_count, joining what that
// reads right after each telegram_users u it selects from (a query and
// the page subquery the users search may nest in it). A query that does
// not read the total is left alone, so it pays for no join.
func scopedTotals(sc scope.Scope, q string) string {
	t, joins := userTotal(sc)
	if t == "u.msg_count" || !strings.Contains(q, "u.msg_count") {
		return q
	}
	// One pass, so the expression's own u.msg_count is left alone;
	// the column keeps its name for the rows that read it by name.
	q = strings.NewReplacer("u.msg_count AS msg_count", t+" AS msg_count",
		"u.msg_count", t).Replace(q)
	return strings.ReplaceAll(q, "telegram_users u ", "telegram_users u"+joins+" ")
}

// usersByID loads the users with these ids; missing ones are left out.
// Their totals are as sc may see them (userTotal).
func usersByID(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) (map[int64]User, error) {
	ids = uniq(ids)
	if len(ids) == 0 {
		return map[int64]User{}, nil
	}
	rows, err := db.QueryContext(ctx, scopedTotals(sc, userQuery)+"WHERE u.id IN ("+placeholders(len(ids))+")",
		int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

var groupQuery = "SELECT g.id, g.type, g.title, " +
	"(SELECT gu.username FROM telegram_group_usernames gu WHERE gu.group_id = g.id " +
	" AND gu.kind = 'active' ORDER BY gu.position LIMIT 1), " +
	"COALESCE(g.photo_file_id, 0), " + fileColumnsOf("f") + ", g.description, g.msg_count, " +
	"pg.group_id IS NOT NULL, UNIX_TIMESTAMP(g.created_at), UNIX_TIMESTAMP(g.updated_at) " +
	"FROM telegram_groups g LEFT JOIN telegram_files f ON f.id = g.photo_file_id " +
	"LEFT JOIN telegram_public_groups pg ON pg.group_id = g.id "

func scanGroups(rows *sql.Rows) ([]Group, error) {
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var (
			g        Group
			username sql.NullString
			photo    uint64
			f        nullFile
		)
		dest := []any{&g.ID, &g.Type, &g.Title, &username, &photo}
		dest = append(dest, f.dest()...)
		dest = append(dest, &g.Description, &g.MsgCount, &g.Exposed, &g.CreatedAt, &g.UpdatedAt)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		g.Username = username.String
		if g.Title == "" {
			g.Title = "(no title)"
		}
		if photo != 0 {
			g.Photo = f.ref(photo)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func groupMap(gs []Group) map[int64]Group {
	m := make(map[int64]Group, len(gs))
	for _, g := range gs {
		m[g.ID] = g
	}
	return m
}

// groupsByID loads the groups with these ids; missing ones are left
// out.
func groupsByID(ctx context.Context, db *sql.DB, ids []int64) (map[int64]Group, error) {
	ids = uniq(ids)
	if len(ids) == 0 {
		return map[int64]Group{}, nil
	}
	rows, err := db.QueryContext(ctx, groupQuery+"WHERE g.id IN ("+placeholders(len(ids))+")",
		int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	gs, err := scanGroups(rows)
	if err != nil {
		return nil, err
	}
	return groupMap(gs), nil
}

// VisibleGroups is the groups among ids that exist and that the scope
// sees, in the order of ids.
func VisibleGroups(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) ([]Group, error) {
	u := uniq(ids)
	if len(u) == 0 {
		return []Group{}, nil
	}
	cond, cargs := sc.GroupFilter("g.id")
	args := append(int64Args(u), cargs...)
	rows, err := db.QueryContext(ctx, groupQuery+"WHERE g.id IN ("+placeholders(len(u))+
		") AND "+cond, args...)
	if err != nil {
		return nil, err
	}
	gs, err := scanGroups(rows)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, groupMap(gs)), nil
}

func inOrder[T any](ids []int64, m map[int64]T) []T {
	out := []T{}
	seen := map[int64]bool{}
	for _, id := range ids {
		if v, ok := m[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, v)
		}
	}
	return out
}

// usersInVisibleGroups is the users among ids that appear in a group
// the scope sees: as a sender of a message there or as one of its
// admins. It is how a viewer without SeesAllUsers sees users
// (web-accounts.md section 5); the sender and admin indexes serve both
// halves.
func usersInVisibleGroups(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) (map[int64]bool, error) {
	ids = uniq(ids)
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	mcond, margs := sc.GroupFilter("m.chat_id")
	acond, aargs := sc.GroupFilter("a.group_id")
	in := placeholders(len(ids))
	args := append(int64Args(ids), margs...)
	args = append(args, int64Args(ids)...)
	args = append(args, aargs...)
	rows, err := db.QueryContext(ctx,
		"SELECT DISTINCT m.sender_user_id FROM telegram_group_messages m "+
			"WHERE m.sender_user_id IN ("+in+") AND "+mcond+
			" UNION SELECT a.user_id FROM telegram_group_admins a "+
			"WHERE a.user_id IN ("+in+") AND "+acond, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// VisibleUsers is the users among ids that exist and that the scope
// sees, in the order of ids: every user for a viewer that sees all
// users, else those that appear in its visible groups.
func VisibleUsers(ctx context.Context, db *sql.DB, sc scope.Scope, ids []int64) ([]User, error) {
	m, err := usersByID(ctx, db, sc, ids)
	if err != nil {
		return nil, err
	}
	if !sc.SeesAllUsers() && len(m) > 0 {
		keys := make([]int64, 0, len(m))
		for id := range m {
			keys = append(keys, id)
		}
		vis, err := usersInVisibleGroups(ctx, db, sc, keys)
		if err != nil {
			return nil, err
		}
		for id := range m {
			if !vis[id] {
				delete(m, id)
			}
		}
	}
	return inOrder(ids, m), nil
}

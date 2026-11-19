// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"fmt"
	"regexp"

	"tgloggerd/webapi/internal/scope"
)

// The searchable entities: their field registries, displayed columns
// and rows, as in web/src/dao/Search.cpp, and the scope condition of
// each.

// Entity is one searchable entity.
type Entity struct {
	// Name is the entity's name in /api/search/{entity}, and the
	// scope's page key of its list page.
	Name string

	from      string // FROM and JOINs of the page query
	countFrom string // of the count, when cheaper; "" = from
	// countBare counts without from's one-to-one LEFT JOIN, whose
	// table's alias is joinAlias, for a WHERE that does not name it:
	// the join cannot change the count, only slow it.
	countBare string
	joinAlias *regexp.Regexp
	sel       string // the page query's columns
	pageSel   string // more columns for the page loaders
	idCol     string // tie breaker and EXISTS join target
	exFK      string // the id column of the EXISTS tables
	defSort   string
	defOrder  string
	fields    []Field
	cols      []Col
	// estimateBrowse: a search without conditions is never counted
	// exactly; it reports the table's row estimate (millions of rows).
	estimateBrowse bool
	// filter is the scope's condition over the entity's rows, with
	// its values; "" when the scope sees every row.
	filter func(sc scope.Scope) (string, []any)
	// full is whether the scope sees the rows in full; nil is always.
	// Otherwise only the fields and columns in limited are there.
	full    func(sc scope.Scope) bool
	limited map[string]bool
	// cells are one row's displayed values by column key, as the API
	// sends them: strings HTML-escaped, photo cells as photoID.
	cells func(sc scope.Scope, r Row) map[string]any
}

// visible is whether the scope sees the entity at all: whether its
// list page is the viewer's.
func (e *Entity) visible(sc scope.Scope) bool { return sc.Page(e.Name) }

func (e *Entity) isFull(sc scope.Scope) bool { return e.full == nil || e.full(sc) }

func (e *Entity) fieldVisible(sc scope.Scope, f *Field) bool {
	return e.isFull(sc) || e.limited[f.Key]
}

func (e *Entity) colVisible(sc scope.Scope, key string) bool {
	return e.isFull(sc) || e.limited[key]
}

// Full is whether the scope sees the entity's rows in full (a member
// sees users only by name and photo, web-accounts.md section 5).
func (e *Entity) Full(sc scope.Scope) bool { return e.isFull(sc) }

const (
	intOps    = OpEQ | OpNE | OpLT | OpGT | OpLE | OpGE
	dtOps     = intOps
	textOps   = OpEQ | OpNE | OpLike | OpNLike | OpCLike | OpNCLike
	likeOps   = OpLike | OpNLike | OpCLike | OpNCLike
	boolOps   = OpEQ | OpNE
	enumOps   = OpEQ | OpNE | OpIn
	nullOps   = OpIsNull | OpIsNotNull
	existsOps = OpEQ | OpNE | OpLike | OpNLike | OpCLike | OpNCLike
	existsPos = OpEQ | OpLike | OpCLike
)

func col(key, expr string, t FieldType, label string, ops Op, sortable bool) Field {
	return Field{Key: key, Label: label, Type: t, Expr: expr, Ops: ops, Sortable: sortable}
}

func exists(key, label, table, column, extra string, ops Op) Field {
	return Field{Key: key, Label: label, Type: Text, Exists: true, ExTable: table,
		ExCol: column, ExExtra: extra, Ops: ops}
}

func internal(f Field) Field {
	f.internal = true
	return f
}

// needsChat marks a field usable only within one chat (Field.NeedsChat).
func needsChat(f Field) Field {
	f.NeedsChat = true
	return f
}

func enum(f Field, values string) Field {
	f.Enum = values
	return f
}

// both returns two scope conditions' values as one list.
func both(a, b []any) []any {
	return append(append([]any{}, a...), b...)
}

// --- users -------------------------------------------------------------

// searchUserSelect is the users query's columns; the photo is
// profile_photo_file_id, the current username username.
const searchUserSelect = "u.id, u.first_name, u.last_name, u.type, u.is_verified, " +
	"u.is_premium, u.is_scam, u.is_fake, u.is_support, " +
	"u.profile_photo_file_id, u.created_at, u.updated_at, " +
	"u.msg_count AS msg_count, u.contact_first_name, u.contact_last_name, " +
	"(SELECT un.username FROM telegram_user_usernames un " +
	"WHERE un.user_id = u.id AND un.kind='active' " +
	"ORDER BY un.position LIMIT 1) AS username, " +
	"COALESCE(e.phone_number,'') AS phone, " +
	"COALESCE(e.language_code,'') AS language, " +
	"COALESCE(e.restriction_reason,'') AS restriction_reason, " +
	"COALESCE(e.has_sensitive_content,0) AS has_sensitive, " +
	"COALESCE(e.restricts_new_chats,0) AS restricts_new_chats, " +
	"COALESCE(e.paid_message_star_count,0) AS paid_star_count, " +
	"e.personal_chat_id AS personal_chat_id, " +
	"e.emoji_status_custom_emoji_id AS emoji_status, " +
	"COALESCE(e.bio,'') AS bio"

// DisplayName is a user's own name as Drogon shows it, never empty.
func DisplayName(first, last string) string {
	n := first
	if last != "" {
		if n != "" {
			n += " "
		}
		n += last
	}
	if n == "" {
		return "(no name)"
	}
	return n
}

// ContactName is the name the logging account saved a contact under,
// "" for one who is not a contact.
func ContactName(r Row) string {
	cf, cl := r.Str("contact_first_name"), r.Str("contact_last_name")
	if cl == "" {
		return cf
	}
	if cf == "" {
		return cl
	}
	return cf + " " + cl
}

// Users are the logged users. A scope that does not see every user
// (a member) sees those of its visible groups: the ones who sent a
// message there or administer one, by id, name, username, type and
// photo only.
var Users = &Entity{
	Name: "users",
	from: "telegram_users u LEFT JOIN telegram_user_extra_info e ON e.user_id = u.id",
	// telegram_user_extra_info is keyed by user_id: at most one row per
	// user, so the join adds no row.
	countBare: "telegram_users u",
	joinAlias: regexp.MustCompile(`(^|[^A-Za-z0-9_.])e\.`),
	sel:       searchUserSelect,
	idCol:     "u.id",
	exFK:      "user_id",
	defSort:   "u.id",
	defOrder:  "DESC",
	fields: []Field{
		col("id", "u.id", Int, "User ID", intOps, true),
		col("first_name", "u.first_name", Text, "First name", textOps, true),
		col("last_name", "u.last_name", Text, "Last name", textOps, false),
		col("any_name", "CONCAT_WS(' ', u.first_name, u.last_name, "+
			"COALESCE(u.contact_first_name,''), COALESCE(u.contact_last_name,''))",
			Text, "Name (own or contact)", likeOps, false),
		col("contact_first_name", "COALESCE(u.contact_first_name,'')", Text,
			"Contact first name", textOps, false),
		col("contact_last_name", "COALESCE(u.contact_last_name,'')", Text,
			"Contact last name", textOps, false),
		col("is_contact", "COALESCE(u.is_contact,0)", Bool, "Contact", boolOps, false),
		exists("username", "Username (current)", "telegram_user_usernames x",
			"x.username", "AND x.kind='active'", existsOps),
		enum(col("type", "u.type", Enum, "Type", enumOps, true), "regular,deleted,bot,unknown"),
		col("msg_count", "u.msg_count", Int, "Messages", intOps, true),
		col("has_photo", "u.profile_photo_file_id", Bool, "Has photo", nullOps, false),
		col("is_verified", "u.is_verified", Bool, "Verified", boolOps, false),
		col("is_premium", "u.is_premium", Bool, "Premium", boolOps, false),
		col("is_scam", "u.is_scam", Bool, "Scam", boolOps, false),
		col("is_fake", "u.is_fake", Bool, "Fake", boolOps, false),
		col("is_support", "u.is_support", Bool, "Support", boolOps, false),
		col("created_at", "u.created_at", Datetime, "Created", dtOps, true),
		col("updated_at", "u.updated_at", Datetime, "Updated", dtOps, true),
		col("bio", "COALESCE(e.bio,'')", Text, "Bio", textOps, false),
		col("phone", "COALESCE(e.phone_number,'')", Text, "Phone", textOps, false),
		col("language", "COALESCE(e.language_code,'')", Text, "Language", textOps, false),
		col("restriction_reason", "COALESCE(e.restriction_reason,'')", Text,
			"Restriction", textOps, false),
		col("has_sensitive", "COALESCE(e.has_sensitive_content,0)", Bool,
			"Sensitive content", boolOps, false),
		col("restricts_new_chats", "COALESCE(e.restricts_new_chats,0)", Bool,
			"Restricts new chats", boolOps, false),
		col("paid_star_count", "COALESCE(e.paid_message_star_count,0)", Int,
			"Paid message stars", intOps, true),
		col("personal_chat_id", "e.personal_chat_id", Int, "Personal chat",
			OpEQ|OpNE|nullOps, false),
		col("emoji_status", "e.emoji_status_custom_emoji_id", Int, "Emoji status",
			nullOps, false),
		exists("hist_username", "Username (ever)", "telegram_user_hist_usernames_events x",
			"x.username", "", existsOps),
		exists("hist_first_name", "First name (ever)", "telegram_user_hist_name x",
			"x.first_name", "", existsPos),
		exists("hist_last_name", "Last name (ever)", "telegram_user_hist_name x",
			"x.last_name", "", existsPos),
		exists("hist_bio", "Bio (ever)", "telegram_user_hist_bio x", "x.bio", "", existsPos),
		exists("hist_phone", "Phone (ever)", "telegram_user_hist_phone_num x",
			"x.phone_number", "", existsPos),
		// The users page's: the own name as one string, and the
		// language, sortable.
		internal(col("full_name", "TRIM(CONCAT_WS(' ', u.first_name, u.last_name))",
			Text, "Name", textOps, false)),
		internal(col("lang", "COALESCE(e.language_code,'')", Text, "Language",
			textOps, true)),
	},
	cols: []Col{
		{"photo", "", "photo", ""},
		{"id", "ID", "id", "id"},
		{"name", "Name", "name", "first_name"},
		{"contact_name", "Contact name", "text", ""},
		{"username", "Username", "username", ""},
		{"type", "Type", "text", "type"},
		{"msg_count", "Messages", "int", "msg_count"},
		{"is_verified", "Verified", "bool", ""},
		{"is_premium", "Premium", "bool", ""},
		{"is_scam", "Scam", "bool", ""},
		{"is_fake", "Fake", "bool", ""},
		{"is_support", "Support", "bool", ""},
		{"phone", "Phone", "text", ""},
		{"language", "Language", "text", ""},
		{"restriction_reason", "Restriction", "longtext", ""},
		{"has_sensitive", "Sensitive", "bool", ""},
		{"restricts_new_chats", "Restricts chats", "bool", ""},
		{"paid_star_count", "Paid stars", "int", "paid_star_count"},
		{"personal_chat_id", "Personal chat", "int", ""},
		{"emoji_status", "Emoji status", "int", ""},
		{"bio", "Bio", "longtext", ""},
		{"created_at", "Created", "datetime", "created_at"},
		{"updated_at", "Updated", "datetime", "updated_at"},
	},
	filter: func(sc scope.Scope) (string, []any) {
		if sc.SeesAllUsers() {
			return "", nil
		}
		return sc.UserFilter("u.id")
	},
	full: func(sc scope.Scope) bool { return sc.SeesAllUsers() },
	limited: map[string]bool{
		"id": true, "first_name": true, "last_name": true, "full_name": true,
		"username": true, "type": true, "has_photo": true,
		"photo": true, "name": true,
	},
	cells: func(_ scope.Scope, r Row) map[string]any {
		return map[string]any{
			"photo":               photoID(r.Uint("profile_photo_file_id")),
			"id":                  r.Int("id"),
			"name":                esc(DisplayName(r.Str("first_name"), r.Str("last_name"))),
			"contact_name":        esc(ContactName(r)),
			"username":            esc(r.Str("username")),
			"type":                r.Str("type"),
			"msg_count":           r.Str("msg_count"),
			"is_verified":         r.Bool("is_verified"),
			"is_premium":          r.Bool("is_premium"),
			"is_scam":             r.Bool("is_scam"),
			"is_fake":             r.Bool("is_fake"),
			"is_support":          r.Bool("is_support"),
			"phone":               esc(r.Str("phone")),
			"language":            esc(r.Str("language")),
			"restriction_reason":  esc(r.Str("restriction_reason")),
			"has_sensitive":       r.Bool("has_sensitive"),
			"restricts_new_chats": r.Bool("restricts_new_chats"),
			"paid_star_count":     r.Str("paid_star_count"),
			"personal_chat_id":    r.Str("personal_chat_id"),
			"emoji_status":        r.Str("emoji_status"),
			"bio":                 esc(r.Str("bio")),
			"created_at":          r.Str("created_at"),
			"updated_at":          r.Str("updated_at"),
		}
	},
}

// --- groups ------------------------------------------------------------

// Groups are the logged groups and channels, limited by GroupFilter.
var Groups = &Entity{
	Name: "groups",
	from: "`telegram_groups` g",
	sel: "g.id, g.title, g.description, g.type, g.photo_file_id, " +
		"g.created_at, g.updated_at, g.msg_count, g.member_count, g.member_count_at, " +
		"(SELECT gu.username FROM telegram_group_usernames gu " +
		"WHERE gu.group_id = g.id AND gu.kind='active' " +
		"ORDER BY gu.position LIMIT 1) AS username, " +
		"(SELECT COUNT(*) FROM telegram_group_admins ga " +
		"WHERE ga.group_id = g.id) AS admin_count",
	pageSel: "EXISTS (SELECT 1 FROM telegram_public_groups pg " +
		"WHERE pg.group_id = g.id) AS exposed",
	idCol:    "g.id",
	exFK:     "group_id",
	defSort:  "g.id",
	defOrder: "DESC",
	fields: []Field{
		col("id", "g.id", Int, "Group ID", intOps, true),
		col("title", "g.title", Text, "Title", textOps, true),
		col("description", "g.description", Text, "Description", textOps, false),
		enum(col("type", "g.type", Enum, "Type", enumOps, true), "basic_group,supergroup,channel"),
		col("msg_count", "g.msg_count", Int, "Messages", intOps, true),
		// Telegram's count as the daemon last stored it; NULL (no
		// match for any operator) where it has not yet.
		col("member_count", "g.member_count", Int, "Members", intOps, true),
		col("has_photo", "g.photo_file_id", Bool, "Has photo", nullOps, false),
		col("created_at", "g.created_at", Datetime, "Created", dtOps, true),
		col("updated_at", "g.updated_at", Datetime, "Updated", dtOps, true),
		exists("username", "Username (current)", "telegram_group_usernames x",
			"x.username", "AND x.kind='active'", existsOps),
		exists("hist_title", "Title (ever)", "telegram_group_hist_title x", "x.title", "",
			existsPos),
		exists("hist_description", "Description (ever)", "telegram_group_hist_description x",
			"x.description", "", existsPos),
		exists("hist_username", "Username (ever)", "telegram_group_hist_usernames_events x",
			"x.username", "", existsOps),
		// The groups page's: exposed through MCP, and the admin count
		// to sort by.
		internal(col("exposed", "(EXISTS (SELECT 1 FROM telegram_public_groups pg "+
			"WHERE pg.group_id = g.id))", Bool, "Public (MCP)", boolOps, false)),
		internal(col("admin_count", "(SELECT COUNT(*) FROM telegram_group_admins ga "+
			"WHERE ga.group_id = g.id)", Int, "Admins", 0, true)),
	},
	cols: []Col{
		{"photo", "", "photo", ""},
		{"id", "ID", "id", "id"},
		{"title", "Title", "name", "title"},
		{"username", "Username", "username", ""},
		{"type", "Type", "text", "type"},
		{"msg_count", "Messages", "int", "msg_count"},
		{"description", "Description", "longtext", ""},
		{"admin_count", "Admins", "int", ""},
		{"created_at", "Created", "datetime", "created_at"},
		{"updated_at", "Updated", "datetime", "updated_at"},
	},
	filter: func(sc scope.Scope) (string, []any) {
		if sc.SeesAllGroups() {
			return "", nil
		}
		return sc.GroupFilter("g.id")
	},
	cells: func(_ scope.Scope, r Row) map[string]any {
		admins := r.Str("admin_count")
		if r.Null("admin_count") {
			admins = "0"
		}
		return map[string]any{
			"photo":       photoID(r.Uint("photo_file_id")),
			"id":          r.Int("id"),
			"title":       esc(GroupTitle(r.Str("title"))),
			"username":    esc(r.Str("username")),
			"type":        r.Str("type"),
			"msg_count":   r.Str("msg_count"),
			"description": esc(r.Str("description")),
			"admin_count": admins,
			"created_at":  r.Str("created_at"),
			"updated_at":  r.Str("updated_at"),
		}
	},
}

// GroupTitle is a group's title as Drogon shows it, never empty.
func GroupTitle(t string) string {
	if t == "" {
		return "(untitled)"
	}
	return t
}

// --- files -------------------------------------------------------------

// HumanSize is a byte count as Drogon's files list shows it.
func HumanSize(n uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v, u := float64(n), 0
	for v >= 1024 && u < 4 {
		v /= 1024
		u++
	}
	if u == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", v, units[u])
}

// Files are the file store's rows. A scope that does not see every
// group and private chat sees the files sent in the messages it may
// see.
var Files = &Entity{
	Name: "files",
	from: "telegram_files f",
	sel: "f.id, f.file_type, f.file_ext, f.orig_file_name, " +
		"f.file_size, f.hit_count, f.on_disk, f.created_at, " +
		"f.accessed_at, f.tg_file_id, HEX(f.sha256) AS sha256_hex",
	idCol:    "f.id",
	defSort:  "f.id",
	defOrder: "DESC",
	fields: []Field{
		col("id", "f.id", Int, "File ID", intOps, true),
		enum(col("file_type", "f.file_type", Enum, "Type", enumOps, true),
			"photo,video,document,audio,voice,sticker,animation,video_note,thumbnail,unknown"),
		col("name", "f.orig_file_name", Text, "Name", textOps, true),
		col("ext", "COALESCE(f.file_ext,'')", Text, "Extension", textOps, true),
		col("size", "f.file_size", Int, "Size", intOps, true),
		col("hits", "f.hit_count", Int, "Hits", intOps, true),
		col("stored", "f.on_disk", Bool, "Stored", boolOps, false),
		col("tg_file_id", "f.tg_file_id", Text, "TG file id", textOps, false),
		// The digest through UNHEX(?), so the unique index is used;
		// exact only, and bad hex matches nothing.
		{Key: "sha256", Label: "SHA-256", Type: Text, Expr: "f.sha256",
			Ops: OpEQ | OpNE, Bind: "UNHEX(?)"},
		col("created_at", "f.created_at", Datetime, "Created", dtOps, true),
		col("updated_at", "f.updated_at", Datetime, "Updated", dtOps, true),
		col("accessed_at", "f.accessed_at", Datetime, "Last access", dtOps, true),
	},
	cols: []Col{
		{"thumb", "", "filethumb", ""},
		{"id", "ID", "id", "id"},
		{"file_type", "Type", "text", "file_type"},
		{"name", "Name", "text", "name"},
		{"ext", "Ext", "text", "ext"},
		{"size", "Size", "text", "size"},
		{"hits", "Hits", "int", "hits"},
		{"stored", "Stored", "bool", ""},
		{"created_at", "First seen", "datetime", "created_at"},
		{"accessed_at", "Last access", "datetime", "accessed_at"},
		{"tg_file_id", "Telegram file ID", "fileid", ""},
		{"sha256", "SHA-256", "hash", ""},
	},
	filter: filesFilter,
	cells: func(_ scope.Scope, r Row) map[string]any {
		return map[string]any{
			"thumb":       photoID(r.Uint("id")),
			"id":          r.Int("id"),
			"file_type":   r.Str("file_type"),
			"name":        esc(r.Str("orig_file_name")),
			"ext":         esc(r.Str("file_ext")),
			"size":        HumanSize(r.Uint("file_size")),
			"hits":        r.Str("hit_count"),
			"stored":      r.Bool("on_disk"),
			"created_at":  esc(r.Str("created_at")),
			"accessed_at": esc(r.Str("accessed_at")),
			"tg_file_id":  esc(r.Str("tg_file_id")),
			"sha256":      esc(r.Str("sha256_hex")),
		}
	},
}

func filesFilter(sc scope.Scope) (string, []any) {
	if sc.SeesAllGroups() && sc.SeesPrivate() {
		return "", nil
	}
	gf, a := sc.GroupFilter("fm.chat_id")
	cond := "EXISTS (SELECT 1 FROM telegram_group_messages fm " +
		"WHERE fm.file_id = f.id AND " + gf + ")"
	if sc.SeesPrivate() {
		cond += " OR EXISTS (SELECT 1 FROM telegram_private_messages fp " +
			"WHERE fp.file_id = f.id)"
	}
	return cond, a
}

// --- messages ----------------------------------------------------------

// MessageTypes are the content types of both message tables, in the
// order of their ENUM (daemon migration 000034 appended video_note to
// unsupported).
const MessageTypes = "text,photo,video,document,audio,voice,sticker,animation,service,unknown," +
	"video_note,location,venue,contact,dice,game,poll,checklist,story,paid_media,invoice," +
	"giveaway,call,rich,widget,expired,unsupported"

func nameCols(r Row, first, last string) string {
	return esc(DisplayName(r.Str(first), r.Str(last)))
}

func sentAt(r Row) string { return r.Str("sent_at") }

// kindSummaryCols are what the message lists summarise a row without
// text by: its content_extra (kind_extra) and its file's name
// (kind_file), read only for such a row.
const kindSummaryCols = "IF(COALESCE(m.text, '') = '', m.content_extra, NULL) AS kind_extra, " +
	"IF(COALESCE(m.text, '') = '', (SELECT kf.orig_file_name FROM telegram_files kf " +
	"WHERE kf.id = m.file_id), NULL) AS kind_file"

// PrivateMessages are the private chats' messages: all of them for a
// scope that sees private chats, and nothing (ErrNotFound) for one
// that does not.
var PrivateMessages = &Entity{
	Name: "private_messages",
	from: "telegram_private_messages m " +
		"LEFT JOIN telegram_users u ON u.id = m.chat_id",
	countFrom: "telegram_private_messages m",
	sel: "m.id, m.chat_id, m.message_id, m.is_outgoing, " +
		"m.is_forwarded, m.content_type, m.text, m.deleted_at, " +
		"FROM_UNIXTIME(m.date) AS sent_at, " +
		"u.first_name AS peer_first, u.last_name AS peer_last, " +
		"u.profile_photo_file_id AS peer_photo, " +
		"(SELECT un.username FROM telegram_user_usernames un " +
		"WHERE un.user_id = m.chat_id AND un.kind='active' " +
		"ORDER BY un.position LIMIT 1) AS peer_username",
	pageSel:  "m.date AS date_unix, m.edit_date, u.type AS peer_type, m.service_type, " + kindSummaryCols,
	idCol:    "m.id",
	defSort:  "m.date",
	defOrder: "DESC",
	fields: []Field{
		col("text", "m.text", FullText, "Text", OpMatch, false),
		enum(col("content_type", "m.content_type", Enum, "Content type", enumOps, false),
			MessageTypes),
		col("chat_id", "m.chat_id", Int, "User (chat) id", intOps, false),
		needsChat(col("tdlib_type", "m.tdlib_type", Text, "TDLib type", OpEQ|OpNE, false)),
		needsChat(col("service_type", "m.service_type", Text, "Service type", OpEQ|OpNE, false)),
		col("sender_id", "m.sender_id", Int, "Sender id", intOps, false),
		col("message_id", "m.message_id", Int, "Message id", intOps, false),
		col("is_outgoing", "m.is_outgoing", Bool, "Outgoing", boolOps, false),
		col("is_forwarded", "m.is_forwarded", Bool, "Forwarded", boolOps, false),
		col("deleted", "m.deleted_at", Bool, "Deleted", nullOps, false),
		col("date", "m.date", Int, "Sent", intOps, true),
		col("created_at", "m.created_at", Datetime, "Logged", dtOps, true),
		internal(col("is_edited", "(m.edit_date <> 0)", Bool, "Edited", boolOps, false)),
	},
	cols: []Col{
		{"user", "User", "party", ""},
		{"outgoing", "Out", "bool", ""},
		{"content_type", "Type", "text", ""},
		{"text", "Text", "longtext", ""},
		{"forwarded", "Fwd", "bool", ""},
		{"message_id", "Msg id", "int", ""},
		{"sent", "Sent", "datetime", "date"},
		{"deleted", "Deleted", "bool", ""},
	},
	// Who may see private chats at all is the entity's visibility
	// (scope.Page): every private chat or none.
	filter: func(scope.Scope) (string, []any) { return "", nil },
	cells: func(_ scope.Scope, r Row) map[string]any {
		return map[string]any{
			"user": party{Kind: "user", ID: r.Int("chat_id"),
				Name:     nameCols(r, "peer_first", "peer_last"),
				Username: esc(r.Str("peer_username")), Photo: photoID(r.Uint("peer_photo"))},
			"outgoing":     r.Bool("is_outgoing"),
			"content_type": r.Str("content_type"),
			"text":         esc(r.Str("text")),
			"forwarded":    r.Bool("is_forwarded"),
			"message_id":   r.Str("message_id"),
			"sent":         sentAt(r),
			"deleted":      !r.Null("deleted_at"),
		}
	},
}

// GroupMessages are the groups' messages, limited by GroupFilter. Only
// indexed columns are searchable through the API: the table holds
// millions of rows.
var GroupMessages = &Entity{
	Name: "group_messages",
	from: "telegram_group_messages m " +
		"LEFT JOIN `telegram_groups` g ON g.id = m.chat_id " +
		"LEFT JOIN telegram_users su ON su.id = m.sender_user_id " +
		"LEFT JOIN `telegram_groups` sg ON sg.id = m.sender_chat_id",
	countFrom: "telegram_group_messages m",
	sel: "m.id, m.chat_id, m.message_id, m.sender_user_id, " +
		"m.sender_chat_id, m.is_forwarded, " +
		"m.content_type, m.text, m.deleted_at, " +
		"FROM_UNIXTIME(m.date) AS sent_at, " +
		"g.title AS group_title, g.photo_file_id AS group_photo, " +
		"su.first_name AS su_first, su.last_name AS su_last, " +
		"su.profile_photo_file_id AS su_photo, " +
		"(SELECT un.username FROM telegram_user_usernames un " +
		"WHERE un.user_id = m.sender_user_id AND un.kind='active' " +
		"ORDER BY un.position LIMIT 1) AS su_username, " +
		"sg.title AS sg_title, sg.photo_file_id AS sg_photo",
	pageSel: "m.date AS date_unix, m.edit_date, su.type AS su_type, m.service_type, " +
		"(SELECT gu.username FROM telegram_group_usernames gu " +
		"WHERE gu.group_id = m.sender_chat_id AND gu.kind='active' " +
		"ORDER BY gu.position LIMIT 1) AS sg_username, " + kindSummaryCols,
	idCol:          "m.id",
	defSort:        "m.date",
	defOrder:       "DESC",
	estimateBrowse: true,
	fields: []Field{
		col("text", "m.text", FullText, "Text", OpMatch, false),
		enum(col("content_type", "m.content_type", Enum, "Content type", enumOps, false),
			MessageTypes),
		col("chat_id", "m.chat_id", Int, "Group id", intOps, false),
		needsChat(col("tdlib_type", "m.tdlib_type", Text, "TDLib type", OpEQ|OpNE, false)),
		needsChat(col("service_type", "m.service_type", Text, "Service type", OpEQ|OpNE, false)),
		col("sender_user_id", "m.sender_user_id", Int, "Sender user id", intOps, false),
		col("deleted", "m.deleted_at", Bool, "Deleted", nullOps, false),
		col("date", "m.date", Int, "Sent", intOps, true),
		// The messages page's flags; not indexed, so the statement
		// time limit bounds a search by them alone.
		internal(col("is_forwarded", "m.is_forwarded", Bool, "Forwarded", boolOps, false)),
		internal(col("is_edited", "(m.edit_date <> 0)", Bool, "Edited", boolOps, false)),
	},
	cols: []Col{
		{"group", "Group", "party", ""},
		{"sender", "Sender", "party", ""},
		{"content_type", "Type", "text", ""},
		{"text", "Text", "longtext", ""},
		{"forwarded", "Fwd", "bool", ""},
		{"message_id", "Msg id", "int", ""},
		{"sent", "Sent", "datetime", "date"},
		{"deleted", "Deleted", "bool", ""},
	},
	filter: func(sc scope.Scope) (string, []any) {
		if sc.SeesAllGroups() {
			return "", nil
		}
		return sc.GroupFilter("m.chat_id")
	},
	cells: func(sc scope.Scope, r Row) map[string]any {
		var sender party
		switch {
		case !r.Null("sender_user_id"):
			sender = party{Kind: "user", ID: r.Int("sender_user_id"),
				Name:     nameCols(r, "su_first", "su_last"),
				Username: esc(r.Str("su_username")), Photo: photoID(r.Uint("su_photo"))}
		case !r.Null("sender_chat_id") && sc.SeesGroup(r.Int("sender_chat_id")):
			sender = party{Kind: "group", ID: r.Int("sender_chat_id"),
				Name: esc(GroupTitle(r.Str("sg_title"))), Photo: photoID(r.Uint("sg_photo"))}
		case !r.Null("sender_chat_id"):
			// A chat the viewer may not see posted it: not named.
			sender = party{Kind: "none", Name: "(hidden)", Photo: photoID(0)}
		default:
			sender = party{Kind: "none", Name: "(unknown)", Photo: photoID(0)}
		}
		return map[string]any{
			"group": party{Kind: "group", ID: r.Int("chat_id"),
				Name: esc(GroupTitle(r.Str("group_title"))), Photo: photoID(r.Uint("group_photo"))},
			"sender":       sender,
			"content_type": r.Str("content_type"),
			"text":         esc(r.Str("text")),
			"forwarded":    r.Bool("is_forwarded"),
			"message_id":   r.Str("message_id"),
			"sent":         sentAt(r),
			"deleted":      !r.Null("deleted_at"),
		}
	},
}

// Entities are the searchable entities by name.
var Entities = map[string]*Entity{
	Users.Name: Users, Groups.Name: Groups, Files.Name: Files,
	PrivateMessages.Name: PrivateMessages, GroupMessages.Name: GroupMessages,
}

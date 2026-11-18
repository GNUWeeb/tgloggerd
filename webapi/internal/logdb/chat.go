// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/scope"
)

// The chat view's message windows, as web/src/dao/Browse.cpp
// (chatHistory) reads them: one rich row per message, then its edit
// history and link preview in one batch per window, then the senders.

// Chat is a chat the request may read: OpenChat returns one only after
// the scope (or, for a group, its exposure) allowed it, so a window is
// never read without that check.
type Chat struct {
	Group bool
	ID    int64 // the group's chat id, or the other user's id
	// Exposed is whether the group is exposed (telegram_public_groups):
	// its chat is public.
	Exposed bool
	// Count is the private chat's message count; for a group, its
	// msg_count is in GroupInfo.
	Count     int64
	GroupInfo Group
	UserInfo  User
	sc        scope.Scope
}

// OpenChat is the chat a request asks for: a group the scope sees, or
// that is exposed; a private chat when the scope sees private chats.
// Anything else is ErrNotFound, whether or not it exists.
func OpenChat(ctx context.Context, db *sql.DB, sc scope.Scope, group bool, id int64) (Chat, error) {
	c := Chat{Group: group, ID: id, sc: sc}
	if group {
		if id >= 0 {
			return Chat{}, ErrNotFound
		}
		cond, args := sc.GroupFilter("g.id")
		rows, err := db.QueryContext(ctx, groupQuery+"WHERE g.id = ? AND ("+cond+
			" OR pg.group_id IS NOT NULL)", append([]any{id}, args...)...)
		if err != nil {
			return Chat{}, err
		}
		gs, err := scanGroups(rows)
		if err != nil {
			return Chat{}, err
		}
		if len(gs) == 0 {
			return Chat{}, ErrNotFound
		}
		c.GroupInfo, c.Exposed = gs[0], gs[0].Exposed
		return c, nil
	}
	if id <= 0 || !sc.SeesPrivate() {
		return Chat{}, ErrNotFound
	}
	us, err := usersByID(ctx, db, sc, []int64{id})
	if err != nil {
		return Chat{}, err
	}
	u, ok := us[id]
	if !ok {
		return Chat{}, ErrNotFound
	}
	c.UserInfo = u
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegram_private_messages "+
		"WHERE chat_id = ?", id).Scan(&c.Count); err != nil {
		return Chat{}, err
	}
	return c, nil
}

// Anchor is where a window lies: at most one of Before, After and
// Around; none is the newest messages.
type Anchor struct {
	Before, After, Around *int64
}

// Reply is what a message replies to.
type Reply struct {
	MessageID int64 // the target's message id
	ChatID    int64 // the target's chat
	// Resolved: the target is logged (same table), and the fields
	// below describe it.
	Resolved    bool
	RowID       uint64
	SenderUser  int64
	SenderChat  int64
	Outgoing    bool
	Snippet     string
	ContentType string
	Deleted     bool // the target is marked deleted
}

// Forward is a forwarded message's origin.
type Forward struct {
	Type       string // user, hidden_user, chat, channel
	SenderName string
	UserID     int64
	ChatID     int64
	MessageID  int64 // the original message's id in ChatID, 0 = unknown
	Date       int64 // when the original was sent (unix), 0 = unknown
}

// Edit is a version of a message before an edit.
type Edit struct {
	At          int64 // when the version began (unix)
	Text        string
	ContentType string
	File        FileRef // ID 0 = none
	// Extra is the version's content_extra (docs/design/message-types.md
	// section 4.2), nil when it has none.
	Extra []byte
}

// LinkPreview is a message's link preview card.
type LinkPreview struct {
	URL         string
	Site        string
	Title       string
	Description string
	Photo       FileRef // ID 0 = none
}

// Message is a logged message.
type Message struct {
	RowID           uint64
	ChatID          int64
	MessageID       int64
	SenderUser      int64 // 0 = none
	SenderChat      int64 // 0 = none
	Outgoing        bool
	AuthorSignature string
	IsChannelPost   bool
	Date            int64
	EditDate        int64 // 0 = never edited
	ContentType     string
	ServiceType     string // a service message's action, or ""
	TdlibType       string // the exact TDLib class, or "" when not recorded
	// Extra is content_extra (docs/design/message-types.md section
	// 4.2): written for rows logged since migration 000034, nil for
	// older ones.
	Extra []byte
	Text  string
	File  FileRef // ID 0 = none
	// Cover is a video's cover photo (telegram_message_files, role
	// cover, since migration 000036); ID 0 = none.
	Cover     FileRef
	DeletedAt int64 // 0 = not deleted
	AlbumID   int64 // 0 = none
	Reply     *Reply
	Forward   *Forward
	Edits     []Edit
	Preview   *LinkPreview
}

// Window is a run of a chat's messages, oldest first, with whether
// more lie either side, and the users and groups its rows name.
type Window struct {
	Messages []Message
	HasOlder bool
	HasNewer bool
	Users    map[int64]User
	Groups   map[int64]Group
}

type chatTables struct {
	msgs, edits, fwd, fk string
	// cols are the columns that differ between the two tables.
	senderUser, senderChat, outgoing, signature, channelPost string
	rSenderUser, rSenderChat, rOutgoing                      string
}

var (
	groupTables = chatTables{
		msgs: "telegram_group_messages", edits: "telegram_group_message_edits",
		fwd: "telegram_group_message_fwd_info", fk: "group_message_id",
		senderUser: "m.sender_user_id", senderChat: "m.sender_chat_id", outgoing: "0",
		signature: "m.author_signature", channelPost: "m.is_channel_post",
		rSenderUser: "rm.sender_user_id", rSenderChat: "rm.sender_chat_id", rOutgoing: "0",
	}
	privateTables = chatTables{
		msgs: "telegram_private_messages", edits: "telegram_private_message_edits",
		fwd: "telegram_private_message_fwd_info", fk: "private_message_id",
		senderUser: "m.sender_id", senderChat: "NULL", outgoing: "m.is_outgoing",
		signature: "NULL", channelPost: "0",
		rSenderUser: "rm.sender_id", rSenderChat: "NULL", rOutgoing: "rm.is_outgoing",
	}
)

func tablesOf(group bool) chatTables {
	if group {
		return groupTables
	}
	return privateTables
}

// messageSelect is the rich row of a message, without its WHERE.
func (t chatTables) messageSelect() string {
	return "SELECT m.id, m.chat_id, m.message_id, " + t.senderUser + ", " + t.senderChat +
		", " + t.outgoing + ", " + t.signature + ", " + t.channelPost + ", m.date, m.edit_date, " +
		"m.content_type, COALESCE(m.service_type, ''), COALESCE(m.tdlib_type, ''), " +
		"m.content_extra, COALESCE(m.text, ''), COALESCE(m.file_id, 0), " + fileColumnsOf("f") + ", " +
		"UNIX_TIMESTAMP(m.deleted_at), m.media_album_id, m.reply_to_chat_id, " +
		"m.reply_to_msg_id, fw.origin_type, fw.origin_sender_name, " +
		"fw.origin_sender_user_id, fw.origin_chat_id, fw.origin_message_id, fw.origin_date, " +
		"rm.id, rm.chat_id, rm.message_id, " +
		t.rSenderUser + ", " + t.rSenderChat + ", " + t.rOutgoing + ", " +
		"LEFT(COALESCE(rm.text, ''), 120), rm.content_type, rm.deleted_at IS NOT NULL " +
		"FROM " + t.msgs + " m " +
		"LEFT JOIN telegram_files f ON f.id = m.file_id " +
		"LEFT JOIN " + t.fwd + " fw ON fw." + t.fk + " = m.id " +
		"LEFT JOIN " + t.msgs + " rm ON rm.id = m.reply_to_id "
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var (
			m                       Message
			sUser, sChat            sql.NullInt64
			sig                     sql.NullString
			fileID                  uint64
			f                       nullFile
			deleted, album          sql.NullInt64
			rChat, rMsg             sql.NullInt64
			oType, oName            sql.NullString
			oUser, oChat            sql.NullInt64
			oMsg, oDate             sql.NullInt64
			rmID                    sql.NullInt64
			rmChat, rmMsg, rmU, rmC sql.NullInt64
			rmOut, rmDeleted        sql.NullBool
			rmSnippet, rmCtype      sql.NullString
		)
		dest := []any{&m.RowID, &m.ChatID, &m.MessageID, &sUser, &sChat, &m.Outgoing, &sig,
			&m.IsChannelPost, &m.Date, &m.EditDate, &m.ContentType, &m.ServiceType,
			&m.TdlibType, &m.Extra, &m.Text, &fileID}
		dest = append(dest, f.dest()...)
		dest = append(dest, &deleted, &album, &rChat, &rMsg, &oType, &oName, &oUser, &oChat,
			&oMsg, &oDate, &rmID, &rmChat, &rmMsg, &rmU, &rmC, &rmOut, &rmSnippet, &rmCtype,
			&rmDeleted)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		m.SenderUser, m.SenderChat = sUser.Int64, sChat.Int64
		m.AuthorSignature = sig.String
		if fileID != 0 {
			m.File = f.ref(fileID)
		}
		m.DeletedAt, m.AlbumID = deleted.Int64, album.Int64
		switch {
		case rmID.Valid:
			m.Reply = &Reply{MessageID: rmMsg.Int64, ChatID: rmChat.Int64, Resolved: true,
				RowID: uint64(rmID.Int64), SenderUser: rmU.Int64, SenderChat: rmC.Int64,
				Outgoing: rmOut.Bool, Snippet: rmSnippet.String, ContentType: rmCtype.String,
				Deleted: rmDeleted.Bool}
		case rMsg.Valid:
			chat := m.ChatID
			if rChat.Valid {
				chat = rChat.Int64
			}
			m.Reply = &Reply{MessageID: rMsg.Int64, ChatID: chat}
		}
		if oType.Valid {
			m.Forward = &Forward{Type: oType.String, SenderName: oName.String,
				UserID: oUser.Int64, ChatID: oChat.Int64, MessageID: oMsg.Int64,
				Date: oDate.Int64}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// albumEdge is how many more rows of an album cut by a window's edge
// are read to complete it: Telegram albums hold at most ten items.
const albumEdge = 10

// Window reads the window of c at a with at most limit messages, plus
// the rest of an album the window's edges cut, so that a message folds
// into the same album bubble whichever window it comes in.
func (c Chat) Window(ctx context.Context, db *sql.DB, a Anchor, limit int) (Window, error) {
	t := tablesOf(c.Group)
	sel := t.messageSelect() + "WHERE m.chat_id = ? "
	var (
		ms  []Message
		err error
	)
	query := func(cond string, desc bool, n int, args ...any) ([]Message, error) {
		order := "ASC"
		if desc {
			order = "DESC"
		}
		q := sel + cond + " ORDER BY m.message_id " + order + " LIMIT ?"
		rows, err := db.QueryContext(ctx, q, append(append([]any{c.ID}, args...), n)...)
		if err != nil {
			return nil, err
		}
		got, err := scanMessages(rows)
		if desc {
			slices.Reverse(got)
		}
		return got, err
	}
	switch {
	case a.Before != nil:
		ms, err = query("AND m.message_id < ?", true, limit, *a.Before)
	case a.After != nil:
		ms, err = query("AND m.message_id > ?", false, limit, *a.After)
	case a.Around != nil:
		// The message itself and the older half, then the newer ones
		// to fill the window.
		ms, err = query("AND m.message_id <= ?", true, limit-limit/2, *a.Around)
		if err == nil {
			var newer []Message
			newer, err = query("AND m.message_id > ?", false, limit-len(ms), *a.Around)
			ms = append(ms, newer...)
		}
	default:
		ms, err = query("", true, limit)
	}
	if err != nil {
		return Window{}, err
	}
	w := Window{Messages: ms}
	if len(ms) > 0 {
		if first := ms[0]; first.AlbumID != 0 {
			more, err := query("AND m.media_album_id = ? AND m.message_id < ?", true,
				albumEdge, first.AlbumID, first.MessageID)
			if err != nil {
				return Window{}, err
			}
			w.Messages = append(more, w.Messages...)
		}
		if last := ms[len(ms)-1]; last.AlbumID != 0 {
			more, err := query("AND m.media_album_id = ? AND m.message_id > ?", false,
				albumEdge, last.AlbumID, last.MessageID)
			if err != nil {
				return Window{}, err
			}
			w.Messages = append(w.Messages, more...)
		}
		lo, hi := w.Messages[0].MessageID, w.Messages[len(w.Messages)-1].MessageID
		if w.HasOlder, err = c.exists(ctx, db, "message_id < ?", lo); err != nil {
			return Window{}, err
		}
		if w.HasNewer, err = c.exists(ctx, db, "message_id > ?", hi); err != nil {
			return Window{}, err
		}
	} else if a.Before != nil || a.After != nil || a.Around != nil {
		// An empty window past one end still says whether the chat
		// goes on at the other.
		var at int64
		switch {
		case a.Before != nil:
			at = *a.Before
		case a.After != nil:
			at = *a.After
		default:
			at = *a.Around
		}
		if w.HasOlder, err = c.exists(ctx, db, "message_id < ?", at); err != nil {
			return Window{}, err
		}
		if w.HasNewer, err = c.exists(ctx, db, "message_id > ?", at); err != nil {
			return Window{}, err
		}
	}
	if err := c.complete(ctx, db, w.Messages); err != nil {
		return Window{}, err
	}
	if err := c.names(ctx, db, &w); err != nil {
		return Window{}, err
	}
	return w, nil
}

func (c Chat) exists(ctx context.Context, db *sql.DB, cond string, arg int64) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1 FROM "+tablesOf(c.Group).msgs+
		" WHERE chat_id = ? AND "+cond+" LIMIT 1", c.ID, arg).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// DateAnchor is the first message at or after the unix time ts, for a
// jump to a date; ok is false when nothing is that recent. It orders by
// date alone, which the (chat_id, date, ...) index gives without a
// sort.
func (c Chat) DateAnchor(ctx context.Context, db *sql.DB, ts int64) (int64, bool, error) {
	var id int64
	err := db.QueryRowContext(ctx, "SELECT message_id FROM "+tablesOf(c.Group).msgs+
		" WHERE chat_id = ? AND date >= ? ORDER BY date LIMIT 1", c.ID, ts).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// KnownMembers is how many people the archive has seen in a group:
// the distinct users who sent a message there and its administrators.
// The logger does not record Telegram's own member count, so this is
// what the chat header can say; 0 for a private chat. The senders are
// counted from the (chat_id, sender_user_id) index alone.
func (c Chat) KnownMembers(ctx context.Context, db *sql.DB) (int64, error) {
	if !c.Group {
		return 0, nil
	}
	var n int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+
		"SELECT sender_user_id AS uid FROM telegram_group_messages "+
		"WHERE chat_id = ? AND sender_user_id IS NOT NULL GROUP BY sender_user_id "+
		"UNION SELECT user_id FROM telegram_group_admins WHERE group_id = ?) known",
		c.ID, c.ID).Scan(&n)
	return n, err
}

func idList(ms []Message) string {
	var b strings.Builder
	for i, m := range ms {
		if i > 0 {
			b.WriteString(",")
		}
		// Our own integers, so inlining them is safe.
		b.WriteString(strconv.FormatUint(m.RowID, 10))
	}
	return b.String()
}

// complete adds the edit histories and link previews of ms, one query
// each for the whole window.
func (c Chat) complete(ctx context.Context, db *sql.DB, ms []Message) error {
	if len(ms) == 0 {
		return nil
	}
	return completeMessages(ctx, db, c.Group, ms)
}

func completeMessages(ctx context.Context, db *sql.DB, group bool, ms []Message) error {
	t := tablesOf(group)
	byRow := make(map[uint64]*Message, len(ms))
	var edited []Message
	for i := range ms {
		byRow[ms[i].RowID] = &ms[i]
		if ms[i].EditDate > 0 {
			edited = append(edited, ms[i])
		}
	}
	if len(edited) > 0 {
		rows, err := db.QueryContext(ctx, "SELECT e."+t.fk+", e.edit_date, "+
			"COALESCE(e.text, ''), e.content_type, COALESCE(e.file_id, 0), "+fileColumnsOf("f")+
			", e.content_extra FROM "+t.edits+" e LEFT JOIN telegram_files f ON f.id = e.file_id "+
			"WHERE e."+t.fk+" IN ("+idList(edited)+") ORDER BY e.id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var (
				mid    uint64
				e      Edit
				fileID uint64
				f      nullFile
			)
			dest := append([]any{&mid, &e.At, &e.Text, &e.ContentType, &fileID}, f.dest()...)
			if err := rows.Scan(append(dest, &e.Extra)...); err != nil {
				rows.Close()
				return err
			}
			if fileID != 0 {
				e.File = f.ref(fileID)
			}
			if m := byRow[mid]; m != nil {
				// The first version began when the message was sent.
				if e.At == 0 {
					e.At = m.Date
				}
				m.Edits = append(m.Edits, e)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	if err := completeCovers(ctx, db, t, byRow, ms); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "SELECT p."+t.fk+", p.url, p.site_name, p.title, "+
		"LEFT(COALESCE(p.description, ''), 1000), COALESCE(p.photo_file_id, 0), "+
		fileColumnsOf("f")+" FROM telegram_link_previews p "+
		"LEFT JOIN telegram_files f ON f.id = p.photo_file_id WHERE p."+t.fk+
		" IN ("+idList(ms)+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			mid   uint64
			p     LinkPreview
			photo uint64
			f     nullFile
		)
		dest := append([]any{&mid, &p.URL, &p.Site, &p.Title, &p.Description, &photo}, f.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		if photo != 0 {
			p.Photo = f.ref(photo)
		}
		if m := byRow[mid]; m != nil {
			m.Preview = &p
		}
	}
	return rows.Err()
}

// completeCovers adds the cover photos of the videos among ms.
func completeCovers(ctx context.Context, db *sql.DB, t chatTables, byRow map[uint64]*Message, ms []Message) error {
	var videos []Message
	for _, m := range ms {
		if m.ContentType == "video" {
			videos = append(videos, m)
		}
	}
	if len(videos) == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, "SELECT mf."+t.fk+", mf.file_id, "+fileColumnsOf("f")+
		" FROM telegram_message_files mf JOIN telegram_files f ON f.id = mf.file_id "+
		"WHERE mf."+t.fk+" IN ("+idList(videos)+") AND mf.role = 'cover' AND mf.position = 0")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			mid, fileID uint64
			f           nullFile
		)
		if err := rows.Scan(append([]any{&mid, &fileID}, f.dest()...)...); err != nil {
			return err
		}
		if m := byRow[mid]; m != nil {
			m.Cover = f.ref(fileID)
		}
	}
	return rows.Err()
}

// names loads the users and groups the window's messages name: senders,
// reply targets' senders, forward origins.
func (c Chat) names(ctx context.Context, db *sql.DB, w *Window) error {
	var users, groups []int64
	for _, m := range w.Messages {
		users = append(users, m.SenderUser)
		groups = append(groups, m.SenderChat)
		if r := m.Reply; r != nil && r.Resolved {
			users = append(users, r.SenderUser)
			groups = append(groups, r.SenderChat)
		}
		if f := m.Forward; f != nil {
			users = append(users, f.UserID)
			groups = append(groups, f.ChatID)
		}
	}
	var err error
	if w.Users, err = usersByID(ctx, db, c.sc, users); err != nil {
		return err
	}
	w.Groups, err = groupsByID(ctx, db, groups)
	return err
}

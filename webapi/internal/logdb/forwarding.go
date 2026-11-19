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
	"strings"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/scope"
)

// The forwarding configurations, in both directions, as the Drogon
// web's dao::telegram_discord and dao::discord_telegram keep them
// (web/docs/platform-forwarding.md): Telegram to Discord webhooks
// (telegram_discord_webhooks) and Discord to Telegram routes
// (discord_telegram_routes) with their bots (bots.go). They are
// the logger's tables; the web's read-only account is granted DML on
// exactly these, so the writes below go through the same pool as the
// reads.
//
// Every read takes the request scope: a configuration is visible when
// its Telegram chat is (a group the scope sees, a private chat to a
// scope that sees private chats), so a configuration touching a hidden
// group does not exist for the request (web-accounts.md section 6).
// Ownership (webdb ownership.go) narrows this further with
// forwarding-per-user, in the same condition.
//
// The secrets, the webhook URL and the bot token, are write-only: the
// token is never selected, and the URL is read only for sending a test
// message and for masking; neither is ever answered.

// FwdDir is one forwarding direction's content types: each key is a
// fwd_<key> column of its table, and Keys is the order the route form
// lists them in (decision 2026-11-20-fwd-more-types), which is also the
// API's type_options. The daemons read the same columns into a bitmask
// of their own, so there is no bit-number contract to keep in step.
type FwdDir struct {
	Keys []string
}

// T2D is Telegram to Discord: a flag per type an admin would separate,
// other for every kind without one (rich messages, calls, giveaways and
// kinds added later).
var T2D = FwdDir{Keys: []string{
	"text",
	"photo", "video", "animation", "video_note", "document", "sticker",
	"voice", "audio",
	"poll", "checklist", "dice", "game",
	"location", "contact",
	"story", "paid_media", "invoice",
	"service",
	"other",
}}

// D2T is Discord to Telegram: an attachment is audio (audio/*), an
// animation (image/gif, and animated stickers), a photo (image/*), a
// video (video/*) or else a document.
var D2T = FwdDir{Keys: []string{
	"text", "photo", "video", "animation", "document", "sticker", "audio",
}}

// FwdTypes is the set of types a route forwards, by key.
type FwdTypes map[string]bool

// List is the keys of t, in the direction's order.
func (d FwdDir) List(t FwdTypes) []string {
	out := []string{}
	for _, k := range d.Keys {
		if t[k] {
			out = append(out, k)
		}
	}
	return out
}

// Parse reads a types list of the direction: an unknown key or an
// empty list fails (disabling a route is how it stops). Repeats count
// once.
func (d FwdDir) Parse(keys []string) (FwdTypes, bool) {
	t := FwdTypes{}
	for _, k := range keys {
		if !slices.Contains(d.Keys, k) {
			return nil, false
		}
		t[k] = true
	}
	return t, len(t) > 0
}

// args is t's flags in the order of columns.
func (d FwdDir) args(t FwdTypes) []any {
	out := make([]any, len(d.Keys))
	for i, k := range d.Keys {
		out[i] = t[k]
	}
	return out
}

// columns is the fwd_<key> columns, with a table alias such as "w.".
func (d FwdDir) columns(alias string) string {
	cols := make([]string, len(d.Keys))
	for i, k := range d.Keys {
		cols[i] = alias + "fwd_" + k
	}
	return strings.Join(cols, ", ")
}

// assignments is "fwd_<key> = ?, ..." for an UPDATE.
func (d FwdDir) assignments() string {
	return strings.ReplaceAll(d.columns(""), ",", " = ?,") + " = ?"
}

// fwdScan reads a row's flags, in the order of columns.
type fwdScan struct {
	d     FwdDir
	flags []bool
}

func (d FwdDir) scanner() *fwdScan {
	return &fwdScan{d: d, flags: make([]bool, len(d.Keys))}
}

func (f *fwdScan) dest() []any {
	out := make([]any, len(f.flags))
	for i := range f.flags {
		out[i] = &f.flags[i]
	}
	return out
}

func (f *fwdScan) types() FwdTypes {
	t := FwdTypes{}
	for i, k := range f.d.Keys {
		if f.flags[i] {
			t[k] = true
		}
	}
	return t
}

// ChatFilter is the condition that keeps rows whose Telegram chat id,
// in column (a constant of the caller's), is visible: a group the
// scope sees or, to a scope that sees private chats, a private chat.
func ChatFilter(sc scope.Scope, column string) (string, []any) {
	gf, ga := sc.GroupFilter(column)
	priv := "1 = 0"
	if sc.SeesPrivate() {
		priv = "1 = 1"
	}
	return "((" + column + " < 0 AND " + gf + ") OR (" + column + " > 0 AND " + priv + "))", ga
}

// FwdChat is the Telegram chat of a configuration.
type FwdChat struct {
	ID    int64
	Kind  string // "group" or "user"
	Title string
	Type  string // a group's type, "" for a user
	// Photo is the group's photo or the user's profile photo, ID 0
	// for none.
	Photo FileRef
}

// chatJoins and chatColumns resolve a configuration's chat title and
// photo live, as the Drogon lists do, instead of a stored copy that
// goes stale. The photo's file row comes in the same join, so a list
// costs one query however many chats it names.
func chatJoins(col string) string {
	return "LEFT JOIN telegram_groups g ON " + col + " < 0 AND g.id = " + col + " " +
		"LEFT JOIN telegram_users u ON " + col + " > 0 AND u.id = " + col + " " +
		"LEFT JOIN telegram_files cf ON cf.id = COALESCE(g.photo_file_id, u.profile_photo_file_id) "
}

var chatColumns = "g.title, g.type, u.first_name, u.last_name, " +
	"COALESCE(g.photo_file_id, u.profile_photo_file_id, 0), " + fileColumnsOf("cf")

// chatCols scans chatColumns.
type chatCols struct {
	title, gtype, first, last sql.NullString
	photo                     uint64
	file                      nullFile
}

func (c *chatCols) dest() []any {
	return append([]any{&c.title, &c.gtype, &c.first, &c.last, &c.photo}, c.file.dest()...)
}

func (c *chatCols) chat(id int64) FwdChat {
	if id < 0 {
		return FwdChat{ID: id, Kind: "group", Title: GroupTitle(c.title.String), Type: c.gtype.String,
			Photo: c.file.ref(c.photo)}
	}
	return FwdChat{ID: id, Kind: "user", Title: DisplayName(c.first.String, c.last.String),
		Photo: c.file.ref(c.photo)}
}

// ResolveChat is a chat the logger has seen and the scope sees, for a
// new configuration: a group from telegram_groups, a private chat from
// telegram_users. A chat never seen, or hidden, is not found.
func ResolveChat(ctx context.Context, db *sql.DB, sc scope.Scope, id int64) (FwdChat, bool, error) {
	if id == 0 || !sc.SeesChat(id) {
		return FwdChat{}, false, nil
	}
	var c chatCols
	var err error
	if id < 0 {
		gf, ga := sc.GroupFilter("g.id")
		err = db.QueryRowContext(ctx, "SELECT g.title, g.type, COALESCE(g.photo_file_id, 0), "+
			fileColumnsOf("cf")+" FROM telegram_groups g "+
			"LEFT JOIN telegram_files cf ON cf.id = g.photo_file_id "+
			"WHERE g.id = ? AND "+gf, append([]any{id}, ga...)...).
			Scan(append([]any{&c.title, &c.gtype, &c.photo}, c.file.dest()...)...)
	} else {
		err = db.QueryRowContext(ctx, "SELECT u.first_name, u.last_name, "+
			"COALESCE(u.profile_photo_file_id, 0), "+fileColumnsOf("cf")+" FROM telegram_users u "+
			"LEFT JOIN telegram_files cf ON cf.id = u.profile_photo_file_id "+
			"WHERE u.id = ?", id).
			Scan(append([]any{&c.first, &c.last, &c.photo}, c.file.dest()...)...)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return FwdChat{}, false, nil
	}
	if err != nil {
		return FwdChat{}, false, err
	}
	return c.chat(id), true, nil
}

// T2DWebhook is a telegram_discord_webhooks row.
type T2DWebhook struct {
	ID      uint64
	Chat    FwdChat
	URL     string // secret: never answered
	Enabled bool
	Types   FwdTypes
	// Sent is how many posts were made to its URL (a message with a
	// caption and media counts twice); LastSent the newest, unix
	// seconds, 0 for none.
	Sent     int64
	LastSent int64
}

func t2dQuery(sc scope.Scope, where string) (string, []any) {
	cf, ca := ChatFilter(sc, "w.telegram_chat_id")
	q := "SELECT w.id, w.telegram_chat_id, w.webhook_url, w.enabled, " +
		T2D.columns("w.") + ", " + chatColumns + ", " +
		"(SELECT COUNT(*) FROM telegram_discord_sent_messages sm WHERE sm.endpoint_id = e.id), " +
		"COALESCE(UNIX_TIMESTAMP((SELECT sm.created_at FROM telegram_discord_sent_messages sm " +
		"WHERE sm.endpoint_id = e.id ORDER BY sm.id DESC LIMIT 1)), 0) " +
		"FROM telegram_discord_webhooks w " + chatJoins("w.telegram_chat_id") +
		"LEFT JOIN discord_endpoints e ON e.webhook_url = w.webhook_url " +
		"WHERE " + cf + where + " ORDER BY w.id DESC"
	return q, ca
}

func scanT2D(rows *sql.Rows) ([]T2DWebhook, error) {
	defer rows.Close()
	out := []T2DWebhook{}
	for rows.Next() {
		var (
			w    T2DWebhook
			chat int64
			c    chatCols
		)
		f := T2D.scanner()
		dest := append([]any{&w.ID, &chat, &w.URL, &w.Enabled}, f.dest()...)
		dest = append(append(dest, c.dest()...), &w.Sent, &w.LastSent)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		w.Types = f.types()
		w.Chat = c.chat(chat)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListT2D is the webhooks the scope sees, newest first.
func ListT2D(ctx context.Context, db *sql.DB, sc scope.Scope) ([]T2DWebhook, error) {
	q, args := t2dQuery(sc, "")
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return scanT2D(rows)
}

// GetT2D is one webhook the scope sees.
func GetT2D(ctx context.Context, db *sql.DB, sc scope.Scope, id uint64) (T2DWebhook, bool, error) {
	q, args := t2dQuery(sc, " AND w.id = ?")
	rows, err := db.QueryContext(ctx, q, append(args, id)...)
	if err != nil {
		return T2DWebhook{}, false, err
	}
	ws, err := scanT2D(rows)
	if err != nil || len(ws) == 0 {
		return T2DWebhook{}, false, err
	}
	return ws[0], true, nil
}

// chatType is the telegram_chat_type of a chat id.
func chatType(id int64) string {
	if id < 0 {
		return "group"
	}
	return "private"
}

// CreateT2D adds a webhook for a chat the caller resolved.
func CreateT2D(ctx context.Context, db *sql.DB, chat int64, url string, enabled bool, t FwdTypes) (uint64, error) {
	args := append([]any{chat, chatType(chat), url, enabled}, T2D.args(t)...)
	res, err := db.ExecContext(ctx, "INSERT INTO telegram_discord_webhooks "+
		"(telegram_chat_id, telegram_chat_type, webhook_url, enabled, "+
		T2D.columns("")+") VALUES ("+placeholders(len(args))+")", args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

// UpdateT2D stores a webhook's chat, URL, state and types.
func UpdateT2D(ctx context.Context, db *sql.DB, id uint64, chat int64, url string, enabled bool,
	t FwdTypes) error {
	args := append([]any{chat, chatType(chat), url, enabled}, T2D.args(t)...)
	_, err := db.ExecContext(ctx, "UPDATE telegram_discord_webhooks SET telegram_chat_id = ?, "+
		"telegram_chat_type = ?, webhook_url = ?, enabled = ?, "+
		T2D.assignments()+" WHERE id = ?", append(args, id)...)
	return err
}

// DeleteT2D removes a webhook. What it posted stays recorded under its
// endpoint (discord_endpoints), so edits of those posts still resolve.
func DeleteT2D(ctx context.Context, db *sql.DB, id uint64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegram_discord_webhooks WHERE id = ?", id)
	return err
}

// D2TRoute is a discord_telegram_routes row with its bots' ids.
type D2TRoute struct {
	ID          uint64
	ChannelID   uint64
	ChannelName string // "" until discordd has seen the channel
	Chat        FwdChat
	Enabled     bool
	Types       FwdTypes
	// The Telegram bot that sends, and the Discord bot that reads (0
	// for a route made before the Discord bots were stored, until the
	// deployment's import gives it one).
	TelegramBotID uint64
	DiscordBotID  uint64
	Sent          int64 // messages sent through the route
}

func d2tQuery(sc scope.Scope, where string) (string, []any) {
	cf, ca := ChatFilter(sc, "r.telegram_chat_id")
	q := "SELECT r.id, r.discord_channel_id, COALESCE(c.name, ''), r.telegram_chat_id, " +
		"r.enabled, " + D2T.columns("r.") + ", " +
		"r.telegram_bot_id, COALESCE(r.discord_bot_id, 0), " + chatColumns + ", " +
		"(SELECT COUNT(*) FROM discord_telegram_sent_messages s WHERE s.route_id = r.id) " +
		"FROM discord_telegram_routes r " +
		"LEFT JOIN discord_channels c ON c.id = r.discord_channel_id " +
		chatJoins("r.telegram_chat_id") +
		"WHERE " + cf + where + " ORDER BY r.id DESC"
	return q, ca
}

func scanD2T(rows *sql.Rows) ([]D2TRoute, error) {
	defer rows.Close()
	out := []D2TRoute{}
	for rows.Next() {
		var (
			d    D2TRoute
			chat int64
			c    chatCols
		)
		f := D2T.scanner()
		dest := append([]any{&d.ID, &d.ChannelID, &d.ChannelName, &chat, &d.Enabled}, f.dest()...)
		dest = append(append(dest, &d.TelegramBotID, &d.DiscordBotID), c.dest()...)
		dest = append(dest, &d.Sent)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		d.Types = f.types()
		d.Chat = c.chat(chat)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListD2T is the routes the scope sees, newest first.
func ListD2T(ctx context.Context, db *sql.DB, sc scope.Scope) ([]D2TRoute, error) {
	q, args := d2tQuery(sc, "")
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return scanD2T(rows)
}

// GetD2T is one route the scope sees.
func GetD2T(ctx context.Context, db *sql.DB, sc scope.Scope, id uint64) (D2TRoute, bool, error) {
	q, args := d2tQuery(sc, " AND r.id = ?")
	rows, err := db.QueryContext(ctx, q, append(args, id)...)
	if err != nil {
		return D2TRoute{}, false, err
	}
	rs, err := scanD2T(rows)
	if err != nil || len(rs) == 0 {
		return D2TRoute{}, false, err
	}
	return rs[0], true, nil
}

// ErrDuplicateRoute: the Discord channel already forwards to that
// Telegram chat (uq_dtr_channel_chat).
var ErrDuplicateRoute = errors.New("the channel already forwards to the chat")

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// CreateD2T adds a route from a channel to a chat, read by Discord bot
// discordBot and sent through Telegram bot telegramBot, which the
// caller checked.
func CreateD2T(ctx context.Context, db *sql.DB, channel uint64, chat int64, telegramBot,
	discordBot uint64, enabled bool, t FwdTypes) (uint64, error) {
	args := append([]any{channel, chat, telegramBot, discordBot, enabled}, D2T.args(t)...)
	res, err := db.ExecContext(ctx, "INSERT INTO discord_telegram_routes "+
		"(discord_channel_id, telegram_chat_id, telegram_bot_id, discord_bot_id, enabled, "+
		D2T.columns("")+") VALUES ("+placeholders(len(args))+")", args...)
	if isDuplicate(err) {
		return 0, ErrDuplicateRoute
	}
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

// UpdateD2T stores a route as d holds it: channel, chat, bots, state
// and types. ErrDuplicateRoute when the channel already forwards to
// the chat through another route.
func UpdateD2T(ctx context.Context, db *sql.DB, d D2TRoute) error {
	var discordBot any
	if d.DiscordBotID != 0 {
		discordBot = d.DiscordBotID
	}
	args := append([]any{d.ChannelID, d.Chat.ID, d.TelegramBotID, discordBot, d.Enabled},
		D2T.args(d.Types)...)
	_, err := db.ExecContext(ctx, "UPDATE discord_telegram_routes SET discord_channel_id = ?, "+
		"telegram_chat_id = ?, telegram_bot_id = ?, discord_bot_id = ?, enabled = ?, "+
		D2T.assignments()+" WHERE id = ?", append(args, d.ID)...)
	if isDuplicate(err) {
		return ErrDuplicateRoute
	}
	return err
}

// D2TSent is how many messages a route has sent. Those rows refer to
// the route under the default RESTRICT, so a route that has sent any
// cannot be deleted: they map Discord messages to the Telegram ones,
// for replies and edits.
func D2TSent(ctx context.Context, db *sql.DB, id uint64) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM discord_telegram_sent_messages "+
		"WHERE route_id = ?", id).Scan(&n)
	return n, err
}

// DeleteD2T removes a route that has sent nothing.
func DeleteD2T(ctx context.Context, db *sql.DB, id uint64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM discord_telegram_routes WHERE id = ?", id)
	return err
}

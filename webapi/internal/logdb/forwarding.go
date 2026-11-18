// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/scope"
)

// The forwarding configurations, in both directions, as the Drogon
// web's dao::telegram_discord and dao::discord_telegram keep them
// (web/docs/platform-forwarding.md): Telegram to Discord webhooks
// (telegram_discord_webhooks) and Discord to Telegram routes
// (discord_telegram_routes) with their bots (telegram_bots). They are
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

// FwdKeys are the content types a route forwards, in the forms' order:
// the fwd_<key> columns (web/src/dao/ForwardTypes.hpp). Discord to
// Telegram has the first five; "other" is Telegram to Discord only.
var FwdKeys = []string{"sticker", "text", "photo", "video", "document", "other"}

// T2DTypeCount and D2TTypeCount are how many of FwdKeys each direction
// has.
const (
	T2DTypeCount = 6
	D2TTypeCount = 5
)

// FwdTypes are the flags of FwdKeys, in that order.
type FwdTypes [6]bool

// AllFwdTypes forwards everything, the columns' default.
func AllFwdTypes() FwdTypes {
	return FwdTypes{true, true, true, true, true, true}
}

// Keys are the enabled types' keys among the first n.
func (t FwdTypes) Keys(n int) []string {
	out := []string{}
	for i := 0; i < n; i++ {
		if t[i] {
			out = append(out, FwdKeys[i])
		}
	}
	return out
}

// ParseFwdTypes reads a list of keys among the first n of FwdKeys; an
// unknown key fails. Repeats count once.
func ParseFwdTypes(keys []string, n int) (FwdTypes, bool) {
	var t FwdTypes
	for _, k := range keys {
		i := -1
		for j := 0; j < n; j++ {
			if FwdKeys[j] == k {
				i = j
			}
		}
		if i < 0 {
			return t, false
		}
		t[i] = true
	}
	return t, true
}

func (t FwdTypes) args(n int) []any {
	out := make([]any, n)
	for i := 0; i < n; i++ {
		out[i] = t[i]
	}
	return out
}

func fwdColumns(alias string, n int) string {
	cols := make([]string, n)
	for i := 0; i < n; i++ {
		cols[i] = alias + "fwd_" + FwdKeys[i]
	}
	return strings.Join(cols, ", ")
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
}

// chatJoins and chatColumns resolve a configuration's chat title live,
// as the Drogon lists do, instead of a stored copy that goes stale.
func chatJoins(col string) string {
	return "LEFT JOIN telegram_groups g ON " + col + " < 0 AND g.id = " + col + " " +
		"LEFT JOIN telegram_users u ON " + col + " > 0 AND u.id = " + col + " "
}

const chatColumns = "g.title, g.type, u.first_name, u.last_name"

func chatOf(id int64, title, gtype, first, last sql.NullString) FwdChat {
	if id < 0 {
		return FwdChat{ID: id, Kind: "group", Title: GroupTitle(title.String), Type: gtype.String}
	}
	return FwdChat{ID: id, Kind: "user", Title: DisplayName(first.String, last.String)}
}

// ResolveChat is a chat the logger has seen and the scope sees, for a
// new configuration: a group from telegram_groups, a private chat from
// telegram_users. A chat never seen, or hidden, is not found.
func ResolveChat(ctx context.Context, db *sql.DB, sc scope.Scope, id int64) (FwdChat, bool, error) {
	if id == 0 || !sc.SeesChat(id) {
		return FwdChat{}, false, nil
	}
	var title, gtype, first, last sql.NullString
	var err error
	if id < 0 {
		gf, ga := sc.GroupFilter("g.id")
		err = db.QueryRowContext(ctx, "SELECT g.title, g.type FROM telegram_groups g "+
			"WHERE g.id = ? AND "+gf, append([]any{id}, ga...)...).Scan(&title, &gtype)
	} else {
		err = db.QueryRowContext(ctx, "SELECT first_name, last_name FROM telegram_users "+
			"WHERE id = ?", id).Scan(&first, &last)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return FwdChat{}, false, nil
	}
	if err != nil {
		return FwdChat{}, false, err
	}
	return chatOf(id, title, gtype, first, last), true, nil
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
		fwdColumns("w.", T2DTypeCount) + ", " + chatColumns + ", " +
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
			w                         T2DWebhook
			chat                      int64
			title, gtype, first, last sql.NullString
		)
		dest := []any{&w.ID, &chat, &w.URL, &w.Enabled}
		for i := 0; i < T2DTypeCount; i++ {
			dest = append(dest, &w.Types[i])
		}
		dest = append(dest, &title, &gtype, &first, &last, &w.Sent, &w.LastSent)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		w.Chat = chatOf(chat, title, gtype, first, last)
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
	args := append([]any{chat, chatType(chat), url, enabled}, t.args(T2DTypeCount)...)
	res, err := db.ExecContext(ctx, "INSERT INTO telegram_discord_webhooks "+
		"(telegram_chat_id, telegram_chat_type, webhook_url, enabled, "+
		fwdColumns("", T2DTypeCount)+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

// UpdateT2D stores a webhook's URL, state and types.
func UpdateT2D(ctx context.Context, db *sql.DB, id uint64, url string, enabled bool, t FwdTypes) error {
	args := append([]any{url, enabled}, t.args(T2DTypeCount)...)
	_, err := db.ExecContext(ctx, "UPDATE telegram_discord_webhooks SET webhook_url = ?, "+
		"enabled = ?, fwd_sticker = ?, fwd_text = ?, fwd_photo = ?, fwd_video = ?, "+
		"fwd_document = ?, fwd_other = ? WHERE id = ?", append(args, id)...)
	return err
}

// DeleteT2D removes a webhook. What it posted stays recorded under its
// endpoint (discord_endpoints), so edits of those posts still resolve.
func DeleteT2D(ctx context.Context, db *sql.DB, id uint64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegram_discord_webhooks WHERE id = ?", id)
	return err
}

// D2TRoute is a discord_telegram_routes row with its bot.
type D2TRoute struct {
	ID          uint64
	ChannelID   uint64
	ChannelName string // "" until discordd has seen the channel
	Chat        FwdChat
	Enabled     bool
	Types       FwdTypes
	// The bot, never its token: its row id, its Telegram user id (0
	// until discordd has logged it in) and username.
	BotID       uint64
	BotUserID   int64
	BotUsername string
	BotEnabled  bool
	Sent        int64 // messages sent through the route
}

func d2tQuery(sc scope.Scope, where string) (string, []any) {
	cf, ca := ChatFilter(sc, "r.telegram_chat_id")
	q := "SELECT r.id, r.discord_channel_id, COALESCE(c.name, ''), r.telegram_chat_id, " +
		"r.enabled, " + fwdColumns("r.", D2TTypeCount) + ", " +
		"b.id, b.bot_user_id, b.username, b.enabled, " + chatColumns + ", " +
		"(SELECT COUNT(*) FROM discord_telegram_sent_messages s WHERE s.route_id = r.id) " +
		"FROM discord_telegram_routes r JOIN telegram_bots b ON b.id = r.telegram_bot_id " +
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
			d                         D2TRoute
			chat                      int64
			title, gtype, first, last sql.NullString
		)
		dest := []any{&d.ID, &d.ChannelID, &d.ChannelName, &chat, &d.Enabled}
		for i := 0; i < D2TTypeCount; i++ {
			dest = append(dest, &d.Types[i])
		}
		dest = append(dest, &d.BotID, &d.BotUserID, &d.BotUsername, &d.BotEnabled,
			&title, &gtype, &first, &last, &d.Sent)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		d.Chat = chatOf(chat, title, gtype, first, last)
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

// Bot is the bot row a token was interned as; Created when the row is
// new, for the caller to record its owner.
type Bot struct {
	ID      uint64
	Created bool
}

// internBot is the bot row of a token: the existing one, enabled
// again, or a new one. The token is unique (and compared as bytes,
// migration 000029), so several routes share one credential.
func internBot(ctx context.Context, tx *sql.Tx, token string) (Bot, error) {
	res, err := tx.ExecContext(ctx, "INSERT INTO telegram_bots (token) VALUES (?) "+
		"ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), enabled = 1", token)
	if err != nil {
		return Bot{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Bot{}, err
	}
	// 1 for an inserted row; 2 for an existing one enabled again, 0
	// for one left as it was.
	n, err := res.RowsAffected()
	return Bot{ID: uint64(id), Created: n == 1}, err
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// CreateD2T adds a route through the bot of token, and returns the
// route's id and the bot.
func CreateD2T(ctx context.Context, db *sql.DB, channel uint64, chat int64, token string,
	enabled bool, t FwdTypes) (uint64, Bot, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, Bot{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM discord_telegram_routes "+
		"WHERE discord_channel_id = ? AND telegram_chat_id = ?", channel, chat).Scan(&n); err != nil {
		return 0, Bot{}, err
	}
	if n > 0 {
		return 0, Bot{}, ErrDuplicateRoute
	}
	bot, err := internBot(ctx, tx, token)
	if err != nil {
		return 0, Bot{}, err
	}
	args := append([]any{channel, chat, bot.ID, enabled}, t.args(D2TTypeCount)...)
	res, err := tx.ExecContext(ctx, "INSERT INTO discord_telegram_routes "+
		"(discord_channel_id, telegram_chat_id, telegram_bot_id, enabled, "+
		fwdColumns("", D2TTypeCount)+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", args...)
	if isDuplicate(err) {
		return 0, Bot{}, ErrDuplicateRoute
	}
	if err != nil {
		return 0, Bot{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, Bot{}, err
	}
	return uint64(id), bot, tx.Commit()
}

// UpdateD2T stores a route's state and types and, with a token, the
// bot it sends through, which it returns (the zero Bot without a
// token). The bot it used before stays, for the other routes that may
// share it.
func UpdateD2T(ctx context.Context, db *sql.DB, id uint64, enabled bool, t FwdTypes,
	token *string) (Bot, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Bot{}, err
	}
	defer tx.Rollback()
	args := append([]any{enabled}, t.args(D2TTypeCount)...)
	set := "enabled = ?, fwd_sticker = ?, fwd_text = ?, fwd_photo = ?, fwd_video = ?, " +
		"fwd_document = ?"
	var bot Bot
	if token != nil {
		if bot, err = internBot(ctx, tx, *token); err != nil {
			return Bot{}, err
		}
		set += ", telegram_bot_id = ?"
		args = append(args, bot.ID)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE discord_telegram_routes SET "+set+
		" WHERE id = ?", append(args, id)...); err != nil {
		return Bot{}, err
	}
	return bot, tx.Commit()
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

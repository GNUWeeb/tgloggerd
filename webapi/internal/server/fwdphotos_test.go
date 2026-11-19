// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/testdb"
)

// The photos of the forwarding pages' chats and the pinned groups on
// the "fwd_photos" fixture: shown when stored, null when the file is
// metadata only or there is none.

const (
	kernelPhoto    = uint64(860003) // Linux Kernel Indonesia, stored
	gnuWeebPhoto   = uint64(880008) // the sample's, stored
	dewiPhoto      = uint64(880007) // the sample's, stored
	fwdPhotosFixtr = "fwd_photos"
)

func photoOf(id uint64) string {
	return "/files/" + auth.NewFileTokens(testKey).Encrypt(id)
}

// routesOf is a forwarding page's routes as role sees them, by id.
func routesOf(t *testing.T, e *adminEnv, role, path string) map[string]map[string]any {
	t.Helper()
	st, b := e.page(role, path)
	if st != 200 {
		t.Fatalf("%s %s: %d %v", role, path, st, b)
	}
	out := map[string]map[string]any{}
	for _, r := range b["props"].(map[string]any)["routes"].([]any) {
		m := r.(map[string]any)
		out[fmt.Sprint(m["id"])] = m
	}
	return out
}

func chatPhoto(r map[string]any) any {
	return r["chat"].(map[string]any)["photo"]
}

func TestForwardingChatPhotos(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)

	t2d := routesOf(t, e, "admin", "/platform-fwd/telegram-discord")
	for id, want := range map[string]any{
		"1": photoOf(gnuWeebPhoto), // a group's stored photo
		"2": photoOf(dewiPhoto),    // a private chat: the user's profile photo
		"3": nil,                   // Rust's photo is metadata only
	} {
		if got := chatPhoto(t2d[id]); got != want {
			t.Errorf("webhook %s photo %v, want %v", id, got, want)
		}
	}

	d2t := routesOf(t, e, "superadmin", "/platform-fwd/discord-telegram")
	if got := chatPhoto(d2t["1"]); got != photoOf(gnuWeebPhoto) {
		t.Errorf("route 1 photo %v", got)
	}
	if got := chatPhoto(d2t["2"]); got != photoOf(kernelPhoto) {
		t.Errorf("route 2 photo %v", got)
	}
	// A member with forward sees its own webhook's chat photo, and no
	// other configuration (so no other chat's photo).
	grantMember(t, e, memberGroups, "forward")
	create := map[string]any{"chat_id": kernelChat, "chat_kind": "group", "webhook_url": newHook,
		"types": []string{"text"}, "enabled": true}
	id := fmt.Sprint(e.want(201, "member", "POST", t2dBase, create)["id"])
	mine := routesOf(t, e, "member", "/platform-fwd/telegram-discord")
	if len(mine) != 1 || chatPhoto(mine[id]) != photoOf(kernelPhoto) {
		t.Errorf("member's webhooks %v", mine)
	}
	_, p := e.page("member", "/platform-fwd/telegram-discord")
	if s := js(p); strings.Contains(s, photoOf(gnuWeebPhoto)) || strings.Contains(s, photoOf(dewiPhoto)) {
		t.Errorf("member sees a hidden chat's photo: %s", s)
	}
}

func TestPinnedGroupPhotos(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	pin := func(uid uint64, groups ...int64) {
		t.Helper()
		for _, g := range groups {
			if _, err := e.app.Exec("INSERT INTO web_pinned_groups (web_user_id, group_id) VALUES (?, ?)",
				uid, g); err != nil {
				t.Fatal(err)
			}
		}
	}
	pins := func(role string) []any {
		t.Helper()
		_, p := e.page(role, "/platform-fwd")
		v, _ := p["viewer"].(map[string]any)["pinned"].([]any)
		return v
	}
	pin(e.uids["admin"], gnuWeeb, rustChat, kernelChat)
	got := pins("admin")
	if len(got) != 3 {
		t.Fatalf("admin's pins %v", got)
	}
	for i, want := range []any{photoOf(gnuWeebPhoto), nil, photoOf(kernelPhoto)} {
		if v, ok := got[i].(map[string]any)["photo"]; !ok || v != want {
			t.Errorf("pin %d photo %v (present %t), want %v", i, v, ok, want)
		}
	}

	// A member sees only the pins of groups it may see, with their
	// photos.
	grantMember(t, e, memberGroups, "forward")
	pin(memberUID, gnuWeeb, kernelChat)
	got = pins("member")
	if len(got) != 1 || got[0].(map[string]any)["id"] != float64(kernelChat) ||
		got[0].(map[string]any)["photo"] != photoOf(kernelPhoto) {
		t.Errorf("member's pins %v", got)
	}
}

// countingConnector counts the statements that name a table, on every
// connection of its pool.
type countingConnector struct {
	driver.Connector
	table string
	n     *atomic.Int64
}

func (c countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn, c: c}, nil
}

type countingConn struct {
	driver.Conn
	c countingConnector
}

func (c *countingConn) note(q string) {
	if strings.Contains(q, c.c.table) {
		c.c.n.Add(1)
	}
}

func (c *countingConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
	if !errors.Is(err, driver.ErrSkip) {
		c.note(q)
	}
	return rows, err
}

func (c *countingConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}

func (c *countingConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	c.note(q)
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, q)
}

func (c *countingConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
}

func (c *countingConn) Ping(ctx context.Context) error {
	return c.Conn.(driver.Pinger).Ping(ctx)
}

func (c *countingConn) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}

func (c *countingConn) IsValid() bool {
	return c.Conn.(driver.Validator).IsValid()
}

func (c *countingConn) CheckNamedValue(v *driver.NamedValue) error {
	return c.Conn.(driver.NamedValueChecker).CheckNamedValue(v)
}

// countFiles points the env's read-only pool at its own database
// through a counting pool, and returns the count of statements that
// read telegram_files.
func countFiles(t *testing.T, e *adminEnv) *atomic.Int64 {
	t.Helper()
	var name string
	if err := e.ro.QueryRow("SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(os.Getenv(testdb.EnvDSN))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = name
	mc, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := &atomic.Int64{}
	db := sql.OpenDB(countingConnector{Connector: mc, table: "telegram_files", n: n})
	t.Cleanup(func() { db.Close() })
	e.with(func(d *Deps) { d.RO = db })
	return n
}

// The photos come with the rows that name them: a page costs as many
// statements on telegram_files with many items as with few.
func TestPhotosAreBatched(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	n := countFiles(t, e)
	cost := func(role, path string) int64 {
		t.Helper()
		before := n.Load()
		if st, b := e.page(role, path); st != 200 {
			t.Fatalf("%s: %d %v", path, st, b)
		}
		return n.Load() - before
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := e.ro.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}

	pin := func(g int64) {
		t.Helper()
		if _, err := e.app.Exec("INSERT INTO web_pinned_groups (web_user_id, group_id) VALUES (?, ?)",
			e.uids["admin"], g); err != nil {
			t.Fatal(err)
		}
	}
	// One pin to begin with: without any, the pins need no query.
	pin(rustChat)

	t2d, d2t := cost("admin", "/platform-fwd/telegram-discord"), cost("admin", "/platform-fwd/discord-telegram")
	pins := cost("admin", "/platform-fwd")
	if t2d == 0 || d2t == 0 || pins == 0 {
		t.Fatalf("the photos are not read with the rows: %d %d %d", t2d, d2t, pins)
	}

	for i := range 6 {
		id := 100 + i
		chat := []int64{gnuWeeb, 9299000001, kernelChat}[i%3]
		kind := "group"
		if chat > 0 {
			kind = "private"
		}
		exec("INSERT INTO telegram_discord_webhooks (id, telegram_chat_id, telegram_chat_type, "+
			"webhook_url, enabled) VALUES (?, ?, ?, ?, 1)", id, chat, kind,
			fmt.Sprintf("https://discord.com/api/webhooks/11000000000000001%02d/batchSecret", id))
		exec("INSERT INTO discord_telegram_routes (id, discord_channel_id, telegram_chat_id, "+
			"telegram_bot_id, enabled) VALUES (?, ?, ?, 1, 1)", id, 710000000000000100+id, chat)
	}
	for _, g := range []int64{gnuWeeb, kernelChat, -1001000000004, -1001000000005} {
		pin(g)
	}

	for _, c := range []struct {
		path string
		was  int64
	}{
		{"/platform-fwd/telegram-discord", t2d},
		{"/platform-fwd/discord-telegram", d2t},
		{"/platform-fwd", pins},
	} {
		if now := cost("admin", c.path); now != c.was {
			t.Errorf("%s: %d statements on telegram_files, %d with fewer items", c.path, now, c.was)
		}
	}
	// And the items are all there, with their photos.
	if r := routesOf(t, e, "admin", "/platform-fwd/telegram-discord"); len(r) != 9 ||
		chatPhoto(r["105"]) != photoOf(kernelPhoto) {
		t.Errorf("webhooks %d %v", len(r), r["105"])
	}
	_, p := e.page("admin", "/platform-fwd")
	if v := p["viewer"].(map[string]any)["pinned"].([]any); len(v) != 5 {
		t.Errorf("pins %v", v)
	}
}

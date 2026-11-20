// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/testdb"
)

// The pictures videos show before they play, on the "video_stills"
// fixture (its header lists each message): in the chat's media as
// poster and thumb, in the files list as thumb, with the minithumb
// beside them; a stored cover before a stored thumbnail, never an
// unstored one, never the video's own URL; read for a page at once.

const stillLab = -1001000000050

var stillTokens = auth.NewFileTokens(testKey)

func stillURL(id uint64) string { return "/files/" + stillTokens.Encrypt(id) }

// stillMedia is Still Lab's media by row id - 5000, as admin sees it.
func stillMedia(t *testing.T, e *adminEnv) map[int]map[string]any {
	t.Helper()
	b := e.want(200, "admin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=50", stillLab), nil)
	out := map[int]map[string]any{}
	for _, m := range b["messages"].([]any) {
		m := m.(map[string]any)
		if md, ok := m["media"].(map[string]any); ok {
			out[int(m["id"].(float64))-5000] = md
		}
	}
	return out
}

// stillRows are the files page's rows of the fixture's videos, by id.
func stillRows(t *testing.T, e *adminEnv, role string) map[uint64]map[string]any {
	t.Helper()
	q := url.Values{"f": {`[{"field":"id","op":">","value":"890000"},` +
		`{"field":"id","op":"<","value":"890100"}]`}, "per": {"100"}}
	st, b := e.page(role, "/files?"+q.Encode())
	if st != 200 {
		t.Fatalf("files page: %d %v", st, b)
	}
	out := map[uint64]map[string]any{}
	for _, r := range b["props"].(map[string]any)["rows"].([]any) {
		r := r.(map[string]any)
		out[uint64(r["id"].(float64))] = r
	}
	return out
}

func strOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("<%v>", v)
}

func TestVideoStillsInChat(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "video_stills")
	md := stillMedia(t, e)
	if len(md) != 9 {
		t.Fatalf("%d media: %v", len(md), md)
	}
	for _, c := range []struct {
		n      int
		poster uint64 // 0: none
		render string
	}{
		{1, 890101, "video"},
		{2, 890102, "animation"},
		{3, 890103, "video"},     // a video note
		{4, 890104, "animation"}, // a .webm sticker
		{5, 890105, "video"},     // the cover, not the thumbnail
		{6, 0, "video"},
		{7, 0, "video"}, // its thumbnail is not stored
		{8, 0, "video"},
		{9, 890108, "video"},
	} {
		m := md[c.n]
		if m["render"] != c.render {
			t.Errorf("%d: render %v, want %s", c.n, m["render"], c.render)
		}
		if strOf(m["url"]) != stillURL(890000+uint64(min(c.n, 8))) {
			t.Errorf("%d: url %v", c.n, m["url"])
		}
		mini, hasMini := m["minithumb"]
		if c.n != 4 && mini != fmt.Sprintf("data:image/jpeg;base64,MINI%dabc", c.n) {
			t.Errorf("%d: minithumb %v", c.n, mini)
		}
		if c.poster == 0 {
			if _, ok := m["poster"]; ok {
				t.Errorf("%d: poster %v, want none", c.n, m["poster"])
			}
			// The blurred preview stays the thumb.
			if !hasMini || m["thumb"] != mini {
				t.Errorf("%d: thumb %v, want the minithumb", c.n, m["thumb"])
			}
			continue
		}
		if p := stillURL(c.poster); strOf(m["poster"]) != p || strOf(m["thumb"]) != p {
			t.Errorf("%d: poster %v thumb %v, want %s", c.n, m["poster"], m["thumb"], p)
		}
	}

	// The message page says the same.
	for _, c := range []struct {
		row    int
		poster string
	}{{5001, stillURL(890101)}, {5005, stillURL(890105)}, {5007, ""}} {
		st, b := e.page("admin", fmt.Sprintf("/messages/group/%d", c.row))
		if st != 200 {
			t.Fatalf("message %d: %d %v", c.row, st, b)
		}
		m := b["props"].(map[string]any)["message"].(map[string]any)["media"].(map[string]any)
		if p, _ := m["poster"].(string); p != c.poster {
			t.Errorf("message %d: poster %v, want %q", c.row, m["poster"], c.poster)
		}
		if c.poster != "" && m["thumb"] != c.poster {
			t.Errorf("message %d: thumb %v", c.row, m["thumb"])
		}
	}
}

func TestVideoStillsInFilesList(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "video_stills")
	rows := stillRows(t, e, "admin")
	if len(rows) != 8 {
		t.Fatalf("%d rows", len(rows))
	}
	for _, c := range []struct {
		id    uint64
		thumb uint64 // 0: null
		mini  string // "": none
	}{
		{890001, 890101, "MINI1"},
		{890002, 890102, "MINI2"},
		{890003, 890103, "MINI3"},
		{890004, 890104, ""},
		{890005, 890105, "MINI5"},
		{890006, 0, "MINI6"},
		{890007, 0, "MINI7"},
		{890008, 890108, "MINI9"}, // from the message sent later
	} {
		r := rows[c.id]
		if c.thumb == 0 {
			if r["thumb"] != nil {
				t.Errorf("%d: thumb %v, want null", c.id, r["thumb"])
			}
		} else if strOf(r["thumb"]) != stillURL(c.thumb) {
			t.Errorf("%d: thumb %v, want %s", c.id, r["thumb"], stillURL(c.thumb))
		}
		if r["thumb"] == r["url"] {
			t.Errorf("%d: thumb is the video's own URL", c.id)
		}
		mini, ok := r["minithumb"]
		if c.mini == "" && ok {
			t.Errorf("%d: minithumb %v, want none", c.id, mini)
		}
		if c.mini != "" && mini != "data:image/jpeg;base64,"+c.mini+"abc" {
			t.Errorf("%d: minithumb %v", c.id, mini)
		}
	}

	// A thumbnail's own row is a picture: it is its own thumb, and
	// a photo to the pages, though its type filters as thumbnail.
	q := url.Values{"f": {`[{"field":"id","op":"=","value":"890101"},` +
		`{"field":"file_type","op":"=","value":"thumbnail"}]`}}
	_, b := e.page("admin", "/files?"+q.Encode())
	rs := b["props"].(map[string]any)["rows"].([]any)
	if len(rs) != 1 || rs[0].(map[string]any)["thumb"] != stillURL(890101) ||
		rs[0].(map[string]any)["file_type"] != "photo" {
		t.Errorf("thumbnail row %v", rs)
	}

	// A member granted Open Stills only sees the stills of its
	// messages: none of Still Lab's pictures or minithumbs.
	if _, err := e.app.Exec("INSERT INTO web_member_grant_groups (web_user_id, federation_id, group_id) "+
		"VALUES (?, 0, ?)", e.uids["member"], int64(-1001000000051)); err != nil {
		t.Fatal(err)
	}
	mem := stillRows(t, e, "member")
	if len(mem) != 2 {
		t.Fatalf("member sees %d rows: %v", len(mem), mem)
	}
	if r := mem[890001]; r["thumb"] != nil || r["minithumb"] != nil {
		t.Errorf("member: 890001 has %v %v, want neither", r["thumb"], r["minithumb"])
	}
	if r := mem[890002]; strOf(r["thumb"]) != stillURL(890102) || r["minithumb"] != nil {
		t.Errorf("member: 890002 has %v %v", r["thumb"], r["minithumb"])
	}
}

// countStatements makes e's logger pool count its statements that name
// table.
func countStatements(t *testing.T, e *adminEnv, table string) *atomic.Int64 {
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
	db := sql.OpenDB(countingConnector{Connector: mc, table: table, n: n})
	t.Cleanup(func() { db.Close() })
	e.with(func(d *Deps) { d.RO = db })
	return n
}

// The stills come with the rows: a chat window and a files page cost
// as many statements on telegram_message_files with many videos as
// with few.
func TestVideoStillsAreBatched(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "video_stills")
	n := countStatements(t, e, "telegram_message_files")
	cost := func(f func()) int64 {
		before := n.Load()
		f()
		return n.Load() - before
	}
	chat := func() { stillMedia(t, e) }
	files := func() { stillRows(t, e, "admin") }
	c0, f0 := cost(chat), cost(files)
	if c0 == 0 || f0 == 0 {
		t.Fatalf("the stills are not read with the rows: %d %d", c0, f0)
	}

	for i := range 10 {
		id, row := 890010+i, 5010+i
		for _, q := range []string{
			fmt.Sprintf("INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, "+
				"file_size, on_disk, sha256, file_ext, orig_file_name) VALUES (%d, 'still-%[1]d', "+
				"'still-u-%[1]d', 'video', 1000, 1, UNHEX(SHA2('still %[1]d', 256)), 'mp4', '')", id),
			fmt.Sprintf("INSERT INTO telegram_group_messages (id, chat_id, message_id, "+
				"sender_user_id, date, content_type, file_id) VALUES (%d, %d, %d, 9299000001, "+
				"1794931200, 'video', %d)", row, stillLab, int64(700100+i)<<20, id),
			fmt.Sprintf("INSERT INTO telegram_message_files (group_message_id, role, position, "+
				"file_id) VALUES (%d, 'thumbnail', 0, 890101)", row),
		} {
			if _, err := e.ro.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	if md := stillMedia(t, e); len(md) != 19 || strOf(md[19]["poster"]) != stillURL(890101) {
		t.Fatalf("chat %d media, %v", len(md), md[19])
	}
	if rows := stillRows(t, e, "admin"); len(rows) != 18 || strOf(rows[890019]["thumb"]) != stillURL(890101) {
		t.Fatalf("files %d rows, %v", len(rows), rows[890019])
	}
	if c, f := cost(chat), cost(files); c != c0 || f != f0 {
		t.Errorf("chat %d statements (was %d), files %d (was %d)", c, c0, f, f0)
	}
}

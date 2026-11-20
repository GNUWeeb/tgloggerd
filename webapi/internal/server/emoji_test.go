// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/daemon"
)

// Custom emoji and message effects (CustomEmoji, MessageEffect) over
// the "custom_emoji" fixture, whose header lists what Emoji Lab holds:
// in text entities, reactions, forum topic icons (the chat's list, the
// message page's topic and the topic lines) and effects, from the
// tables; the ids the tables lack asked of a fake daemon once per page,
// a page that does not wait for a slow daemon beyond its budget, and
// no asks while the daemon is away or Telegram asks to wait.

const emojiLab = int64(-1001000000080)

// The fixture's custom emoji (ce) and effects (fx), by their last digit.
func ce(n int) string { return fmt.Sprintf("500000000000000000%d", n) }
func fx(n int) string { return fmt.Sprintf("510000000000000000%d", n) }

// emojiDaemon is a fake daemon for POST /v1/custom-emoji and
// /v1/message-effects. It knows custom emoji 3 and effect 3 and, like
// the daemon, stores them in the logger's database before it answers;
// Telegram does not know any other id.
type emojiDaemon struct {
	db *sql.DB
	mu sync.Mutex
	// asked are the asks as "<path> <ids>".
	asked []string
	// mode is "ok", "flood" (a flood wait of 120 s), "down" (a 500)
	// or "block" (held until release is closed, then ok).
	mode    string
	release chan struct{}
}

func (d *emojiDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d.mu.Lock()
	d.asked = append(d.asked, r.URL.Path+" "+strings.Join(body.IDs, ","))
	mode, release := d.mode, d.release
	d.mu.Unlock()
	effects := r.URL.Path == "/v1/message-effects"
	if r.URL.Path != "/v1/custom-emoji" && !effects {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch mode {
	case "down":
		w.WriteHeader(http.StatusInternalServerError)
		return
	case "block":
		// Bounded, so a failed test cannot hang on it.
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
	}
	list := "emoji"
	if effects {
		list = "effects"
	}
	if mode == "flood" {
		fmt.Fprintf(w, `{"outcome":"flood_wait","message":"","asked":%d,"retry_after":120,`+
			`%q:[],"missing":[]}`, len(body.IDs), list)
		return
	}
	var rows, missing []string
	for _, id := range body.IDs {
		switch {
		case !effects && id == ce(3):
			d.exec("INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, "+
				"file_size, on_disk, sha256, file_ext) VALUES (876006, 'emo-876006', "+
				"'emo-u-876006', 'sticker', 900, 1, UNHEX(SHA2('emo 876006', 256)), 'webm')",
				"INSERT INTO telegram_custom_emoji (custom_emoji_id, emoji, format, "+
					"needs_repainting, file_id) VALUES ("+id+", 'c', 'webm', 0, 876006)")
			rows = append(rows, `{"custom_emoji_id":"`+id+`","emoji":"c","set_id":null,`+
				`"format":"webm","needs_repainting":false,"file_id":876006}`)
		case effects && id == fx(3):
			d.exec("INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, "+
				"file_size, on_disk, sha256, file_ext) VALUES (876007, 'emo-876007', "+
				"'emo-u-876007', 'sticker', 800, 1, UNHEX(SHA2('emo 876007', 256)), 'webm')",
				"INSERT INTO telegram_message_effects (effect_id, emoji, is_premium, "+
					"effect_type, sticker_file_id, sticker_format) VALUES ("+id+", 'p', 1, "+
					"'premium_sticker', 876007, 'webm')")
			rows = append(rows, `{"effect_id":"`+id+`","emoji":"p","is_premium":true,`+
				`"type":"premium_sticker","static_icon_file_id":null,`+
				`"select_animation_file_id":null,"effect_animation_file_id":null,`+
				`"sticker_file_id":876007,"sticker_format":"webm"}`)
		default:
			missing = append(missing, `"`+id+`"`)
		}
	}
	fmt.Fprintf(w, `{"outcome":"ok","message":"","asked":%d,%q:[%s],"missing":[%s]}`,
		len(body.IDs), list, strings.Join(rows, ","), strings.Join(missing, ","))
}

func (d *emojiDaemon) exec(qs ...string) {
	for _, q := range qs {
		if _, err := d.db.Exec(q); err != nil {
			panic(err)
		}
	}
}

func (d *emojiDaemon) set(mode string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mode = mode
}

func (d *emojiDaemon) calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.asked...)
}

type emojiEnv struct {
	*adminEnv
	fake *emojiDaemon // nil without a daemon
	res  *emojiResolver
	mu   sync.Mutex
	now  time.Time
}

// newEmojiEnv is Emoji Lab with the fake daemon (withDaemon) or with
// the daemon API off, and a resolver whose page budget is budget.
func newEmojiEnv(t *testing.T, withDaemon bool, budget time.Duration) *emojiEnv {
	t.Helper()
	e := &emojiEnv{adminEnv: newAdminEnv(t, "custom_emoji"), now: chatNow}
	var dc *daemon.Client
	if withDaemon {
		e.fake = &emojiDaemon{db: e.ro, mode: "ok", release: make(chan struct{})}
		srv := httptest.NewServer(e.fake)
		t.Cleanup(srv.Close)
		var err error
		if dc, err = daemon.New(srv.URL, testKey, 5*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	clock := func() time.Time {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.now
	}
	e.res = newEmojiResolver(slog.New(slog.NewTextHandler(io.Discard, nil)), dc, clock)
	e.res.budget = budget
	// No ask outlives the test's daemon; registered after its Close,
	// so it runs first.
	t.Cleanup(e.res.wait)
	e.deps.Daemon = dc
	e.deps.emoji = e.res
	// The fake records every ask; the chat windows' rechecks are not
	// these tests'.
	e.deps.Recheck = RecheckConfig{Off: true}
	e.h = New(e.deps)
	return e
}

func (e *emojiEnv) advance(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = e.now.Add(d)
}

// window is Emoji Lab's chat window as the superadmin sees it, by row.
func (e *emojiEnv) window() map[int]map[string]any {
	e.t.Helper()
	b := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=50", emojiLab), nil)
	out := map[int]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		out[int(num(m["id"]))] = m
	}
	if len(out) != 6 {
		e.t.Fatalf("%d messages", len(out))
	}
	return out
}

// file is the URL of a fixture file.
func (e *emojiEnv) file(id uint64) string {
	return "/files/" + e.deps.Tokens.Encrypt(id)
}

// sticker is a CustomEmoji's or a MessageEffect's JSON, with %s for the
// URL of file (none for 0).
func (e *emojiEnv) sticker(emoji, format string, file uint64, repaint *bool) string {
	f, u := "null", "null"
	if format != "" {
		f, u = `"`+format+`"`, `"`+e.file(file)+`"`
	}
	r := ""
	if repaint != nil {
		r = fmt.Sprintf(`,"needs_repainting":%v`, *repaint)
	}
	return fmt.Sprintf(`{"emoji":%q,"format":%s%s,"url":%s}`, emoji, f, r, u)
}

// emojiOf is the custom emoji fields of a list of entities or
// reactions: "<id>=<custom_emoji>" for those with one, "-" for others,
// which must not have the key.
func emojiOf(t *testing.T, v any) []string {
	t.Helper()
	var out []string
	for _, x := range asList(v) {
		x := x.(map[string]any)
		id, has := x["custom_emoji_id"]
		c, hasC := x["custom_emoji"]
		switch {
		case has != hasC:
			t.Errorf("custom_emoji_id %v, custom_emoji %v %v", id, c, hasC)
		case !has:
			out = append(out, "-")
		default:
			out = append(out, fmt.Sprintf("%v=%s", id, js(c)))
		}
	}
	return out
}

var (
	fls = false
	tru = true
)

// The ids the tables have, with the daemon API off: nothing asked, the
// rest null, a sticker not stored without a URL (nothing could fetch
// it), and one of unknown format without one either.
func TestCustomEmojiFromTables(t *testing.T) {
	t.Parallel()
	e := newEmojiEnv(t, false, time.Second)
	e1 := e.sticker("\U0001F642", "webp", 876001, &fls)
	e2 := e.sticker("\U0001F600", "", 0, &tru)
	e5 := e.sticker("\U0001F916", "", 0, &fls)
	w := e.window()

	sameList(t, "entities", emojiOf(t, w[8001]["entities"]), []string{
		ce(1) + "=" + e1, ce(2) + "=" + e2, ce(3) + "=null", ce(4) + "=null",
		ce(5) + "=" + e5, "-"})
	sameList(t, "reactions", emojiOf(t, w[8002]["reactions"]), []string{
		ce(1) + "=" + e1, ce(4) + "=null", "-"})

	for row, want := range map[int]string{
		8001: `null null`,
		8002: `"` + fx(1) + `" ` + e.sticker("\U0001F525", "webp", 876004, nil),
		8003: `"` + fx(2) + `" ` + e.sticker("\U0001F389", "tgs", 876005, nil),
		8004: `"` + fx(3) + `" null`,
	} {
		if got := js(w[row]["effect_id"]) + " " + js(w[row]["effect"]); got != want {
			t.Errorf("row %d: effect %s, want %s", row, got, want)
		}
	}

	// The topic lines: created with custom emoji 1, then its icon
	// removed, which the topic as last seen (emoji 5 now) does not
	// overrule.
	for row, want := range map[int]string{
		8005: `"` + ce(1) + `" ` + e1,
		8006: `null null`,
	} {
		tp := w[row]["service"].(map[string]any)["detail"].(map[string]any)["topic"].(map[string]any)
		if got := js(tp["icon_custom_emoji_id"]) + " " + js(tp["icon_custom_emoji"]); got != want {
			t.Errorf("row %d: topic icon %s, want %s", row, got, want)
		}
	}

	// The chat page's topics, and the message page's.
	st, p := e.page("superadmin", fmt.Sprintf("/groups/%d/chat", emojiLab))
	if st != 200 {
		t.Fatalf("chat page %d", st)
	}
	var topics []string
	for _, x := range asList(p["props"].(map[string]any)["topics"]) {
		x := x.(map[string]any)
		topics = append(topics, fmt.Sprintf("%v %s %s", x["id"], js(x["icon_custom_emoji_id"]),
			js(x["icon_custom_emoji"])))
	}
	sameList(t, "topics", topics, []string{`21 "` + ce(5) + `" ` + e5, `22 null null`})
	m := pageMessage(t, e.adminEnv, "superadmin", "group", 8001)
	tp := m["topic"].(map[string]any)
	if got := js(tp["icon_custom_emoji_id"]) + " " + js(tp["icon_custom_emoji"]); got != `"`+ce(5)+`" `+e5 {
		t.Errorf("message page topic %s", got)
	}
	if js(m["entities"]) != js(w[8001]["entities"]) {
		t.Errorf("message page entities %s, window %s", js(m["entities"]), js(w[8001]["entities"]))
	}
}

// The ids the tables lack, asked of the daemon in one batch per kind:
// those it resolves in time are on the page, the one Telegram does not
// know stays null and is not asked again, nor are those stored now.
func TestCustomEmojiAskedOfDaemon(t *testing.T) {
	t.Parallel()
	e := newEmojiEnv(t, true, 5*time.Second)
	w := e.window()
	// The two asks run at once.
	asks := e.fake.calls()
	sort.Strings(asks)
	sameList(t, "asks", asks, []string{
		"/v1/custom-emoji " + ce(3) + "," + ce(4), "/v1/message-effects " + fx(3)})
	// With the daemon, a sticker not stored has its fetching URL.
	sameList(t, "entities", emojiOf(t, w[8001]["entities"]), []string{
		ce(1) + "=" + e.sticker("\U0001F642", "webp", 876001, &fls),
		ce(2) + "=" + e.sticker("\U0001F600", "tgs", 876002, &tru),
		ce(3) + "=" + e.sticker("c", "webm", 876006, &fls), ce(4) + "=null",
		ce(5) + "=" + e.sticker("\U0001F916", "", 0, &fls), "-"})
	if got := js(w[8004]["effect"]); got != e.sticker("p", "webm", 876007, nil) {
		t.Errorf("effect 3: %s", got)
	}

	e.res.wait()
	w = e.window()
	if n := len(e.fake.calls()); n != 2 {
		t.Errorf("asked again: %q", e.fake.calls())
	}
	if got := emojiOf(t, w[8001]["entities"])[2]; got != ce(3)+"="+e.sticker("c", "webm", 876006, &fls) {
		t.Errorf("stored custom emoji 3: %s", got)
	}
	// An hour on, the unknown one is asked for again.
	e.advance(emojiMissingRetry + time.Second)
	e.window()
	e.res.wait()
	sameList(t, "asks an hour on", e.fake.calls()[2:], []string{"/v1/custom-emoji " + ce(4)})
}

// A daemon slower than the page's budget: the page answers in about
// the budget with the fallbacks; a page meanwhile does not ask for the
// same ids again, and a page after the answer has them from the tables.
func TestCustomEmojiSlowDaemon(t *testing.T) {
	t.Parallel()
	e := newEmojiEnv(t, true, 100*time.Millisecond)
	e.fake.set("block")
	start := time.Now()
	w := e.window()
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("page took %v", d)
	}
	if js(w[8004]["effect"]) != "null" || emojiOf(t, w[8001]["entities"])[2] != ce(3)+"=null" {
		t.Errorf("not resolved in time, yet: %s %s", js(w[8004]["effect"]), js(w[8001]["entities"]))
	}
	e.window()
	if n := len(e.fake.calls()); n != 2 {
		t.Errorf("asks while one is in flight: %q", e.fake.calls())
	}
	close(e.fake.release)
	e.res.wait()
	w = e.window()
	if js(w[8004]["effect"]) != e.sticker("p", "webm", 876007, nil) {
		t.Errorf("effect 3 later: %s", js(w[8004]["effect"]))
	}
	if n := len(e.fake.calls()); n != 2 {
		t.Errorf("asks after the answer: %q", e.fake.calls())
	}
}

// A daemon that fails, or a flood wait: the page has the fallbacks, and
// no page asks again before the pause is over.
func TestCustomEmojiDaemonAway(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode  string
		pause time.Duration
	}{{"down", emojiAwayRetry}, {"flood", 120 * time.Second}} {
		t.Run(c.mode, func(t *testing.T) {
			t.Parallel()
			e := newEmojiEnv(t, true, 5*time.Second)
			e.fake.set(c.mode)
			w := e.window()
			e.res.wait()
			if js(w[8004]["effect"]) != "null" || len(e.fake.calls()) != 2 {
				t.Fatalf("first page: %s %q", js(w[8004]["effect"]), e.fake.calls())
			}
			e.fake.set("ok")
			e.advance(c.pause - time.Second)
			e.window()
			e.res.wait()
			if n := len(e.fake.calls()); n != 2 {
				t.Errorf("asked during the pause: %q", e.fake.calls())
			}
			e.advance(2 * time.Second)
			e.window()
			e.res.wait()
			if n := len(e.fake.calls()); n != 4 {
				t.Errorf("not asked after the pause: %q", e.fake.calls())
			}
		})
	}
}

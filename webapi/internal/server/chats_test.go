// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/testdb"
)

// The chat API, GET /api/entities and the page loaders of the user,
// group, chat, message and dashboard pages, over the "sample" and
// "chats" fixtures of the logger and the "accounts" fixture of the web
// (superadmin 1 "admin", admin 3 "gw_admin", member 5 "researcher").

const (
	pagingLab = -1001000000020 // 130 messages, not exposed
	openLab   = -1001000000021 // exposed
	gnuWeeb   = -1001000000001 // the sample's, exposed
	rina      = 9299000020     // the private chat
	dewi      = 9299000001
)

// chatNow is the clock of these tests: an hour after the sample's
// "now".
var chatNow = time.Unix(1794931200+3600, 0)

// labMsg is the message id of Paging Lab's message n.
func labMsg(n int64) int64 { return (700000 + n) << 20 }

type chatEnv struct {
	t       *testing.T
	ro      *sql.DB
	h       http.Handler
	cookies *auth.Cookies
	tokens  *auth.FileTokens
}

func newChatEnv(t *testing.T, withDaemon bool) *chatEnv {
	t.Helper()
	return newChatEnvFor(t, withDaemon, nil, "sample", "chats")
}

// newChatEnvFor is newChatEnv over other logger fixtures, with the
// accounts' preferences from prefs when it is not nil.
func newChatEnvFor(t *testing.T, withDaemon bool, prefs func(context.Context, uint64) (settings.Prefs, error),
	fixtures ...string) *chatEnv {
	t.Helper()
	ro := testdb.New(t, testdb.Logger, fixtures...).DB
	app := testdb.New(t, testdb.Web, "accounts").DB
	signer := auth.NewSigner(testKey)
	e := &chatEnv{t: t, ro: ro, cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		tokens: auth.NewFileTokens(testKey)}
	var dc *daemon.Client
	if withDaemon {
		// Never called: only whether the daemon API is on matters.
		var err error
		if dc, err = daemon.New("http://127.0.0.1:9", testKey, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	e.h = New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      ro,
		App:     app,
		Signer:  signer,
		Cookies: e.cookies,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Daemon:  dc,
		Tokens:  e.tokens,
		Media:   MediaConfig{StoreFileCap: 1 << 30},
		clock:   func() time.Time { return chatNow },
		prefs:   prefs,
	})
	return e
}

// roExec adds rows of a test's own to its logger database.
func (e *chatEnv) roExec(q string, args ...any) (sql.Result, error) {
	return e.ro.Exec(q, args...)
}

// as returns the session cookie of a fixture account; "" is signed out.
func (e *chatEnv) as(who string) string {
	accts := map[string]struct {
		id   uint64
		role string
	}{"admin": {1, "superadmin"}, "gw_admin": {3, "admin"}, "researcher": {5, "member"}}
	if who == "" {
		return ""
	}
	a, ok := accts[who]
	if !ok {
		e.t.Fatalf("no account %q", who)
	}
	return e.cookies.Issue(httptest.NewRecorder(), e.cookies.NewSession(a.id, who, a.role, 0))
}

func (e *chatEnv) get(path, cookie string) (int, http.Header, []byte) {
	e.t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: cookie})
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w.Code, w.Header(), w.Body.Bytes()
}

// decode GETs path and decodes the answer into v, which must come with
// status want.
func (e *chatEnv) decode(path, cookie string, want int, v any) {
	e.t.Helper()
	code, _, body := e.get(path, cookie)
	if code != want {
		e.t.Fatalf("GET %s: %d, want %d: %s", path, code, want, body)
	}
	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			e.t.Fatalf("GET %s: %v: %s", path, err, body)
		}
	}
}

type tMedia struct {
	Stored, Fetchable bool
	URL, Thumb        *string
	Name              string
	Size              uint64
}

type tUser struct {
	ID          int64
	Name        string
	Username    string
	Type        string
	Photo       *string
	ContactName string  `json:"contact_name"`
	MsgCount    *uint64 `json:"msg_count"`
	CreatedAt   *string `json:"created_at"`
	Bio         *string
}

type tMsg struct {
	ID          uint64
	MessageID   int64 `json:"message_id"`
	Date        string
	Scope       string
	Sender      *tUser
	ContentType string `json:"content_type"`
	Text        string
	Media       *tMedia
	Album       []tMedia
	ReplyTo     *struct {
		MessageID  int64  `json:"message_id"`
		SenderID   int64  `json:"sender_id"`
		SenderName string `json:"sender_name"`
		Snippet    string
		ChatID     *int64 `json:"chat_id"`
	} `json:"reply_to"`
	Forward     *string
	Edits       []struct{ At, Text string }
	EditedAt    *string `json:"edited_at"`
	DeletedAt   *string `json:"deleted_at"`
	LinkPreview *struct {
		Site, Title, Desc, URL string
		Media                  *tMedia
	} `json:"link_preview"`
}

type tWindow struct {
	Messages []tMsg
	HasOlder bool `json:"has_older"`
	HasNewer bool `json:"has_newer"`
}

func (e *chatEnv) window(cookie, scope string, id int64, query string) tWindow {
	e.t.Helper()
	var w tWindow
	e.decode(fmt.Sprintf("/api/chats/%s/%d/messages%s", scope, id, query), cookie, http.StatusOK, &w)
	return w
}

func byRow(ms []tMsg, id uint64) *tMsg {
	for i := range ms {
		if ms[i].ID == id {
			return &ms[i]
		}
	}
	return nil
}

func TestChatNewestWindow(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, true)
	w := e.window(e.as("admin"), "group", pagingLab, "")
	if !w.HasOlder || w.HasNewer {
		t.Errorf("has_older %v has_newer %v, want true false", w.HasOlder, w.HasNewer)
	}
	// n = 71 - 130, with 70 read to complete the album 70 - 72, which
	// folds into one message.
	if len(w.Messages) != 59 {
		t.Fatalf("%d messages, want 59", len(w.Messages))
	}
	first, last := w.Messages[0], w.Messages[len(w.Messages)-1]
	if first.ID != 1070 || first.MessageID != labMsg(70) || len(first.Album) != 2 ||
		first.Text != "edge album caption" || first.Media == nil {
		t.Errorf("first %+v, want the album bubble of 1070 with two more items", first)
	}
	if last.ID != 1130 {
		t.Errorf("last %d, want 1130", last.ID)
	}
	for i := 1; i < len(w.Messages); i++ {
		if w.Messages[i].MessageID <= w.Messages[i-1].MessageID {
			t.Fatalf("not oldest first at %d", i)
		}
	}
	if s := first.Sender; s == nil || s.ID != dewi || s.Name != "Dewi Kusuma" ||
		s.Username != "dewikusuma" || s.Photo == nil {
		t.Errorf("sender %+v", first.Sender)
	}
	if first.Date != "2026-11-16T17:10:00Z" || first.Scope != "group" {
		t.Errorf("date %s scope %s", first.Date, first.Scope)
	}
}

// TestChatPaging walks the chat both ways and checks every message
// comes exactly once, in order, whatever the windows cut.
func TestChatPaging(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	c := e.as("admin")
	all := map[uint64]int{}
	w := e.window(c, "group", pagingLab, "?limit=17")
	var older []tMsg
	for {
		older = append(append([]tMsg{}, w.Messages...), older...)
		if !w.HasOlder {
			break
		}
		w = e.window(c, "group", pagingLab, fmt.Sprintf("?limit=17&before=%d", w.Messages[0].MessageID))
	}
	for _, m := range older {
		all[m.ID]++
	}
	// 130 rows, two albums of three: 126 messages.
	if len(all) != 126 {
		t.Errorf("%d distinct messages walking back, want 126", len(all))
	}
	for id, n := range all {
		if n != 1 {
			t.Errorf("message %d came %d times", id, n)
		}
	}

	// Forward from the start: the client drops a message it already
	// has (by id), as an album cut by the cursor comes again whole.
	seen := map[uint64]bool{}
	var fwd []tMsg
	w = e.window(c, "group", pagingLab, fmt.Sprintf("?limit=17&after=%d", labMsg(0)))
	if w.HasOlder {
		t.Error("has_older at the start")
	}
	for {
		for _, m := range w.Messages {
			if !seen[m.ID] {
				seen[m.ID] = true
				fwd = append(fwd, m)
			}
		}
		if !w.HasNewer {
			break
		}
		w = e.window(c, "group", pagingLab, fmt.Sprintf("?limit=17&after=%d",
			w.Messages[len(w.Messages)-1].MessageID))
	}
	if len(fwd) != len(older) {
		t.Fatalf("%d messages forward, %d back", len(fwd), len(older))
	}
	for i := range fwd {
		if fwd[i].ID != older[i].ID || len(fwd[i].Album) != len(older[i].Album) {
			t.Fatalf("at %d: forward %d (%d items), back %d (%d items)", i, fwd[i].ID,
				len(fwd[i].Album), older[i].ID, len(older[i].Album))
		}
	}
	if a := byRow(fwd, 1040); a == nil || len(a.Album) != 2 || a.Text != "album caption" {
		t.Errorf("album 40 - 42: %+v", a)
	}

	// After the album bubble's own id: the album comes again whole.
	w = e.window(c, "group", pagingLab, fmt.Sprintf("?limit=5&after=%d", labMsg(70)))
	if w.Messages[0].ID != 1070 || len(w.Messages[0].Album) != 2 {
		t.Errorf("after the album: first %d with %d items", w.Messages[0].ID, len(w.Messages[0].Album))
	}

	// Past the ends.
	w = e.window(c, "group", pagingLab, fmt.Sprintf("?after=%d", labMsg(130)))
	if len(w.Messages) != 0 || !w.HasOlder || w.HasNewer {
		t.Errorf("after the newest: %+v", w)
	}
	w = e.window(c, "group", pagingLab, fmt.Sprintf("?before=%d", labMsg(1)))
	if len(w.Messages) != 0 || w.HasOlder || !w.HasNewer {
		t.Errorf("before the oldest: %+v", w)
	}
}

func TestChatAroundAndLimit(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	c := e.as("admin")
	w := e.window(c, "group", pagingLab, fmt.Sprintf("?around=%d&limit=3", labMsg(50)))
	if len(w.Messages) != 3 || w.Messages[0].ID != 1049 || w.Messages[1].ID != 1050 ||
		w.Messages[2].ID != 1051 || !w.HasOlder || !w.HasNewer {
		t.Errorf("around 50: %+v", w)
	}
	// Near the end the window fills from the older side's leftovers.
	w = e.window(c, "group", pagingLab, fmt.Sprintf("?around=%d&limit=4", labMsg(2)))
	if len(w.Messages) != 4 || w.Messages[0].ID != 1001 || w.HasOlder {
		t.Errorf("around 2: %+v", w)
	}
	if w = e.window(c, "group", pagingLab, "?limit=1000"); len(w.Messages) < 95 {
		t.Errorf("limit 1000: %d messages, want 100 rows", len(w.Messages))
	}
	if w = e.window(c, "group", pagingLab, "?limit=0"); len(w.Messages) != 1 {
		t.Errorf("limit 0: %d messages, want 1", len(w.Messages))
	}
	for _, q := range []string{"?before=x", "?limit=x", fmt.Sprintf("?before=%d&after=%d",
		labMsg(5), labMsg(9))} {
		code, _, body := e.get(fmt.Sprintf("/api/chats/group/%d/messages%s", pagingLab, q), c)
		if code != http.StatusBadRequest || !strings.Contains(string(body), "bad_request") {
			t.Errorf("%s: %d %s", q, code, body)
		}
	}
}

func TestChatMessageContent(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, true)
	c := e.as("admin")
	get := func(n int64) tMsg {
		w := e.window(c, "group", pagingLab, fmt.Sprintf("?around=%d&limit=1", labMsg(n)))
		if len(w.Messages) != 1 || w.Messages[0].ID != uint64(1000+n) {
			t.Fatalf("message %d: %+v", n, w.Messages)
		}
		return w.Messages[0]
	}

	m := get(10)
	if len(m.Edits) != 2 || m.Edits[0].Text != "paging message 10" ||
		m.Edits[0].At != "2026-11-16T16:10:00Z" || m.Edits[1].At != "2026-11-16T16:10:10Z" ||
		m.EditedAt == nil || *m.EditedAt != "2026-11-16T16:10:30Z" ||
		m.Text != "paging message 10, third version" {
		t.Errorf("edited: %+v", m)
	}
	if m = get(20); m.DeletedAt == nil || *m.DeletedAt != "2026-11-17T12:00:00Z" {
		t.Errorf("deleted: %+v", m.DeletedAt)
	}
	if m = get(21); m.DeletedAt != nil || m.EditedAt != nil || len(m.Edits) != 0 {
		t.Errorf("plain: %+v", m)
	}

	m = get(100)
	if r := m.ReplyTo; r == nil || r.MessageID != labMsg(99) || r.Snippet != "paging message 99" ||
		r.SenderID != 9299000001 || r.SenderName != "Dewi Kusuma" || r.ChatID != nil {
		t.Errorf("reply in the window: %+v", m.ReplyTo)
	}
	if m = get(101); m.ReplyTo == nil || m.ReplyTo.MessageID != labMsg(5) {
		t.Errorf("reply off the window: %+v", m.ReplyTo)
	}
	m = get(102)
	if r := m.ReplyTo; r == nil || r.ChatID == nil || *r.ChatID != gnuWeeb ||
		r.SenderName != "Dewi Kusuma" || r.Snippet != "anyone tried the new kernel rc yet?" {
		t.Errorf("reply to another chat: %+v", m.ReplyTo)
	}
	if m = get(103); m.ReplyTo == nil || m.ReplyTo.SenderName != "" || m.ReplyTo.ChatID == nil {
		t.Errorf("reply to an unlogged chat: %+v", m.ReplyTo)
	}

	m = get(110)
	if p := m.LinkPreview; p == nil || p.Site != "Example" || p.Title != "Kernel notes" ||
		p.Desc != "Notes on the kernel." || p.URL != "https://example.org/kernel" ||
		p.Media == nil || p.Media.URL == nil || *p.Media.URL != "/files/"+e.tokens.Encrypt(870004) {
		t.Errorf("link preview: %+v", m.LinkPreview)
	}
	if m = get(111); m.Forward == nil || *m.Forward != "Linus" {
		t.Errorf("forward: %v", m.Forward)
	}
	if m = get(112); m.Forward != nil || m.LinkPreview != nil || m.Media != nil {
		t.Errorf("plain: %+v", m)
	}

	// Not stored: fetchable from Telegram, small or not, with a session.
	m = get(120)
	if md := m.Media; md == nil || md.Stored || !md.Fetchable || md.URL == nil || md.Thumb == nil {
		t.Errorf("unstored photo: %+v", m.Media)
	}
	m = get(121)
	if md := m.Media; md == nil || md.Stored || !md.Fetchable || md.Thumb != nil ||
		md.Name != "long-talk.mp4" || md.Size != 2147483648 {
		t.Errorf("unstored video: %+v", m.Media)
	}
	m = get(40)
	if md := m.Media; md == nil || !md.Stored || md.Fetchable ||
		*md.URL != "/files/"+e.tokens.Encrypt(870001) || *md.Thumb != *md.URL {
		t.Errorf("stored photo: %+v", m.Media)
	}

	// Without the daemon nothing unstored can be fetched.
	e2 := newChatEnv(t, false)
	w := e2.window(e2.as("admin"), "group", pagingLab, fmt.Sprintf("?around=%d&limit=1", labMsg(120)))
	if md := w.Messages[0].Media; md.Fetchable || md.URL != nil {
		t.Errorf("no daemon: %+v", md)
	}
}

// TestChatFetchableSignedOut: a request without a session may fetch
// only files under the store's per-file cap (web/docs/media.md).
func TestChatFetchableSignedOut(t *testing.T) {
	t.Parallel()
	s := &Server{daemon: &daemon.Client{}, media: MediaConfig{StoreFileCap: 100},
		tokens: auth.NewFileTokens(testKey)}
	small := logdb.FileRef{ID: 1, Exists: true, Size: 99}
	big := logdb.FileRef{ID: 2, Exists: true, Size: 100}
	anon := s.viewFor(scope.Anonymous(), false)
	signed := s.viewFor(scope.Anonymous(), true)
	fetchable := func(v view, f logdb.FileRef) bool {
		l, _ := v.file(f)
		return l.Fetchable
	}
	if !fetchable(anon, small) || fetchable(anon, big) || !fetchable(signed, big) {
		t.Error("the cap does not decide for signed-out requests only")
	}
	if l, stored := anon.file(big); l.URL != nil || l.Fetch != nil || stored || l.Fetchable {
		t.Errorf("big file signed out: %+v %v", l, stored)
	}
	// Signed in, a big file is fetchable through the action only; a
	// small one is linked as well.
	if l, _ := signed.file(big); l.URL != nil || l.Fetch == nil || !l.Fetchable {
		t.Errorf("big file signed in: %+v", l)
	}
	if l, _ := signed.file(small); l.URL == nil || l.Fetch == nil || !l.Fetchable {
		t.Errorf("small file signed in: %+v", l)
	}
	if l, _ := anon.file(logdb.FileRef{ID: 3}); l.URL != nil {
		t.Error("a file without its row has a URL")
	}
}

func TestChatPrivate(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	for _, who := range []string{"admin", "gw_admin"} {
		w := e.window(e.as(who), "private", rina, "")
		// Eight rows, an album of two.
		if len(w.Messages) != 7 || w.HasOlder || w.HasNewer {
			t.Fatalf("%s: %d messages %+v", who, len(w.Messages), w)
		}
		if m := byRow(w.Messages, 1002); m == nil || m.Sender != nil || m.Scope != "private" {
			t.Errorf("%s: outgoing message %+v", who, m)
		}
		if m := byRow(w.Messages, 1005); m == nil || len(m.Album) != 1 || m.Text != "holiday" ||
			m.Sender == nil || m.Sender.Name != "Rina Hartono" {
			t.Errorf("%s: album %+v", who, m)
		}
		if m := byRow(w.Messages, 1008); m == nil || m.ReplyTo == nil ||
			m.ReplyTo.SenderName != "Rina Hartono" || m.ReplyTo.Snippet != "hai" {
			t.Errorf("%s: reply %+v", who, m)
		}
	}
	e.decode(fmt.Sprintf("/api/chats/private/%d/messages", rina), e.as("researcher"),
		http.StatusNotFound, nil)
	e.decode(fmt.Sprintf("/api/chats/private/%d/messages", rina), "", http.StatusUnauthorized, nil)
}

// TestChatScope: hidden chats answer like missing ones, 404 signed in
// and 401 signed out; exposed groups are public.
func TestChatScope(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	path := func(scope string, id int64) string {
		return fmt.Sprintf("/api/chats/%s/%d/messages", scope, id)
	}
	cases := []struct {
		who   string
		scope string
		id    int64
		want  int
	}{
		{"admin", "group", pagingLab, 200},
		{"gw_admin", "group", pagingLab, 200},
		{"researcher", "group", pagingLab, 404},
		{"researcher", "group", openLab, 200},
		{"", "group", openLab, 200},
		{"", "group", pagingLab, 401},
		{"", "group", -1009999999999, 401},
		{"", "private", rina, 401},
		{"", "bogus", rina, 401},
		{"admin", "group", -1009999999999, 404},
		{"admin", "group", rina, 404},
		{"admin", "private", 9299999999, 404},
		{"admin", "bogus", pagingLab, 404},
		{"admin", "group", 0, 404},
	}
	for _, c := range cases {
		code, h, body := e.get(path(c.scope, c.id), e.as(c.who))
		if code != c.want {
			t.Errorf("%q %s %d: %d, want %d: %s", c.who, c.scope, c.id, code, c.want, body)
		}
		if h.Get("Cache-Control") != "no-store" {
			t.Errorf("%q %s %d: Cache-Control %q", c.who, c.scope, c.id, h.Get("Cache-Control"))
		}
	}

	// A signed-out visitor sees the public chat without what its
	// scope hides: the quote of a message of a hidden group, the
	// counts of the senders, contact names.
	w := e.window("", "group", openLab, "")
	if len(w.Messages) != 4 {
		t.Fatalf("%d messages", len(w.Messages))
	}
	if m := byRow(w.Messages, 2004); m == nil || m.ReplyTo != nil {
		t.Errorf("quote of a hidden chat: %+v", m)
	}
	if m := byRow(w.Messages, 2002); m == nil || m.ReplyTo == nil || m.ReplyTo.Snippet != "open hello" {
		t.Errorf("reply within the chat: %+v", m)
	}
	for _, m := range w.Messages {
		if m.Sender == nil || m.Sender.MsgCount != nil || m.Sender.ContactName != "" ||
			m.Sender.CreatedAt != nil {
			t.Errorf("signed-out sender %+v", m.Sender)
		}
	}
	// A superadmin sees the quote and the senders in full.
	w = e.window(e.as("admin"), "group", openLab, "")
	if m := byRow(w.Messages, 2004); m == nil || m.ReplyTo == nil || m.ReplyTo.ChatID == nil {
		t.Errorf("superadmin quote: %+v", m)
	}
	if m := byRow(w.Messages, 2002); m == nil || m.Sender.MsgCount == nil ||
		m.Sender.ContactName != "Dewi (kernel)" {
		t.Errorf("superadmin sender %+v", m.Sender)
	}
}

func TestEntities(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	type ents struct {
		Users  []tUser
		Groups []struct {
			ID       int64
			Title    string
			MsgCount *uint64 `json:"msg_count"`
			Exposed  *bool
		}
	}
	q := fmt.Sprintf("/api/entities?users=%d,%d,1,%d&groups=%d,-1,%d", rina, dewi, rina, pagingLab, openLab)
	var got ents
	e.decode(q, e.as("admin"), http.StatusOK, &got)
	if len(got.Users) != 2 || got.Users[0].ID != rina || got.Users[1].ID != dewi ||
		got.Users[1].MsgCount == nil || *got.Users[1].MsgCount != 6 {
		t.Errorf("users %+v", got.Users)
	}
	if len(got.Groups) != 2 || got.Groups[0].Title != "Paging Lab" || got.Groups[0].MsgCount == nil ||
		*got.Groups[1].Exposed != true || *got.Groups[0].Exposed != false {
		t.Errorf("groups %+v", got.Groups)
	}

	// A member sees no groups yet (no grants), so no one either.
	got = ents{}
	e.decode(q, e.as("researcher"), http.StatusOK, &got)
	if len(got.Users) != 0 || len(got.Groups) != 0 {
		t.Errorf("member: %+v", got)
	}
	var empty ents
	e.decode("/api/entities", e.as("admin"), http.StatusOK, &empty)
	if empty.Users == nil || empty.Groups == nil {
		t.Error("empty lists are not []")
	}

	e.decode(q, "", http.StatusUnauthorized, nil)
	e.decode("/api/entities?users=1,x", e.as("admin"), http.StatusBadRequest, nil)
	many := strings.TrimSuffix(strings.Repeat("1,", 201), ",")
	e.decode("/api/entities?groups="+many, e.as("admin"), http.StatusBadRequest, nil)
	e.decode("/api/entities?groups="+strings.TrimSuffix(strings.Repeat("1,", 200), ","),
		e.as("admin"), http.StatusOK, nil)
}

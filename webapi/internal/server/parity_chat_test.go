// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/settings"
)

// The chat view's media and details as the Drogon chat view shows them,
// over the "sample" and "parity_chat" fixtures (the latter's header
// lists what each message is): content_extra where the row has it, the
// files table otherwise; edits with their media; forward origins,
// signatures, channel senders, service types, replies; the chat page's
// query (dates in the viewer's timezone, Drogon's after, after_ts and
// limit); the members the archive has seen; the /messages/{scope}
// redirect; and that every key answered is described in openapi.yaml.

const (
	mediaLab    = -1001000000030
	channelNews = -1001000000031
)

// mediaMsg is the message id of Media Lab's message n.
func mediaMsg(n int64) int64 { return (600000 + n) << 20 }

// mediaDate is the date of Media Lab's message n.
func mediaDate(n int64) int64 { return 1794931200 - 7200 + 60*n }

// gwAdminUTC gives the account gw_admin (3) the UTC timezone; the
// others keep the site's default, Asia/Jakarta.
func gwAdminUTC(_ context.Context, uid uint64) (settings.Prefs, error) {
	if uid == 3 {
		tz := "UTC"
		return settings.Prefs{Timezone: &tz}, nil
	}
	return settings.Prefs{}, nil
}

func newParityEnv(t *testing.T) *chatEnv {
	t.Helper()
	return newChatEnvFor(t, false, gwAdminUTC, "sample", "parity_chat")
}

type pMedia struct {
	Stored, Fetchable   bool
	URL, Thumb          *string
	Name                string
	Render, Ext, Format string
	Mime, Emoji         string
	W, H, Duration      int
	Wave                []int
	Title, Performer    string
	Minithumb           string
	Poster              *string
	Round               bool
	MessageID           int64   `json:"message_id"`
	Kind                string  `json:"kind"`
	DeletedAt           *string `json:"deleted_at"`
	FileID              uint64  `json:"file_id"`
}

type pGroup struct {
	ID    int64
	Title string
	Type  string
}

type pMsg struct {
	ID          uint64
	MessageID   int64  `json:"message_id"`
	ContentType string `json:"content_type"`
	Text        string
	Sender      *tUser
	SenderChat  *pGroup `json:"sender_chat"`
	Media       *pMedia
	Album       []pMedia
	DeletedAt   *string `json:"deleted_at"`
	Edits       []struct {
		At, Text    string
		ContentType string `json:"content_type"`
		Media       *pMedia
	}
	Forward     *string
	ForwardFrom *struct {
		Type, Name string
		UserID     *int64  `json:"user_id"`
		ChatID     *int64  `json:"chat_id"`
		MessageID  *int64  `json:"message_id"`
		Date       *string `json:"date"`
	} `json:"forward_from"`
	ReplyTo *struct {
		MessageID int64 `json:"message_id"`
		Resolved  bool
		Deleted   bool
	} `json:"reply_to"`
	AuthorSignature string `json:"author_signature"`
	IsChannelPost   bool   `json:"is_channel_post"`
	ServiceType     string `json:"service_type"`
	TdlibType       string `json:"tdlib_type"`
}

func pByRow(t *testing.T, ms []pMsg, id uint64) pMsg {
	t.Helper()
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no message row %d", id)
	return pMsg{}
}

func (e *chatEnv) pWindow(cookie string, chat int64, query string) []pMsg {
	e.t.Helper()
	var w struct{ Messages []pMsg }
	e.decode(fmt.Sprintf("/api/chats/group/%d/messages%s", chat, query), cookie,
		http.StatusOK, &w)
	return w.Messages
}

func TestChatMediaDetail(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	ms := e.pWindow(e.as("admin"), mediaLab, "")
	if len(ms) != 17 { // 19 rows, the album's three folded into one
		t.Fatalf("%d messages", len(ms))
	}

	m := pByRow(t, ms, 3001)
	mini := "data:image/jpeg;base64,/9j/4AAQ"
	if md := m.Media; md == nil || md.Render != "video" || md.W != 1280 || md.H != 720 ||
		md.Duration != 42 || md.Mime != "video/mp4" || md.Ext != "mp4" || md.Minithumb != mini ||
		md.Poster == nil || md.Thumb == nil || *md.Thumb != *md.Poster ||
		*md.Poster != "/files/"+e.tokens.Encrypt(860014) || md.URL == nil ||
		m.TdlibType != "messageVideo" {
		t.Errorf("video %+v", m.Media)
	}
	// An older row: what the files table says, nothing more.
	if md := pByRow(t, ms, 3002).Media; md == nil || md.Render != "video" || md.W != 0 ||
		md.Duration != 0 || md.Thumb != nil || md.Stored || md.Minithumb != "" {
		t.Errorf("old video %+v", md)
	}
	if md := pByRow(t, ms, 3003).Media; md == nil || md.Render != "animation" || md.W != 320 ||
		md.H != 240 || md.Duration != 3 || md.Poster != nil || md.Thumb == nil ||
		*md.Thumb != "data:image/jpeg;base64,AAEC" {
		t.Errorf("animation %+v", md)
	}
	if md := pByRow(t, ms, 3004).Media; md == nil || md.Render != "image" || md.Format != "webp" ||
		md.Emoji != "❤" || md.W != 512 || md.Thumb == nil || *md.Thumb != *md.URL {
		t.Errorf("webp sticker %+v", md)
	}
	if md := pByRow(t, ms, 3005).Media; md == nil || md.Render != "animation" || md.Format != "webm" ||
		md.Emoji != "" || md.Thumb != nil {
		t.Errorf("old webm sticker %+v", md)
	}
	if md := pByRow(t, ms, 3006).Media; md == nil || md.Render != "lottie" || md.Format != "tgs" ||
		md.Emoji != "ok" || md.Thumb != nil {
		t.Errorf("tgs sticker %+v", md)
	}
	if md := pByRow(t, ms, 3007).Media; md == nil || md.Render != "audio" || md.Duration != 7 ||
		!slices.Equal(md.Wave, []int{1, 2, 3, 4, 5, 6, 7, 31}) || md.Format != "" {
		t.Errorf("voice %+v", md)
	}
	if md := pByRow(t, ms, 3008).Media; md == nil || md.Render != "audio" || md.Title != "Song" ||
		md.Performer != "Band" || md.Duration != 200 {
		t.Errorf("audio %+v", md)
	}

	// A mixed album: each item says its own kind, id and deletion.
	a := pByRow(t, ms, 3009)
	if a.Text != "mixed album" || a.DeletedAt != nil || a.Media == nil || len(a.Album) != 2 {
		t.Fatalf("album %+v", a)
	}
	items := append([]pMedia{*a.Media}, a.Album...)
	for i, want := range []struct {
		kind, render string
		deleted      bool
	}{{"photo", "image", false}, {"video", "video", true}, {"document", "file", false}} {
		it := items[i]
		if it.Kind != want.kind || it.Render != want.render || it.MessageID != mediaMsg(int64(9+i)) ||
			(it.DeletedAt != nil) != want.deleted {
			t.Errorf("album item %d: %+v", i, it)
		}
	}
	if items[0].W != 800 || items[0].H != 600 {
		t.Errorf("album photo size %+v", items[0])
	}
	// Outside albums, media carries no item fields.
	if md := pByRow(t, ms, 3001).Media; md.Kind != "" || md.MessageID != 0 {
		t.Errorf("item fields outside an album: %+v", md)
	}

	// Edits with each version's type and media.
	ed := pByRow(t, ms, 3012).Edits
	if len(ed) != 2 || ed[0].ContentType != "text" || ed[0].Media != nil ||
		ed[0].Text != "before the photo" || ed[1].ContentType != "video" || ed[1].Media == nil ||
		ed[1].Media.Render != "video" || ed[1].Media.Duration != 5 || ed[1].Media.URL == nil {
		t.Errorf("edits %+v", ed)
	}

	f := pByRow(t, ms, 3013)
	if ff := f.ForwardFrom; ff == nil || ff.Type != "user" || ff.UserID == nil ||
		*ff.UserID != 9299000003 || ff.Name != "Sari Wulandari" || ff.ChatID != nil ||
		ff.Date == nil || *ff.Date != iso(1794900000) || f.Forward == nil || *f.Forward != ff.Name {
		t.Errorf("forward from a user %+v", f.ForwardFrom)
	}
	if ff := pByRow(t, ms, 3014).ForwardFrom; ff == nil || ff.Type != "channel" ||
		ff.ChatID == nil || *ff.ChatID != channelNews || ff.MessageID == nil ||
		*ff.MessageID != 77<<20 || ff.Name != "Channel News" || ff.UserID != nil {
		t.Errorf("forward from a channel %+v", ff)
	}

	c := pByRow(t, ms, 3015)
	if c.SenderChat == nil || c.SenderChat.ID != channelNews || c.SenderChat.Title != "Channel News" ||
		c.SenderChat.Type != "channel" || c.Sender == nil || c.Sender.ID != channelNews ||
		c.AuthorSignature != "Editor" || c.IsChannelPost {
		t.Errorf("posted as a channel %+v %+v", c.SenderChat, c)
	}
	if pByRow(t, ms, 3013).SenderChat != nil {
		t.Error("sender_chat on a user's message")
	}

	if r := pByRow(t, ms, 3016).ReplyTo; r == nil || !r.Resolved || !r.Deleted ||
		r.MessageID != mediaMsg(10) {
		t.Errorf("reply to a deleted message %+v", r)
	}
	if r := pByRow(t, ms, 3017).ReplyTo; r == nil || r.Resolved || r.Deleted || r.MessageID != 5<<20 {
		t.Errorf("reply to an unlogged message %+v", r)
	}

	// A service message's text is its description, rebuilt to name the
	// member added.
	s := pByRow(t, ms, 3018)
	if s.ContentType != "service" || s.ServiceType != "chat_add_members" ||
		s.Text != "added Andi Santoso to the group" ||
		s.Sender == nil || s.Sender.ID != dewi {
		t.Errorf("service %+v", s)
	}

	// A kind migration 000034 added keeps its own content type, and a
	// video note renders as a round video.
	vn := pByRow(t, ms, 3019)
	if vn.ContentType != "video_note" || vn.TdlibType != "messageVideoNote" || vn.Media == nil ||
		vn.Media.Render != "video" || !vn.Media.Round || vn.Media.W != 240 || vn.Media.H != 240 ||
		vn.Media.Duration != 9 || len(vn.Media.Wave) != 8 {
		t.Errorf("video note %+v %+v", vn, vn.Media)
	}

	// A channel's own post.
	ch := e.pWindow(e.as("admin"), channelNews, "")
	if len(ch) != 1 || !ch[0].IsChannelPost || ch[0].AuthorSignature != "Ana" ||
		ch[0].SenderChat == nil || ch[0].SenderChat.ID != channelNews {
		t.Errorf("channel post %+v", ch)
	}

	// The private chat: an outgoing older row without content_extra.
	var pw struct{ Messages []pMsg }
	e.decode("/api/chats/private/9299000030/messages", e.as("admin"), http.StatusOK, &pw)
	if len(pw.Messages) != 2 || pw.Messages[0].Media == nil || pw.Messages[0].Media.W != 800 ||
		pw.Messages[1].Sender != nil || pw.Messages[1].Media == nil ||
		pw.Messages[1].Media.Render != "audio" || pw.Messages[1].Media.Wave != nil {
		t.Errorf("private %+v", pw.Messages)
	}
}

// TestChatParityScope: a member sees none of it, and a signed-out
// visitor is asked to sign in, whether the chat exists or not.
func TestChatParityScope(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	for _, path := range []string{
		fmt.Sprintf("/api/chats/group/%d/messages", mediaLab),
		fmt.Sprintf("/api/chats/group/%d/messages?after_ts=%d", mediaLab, mediaDate(3)),
	} {
		if code, _, _ := e.get(path, e.as("researcher")); code != http.StatusNotFound {
			t.Errorf("%s as a member: %d", path, code)
		}
		if code, _, _ := e.get(path, ""); code != http.StatusUnauthorized {
			t.Errorf("%s signed out: %d", path, code)
		}
	}
	notFoundPage(e, fmt.Sprintf("/groups/%d/chat?date=2026-11-17", mediaLab), "researcher")
	notFoundPage(e, "/messages/group/3015", "researcher")
}

func TestChatAfterTS(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	ck := e.as("admin")
	ms := e.pWindow(ck, mediaLab, fmt.Sprintf("?after_ts=%d&limit=3", mediaDate(3)-30))
	if len(ms) != 3 || ms[0].MessageID != mediaMsg(3) {
		t.Errorf("after_ts: %+v", ms)
	}
	// Nothing that recent: the newest messages, as Drogon answered.
	ms = e.pWindow(ck, mediaLab, fmt.Sprintf("?after_ts=%d&limit=2", mediaDate(100)))
	if len(ms) != 2 || ms[1].MessageID != mediaMsg(19) {
		t.Errorf("after_ts past the end: %+v", ms)
	}
	// The oldest messages.
	ms = e.pWindow(ck, mediaLab, "?after=0&limit=2")
	if len(ms) != 2 || ms[0].MessageID != mediaMsg(1) {
		t.Errorf("after=0: %+v", ms)
	}
	for _, q := range []string{"?after_ts=x", fmt.Sprintf("?after_ts=1&before=%d", mediaMsg(5))} {
		if code, _, _ := e.get(fmt.Sprintf("/api/chats/group/%d/messages%s", mediaLab, q), ck); code != http.StatusBadRequest {
			t.Errorf("%s: %d", q, code)
		}
	}
}

type pChatProps struct {
	Chat struct {
		Group *struct {
			ID       int64
			Members  int64
			MsgCount int64 `json:"msg_count"`
		}
	}
	Messages []pMsg
	HasOlder bool   `json:"has_older"`
	At       *int64 `json:"at"`
}

func TestPageChatQuery(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	path := fmt.Sprintf("/groups/%d/chat", mediaLab)
	at := func(who, query string) *int64 {
		t.Helper()
		return page[pChatProps](e, path+query, who, 200, "group_chat").At
	}
	is := func(got *int64, n int64) bool { return got != nil && *got == mediaMsg(n) }

	p := page[pChatProps](e, path, "admin", 200, "group_chat")
	if p.Chat.Group == nil || p.Chat.Group.Members != 5 || p.Chat.Group.MsgCount != 19 {
		t.Errorf("header %+v", p.Chat.Group)
	}
	// The group page counts the same members as the chat header.
	if g := page[tGroupProps](e, fmt.Sprintf("/groups/%d", mediaLab), "admin", 200, "group").Group; g.Members != 5 {
		t.Errorf("group page members %d, want 5", g.Members)
	}

	// Message 5 was sent at 14:05 UTC, 21:05 in Asia/Jakarta, the site's
	// default timezone; gw_admin reads times in UTC.
	for _, c := range []struct {
		who, date string
		n         int64
	}{
		{"admin", "2026-11-17T21:05", 5},
		{"admin", "2026-11-17 21:05:30", 6},
		{"admin", "2026-11-17T21:04:01", 5},
		{"admin", "2026-11-17", 1},
		{"admin", "2026-11-17T14:05:00Z", 5},
		{"gw_admin", "2026-11-17T14:05", 5},
		{"gw_admin", "2026-11-17 14:07", 7},
	} {
		if got := at(c.who, "?date="+url.QueryEscape(c.date)); !is(got, c.n) {
			t.Errorf("%s ?date=%s: at %v, want message %d", c.who, c.date, got, c.n)
		}
	}
	// 21:05 UTC is after the last message: the newest, opened at none.
	if got := at("gw_admin", "?date=2026-11-17T21:05"); got != nil {
		t.Errorf("past the end: at %v", got)
	}

	// Drogon's links: after_ts, after (0 the oldest), limit.
	if got := at("admin", fmt.Sprintf("?after_ts=%d", mediaDate(4))); !is(got, 4) {
		t.Errorf("after_ts: at %v", got)
	}
	p = page[pChatProps](e, path+"?after=0&limit=3", "admin", 200, "group_chat")
	if !is(p.At, 1) || len(p.Messages) != 3 || p.HasOlder || p.Messages[0].MessageID != mediaMsg(1) {
		t.Errorf("oldest: at %v, %d messages", p.At, len(p.Messages))
	}
	p = page[pChatProps](e, path+fmt.Sprintf("?limit=60&after=%d", mediaMsg(15)), "admin", 200, "group_chat")
	if !is(p.At, 16) || len(p.Messages) != 4 || !p.HasOlder {
		t.Errorf("after 15: at %v, %d messages", p.At, len(p.Messages))
	}
	if p = page[pChatProps](e, path+"?limit=2", "admin", 200, "group_chat"); len(p.Messages) != 2 || p.At != nil {
		t.Errorf("limit 2: %d messages", len(p.Messages))
	}
	// ?at= comes first.
	if got := at("admin", fmt.Sprintf("?at=%d&after=0", mediaMsg(12))); !is(got, 12) {
		t.Errorf("at and after: %v", got)
	}
}

func TestParseChatDate(t *testing.T) {
	t.Parallel()
	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"2026-11-17":                "2026-11-16T17:00:00Z",
		"2026-11-17T08:30":          "2026-11-17T01:30:00Z",
		"2026-11-17 08:30:15":       "2026-11-17T01:30:15Z",
		"2026-11-17T08:30:15+02:00": "2026-11-17T06:30:15Z",
	} {
		got, ok := parseChatDate(in, jkt)
		if !ok || got.UTC().Format(time.RFC3339) != want {
			t.Errorf("%s: %v %v, want %s", in, got.UTC(), ok, want)
		}
	}
	for _, bad := range []string{"", "yesterday", "2026-13-01", "2026-11-17T25:00"} {
		if _, ok := parseChatDate(bad, jkt); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestDecodeWaveform(t *testing.T) {
	t.Parallel()
	if got := decodeWaveform("QQxSzPk="); !slices.Equal(got, []int{1, 2, 3, 4, 5, 6, 7, 31}) {
		t.Errorf("waveform %v", got)
	}
	if got := decodeWaveform("not base64!"); got != nil {
		t.Errorf("malformed: %v", got)
	}
	// 3 bytes hold 4 samples; the last reads past the end as zeros.
	if got := decodeWaveform("//8A"); !slices.Equal(got, []int{31, 31, 31, 1}) {
		t.Errorf("short: %v", got)
	}
}

type pMessageProps struct {
	Message pMsg
}

func TestPageMessageParity(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	msg := func(row int) pMsg {
		t.Helper()
		return page[pMessageProps](e, fmt.Sprintf("/messages/group/%d", row), "admin", 200,
			"message").Message
	}
	if m := msg(3001); m.Media == nil || m.Media.FileID != 860001 || m.Media.Render != "video" ||
		m.Media.W != 1280 || m.Media.Duration != 42 || m.Media.Minithumb == "" ||
		m.TdlibType != "messageVideo" {
		t.Errorf("video %+v", m.Media)
	}
	// An album's message page gives its own media and the album's
	// other items, in message order, each with its message, kind and
	// deletion, as the chat's album bubble does; a message outside an
	// album has an empty album.
	if m := msg(3010); m.Media == nil || m.Media.FileID != 860010 || len(m.Album) != 2 ||
		m.Album[0].Kind != "photo" || m.Album[0].Render != "image" || m.Album[0].W != 800 ||
		m.Album[0].DeletedAt != nil || m.Album[0].MessageID == 0 ||
		m.Album[1].Kind != "document" || m.Album[1].Name != "notes.pdf" ||
		m.Album[1].MessageID <= m.Album[0].MessageID {
		t.Errorf("album page %+v %+v", m.Media, m.Album)
	}
	if m := msg(3001); m.Album == nil || len(m.Album) != 0 {
		t.Errorf("no album: %+v", m.Album)
	}
	if m := msg(3004); m.Media == nil || m.Media.Emoji != "❤" || m.Media.Format != "webp" {
		t.Errorf("sticker %+v", m.Media)
	}
	if m := msg(3012); len(m.Edits) != 2 || m.Edits[1].Media == nil || m.Edits[1].ContentType != "video" {
		t.Errorf("edits %+v", m.Edits)
	}
	if ff := msg(3014).ForwardFrom; ff == nil || ff.ChatID == nil || *ff.ChatID != channelNews ||
		ff.MessageID == nil || ff.Date == nil {
		t.Errorf("forward %+v", ff)
	}
	if m := msg(3015); m.AuthorSignature != "Editor" || m.SenderChat == nil ||
		m.SenderChat.ID != channelNews {
		t.Errorf("signature %+v", m)
	}
	if m := msg(3101); !m.IsChannelPost || m.AuthorSignature != "Ana" {
		t.Errorf("channel post %+v", m)
	}
	if m := msg(3018); m.ServiceType != "chat_add_members" {
		t.Errorf("service %+v", m)
	}
	if m := msg(3019); m.ContentType != "video_note" || m.TdlibType != "messageVideoNote" {
		t.Errorf("video note %+v", m)
	}
}

func TestMessagesRedirect(t *testing.T) {
	t.Parallel()
	e := newParityEnv(t)
	ck := e.as("admin")
	f := url.QueryEscape(`[{"field":"chat_id","op":"=","value":"-1001000000030"}]`)
	for path, loc := range map[string]string{
		"/messages/group?chat_id=-1001000000030&limit=20&before=5": "/group_messages?f=" + f + "&per=20",
		"/messages/group?limit=5000":                               "/group_messages?per=1000",
		"/messages/private":                                        "/private_messages",
		"/messages/private?chat_id=x":                              "/private_messages",
	} {
		code, h, _ := e.get(path, ck)
		if code != http.StatusFound || h.Get("Location") != loc {
			t.Errorf("%s: %d %q, want %q", path, code, h.Get("Location"), loc)
		}
	}
	// In the browser's navigation: a full load, which redirects.
	if code, _, body := e.get("/api/page/messages/group?chat_id=1", ck); code != 200 ||
		string(body) != "{\"reload\":true}\n" {
		t.Errorf("page data: %d %s", code, body)
	}
	// Signed out: sign in first, as for every page that is not public.
	if code, h, _ := e.get("/messages/group?chat_id=1", ""); code != http.StatusFound ||
		!strings.HasPrefix(h.Get("Location"), "/login?next=") {
		t.Errorf("signed out: %d %q", code, h.Get("Location"))
	}
	// The pages beside it are untouched.
	page[pMessageProps](e, "/messages/group/3001", "admin", 200, "message")
}

// specProps is the property names of a component schema, through its
// allOf parts and their $refs, read with the OpenAPI test's reader.
func specProps(t *testing.T, root *yamlNode, name string) map[string]bool {
	t.Helper()
	n := root.get("components", "schemas", name)
	if n == nil {
		t.Fatalf("no schema %s", name)
	}
	out := map[string]bool{}
	var walk func(n *yamlNode)
	walk = func(n *yamlNode) {
		if r := n.get("$ref"); r != nil {
			walk(root.get("components", "schemas", strings.TrimPrefix(r.value, "#/components/schemas/")))
		}
		if p := n.get("properties"); p != nil {
			for _, k := range p.keys {
				out[k] = true
			}
		}
		if a := n.get("allOf"); a != nil {
			for _, k := range a.keys {
				walk(a.child[k])
			}
		}
	}
	walk(n)
	return out
}

// TestChatAnswersDescribed: every key the chat window, the chat page
// and the message page answer for the fixture is a property of its
// schema in openapi.yaml, so the description stays exact as fields are
// added.
func TestChatAnswersDescribed(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseYAMLKeys(string(src))
	if err != nil {
		t.Fatal(err)
	}
	e := newParityEnv(t)
	var undescribed []string
	check := func(where, schema string, obj map[string]any) {
		props := specProps(t, root, schema)
		for k := range obj {
			if !props[k] {
				undescribed = append(undescribed, where+"."+k+" ("+schema+")")
			}
		}
	}
	var msg func(where, schema string, m map[string]any)
	media := func(where, schema string, v any) {
		if md, ok := v.(map[string]any); ok {
			check(where, schema, md)
		}
	}
	msg = func(where, schema string, m map[string]any) {
		check(where, schema, m)
		if schema == "ChatMessage" {
			media(where+".media", "ChatMedia", m["media"])
		}
		if schema == "MessageFull" {
			// MessageFull's media is described inline: MediaDetail and
			// the file's own keys.
			if md, ok := m["media"].(map[string]any); ok {
				props := specProps(t, root, "MediaDetail")
				for k := range md {
					if !props[k] && !slices.Contains([]string{"file_id", "name", "size", "stored",
						"thumb", "url"}, k) {
						undescribed = append(undescribed, where+".media."+k)
					}
				}
			}
			delete(m, "media")
		}
		for _, a := range asList(m["album"]) {
			media(where+".album", "ChatMedia", a)
		}
		for _, ed := range asList(m["edits"]) {
			if ed, ok := ed.(map[string]any); ok {
				check(where+".edits", "Edit", ed)
				media(where+".edits.media", "ChatMedia", ed["media"])
			}
		}
		media(where+".forward_from", "ForwardOrigin", m["forward_from"])
		media(where+".sender_chat", "GroupSummary", m["sender_chat"])
	}
	var win struct{ Messages []map[string]any }
	e.decode(fmt.Sprintf("/api/chats/group/%d/messages", mediaLab), e.as("admin"), 200, &win)
	for _, m := range win.Messages {
		msg("ChatMessage", "ChatMessage", m)
	}
	var mp struct {
		Props struct{ Message map[string]any }
	}
	for _, row := range []int{3001, 3012, 3014, 3015} {
		e.decode(fmt.Sprintf("/api/page/messages/group/%d", row), e.as("admin"), 200, &mp)
		msg("MessageFull", "MessageFull", mp.Props.Message)
	}
	// The reply object is inline in ChatMessageBase.
	reply := root.get("components", "schemas", "ChatMessageBase", "properties", "reply_to",
		"oneOf", "-0", "properties")
	replies := 0
	for _, m := range win.Messages {
		if r, ok := m["reply_to"].(map[string]any); ok {
			replies++
			for k := range r {
				if reply.get(k) == nil {
					undescribed = append(undescribed, "ChatMessage.reply_to."+k)
				}
			}
		}
	}
	if replies == 0 {
		t.Error("no reply in the window")
	}
	sort.Strings(undescribed)
	if len(undescribed) > 0 {
		t.Errorf("answered but not described: %s", strings.Join(slices.Compact(undescribed), ", "))
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

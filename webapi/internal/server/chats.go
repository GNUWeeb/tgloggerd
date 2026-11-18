// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The chat API (GET /api/chats/{scope}/{id}/messages), GET
// /api/entities, and the JSON shapes of users, groups and messages the
// pages share (webapi/openapi.yaml: UserSummary, GroupSummary,
// ChatMessage). The window semantics are the Drogon /v1 chat API's
// (web/src/controllers/ApiController.cpp): the same message ids as
// cursors, replies resolved within the chat, albums folded into one
// message, media with the same stored and fetchable rules
// (web/docs/media.md).

// view turns rows into JSON for one request: its scope decides which
// fields a user or a group shows, and whether it has a session which
// files it may fetch.
type view struct {
	s       *Server
	sc      scope.Scope
	session bool
}

func (s *Server) viewFor(sc scope.Scope, session bool) view {
	return view{s: s, sc: sc, session: session}
}

// full is whether the viewer sees users and groups in full: their
// counts, contact names and exposure (web-accounts.md section 5).
func (v view) full() bool {
	return v.sc.SeesAllUsers()
}

// iso writes a unix time as the API's timestamps are written.
func iso(t int64) string {
	return time.Unix(t, 0).UTC().Format(time.RFC3339)
}

func isoPtr(t int64) *string {
	if t == 0 {
		return nil
	}
	s := iso(t)
	return &s
}

// file is a file's /files/<token> URL, when its bytes are stored or
// may be fetched from Telegram by this request (Server.fileURL), and
// both flags.
func (v view) file(f logdb.FileRef) (url *string, stored, fetchable bool) {
	if f.ID == 0 || !f.Exists {
		return nil, false, false
	}
	url, fetchable = v.s.fileURL(v.sc, v.session, f.ID, f.Size, f.OnDisk)
	return url, f.OnDisk, fetchable
}

// photo is a picture's URL, or nil.
func (v view) photo(f logdb.FileRef) *string {
	u, _, _ := v.file(f)
	return u
}

type userSummaryJSON struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Username    string  `json:"username"`
	Type        string  `json:"type"`
	Photo       *string `json:"photo"`
	ContactName string  `json:"contact_name,omitempty"`
	*userCountsJSON
}

// userCountsJSON are the fields only a full-access viewer gets.
type userCountsJSON struct {
	MsgCount   uint64 `json:"msg_count"`
	IsVerified bool   `json:"is_verified"`
	IsPremium  bool   `json:"is_premium"`
	IsScam     bool   `json:"is_scam"`
	IsFake     bool   `json:"is_fake"`
	IsSupport  bool   `json:"is_support"`
	Language   string `json:"language"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// user is a user's summary as the viewer may see it: in full, or name
// and photo only.
func (v view) user(u logdb.User) userSummaryJSON {
	j := userSummaryJSON{ID: u.ID, Name: u.Name(), Username: u.Username, Type: u.Type,
		Photo: v.photo(u.Photo)}
	if v.full() {
		j.ContactName = u.ContactName()
		j.userCountsJSON = &userCountsJSON{MsgCount: u.MsgCount, IsVerified: u.IsVerified,
			IsPremium: u.IsPremium, IsScam: u.IsScam, IsFake: u.IsFake, IsSupport: u.IsSupport,
			Language: u.Language, CreatedAt: iso(u.CreatedAt), UpdatedAt: iso(u.UpdatedAt)}
	}
	return j
}

// userOf is the user with this id from a loaded set; one whose row is
// missing (never seen in full) still gets a summary.
func (v view) userOf(users map[int64]logdb.User, id int64) userSummaryJSON {
	if u, ok := users[id]; ok {
		return v.user(u)
	}
	return userSummaryJSON{ID: id, Name: "#" + strconv.FormatInt(id, 10), Type: "unknown"}
}

type groupSummaryJSON struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Username string  `json:"username"`
	Type     string  `json:"type"`
	Photo    *string `json:"photo"`
	MsgCount *uint64 `json:"msg_count,omitempty"`
	Exposed  *bool   `json:"exposed,omitempty"`
}

// group is a group's summary; its count and exposure only for a
// full-access viewer.
func (v view) group(g logdb.Group) groupSummaryJSON {
	j := groupSummaryJSON{ID: g.ID, Title: g.Title, Username: g.Username, Type: g.Type,
		Photo: v.photo(g.Photo)}
	if v.full() {
		n, e := g.MsgCount, g.Exposed
		j.MsgCount, j.Exposed = &n, &e
	}
	return j
}

type chatMediaJSON struct {
	Stored    bool    `json:"stored"`
	Fetchable bool    `json:"fetchable"`
	Thumb     *string `json:"thumb"`
	URL       *string `json:"url"`
	Name      string  `json:"name,omitempty"`
	Size      uint64  `json:"size,omitempty"`
}

// isPicture is whether a message's media shows as an image, so its URL
// is also its thumbnail.
func isPicture(ctype string, f logdb.FileRef) bool {
	switch ctype {
	case "photo":
		return true
	case "sticker":
		// Video (.webm) and Lottie (.tgs) stickers are not images.
		return f.Ext != "webm" && f.Ext != "tgs"
	}
	return f.Type == "photo"
}

func (v view) media(ctype string, f logdb.FileRef) *chatMediaJSON {
	if f.ID == 0 {
		return nil
	}
	url, stored, fetchable := v.file(f)
	m := &chatMediaJSON{Stored: stored, Fetchable: fetchable, URL: url, Name: f.Name, Size: f.Size}
	if url != nil && isPicture(ctype, f) {
		m.Thumb = url
	}
	return m
}

type chatReplyJSON struct {
	MessageID  int64  `json:"message_id"`
	SenderID   int64  `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Snippet    string `json:"snippet"`
	// ChatID is set when the target is in another chat.
	ChatID *int64 `json:"chat_id,omitempty"`
}

type editJSON struct {
	At   string `json:"at"`
	Text string `json:"text"`
}

type linkPreviewJSON struct {
	Site  string         `json:"site"`
	Title string         `json:"title"`
	Desc  string         `json:"desc"`
	URL   string         `json:"url"`
	Media *chatMediaJSON `json:"media,omitempty"`
}

type chatMessageJSON struct {
	ID          uint64           `json:"id"`
	MessageID   int64            `json:"message_id"`
	Date        string           `json:"date"`
	Scope       string           `json:"scope"`
	Sender      *userSummaryJSON `json:"sender"`
	ContentType string           `json:"content_type"`
	Text        string           `json:"text"`
	Media       *chatMediaJSON   `json:"media"`
	Album       []chatMediaJSON  `json:"album"`
	ReplyTo     *chatReplyJSON   `json:"reply_to"`
	Forward     *string          `json:"forward"`
	Edits       []editJSON       `json:"edits"`
	EditedAt    *string          `json:"edited_at"`
	DeletedAt   *string          `json:"deleted_at"`
	LinkPreview *linkPreviewJSON `json:"link_preview"`
}

type chatWindowJSON struct {
	Messages []chatMessageJSON `json:"messages"`
	HasOlder bool              `json:"has_older"`
	HasNewer bool              `json:"has_newer"`
}

func scopeName(group bool) string {
	if group {
		return "group"
	}
	return "private"
}

// names are the users and groups a set of messages refers to.
type names struct {
	users  map[int64]logdb.User
	groups map[int64]logdb.Group
}

// sender is a message's sender: nil for the logging account's own
// messages, a user, or a chat posting in its own name (a channel, an
// anonymous admin), which is shown like a user with the chat's id.
func (v view) sender(n names, m logdb.Message) *userSummaryJSON {
	switch {
	case m.Outgoing:
		return nil
	case m.SenderChat != 0:
		j := userSummaryJSON{ID: m.SenderChat, Name: "#" + strconv.FormatInt(m.SenderChat, 10),
			Type: "unknown"}
		if g, ok := n.groups[m.SenderChat]; ok {
			j.Name, j.Username, j.Photo = g.Title, g.Username, v.photo(g.Photo)
		}
		return &j
	case m.SenderUser != 0:
		j := v.userOf(n.users, m.SenderUser)
		return &j
	}
	name := m.AuthorSignature
	if name == "" {
		name = "(unknown)"
	}
	return &userSummaryJSON{Name: name, Type: "unknown"}
}

// nameOf is the display name of a user or a chat, "" when unknown.
func (n names) nameOf(user, chat int64) string {
	if chat != 0 {
		if g, ok := n.groups[chat]; ok {
			return g.Title
		}
		return ""
	}
	if u, ok := n.users[user]; ok {
		return u.Name()
	}
	return ""
}

func (n names) forward(m logdb.Message) *string {
	f := m.Forward
	if f == nil {
		return nil
	}
	name := f.SenderName
	if name == "" {
		name = n.nameOf(f.UserID, 0)
	}
	if name == "" {
		name = n.nameOf(0, f.ChatID)
	}
	if name == "" {
		name = "Unknown"
	}
	return &name
}

func snippetOf(text, ctype string) string {
	if text == "" && ctype != "" && ctype != "text" {
		return "[" + ctype + "]"
	}
	return text
}

// reply is what a message replies to, as the viewer may see it: a
// target in another chat is shown only to a viewer that sees that
// chat, and otherwise not at all.
func (v view) reply(n names, chatID int64, m logdb.Message) *chatReplyJSON {
	r := m.Reply
	if r == nil {
		return nil
	}
	j := &chatReplyJSON{MessageID: r.MessageID}
	if r.ChatID != chatID {
		if !v.sc.SeesChat(r.ChatID) {
			return nil
		}
		c := r.ChatID
		j.ChatID = &c
	}
	if !r.Resolved {
		return j
	}
	switch {
	case r.SenderChat != 0:
		j.SenderID, j.SenderName = r.SenderChat, n.nameOf(0, r.SenderChat)
	case r.SenderUser != 0:
		j.SenderID, j.SenderName = r.SenderUser, n.nameOf(r.SenderUser, 0)
	case r.Outgoing:
		j.SenderName = "You"
	}
	j.Snippet = snippetOf(r.Snippet, r.ContentType)
	return j
}

func (v view) edits(m logdb.Message) []editJSON {
	out := []editJSON{}
	for _, e := range m.Edits {
		out = append(out, editJSON{At: iso(e.At), Text: e.Text})
	}
	return out
}

func (v view) chatMessage(n names, group bool, chatID int64, m logdb.Message) chatMessageJSON {
	j := chatMessageJSON{
		ID: m.RowID, MessageID: m.MessageID, Date: iso(m.Date), Scope: scopeName(group),
		Sender: v.sender(n, m), ContentType: m.ContentType, Text: m.Text,
		Media: v.media(m.ContentType, m.File), Album: []chatMediaJSON{},
		ReplyTo: v.reply(n, chatID, m), Forward: n.forward(m), Edits: v.edits(m),
		DeletedAt: isoPtr(m.DeletedAt),
	}
	if m.EditDate > 0 {
		j.EditedAt = isoPtr(m.EditDate)
	}
	if p := m.Preview; p != nil {
		j.LinkPreview = &linkPreviewJSON{Site: p.Site, Title: p.Title, Desc: p.Description,
			URL: p.URL, Media: v.media("photo", p.Photo)}
	}
	return j
}

// chatMessages turns a window's rows into messages, folding each run
// of two or more consecutive rows of one album into one message, as
// Browse.cpp's chatHistory does: the first item's ids, date and sender,
// its media and the others' as the album, the caption (and its edits)
// of the item that has one, the first reply and forward found. Such a
// message is deleted only when all its items are.
func (v view) chatMessages(n names, group bool, chatID int64, ms []logdb.Message) []chatMessageJSON {
	out := []chatMessageJSON{}
	for i := 0; i < len(ms); {
		k := i + 1
		if a := ms[i].AlbumID; a != 0 {
			for k < len(ms) && ms[k].AlbumID == a {
				k++
			}
		}
		j := v.chatMessage(n, group, chatID, ms[i])
		if k-i > 1 {
			var lastDeleted int64
			allDeleted := true
			for x := i; x < k; x++ {
				item := v.chatMessage(n, group, chatID, ms[x])
				if x > i && item.Media != nil {
					j.Album = append(j.Album, *item.Media)
				}
				if j.Text == "" && item.Text != "" {
					j.Text, j.Edits, j.EditedAt = item.Text, item.Edits, item.EditedAt
				}
				if j.ReplyTo == nil {
					j.ReplyTo = item.ReplyTo
				}
				if j.Forward == nil {
					j.Forward = item.Forward
				}
				if ms[x].DeletedAt == 0 {
					allDeleted = false
				}
				lastDeleted = max(lastDeleted, ms[x].DeletedAt)
			}
			j.DeletedAt = nil
			if allDeleted {
				j.DeletedAt = isoPtr(lastDeleted)
			}
		}
		out = append(out, j)
		i = k
	}
	return out
}

func (v view) window(c logdb.Chat, w logdb.Window) chatWindowJSON {
	return chatWindowJSON{
		Messages: v.chatMessages(names{w.Users, w.Groups}, c.Group, c.ID, w.Messages),
		HasOlder: w.HasOlder, HasNewer: w.HasNewer,
	}
}

// The window sizes of getChatMessages.
const (
	chatDefaultLimit = 60
	chatMaxLimit     = 100
)

var errBadAnchor = errors.New("at most one of before, after and around")

// chatAnchor reads before, after, around and limit; limit is clamped
// to 1..chatMaxLimit, as the Drogon API clamps it.
func chatAnchor(q map[string][]string) (logdb.Anchor, int, error) {
	var (
		a   logdb.Anchor
		set int
	)
	for _, p := range []struct {
		name string
		dst  **int64
	}{{"before", &a.Before}, {"after", &a.After}, {"around", &a.Around}} {
		vs := q[p.name]
		if len(vs) == 0 || vs[0] == "" {
			continue
		}
		n, err := strconv.ParseInt(vs[0], 10, 64)
		if err != nil {
			return a, 0, errors.New(p.name + " is not a message id")
		}
		*p.dst = &n
		set++
	}
	if set > 1 {
		return a, 0, errBadAnchor
	}
	limit := chatDefaultLimit
	if vs := q["limit"]; len(vs) > 0 && vs[0] != "" {
		n, err := strconv.Atoi(vs[0])
		if err != nil {
			return a, 0, errors.New("limit is not a number")
		}
		limit = min(max(n, 1), chatMaxLimit)
	}
	return a, limit, nil
}

// optionalSession is the session of a request that signed-out visitors
// may make too: nil without one. A gate of a signed-in account answers
// as RequireSession answers it; ok is false when the answer is sent.
func (s *Server) optionalSession(w http.ResponseWriter, r *http.Request) (sess *auth.Session, sc scope.Scope, ok bool) {
	ps, valid := s.pageSession(w, r)
	if !valid {
		noStore(w)
		writeError(w, http.StatusServiceUnavailable, "unavailable", nil)
		return nil, sc, false
	}
	sc = s.requestScope(r, ps)
	if g := sc.Gate(); g != scope.None {
		noStore(w)
		writeError(w, http.StatusForbidden, string(g), map[string]any{"message": gateMessages[g]})
		return nil, sc, false
	}
	return ps, sc, true
}

// getChatMessages is GET /api/chats/{scope}/{id}/messages. Signed-out
// visitors may read the chats of exposed groups, as the public chat
// page does; for anything else they get 401, whether it exists or not,
// so they learn nothing about it. A signed-in viewer gets 404 for what
// its scope hides.
func (s *Server) getChatMessages(w http.ResponseWriter, r *http.Request) {
	sess, sc, ok := s.optionalSession(w, r)
	if !ok {
		return
	}
	noStore(w)
	hidden := func() {
		if sess == nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
			return
		}
		notFound(w)
	}
	kind := r.PathValue("scope")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if (kind != "group" && kind != "private") || err != nil {
		hidden()
		return
	}
	c, err := logdb.OpenChat(r.Context(), s.ro, sc, kind == "group", id)
	if errors.Is(err, logdb.ErrNotFound) {
		hidden()
		return
	}
	if err != nil {
		s.internalError(w, "chat lookup", err)
		return
	}
	a, limit, err := chatAnchor(r.URL.Query())
	if err != nil {
		badRequest(w, "bad_request", err.Error())
		return
	}
	win, err := c.Window(r.Context(), s.ro, a, limit)
	if err != nil {
		s.internalError(w, "chat window", err)
		return
	}
	writeJSON(w, http.StatusOK, s.viewFor(sc, sess != nil).window(c, win))
}

// entitiesMax is how many ids each list of GET /api/entities takes.
const entitiesMax = 200

func parseIDs(s string) ([]int64, bool) {
	if s == "" {
		return nil, true
	}
	parts := strings.Split(s, ",")
	if len(parts) > entitiesMax {
		return nil, false
	}
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, false
		}
		ids = append(ids, n)
	}
	return ids, true
}

type entitiesJSON struct {
	Users  []userSummaryJSON  `json:"users"`
	Groups []groupSummaryJSON `json:"groups"`
}

// getEntities is GET /api/entities: the summaries of the users and
// groups asked for that the scope sees; the others are left out.
func (s *Server) getEntities(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	q := r.URL.Query()
	uids, ok1 := parseIDs(q.Get("users"))
	gids, ok2 := parseIDs(q.Get("groups"))
	if !ok1 || !ok2 {
		badRequest(w, "bad_request", "At most 200 numeric ids per list.")
		return
	}
	us, err := logdb.VisibleUsers(r.Context(), s.ro, sc, uids)
	if err != nil {
		s.internalError(w, "entities", err)
		return
	}
	gs, err := logdb.VisibleGroups(r.Context(), s.ro, sc, gids)
	if err != nil {
		s.internalError(w, "entities", err)
		return
	}
	v := s.viewFor(sc, true)
	out := entitiesJSON{Users: []userSummaryJSON{}, Groups: []groupSummaryJSON{}}
	for _, u := range us {
		out.Users = append(out.Users, v.user(u))
	}
	for _, g := range gs {
		out.Groups = append(out.Groups, v.group(g))
	}
	writeJSON(w, http.StatusOK, out)
}

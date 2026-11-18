// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"strconv"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/settings"
)

// The loaders of the dashboard, user, group, chat and message pages
// (webapi/openapi.yaml: DashboardProps, UserProps, GroupProps,
// ChatProps, MessageProps). Each reads through the request's scope;
// what it hides is errPageNotFound, the same as what does not exist.

func (pr *pageRequest) view() view {
	return pr.s.viewFor(pr.scope, pr.sess != nil)
}

func (pr *pageRequest) now() int64 {
	return pr.s.clock().Unix()
}

// pathID is the {id} of the page's path, which matchPage has checked.
func (pr *pageRequest) pathID() int64 {
	n, _ := strconv.ParseInt(pr.params["id"], 10, 64)
	return n
}

// notFoundOr turns logdb.ErrNotFound into the not_found page.
func notFoundOr(err error) error {
	if errors.Is(err, logdb.ErrNotFound) {
		return errPageNotFound
	}
	return err
}

type kpiJSON struct {
	Key   string `json:"key"`
	Total int64  `json:"total"`
	Today int64  `json:"today"`
}

type groupCountedJSON struct {
	groupSummaryJSON
	MsgCount uint64 `json:"msg_count"`
}

type deletedJSON struct {
	ID        uint64           `json:"id"`
	Scope     string           `json:"scope"`
	Chat      groupSummaryJSON `json:"chat"`
	Sender    userSummaryJSON  `json:"sender"`
	Snippet   string           `json:"snippet"`
	DeletedAt string           `json:"deleted_at"`
	MessageID int64            `json:"message_id"`
}

type dashboardProps struct {
	KPIs    []kpiJSON          `json:"kpis"`
	PerDay  []int64            `json:"per_day"`
	Busiest []groupCountedJSON `json:"busiest"`
	Deleted []deletedJSON      `json:"deleted"`
}

func loadDashboard(ctx context.Context, pr *pageRequest) (any, error) {
	d, err := logdb.LoadDashboard(ctx, pr.s.ro, pr.scope, pr.now())
	if err != nil {
		return nil, err
	}
	v := pr.view()
	p := dashboardProps{KPIs: []kpiJSON{}, PerDay: d.PerDay, Busiest: []groupCountedJSON{},
		Deleted: []deletedJSON{}}
	for _, k := range d.KPIs {
		p.KPIs = append(p.KPIs, kpiJSON(k))
	}
	for _, g := range d.Busiest {
		gj := v.group(g)
		gj.MsgCount = nil
		p.Busiest = append(p.Busiest, groupCountedJSON{groupSummaryJSON: gj, MsgCount: g.MsgCount})
	}
	n := names{d.Users, d.Groups}
	for _, m := range d.Deleted {
		g, ok := d.Groups[m.ChatID]
		if !ok {
			g = logdb.Group{ID: m.ChatID, Title: "(no title)"}
		}
		s := v.sender(n, m)
		if s == nil {
			s = &userSummaryJSON{Name: "(unknown)", Type: "unknown"}
		}
		p.Deleted = append(p.Deleted, deletedJSON{ID: m.RowID, Scope: "group", Chat: v.group(g),
			Sender: *s, Snippet: snippetOf(m.Text, m.ContentType), DeletedAt: iso(m.DeletedAt),
			MessageID: m.MessageID})
	}
	return p, nil
}

type usernameJSON struct {
	Username string `json:"username"`
	Active   bool   `json:"active"`
}

type userFullJSON struct {
	userSummaryJSON
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Bio       string         `json:"bio"`
	Phone     string         `json:"phone"`
	Usernames []usernameJSON `json:"usernames"`
}

type userGroupJSON struct {
	groupSummaryJSON
	Msgs int64 `json:"msgs"`
}

type historyJSON struct {
	Kind    string           `json:"kind"`
	At      string           `json:"at"`
	Detail  string           `json:"detail,omitempty"`
	Action  string           `json:"action,omitempty"`
	Photo   *string          `json:"photo,omitempty"`
	Removed bool             `json:"removed,omitempty"`
	User    *userSummaryJSON `json:"user,omitempty"`
}

type userProps struct {
	Limited    bool            `json:"limited"`
	User       userFullJSON    `json:"user"`
	Groups     []userGroupJSON `json:"groups"`
	History    []historyJSON   `json:"history"`
	Activity   []int64         `json:"activity"`
	CanPrivate bool            `json:"can_private"`
}

type userLimitedProps struct {
	Limited      bool               `json:"limited"`
	User         userSummaryJSON    `json:"user"`
	SharedGroups []groupSummaryJSON `json:"shared_groups"`
}

func (v view) history(e logdb.HistoryEntry) historyJSON {
	j := historyJSON{Kind: e.Kind, At: iso(e.At), Detail: e.Detail, Action: e.Action}
	if e.Kind == "photo" {
		j.Detail = ""
		if e.Photo.ID == 0 {
			j.Removed = true
		} else {
			j.Photo = v.photo(e.Photo)
		}
	}
	return j
}

func loadUserPage(ctx context.Context, pr *pageRequest, history bool) (any, error) {
	p, err := logdb.LoadUserPage(ctx, pr.s.ro, pr.scope, pr.pathID(), pr.now())
	if err != nil {
		return nil, notFoundOr(err)
	}
	v := pr.view()
	if p.Limited {
		// A member sees no one's history (web-accounts.md section 5).
		if history {
			return nil, errPageNotFound
		}
		lp := userLimitedProps{Limited: true, User: v.user(p.User.User),
			SharedGroups: []groupSummaryJSON{}}
		for _, g := range p.Groups {
			lp.SharedGroups = append(lp.SharedGroups, v.group(g.Group))
		}
		return lp, nil
	}
	up := userProps{
		User: userFullJSON{userSummaryJSON: v.user(p.User.User), FirstName: p.User.FirstName,
			LastName: p.User.LastName, Bio: p.User.Bio, Phone: p.User.Phone,
			Usernames: []usernameJSON{}},
		Groups: []userGroupJSON{}, History: []historyJSON{}, Activity: p.Activity,
		CanPrivate: pr.scope.SeesPrivate(),
	}
	for _, u := range p.User.Usernames {
		up.User.Usernames = append(up.User.Usernames, usernameJSON(u))
	}
	for _, g := range p.Groups {
		up.Groups = append(up.Groups, userGroupJSON{groupSummaryJSON: v.group(g.Group), Msgs: g.Msgs})
	}
	for _, e := range p.History {
		up.History = append(up.History, v.history(e))
	}
	return up, nil
}

func loadUser(ctx context.Context, pr *pageRequest) (any, error) {
	return loadUserPage(ctx, pr, false)
}

func loadUserHistory(ctx context.Context, pr *pageRequest) (any, error) {
	return loadUserPage(ctx, pr, true)
}

type groupFullJSON struct {
	groupSummaryJSON
	Description string `json:"description"`
	// Members is not recorded by the logger; 0 until it is.
	Members    int64  `json:"members"`
	MsgCount   uint64 `json:"msg_count"`
	AdminCount int64  `json:"admin_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type groupAdminJSON struct {
	User        userSummaryJSON `json:"user"`
	Status      string          `json:"status"`
	CustomTitle string          `json:"custom_title"`
	IsAnonymous bool            `json:"is_anonymous"`
	Perms       []string        `json:"perms"`
	Since       string          `json:"since"`
}

type posterJSON struct {
	User userSummaryJSON `json:"user"`
	Msgs int64           `json:"msgs"`
}

type groupProps struct {
	Group      groupFullJSON    `json:"group"`
	Admins     []groupAdminJSON `json:"admins"`
	History    []historyJSON    `json:"history"`
	TopPosters []posterJSON     `json:"top_posters"`
	Activity   []int64          `json:"activity"`
}

func loadGroup(ctx context.Context, pr *pageRequest) (any, error) {
	p, err := logdb.LoadGroupPage(ctx, pr.s.ro, pr.scope, pr.pathID(), pr.now())
	if err != nil {
		return nil, notFoundOr(err)
	}
	v := pr.view()
	gs := v.group(p.Group.Group)
	gs.MsgCount = nil
	g := p.Group
	out := groupProps{
		Group: groupFullJSON{groupSummaryJSON: gs, Description: g.Description,
			MsgCount: g.MsgCount, AdminCount: g.AdminCount, CreatedAt: iso(g.CreatedAt),
			UpdatedAt: iso(g.UpdatedAt)},
		Admins: []groupAdminJSON{}, History: []historyJSON{}, TopPosters: []posterJSON{},
		Activity: p.Activity,
	}
	// The people named here appear in the group, so its viewer sees
	// them, but only in full when it sees users in full.
	for _, a := range p.Admins {
		out.Admins = append(out.Admins, groupAdminJSON{User: v.userOf(p.Users, a.UserID),
			Status: a.Status, CustomTitle: a.CustomTitle, IsAnonymous: a.IsAnonymous,
			Perms: a.Perms, Since: iso(a.Since)})
	}
	for _, e := range p.History {
		h := v.history(e)
		if e.Kind == "admin" {
			u := v.userOf(p.Users, e.UserID)
			h.User = &u
		}
		out.History = append(out.History, h)
	}
	for _, t := range p.TopPosters {
		out.TopPosters = append(out.TopPosters, posterJSON{User: v.userOf(p.Users, t.UserID),
			Msgs: t.Msgs})
	}
	return out, nil
}

type chatGroupJSON struct {
	groupSummaryJSON
	// Members is how many people the archive has seen in the group
	// (logdb.Chat.KnownMembers): the logger does not record Telegram's
	// own count.
	Members  int64  `json:"members"`
	MsgCount uint64 `json:"msg_count"`
}

type chatUserJSON struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Photo *string `json:"photo"`
}

type chatHeadJSON struct {
	Kind  string         `json:"kind"`
	Group *chatGroupJSON `json:"group,omitempty"`
	User  *chatUserJSON  `json:"user,omitempty"`
	Count *int64         `json:"count,omitempty"`
}

type chatProps struct {
	Chat chatHeadJSON `json:"chat"`
	chatWindowJSON
	At *int64 `json:"at"`
}

// chatDateLayouts are the forms ?date= takes, read in the viewer's
// timezone: a day, or a day and a time (with or without seconds, a T
// or a space between).
var chatDateLayouts = []string{time.DateOnly, "2006-01-02T15:04", "2006-01-02 15:04",
	"2006-01-02T15:04:05", time.DateTime}

// parseChatDate reads ?date=: one of chatDateLayouts in loc, or an
// RFC 3339 time with its own offset.
func parseChatDate(v string, loc *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	for _, l := range chatDateLayouts {
		if t, err := time.ParseInLocation(l, v, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// viewerLocation is the timezone the viewer reads times in: its own
// preference, else the site's default (pageViewer's timezone).
func (pr *pageRequest) viewerLocation(ctx context.Context) (*time.Location, error) {
	var prefs settings.Prefs
	if pr.sess != nil {
		var err error
		if prefs, err = pr.s.viewerPrefs(ctx, pr.sess.UID); err != nil {
			return nil, err
		}
	}
	loc, err := time.LoadLocation(settings.Resolve(prefs, pr.s.defaults).Timezone)
	if err != nil {
		return time.UTC, nil
	}
	return loc, nil
}

// chatPageAnchor reads the chat page's query, the first of these that
// parses: ?at=<message id> opens the window around that message;
// ?after_ts=<unix time> at the first message sent then or later, as
// the Drogon chat page did; ?date= around the first message at or
// after that day or time in the viewer's timezone; ?after=<message id>
// at the messages after it (the Drogon page's paging links, and
// ?after=0 for the oldest messages). With none, or past the end, the
// newest messages. ?limit= sizes the window as for the API. at is the
// message the page is opened at; for ?after= the window's first.
func chatPageAnchor(ctx context.Context, pr *pageRequest, c logdb.Chat) (logdb.Anchor, *int64, error) {
	q := pr.query
	if at, err := strconv.ParseInt(q.Get("at"), 10, 64); err == nil {
		return logdb.Anchor{Around: &at}, &at, nil
	}
	var ts int64
	ok := false
	if n, err := strconv.ParseInt(q.Get("after_ts"), 10, 64); err == nil {
		id, found, err := c.DateAnchor(ctx, pr.s.ro, n)
		if err != nil || !found {
			return logdb.Anchor{}, nil, err
		}
		after := id - 1
		return logdb.Anchor{After: &after}, &id, nil
	}
	if v := q.Get("date"); v != "" {
		loc, err := pr.viewerLocation(ctx)
		if err != nil {
			return logdb.Anchor{}, nil, err
		}
		var t time.Time
		if t, ok = parseChatDate(v, loc); ok {
			ts = t.Unix()
		}
	}
	if ok {
		id, found, err := c.DateAnchor(ctx, pr.s.ro, ts)
		if err != nil || !found {
			return logdb.Anchor{}, nil, err
		}
		return logdb.Anchor{Around: &id}, &id, nil
	}
	if after, err := strconv.ParseInt(q.Get("after"), 10, 64); err == nil {
		return logdb.Anchor{After: &after}, nil, nil
	}
	return logdb.Anchor{}, nil, nil
}

func loadChat(ctx context.Context, pr *pageRequest, group bool) (any, error) {
	c, err := logdb.OpenChat(ctx, pr.s.ro, pr.scope, group, pr.pathID())
	if errors.Is(err, logdb.ErrNotFound) && pr.sess == nil {
		return nil, errPageSignIn
	}
	if err != nil {
		return nil, notFoundOr(err)
	}
	a, at, err := chatPageAnchor(ctx, pr, c)
	if err != nil {
		return nil, err
	}
	_, limit, err := chatAnchor(map[string][]string{"limit": pr.query["limit"]})
	if err != nil {
		limit = chatDefaultLimit
	}
	w, err := c.Window(ctx, pr.s.ro, a, limit)
	if err != nil {
		return nil, err
	}
	if a.After != nil && at == nil && len(w.Messages) > 0 {
		first := w.Messages[0].MessageID
		at = &first
	}
	v := pr.view()
	p := chatProps{chatWindowJSON: v.window(c, w), At: at}
	if group {
		gs := v.group(c.GroupInfo)
		gs.MsgCount = nil
		members, err := c.KnownMembers(ctx, pr.s.ro)
		if err != nil {
			return nil, err
		}
		p.Chat = chatHeadJSON{Kind: "group", Group: &chatGroupJSON{groupSummaryJSON: gs,
			Members: members, MsgCount: c.GroupInfo.MsgCount}}
	} else {
		n := c.Count
		p.Chat = chatHeadJSON{Kind: "user", User: &chatUserJSON{ID: c.UserInfo.ID,
			Name: c.UserInfo.Name(), Photo: v.photo(c.UserInfo.Photo)}, Count: &n}
	}
	return p, nil
}

// loadGroupChat is the chat of a group: for a signed-out visitor only
// that of an exposed group (OpenChat decides), which is the public
// archive page search engines may index.
func loadGroupChat(ctx context.Context, pr *pageRequest) (any, error) {
	return loadChat(ctx, pr, true)
}

func loadUserChat(ctx context.Context, pr *pageRequest) (any, error) {
	return loadChat(ctx, pr, false)
}

type chatRefJSON struct {
	ID    int64   `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Photo *string `json:"photo"`
}

type messageReplyJSON struct {
	Scope      string `json:"scope"`
	ID         uint64 `json:"id"`
	SenderName string `json:"sender_name"`
	Snippet    string `json:"snippet"`
}

type messageMediaJSON struct {
	FileID uint64  `json:"file_id"`
	Name   string  `json:"name"`
	Size   uint64  `json:"size"`
	Stored bool    `json:"stored"`
	Thumb  *string `json:"thumb"`
	URL    *string `json:"url"`
	mediaDetailJSON
}

type messageFullJSON struct {
	ID          uint64            `json:"id"`
	Scope       string            `json:"scope"`
	MessageID   int64             `json:"message_id"`
	Date        string            `json:"date"`
	Chat        chatRefJSON       `json:"chat"`
	Sender      *userSummaryJSON  `json:"sender"`
	ContentType string            `json:"content_type"`
	Snippet     string            `json:"snippet"`
	Forwarded   *string           `json:"forwarded"`
	Edited      bool              `json:"edited"`
	DeletedAt   *string           `json:"deleted_at"`
	Text        string            `json:"text"`
	ReplyTo     *messageReplyJSON `json:"reply_to"`
	Edits       []editJSON        `json:"edits"`
	EditedAt    *string           `json:"edited_at"`
	Media       *messageMediaJSON `json:"media"`
	messageExtraJSON
}

type messageProps struct {
	Message messageFullJSON `json:"message"`
}

// snippetLen is how much of a message's text a snippet shows.
const snippetLen = 200

func snippet(m logdb.Message) string {
	t := []rune(m.Text)
	if len(t) > snippetLen {
		t = t[:snippetLen]
	}
	s := string(t)
	if s == "" && m.File.Name != "" {
		s = m.File.Name
	}
	return snippetOf(s, m.ContentType)
}

func loadMessage(ctx context.Context, pr *pageRequest) (any, error) {
	group := pr.params["scope"] == "group"
	id, err := strconv.ParseUint(pr.params["id"], 10, 64)
	if err != nil {
		return nil, errPageNotFound
	}
	d, err := logdb.LoadMessage(ctx, pr.s.ro, pr.scope, group, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	v := pr.view()
	n := names{d.Users, d.Groups}
	m := d.Message
	j := messageFullJSON{
		ID: m.RowID, Scope: scopeName(group), MessageID: m.MessageID, Date: iso(m.Date),
		Sender: v.sender(n, m), ContentType: chatType(m.ContentType), Snippet: snippet(m),
		Forwarded: n.forward(m), Edited: m.EditDate > 0, DeletedAt: isoPtr(m.DeletedAt),
		Text: m.Text, Edits: v.edits(m), messageExtraJSON: v.messageExtra(n, m),
	}
	if m.EditDate > 0 {
		j.EditedAt = isoPtr(m.EditDate)
	}
	if group {
		j.Chat = chatRefJSON{ID: d.ChatGroup.ID, Title: d.ChatGroup.Title, Kind: "group",
			Photo: v.photo(d.ChatGroup.Photo)}
	} else {
		j.Chat = chatRefJSON{ID: d.ChatUser.ID, Title: d.ChatUser.Name(), Kind: "user",
			Photo: v.photo(d.ChatUser.Photo)}
	}
	if r := m.Reply; r != nil && r.Resolved && (r.ChatID == m.ChatID || pr.scope.SeesChat(r.ChatID)) {
		rep := v.reply(n, m.ChatID, m)
		j.ReplyTo = &messageReplyJSON{Scope: scopeName(group), ID: r.RowID,
			SenderName: rep.SenderName, Snippet: rep.Snippet}
	}
	if m.File.ID != 0 {
		md := v.media(m.ContentType, m.File, m.Extra)
		v.poster(md, m)
		j.Media = &messageMediaJSON{FileID: m.File.ID, Name: md.Name, Size: md.Size,
			Stored: md.Stored, Thumb: md.Thumb, URL: md.URL, mediaDetailJSON: md.mediaDetailJSON}
	}
	return messageProps{Message: j}, nil
}

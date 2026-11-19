// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/webdb"
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
	KPIs   []kpiJSON `json:"kpis"`
	PerDay []int64   `json:"per_day"`
	// PerDayStart is the date of PerDay's first day in the viewer's
	// timezone (YYYY-MM-DD); each count is one calendar day there.
	PerDayStart string             `json:"per_day_start"`
	Busiest     []groupCountedJSON `json:"busiest"`
	Deleted     []deletedJSON      `json:"deleted"`
	// Status is the logger status card, only for a viewer that sees
	// every group and private chat (status.go).
	Status *statusJSON `json:"status,omitempty"`
}

func loadDashboard(ctx context.Context, pr *pageRequest) (any, error) {
	loc, err := pr.viewerLocation(ctx)
	if err != nil {
		return nil, err
	}
	var probe *daemonProbe
	if seesStatus(pr.scope) {
		probe = pr.s.probeDaemon(ctx)
	}
	d, err := logdb.LoadDashboard(ctx, pr.s.ro, pr.scope, pr.now(), loc)
	if err != nil {
		return nil, err
	}
	v := pr.view()
	p := dashboardProps{KPIs: []kpiJSON{}, PerDay: d.PerDay,
		PerDayStart: time.Unix(d.PerDayStart, 0).In(loc).Format(time.DateOnly),
		Busiest:     []groupCountedJSON{}, Deleted: []deletedJSON{}}
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
	if probe != nil {
		if p.Status, err = pr.s.loggerStatus(ctx, pr.scope, probe); err != nil {
			return nil, err
		}
	}
	return p, nil
}

type usernameJSON struct {
	Username    string `json:"username"`
	Active      bool   `json:"active"`
	Kind        string `json:"kind"`
	Collectible bool   `json:"collectible"`
}

func usernamesJSON(us []logdb.Username) []usernameJSON {
	out := []usernameJSON{}
	for _, u := range us {
		out = append(out, usernameJSON{Username: u.Username, Active: u.Active, Kind: u.Kind,
			Collectible: u.Collectible})
	}
	return out
}

type birthdayJSON struct {
	Day   int  `json:"day"`
	Month int  `json:"month"`
	Year  *int `json:"year"`
}

type emojiStatusJSON struct {
	// Custom emoji ids are 64-bit, beyond what a JSON number keeps
	// exactly in a browser: strings.
	CustomEmojiID string  `json:"custom_emoji_id"`
	ExpiresAt     *string `json:"expires_at"`
}

type userFullJSON struct {
	userSummaryJSON
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Bio       string         `json:"bio"`
	Phone     string         `json:"phone"`
	Usernames []usernameJSON `json:"usernames"`

	PhotoFileID         *uint64           `json:"photo_file_id"`
	Birthday            *birthdayJSON     `json:"birthday"`
	PersonalChatID      *int64            `json:"personal_chat_id"`
	PersonalChat        *groupSummaryJSON `json:"personal_chat"`
	PaidMessageStars    int64             `json:"paid_message_star_count"`
	EmojiStatus         *emojiStatusJSON  `json:"emoji_status"`
	AccentColor         int               `json:"accent_color_id"`
	ProfileAccentColor  *int              `json:"profile_accent_color_id"`
	BackgroundEmoji     *string           `json:"background_custom_emoji_id"`
	ProfileBgEmoji      *string           `json:"profile_background_custom_emoji_id"`
	RestrictionReason   string            `json:"restriction_reason"`
	HasSensitiveContent bool              `json:"has_sensitive_content"`
	RestrictsNewChats   bool              `json:"restricts_new_chats"`
	IsContact           bool              `json:"is_contact"`
	IsMutualContact     bool              `json:"is_mutual_contact"`
	OwnNameKnown        bool              `json:"own_name_known"`
	OwnNameCheckedAt    *string           `json:"own_name_checked_at"`
}

// emojiID is a custom emoji id as the API sends it, nil for none.
func emojiID(id int64) *string {
	if id == 0 {
		return nil
	}
	s := strconv.FormatInt(id, 10)
	return &s
}

// userFull is the user page's profile for a full-access viewer.
func (v view) userFull(ctx context.Context, pr *pageRequest, u logdb.UserDetail) (userFullJSON, error) {
	j := userFullJSON{userSummaryJSON: v.user(u.User), FirstName: u.FirstName,
		LastName: u.LastName, Bio: u.Bio, Phone: u.Phone, Usernames: usernamesJSON(u.Usernames),
		PaidMessageStars: u.PaidMessageStars, AccentColor: u.AccentColor,
		BackgroundEmoji: emojiID(u.BackgroundEmoji), ProfileBgEmoji: emojiID(u.ProfileBgEmoji),
		RestrictionReason: u.RestrictionReason, HasSensitiveContent: u.HasSensitiveContent,
		RestrictsNewChats: u.RestrictsNewChats, IsContact: u.Profile.IsContact,
		IsMutualContact: u.IsMutualContact, OwnNameKnown: u.OwnNameKnown,
		OwnNameCheckedAt: isoPtr(u.OwnNameCheckedAt)}
	if u.Photo.ID != 0 {
		j.PhotoFileID = &u.Photo.ID
	}
	if b := u.Birthday; b != nil {
		j.Birthday = &birthdayJSON{Day: b.Day, Month: b.Month}
		if b.Year != 0 {
			j.Birthday.Year = &b.Year
		}
	}
	if u.PersonalChatID != 0 {
		j.PersonalChatID = &u.PersonalChatID
		gs, err := logdb.VisibleGroups(ctx, pr.s.ro, pr.scope, []int64{u.PersonalChatID})
		if err != nil {
			return userFullJSON{}, err
		}
		if len(gs) == 1 {
			g := v.group(gs[0])
			j.PersonalChat = &g
		}
	}
	if id := emojiID(u.EmojiStatus); id != nil {
		j.EmojiStatus = &emojiStatusJSON{CustomEmojiID: *id, ExpiresAt: isoPtr(u.EmojiStatusExpires)}
	}
	if u.ProfileAccentColor >= 0 {
		j.ProfileAccentColor = &u.ProfileAccentColor
	}
	return j, nil
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
	// A username event's state after it, an admin change's status.
	UsernameKind string `json:"username_kind,omitempty"`
	Collectible  bool   `json:"collectible,omitempty"`
	Status       string `json:"status,omitempty"`
	// PhotoID is a photo change's photo, the id of its item in GET
	// /api/users/{id}/photos or /api/groups/{id}/photos; absent
	// elsewhere.
	PhotoID *uint64 `json:"photo_id,omitempty"`
}

type userProps struct {
	Limited     bool            `json:"limited"`
	User        userFullJSON    `json:"user"`
	Groups      []userGroupJSON `json:"groups"`
	History     []historyJSON   `json:"history"`
	HistoryNext *string         `json:"history_next"`
	Activity    []int64         `json:"activity"`
	CanPrivate  bool            `json:"can_private"`
	// CanLookupName: the page offers "Look up real name" (POST
	// /api/users/{id}/lookup-name).
	CanLookupName bool `json:"can_lookup_name"`
	// LookupPending: a "Look up real name" removed the contact and
	// has not added it back yet; since when, and its last error.
	LookupPending      bool    `json:"lookup_pending"`
	LookupPendingSince *string `json:"lookup_pending_since"`
	LookupPendingError *string `json:"lookup_pending_error"`
}

type userLimitedProps struct {
	Limited      bool               `json:"limited"`
	User         userSummaryJSON    `json:"user"`
	SharedGroups []groupSummaryJSON `json:"shared_groups"`
}

func (v view) history(e logdb.HistoryEntry) historyJSON {
	j := historyJSON{Kind: e.Kind, At: iso(e.At), Detail: e.Detail, Action: e.Action,
		UsernameKind: e.UsernameKind, Collectible: e.Collectible, Status: e.Status}
	if e.Kind == "photo" {
		j.Detail = ""
		if e.Photo.ID == 0 {
			j.Removed = true
		} else {
			j.Photo = v.photo(e.Photo)
			j.PhotoID = ptrTo(e.RowID())
		}
	}
	return j
}

// historyCursor is the page's ?before=, the history's cursor; one that
// does not parse starts from the newest changes.
func (pr *pageRequest) historyCursor(kinds func(string) bool) string {
	b := pr.query.Get("before")
	if !kinds(b) {
		return ""
	}
	return b
}

// nextCursor is a history page's next cursor as the API sends it.
func nextCursor(h logdb.HistoryPage) *string {
	if h.Next == "" {
		return nil
	}
	return &h.Next
}

func loadUserPage(ctx context.Context, pr *pageRequest, history bool) (any, error) {
	p, err := logdb.LoadUserPage(ctx, pr.s.ro, pr.scope, pr.pathID(), pr.now(),
		pr.historyCursor(logdb.ValidUserCursor))
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
	uj, err := v.userFull(ctx, pr, p.User)
	if err != nil {
		return nil, err
	}
	up := userProps{
		User: uj, Groups: []userGroupJSON{}, History: []historyJSON{}, Activity: p.Activity,
		CanPrivate: pr.scope.SeesPrivate(),
		// "Look up real name" is for a contact, by a viewer who may,
		// with the daemon's API to do it.
		CanLookupName: uj.IsContact && pr.scope.Can(scope.LookupName) && pr.s.daemon != nil,
		LookupPending: p.User.LookupPending,
	}
	if p.User.LookupPending {
		up.LookupPendingSince = ptrTo(iso(p.User.LookupPendingSince))
		up.LookupPendingError = ptrTo(p.User.LookupPendingError)
	}
	for _, g := range p.Groups {
		up.Groups = append(up.Groups, userGroupJSON{groupSummaryJSON: v.group(g.Group), Msgs: g.Msgs})
	}
	for _, e := range p.History.Entries {
		up.History = append(up.History, v.history(e))
	}
	up.HistoryNext = nextCursor(p.History)
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
	Usernames   []usernameJSON `json:"usernames"`
	PhotoFileID *uint64        `json:"photo_file_id"`
	Description string         `json:"description"`
	// Members is how many people the archive has seen in the group
	// (logdb.Chat.KnownMembers), as on the chat page.
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
	Group       groupFullJSON    `json:"group"`
	Admins      []groupAdminJSON `json:"admins"`
	History     []historyJSON    `json:"history"`
	HistoryNext *string          `json:"history_next"`
	TopPosters  []posterJSON     `json:"top_posters"`
	Activity    []int64          `json:"activity"`
	// Whether a superadmin hid the channel's member list, and whether
	// the viewer may change that (PATCH /api/groups/{id}).
	HideMemberList    bool `json:"hide_member_list"`
	CanHideMemberList bool `json:"can_hide_member_list"`
}

// groupPublicProps is GroupPublicProps: an exposed group's details for a
// viewer whose scope does not see the group (signed out, a member, an
// admin it is hidden from), as public as its chat: the group's own
// summary and counts, none of its admins, history, posters or files.
type groupPublicProps struct {
	Public bool            `json:"public"`
	Group  groupPublicJSON `json:"group"`
}

type groupPublicJSON struct {
	groupSummaryJSON
	Usernames   []usernameJSON `json:"usernames"`
	Description string         `json:"description"`
	Members     int64          `json:"members"`
	MsgCount    uint64         `json:"msg_count"`
	CreatedAt   string         `json:"created_at"`
}

// loadGroupPage is the group route: the group's page for a viewer that
// sees it, else the public details of an exposed group; anything else
// asks a signed-out visitor to sign in and answers 404 to the others,
// whether or not it exists.
func loadGroupPage(ctx context.Context, pr *pageRequest) (any, error) {
	if pr.sess != nil && pr.scope.SeesGroup(pr.pathID()) {
		return loadGroup(ctx, pr)
	}
	c, err := logdb.OpenChat(ctx, pr.s.ro, pr.scope, true, pr.pathID())
	switch {
	case errors.Is(err, logdb.ErrNotFound) || (err == nil && !c.Exposed):
		if pr.sess == nil {
			return nil, errPageSignIn
		}
		return nil, errPageNotFound
	case err != nil:
		return nil, err
	}
	members, err := c.KnownMembers(ctx, pr.s.ro)
	if err != nil {
		return nil, err
	}
	names, err := logdb.GroupUsernames(ctx, pr.s.ro, pr.pathID())
	if err != nil {
		return nil, err
	}
	g := c.GroupInfo
	pr.s.refreshMemberCount(g)
	gs := pr.view().group(g)
	gs.MsgCount = nil
	return groupPublicProps{Public: true, Group: groupPublicJSON{groupSummaryJSON: gs,
		Usernames: usernamesJSON(names), Description: g.Description, Members: members,
		MsgCount: g.MsgCount, CreatedAt: iso(g.CreatedAt)}}, nil
}

func loadGroup(ctx context.Context, pr *pageRequest) (any, error) {
	p, err := logdb.LoadGroupPage(ctx, pr.s.ro, pr.scope, pr.pathID(), pr.now(),
		pr.historyCursor(logdb.ValidGroupCursor))
	if err != nil {
		return nil, notFoundOr(err)
	}
	members, err := logdb.Chat{Group: true, ID: pr.pathID()}.KnownMembers(ctx, pr.s.ro)
	if err != nil {
		return nil, err
	}
	pr.s.refreshMemberCount(p.Group.Group)
	v := pr.view()
	gs := v.group(p.Group.Group)
	gs.MsgCount = nil
	g := p.Group
	out := groupProps{
		Group: groupFullJSON{groupSummaryJSON: gs, Usernames: usernamesJSON(g.Usernames),
			Description: g.Description, Members: members,
			MsgCount: g.MsgCount, AdminCount: g.AdminCount, CreatedAt: iso(g.CreatedAt),
			UpdatedAt: iso(g.UpdatedAt)},
		Admins: []groupAdminJSON{}, History: []historyJSON{}, TopPosters: []posterJSON{},
		Activity: p.Activity, CanHideMemberList: mayHideMemberList(pr.scope, p.Group.Group),
	}
	if out.HideMemberList, err = webdb.MemberListHidden(ctx, pr.s.app, pr.pathID()); err != nil {
		return nil, err
	}
	// The people named here appear in the group, so its viewer sees
	// them, but only in full when it sees users in full.
	for _, a := range p.Admins {
		out.Admins = append(out.Admins, groupAdminJSON{User: v.userOf(p.Users, a.UserID),
			Status: a.Status, CustomTitle: a.CustomTitle, IsAnonymous: a.IsAnonymous,
			Perms: a.Perms, Since: iso(a.Since)})
	}
	if g.Photo.ID != 0 {
		out.Group.PhotoFileID = &g.Photo.ID
	}
	out.History = v.groupHistory(p.History.Entries, p.Users)
	out.HistoryNext = nextCursor(p.History)
	for _, t := range p.TopPosters {
		out.TopPosters = append(out.TopPosters, posterJSON{User: v.userOf(p.Users, t.UserID),
			Msgs: t.Msgs})
	}
	return out, nil
}

type chatGroupJSON struct {
	groupSummaryJSON
	// Members is how many people the archive has seen in the group
	// (logdb.Chat.KnownMembers); Telegram's own count is MemberCount.
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
	At     *int64      `json:"at"`
	Topics []topicJSON `json:"topics"`
	Topic  *int64      `json:"topic"`
}

type topicJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IconColor int64  `json:"icon_color"`
	Closed    bool   `json:"closed"`
	Hidden    bool   `json:"hidden"`
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
	var uid uint64
	if pr.sess != nil {
		uid = pr.sess.UID
	}
	return pr.s.locationOf(ctx, uid)
}

// locationOf is the timezone of the account uid (0 for none): its own
// preference, else the site's default.
func (s *Server) locationOf(ctx context.Context, uid uint64) (*time.Location, error) {
	var prefs settings.Prefs
	if uid != 0 {
		var err error
		if prefs, err = s.viewerPrefs(ctx, uid); err != nil {
			return nil, err
		}
	}
	loc, err := time.LoadLocation(settings.Resolve(prefs, s.defaults).Timezone)
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
	// A topic that is not one shows the whole chat, as an unknown
	// anchor does.
	if t, err := chatTopic(pr.query.Get("topic"), c); err == nil {
		c.Topic = t
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
	k, err := pr.s.loadKinds(ctx, pr.scope, c.ID, w.Messages)
	if err != nil {
		return nil, err
	}
	v := pr.view()
	p := chatProps{chatWindowJSON: v.window(c, w, k), At: at, Topics: []topicJSON{}}
	if c.Topic != 0 {
		t := c.Topic
		p.Topic = &t
	}
	ts, err := c.Topics(ctx, pr.s.ro)
	if err != nil {
		return nil, err
	}
	for _, t := range ts {
		p.Topics = append(p.Topics, topicJSON{ID: t.ID, Name: t.Name, IconColor: t.Color,
			Closed: t.Closed, Hidden: t.Hidden})
	}
	if group {
		pr.s.refreshMemberCount(c.GroupInfo)
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
	messageKindsJSON
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
	k, err := pr.s.loadKinds(ctx, pr.scope, d.ChatID, []logdb.Message{d.Message})
	if err != nil {
		return nil, err
	}
	v := pr.view()
	n := names{d.Users, d.Groups}
	m := d.Message
	// A service message's text is its description for this viewer.
	m.Text = v.text(k, m)
	j := messageFullJSON{
		ID: m.RowID, Scope: scopeName(group), MessageID: m.MessageID, Date: iso(m.Date),
		Sender: v.sender(n, m), ContentType: chatType(m.ContentType), Snippet: snippet(m),
		Forwarded: v.forwardName(n, m), Edited: m.EditDate > 0, DeletedAt: isoPtr(m.DeletedAt),
		Text: m.Text, Edits: v.edits(m), messageExtraJSON: v.messageExtra(n, m),
		messageKindsJSON: v.messageKinds(k, m),
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

// messagesRedirect sends the Drogon message lists, /messages/{scope}
// with ?chat_id= and ?limit=, to the list of that scope filtered by the
// chat, the rows per page as many as the limit. Its ?before= cursor
// has no equivalent there and opens the list's first page.
func messagesRedirect(pr *pageRequest) string {
	list := "/private_messages"
	if pr.params["scope"] == "group" {
		list = "/group_messages"
	}
	q := url.Values{}
	if c := pr.query.Get("chat_id"); isInt(c) {
		f, _ := json.Marshal([]listCond{{Field: "chat_id", Op: "=", Value: c}})
		q.Set("f", string(f))
	}
	if n, err := strconv.Atoi(pr.query.Get("limit")); err == nil && n > 0 {
		q.Set("per", strconv.Itoa(min(n, maxListPer)))
	}
	if len(q) == 0 {
		return list
	}
	return list + "?" + q.Encode()
}

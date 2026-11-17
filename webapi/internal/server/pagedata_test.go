// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The page data of the dashboard, user, group, chat and message pages
// through GET /api/page, and the public chat's document; the fixtures
// are those of chats_test.go.

type tPage[P any] struct {
	Route  string
	Status int
	Props  P
}

// page GETs the page data of path and checks its status and route.
func page[P any](e *chatEnv, path, who string, status int, route string) P {
	e.t.Helper()
	var p tPage[P]
	e.decode("/api/page"+path, e.as(who), status, &p)
	if p.Route != route || p.Status != status {
		e.t.Fatalf("%s as %q: route %q status %d, want %q %d", path, who, p.Route, p.Status,
			route, status)
	}
	return p.Props
}

func notFoundPage(e *chatEnv, path, who string) {
	e.t.Helper()
	page[struct{}](e, path, who, http.StatusNotFound, "not_found")
}

func TestPageDashboard(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	type dash struct {
		KPIs []struct {
			Key          string
			Total, Today int64
		}
		PerDay  []int64 `json:"per_day"`
		Busiest []struct {
			ID       int64
			MsgCount int64 `json:"msg_count"`
		}
		Deleted []struct {
			ID        uint64
			Chat      struct{ Title string }
			Sender    tUser
			Snippet   string
			DeletedAt string `json:"deleted_at"`
		}
	}
	d := page[dash](e, "/", "admin", 200, "dashboard")
	keys := []string{}
	totals := map[string]int64{}
	for _, k := range d.KPIs {
		keys = append(keys, k.Key)
		totals[k.Key] = k.Total
	}
	if !slices.Equal(keys, []string{"users", "groups", "group_messages", "private_messages", "files"}) {
		t.Errorf("kpis %v", keys)
	}
	if totals["users"] != 9 || totals["groups"] != 8 || totals["group_messages"] != 16+130+4 ||
		totals["private_messages"] != 6+8 || totals["files"] != 14 {
		t.Errorf("totals %v", totals)
	}
	if len(d.PerDay) != 30 || d.PerDay[29] == 0 {
		t.Errorf("per_day %v", d.PerDay)
	}
	if len(d.Busiest) != 6 || d.Busiest[0].ID != pagingLab || d.Busiest[0].MsgCount != 130 {
		t.Errorf("busiest %+v", d.Busiest)
	}
	if len(d.Deleted) != 2 || d.Deleted[0].ID != 6 || d.Deleted[1].ID != 1020 ||
		d.Deleted[1].Chat.Title != "Paging Lab" || d.Deleted[1].Snippet != "paging message 20" {
		t.Errorf("deleted %+v", d.Deleted)
	}

	// A member sees counts over its groups only, none so far, and no
	// private messages or files.
	d = page[dash](e, "/", "researcher", 200, "dashboard")
	keys = keys[:0]
	for _, k := range d.KPIs {
		keys = append(keys, k.Key)
		if k.Total != 0 || k.Today != 0 {
			t.Errorf("member kpi %+v", k)
		}
	}
	if !slices.Equal(keys, []string{"users", "groups", "group_messages"}) || len(d.Busiest) != 0 ||
		len(d.Deleted) != 0 || slices.Max(d.PerDay) != 0 {
		t.Errorf("member dashboard %+v", d)
	}
}

type tUserProps struct {
	Limited bool
	User    struct {
		tUser
		FirstName string `json:"first_name"`
		Usernames []struct {
			Username string
			Active   bool
		}
	}
	Groups []struct {
		ID   int64
		Msgs int64
	}
	History []struct {
		Kind, At, Detail, Action string
		Photo                    *string
		Removed                  bool
	}
	Activity   []int64
	CanPrivate bool `json:"can_private"`
}

func TestPageUser(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	if _, err := e.roExec("INSERT INTO telegram_user_hist_name (user_id, first_name, last_name, "+
		"created_at) VALUES (?, 'Dewi', 'K', '2026-11-01 10:00:00')", dewi); err != nil {
		t.Fatal(err)
	}
	if _, err := e.roExec("INSERT INTO telegram_user_hist_profile_photo (user_id, file_id, "+
		"created_at) VALUES (?, NULL, '2026-11-02 10:00:00')", dewi); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"user", "user_history"} {
		path := fmt.Sprintf("/users/%d", dewi)
		if route == "user_history" {
			path += "/history"
		}
		for _, who := range []string{"admin", "gw_admin"} {
			p := page[tUserProps](e, path, who, 200, route)
			u := p.User
			if p.Limited || u.Name != "Dewi Kusuma" || u.FirstName != "Dewi" || u.Bio == nil ||
				*u.Bio != "Kernel hacker. Coffee first. She/her." || u.MsgCount == nil ||
				len(u.Usernames) != 1 || !u.Usernames[0].Active || u.Photo == nil {
				t.Errorf("%s %s: user %+v", who, route, u)
			}
			// Dewi wrote in Paging Lab (45, the day before), GNU/Weeb
			// (2), Linux Kernel Indonesia and Open Lab (1 each), and
			// three private messages.
			if len(p.Groups) != 4 || p.Groups[0].ID != pagingLab || p.Groups[0].Msgs != 45 {
				t.Errorf("%s %s: groups %+v", who, route, p.Groups)
			}
			if len(p.History) != 2 || p.History[0].Kind != "photo" || !p.History[0].Removed ||
				p.History[1].Kind != "name" || p.History[1].Detail != "Dewi K" ||
				p.History[1].At != "2026-11-01T10:00:00Z" {
				t.Errorf("%s %s: history %+v", who, route, p.History)
			}
			if len(p.Activity) != 30 || p.Activity[28] != 45 || p.Activity[29] != 2+1+1+3 {
				t.Errorf("%s %s: activity %v", who, route, p.Activity)
			}
			if !p.CanPrivate {
				t.Errorf("%s %s: can_private false", who, route)
			}
		}
		// Hidden from a member, like a missing user.
		notFoundPage(e, path, "researcher")
		notFoundPage(e, strings.Replace(path, fmt.Sprint(dewi), "9299999999", 1), "admin")
	}
}

type tGroupProps struct {
	Group struct {
		ID          int64
		Title       string
		Username    string
		Description string
		MsgCount    int64 `json:"msg_count"`
		AdminCount  int64 `json:"admin_count"`
		Photo       *string
		Exposed     *bool
	}
	Admins []struct {
		User        tUser
		Status      string
		CustomTitle string `json:"custom_title"`
		Perms       []string
		Since       string
	}
	History []struct {
		Kind, Detail, Action string
		User                 *tUser
	}
	TopPosters []struct {
		User tUser
		Msgs int64
	} `json:"top_posters"`
	Activity []int64
}

func TestPageGroup(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	if _, err := e.roExec("INSERT INTO telegram_group_admin_hist (group_id, user_id, action, "+
		"status, custom_title, created_at) VALUES (?, ?, 'added', 'administrator', '', "+
		"'2026-11-03 10:00:00')", gnuWeeb, dewi); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"group", "group_admins", "group_history"} {
		path := fmt.Sprintf("/groups/%d", gnuWeeb)
		if route != "group" {
			path += strings.TrimPrefix(route, "group")
			path = strings.Replace(path, "_", "/", 1)
		}
		p := page[tGroupProps](e, path, "admin", 200, route)
		g := p.Group
		if g.Title != "GNU/Weeb" || g.Username != "gnuweeb" || g.MsgCount != 8 || g.AdminCount != 2 ||
			g.Photo == nil || g.Exposed == nil || !*g.Exposed {
			t.Errorf("%s: group %+v", route, g)
		}
		if len(p.Admins) != 2 || p.Admins[0].Status != "creator" || p.Admins[0].CustomTitle != "founder" ||
			!slices.Equal(p.Admins[0].Perms, []string{"manage chat", "delete messages",
				"restrict members", "pin messages"}) || p.Admins[0].User.Name != "Sari Wulandari" {
			t.Errorf("%s: admins %+v", route, p.Admins)
		}
		if len(p.History) != 1 || p.History[0].Kind != "admin" || p.History[0].Action != "added" ||
			p.History[0].User == nil || p.History[0].User.ID != dewi {
			t.Errorf("%s: history %+v", route, p.History)
		}
		if len(p.TopPosters) == 0 || p.TopPosters[0].User.ID != 9299000003 || p.TopPosters[0].Msgs != 3 {
			t.Errorf("%s: top posters %+v", route, p.TopPosters)
		}
		if len(p.Activity) != 30 || p.Activity[29] != 8 {
			t.Errorf("%s: activity %v", route, p.Activity)
		}
		notFoundPage(e, path, "researcher")
	}
	notFoundPage(e, "/groups/-1009999999999", "admin")
	// A group page is not public, even for an exposed group.
	code, _, _ := e.get(fmt.Sprintf("/api/page/groups/%d", gnuWeeb), "")
	if code != http.StatusUnauthorized {
		t.Errorf("signed out: %d", code)
	}
}

type tChatProps struct {
	Chat struct {
		Kind  string
		Group *struct {
			ID       int64
			Title    string
			MsgCount int64 `json:"msg_count"`
			Members  *int64
		}
		User *struct {
			ID   int64
			Name string
		}
		Count *int64
	}
	Messages []tMsg
	HasOlder bool `json:"has_older"`
	HasNewer bool `json:"has_newer"`
	At       *int64
}

func TestPageChat(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	gpath := fmt.Sprintf("/groups/%d/chat", pagingLab)
	p := page[tChatProps](e, gpath, "admin", 200, "group_chat")
	if p.Chat.Kind != "group" || p.Chat.Group == nil || p.Chat.Group.Title != "Paging Lab" ||
		p.Chat.Group.MsgCount != 130 || p.Chat.Group.Members == nil || p.At != nil ||
		len(p.Messages) != 59 || !p.HasOlder || p.HasNewer {
		t.Errorf("group chat %+v", p.Chat)
	}
	p = page[tChatProps](e, fmt.Sprintf("%s?at=%d", gpath, labMsg(50)), "admin", 200, "group_chat")
	if p.At == nil || *p.At != labMsg(50) || byRow(p.Messages, 1050) == nil || !p.HasNewer {
		t.Errorf("at 50: at %v, %d messages", p.At, len(p.Messages))
	}
	notFoundPage(e, gpath, "researcher")

	upath := fmt.Sprintf("/users/%d/chat", rina)
	p = page[tChatProps](e, upath, "gw_admin", 200, "user_chat")
	if p.Chat.Kind != "user" || p.Chat.User == nil || p.Chat.User.Name != "Rina Hartono" ||
		p.Chat.Count == nil || *p.Chat.Count != 8 || len(p.Messages) != 7 {
		t.Errorf("user chat %+v", p.Chat)
	}
	p = page[tChatProps](e, upath+"?date=2026-11-17", "admin", 200, "user_chat")
	if p.At == nil || *p.At != 900001<<20 {
		t.Errorf("date: at %v", p.At)
	}
	p = page[tChatProps](e, upath+"?date=2027-01-01", "admin", 200, "user_chat")
	if p.At != nil || len(p.Messages) != 7 {
		t.Errorf("date past the end: at %v", p.At)
	}
	notFoundPage(e, upath, "researcher")
	notFoundPage(e, "/users/9299999999/chat", "admin")
	if code, _, _ := e.get("/api/page"+upath, ""); code != http.StatusUnauthorized {
		t.Errorf("private chat signed out: %d", code)
	}
}

// TestPagePublicChat: the chat of an exposed group is public, and its
// document cacheable; any other chat sends a signed-out visitor to
// sign in, whether it exists or not.
func TestPagePublicChat(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	path := fmt.Sprintf("/groups/%d/chat", openLab)
	p := page[tChatProps](e, path, "", 200, "group_chat")
	if p.Chat.Group == nil || p.Chat.Group.Title != "Open Lab" || len(p.Messages) != 4 {
		t.Errorf("public chat %+v", p)
	}
	code, h, body := e.get(path, "")
	if code != http.StatusOK || h.Get("Cache-Control") != "public, max-age=60" ||
		!strings.Contains(string(body), "Open Lab") {
		t.Errorf("public document: %d %q", code, h.Get("Cache-Control"))
	}
	var doc struct {
		Viewer struct {
			SignedIn bool `json:"signed_in"`
		}
	}
	const tag = `id="__tgd">`
	if i := strings.Index(string(body), tag); i < 0 {
		t.Error("no page data in the document")
	} else if err := json.NewDecoder(strings.NewReader(string(body[i+len(tag):]))).Decode(&doc); err != nil ||
		doc.Viewer.SignedIn {
		t.Errorf("page data: %v %+v", err, doc)
	}

	for _, id := range []int64{pagingLab, -1009999999999} {
		path := fmt.Sprintf("/groups/%d/chat", id)
		if code, _, _ := e.get("/api/page"+path, ""); code != http.StatusUnauthorized {
			t.Errorf("%d: page data %d, want 401", id, code)
		}
		code, h, _ := e.get(path, "")
		if code != http.StatusFound || h.Get("Location") != "/login?next="+
			strings.ReplaceAll(path, "/", "%2F") {
			t.Errorf("%d: document %d %q", id, code, h.Get("Location"))
		}
	}
}

type tMessageProps struct {
	Message struct {
		ID        uint64
		Scope     string
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID    int64
			Title string
			Kind  string
		}
		Sender  *tUser
		Snippet string
		Text    string
		Edited  bool
		ReplyTo *struct {
			Scope      string
			ID         uint64
			SenderName string `json:"sender_name"`
			Snippet    string
		} `json:"reply_to"`
		Edits []struct{ At, Text string }
		Media *struct {
			FileID uint64 `json:"file_id"`
			Name   string
			Stored bool
		}
	}
}

func TestPageMessage(t *testing.T) {
	t.Parallel()
	e := newChatEnv(t, false)
	m := page[tMessageProps](e, "/messages/group/1100", "admin", 200, "message").Message
	if m.ID != 1100 || m.Chat.Title != "Paging Lab" || m.Chat.Kind != "group" ||
		m.ReplyTo == nil || m.ReplyTo.ID != 1099 || m.ReplyTo.Snippet != "paging message 99" ||
		m.Sender == nil {
		t.Errorf("message %+v", m)
	}
	m = page[tMessageProps](e, "/messages/group/1010", "admin", 200, "message").Message
	if !m.Edited || len(m.Edits) != 2 {
		t.Errorf("edited %+v", m)
	}
	m = page[tMessageProps](e, "/messages/group/1121", "admin", 200, "message").Message
	if m.Media == nil || m.Media.FileID != 870006 || m.Media.Stored || m.Media.Name != "long-talk.mp4" {
		t.Errorf("media %+v", m.Media)
	}
	m = page[tMessageProps](e, "/messages/private/1008", "gw_admin", 200, "message").Message
	if m.Sender != nil || m.Chat.Kind != "user" || m.Chat.Title != "Rina Hartono" ||
		m.ReplyTo == nil || m.ReplyTo.ID != 1001 || m.ReplyTo.Scope != "private" {
		t.Errorf("private %+v", m)
	}
	notFoundPage(e, "/messages/group/1100", "researcher")
	notFoundPage(e, "/messages/private/1008", "researcher")
	notFoundPage(e, "/messages/group/999999", "admin")
	// A row id of the other table does not open there.
	notFoundPage(e, "/messages/private/1100", "admin")
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/testdb"
)

// The user and group pages' profile and history, on the logger
// fixtures "sample" and "people" (internal/testdb), with the accounts
// of chatEnv (superadmin "admin", admin "gw_admin", member
// "researcher").

const (
	persona    = 9299200001 // every profile field, 160 history entries
	kontak     = 9299200002 // own name never seen, lookup pending
	historyLab = -1001000200001
)

func newPeopleEnv(t *testing.T, withDaemon ...bool) *chatEnv {
	t.Helper()
	ro := testdb.New(t, testdb.Logger, "sample", "people").DB
	app := testdb.New(t, testdb.Web, "accounts").DB
	signer := auth.NewSigner(testKey)
	e := &chatEnv{t: t, ro: ro, cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		tokens: auth.NewFileTokens(testKey)}
	var dc *daemon.Client
	if len(withDaemon) > 0 && withDaemon[0] {
		// Never called: only whether the daemon API is on matters.
		var err error
		if dc, err = daemon.New("http://127.0.0.1:9", testKey, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	e.h = New(Deps{
		Daemon:  dc,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      ro,
		App:     app,
		Signer:  signer,
		Cookies: e.cookies,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Tokens:  e.tokens,
		clock:   func() time.Time { return chatNow },
	})
	return e
}

type tHist struct {
	Kind, At, Detail, Action string
	UsernameKind             string `json:"username_kind"`
	Collectible              bool
	Status                   string
	Photo                    *string
	Removed                  bool
	User                     *tUser
}

type tHistPage struct {
	Entries []tHist
	Next    *string
}

func histPath(kind string, id int64, before string, limit int) string {
	q := url.Values{}
	if before != "" {
		q.Set("before", before)
	}
	if limit != 0 {
		q.Set("limit", fmt.Sprint(limit))
	}
	return fmt.Sprintf("/api/%s/%d/history?%s", kind, id, q.Encode())
}

func histSummary(es []tHist) string {
	var out []string
	for _, h := range es {
		s := h.Kind + ":" + h.Detail
		if h.Action != "" {
			s += ":" + h.Action
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

// walk reads a whole timeline, limit entries a page, and returns every
// entry and the number of pages.
func walk(e *chatEnv, kind string, id int64, limit int) ([]tHist, int) {
	e.t.Helper()
	var all []tHist
	before, pages := "", 0
	for {
		var p tHistPage
		e.decode(histPath(kind, id, before, limit), e.as("admin"), 200, &p)
		pages++
		all = append(all, p.Entries...)
		if p.Next == nil {
			return all, pages
		}
		if pages > 1000 {
			e.t.Fatal("no end")
		}
		before = *p.Next
	}
}

func TestUserHistoryPaging(t *testing.T) {
	t.Parallel()
	e := newPeopleEnv(t)
	tie := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	// The six entries of the same second come in kind order, then
	// newest row first, and a page boundary between any two of them
	// loses or repeats none.
	var p tHistPage
	e.decode(histPath("users", persona, "", 2), e.as("admin"), 200, &p)
	if histSummary(p.Entries) != "bio:Every field, for tests.,name:Tie B" || p.Next == nil ||
		*p.Next != fmt.Sprintf("%d.name.502", tie.Unix()) ||
		p.Entries[0].At != "2026-06-01T12:00:00Z" {
		t.Fatalf("first page %+v %v", p.Entries, p.Next)
	}
	e.decode(histPath("users", persona, *p.Next, 2), e.as("admin"), 200, &p)
	if histSummary(p.Entries) != "name:Tie A,phone:+62 800 0000 0001" {
		t.Fatalf("second page %+v", p.Entries)
	}
	e.decode(histPath("users", persona, *p.Next, 2), e.as("admin"), 200, &p)
	if len(p.Entries) != 2 || p.Entries[0].Kind != "photo" || p.Entries[0].Photo == nil ||
		*p.Entries[0].Photo != "/files/"+e.tokens.Encrypt(880007) || p.Entries[0].Removed ||
		histSummary(p.Entries[1:]) != "username:persona:added" ||
		p.Entries[1].UsernameKind != "active" || p.Entries[1].Collectible {
		t.Fatalf("third page %+v", p.Entries)
	}
	// Every username event, with its state after it.
	e.decode(histPath("users", persona, *p.Next, 4), e.as("admin"), 200, &p)
	if histSummary(p.Entries) != "username:persona_nft:reordered,username:persona_old:kind_changed,"+
		"username:persona_nft:collectible_changed,username:persona_old:removed" ||
		p.Entries[1].UsernameKind != "disabled" || !p.Entries[2].Collectible ||
		p.Entries[3].UsernameKind != "" {
		t.Fatalf("username events %+v", p.Entries)
	}
	e.decode(histPath("users", persona, *p.Next, 0), e.as("gw_admin"), 200, &p)
	if len(p.Entries) != 100 || p.Entries[0].Detail != "Name 150" || p.Entries[99].Detail != "Name 51" ||
		p.Next == nil {
		t.Fatalf("names page %d %v", len(p.Entries), p.Next)
	}
	e.decode(histPath("users", persona, *p.Next, 0), e.as("admin"), 200, &p)
	if len(p.Entries) != 50 || p.Entries[49].Detail != "Name 1" || p.Next != nil {
		t.Fatalf("last page %d %v", len(p.Entries), p.Next)
	}

	// Whatever the page size, the whole timeline comes back once.
	for _, limit := range []int{1, 7, 100, 159, 160, 500} {
		all, pages := walk(e, "users", persona, limit)
		seen := map[string]bool{}
		for _, h := range all {
			k := h.Kind + h.At + h.Detail + h.Action
			if seen[k] {
				t.Errorf("limit %d: %s twice", limit, k)
			}
			seen[k] = true
		}
		want := (160 + limit - 1) / limit
		if len(all) != 160 || pages != want {
			t.Errorf("limit %d: %d entries in %d pages, want 160 in %d", limit, len(all), pages, want)
		}
	}

	// The page carries the first page and the next cursor; the history
	// page takes ?before= as well, and ignores one that does not
	// parse.
	type props struct {
		History     []tHist
		HistoryNext *string `json:"history_next"`
	}
	up := page[props](e, fmt.Sprintf("/users/%d", persona), "admin", 200, "user")
	if len(up.History) != 100 || up.HistoryNext == nil || up.History[0].Kind != "bio" {
		t.Fatalf("page history %d %v", len(up.History), up.HistoryNext)
	}
	next := *up.HistoryNext
	up = page[props](e, fmt.Sprintf("/users/%d/history?before=%s", persona, next), "admin", 200,
		"user_history")
	if len(up.History) != 60 || up.HistoryNext != nil || up.History[59].Detail != "Name 1" {
		t.Errorf("page history after %s: %d %v", next, len(up.History), up.HistoryNext)
	}
	up = page[props](e, fmt.Sprintf("/users/%d/history?before=nonsense", persona), "admin", 200,
		"user_history")
	if len(up.History) != 100 || up.History[0].Kind != "bio" {
		t.Errorf("bad page cursor: %d", len(up.History))
	}
	// A user with no history has an empty one and no cursor.
	up = page[props](e, fmt.Sprintf("/users/%d", kontak), "admin", 200, "user")
	if len(up.History) != 0 || up.HistoryNext != nil {
		t.Errorf("kontak history %+v", up)
	}

	// Refusals: bad cursors (a group's kind too) and limits; a member
	// sees no one's history, a hidden or unknown user is 404, and a
	// signed-out request 401.
	for _, path := range []string{
		histPath("users", persona, "1.title.1", 0),
		histPath("users", persona, "x.name.1", 0),
		histPath("users", persona, "1.name", 0),
		histPath("users", persona, "-1.name.1", 0),
		histPath("users", persona, "", -1),
		histPath("users", persona, "", 501),
		fmt.Sprintf("/api/users/%d/history?limit=two", persona),
	} {
		e.decode(path, e.as("admin"), 400, nil)
	}
	e.decode(histPath("users", persona, "", 0), e.as("researcher"), 404, nil)
	e.decode(histPath("users", 9299999999, "", 0), e.as("admin"), 404, nil)
	e.decode("/api/users/x/history", e.as("admin"), 404, nil)
	if code, _, _ := e.get(histPath("users", persona, "", 0), ""); code != http.StatusUnauthorized {
		t.Errorf("signed out: %d", code)
	}
}

func TestGroupHistoryPaging(t *testing.T) {
	t.Parallel()
	e := newPeopleEnv(t)

	var p tHistPage
	e.decode(histPath("groups", historyLab, "", 3), e.as("gw_admin"), 200, &p)
	if histSummary(p.Entries) != "admin:keeper:added,description:A group with a long history.,"+
		"username:historylab:added" || p.Entries[0].Status != "administrator" ||
		p.Entries[0].User == nil || p.Entries[0].User.ID != 9299000003 ||
		p.Entries[0].User.Name != "Sari Wulandari" || p.Entries[2].UsernameKind != "active" {
		t.Fatalf("first page %+v", p.Entries)
	}
	e.decode(histPath("groups", historyLab, *p.Next, 3), e.as("gw_admin"), 200, &p)
	if len(p.Entries) != 3 || p.Entries[0].Kind != "photo" || !p.Entries[0].Removed ||
		p.Entries[1].Kind != "admin" || p.Entries[1].Action != "updated" ||
		p.Entries[1].Status != "creator" || p.Entries[1].Detail != "creator" ||
		p.Entries[2].Detail != "Title 120" {
		t.Fatalf("second page %+v", p.Entries)
	}
	for _, limit := range []int{1, 2, 100, 125, 500} {
		all, pages := walk(e, "groups", historyLab, limit)
		if want := (125 + limit - 1) / limit; len(all) != 125 || pages != want ||
			all[124].Detail != "Title 1" {
			t.Errorf("limit %d: %d entries in %d pages, want 125 in %d", limit, len(all), pages, want)
		}
	}

	type props struct {
		History     []tHist
		HistoryNext *string `json:"history_next"`
	}
	for _, path := range []string{"", "/history", "/admins"} {
		gp := page[props](e, fmt.Sprintf("/groups/%d%s", historyLab, path), "admin", 200,
			"group"+strings.ReplaceAll(path, "/", "_"))
		if len(gp.History) != 100 || gp.HistoryNext == nil || gp.History[0].Kind != "admin" {
			t.Errorf("%s: page history %d %v", path, len(gp.History), gp.HistoryNext)
		}
	}
	gp := page[props](e, fmt.Sprintf("/groups/%d/history?before=1.name.1", historyLab), "admin",
		200, "group_history")
	if len(gp.History) != 100 {
		t.Errorf("a user's cursor on a group page: %d", len(gp.History))
	}

	e.decode(histPath("groups", historyLab, "1.name.1", 0), e.as("admin"), 400, nil)
	e.decode(histPath("groups", historyLab, "", 0), e.as("researcher"), 404, nil)
	e.decode(histPath("groups", -1009999999999, "", 0), e.as("admin"), 404, nil)
	if code, _, _ := e.get(histPath("groups", historyLab, "", 0), ""); code != http.StatusUnauthorized {
		t.Errorf("signed out: %d", code)
	}
}

func TestUserProfile(t *testing.T) {
	t.Parallel()
	e := newPeopleEnv(t, true)
	type props struct {
		User          map[string]any
		CanLookupName bool    `json:"can_lookup_name"`
		LookupPending bool    `json:"lookup_pending"`
		Since         *string `json:"lookup_pending_since"`
		Error         *string `json:"lookup_pending_error"`
	}
	path := fmt.Sprintf("/users/%d", persona)
	for _, who := range []string{"admin", "gw_admin"} {
		p := page[props](e, path, who, 200, "user")
		u := p.User
		delete(u, "personal_chat")
		want := map[string]any{
			"id": persona, "name": "Persona Lengkap", "username": "persona", "type": "regular",
			"photo": "/files/" + e.tokens.Encrypt(880007), "contact_name": "Persona (fixture)",
			"msg_count": 0, "is_verified": true, "is_premium": true, "is_scam": false,
			"is_fake": false, "is_support": false, "language": "id",
			"created_at": "2025-12-01T08:00:00Z", "updated_at": "2026-11-10T10:00:00Z",
			"first_name": "Persona", "last_name": "Lengkap", "bio": "Every field, for tests.",
			"phone": "+62 800 0000 0001",
			"usernames": []any{
				map[string]any{"username": "persona", "active": true, "kind": "active", "collectible": false},
				map[string]any{"username": "persona_nft", "active": true, "kind": "active", "collectible": true},
				map[string]any{"username": "persona_old", "active": false, "kind": "disabled", "collectible": false},
			},
			"photo_file_id":           880007,
			"birthday":                map[string]any{"day": 17, "month": 8, "year": 1995},
			"personal_chat_id":        gnuWeeb,
			"paid_message_star_count": 250,
			"emoji_status": map[string]any{"custom_emoji_id": "5373141891321699086",
				"expires_at": "2027-01-01T00:00:00Z"},
			"accent_color_id":                    5,
			"profile_accent_color_id":            3,
			"background_custom_emoji_id":         "5368324170671202286",
			"profile_background_custom_emoji_id": "5370870893004203704",
			"restriction_reason":                 "Restricted on some platforms (fixture)",
			"has_sensitive_content":              true,
			"restricts_new_chats":                true,
			"is_contact":                         true,
			"is_mutual_contact":                  true,
			"own_name_known":                     true,
			"own_name_checked_at":                "2026-10-01T09:00:00Z",
		}
		if js(u) != js(want) {
			t.Errorf("%s: profile\n got %s\nwant %s", who, js(u), js(want))
		}
		pc := page[struct {
			User struct {
				PersonalChat *struct{ ID int64 } `json:"personal_chat"`
			}
		}](e, path, who, 200, "user")
		if pc.User.PersonalChat == nil || pc.User.PersonalChat.ID != gnuWeeb {
			t.Errorf("%s: personal chat %+v", who, pc.User.PersonalChat)
		}
		// Only a superadmin may look up a name.
		if p.CanLookupName != (who == "admin") || p.LookupPending || p.Since != nil || p.Error != nil {
			t.Errorf("%s: lookup %+v", who, p)
		}
	}

	// A contact whose own name was never seen, with a lookup pending;
	// nothing in telegram_user_extra_info reads as none.
	p := page[props](e, fmt.Sprintf("/users/%d", kontak), "admin", 200, "user")
	u := p.User
	for k, v := range map[string]any{"own_name_known": false, "own_name_checked_at": nil,
		"birthday": map[string]any{"day": 3, "month": 12, "year": nil}, "photo_file_id": nil,
		"personal_chat_id": nil, "personal_chat": nil, "emoji_status": nil,
		"profile_accent_color_id": nil, "background_custom_emoji_id": nil,
		"profile_background_custom_emoji_id": nil, "paid_message_star_count": 0,
		"accent_color_id": 0, "restriction_reason": "", "is_contact": true,
		"is_mutual_contact": false, "has_sensitive_content": false, "usernames": []any{}} {
		if js(u[k]) != js(v) {
			t.Errorf("kontak %s: %s, want %s", k, js(u[k]), js(v))
		}
	}
	if !p.CanLookupName || !p.LookupPending || p.Since == nil || *p.Since != "2026-11-17T09:59:00Z" ||
		p.Error == nil || *p.Error != "FLOOD_WAIT_30 <retry>" {
		t.Errorf("kontak lookup %+v", p)
	}
	// Without the daemon's API there is no lookup to offer.
	p = page[props](newPeopleEnv(t), fmt.Sprintf("/users/%d", kontak), "admin", 200, "user")
	if p.CanLookupName || !p.LookupPending {
		t.Errorf("no daemon: %+v", p)
	}
	// A user who is no contact: nothing to look up.
	p = page[props](e, fmt.Sprintf("/users/%d", 9299000004), "admin", 200, "user")
	if p.CanLookupName || p.User["is_contact"] != false {
		t.Errorf("no contact: %+v", p)
	}
	// A member sees no profile (and, without grants, nobody).
	notFoundPage(e, path, "researcher")
}

func TestGroupProfile(t *testing.T) {
	t.Parallel()
	e := newPeopleEnv(t)
	type props struct {
		Group map[string]any
	}
	for _, who := range []string{"admin", "gw_admin"} {
		g := page[props](e, fmt.Sprintf("/groups/%d", historyLab), who, 200, "group").Group
		want := []any{
			map[string]any{"username": "historylab", "active": true, "kind": "active", "collectible": false},
			map[string]any{"username": "historylab_nft", "active": true, "kind": "active", "collectible": true},
			map[string]any{"username": "historylab_old", "active": false, "kind": "disabled", "collectible": false},
		}
		if js(g["usernames"]) != js(want) || g["username"] != "historylab" ||
			js(g["photo_file_id"]) != "880008" || g["photo"] != "/files/"+e.tokens.Encrypt(880008) {
			t.Errorf("%s: group %s", who, js(g))
		}
	}
	if _, err := e.roExec("UPDATE telegram_groups SET photo_file_id = NULL WHERE id = ?",
		historyLab); err != nil {
		t.Fatal(err)
	}
	g := page[props](e, fmt.Sprintf("/groups/%d", historyLab), "admin", 200, "group").Group
	if g["photo_file_id"] != nil || g["photo"] != nil {
		t.Errorf("no photo: %v %v", g["photo_file_id"], g["photo"])
	}
	notFoundPage(e, fmt.Sprintf("/groups/%d", historyLab), "researcher")
}

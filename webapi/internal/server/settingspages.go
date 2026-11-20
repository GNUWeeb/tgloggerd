// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/webdb"
)

// The settings pages' data: the account page (settings), the
// preferences page (settings_preferences), and GET
// /api/account/preferences, which answers the latter's props.

// settingsProps are the account page's (SettingsProps).
type settingsProps struct {
	Account settingsAccount `json:"account"`
	// Notices are the security notices the account is sent; they
	// cannot be turned off.
	Notices []string `json:"notices"`
}

type settingsAccount struct {
	Username      string  `json:"username"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	Email         *string `json:"email"`
	EmailVerified bool    `json:"email_verified"`
}

// noticeKeys are the security notices, in the order shown.
var noticeKeys = []string{
	"notice.password", "notice.email", "notice.new_device", "notice.disabled", "notice.role",
}

func loadSettings(ctx context.Context, pr *pageRequest) (any, error) {
	s := pr.s
	a, err := webdb.GetAccount(ctx, s.app, pr.sess.UID)
	if err != nil {
		return nil, err
	}
	lang := s.textLang(ctx, a.ID)
	p := settingsProps{Account: settingsAccount{
		Username: a.Username, Name: a.DisplayName, Role: a.Role,
	}}
	if p.Account.Name == "" {
		p.Account.Name = a.Username
	}
	if a.Email != "" {
		e := a.Email
		p.Account.Email = &e
		p.Account.EmailVerified = a.EmailVerified
	}
	for _, k := range noticeKeys {
		p.Notices = append(p.Notices, i18n.T(lang, k))
	}
	return p, nil
}

// preferencesProps are the preferences page's (PreferencesProps).
type preferencesProps struct {
	Prefs     settings.Prefs `json:"prefs"`
	Timezones []string       `json:"timezones"`
	// LandingOptions are [path, label] pairs.
	LandingOptions [][2]string         `json:"landing_options"`
	Pinned         []pinJSON           `json:"pinned"`
	SavedSearches  []webdb.SavedSearch `json:"saved_searches"`
}

type pinJSON struct {
	ID    int64   `json:"id"`
	Title string  `json:"title"`
	Photo *string `json:"photo"`
}

// timezones are the zones the preferences page offers: Indonesia's and
// its neighbours' first, then one for each other offset most accounts
// are in. Any other IANA zone is accepted through the API, and the
// account's own is added when it is not one of these.
var timezones = []string{
	"Asia/Jakarta", "Asia/Pontianak", "Asia/Makassar", "Asia/Jayapura",
	"Asia/Singapore", "Asia/Kuala_Lumpur", "Asia/Bangkok", "Asia/Manila",
	"Asia/Ho_Chi_Minh", "Asia/Hong_Kong", "Asia/Shanghai", "Asia/Taipei",
	"Asia/Tokyo", "Asia/Seoul", "Asia/Kolkata", "Asia/Dubai", "Asia/Karachi",
	"Australia/Perth", "Australia/Sydney", "Pacific/Auckland",
	"Europe/London", "Europe/Berlin", "Europe/Paris", "Europe/Amsterdam",
	"Europe/Moscow", "Europe/Istanbul", "Africa/Cairo", "Africa/Lagos",
	"America/New_York", "America/Chicago", "America/Denver",
	"America/Los_Angeles", "America/Sao_Paulo", "UTC",
}

// landingPages are the menu's pages that can be a landing page, by
// menu key, with the path each opens.
var landingPages = []struct{ key, path string }{
	{"dashboard", "/"}, {"users", "/users"}, {"groups", "/groups"},
	{"group_messages", "/group_messages"}, {"private_messages", "/private_messages"},
	{"files", "/files"}, {"platform_fwd", "/platform-fwd"}, {"join_chat", "/join-chat"},
	{"mcp", "/mcp-admin"}, {"accounts", "/settings/accounts"},
	{"federations", "/settings/federations"}, {"settings", "/settings"},
}

// preferences builds the preferences page's props for the account.
func (s *Server) preferences(ctx context.Context, sess *auth.Session, sc scope.Scope) (preferencesProps, error) {
	var p preferencesProps
	var err error
	if p.Prefs, err = s.viewerPrefs(ctx, sess.UID); err != nil {
		return p, err
	}
	lang := settings.Resolve(p.Prefs, s.defaults).Lang
	p.Timezones = timezones
	if tz := p.Prefs.Timezone; tz != nil && !slices.Contains(timezones, *tz) {
		p.Timezones = append(slices.Clone(timezones), *tz)
	}
	nav := sc.Nav()
	for _, lp := range landingPages {
		if slices.Contains(nav, lp.key) {
			p.LandingOptions = append(p.LandingOptions, [2]string{lp.path, i18n.T(lang, "page."+lp.key)})
		}
	}
	pins, err := s.viewerPins(ctx, sess.UID, sc)
	if err != nil {
		return p, err
	}
	p.Pinned = []pinJSON{}
	for _, g := range pins {
		j := pinJSON{ID: g.ID, Title: g.Title}
		if g.PhotoID != 0 && s.tokens != nil {
			u := "/files/" + s.tokens.Encrypt(g.PhotoID)
			j.Photo = &u
		}
		p.Pinned = append(p.Pinned, j)
		p.LandingOptions = append(p.LandingOptions, [2]string{
			"/groups/" + strconv.FormatInt(g.ID, 10) + "/chat",
			i18n.T(lang, "landing.chat", "title", g.Title),
		})
	}
	if p.SavedSearches, err = webdb.SavedSearches(ctx, s.app, sess.UID); err != nil {
		return p, err
	}
	return p, nil
}

func loadPreferences(ctx context.Context, pr *pageRequest) (any, error) {
	return pr.s.preferences(ctx, pr.sess, pr.scope)
}

// getPreferences answers GET /api/account/preferences with the
// preferences page's props.
func (s *Server) getPreferences(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	p, err := s.preferences(ctx, &sess, ScopeFrom(ctx))
	if err != nil {
		s.internalError(w, "preferences", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

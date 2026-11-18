// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package settings holds the preferences of an account
// (docs/design/web-accounts.md section 7): the known keys, how each is
// validated, the site defaults that apply to a key an account has not
// set, and their storage in web_user_settings, one row per key set.
//
// A preference is null until the account sets it, and null again when
// it goes back to the site default, so a change of a site default
// reaches every account that did not choose otherwise.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	// Zone data inside the binary, so every IANA zone validates and
	// converts whatever the image carries.
	_ "time/tzdata"

	"tgloggerd/webapi/internal/i18n"
)

// Prefs are an account's preferences; nil is the site default. The
// JSON names are the API's (the Prefs schema of webapi/openapi.yaml)
// and the keys of web_user_settings.
type Prefs struct {
	Timezone        *string `json:"timezone"`
	DateFormat      *string `json:"date_format"`
	Lang            *string `json:"lang"`
	Theme           *string `json:"theme"`
	RowsPerPage     *int    `json:"rows_per_page"`
	Landing         *string `json:"landing"`
	ChatAutoLoad    *bool   `json:"chat_auto_load"`
	ChatLive        *bool   `json:"chat_live"`
	ChatMedia       *string `json:"chat_media"`
	ChatShowDeleted *bool   `json:"chat_show_deleted"`
	ChatShowEdited  *bool   `json:"chat_show_edited"`
	ChatShowService *bool   `json:"chat_show_service"`
}

// The values the enumerated keys take.
var (
	DateFormats = []string{"24h", "12h", "iso"}
	// Langs are the languages an account may choose
	// (web-accounts.md section 7). One without a catalogue of its own
	// (internal/i18n) is shown in the site's language until it gets one.
	Langs       = []string{"en", "id"}
	Themes      = []string{"system", "light", "dark"}
	RowsPerPage = []int{25, 50, 100}
	ChatMedia   = []string{"auto", "click"}
)

// MaxLanding is the longest landing page path.
const MaxLanding = 255

// Defaults are the site's defaults for the keys the configuration
// sets (decisions/2026-11-18-web-rollout.md); the other keys have
// fixed defaults (Effective).
type Defaults struct {
	Timezone   string
	DateFormat string
	Lang       string
	Theme      string
}

// SiteDefaults are the defaults the rollout decision set: Jakarta's
// time, a 24-hour clock, English and the system's theme.
var SiteDefaults = Defaults{
	Timezone:   "Asia/Jakarta",
	DateFormat: "24h",
	Lang:       "en",
	Theme:      "system",
}

// OrSite fills every empty field from SiteDefaults.
func (d Defaults) OrSite() Defaults {
	or := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	return Defaults{
		Timezone:   or(d.Timezone, SiteDefaults.Timezone),
		DateFormat: or(d.DateFormat, SiteDefaults.DateFormat),
		Lang:       or(d.Lang, SiteDefaults.Lang),
		Theme:      or(d.Theme, SiteDefaults.Theme),
	}
}

// Check validates the defaults themselves, as the configuration gives
// them.
func (d Defaults) Check() error {
	for _, c := range []struct {
		key string
		err error
	}{
		{"timezone", CheckTimezone(d.Timezone)},
		{"date_format", oneOf(d.DateFormat, DateFormats)},
		{"lang", oneOf(d.Lang, Langs)},
		{"theme", oneOf(d.Theme, Themes)},
	} {
		if c.err != nil {
			return fmt.Errorf("%s: %w", c.key, c.err)
		}
	}
	return nil
}

// zoneName is what an IANA zone name looks like ("Asia/Jakarta",
// "Etc/GMT+7", "UTC").
var zoneName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+)*$`)

// CheckTimezone accepts an IANA zone name the zone data knows. "Local"
// (the server's own zone) is refused: it would differ between hosts.
func CheckTimezone(name string) error {
	if len(name) > 64 || !zoneName.MatchString(name) || name == "Local" {
		return fmt.Errorf("%q is not a time zone name", name)
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("%q is not a known time zone", name)
	}
	return nil
}

func oneOf[T comparable](v T, set []T) error {
	if !slices.Contains(set, v) {
		return fmt.Errorf("%v is not one of %v", v, set)
	}
	return nil
}

// CheckLanding accepts a path on this site: one leading slash, no
// backslash, no control character, at most MaxLanding bytes. Where it
// leads is checked when it is used, like any other link.
func CheckLanding(p string) error {
	if len(p) > MaxLanding || !SameSitePath(p) {
		return fmt.Errorf("%q is not a path on this site", p)
	}
	return nil
}

// SameSitePath is whether p is a path on this site: it starts with one
// slash, and holds no backslash (a browser reads "/\" as "//", another
// host) and no control character.
func SameSitePath(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") ||
		strings.ContainsRune(p, '\\') {
		return false
	}
	for _, c := range p {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

// FieldError names the key whose value was refused.
type FieldError struct {
	Key string
	Err error
}

func (e *FieldError) Error() string { return e.Key + ": " + e.Err.Error() }

// Check validates every key that is set.
func (p Prefs) Check() error {
	str := func(key string, v *string, check func(string) error) error {
		if v == nil {
			return nil
		}
		if err := check(*v); err != nil {
			return &FieldError{key, err}
		}
		return nil
	}
	in := func(set []string) func(string) error {
		return func(v string) error { return oneOf(v, set) }
	}
	for _, err := range []error{
		str("timezone", p.Timezone, CheckTimezone),
		str("date_format", p.DateFormat, in(DateFormats)),
		str("lang", p.Lang, in(Langs)),
		str("theme", p.Theme, in(Themes)),
		str("landing", p.Landing, CheckLanding),
		str("chat_media", p.ChatMedia, in(ChatMedia)),
	} {
		if err != nil {
			return err
		}
	}
	if p.RowsPerPage != nil {
		if err := oneOf(*p.RowsPerPage, RowsPerPage); err != nil {
			return &FieldError{"rows_per_page", err}
		}
	}
	return nil
}

// fields lists the keys with a pointer to each, in the order of the
// API.
func (p *Prefs) fields() []struct {
	key string
	ptr any
} {
	return []struct {
		key string
		ptr any
	}{
		{"timezone", &p.Timezone},
		{"date_format", &p.DateFormat},
		{"lang", &p.Lang},
		{"theme", &p.Theme},
		{"rows_per_page", &p.RowsPerPage},
		{"landing", &p.Landing},
		{"chat_auto_load", &p.ChatAutoLoad},
		{"chat_live", &p.ChatLive},
		{"chat_media", &p.ChatMedia},
		{"chat_show_deleted", &p.ChatShowDeleted},
		{"chat_show_edited", &p.ChatShowEdited},
		{"chat_show_service", &p.ChatShowService},
	}
}

// Keys are the known keys.
func Keys() []string {
	var p Prefs
	var out []string
	for _, f := range p.fields() {
		out = append(out, f.key)
	}
	return out
}

// Load reads an account's preferences. Rows of keys this version does
// not know, or whose value no longer validates, read as unset.
func Load(ctx context.Context, db *sql.DB, uid uint64) (Prefs, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT name, value FROM web_user_settings WHERE web_user_id = ?", uid)
	if err != nil {
		return Prefs{}, err
	}
	defer rows.Close()
	vals := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return Prefs{}, err
		}
		vals[k] = v
	}
	if err := rows.Err(); err != nil {
		return Prefs{}, err
	}
	var p Prefs
	for _, k := range Keys() {
		v, ok := vals[k]
		if !ok {
			continue
		}
		obj := []byte(`{"` + k + `":` + v + `}`)
		var one Prefs
		if json.Unmarshal(obj, &one) != nil || one.Check() != nil {
			continue
		}
		_ = json.Unmarshal(obj, &p)
	}
	return p, nil
}

// Save replaces an account's preferences with p, which the caller has
// checked: the keys set are stored, the others removed.
func Save(ctx context.Context, db *sql.DB, uid uint64, p Prefs) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM web_user_settings WHERE web_user_id = ?", uid); err != nil {
		return err
	}
	for _, f := range p.fields() {
		b, err := json.Marshal(f.ptr)
		if err != nil {
			return err
		}
		if string(b) == "null" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO web_user_settings (web_user_id, name, value) VALUES (?, ?, ?)",
			uid, f.key, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Chat is the chat view's settings in effect.
type Chat struct {
	AutoLoad    bool   `json:"auto_load"`
	Live        bool   `json:"live"`
	Media       string `json:"media"`
	ShowDeleted bool   `json:"show_deleted"`
	ShowEdited  bool   `json:"show_edited"`
	ShowService bool   `json:"show_service"`
}

// Effective is every setting in effect for an account: its own where
// it set one, the site default otherwise.
type Effective struct {
	Timezone   string
	DateFormat string
	// Lang is the language texts are shown in: the chosen one when it
	// has a catalogue, else the site's.
	Lang        string
	Theme       string
	RowsPerPage int
	Landing     string
	Chat        Chat
}

// DefaultRowsPerPage and DefaultLanding are the fixed defaults of
// their keys.
const (
	DefaultRowsPerPage = 25
	DefaultLanding     = "/"
)

// Resolve is the settings in effect under p and the site defaults d.
func Resolve(p Prefs, d Defaults) Effective {
	d = d.OrSite()
	s := func(v *string, def string) string {
		if v != nil {
			return *v
		}
		return def
	}
	b := func(v *bool) bool { return v == nil || *v }
	e := Effective{
		Timezone:    s(p.Timezone, d.Timezone),
		DateFormat:  s(p.DateFormat, d.DateFormat),
		Lang:        i18n.Pick(s(p.Lang, d.Lang)),
		Theme:       s(p.Theme, d.Theme),
		RowsPerPage: DefaultRowsPerPage,
		Landing:     s(p.Landing, DefaultLanding),
		Chat: Chat{
			AutoLoad:    b(p.ChatAutoLoad),
			Live:        b(p.ChatLive),
			Media:       s(p.ChatMedia, "auto"),
			ShowDeleted: b(p.ChatShowDeleted),
			ShowEdited:  b(p.ChatShowEdited),
			ShowService: b(p.ChatShowService),
		},
	}
	if p.RowsPerPage != nil {
		e.RowsPerPage = *p.RowsPerPage
	}
	return e
}

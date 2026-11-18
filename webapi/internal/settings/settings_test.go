// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package settings

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func str(s string) *string { return &s }
func num(n int) *int       { return &n }
func yes(b bool) *bool     { return &b }

// Every key takes what it should and refuses the rest, naming itself.
func TestCheckEachKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		key  string
		good []Prefs
		bad  []Prefs
	}{
		{"timezone",
			[]Prefs{{Timezone: str("Asia/Jakarta")}, {Timezone: str("UTC")},
				{Timezone: str("America/Argentina/Buenos_Aires")}, {Timezone: str("Etc/GMT+7")}},
			[]Prefs{{Timezone: str("")}, {Timezone: str("Local")}, {Timezone: str("Mars/Olympus")},
				{Timezone: str("../../etc/passwd")}, {Timezone: str("Asia/Jakarta ")},
				{Timezone: str(strings.Repeat("A", 65))}}},
		{"date_format",
			[]Prefs{{DateFormat: str("24h")}, {DateFormat: str("12h")}, {DateFormat: str("iso")}},
			[]Prefs{{DateFormat: str("")}, {DateFormat: str("24H")}, {DateFormat: str("us")}}},
		{"lang",
			[]Prefs{{Lang: str("en")}, {Lang: str("id")}},
			[]Prefs{{Lang: str("")}, {Lang: str("EN")}, {Lang: str("fr")}}},
		{"theme",
			[]Prefs{{Theme: str("system")}, {Theme: str("light")}, {Theme: str("dark")}},
			[]Prefs{{Theme: str("")}, {Theme: str("blue")}}},
		{"dashboard_numbers",
			[]Prefs{{DashNumbers: str("exact")}, {DashNumbers: str("compact")}},
			[]Prefs{{DashNumbers: str("")}, {DashNumbers: str("short")}, {DashNumbers: str("Exact")}}},
		{"sidebar",
			[]Prefs{{Sidebar: str("open")}, {Sidebar: str("collapsed")}},
			[]Prefs{{Sidebar: str("")}, {Sidebar: str("closed")}, {Sidebar: str("OPEN")}}},
		{"rows_per_page",
			[]Prefs{{RowsPerPage: num(25)}, {RowsPerPage: num(50)}, {RowsPerPage: num(100)}},
			[]Prefs{{RowsPerPage: num(0)}, {RowsPerPage: num(-25)}, {RowsPerPage: num(1000)}}},
		{"landing",
			[]Prefs{{Landing: str("/")}, {Landing: str("/groups/-1001000000001/chat")},
				{Landing: str("/group_messages?f=%5B%5D")}},
			[]Prefs{{Landing: str("")}, {Landing: str("groups")}, {Landing: str("//evil.example")},
				{Landing: str("/\\evil.example")}, {Landing: str("https://evil.example/")},
				{Landing: str("/a\nb")}, {Landing: str("/" + strings.Repeat("a", MaxLanding))}}},
		{"chat_media",
			[]Prefs{{ChatMedia: str("auto")}, {ChatMedia: str("click")}},
			[]Prefs{{ChatMedia: str("")}, {ChatMedia: str("never")}}},
	} {
		for _, p := range tc.good {
			if err := p.Check(); err != nil {
				t.Errorf("%s: %+v refused: %v", tc.key, p, err)
			}
		}
		for _, p := range tc.bad {
			var fe *FieldError
			if err := p.Check(); !errors.As(err, &fe) || fe.Key != tc.key {
				t.Errorf("%s: %+v not refused for its key: %v", tc.key, p, err)
			}
		}
	}
	// The switches take either value; unset is fine for every key.
	all := Prefs{ChatAutoLoad: yes(false), ChatLive: yes(true), ChatShowDeleted: yes(false),
		ChatShowEdited: yes(false), ChatShowService: yes(false)}
	if err := all.Check(); err != nil {
		t.Error(err)
	}
	if err := (Prefs{}).Check(); err != nil {
		t.Error(err)
	}
}

// The JSON names are the API's, and an unset key is null.
func TestJSON(t *testing.T) {
	t.Parallel()
	b, _ := json.Marshal(Prefs{Timezone: str("UTC"), ChatLive: yes(false)})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if len(m) != len(Keys()) || m["timezone"] != "UTC" || m["chat_live"] != false ||
		m["lang"] != nil {
		t.Errorf("%s", b)
	}
	for _, k := range Keys() {
		if _, ok := m[k]; !ok {
			t.Errorf("%s missing from %s", k, b)
		}
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	// The rollout decision: Jakarta, 24-hour clock, English, the
	// system's theme; everything else on.
	e := Resolve(Prefs{}, Defaults{})
	want := Effective{Timezone: "Asia/Jakarta", DateFormat: "24h", Lang: "en", Theme: "system",
		Sidebar: "open", DashNumbers: "exact", RowsPerPage: 25, Landing: "/", Chat: Chat{AutoLoad: true, Live: true, Media: "auto",
			ShowDeleted: true, ShowEdited: true, ShowService: true}}
	if e != want {
		t.Errorf("site defaults %+v", e)
	}
	// A configured default applies where the account set nothing; the
	// account's own choice wins.
	d := Defaults{Timezone: "UTC", DateFormat: "iso", Theme: "dark"}
	e = Resolve(Prefs{DateFormat: str("12h"), RowsPerPage: num(100), ChatLive: yes(false),
		ChatMedia: str("click")}, d)
	if e.Timezone != "UTC" || e.DateFormat != "12h" || e.Theme != "dark" || e.Lang != "en" ||
		e.RowsPerPage != 100 || e.Chat.Live || !e.Chat.AutoLoad || e.Chat.Media != "click" {
		t.Errorf("resolved %+v", e)
	}
	// A language without texts yet is shown in English.
	if e := Resolve(Prefs{Lang: str("id")}, d); e.Lang != "en" {
		t.Errorf("lang id resolves to %q", e.Lang)
	}
	if err := SiteDefaults.Check(); err != nil {
		t.Error(err)
	}
	for _, bad := range []Defaults{
		{Timezone: "Nowhere", DateFormat: "24h", Lang: "en", Theme: "system"},
		{Timezone: "UTC", DateFormat: "24", Lang: "en", Theme: "system"},
		{Timezone: "UTC", DateFormat: "24h", Lang: "xx", Theme: "system"},
		{Timezone: "UTC", DateFormat: "24h", Lang: "en", Theme: "auto"},
	} {
		if bad.Check() == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

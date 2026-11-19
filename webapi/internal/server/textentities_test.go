// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

// The text entities (TextEntity) over the "text_entities" fixture,
// whose header lists what each message of Format Lab holds, as a
// superadmin and as a member granted Format Lab alone: every kind as
// stored, offsets in UTF-16, the people mentioned resolved within the
// viewer's scope, edits, polls and checklist tasks, and none for a
// service message, malformed values and plain text.

const formatLab = int64(-1001000000060)

func formatEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t, "text_entities")
	grantMember(t, e, []int64{formatLab})
	return e
}

// formatWindow is Format Lab's messages as role sees them, by row id.
func formatWindow(t *testing.T, e *adminEnv, role string) map[int]map[string]any {
	t.Helper()
	b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100", formatLab), nil)
	out := map[int]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		out[int(m["id"].(float64))] = m
	}
	if len(out) != 8 {
		t.Fatalf("%s: %d messages", role, len(out))
	}
	return out
}

// entityList is a list of TextEntity as "offset+length type" with the
// user id when present ("=null" when null), and the url, language and
// custom emoji id when present.
func entityList(t *testing.T, v any) []string {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("entities %v (%T), want a list", v, v)
	}
	out := []string{}
	for _, x := range l {
		e := x.(map[string]any)
		s := fmt.Sprintf("%d+%d %v", num(e["offset"]), num(e["length"]), e["type"])
		if u, ok := e["user_id"]; ok {
			if u == nil {
				s += " =null"
			} else {
				s += fmt.Sprintf(" =%d", num(u))
			}
		}
		for _, k := range []string{"url", "language", "custom_emoji_id"} {
			if x, ok := e[k]; ok {
				s += fmt.Sprintf(" %s=%v", k, x)
			}
		}
		out = append(out, s)
	}
	return out
}

func sameList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// message 1's entities, with the user ids a viewer gets: dewi and
// bima for the @mentions, sari for @SariW, and the two mention_names.
func message1(dewi, bima, sari, mBima, mDewi string) []string {
	return []string{
		"0+2 custom_emoji custom_emoji_id=5368324170671202286",
		"6+11 mention" + dewi,
		"19+13 mention" + bima,
		"34+6 mention" + sari,
		"42+8 mention",
		"52+11 mention",
		"68+12 mention",
		"86+19 url",
		"114+4 text_url url=https://example.org/docs",
		"124+4 mention_name" + mBima,
		"132+4 mention_name" + mDewi,
		"138+8 hashtag",
		"147+4 bold",
		"152+6 spoiler",
		"159+13 email",
		"173+3 future_thing",
		"177+13 pre_code language=go",
	}
}

func TestTextEntitiesInTheChat(t *testing.T) {
	t.Parallel()
	e := formatEnv(t)
	for _, c := range []struct {
		role string
		want []string
	}{
		// A superadmin sees every user: each known active username
		// and both people named.
		{"superadmin", message1(" =9299000001", " =9299000002", " =9299000003",
			" =9299000002", " =9299000001")},
		// The member sees Dewi (Format Lab's admin and a sender), not
		// Bima or Sari, who never appear in Format Lab.
		{"member", message1(" =9299000001", "", "", " =null", " =9299000001")},
	} {
		ms := formatWindow(t, e, c.role)
		sameList(t, c.role+" message 1", entityList(t, ms[6001]["entities"]), c.want)

		// An edited message: its own entities, and its earlier
		// version's.
		sameList(t, c.role+" message 2", entityList(t, ms[6002]["entities"]),
			[]string{"6+3 bold"})
		edits := asList(ms[6002]["edits"])
		if len(edits) != 1 {
			t.Fatalf("%s: edits %v", c.role, ms[6002]["edits"])
		}
		sameList(t, c.role+" message 2's edit", entityList(t, edits[0].(map[string]any)["entities"]),
			[]string{"10+11 mention =9299000001"})

		// The poll's question and explanation.
		d := ms[6003]["detail"].(map[string]any)
		sameList(t, c.role+" poll question", entityList(t, d["question_entities"]),
			[]string{"5+2 italic"})
		bima := " =9299000002"
		if c.role == "member" {
			bima = " =null"
		}
		sameList(t, c.role+" poll explanation", entityList(t, d["explanation_entities"]),
			[]string{"4+4 mention_name" + bima})

		// The checklist's tasks.
		tasks := asList(ms[6004]["detail"].(map[string]any)["tasks"])
		if len(tasks) != 3 {
			t.Fatalf("%s: tasks %v", c.role, tasks)
		}
		sameList(t, c.role+" task 1", entityList(t, tasks[0].(map[string]any)["entities"]),
			[]string{"7+3 hashtag"})
		sameList(t, c.role+" task 2", entityList(t, tasks[1].(map[string]any)["entities"]),
			[]string{"4+4 mention_name =9299000001"})
		sameList(t, c.role+" task 3", entityList(t, tasks[2].(map[string]any)["entities"]), []string{})

		// None for a service message, malformed values and plain
		// text.
		for _, row := range []int{6005, 6006, 6007, 6008} {
			sameList(t, fmt.Sprintf("%s row %d", c.role, row), entityList(t, ms[row]["entities"]),
				[]string{})
		}
		// Messages without a text have [] as well.
		sameList(t, c.role+" poll message", entityList(t, ms[6003]["entities"]), []string{})
	}
}

func TestTextEntitiesOnTheMessagePage(t *testing.T) {
	t.Parallel()
	e := formatEnv(t)
	b := e.want(200, "member", "GET", "/api/page/messages/group/6001", nil)
	m := b["props"].(map[string]any)["message"].(map[string]any)
	sameList(t, "message page", entityList(t, m["entities"]),
		message1(" =9299000001", "", "", " =null", " =9299000001"))

	b = e.want(200, "superadmin", "GET", "/api/page/messages/group/6002", nil)
	m = b["props"].(map[string]any)["message"].(map[string]any)
	sameList(t, "message page, edited", entityList(t, m["entities"]), []string{"6+3 bold"})
	edits := asList(m["edits"])
	if len(edits) != 1 {
		t.Fatalf("edits %v", m["edits"])
	}
	sameList(t, "message page's edit", entityList(t, edits[0].(map[string]any)["entities"]),
		[]string{"10+11 mention =9299000001"})

	b = e.want(200, "superadmin", "GET", "/api/page/messages/group/6005", nil)
	m = b["props"].(map[string]any)["message"].(map[string]any)
	sameList(t, "service message page", entityList(t, m["entities"]), []string{})

	b = e.want(200, "superadmin", "GET", "/api/page/messages/private/6101", nil)
	m = b["props"].(map[string]any)["message"].(map[string]any)
	sameList(t, "private message page", entityList(t, m["entities"]), []string{"4+13 url"})
}

func TestTextEntitiesInAPrivateChat(t *testing.T) {
	t.Parallel()
	e := formatEnv(t)
	b := e.want(200, "superadmin", "GET", "/api/chats/private/9299000001/messages?limit=100", nil)
	for _, x := range asList(b["messages"]) {
		m := x.(map[string]any)
		if num(m["id"]) == 6101 {
			sameList(t, "private chat", entityList(t, m["entities"]), []string{"4+13 url"})
			return
		}
	}
	t.Fatal("row 6101 not in the private chat")
}

func TestParseEntities(t *testing.T) {
	t.Parallel()
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, c := range []struct {
		raw  string
		want int // entities read; -1 for none
	}{
		{"", -1},
		{"null", -1},
		{"[]", 0},
		{`[{"offset":0,"length":1,"type":"bold"}]`, 1},
		{`[{"offset":0,"length":1,"type":"custom_emoji","custom_emoji_id":"12"}]`, 1},
		{`{"offset":0}`, -1},
		{`[{"offset":0,"length":1}]`, -1},
		{`[{"offset":-1,"length":1,"type":"bold"}]`, -1},
		{`[{"offset":0,"length":1,"type":"bold","user_id":"x"}]`, -1},
		{`[{"offset":0,"length":1,"type":"custom_emoji","custom_emoji_id":1.5}]`, -1},
		{`[1]`, -1},
	} {
		got := s.parseEntities([]byte(c.raw), "test")
		n := len(got)
		if got == nil && c.want != 0 {
			n = -1
		}
		if n != c.want {
			t.Errorf("%s: %d entities, want %d", c.raw, n, c.want)
		}
	}
	id, ok := entityEmojiID(json.RawMessage("5368324170671202286"))
	if !ok || id != "5368324170671202286" {
		t.Errorf("emoji id %q %v", id, ok)
	}
}

func TestMentionUsername(t *testing.T) {
	t.Parallel()
	at := func(off, n int64) storedEntity { return storedEntity{Offset: &off, Length: &n} }
	for _, c := range []struct {
		text string
		e    storedEntity
		want string
	}{
		{"hi @dewi", at(3, 5), "dewi"},
		// UTF-16 offsets: the emoji is two code units.
		{"\U0001F600 @dewi", at(3, 5), "dewi"},
		{"hi @dewi", at(3, 9), ""},
		{"hi @dewi", at(9, 1), ""},
		{"hi dewi!", at(3, 5), ""},
		{"hi @de-wi", at(3, 6), ""},
		{"hi @", at(3, 1), ""},
	} {
		if got := mentionUsername(c.text, c.e); got != c.want {
			t.Errorf("%q at %d+%d: %q, want %q", c.text, *c.e.Offset, *c.e.Length, got, c.want)
		}
	}
}

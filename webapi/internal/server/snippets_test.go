// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"net/url"
	"testing"
)

// The summaries of messages without text (snippets.go) over the
// "snippets" fixture: the quote of a reply to each kind, the message
// lists' snippet of each, the message page and the dashboard, as a
// superadmin, an admin a superadmin hid C/C++ Study Group from, and a
// member granted Snippets Lab alone, neither of whom may see the story's
// poster.

const snippetsLab = int64(-1001000000070)

// snippetsWant are the summaries by n: message 7000 + n, quoted by reply
// 7100 + n.
var snippetsWant = map[int]string{
	1:  "Lunch at noon?",
	2:  "Capital of France?",
	3:  "Monas",
	4:  "Location",
	5:  "Live location",
	6:  "Bima Pratama",
	7:  "Video call, missed",
	8:  "Voice call, ended (1:23)",
	9:  "Voice chat, ongoing",
	10: "\U0001F3AF 6",
	11: "Tetris",
	12: "T-shirt",
	13: "Story by C/C++ Study Group",
	14: "Giveaway: Telegram Premium for 6 months and Stickers",
	15: "report.pdf",
	16: "[photo]",
	17: "[checklist]",
	18: "[venue]",
	19: "Giveaway: 1000 Stars",
	20: "Vote!",
}

// snippetFor is the summary role sees for n: the story's poster is
// named only to a viewer that sees it.
func snippetFor(role string, n int) string {
	if n == 13 && role != "superadmin" {
		return "Story"
	}
	return snippetsWant[n]
}

func snippetsEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t, "snippets")
	grantMember(t, e, []int64{snippetsLab})
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) "+
		"VALUES (3, ?)", cppGroup); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestKindSnippets(t *testing.T) {
	t.Parallel()
	e := snippetsEnv(t)
	for _, role := range []string{"superadmin", "admin", "member"} {
		// The chat's reply quotes.
		b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100",
			snippetsLab), nil)
		quotes := map[int]string{}
		for _, m := range asList(b["messages"]) {
			m := m.(map[string]any)
			if r, ok := m["reply_to"].(map[string]any); ok {
				quotes[int(num(m["id"]))-7100] = fmt.Sprint(r["snippet"])
			}
		}
		// The group messages list's snippets.
		q := url.Values{"per": {"100"},
			"f": {fmt.Sprintf(`[{"field":"chat_id","op":"=","value":"%d"}]`, snippetsLab)}}
		b = e.want(200, role, "GET", "/api/page/group_messages?"+q.Encode(), nil)
		props, _ := b["props"].(map[string]any)
		listed := map[int]string{}
		for _, r := range asList(props["rows"]) {
			r := r.(map[string]any)
			listed[int(num(r["id"]))-7000] = fmt.Sprint(r["snippet"])
		}
		for n := range snippetsWant {
			want := snippetFor(role, n)
			if quotes[n] != want {
				t.Errorf("%s: reply to %d quotes %q, want %q", role, n, quotes[n], want)
			}
			if listed[n] != want {
				t.Errorf("%s: list row %d shows %q, want %q", role, n, listed[n], want)
			}
		}
		if listed[101] != "Re 1" {
			t.Errorf("%s: list row 7101 shows %q", role, listed[101])
		}

		// The message page: its own snippet and its reply's quote.
		b = e.want(200, role, "GET", "/api/page/messages/group/7001", nil)
		msg := b["props"].(map[string]any)["message"].(map[string]any)
		if msg["snippet"] != "Lunch at noon?" || msg["text"] != "" {
			t.Errorf("%s: poll page %v / %v", role, msg["snippet"], msg["text"])
		}
		b = e.want(200, role, "GET", "/api/page/messages/group/7113", nil)
		msg = b["props"].(map[string]any)["message"].(map[string]any)
		if r, _ := msg["reply_to"].(map[string]any); r == nil || r["snippet"] != snippetFor(role, 13) {
			t.Errorf("%s: story reply page %v", role, msg["reply_to"])
		}
	}

	// The dashboard's deleted messages: the venue, deleted last.
	b := e.want(200, "superadmin", "GET", "/api/page/", nil)
	deleted := asList(b["props"].(map[string]any)["deleted"])
	if len(deleted) == 0 || num(deleted[0].(map[string]any)["id"]) != 7003 ||
		deleted[0].(map[string]any)["snippet"] != "Monas" {
		t.Errorf("dashboard deleted %v", deleted)
	}
}

// The summaries the fixture leaves out: a call's other outcomes, a die
// without its emoji, a giveaway without a prize, a story without a
// poster, a contact without a name, a poll not loaded.
func TestKindSummaryEdges(t *testing.T) {
	t.Parallel()
	v := view{}
	one := int64(1)
	for _, c := range []struct {
		ctype string
		x     *kindExtra
		want  string
	}{
		{"call", &kindExtra{DiscardReason: "declined"}, "Voice call, declined"},
		{"call", &kindExtra{DiscardReason: "disconnected", Duration: 3725}, "Voice call, disconnected (1:02:05)"},
		{"call", &kindExtra{DiscardReason: "upgrade_to_group_call"}, "Voice call, moved to a group call"},
		{"call", &kindExtra{Group: true, Video: true, Duration: 60}, "Video chat, ended (1:00)"},
		{"call", &kindExtra{Group: true, Missed: true}, "Voice chat, missed"},
		{"dice", &kindExtra{Value: 5}, "\U0001F3B2 5"},
		{"dice", &kindExtra{Emoji: "\U0001F3B0"}, "\U0001F3B0"},
		{"giveaway", &kindExtra{}, "Giveaway"},
		{"giveaway", &kindExtra{Prize: &struct {
			PremiumMonths *int64 `json:"premium_months"`
			Stars         *int64 `json:"stars"`
		}{PremiumMonths: &one}}, "Giveaway: Telegram Premium for 1 month"},
		{"story", &kindExtra{}, "Story"},
		{"contact", &kindExtra{}, "[contact]"},
		{"contact", &kindExtra{LastName: "Pratama"}, "Pratama"},
		{"poll", &kindExtra{PollID: "42"}, "[poll]"},
		{"venue", &kindExtra{}, "[venue]"},
		{"paid_media", &kindExtra{StarCount: 5}, "[paid_media]"},
	} {
		k := kinds{extra: map[uint64]*kindExtra{1: c.x}}
		if got := v.summary(k, 1, c.ctype, ""); got != c.want {
			t.Errorf("%s %+v: %q, want %q", c.ctype, c.x, got, c.want)
		}
	}
	// A file's name stands in for a kind without a summary, never for
	// one with it.
	k := kinds{extra: map[uint64]*kindExtra{1: {Title: "Monas"}}}
	if got := v.summary(k, 1, "venue", "map.png"); got != "Monas" {
		t.Errorf("venue with a file: %q", got)
	}
	if got := v.summary(kinds{}, 1, "video", "clip.mp4"); got != "clip.mp4" {
		t.Errorf("video: %q", got)
	}
}

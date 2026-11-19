// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// checkParts fails unless a ServiceInfo's parts spell text and each
// name points at a ref of that title; it returns the parts as
// "text" for plain text and "[u0:text]" or "[c0:text]" for a name.
func checkParts(t *testing.T, where string, s serviceJSON, text string) string {
	t.Helper()
	var b, shape strings.Builder
	for _, p := range s.Parts {
		b.WriteString(p.Text)
		switch {
		case p.User != nil && p.Chat != nil:
			t.Errorf("%s: part %q is both a user and a chat", where, p.Text)
		case p.User != nil:
			if *p.User >= len(s.Refs.Users) || s.Refs.Users[*p.User].Title != p.Text {
				t.Errorf("%s: part %q points at user %d of %v", where, p.Text, *p.User, s.Refs.Users)
			}
			fmt.Fprintf(&shape, "[u%d:%s]", *p.User, p.Text)
		case p.Chat != nil:
			if *p.Chat >= len(s.Refs.Chats) || s.Refs.Chats[*p.Chat].Title != p.Text {
				t.Errorf("%s: part %q points at chat %d of %v", where, p.Text, *p.Chat, s.Refs.Chats)
			}
			fmt.Fprintf(&shape, "[c%d:%s]", *p.Chat, p.Text)
		default:
			shape.WriteString(p.Text)
		}
	}
	if s.Parts != nil && b.String() != text {
		t.Errorf("%s: parts spell %q, text %q", where, b.String(), text)
	}
	return shape.String()
}

// Every action service.go rebuilds gives parts that spell its text,
// each name pointing at its ref, for a viewer that sees some of the
// people named and not the others; the stored text has no parts.
func TestServicePartsEveryType(t *testing.T) {
	t.Parallel()
	const (
		dewiID  = int64(9299000101)
		bimaID  = int64(9299000102)
		hidden  = int64(9299000199)
		chatID  = int64(-1001000000901)
		boosted = int64(-1001000000902)
	)
	k := kinds{chat: chatID, service: map[uint64]*serviceExtra{},
		users: map[int64]logdb.User{
			dewiID: {ID: dewiID, FirstName: "Dewi", LastName: "Kusuma"},
			bimaID: {ID: bimaID, FirstName: "Bima", LastName: "Pratama"},
		},
		groups: map[int64]logdb.Group{boosted: {ID: boosted, Title: "Boost Club"}}}
	v := view{sc: scope.Anonymous()}
	cases := []struct {
		typ, extra string
		sender     int64
		want       string // parts; "" for none
	}{
		{"chat_add_members", fmt.Sprintf(`{"user_ids":[%d,%d,%d]}`, dewiID, hidden, bimaID), 1,
			"added [u0:Dewi Kusuma], [u1:Bima Pratama] and 1 other member to the group"},
		{"chat_add_members", fmt.Sprintf(`{"user_ids":[%d,%d]}`, dewiID, bimaID), 1,
			"added [u0:Dewi Kusuma] and [u1:Bima Pratama] to the group"},
		{"chat_add_members", fmt.Sprintf(`{"user_ids":[%d]}`, hidden), 1, ""},
		{"chat_add_members", fmt.Sprintf(`{"user_ids":[%d]}`, dewiID), dewiID, ""},
		{"chat_delete_member", fmt.Sprintf(`{"user_id":%d}`, bimaID), 1,
			"removed [u0:Bima Pratama] from the group"},
		{"chat_delete_member", fmt.Sprintf(`{"user_id":%d}`, hidden), 1, ""},
		{"chat_owner_left", fmt.Sprintf(`{"new_owner_user_id":%d}`, dewiID), 1,
			"left the group and handed ownership to [u0:Dewi Kusuma]"},
		{"chat_owner_changed", fmt.Sprintf(`{"new_owner_user_id":%d}`, bimaID), 1,
			"transferred ownership of the group to [u0:Bima Pratama]"},
		{"invite_video_chat_participants",
			fmt.Sprintf(`{"user_ids":[%d,%d,%d,%d]}`, hidden, bimaID, hidden, dewiID), 1,
			"invited [u0:Bima Pratama], [u1:Dewi Kusuma] and 2 other participants to the video chat"},
		{"giveaway_winners", fmt.Sprintf(`{"boosted_chat_id":%d,"winner_user_ids":[%d,%d]}`,
			boosted, dewiID, hidden), 1,
			"announced the winners of the giveaway in [c0:Boost Club]: [u0:Dewi Kusuma] and 1 other winner"},
		{"giveaway_winners", fmt.Sprintf(`{"boosted_chat_id":%d,"winner_user_ids":[%d]}`,
			boosted, hidden), 1, "announced the winners of the giveaway in [c0:Boost Club]"},
		{"giveaway_winners", fmt.Sprintf(`{"boosted_chat_id":-1001000000999,"winner_user_ids":[%d]}`,
			bimaID), 1, "announced the giveaway winners: [u0:Bima Pratama]"},
		{"chat_shared", fmt.Sprintf(`{"chat":{"chat_id":%d}}`, boosted), 1,
			"shared [c0:Boost Club] with the bot"},
		{"chat_shared", fmt.Sprintf(`{"chat":{"user_id":%d}}`, dewiID), 1,
			"shared [u0:Dewi Kusuma] with the bot"},
		{"chat_shared", `{"chat":{"chat_id":-1001000000999}}`, 1, ""},
		{"pin_message", `{"message_id":700003}`, 1, ""},
	}
	for i, c := range cases {
		m := logdb.Message{RowID: uint64(i + 1), ChatID: chatID, SenderUser: c.sender,
			ContentType: "service", ServiceType: c.typ, Text: "stored text"}
		k.service[m.RowID] = parseServiceExtra([]byte(c.extra))
		s := v.service(k, m)
		text := v.text(k, m)
		where := fmt.Sprintf("%s %s", c.typ, c.extra)
		if got := checkParts(t, where, *s, text); got != c.want {
			t.Errorf("%s: parts %q, want %q", where, got, c.want)
		}
		if c.want == "" && (s.Parts != nil || text != "stored text") {
			t.Errorf("%s: stored text %q with parts %v", where, text, s.Parts)
		}
		// Absent, not null or empty, where there are none.
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"parts"`) != (c.want != "") {
			t.Errorf("%s: %s", where, raw)
		}
		// The one hidden is never named.
		if strings.Contains(string(raw), "9299000199") {
			t.Errorf("%s names the hidden user: %s", where, raw)
		}
	}
}

// serviceAt is the ServiceInfo and text of a message as the API sent
// them.
func serviceAt(t *testing.T, m map[string]any) (serviceJSON, string) {
	t.Helper()
	var s serviceJSON
	raw, _ := json.Marshal(m["service"])
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("service %s: %v", raw, err)
	}
	return s, fmt.Sprint(m["text"])
}

// Over the API, in the chat and on the message page, each viewer's
// parts spell its text and name only whom it may see: a member granted
// Kinds Lab alone gets Andi linked and Bima counted, and nothing on a
// line that names nobody it may see.
func TestServiceParts(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	cases := map[string]map[int]string{
		"superadmin": {
			4018: "added [u0:Andi Santoso] and [u1:Bima Pratama] to the group",
			4019: "announced the winners of the giveaway in [c0:C/C++ Study Group]: " +
				"[u0:Bima Pratama] and [u1:Dewi Kusuma]",
			4021: "",
			4022: "removed [u0:Bima Pratama] from the group",
		},
		"admin": {
			4018: "added [u0:Andi Santoso] and [u1:Bima Pratama] to the group",
			4019: "announced the giveaway winners: [u0:Bima Pratama] and [u1:Dewi Kusuma]",
			4022: "removed [u0:Bima Pratama] from the group",
		},
		"member": {
			4018: "added [u0:Andi Santoso] and 1 other member to the group",
			4019: "announced the giveaway winners: [u0:Dewi Kusuma] and 1 other winner",
			4020: "",
			4021: "",
			4022: "",
		},
	}
	for role, rows := range cases {
		ms := kindsWindow(t, e, role, "")
		for row, want := range rows {
			s, text := serviceAt(t, ms[row])
			where := fmt.Sprintf("%s row %d", role, row)
			if got := checkParts(t, where, s, text); got != want {
				t.Errorf("%s: parts %q, want %q", where, got, want)
			}
			if _, ok := ms[row]["service"].(map[string]any)["parts"]; ok != (want != "") {
				t.Errorf("%s: parts present %v", where, ok)
			}
		}
	}
	// The message page sends the same parts.
	for role, want := range map[string]string{"superadmin": "removed [u0:Bima Pratama] from the group",
		"member": ""} {
		b := e.want(200, role, "GET", "/api/page/messages/group/4022", nil)
		msg := b["props"].(map[string]any)["message"].(map[string]any)
		s, text := serviceAt(t, msg)
		if got := checkParts(t, role+" message page", s, text); got != want {
			t.Errorf("%s message page: parts %q, want %q", role, got, want)
		}
	}
}

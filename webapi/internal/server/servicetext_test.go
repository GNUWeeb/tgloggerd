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

func TestAutoDeletePeriod(t *testing.T) {
	t.Parallel()
	for sec, want := range map[int64]string{1: "1 second", 45: "45 seconds", 60: "1 minute",
		300: "5 minutes", 3600: "1 hour", 90000: "25 hours", 86400: "1 day",
		2 * 86400: "2 days", 7 * 86400: "1 week", 14 * 86400: "2 weeks",
		30 * 86400: "1 month", 31 * 86400: "1 month", 62 * 86400: "2 months",
		365 * 86400: "1 year"} {
		if got := autoDeletePeriod(sec); got != want {
			t.Errorf("%d s: %q, want %q", sec, got, want)
		}
	}
}

// Each action servicetext.go rebuilds, one subtest per type, for a
// viewer that sees Dewi, Bima and Boost Club but not the hidden user:
// the parts spell the text, each name points at its ref, the hidden
// one is never named, and a line with nothing to add keeps the stored
// text without parts. Dewi is the sender, whom a line does not name
// again.
func TestServiceTextEveryType(t *testing.T) {
	t.Parallel()
	const (
		dewiID  = int64(9299000101)
		bimaID  = int64(9299000102)
		botID   = int64(9299000103)
		hidden  = int64(9299000199)
		chatID  = int64(-1001000000901)
		boosted = int64(-1001000000902)
	)
	k := kinds{chat: chatID, service: map[uint64]*serviceExtra{},
		users: map[int64]logdb.User{
			dewiID: {ID: dewiID, FirstName: "Dewi", LastName: "Kusuma"},
			bimaID: {ID: bimaID, FirstName: "Bima", LastName: "Pratama"},
			botID:  {ID: botID, FirstName: "Release Bot"},
		},
		groups: map[int64]logdb.Group{boosted: {ID: boosted, Title: "Boost Club"}},
		targets: map[int64]*kindExtra{
			// A checklist whose task 3 has no text.
			700009: parseKindExtra([]byte(`{"tasks":[{"id":1,"text":"Buy milk"},` +
				`{"id":2,"text":"Call Bima"},{"id":3},{"id":4,"text":"Ship it"},` +
				`{"id":5,"text":"Rest"}]}`)),
			700012: parseKindExtra([]byte(`{"prize":{"premium_months":6},` +
				`"prize_description":"Stickers","winner_count":3}`)),
			700013: parseKindExtra([]byte(`{"prize":{"stars":500}}`)),
		}}
	v := view{sc: scope.Anonymous()}
	type tc struct {
		extra string
		want  string // parts; "" for the stored text
	}
	cases := map[string][]tc{
		"chat_set_message_auto_delete_time": {
			{`{"seconds":86400}`, "set messages to auto-delete after 1 day"},
			{`{"seconds":604800}`, "set messages to auto-delete after 1 week"},
			{fmt.Sprintf(`{"seconds":2678400,"from_user_id":%d}`, bimaID),
				"set messages to auto-delete after 1 month, the default setting of [u0:Bima Pratama]"},
			{fmt.Sprintf(`{"seconds":3600,"from_user_id":%d}`, hidden),
				"set messages to auto-delete after 1 hour"},
			{fmt.Sprintf(`{"seconds":0,"from_user_id":%d}`, bimaID),
				"disabled auto-delete for messages, the default setting of [u0:Bima Pratama]"},
			{`{"seconds":0}`, ""},
		},
		"proximity_alert_triggered": {
			{fmt.Sprintf(`{"traveler":{"user_id":%d},"watcher":{"user_id":%d},"distance":120}`,
				dewiID, bimaID), "came within 120 metres of [u1:Bima Pratama]"},
			{fmt.Sprintf(`{"traveler":{"user_id":%d},"watcher":{"user_id":%d},"distance":1}`,
				bimaID, dewiID),
				"reported that [u0:Bima Pratama] came within 1 metre of [u1:Dewi Kusuma]"},
			{fmt.Sprintf(`{"traveler":{"user_id":%d},"watcher":{"chat_id":%d},"distance":50}`,
				hidden, boosted), "came within 50 metres of [c0:Boost Club]"},
			{fmt.Sprintf(`{"traveler":{"user_id":%d},"watcher":{"user_id":%d},"distance":50}`,
				dewiID, hidden), ""},
		},
		"users_shared": {
			{fmt.Sprintf(`{"users":[{"user_id":%d,"first_name":"Bima"},{"user_id":%d,"first_name":"X"}]}`,
				bimaID, hidden), "shared [u0:Bima Pratama] and 1 other user with the bot"},
			{fmt.Sprintf(`{"users":[{"user_id":%d}]}`, hidden), ""},
		},
		"gifted_premium": {
			{fmt.Sprintf(`{"receiver_user_id":%d,"month_count":3}`, bimaID),
				"gifted a Telegram Premium subscription for 3 months to [u0:Bima Pratama]"},
			{fmt.Sprintf(`{"gifter_user_id":%d,"day_count":7}`, dewiID),
				"gifted a Telegram Premium subscription for 7 days"},
			{fmt.Sprintf(`{"gifter_user_id":%d,"receiver_user_id":%d}`, bimaID, dewiID),
				"gifted a Telegram Premium subscription from [u0:Bima Pratama] to [u1:Dewi Kusuma]"},
			{fmt.Sprintf(`{"receiver_user_id":%d}`, hidden), ""},
		},
		"gifted_stars": {
			{fmt.Sprintf(`{"receiver_user_id":%d,"star_count":500}`, bimaID),
				"gifted 500 Telegram Stars to [u0:Bima Pratama]"},
			{fmt.Sprintf(`{"receiver_user_id":%d,"star_count":1}`, hidden),
				"gifted 1 Telegram Star"},
			{fmt.Sprintf(`{"gifter_user_id":%d}`, hidden), ""},
		},
		"gift": {
			{fmt.Sprintf(`{"sender":{"user_id":%d},"receiver":{"chat_id":%d}}`, dewiID, boosted),
				"sent a gift to [c0:Boost Club]"},
			{fmt.Sprintf(`{"sender":{"user_id":%d},"receiver":{"user_id":%d}}`, bimaID, dewiID),
				"sent a gift from [u0:Bima Pratama] to [u1:Dewi Kusuma]"},
			{fmt.Sprintf(`{"sender":{"user_id":%d},"receiver":{"user_id":%d}}`, dewiID, hidden), ""},
		},
		"managed_bot_created": {
			{fmt.Sprintf(`{"bot_user_id":%d,"manager_bot_user_id":%d}`, botID, hidden),
				"created the bot [u0:Release Bot]"},
			{fmt.Sprintf(`{"bot_user_id":%d}`, hidden), ""},
		},
		"checklist_tasks_done": {
			{`{"checklist_message_id":700009,"done_task_ids":[1,2]}`,
				`marked "Buy milk" and "Call Bima" as done`},
			{`{"checklist_message_id":700009,"done_task_ids":[1,2,4,5],"not_done_task_ids":[3]}`,
				`marked "Buy milk", "Call Bima", "Ship it" and 1 other task as done and 1 task as not done`},
			{`{"checklist_message_id":700009,"not_done_task_ids":[3,4,9]}`,
				`marked "Ship it" and 2 other tasks as not done`},
			{`{"checklist_message_id":700777,"done_task_ids":[1,2]}`, "marked 2 tasks as done"},
			{`{"checklist_message_id":700009}`, ""},
		},
		"checklist_tasks_added": {
			{`{"checklist_message_id":700009,"tasks":[{"id":6,"text":"Write notes"}]}`,
				`added "Write notes" to the checklist`},
			{`{"tasks":[{"id":6,"text":"A"},{"id":7,"text":"B"},{"id":8,"text":"C"},` +
				`{"id":9,"text":"D"},{"id":10}]}`,
				`added "A", "B", "C" and 2 other tasks to the checklist`},
			{`{"checklist_message_id":700009}`, ""},
		},
		"suggest_birthdate": {
			{`{"day":14,"month":3,"year":1995}`, "suggested a birthdate: 14 March 1995"},
			{`{"day":1,"month":12}`, "suggested a birthdate: 1 December"},
			{`{"day":1,"month":13}`, ""},
		},
		"giveaway_created": {
			{`{"star_count":500}`, "created a giveaway of 500 Telegram Stars"},
			{`{}`, ""},
		},
		"giveaway_completed": {
			{`{"giveaway_message_id":700012,"winner_count":3,"unclaimed_prize_count":1}`,
				"the giveaway of Telegram Premium for 6 months and Stickers was completed " +
					"with 3 winners and 1 unclaimed prize"},
			{`{"giveaway_message_id":700013,"winner_count":1,"star_giveaway":true}`,
				"the giveaway of 500 Telegram Stars was completed with 1 winner"},
			{`{"giveaway_message_id":700777,"winner_count":2,"star_giveaway":true,` +
				`"unclaimed_prize_count":2}`,
				"the Telegram Stars giveaway was completed with 2 winners and 2 unclaimed prizes"},
			{`{"giveaway_message_id":700777}`, ""},
		},
	}
	row := uint64(0)
	for typ, tcs := range cases {
		t.Run(typ, func(t *testing.T) {
			for _, c := range tcs {
				row++
				m := logdb.Message{RowID: row, ChatID: chatID, SenderUser: dewiID,
					ContentType: "service", ServiceType: typ, Text: "stored text",
					Extra: []byte(c.extra)}
				k.service[m.RowID] = parseServiceExtra(m.Extra)
				s := v.service(k, m)
				text := v.text(k, m)
				where := typ + " " + c.extra
				if c.want == "" {
					if s.Parts != nil || text != "stored text" {
						t.Errorf("%s: text %q with parts %v, want the stored text",
							where, text, s.Parts)
					}
					continue
				}
				if got := checkParts(t, where, *s, text); got != c.want {
					t.Errorf("%s: parts %q, want %q", where, got, c.want)
				}
				raw, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), "9299000199") {
					t.Errorf("%s names the hidden user: %s", where, raw)
				}
			}
		})
	}
}

// Over the API, a checklist and a giveaway the archive holds give their
// tasks' text and prize to the lines about them, and one it does not
// hold leaves counts only.
func TestServiceTextTargets(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	// Kinds Lab's checklist (700009) and giveaway (700012), and a
	// checklist the archive does not hold (700099).
	if _, err := e.ro.Exec("INSERT INTO telegram_group_messages (id, chat_id, message_id, " +
		"sender_user_id, date, content_type, text, tdlib_type, content_extra, service_type) VALUES " +
		"(4031, -1001000000040, 700031, 9299000001, 1794928980, 'service', " +
		"'marked checklist tasks as done', 'messageChecklistTasksDone', " +
		"'{\"checklist_message_id\":700009,\"done_task_ids\":[2,3],\"not_done_task_ids\":[1]}', " +
		"'checklist_tasks_done'), " +
		"(4032, -1001000000040, 700032, 9299000004, 1794929040, 'service', " +
		"'the giveaway was completed', 'messageGiveawayCompleted', " +
		"'{\"giveaway_message_id\":700012,\"winner_count\":3}', 'giveaway_completed'), " +
		"(4033, -1001000000040, 700033, 9299000001, 1794929100, 'service', " +
		"'marked checklist tasks as done', 'messageChecklistTasksDone', " +
		"'{\"checklist_message_id\":700099,\"done_task_ids\":[2,3]}', 'checklist_tasks_done')"); err != nil {
		t.Fatal(err)
	}
	want := map[int]string{
		4031: `marked "Task 2" and "Task 3" as done and "Task 1" as not done`,
		4032: "the giveaway of Telegram Premium for 6 months and Stickers was completed " +
			"with 3 winners",
		4033: "marked 2 tasks as done",
	}
	for _, role := range []string{"superadmin", "member"} {
		ms := kindsWindow(t, e, role, "")
		for row, w := range want {
			s, text := serviceAt(t, ms[row])
			where := fmt.Sprintf("%s row %d", role, row)
			if got := checkParts(t, where, s, text); got != w || text != w {
				t.Errorf("%s: parts %q, text %q, want %q", where, got, text, w)
			}
		}
	}
	// The message page reads the checklist too.
	b := e.want(200, "superadmin", "GET", "/api/page/messages/group/4031", nil)
	msg := b["props"].(map[string]any)["message"].(map[string]any)
	if _, text := serviceAt(t, msg); text != want[4031] {
		t.Errorf("message page: text %q, want %q", text, want[4031])
	}
}

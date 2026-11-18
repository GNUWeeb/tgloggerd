// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The message kinds' detail (MessageDetail) over the "kinds" fixture,
// whose header lists what each message of Kinds Lab is, as a
// superadmin, an admin a superadmin hid C/C++ Study Group from, and a
// member granted Kinds Lab alone: each kind from its content_extra and
// files, the poll from the poll tables, the 20-item cap with its
// total, a contact's phone for full viewers only, and the people and
// chats named resolved through the scope.

const (
	kindsLab = int64(-1001000000040)
	cppGroup = int64(-1001000000004)
	bima     = int64(9299000002)
	andi     = int64(9299000004)
)

// kindsEnv is the admin env over the "kinds" fixture, with the member
// granted Kinds Lab and C/C++ Study Group hidden from the admin.
func kindsEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t, "kinds")
	grantMember(t, e, []int64{kindsLab})
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) "+
		"VALUES (3, ?)", cppGroup); err != nil {
		t.Fatal(err)
	}
	return e
}

// kindsWindow is Kinds Lab's messages as role sees them, by row id.
func kindsWindow(t *testing.T, e *adminEnv, role, query string) map[int]map[string]any {
	t.Helper()
	b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100%s",
		kindsLab, query), nil)
	out := map[int]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		out[int(m["id"].(float64))] = m
	}
	return out
}

// detailOf is a message's detail, failing when it has none.
func detailOf(t *testing.T, ms map[int]map[string]any, row int) map[string]any {
	t.Helper()
	m, ok := ms[row]
	if !ok {
		t.Fatalf("row %d not in the window", row)
	}
	d, ok := m["detail"].(map[string]any)
	if !ok {
		t.Fatalf("row %d: detail %v", row, m["detail"])
	}
	return d
}

// refIDs are the ids of a list of ChatRefs.
func refIDs(v any) []int64 {
	out := []int64{}
	for _, r := range asList(v) {
		out = append(out, num(r.(map[string]any)["id"]))
	}
	return out
}

func refID(v any) int64 {
	if r, ok := v.(map[string]any); ok {
		return num(r["id"])
	}
	return 0
}

func TestKindDetails(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	ms := kindsWindow(t, e, "superadmin", "")
	if len(ms) != 22 {
		t.Fatalf("%d messages", len(ms))
	}
	for row, want := range map[int]string{4001: "location", 4005: "dice", 4008: "poll",
		4014: "call", 4016: "rich", 4017: "expired", 4018: "service"} {
		if ms[row]["content_type"] != want {
			t.Errorf("row %d: content_type %v, want %s", row, ms[row]["content_type"], want)
		}
	}

	if d := detailOf(t, ms, 4001); d["kind"] != "location" || d["lat"] != -6.2 ||
		d["lon"] != 106.8166 || d["accuracy"] != 5.5 || d["live"] != nil {
		t.Errorf("location %v", d)
	}
	// A live location: its latest position and how often it moved.
	d := detailOf(t, ms, 4002)
	live, _ := d["live"].(map[string]any)
	if d["lat"] != -6.21 || d["accuracy"] != nil || live == nil || num(live["period"]) != 900 ||
		num(live["heading"]) != 90 || num(live["proximity_radius"]) != 100 || num(live["updates"]) != 3 {
		t.Errorf("live location %v", d)
	}
	if d := detailOf(t, ms, 4003); d["kind"] != "venue" || d["title"] != "Monas" ||
		d["address"] != "Gambir, Jakarta" || d["provider"] != "foursquare" ||
		d["venue_id"] != "4b0587" || d["venue_type"] != "arts_entertainment/monument" {
		t.Errorf("venue %v", d)
	}
	if d := detailOf(t, ms, 4004); d["kind"] != "contact" || d["first_name"] != "Bima" ||
		d["phone"] != "+628123456789" || !strings.Contains(fmt.Sprint(d["vcard"]), "TEL:") ||
		refID(d["user"]) != bima {
		t.Errorf("contact %v", d)
	}
	if d := detailOf(t, ms, 4005); d["kind"] != "dice" || d["emoji"] != "\U0001F3B2" ||
		num(d["value"]) != 4 || d["stake"] != false {
		t.Errorf("dice %v", d)
	}
	if d := detailOf(t, ms, 4006); d["kind"] != "dice" || num(d["value"]) != 3 || d["stake"] != true {
		t.Errorf("stake dice %v", d)
	}
	d = detailOf(t, ms, 4007)
	if photo, _ := d["photo"].(map[string]any); d["kind"] != "game" || d["title"] != "Tetris" ||
		d["short_name"] != "tetris" || d["description"] != "Falling blocks." || photo == nil ||
		photo["render"] != "image" || photo["url"] == nil {
		t.Errorf("game %v", d)
	}
	if ms[4007]["text"] != "Play it" {
		t.Errorf("game text %v", ms[4007]["text"])
	}

	// The poll from its tables: the first 20 of its 25 options (the
	// removed one left out), the quiz's correct option marked.
	d = detailOf(t, ms, 4008)
	opts := asList(d["options"])
	if d["kind"] != "poll" || d["question"] != "Which option?" || d["type"] != "quiz" ||
		d["anonymous"] != true || d["multiple"] != false || d["closed"] != true ||
		num(d["total_voters"]) != 325 || d["explanation"] != "The third one." ||
		d["close_date"] != "2026-11-17T16:00:00Z" || len(opts) != 20 || num(d["options_total"]) != 25 {
		t.Errorf("poll %v", d)
	}
	for i, o := range opts {
		o := o.(map[string]any)
		if o["text"] != fmt.Sprintf("Option %d", i+1) || num(o["voters"]) != int64(i+1) ||
			num(o["percent"]) != 4 || o["correct"] != (i == 2) {
			t.Errorf("poll option %d: %v", i, o)
		}
	}

	// The checklist: 20 of 22 tasks, who did the first two.
	d = detailOf(t, ms, 4009)
	tasks := asList(d["tasks"])
	if d["kind"] != "checklist" || len(tasks) != 20 || num(d["tasks_total"]) != 22 ||
		d["others_can_add"] != true || d["others_can_mark"] != false {
		t.Errorf("checklist %v", d)
	}
	if len(tasks) == 20 {
		t1, t2, t3 := tasks[0].(map[string]any), tasks[1].(map[string]any), tasks[2].(map[string]any)
		if t1["text"] != "Task 1" || t1["done"] != true || refID(t1["done_by"]) != bima ||
			t1["done_at"] == nil || refID(t2["done_by"]) != dewi || t3["done"] != false ||
			t3["done_by"] != nil || t3["done_at"] != nil {
			t.Errorf("tasks %v %v %v", t1, t2, t3)
		}
	}
	if d := detailOf(t, ms, 4010); d["kind"] != "story" || num(d["story_id"]) != 12 ||
		refID(d["poster"]) != cppGroup {
		t.Errorf("story %v", d)
	}
	d = detailOf(t, ms, 4011)
	if photo, _ := d["photo"].(map[string]any); d["kind"] != "invoice" || d["title"] != "T-shirt" ||
		d["description"] != "A black T-shirt." || d["currency"] != "IDR" ||
		num(d["total_amount"]) != 15000000 || d["is_test"] != true || photo == nil {
		t.Errorf("invoice %v", d)
	}
	d = detailOf(t, ms, 4012)
	prize, _ := d["prize"].(map[string]any)
	if d["kind"] != "giveaway" || num(d["winner_count"]) != 3 || prize == nil ||
		num(prize["premium_months"]) != 6 || prize["stars"] != nil || d["only_new_members"] != true ||
		d["prize_description"] != "Stickers" || d["winners_selection_date"] == nil ||
		fmt.Sprint(refIDs(d["chats"])) != fmt.Sprint([]int64{kindsLab, cppGroup, kernelChat}) ||
		num(d["chats_total"]) != 3 {
		t.Errorf("giveaway %v", d)
	}
	if d := detailOf(t, ms, 4013); d["kind"] != "call" || d["video"] != true || d["group"] != false ||
		num(d["duration"]) != 83 || d["outcome"] != "hung_up" || len(asList(d["participants"])) != 0 ||
		num(d["participants_total"]) != 0 {
		t.Errorf("call %v", d)
	}
	// A group call of 25: the first 20, the users the archive has no
	// row for named by their id.
	d = detailOf(t, ms, 4014)
	ps := asList(d["participants"])
	if d["group"] != true || d["outcome"] != "ended" || num(d["duration"]) != 3725 ||
		len(ps) != 20 || num(d["participants_total"]) != 25 {
		t.Errorf("group call %v", d)
	}
	if ids := refIDs(ps); len(ids) == 20 && (ids[0] != dewi || ids[1] != bima || ids[2] != cppGroup ||
		ids[3] != andi || ids[4] != 9299000101) {
		t.Errorf("group call participants %v", ids)
	}
	if p := ps[4].(map[string]any); p["title"] != "#9299000101" || p["kind"] != "user" {
		t.Errorf("unknown participant %v", p)
	}
	d = detailOf(t, ms, 4015)
	items := asList(d["items"])
	if d["kind"] != "paid_media" || num(d["star_count"]) != 50 || len(items) != 2 {
		t.Fatalf("paid media %v", d)
	}
	if i0, i1 := items[0].(map[string]any), items[1].(map[string]any); i0["kind"] != "photo" ||
		i0["media"] == nil || i1["kind"] != "preview" || i1["media"] != nil {
		t.Errorf("paid items %v", items)
	}
	for row, kind := range map[int]string{4016: "rich", 4017: "expired"} {
		if d := detailOf(t, ms, row); len(d) != 1 || d["kind"] != kind {
			t.Errorf("row %d: %v", row, d)
		}
	}
	// Service messages and rows logged before migration 000034 have
	// no detail.
	for _, row := range []int{4018, 4021} {
		if _, ok := ms[row]["detail"]; !ok || ms[row]["detail"] != nil {
			t.Errorf("row %d: detail %v", row, ms[row]["detail"])
		}
	}

	// The message page carries the same detail.
	b := e.want(200, "superadmin", "GET", "/api/page/messages/group/4008", nil)
	msg := b["props"].(map[string]any)["message"].(map[string]any)
	if pd, _ := msg["detail"].(map[string]any); pd["kind"] != "poll" || len(asList(pd["options"])) != 20 {
		t.Errorf("message page detail %v", msg["detail"])
	}
}

// What the detail names is resolved through the scope: a member sees
// only the people of its groups and no phone; an admin restricted from
// a group does not see it named.
func TestKindDetailsScoped(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	mem := kindsWindow(t, e, "member", "")
	if len(mem) != 22 {
		t.Fatalf("member: %d messages", len(mem))
	}
	if d := detailOf(t, mem, 4004); d["phone"] != nil || d["vcard"] != nil || d["user"] != nil ||
		d["first_name"] != "Bima" {
		t.Errorf("member contact %v", d)
	}
	tasks := asList(detailOf(t, mem, 4009)["tasks"])
	if t1, t2 := tasks[0].(map[string]any), tasks[1].(map[string]any); t1["done"] != true ||
		t1["done_by"] != nil || refID(t2["done_by"]) != dewi {
		t.Errorf("member tasks %v %v", t1, t2)
	}
	if d := detailOf(t, mem, 4010); d["poster"] != nil {
		t.Errorf("member story %v", d)
	}
	if d := detailOf(t, mem, 4012); fmt.Sprint(refIDs(d["chats"])) != fmt.Sprint([]int64{kindsLab}) ||
		num(d["chats_total"]) != 3 {
		t.Errorf("member giveaway %v", d)
	}
	if d := detailOf(t, mem, 4014); fmt.Sprint(refIDs(d["participants"])) != fmt.Sprint([]int64{dewi, andi}) ||
		num(d["participants_total"]) != 25 {
		t.Errorf("member group call %v", d)
	}
	// Nothing hidden is named anywhere in the member's window, but for
	// the contact card's own names, which are its content.
	delete(detailOf(t, mem, 4004), "first_name")
	delete(detailOf(t, mem, 4004), "last_name")
	text := js(mem)
	for _, hidden := range []string{"Bima", "Pratama", "C/C++", "628123456789", "9299000101"} {
		if strings.Contains(text, hidden) {
			t.Errorf("member window names %q", hidden)
		}
	}

	adm := kindsWindow(t, e, "admin", "")
	if d := detailOf(t, adm, 4004); d["phone"] != "+628123456789" || refID(d["user"]) != bima {
		t.Errorf("admin contact %v", d)
	}
	if d := detailOf(t, adm, 4010); d["poster"] != nil {
		t.Errorf("admin story %v", d)
	}
	if d := detailOf(t, adm, 4012); fmt.Sprint(refIDs(d["chats"])) != fmt.Sprint([]int64{kindsLab, kernelChat}) {
		t.Errorf("admin giveaway %v", d)
	}
	if strings.Contains(js(adm), "C/C++") {
		t.Error("admin window names the hidden group")
	}
}

// TestKindsDescribed: every key of every detail the fixture answers is
// a property of its schema in openapi.yaml, the one MessageDetail's
// discriminator maps its kind to.
func TestKindsDescribed(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseYAMLKeys(string(src))
	if err != nil {
		t.Fatal(err)
	}
	mapping := root.get("components", "schemas", "MessageDetail", "discriminator", "mapping")
	e := kindsEnv(t)
	var undescribed []string
	for row, m := range kindsWindow(t, e, "superadmin", "") {
		d, ok := m["detail"].(map[string]any)
		if !ok {
			continue
		}
		ref := mapping.get(fmt.Sprint(d["kind"]))
		if ref == nil {
			t.Errorf("row %d: kind %v not in the discriminator", row, d["kind"])
			continue
		}
		schema := strings.TrimPrefix(ref.value, "#/components/schemas/")
		props := specProps(t, root, schema)
		for k := range d {
			if !props[k] {
				undescribed = append(undescribed, schema+"."+k)
			}
		}
	}
	sort.Strings(undescribed)
	if len(undescribed) > 0 {
		t.Errorf("answered but not described: %s", strings.Join(slices.Compact(undescribed), ", "))
	}
}

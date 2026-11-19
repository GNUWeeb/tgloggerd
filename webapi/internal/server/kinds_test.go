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
	if d := detailOf(t, ms, 4017); len(d) != 1 || d["kind"] != "expired" {
		t.Errorf("row 4017: %v", d)
	}
	// A rich row stored before its mapping keeps only its class: no
	// page to draw (rich_test.go has the pages).
	if d := detailOf(t, ms, 4016); d["kind"] != "rich" || d["blocks"] != nil {
		t.Errorf("row 4016: %v", d)
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

// TestStoryUsername: a story's detail names its poster's active
// username, a user's or a chat's, from the batch that names the
// poster; null without one and for a poster the viewer may not see.
func TestStoryUsername(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "kinds", "story_posters")
	const storyLab = -1001000000041
	grantMember(t, e, []int64{storyLab})
	window := func(role string, chat int64) map[int]map[string]any {
		b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100", chat), nil)
		out := map[int]map[string]any{}
		for _, m := range asList(b["messages"]) {
			m := m.(map[string]any)
			out[int(m["id"].(float64))] = m
		}
		return out
	}
	ms := window("superadmin", storyLab)
	if d := detailOf(t, ms, 4101); refID(d["poster"]) != dewi || d["username"] != "dewikusuma" ||
		num(d["story_id"]) != 31 {
		t.Errorf("user's story %v", d)
	}
	if d := detailOf(t, ms, 4102); refID(d["poster"]) != kernelChat || d["username"] != "linuxkernelid" {
		t.Errorf("chat's story %v", d)
	}
	// A poster without a username.
	if d, ok := detailOf(t, window("superadmin", kindsLab), 4010)["username"]; !ok || d != nil {
		t.Errorf("story without username: %v %v", d, ok)
	}
	// The member sees Story Lab but neither poster.
	mem := window("member", storyLab)
	for _, row := range []int{4101, 4102} {
		if d := detailOf(t, mem, row); d["poster"] != nil || d["username"] != nil {
			t.Errorf("member story %d: %v", row, d)
		}
	}
	if text := js(mem); strings.Contains(text, "dewikusuma") || strings.Contains(text, "linuxkernelid") {
		t.Error("member window names a hidden poster")
	}
	// The message page carries the same detail.
	b := e.want(200, "superadmin", "GET", "/api/page/messages/group/4101", nil)
	msg := b["props"].(map[string]any)["message"].(map[string]any)
	if d, _ := msg["detail"].(map[string]any); d["username"] != "dewikusuma" {
		t.Errorf("message page story %v", msg["detail"])
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

// editsOf is a message's earlier versions.
func editsOf(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, e := range asList(m["edits"]) {
		out = append(out, e.(map[string]any))
	}
	return out
}

// TestEditDetails: each earlier version of a message has its own
// detail from its content_extra, in the chat window and on the message
// page: a live location's earlier positions, how many came before
// each, a checklist's earlier tasks with who did them as the viewer
// may see them; a poll's versions have none, as the poll tables keep
// only the latest results.
func TestEditDetails(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	ms := kindsWindow(t, e, "superadmin", "")
	for name, m := range map[string]map[string]any{"window": ms[4002],
		"page": pageMessage(t, e, "superadmin", "group", 4002)} {
		eds := editsOf(t, m)
		if len(eds) != 3 {
			t.Fatalf("%s: live location edits %v", name, m["edits"])
		}
		for i, want := range [][2]float64{{-6.2, 106.81}, {-6.205, 106.815}, {-6.208, 106.818}} {
			d, _ := eds[i]["detail"].(map[string]any)
			live, _ := d["live"].(map[string]any)
			if eds[i]["content_type"] != "location" || d["kind"] != "location" ||
				d["lat"] != want[0] || d["lon"] != want[1] || live == nil ||
				num(live["period"]) != 900 || num(live["updates"]) != int64(i) {
				t.Errorf("%s: version %d at %v: %v", name, i, eds[i]["at"], eds[i]["detail"])
			}
		}
		if eds[0]["at"] != "2026-11-17T15:02:00Z" || eds[1]["at"] != "2026-11-17T15:03:00Z" {
			t.Errorf("%s: version times %v %v", name, eds[0]["at"], eds[1]["at"])
		}
	}

	eds := editsOf(t, ms[4009])
	if len(eds) != 1 {
		t.Fatalf("checklist edits %v", ms[4009]["edits"])
	}
	d, _ := eds[0]["detail"].(map[string]any)
	tasks := asList(d["tasks"])
	if d["kind"] != "checklist" || len(tasks) != 2 || num(d["tasks_total"]) != 2 ||
		d["others_can_add"] != false || eds[0]["text"] != "Release checklist" {
		t.Fatalf("checklist version %v", eds[0])
	}
	if t1, t2 := tasks[0].(map[string]any), tasks[1].(map[string]any); t1["text"] != "Task 1" ||
		t1["done"] != true || refID(t1["done_by"]) != bima || t2["done"] != false {
		t.Errorf("checklist version tasks %v %v", t1, t2)
	}
	// The message itself keeps its latest detail.
	if num(detailOf(t, ms, 4009)["tasks_total"]) != 22 {
		t.Errorf("checklist %v", ms[4009]["detail"])
	}

	eds = editsOf(t, ms[4008])
	if len(eds) != 1 || eds[0]["text"] != "Pick" || eds[0]["content_type"] != "poll" {
		t.Fatalf("poll edits %v", ms[4008]["edits"])
	}
	if v, ok := eds[0]["detail"]; !ok || v != nil {
		t.Errorf("poll version detail %v %v", v, ok)
	}
	if detailOf(t, ms, 4008)["kind"] != "poll" {
		t.Errorf("poll %v", ms[4008]["detail"])
	}

	// A member does not see Bima, so the version names nobody for
	// the task he did.
	for name, m := range map[string]map[string]any{"window": kindsWindow(t, e, "member", "")[4009],
		"page": pageMessage(t, e, "member", "group", 4009)} {
		eds := editsOf(t, m)
		if len(eds) != 1 {
			t.Fatalf("member %s: checklist edits %v", name, m["edits"])
		}
		d, _ := eds[0]["detail"].(map[string]any)
		tasks := asList(d["tasks"])
		if len(tasks) != 2 || tasks[0].(map[string]any)["done"] != true ||
			tasks[0].(map[string]any)["done_by"] != nil {
			t.Errorf("member %s: checklist version %v", name, d)
		}
	}
}

// TestKindsDescribed: every key of every detail the fixture answers,
// a message's or an earlier version's, is a property of its schema in
// openapi.yaml, the one MessageDetail's discriminator maps its kind
// to.
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
	var details []map[string]any
	var rows []int
	for row, m := range kindsWindow(t, e, "superadmin", "") {
		if d, ok := m["detail"].(map[string]any); ok {
			details, rows = append(details, d), append(rows, row)
		}
		// The earlier versions' details too.
		for _, ed := range editsOf(t, m) {
			if d, ok := ed["detail"].(map[string]any); ok {
				details, rows = append(details, d), append(rows, row)
			}
		}
	}
	for i, d := range details {
		row := rows[i]
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

// serviceOf is a service message's text, refs' user and chat ids and
// message id.
func serviceOf(t *testing.T, ms map[int]map[string]any, row int) (text string, users, chats []int64, msg any) {
	t.Helper()
	m := ms[row]
	s, ok := m["service"].(map[string]any)
	if !ok {
		t.Fatalf("row %d: service %v", row, m["service"])
	}
	refs := s["refs"].(map[string]any)
	return fmt.Sprint(m["text"]), refIDs(refs["users"]), refIDs(refs["chats"]), refs["message_id"]
}

// A service message's text names only whom the viewer may see, and its
// refs list only them: a superadmin sees everyone named, an admin
// every user but not the group hidden from it, a member only the
// people of Kinds Lab. A row logged before migration 000034 keeps its
// stored text, which names nobody.
func TestServiceRefs(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	type want struct {
		text         string
		users, chats []int64
	}
	cases := map[string]map[int]want{
		"superadmin": {
			4018: {"added Andi Santoso and Bima Pratama to the group", []int64{andi, bima}, []int64{}},
			4019: {"announced the winners of the giveaway in C/C++ Study Group: Bima Pratama and Dewi Kusuma",
				[]int64{bima, dewi}, []int64{cppGroup}},
			4022: {"removed Bima Pratama from the group", []int64{bima}, []int64{}},
		},
		"admin": {
			4018: {"added Andi Santoso and Bima Pratama to the group", []int64{andi, bima}, []int64{}},
			4019: {"announced the giveaway winners: Bima Pratama and Dewi Kusuma", []int64{bima, dewi}, []int64{}},
			4022: {"removed Bima Pratama from the group", []int64{bima}, []int64{}},
		},
		"member": {
			4018: {"added Andi Santoso and 1 other member to the group", []int64{andi}, []int64{}},
			4019: {"announced the giveaway winners: Dewi Kusuma and 1 other winner", []int64{dewi}, []int64{}},
			4022: {"removed a member from the group", []int64{}, []int64{}},
		},
	}
	for role, rows := range cases {
		ms := kindsWindow(t, e, role, "")
		for row, w := range rows {
			text, users, chats, msg := serviceOf(t, ms, row)
			if text != w.text || fmt.Sprint(users) != fmt.Sprint(w.users) ||
				fmt.Sprint(chats) != fmt.Sprint(w.chats) {
				t.Errorf("%s row %d: %q %v %v, want %q %v %v", role, row, text, users, chats,
					w.text, w.users, w.chats)
			}
			if row == 4019 && num(msg) != 700012 {
				t.Errorf("%s row %d: message_id %v", role, row, msg)
			}
		}
		// The pinned message, for a link to it.
		if _, _, _, msg := serviceOf(t, ms, 4020); num(msg) != 700003 {
			t.Errorf("%s pin: message_id %v", role, msg)
		}
		// Logged before migration 000034: as stored, no refs.
		text, users, chats, msg := serviceOf(t, ms, 4021)
		if text != "added a member to the group" || len(users) != 0 || len(chats) != 0 || msg != nil {
			t.Errorf("%s old row: %q %v %v %v", role, text, users, chats, msg)
		}
		if ms[4021]["service"].(map[string]any)["type"] != "chat_add_members" {
			t.Errorf("%s old row type %v", role, ms[4021]["service"])
		}
		// Other kinds have no service.
		if s, ok := ms[4001]["service"]; !ok || s != nil {
			t.Errorf("%s location service %v", role, s)
		}
	}
	// The member's window names neither Bima nor the hidden group.
	mem := kindsWindow(t, e, "member", "")
	for _, row := range []int{4018, 4019, 4022} {
		if s := js(mem[row]); strings.Contains(s, "Bima") || strings.Contains(s, "C/C++") {
			t.Errorf("member row %d names a hidden one: %s", row, s)
		}
	}
	// The message page rebuilds the text the same way.
	for role, want := range map[string]string{"superadmin": "removed Bima Pratama from the group",
		"member": "removed a member from the group"} {
		b := e.want(200, role, "GET", "/api/page/messages/group/4022", nil)
		msg := b["props"].(map[string]any)["message"].(map[string]any)
		if msg["text"] != want || msg["snippet"] != want {
			t.Errorf("%s message page: %v / %v", role, msg["text"], msg["snippet"])
		}
	}
}

// The interaction totals and the pinned state, and a forum's topics:
// the chat page lists them, and ?topic= keeps the chat window and the
// page to one topic's messages, with has_older and has_newer within it.
func TestKindsInteractionAndTopics(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	ms := kindsWindow(t, e, "superadmin", "")
	m1 := ms[4001]
	rs := asList(m1["reactions"])
	if num(m1["views"]) != 120 || num(m1["forwards"]) != 3 || num(m1["replies"]) != 2 ||
		m1["pinned"] != false || num(m1["topic_id"]) != 5 || len(rs) != 3 {
		t.Fatalf("message 1 %v", m1)
	}
	if js(rs) != js([]map[string]any{{"count": 5, "emoji": "\U0001F44D"},
		{"count": 2, "custom_emoji_id": "5368324170671202286"}, {"count": 1, "paid": true}}) {
		t.Errorf("reactions %s", js(rs))
	}
	// A group's own message has no view or forward count, even when 0
	// is stored: Telegram counts them for channel posts and forwards
	// from channels only (message 1).
	if m := ms[4002]; m["views"] != nil || m["forwards"] != nil || len(asList(m["reactions"])) != 0 {
		t.Errorf("message 2 %v", m)
	}
	if ms[4003]["pinned"] != true {
		t.Errorf("message 3 pinned %v", ms[4003]["pinned"])
	}
	// No topic, a thread's, and a row without interaction.
	for _, row := range []int{4007, 4021} {
		m := ms[row]
		if m["topic_id"] != nil || m["views"] != nil || m["forwards"] != nil || m["replies"] != nil ||
			m["pinned"] != false || len(asList(m["reactions"])) != 0 {
			t.Errorf("row %d %v", row, m)
		}
	}

	// The window of one topic.
	in := kindsWindow(t, e, "superadmin", "&topic=5")
	if len(in) != 5 {
		t.Errorf("topic 5: %d messages", len(in))
	}
	for row := range in {
		if row < 4001 || row > 4005 {
			t.Errorf("topic 5 has row %d", row)
		}
	}
	b := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?topic=5&limit=2&before=%d",
		kindsLab, 700004), nil)
	if ids := asList(b["messages"]); len(ids) != 2 || b["has_older"] != true || b["has_newer"] != true {
		t.Errorf("topic 5 before 4: %v", b)
	}
	b = e.want(200, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?topic=5&limit=2&before=%d",
		kindsLab, 700003), nil)
	if ids := asList(b["messages"]); len(ids) != 2 || b["has_older"] != false {
		t.Errorf("topic 5 before 3: %v", b)
	}
	if b := e.want(200, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?topic=9",
		kindsLab), nil); len(asList(b["messages"])) != 0 || b["has_older"] != false {
		t.Errorf("topic 9: %v", b)
	}
	for _, q := range []string{"topic=x", "topic=0", "topic=-1"} {
		e.want(400, "superadmin", "GET", fmt.Sprintf("/api/chats/group/%d/messages?%s", kindsLab, q), nil)
	}
	e.want(400, "superadmin", "GET", "/api/chats/private/9299000002/messages?topic=5", nil)

	// The chat page: the topics, pinned first, then the newest, and
	// the topic it is limited to.
	_, p := e.page("superadmin", fmt.Sprintf("/groups/%d/chat?topic=5", kindsLab))
	props := p["props"].(map[string]any)
	if num(props["topic"]) != 5 || len(asList(props["messages"])) != 5 {
		t.Errorf("page topic %v, %d messages", props["topic"], len(asList(props["messages"])))
	}
	want := `[{"closed":false,"hidden":false,"icon_color":7322096,"id":5,"name":"Releases"},` +
		`{"closed":true,"hidden":false,"icon_color":16749490,"id":9,"name":"Off-topic"},` +
		`{"closed":false,"hidden":true,"icon_color":0,"id":1,"name":"General"}]`
	if got := js(props["topics"]); got != want {
		t.Errorf("topics %s", got)
	}
	_, p = e.page("superadmin", fmt.Sprintf("/groups/%d/chat", kindsLab))
	if props := p["props"].(map[string]any); props["topic"] != nil || len(asList(props["messages"])) != 22 {
		t.Errorf("page without topic: %v", props["topic"])
	}
	// Another group has no topics.
	_, p = e.page("superadmin", fmt.Sprintf("/groups/%d/chat", kernelChat))
	if ts := p["props"].(map[string]any)["topics"]; ts == nil || len(asList(ts)) != 0 {
		t.Errorf("kernel topics %v", ts)
	}
}

// The search takes the new content types, and tdlib_type and
// service_type within one chat.
func TestKindsSearch(t *testing.T) {
	t.Parallel()
	e := kindsEnv(t)
	chat := fmt.Sprintf(`{"c":"chat_id","o":"=","v":"%d","n":"AND"}`, kindsLab)
	for cond, want := range map[string]int64{
		`{"c":"content_type","o":"=","v":"poll"}`:                 1,
		`{"c":"content_type","o":"=","v":"call"}`:                 2,
		`{"c":"content_type","o":"=","v":"service"}`:              5,
		`{"c":"service_type","o":"=","v":"pin_message"}`:          1,
		`{"c":"service_type","o":"=","v":"chat_add_members"}`:     2,
		`{"c":"tdlib_type","o":"=","v":"messageStakeDice"}`:       1,
		`{"c":"tdlib_type","o":"=","v":"messageGroupCall"}`:       1,
		`{"c":"tdlib_type","o":"!=","v":"messageChatAddMembers"}`: 20, // NULL is no match
		`{"c":"content_type","o":"=","v":"location","n":"AND"},` +
			`{"c":"tdlib_type","o":"=","v":"messageLiveLocation"}`: 1,
	} {
		for _, role := range []string{"superadmin", "member"} {
			b := e.want(200, role, "GET", search("group_messages", "["+chat+","+cond+"]", "limit", "50"), nil)
			if num(b["total"]) != want || len(rows(b)) != int(want) {
				t.Errorf("%s %s: %v rows, total %v, want %d", role, cond, len(rows(b)), b["total"], want)
			}
		}
	}
	// The new content types need no chat.
	b := e.want(200, "superadmin", "GET", search("group_messages", `[{"c":"content_type","o":"=","v":"expired"}]`), nil)
	if num(b["total"]) != 1 {
		t.Errorf("expired: %v", b["total"])
	}
	for _, conds := range []string{
		`[{"c":"service_type","o":"=","v":"pin_message"}]`,
		`[{"c":"chat_id","o":"=","v":"-1001000000040","n":"OR"},{"c":"service_type","o":"=","v":"pin_message"}]`,
	} {
		if st, b, _ := e.call("superadmin", "GET", search("group_messages", conds), nil); st != 400 ||
			b["error"] != "bad_search" {
			t.Errorf("%s: %d %v", conds, st, b)
		}
	}
	// The fields say which need a chat.
	b = e.want(200, "superadmin", "GET", search("group_messages", ""), nil)
	needs := []string{}
	for _, f := range asList(b["fields"]) {
		if f := f.(map[string]any); f["needs_chat"] == true {
			needs = append(needs, f["key"].(string))
		}
	}
	if fmt.Sprint(needs) != "[tdlib_type service_type]" {
		t.Errorf("needs_chat %v", needs)
	}
}

// A poll past its close date is closed, even when the update closing it
// never came and the stored flag still says open.
func TestPollClosed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		closed         bool
		closeDate, now int64
		want           bool
	}{
		{true, 0, 100, true},
		{false, 0, 100, false},
		{false, 200, 100, false},
		{false, 100, 100, true},
		{false, 50, 100, true},
	} {
		if got := pollClosed(c.closed, c.closeDate, c.now); got != c.want {
			t.Errorf("pollClosed(%v, %d, %d) = %v, want %v", c.closed, c.closeDate, c.now, got, c.want)
		}
	}
}

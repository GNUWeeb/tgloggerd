// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const serviceLab = int64(-1001000000042)

// serviceDetailEnv is the "service_detail" fixture with a member
// granted Service Lab alone and an admin that hides C/C++ Study Group.
func serviceDetailEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t, "service_detail")
	grantMember(t, e, []int64{serviceLab})
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) "+
		"VALUES (3, ?)", cppGroup); err != nil {
		t.Fatal(err)
	}
	return e
}

// serviceDetails are Service Lab's service details as role sees them,
// by row id, as JSON; a row without one is "".
func serviceDetails(t *testing.T, e *adminEnv, role string) map[int]string {
	t.Helper()
	b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100",
		serviceLab), nil)
	out := map[int]string{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		row := int(m["id"].(float64))
		s, ok := m["service"].(map[string]any)
		if !ok {
			continue
		}
		d, ok := s["detail"]
		if !ok {
			out[row] = ""
			continue
		}
		if d == nil {
			t.Errorf("%s row %d: detail is null, not absent", role, row)
		}
		raw, _ := json.Marshal(d)
		out[row] = string(raw)
	}
	return out
}

// Each type's detail, as each viewer may see it: the community and the
// story's poster in the pinned snippet are named only to a viewer that
// sees C/C++ Study Group, the basic group linked only to one that sees
// it, and a pin of a message the archive lacks has no snippet.
func TestServiceDetail(t *testing.T) {
	t.Parallel()
	e := serviceDetailEnv(t)
	photo := func(w int) func(string) bool {
		return func(d string) bool {
			var j struct {
				Kind  string `json:"kind"`
				Photo *struct {
					Render string `json:"render"`
					W      int    `json:"w"`
				} `json:"photo"`
			}
			return json.Unmarshal([]byte(d), &j) == nil && j.Kind == "photo" && j.Photo != nil &&
				j.Photo.Render == "image" && j.Photo.W == w
		}
	}
	// is compares as JSON: the detail went through a map.
	is := func(want string) func(string) bool {
		return func(d string) bool {
			if want == "" || d == "" {
				return d == want
			}
			var a, b any
			return json.Unmarshal([]byte(d), &a) == nil &&
				json.Unmarshal([]byte(want), &b) == nil && reflect.DeepEqual(a, b)
		}
	}
	// bg compares a background's detail without its image, which
	// has render image (stored, with its URL), render file (the
	// pattern's TGV, not stored) or is null (render "").
	bg := func(want, render string) func(string) bool {
		return func(d string) bool {
			var a map[string]any
			if json.Unmarshal([]byte(d), &a) != nil {
				return false
			}
			img, _ := a["image"].(map[string]any)
			delete(a, "image")
			switch {
			case render == "":
				if img != nil {
					return false
				}
			case img == nil || img["render"] != render:
				return false
			case render == "image" && (img["stored"] != true || img["url"] == nil ||
				img["thumb"] != img["url"]):
				return false
			case render == "file" && (img["stored"] != false || img["ext"] != "tgv"):
				return false
			}
			var b any
			return json.Unmarshal([]byte(want), &b) == nil && reflect.DeepEqual(any(a), b)
		}
	}
	const none = `"type":null,"dark":false,"image":null,"fill":null,"intensity":null,` +
		`"is_inverted":false,"is_moving":false,"is_blurred":false,"theme_name":null`
	common := map[int]func(string) bool{
		4201: photo(640),
		4202: is(`{"kind":"video_chat","duration":3725,"start_date":null}`),
		4203: is(`{"kind":"video_chat","duration":null,"start_date":"2026-11-18T11:06:40Z"}`),
		4205: is(`{"kind":"pin","snippet":"Release notes for 1.0 are out"}`),
		4208: is(""),
		4209: is(`{"kind":"poll_option","text":"Option C"}`),
		4210: is(`{"kind":"poll_option","text":"Option B"}`),
		4211: is(`{"kind":"topic","topic":{"id":11,"name":"Releases","icon_color":7322096,"icon_custom_emoji_id":null,"icon_custom_emoji":null}}`),
		4212: is(`{"kind":"topic","topic":{"id":11,"name":"Releases 2","icon_color":16749490,"icon_custom_emoji_id":null,"icon_custom_emoji":null}}`),
		4214: is(""),
		4215: is(""),
		4216: is(`{"kind":"title","title":"Service Lab","basic_group":null}`),
		4218: is(`{"kind":"background","only_for_self":true,` + none + `}`),
		4221: bg(`{"kind":"background","only_for_self":false,"type":"wallpaper","dark":false,`+
			`"fill":null,"intensity":null,"is_inverted":false,"is_moving":false,"is_blurred":true,`+
			`"theme_name":null}`, "image"),
		4222: bg(`{"kind":"background","only_for_self":false,"type":"pattern","dark":true,`+
			`"fill":{"type":"gradient","colors":[16744448,255],"rotation_angle":45},"intensity":50,`+
			`"is_inverted":true,"is_moving":true,"is_blurred":false,"theme_name":null}`, "file"),
		4223: bg(`{"kind":"background","only_for_self":false,"type":"fill","dark":false,`+
			`"fill":{"type":"freeform_gradient","colors":[14544639,8388863,16777215,0],`+
			`"rotation_angle":0},"intensity":null,"is_inverted":false,"is_moving":false,`+
			`"is_blurred":false,"theme_name":null}`, ""),
		4224: bg(`{"kind":"background","only_for_self":true,"type":"chat_theme","dark":false,`+
			`"fill":null,"intensity":null,"is_inverted":false,"is_moving":false,"is_blurred":false,`+
			`"theme_name":"\ud83c\udf3f"}`, ""),
		4225: photo(640),
		// Without its detail, the photo has no size.
		4219: photo(0),
		4220: photo(320),
	}
	basic := `{"kind":"title","title":"Service Lab (basic)","basic_group":{"id":-4242001,` +
		`"title":"Service Lab (basic)","kind":"group","photo":null}}`
	cases := map[string]map[int]func(string) bool{
		"superadmin": {
			4207: is(`{"kind":"pin","snippet":"Story by C/C++ Study Group"}`),
			4213: is(`{"kind":"community","community":{"id":-1001000000004,"title":"C/C++ Study Group",` +
				`"kind":"group","photo":null}}`),
			4217: is(basic),
		},
		"admin": {
			4207: is(`{"kind":"pin","snippet":"Story"}`),
			4213: is(""),
			4217: is(basic),
		},
		"member": {
			4207: is(`{"kind":"pin","snippet":"Story"}`),
			4213: is(""),
			4217: is(`{"kind":"title","title":"Service Lab (basic)","basic_group":null}`),
		},
	}
	for role, own := range cases {
		got := serviceDetails(t, e, role)
		if len(got) != 23 {
			t.Errorf("%s: %d service messages", role, len(got))
		}
		for row, ok := range common {
			if !ok(got[row]) {
				t.Errorf("%s row %d: detail %s", role, row, got[row])
			}
		}
		for row, ok := range own {
			if !ok(got[row]) {
				t.Errorf("%s row %d: detail %s", role, row, got[row])
			}
		}
		if role != "superadmin" {
			for row, d := range got {
				if strings.Contains(d, "1001000000004") || strings.Contains(d, "C/C++") {
					t.Errorf("%s row %d names the hidden group: %s", role, row, d)
				}
			}
		}
	}
	// The photo's file goes out as any media's: stored, it has its URL.
	var j struct {
		Photo map[string]any `json:"photo"`
	}
	_ = json.Unmarshal([]byte(serviceDetails(t, e, "member")[4201]), &j)
	if u, _ := j.Photo["url"].(string); !strings.HasPrefix(u, "/files/") {
		t.Errorf("photo %v", j.Photo)
	}
	// A still photo has no animation; an animated one's video plays
	// as an animation the photo's size, the still photo its poster.
	var anim struct {
		Animation map[string]any `json:"animation"`
	}
	if _ = json.Unmarshal([]byte(serviceDetails(t, e, "member")[4201]), &anim); anim.Animation != nil {
		t.Errorf("still photo's animation %v", anim.Animation)
	}
	_ = json.Unmarshal([]byte(serviceDetails(t, e, "member")[4225]), &anim)
	if a := anim.Animation; a == nil || a["render"] != "animation" || a["stored"] != true ||
		a["w"] != float64(640) || a["poster"] != j.Photo["url"] || a["url"] == nil ||
		a["url"] == a["poster"] {
		t.Errorf("animated photo's animation %v (photo %v)", anim.Animation, j.Photo)
	}
	// The message page sends the same detail.
	for row, key := range map[int]string{4221: "image", 4225: "animation"} {
		b := e.want(200, "member", "GET", fmt.Sprintf("/api/page/messages/group/%d", row), nil)
		msg := b["props"].(map[string]any)["message"].(map[string]any)
		d, _ := msg["service"].(map[string]any)["detail"].(map[string]any)
		if f, _ := d[key].(map[string]any); f == nil || f["stored"] != true {
			t.Errorf("message page %d: detail %v", row, d)
		}
	}
	for role, want := range map[string]any{
		"superadmin": map[string]any{"kind": "pin", "snippet": "Story by C/C++ Study Group"},
		"member":     map[string]any{"kind": "pin", "snippet": "Story"},
	} {
		b := e.want(200, role, "GET", "/api/page/messages/group/4207", nil)
		msg := b["props"].(map[string]any)["message"].(map[string]any)
		d, _ := msg["service"].(map[string]any)["detail"].(map[string]any)
		if !reflect.DeepEqual(d, want) {
			t.Errorf("%s message page: detail %v", role, d)
		}
	}
}

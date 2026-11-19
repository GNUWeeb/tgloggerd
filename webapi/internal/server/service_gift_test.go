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

const giftLab = int64(-1001000000043)

// giftEnv is the "gifts" fixture with a member granted Gift Lab alone
// and an admin that hides C/C++ Study Group.
func giftEnv(t *testing.T) *adminEnv {
	t.Helper()
	e := newAdminEnv(t, "gifts")
	grantMember(t, e, []int64{giftLab})
	if _, err := e.app.Exec("INSERT INTO web_admin_hidden_groups (web_user_id, group_id) "+
		"VALUES (3, ?)", cppGroup); err != nil {
		t.Fatal(err)
	}
	return e
}

// giftDetails are Gift Lab's message details as role sees them, by
// row id, decoded: a service message's ServiceInfo.detail, another's
// detail; nil for none.
func giftDetails(t *testing.T, e *adminEnv, role string) map[int]map[string]any {
	t.Helper()
	b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=100", giftLab), nil)
	out := map[int]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		d := m["detail"]
		if s, ok := m["service"].(map[string]any); ok {
			d = s["detail"]
		}
		dm, _ := d.(map[string]any)
		out[int(m["id"].(float64))] = dm
	}
	return out
}

// jsonOf is v as compact JSON, to compare with a literal.
func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// sameJSON is whether a and b (JSON texts) hold the same value.
func sameJSON(a, b string) bool {
	var x, y any
	return json.Unmarshal([]byte(a), &x) == nil && json.Unmarshal([]byte(b), &y) == nil &&
		reflect.DeepEqual(x, y)
}

// The gift detail of each type as each viewer may see it: the
// stickers by position, the upgraded gift's look and original details,
// prices, and the people and chats only as far as the scope lets the
// viewer see them (the owner, C/C++ Study Group, to a superadmin alone;
// Andi, who never writes in Gift Lab, not to the member).
func TestServiceGiftDetail(t *testing.T) {
	t.Parallel()
	e := giftEnv(t)
	const (
		dewi = `{"id":9299000001,"kind":"user","title":"Dewi Kusuma","photo":"*"}`
		bima = `{"id":9299000002,"kind":"user","title":"Bima Pratama","photo":null}`
		andi = `{"id":9299000004,"kind":"user","title":"Andi Santoso","photo":null}`
		cpp  = `{"id":-1001000000004,"kind":"group","title":"C/C++ Study Group","photo":null}`
	)
	// ref replaces a ref's photo URL, which holds a token, by "*".
	ref := func(v any) any {
		if m, ok := v.(map[string]any); ok {
			if p, ok := m["photo"].(string); ok && p != "" {
				m["photo"] = "*"
			}
		}
		return v
	}
	for role, who := range map[string]struct{ owner, origSender string }{
		"superadmin": {cpp, andi},
		"admin":      {"null", andi},
		"member":     {"null", "null"},
	} {
		got := giftDetails(t, e, role)
		check := func(row int, key, want string) {
			t.Helper()
			d := got[row]
			if d == nil {
				t.Errorf("%s row %d: no detail", role, row)
				return
			}
			if s := jsonOf(d[key]); !sameJSON(s, want) {
				t.Errorf("%s row %d: %s = %s, want %s", role, row, key, s, want)
			}
		}
		sticker := func(row int, key string, render string, stored bool) {
			t.Helper()
			s, _ := got[row][key].(map[string]any)
			if s == nil || s["render"] != render || s["stored"] != stored {
				t.Errorf("%s row %d: %s = %v, want render %s stored %v", role, row, key, s, render, stored)
				return
			}
			if u, _ := s["url"].(string); stored && !strings.HasPrefix(u, "/files/") {
				t.Errorf("%s row %d: %s url %q", role, row, key, u)
			}
		}

		// A regular gift: its sticker with the gift's emoji, price,
		// giver and receiver, and text.
		for _, d := range got {
			for _, k := range []string{"sender", "receiver"} {
				ref(d[k])
			}
			if u, ok := d["upgraded"].(map[string]any); ok {
				ref(u["owner"])
				if o, ok := u["original_details"].(map[string]any); ok {
					ref(o["sender"])
					ref(o["receiver"])
				}
			}
		}
		check(4401, "kind", `"gift"`)
		sticker(4401, "sticker", "lottie", true)
		if s, _ := got[4401]["sticker"].(map[string]any); s != nil && s["emoji"] != "❤" {
			t.Errorf("%s row 4401: sticker emoji %v", role, s["emoji"])
		}
		check(4401, "symbol_sticker", "null")
		check(4401, "star_count", "50")
		check(4401, "price", "null")
		check(4401, "sender", bima)
		check(4401, "receiver", dewi)
		check(4401, "text", `"Happy birthday"`)
		check(4401, "entities", `[]`)
		check(4401, "upgraded", "null")

		// An upgraded gift: the model sticker at 0, the symbol's at 1
		// (not stored, so fetchable at most), and its look.
		sticker(4402, "sticker", "lottie", true)
		sticker(4402, "symbol_sticker", "lottie", false)
		check(4402, "star_count", "null")
		check(4402, "sender", dewi)
		check(4402, "receiver", bima)
		check(4402, "upgraded", `{"id":"77","regular_gift_id":"5170145012310081615",`+
			`"title":"Plush Pepe","name":"PlushPepe-42","number":42,"owner":`+who.owner+`,`+
			`"model":{"name":"Pumpkin","rarity_per_mille":15,"rarity":null,"crafted":true},`+
			`"symbol":{"name":"Star","rarity_per_mille":null,"rarity":"epic"},`+
			`"backdrop":{"name":"Onyx Black","center_color":0,"edge_color":3355443,`+
			`"symbol_color":16777215,"text_color":16711680,"rarity_per_mille":0,"rarity":null},`+
			`"original_details":{"sender":`+who.origSender+`,"receiver":`+dewi+`,`+
			`"text":"For you","entities":[],"date":"2026-11-17T07:20:00Z"}}`)

		// A purchase offer: its price; the gift without its look.
		check(4403, "sticker", "null")
		check(4403, "price", `{"gram_cents":1250}`)
		check(4403, "upgraded", `{"id":"78","regular_gift_id":null,"title":"Plush Pepe",`+
			`"name":"PlushPepe-43","number":43,"owner":null,"model":null,"symbol":null,`+
			`"backdrop":null,"original_details":null}`)

		// Stars given, and won: the animation sticker and the Stars.
		sticker(4404, "sticker", "lottie", true)
		check(4404, "star_count", "500")
		check(4404, "sender", bima)
		check(4404, "receiver", dewi)
		check(4405, "sticker", "null")
		check(4405, "star_count", "1000")
		check(4405, "sender", "null")

		// A refunded private gift: no sender.
		check(4412, "star_count", "25")
		check(4412, "sender", "null")
		check(4412, "receiver", dewi)

		// Chat themes: a gift's, an emoji's, none.
		if d := jsonOf(got[4406]); !sameJSON(d,
			`{"kind":"theme","theme_name":null,"gift_id":"77","gift_name":"PlushPepe-42"}`) {
			t.Errorf("%s row 4406: %s", role, d)
		}
		if d := jsonOf(got[4407]); !sameJSON(d,
			`{"kind":"theme","theme_name":"❤","gift_id":null,"gift_name":null}`) {
			t.Errorf("%s row 4407: %s", role, d)
		}
		// Nothing stored: no detail.
		for _, row := range []int{4408, 4409} {
			if got[row] != nil {
				t.Errorf("%s row %d: detail %v", role, row, got[row])
			}
		}

		// A giveaway's prize sticker.
		check(4410, "kind", `"giveaway"`)
		sticker(4410, "sticker", "animation", true)

		if role == "member" {
			for row, d := range got {
				if s := jsonOf(d); strings.Contains(s, "9299000004") ||
					strings.Contains(s, "1001000000004") {
					t.Errorf("member row %d names a hidden one: %s", row, s)
				}
			}
		}
	}

	// The message page sends the same detail.
	b := e.want(200, "member", "GET", "/api/page/messages/group/4402", nil)
	msg := b["props"].(map[string]any)["message"].(map[string]any)
	d, _ := msg["service"].(map[string]any)["detail"].(map[string]any)
	if u, _ := d["upgraded"].(map[string]any); u == nil || u["name"] != "PlushPepe-42" ||
		d["symbol_sticker"] == nil {
		t.Errorf("member message page: detail %v", d)
	}
}

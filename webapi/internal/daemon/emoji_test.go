// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// POST /v1/custom-emoji and /v1/message-effects against a fake daemon:
// the ids sent as strings, the rows and missing ids as answered, and
// every answer that is not one (off, away, slow, refusing, malformed)
// as no answer.
func TestEmojiLookups(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	status, answer := 200, ""
	var asked []string
	delay := time.Duration(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		st, a, d := status, answer, delay
		asked = append(asked, r.Method+" "+r.URL.RequestURI()+" "+string(b))
		mu.Unlock()
		if want, _ := Bearer(testKey); r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		time.Sleep(d)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		io.WriteString(w, a)
	}))
	defer srv.Close()
	set := func(st int, a string) {
		mu.Lock()
		defer mu.Unlock()
		status, answer = st, a
	}
	c, err := New(srv.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	set(200, `{"outcome":"ok","message":"","asked":2,"emoji":[`+
		`{"custom_emoji_id":"5368324170671202286","emoji":"❤","set_id":"773947703670341676",`+
		`"format":"tgs","needs_repainting":false,"file_id":123456},`+
		`{"custom_emoji_id":"5370870893004203704","emoji":"x","set_id":null,`+
		`"format":null,"needs_repainting":true,"file_id":null}],`+
		`"missing":["5370870893004203705"]}`)
	l, ok, _ := c.CustomEmojiStickers(ctx, []int64{5368324170671202286, 5370870893004203704,
		5370870893004203705})
	want := EmojiLookup{Outcome: EmojiOK, Asked: 2, Emoji: []CustomEmoji{
		{ID: 5368324170671202286, Emoji: "❤", SetID: 773947703670341676,
			Format: "tgs", FileID: 123456},
		{ID: 5370870893004203704, Emoji: "x", NeedsRepainting: true},
	}, Missing: []int64{5370870893004203705}}
	if !ok || !reflect.DeepEqual(l, want) {
		t.Errorf("custom emoji = %+v %v", l, ok)
	}

	set(200, `{"outcome":"ok","message":"","asked":0,"effects":[`+
		`{"effect_id":"5104841245755180586","emoji":"f","is_premium":false,"type":"emoji_reaction",`+
		`"static_icon_file_id":1,"select_animation_file_id":2,"effect_animation_file_id":3,`+
		`"sticker_file_id":null,"sticker_format":null},`+
		`{"effect_id":"5046509860389126442","emoji":"p","is_premium":true,"type":"premium_sticker",`+
		`"static_icon_file_id":null,"select_animation_file_id":null,"effect_animation_file_id":5,`+
		`"sticker_file_id":4,"sticker_format":"webm"}],"missing":[]}`)
	l, ok, _ = c.MessageEffects(ctx, []int64{5104841245755180586, 5046509860389126442})
	wantEff := EmojiLookup{Outcome: EmojiOK, Effects: []MessageEffect{
		{ID: 5104841245755180586, Emoji: "f", Type: "emoji_reaction",
			StaticIconFileID: 1, SelectAnimationFileID: 2, EffectAnimationFileID: 3},
		{ID: 5046509860389126442, Emoji: "p", IsPremium: true, Type: "premium_sticker",
			EffectAnimationFileID: 5, StickerFileID: 4, StickerFormat: "webm"},
	}}
	if !ok || !reflect.DeepEqual(l, wantEff) {
		t.Errorf("effects = %+v %v", l, ok)
	}

	// A failed call to Telegram is an answer, with the stored rows.
	set(200, `{"outcome":"flood_wait","message":"","asked":1,"retry_after":42,`+
		`"emoji":[{"custom_emoji_id":"7","emoji":"e","set_id":"1","format":"webp",`+
		`"needs_repainting":false,"file_id":9}],"missing":[]}`)
	if l, ok, _ := c.CustomEmojiStickers(ctx, []int64{7, 8}); !ok ||
		l.Outcome != EmojiFloodWait || l.RetryAfter != 42 || len(l.Emoji) != 1 ||
		l.Emoji[0].FileID != 9 || len(l.Missing) != 0 {
		t.Errorf("flood wait = %+v %v", l, ok)
	}
	set(200, `{"outcome":"unavailable","message":"not ready","asked":1,"retry_after":3,`+
		`"effects":[],"missing":[]}`)
	if l, ok, _ := c.MessageEffects(ctx, []int64{-1}); !ok ||
		l.Outcome != EmojiUnavailable || l.RetryAfter != 0 || l.Message != "not ready" {
		t.Errorf("unavailable = %+v %v", l, ok)
	}

	mu.Lock()
	wantAsked := []string{
		`POST /v1/custom-emoji {"ids":["5368324170671202286","5370870893004203704","5370870893004203705"]}`,
		`POST /v1/message-effects {"ids":["5104841245755180586","5046509860389126442"]}`,
		`POST /v1/custom-emoji {"ids":["7","8"]}`,
		`POST /v1/message-effects {"ids":["-1"]}`,
	}
	if !reflect.DeepEqual(asked, wantAsked) {
		t.Errorf("asked %q", asked)
	}
	asked = nil
	mu.Unlock()

	// Nothing is sent for no ids or too many.
	tooMany := make([]int64, CustomEmojiLimit+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	if _, ok, r := c.CustomEmojiStickers(ctx, nil); ok || r.Sent {
		t.Error("no ids were sent")
	}
	if _, ok, r := c.CustomEmojiStickers(ctx, tooMany); ok || r.Sent {
		t.Error("201 ids were sent")
	}
	if _, ok, r := c.MessageEffects(ctx, tooMany[:MessageEffectLimit+1]); ok || r.Sent {
		t.Error("51 effects were sent")
	}
	if _, ok, r := c.MessageEffects(ctx, tooMany[:MessageEffectLimit]); !ok || !r.Sent {
		t.Error("50 effects were not sent")
	}
	mu.Lock()
	if len(asked) != 1 || !strings.HasPrefix(asked[0], "POST /v1/message-effects ") {
		t.Errorf("asked %q", asked)
	}
	mu.Unlock()

	// No answer to go by.
	for _, c2 := range []struct {
		st   int
		body string
	}{
		{400, `{"error":"bad_request"}`},
		{503, `{"error":"unavailable"}`},
		{500, `{"error":"boom"}`},
		{200, `not json`},
		{200, `{"outcome":"ok","emoji":[]}`},
		{200, `{"outcome":"ok","missing":[]}`},
		{200, `{"outcome":"maybe","emoji":[],"effects":[],"missing":[]}`},
		{200, `{"outcome":"ok","emoji":[{"emoji":"x"}],"effects":[{"emoji":"x"}],"missing":[]}`},
		{200, `{"outcome":"ok","emoji":[{"custom_emoji_id":7}],"effects":[{"effect_id":7}],"missing":[]}`},
		{200, `{"outcome":"ok","emoji":[],"effects":[],"missing":["x"]}`},
		{200, `{"outcome":"ok","emoji":[{"custom_emoji_id":"1","set_id":5}],` +
			`"effects":[{"effect_id":"1","static_icon_file_id":"5"}],"missing":[]}`},
	} {
		set(c2.st, c2.body)
		if l, ok, _ := c.CustomEmojiStickers(ctx, []int64{1}); ok {
			t.Errorf("%d %s taken as %+v", c2.st, c2.body, l)
		}
		if l, ok, _ := c.MessageEffects(ctx, []int64{1}); ok {
			t.Errorf("effects %d %s taken as %+v", c2.st, c2.body, l)
		}
	}
	// Each answer is read for its own list only.
	set(200, `{"outcome":"ok","effects":[],"missing":[]}`)
	if _, ok, _ := c.CustomEmojiStickers(ctx, []int64{1}); ok {
		t.Error("an effects answer taken for custom emoji")
	}
	set(200, `{"outcome":"ok","emoji":[],"missing":[]}`)
	if _, ok, _ := c.MessageEffects(ctx, []int64{1}); ok {
		t.Error("a custom emoji answer taken for effects")
	}

	// A slow daemon counts as away, within the timeout.
	set(200, `{"outcome":"ok","emoji":[],"missing":[]}`)
	mu.Lock()
	delay = 500 * time.Millisecond
	mu.Unlock()
	start := time.Now()
	if _, ok, _ := c.emojiLookup(ctx, "/v1/custom-emoji", []int64{1}, false,
		100*time.Millisecond); ok {
		t.Error("a slow answer was taken")
	}
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Errorf("a slow lookup took %v", d)
	}
	mu.Lock()
	delay = 0
	mu.Unlock()

	var off *Client
	if _, ok, r := off.CustomEmojiStickers(ctx, []int64{1}); ok || r.Sent {
		t.Error("an off client looked up")
	}
	srv.Close()
	if _, ok, _ := c.MessageEffects(ctx, []int64{1}); ok {
		t.Error("an unreachable daemon answered")
	}
}

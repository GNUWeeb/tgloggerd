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
	"sync"
	"testing"
	"time"
)

// GET /v1/users/{id}/photos and /v1/chats/{id}/photos against a fake
// daemon: the list as answered, and every answer that is not one (off,
// away, slow, unknown, malformed) as no answer.
func TestPhotoLists(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	status, answer := 200, ""
	var asked []string
	delay := time.Duration(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		st, a, d := status, answer, delay
		asked = append(asked, r.Method+" "+r.URL.RequestURI())
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

	set(200, `{"outcome":"ok","message":"","total":7,"photos":[`+
		`{"photo_id":"5120000000000000001","date":1756745880,`+
		`"file_id":1227222,"is_current":true,"animated":false},`+
		`{"photo_id":"5120000000000000000","date":1756745951,"file_id":1190282,`+
		`"is_current":false,"animated":true}]}`)
	l, ok, _ := c.UserPhotos(ctx, 2102916111)
	if !ok || l.Outcome != PhotosOK || l.Total != 7 || len(l.Photos) != 2 ||
		*l.Photos[0].PhotoID != "5120000000000000001" || *l.Photos[0].Date != 1756745880 ||
		l.Photos[0].FileID != 1227222 || !l.Photos[0].IsCurrent || l.Photos[0].Animated ||
		*l.Photos[1].PhotoID != "5120000000000000000" || l.Photos[1].FileID != 1190282 ||
		l.Photos[1].IsCurrent || !l.Photos[1].Animated {
		t.Errorf("user photos = %+v %v", l, ok)
	}
	// A group's current photo without a change message has no id or
	// date.
	set(200, `{"outcome":"ok","message":"","total":1,"photos":[{"photo_id":null,`+
		`"date":null,"file_id":880008,"is_current":true,"animated":false}]}`)
	if l, ok, _ := c.ChatPhotos(ctx, -1001000000001); !ok || len(l.Photos) != 1 ||
		l.Photos[0].PhotoID != nil || l.Photos[0].Date != nil || l.Photos[0].FileID != 880008 {
		t.Errorf("chat photos = %+v %v", l, ok)
	}
	// No total: the photos answered.
	set(200, `{"outcome":"ok","photos":[{"photo_id":"1","date":1,"file_id":2}]}`)
	if l, ok, _ := c.ChatPhotos(ctx, -5); !ok || l.Total != 1 {
		t.Errorf("no total = %+v %v", l, ok)
	}
	// A flood wait and a Telegram error are answers.
	set(200, `{"outcome":"flood_wait","message":"","retry_after":420,"total":null,"photos":[]}`)
	if l, ok, _ := c.UserPhotos(ctx, 1); !ok || l.Outcome != PhotosFloodWait || l.RetryAfter != 420 {
		t.Errorf("flood wait = %+v %v", l, ok)
	}
	set(200, `{"outcome":"error","message":"USER_ID_INVALID","retry_after":5,"total":null,"photos":[]}`)
	if l, ok, _ := c.UserPhotos(ctx, 1); !ok || l.Outcome != PhotosError ||
		l.Message != "USER_ID_INVALID" || l.RetryAfter != 0 {
		t.Errorf("error = %+v %v", l, ok)
	}
	mu.Lock()
	want := []string{
		"GET /v1/users/2102916111/photos?offset=0&limit=100",
		"GET /v1/chats/-1001000000001/photos",
		"GET /v1/chats/-5/photos",
		"GET /v1/users/1/photos?offset=0&limit=100",
		"GET /v1/users/1/photos?offset=0&limit=100",
	}
	if len(asked) != len(want) {
		t.Errorf("asked %q", asked)
	} else {
		for i := range want {
			if asked[i] != want[i] {
				t.Errorf("asked %q, want %q", asked[i], want[i])
			}
		}
	}
	mu.Unlock()

	// No answer to go by.
	for _, c2 := range []struct {
		st   int
		body string
	}{
		{404, `{"error":"not_found"}`},
		{503, `{"error":"TDLib is not ready"}`},
		{504, `{"error":"timeout"}`},
		{500, `{"error":"boom"}`},
		{200, `not json`},
		{200, `{"total":3}`},
		{200, `{"outcome":"ok","total":3}`},
		{200, `{"outcome":"ok","photos":"many"}`},
		{200, `{"outcome":"maybe","photos":[]}`},
	} {
		set(c2.st, c2.body)
		if l, ok, _ := c.UserPhotos(ctx, 1); ok {
			t.Errorf("%d %s taken as %+v", c2.st, c2.body, l)
		}
		if l, ok, _ := c.ChatPhotos(ctx, -1); ok {
			t.Errorf("chat %d %s taken as %+v", c2.st, c2.body, l)
		}
	}

	// A slow daemon counts as away, within the timeout.
	set(200, `{"outcome":"ok","photos":[],"total":0}`)
	mu.Lock()
	delay = 500 * time.Millisecond
	mu.Unlock()
	start := time.Now()
	if _, ok, _ := c.photoList(ctx, "/v1/chats/-1/photos", 100*time.Millisecond); ok {
		t.Error("a slow answer was taken")
	}
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Errorf("a slow list took %v", d)
	}
	mu.Lock()
	delay = 0
	mu.Unlock()

	var off *Client
	if _, ok, r := off.UserPhotos(ctx, 1); ok || r.Sent {
		t.Error("an off client listed")
	}
	srv.Close()
	if _, ok, _ := c.ChatPhotos(ctx, -1); ok {
		t.Error("an unreachable daemon listed")
	}
}

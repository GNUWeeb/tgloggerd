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

// POST /v1/users/resolve against a fake daemon: each outcome as
// answered, and every answer that is not one (off, away, slow, not
// ready, malformed) as no answer, never as an outcome.
func TestResolveUser(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var status int
	var answer string
	var bodies []string
	delay := time.Duration(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		st, a, d := status, answer, delay
		bodies = append(bodies, r.Method+" "+r.URL.Path+" "+string(b))
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

	set(200, `{"outcome":"ok","message":"","user":{"user_id":7000000001,"username":"gw_bridge_bot","is_bot":true}}`)
	res, ok, _ := c.ResolveUser(ctx, "gw_bridge_bot")
	if !ok || res.Outcome != ResolveOK || res.User == nil ||
		*res.User != (ResolvedUser{7000000001, "gw_bridge_bot", true}) {
		t.Errorf("ok = %+v %v", res, ok)
	}
	mu.Lock()
	if len(bodies) != 1 || bodies[0] != `POST /v1/users/resolve {"username":"@gw_bridge_bot"}` {
		t.Errorf("asked %q", bodies)
	}
	mu.Unlock()
	// A name given with its @ is sent as it is.
	c.ResolveUser(ctx, "@gw_bridge_bot")
	mu.Lock()
	if bodies[1] != `POST /v1/users/resolve {"username":"@gw_bridge_bot"}` {
		t.Errorf("asked %q", bodies[1])
	}
	mu.Unlock()

	for _, c2 := range []struct {
		st         int
		body       string
		outcome    string
		retryAfter int64
	}{
		{200, `{"outcome":"not_found","message":"no such username","user":null}`, ResolveNotFound, 0},
		{200, `{"outcome":"not_a_user","message":"a channel","user":null}`, ResolveNotAUser, 0},
		{200, `{"outcome":"flood_wait","message":"","retry_after":420,"user":null}`, ResolveFloodWait, 420},
		{200, `{"outcome":"error","message":"USERNAME_INVALID","user":null}`, ResolveError, 0},
		// Only a flood wait carries a wait.
		{200, `{"outcome":"not_found","retry_after":99,"user":null}`, ResolveNotFound, 0},
		// A malformed username: asking again would be refused again.
		{400, `{"error":"bad username"}`, ResolveError, 0},
		{400, ``, ResolveError, 0},
	} {
		set(c2.st, c2.body)
		res, ok, _ := c.ResolveUser(ctx, "x_name")
		if !ok || res.Outcome != c2.outcome || res.RetryAfter != c2.retryAfter || res.User != nil {
			t.Errorf("%d %s = %+v %v", c2.st, c2.body, res, ok)
		}
	}

	// No answer to go by.
	for _, c2 := range []struct {
		st   int
		body string
	}{
		{503, `{"error":"TDLib is not ready"}`},
		{500, `{"error":"boom"}`},
		{404, ``},
		{200, `not json`},
		{200, `{"outcome":"maybe"}`},
		{200, `{"outcome":"ok","user":null}`},
		{200, `{"outcome":"ok","user":{"user_id":0}}`},
	} {
		set(c2.st, c2.body)
		if res, ok, _ := c.ResolveUser(ctx, "x_name"); ok {
			t.Errorf("%d %s taken as %+v", c2.st, c2.body, res)
		}
	}

	// A slow daemon counts as away, within the timeout.
	set(200, `{"outcome":"not_found","user":null}`)
	mu.Lock()
	delay = 500 * time.Millisecond
	mu.Unlock()
	start := time.Now()
	if _, ok, _ := c.resolveUser(ctx, "x_name", 100*time.Millisecond); ok {
		t.Error("a slow answer was taken")
	}
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Errorf("a slow resolve took %v", d)
	}
	mu.Lock()
	delay = 0
	mu.Unlock()

	var off *Client
	if _, ok, r := off.ResolveUser(ctx, "x_name"); ok || r.Sent {
		t.Error("an off client resolved")
	}
	srv.Close()
	if _, ok, _ := c.ResolveUser(ctx, "x_name"); ok {
		t.Error("an unreachable daemon resolved")
	}
}

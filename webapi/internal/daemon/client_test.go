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
	"strings"
	"sync"
	"testing"
	"time"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestParseBase(t *testing.T) {
	for in, want := range map[string]string{
		"http://10.0.0.1:8091":  "http://10.0.0.1:8091",
		"http://10.0.0.1:8091/": "http://10.0.0.1:8091",
		"https://daemon":        "https://daemon",
		"http://h:1/v1":         "",
		"http://h?x":            "",
		"http://u@h":            "",
		"http://":               "",
		"http:///":              "",
		"ftp://h":               "",
		"10.0.0.1:8091":         "",
		"http://h//":            "",
	} {
		got, ok := parseBase(in)
		if ok != (want != "") || got != want {
			t.Errorf("parseBase(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestFetchTimeout(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"": 40 * time.Second, "0": 40 * time.Second, "-5": 40 * time.Second,
		"x": 40 * time.Second, "50": 60 * time.Second,
	} {
		if got := FetchTimeout(in); got != want {
			t.Errorf("FetchTimeout(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestOff(t *testing.T) {
	c, err := New("", testKey, time.Second)
	if c != nil || err != nil {
		t.Fatalf("New(\"\") = %v, %v", c, err)
	}
	if c.Enabled() || c.BaseURL() != "" {
		t.Error("an off client is enabled")
	}
	r := c.RecordAccess(context.Background(), []uint64{1})
	if r.Sent || r.Error == "" {
		t.Errorf("an off client answered %+v", r)
	}
	if _, err := New("nonsense", testKey, time.Second); err == nil {
		t.Error("a malformed base was taken")
	}
}

func TestCall(t *testing.T) {
	type seen struct{ method, path, auth, ctype, body string }
	var got seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = seen{r.Method, r.URL.Path, r.Header.Get("Authorization"),
			r.Header.Get("Content-Type"), string(b)}
		switch r.URL.Path {
		case "/v1/files/access":
			w.WriteHeader(http.StatusNoContent)
		case "/v1/busy":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":"not ready"}`)
		case "/v1/text":
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "oops")
		case "/v1/slow":
			time.Sleep(300 * time.Millisecond)
		default:
			io.WriteString(w, `{"state":"stored","id":5}`)
		}
	}))
	defer srv.Close()

	c, err := New(srv.URL+"/", testKey, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bearer, _ := Bearer(testKey)

	r := c.RecordAccess(ctx, nil)
	if !r.OK() || r.Status != 204 || r.Body != nil || r.Error != "" {
		t.Errorf("RecordAccess = %+v", r)
	}
	if want := (seen{"POST", "/v1/files/access", bearer,
		"application/json; charset=utf-8", `{"ids":[]}`}); got != want {
		t.Errorf("request %+v, want %+v", got, want)
	}

	r = c.Fetch(ctx, 5, 10, 0)
	if !r.OK() || string(r.Body) != `{"state":"stored","id":5}` {
		t.Errorf("Fetch = %+v", r)
	}
	if got.path != "/v1/files/5/fetch" || got.body != `{"length":0,"offset":10}` {
		t.Errorf("Fetch sent %+v", got)
	}

	r = c.Call(ctx, "POST", "/v1/busy", nil, time.Second)
	if r.OK() || !r.Sent || r.Status != 503 || r.RetryAfter != "7" ||
		r.Error != "HTTP 503 not ready" {
		t.Errorf("busy = %+v", r)
	}
	if got.ctype != "" || got.body != "" {
		t.Errorf("a nil body sent %+v", got)
	}

	r = c.Call(ctx, "GET", "/v1/text", nil, time.Second)
	if r.Status != 500 || r.Body != nil || r.Error != "HTTP 500" {
		t.Errorf("text = %+v", r)
	}

	r = c.Call(ctx, "GET", "/v1/slow", nil, 50*time.Millisecond)
	if r.Sent || r.Status != 0 || r.Error == "" {
		t.Errorf("slow = %+v", r)
	}

	srv.Close()
	r = c.Call(ctx, "GET", "/v1/x", nil, time.Second)
	if r.Sent || r.Error == "" {
		t.Errorf("refused = %+v", r)
	}
	if !strings.HasPrefix(c.BaseURL(), "http://127.0.0.1:") {
		t.Errorf("BaseURL = %q", c.BaseURL())
	}
}

func TestStatusAndStorage(t *testing.T) {
	var mu sync.Mutex
	answers := map[string]string{}
	var slow bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		body, ok := answers[r.URL.Path]
		s := slow
		mu.Unlock()
		if want, _ := Bearer(testKey); r.Header.Get("Authorization") != want || r.Method != "GET" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if s {
			time.Sleep(1500 * time.Millisecond)
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, body)
	}))
	defer srv.Close()
	set := func(path, body string) {
		mu.Lock()
		defer mu.Unlock()
		if body == "" {
			delete(answers, path)
		} else {
			answers[path] = body
		}
	}

	c, err := New(srv.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	set("/v1/status", `{"tdlib":"ready","since":1763400000}`)
	if st, ok := c.Status(ctx); !ok || st != (Status{"ready", 1763400000}) {
		t.Errorf("Status = %+v, %v", st, ok)
	}
	set("/v1/storage", `{"files":3,"bytes":1024,"max_bytes":4096,"dry_run":true,"last_sweep":null}`)
	if st, ok := c.Storage(ctx); !ok || st != (Storage{3, 1024, 4096}) {
		t.Errorf("Storage = %+v, %v", st, ok)
	}

	// Not an answer of that shape: fail soft.
	set("/v1/status", `{"since":5}`)
	if st, ok := c.Status(ctx); ok {
		t.Errorf("a status without tdlib = %+v", st)
	}
	set("/v1/storage", `{"files":3}`)
	if st, ok := c.Storage(ctx); ok {
		t.Errorf("a storage without bytes = %+v", st)
	}
	set("/v1/status", "")
	set("/v1/storage", "")
	if _, ok := c.Status(ctx); ok {
		t.Error("a 404 status was taken")
	}
	if _, ok := c.Storage(ctx); ok {
		t.Error("a 404 storage was taken")
	}

	// A daemon slower than the timeout counts as away, within it.
	set("/v1/status", `{"tdlib":"ready","since":1}`)
	mu.Lock()
	slow = true
	mu.Unlock()
	start := time.Now()
	if _, ok := c.Status(ctx); ok {
		t.Error("a slow status was taken")
	}
	if d := time.Since(start); d > 1400*time.Millisecond {
		t.Errorf("a slow status took %v", d)
	}

	// Off and unreachable.
	var off *Client
	if _, ok := off.Status(ctx); ok {
		t.Error("an off client has a status")
	}
	if _, ok := off.Storage(ctx); ok {
		t.Error("an off client has a storage")
	}
	srv.Close()
	if _, ok := c.Status(ctx); ok {
		t.Error("an unreachable daemon has a status")
	}
}

// The group member calls: their paths and query, the answers read, and
// failing soft.
func TestRefreshAndMembers(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	answers := map[string]struct {
		status int
		body   string
	}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.Method+" "+r.URL.RequestURI())
		a, ok := answers[r.URL.Path]
		mu.Unlock()
		if want, _ := Bearer(testKey); r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(a.status)
		io.WriteString(w, a.body)
	}))
	defer srv.Close()
	set := func(path string, status int, body string) {
		mu.Lock()
		defer mu.Unlock()
		answers[path] = struct {
			status int
			body   string
		}{status, body}
	}
	c, err := New(srv.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	set("/v1/chats/-1001/refresh", 200, `{"member_count":1234,"can_get_members":true,`+
		`"is_channel":false,"account_is_admin":true,"fetched_at":1763540000}`)
	if ref, _, ok := c.RefreshChat(ctx, -1001); !ok || ref.MemberCount == nil ||
		*ref.MemberCount != 1234 || !ref.CanGetMembers || !ref.AccountIsAdmin ||
		ref.FetchedAt == nil || *ref.FetchedAt != 1763540000 || ref.Error != "" {
		t.Errorf("refresh = %+v %v", ref, ok)
	}
	set("/v1/chats/-1001/refresh", 200, `{"member_count":null,"can_get_members":false,`+
		`"is_channel":true,"account_is_admin":false,"fetched_at":null,"error":"CHANNEL_PRIVATE"}`)
	if ref, _, ok := c.RefreshChat(ctx, -1001); !ok || ref.MemberCount != nil ||
		ref.FetchedAt != nil || ref.Error != "CHANNEL_PRIVATE" || !ref.IsChannel {
		t.Errorf("refresh with an error = %+v %v", ref, ok)
	}
	set("/v1/chats/-1001/refresh", 504, `{"error":"timeout"}`)
	if _, r, ok := c.RefreshChat(ctx, -1001); ok || r.Status != 504 {
		t.Errorf("refresh 504 = %+v %v", r, ok)
	}

	set("/v1/chats/-1001/members", 200, `{"available":true,"reason":"","total":3,`+
		`"members":[{"user_id":7,"status":"creator","joined_at":null},`+
		`{"user_id":8,"status":"member","joined_at":1700000000}]}`)
	m, _, ok := c.ChatMembers(ctx, -1001, 50, 2)
	if !ok || !m.Available || m.Total == nil || *m.Total != 3 || len(m.Members) != 2 ||
		m.Members[0] != (Member{UserID: 7, Status: "creator"}) || m.Members[1].JoinedAt == nil ||
		*m.Members[1].JoinedAt != 1700000000 {
		t.Errorf("members = %+v %v", m, ok)
	}
	set("/v1/chats/-1001/members", 200, `{"available":false,"reason":"flood_wait",`+
		`"retry_after":30,"total":null,"members":[]}`)
	if m, _, ok = c.ChatMembers(ctx, -1001, 0, 50); !ok || m.Available || m.Reason != "flood_wait" ||
		m.RetryAfter != 30 || m.Total != nil || len(m.Members) != 0 {
		t.Errorf("flood wait = %+v %v", m, ok)
	}
	if _, r, ok := c.ChatMembers(ctx, -1002, 0, 50); ok || r.Status != 404 {
		t.Errorf("unknown chat = %+v %v", r, ok)
	}
	mu.Lock()
	want := []string{"POST /v1/chats/-1001/refresh", "POST /v1/chats/-1001/refresh",
		"POST /v1/chats/-1001/refresh", "GET /v1/chats/-1001/members?offset=50&limit=2",
		"GET /v1/chats/-1001/members?offset=0&limit=50", "GET /v1/chats/-1002/members?offset=0&limit=50"}
	if strings.Join(asked, "\n") != strings.Join(want, "\n") {
		t.Errorf("asked\n%s", strings.Join(asked, "\n"))
	}
	mu.Unlock()

	var off *Client
	if _, _, ok := off.RefreshChat(ctx, -1001); ok {
		t.Error("refresh with the client off")
	}
	if _, _, ok := off.ChatMembers(ctx, -1001, 0, 50); ok {
		t.Error("members with the client off")
	}
}

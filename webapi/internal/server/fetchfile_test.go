// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The "Fetch from Telegram" action, POST /api/files/{token}/fetch
// (fetchfile.go), against a real logger schema and the fake daemon of
// media_test.go.

// post sends the action as cookie's session, with its CSRF token
// unless noCSRF.
func (e *mediaEnv) post(path, cookie string, noCSRF ...bool) got {
	r := httptest.NewRequest(http.MethodPost, path, nil)
	r.RemoteAddr = "192.0.2.7:4321"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "tgw_session", Value: cookie})
		if len(noCSRF) == 0 || !noCSRF[0] {
			r.Header.Set(CSRFHeader, e.cookies.Signer.SessionCSRF(cookie))
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	res := w.Result()
	b, _ := io.ReadAll(res.Body)
	return got{res.StatusCode, res.Header, string(b)}
}

func fetchAnswerOf(t *testing.T, g got) fileFetchJSON {
	t.Helper()
	expect(t, g, 200, "")
	var a fileFetchJSON
	if err := json.Unmarshal([]byte(g.body), &a); err != nil {
		t.Fatalf("answer %q: %v", g.body, err)
	}
	return a
}

func errorCode(t *testing.T, g got, status int, code string) {
	t.Helper()
	expect(t, g, status, "")
	var b struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(g.body), &b); err != nil || b.Error != code {
		t.Errorf("error %q, want %q", g.body, code)
	}
}

func (e *mediaEnv) fetchCount() int {
	e.fake.mu.Lock()
	defer e.fake.mu.Unlock()
	return len(e.fake.fetches)
}

// A big file is fetched by the action, small and big alike answer with
// the URL that serves them, and a stored one is answered without
// asking the daemon. Each request that reaches the daemon is audited.
func TestFetchFileAction(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	ck, uid := e.signIn("fetcher")

	data := []byte("small file bytes")
	stored := e.addFile(fileRow{id: 50, data: data, ext: "jpg", typ: "photo", onDisk: true})
	small := e.addFile(fileRow{id: 51, typ: "photo", ext: "jpg", size: uint64(len(data))})
	big := e.addFile(fileRow{id: 52, typ: "video", ext: "mp4", size: 1000})

	a := fetchAnswerOf(t, e.post("/api/files/"+stored+"/fetch", ck))
	if a != (fileFetchJSON{"stored", "/files/" + stored, uint64(len(data))}) {
		t.Errorf("stored: %+v", a)
	}
	if n := e.fetchCount(); n != 0 {
		t.Errorf("the daemon was asked for a stored file (%d)", n)
	}

	e.fake.answer(51, 200, "", `{"state":"stored","id":50}`)
	a = fetchAnswerOf(t, e.post("/api/files/"+small+"/fetch", ck))
	if a != (fileFetchJSON{"stored", "/files/" + small, uint64(len(data))}) {
		t.Errorf("small: %+v", a)
	}
	if f := e.fake.lastFetch(); f != `51 {"length":0,"offset":0}` {
		t.Errorf("fetch %q", f)
	}

	part := filepath.Join(e.tdlib, "videos", "file_52.mp4")
	if err := os.WriteFile(part, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.fake.answer(52, 200, "", fmt.Sprintf(
		`{"state":"partial","path":%q,"offset":0,"available":10,"size":1000}`, part))
	a = fetchAnswerOf(t, e.post("/api/files/"+big+"/fetch", ck))
	if a != (fileFetchJSON{"partial", "/files/" + big, 1000}) {
		t.Errorf("big: %+v", a)
	}
	g := e.get(a.URL, ck)
	expect(t, g, 206, "0123456789")
	expectHeader(t, g, "Content-Range", "bytes 0-9/1000")
	expectHeader(t, e.post("/api/files/"+big+"/fetch", ck), "Cache-Control", "no-store")

	var fetches []string
	for _, r := range e.audits() {
		if r.action == "file_fetch" {
			if !r.uid.Valid || uint64(r.uid.Int64) != uid {
				t.Errorf("audit uid %v", r.uid)
			}
			fetches = append(fetches, r.detail)
		}
	}
	if strings.Join(fetches, ",") != "file 51: ok,file 52: ok,file 52: ok" {
		t.Errorf("audits %q", fetches)
	}
}

// The action is for signed-in accounts that hold fetch_files, with the
// CSRF token, on a token that opens.
func TestFetchFileActionRefused(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	big := e.addFile(fileRow{id: 60, typ: "video", ext: "mp4", size: 1000})
	e.fake.answer(60, 200, "", `{"state":"stored","id":60}`)
	path := "/api/files/" + big + "/fetch"

	errorCode(t, e.post(path, ""), 401, "unauthenticated")
	admin, _ := e.signIn("admin")
	errorCode(t, e.post(path, admin, true), 403, "csrf")
	errorCode(t, e.post("/api/files/not-a-token/fetch", admin), 404, "not_found")
	errorCode(t, e.post("/api/files/"+e.tokens.Encrypt(69)+"/fetch", admin), 404, "not_found")

	// A member without fetch_files: 403, and the daemon is not asked.
	member, mid := e.signInAs("member", "member")
	errorCode(t, e.post(path, member), 403, "fetch_not_allowed")
	if n := e.fetchCount(); n != 0 {
		t.Errorf("the daemon was asked %d times", n)
	}
	// Given fetch_files, it may.
	if _, err := e.app.Exec("INSERT INTO web_member_abilities (web_user_id, ability) VALUES (?, ?)",
		mid, "fetch_files"); err != nil {
		t.Fatal(err)
	}
	if a := fetchAnswerOf(t, e.post(path, member)); a.State != "stored" {
		t.Errorf("member with fetch_files: %+v", a)
	}
	if n := e.fetchCount(); n != 1 {
		t.Errorf("the daemon was asked %d times, want 1", n)
	}
}

// The daemon's failures map to the action's errors.
func TestFetchFileActionDaemonErrors(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	ck, _ := e.signIn("fetcher")
	for i, c := range []struct {
		status     int
		retryAfter string
		body       string
		want       int
		code       string
		wantRetry  string
	}{
		{410, "", `{"error":"gone"}`, 410, "gone", ""},
		{503, "7", `{"error":"unavailable"}`, 503, "unavailable", "7"},
		{503, "", `{"error":"unavailable"}`, 503, "unavailable", "5"},
		{504, "", `{"error":"timeout"}`, 504, "timeout", ""},
		{404, "", `{"error":"not_found"}`, 404, "not_found", ""},
		{500, "", `{"error":"internal"}`, 502, "daemon", ""},
		{200, "", `{"state":"what"}`, 502, "daemon", ""},
		{200, "", `not json`, 502, "daemon", ""},
	} {
		id := uint64(70 + i)
		tok := e.addFile(fileRow{id: id, typ: "video", ext: "mp4", size: 1000})
		e.fake.answer(id, c.status, c.retryAfter, c.body)
		g := e.post("/api/files/"+tok+"/fetch", ck)
		errorCode(t, g, c.want, c.code)
		expectHeader(t, g, "Retry-After", c.wantRetry)
	}
}

// Without the daemon API nothing can be fetched: 502 daemon.
func TestFetchFileActionNoDaemon(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{noDaemon: true, cap: 100})
	ck, _ := e.signIn("fetcher")
	tok := e.addFile(fileRow{id: 80, typ: "video", ext: "mp4", size: 1000})
	errorCode(t, e.post("/api/files/"+tok+"/fetch", ck), 502, "daemon")
}

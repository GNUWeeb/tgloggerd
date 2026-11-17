// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/media"
	"tgloggerd/webapi/internal/testdb"
)

// These tests serve files from a real logger schema and audit into a
// real web schema, each in databases of its own (internal/testdb). They
// run only when WEBAPI_TEST_MYSQL_DSN names a test MySQL. The daemon is
// a fake on a loopback port the OS picks.

func testRODB(t *testing.T) *sql.DB {
	return testdb.New(t, testdb.Logger).DB
}

// fakeDaemon answers POST /v1/files/{id}/fetch from a table, and
// records the bodies it got and the access reports.
type fakeDaemon struct {
	mu      sync.Mutex
	answers map[string]func(w http.ResponseWriter)
	fetches []string // "<id> <body>"
	access  []uint64
}

func (f *fakeDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/v1/files/access" {
		var in struct {
			IDs []uint64 `json:"ids"`
		}
		_ = json.Unmarshal(b, &in)
		f.access = append(f.access, in.IDs...)
		w.Write([]byte(`{"ids":0,"updated":0}`))
		return
	}
	id, ok := strings.CutPrefix(r.URL.Path, "/v1/files/")
	id, ok2 := strings.CutSuffix(id, "/fetch")
	if !ok || !ok2 || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	f.fetches = append(f.fetches, id+" "+string(bytes.TrimSpace(b)))
	a := f.answers[id]
	if a == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not_found"}`))
		return
	}
	a(w)
}

func (f *fakeDaemon) answer(id uint64, status int, retryAfter, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers[fmt.Sprint(id)] = func(w http.ResponseWriter) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}
}

func (f *fakeDaemon) lastFetch() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.fetches) == 0 {
		return ""
	}
	return f.fetches[len(f.fetches)-1]
}

type mediaEnv struct {
	t       *testing.T
	ro, app *sql.DB
	h       http.Handler
	tokens  *auth.FileTokens
	cookies *auth.Cookies
	access  *daemon.AccessReporter
	fake    *fakeDaemon
	store   string // WEB_STORAGE_DIR
	tdlib   string // <TG_DATA_DIR>/tdlib, canonical
	nextTG  int
}

type mediaOpts struct {
	noDaemon bool
	cap      uint64
}

func newMediaEnv(t *testing.T, o mediaOpts) *mediaEnv {
	e := &mediaEnv{t: t, ro: testRODB(t), app: testDB(t), fake: &fakeDaemon{
		answers: map[string]func(http.ResponseWriter){}}}
	base := t.TempDir()
	e.store = filepath.Join(base, "storage")
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "tdlib", "videos"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.tdlib = media.TDLibDir(dataDir)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var dc *daemon.Client
	if !o.noDaemon {
		srv := httptest.NewServer(e.fake)
		t.Cleanup(srv.Close)
		var err error
		if dc, err = daemon.New(srv.URL, testKey, 5*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	e.access = daemon.NewAccessReporter(dc, log)
	signer := auth.NewSigner(testKey)
	e.cookies = &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true}
	e.tokens = auth.NewFileTokens(testKey)
	e.h = New(Deps{
		Log:     log,
		RO:      e.ro,
		App:     e.app,
		Signer:  signer,
		Cookies: e.cookies,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Daemon:  dc,
		Tokens:  e.tokens,
		Access:  e.access,
		Media:   MediaConfig{StorageDir: e.store, TDLibDir: e.tdlib, StoreFileCap: o.cap},
	})
	return e
}

type fileRow struct {
	id       uint64
	data     []byte // stored bytes; nil = no sha256
	noBlob   bool   // a sha256 but no blob on disk
	ext      string
	typ      string
	name     string
	size     uint64 // 0 = len(data)
	onDisk   bool
	aliasOf  uint64
	shaOfHex string // the sha256 hex to record instead of data's
}

// addFile inserts a telegram_files row and, for one with data, puts its
// blob in the store, as the daemon would.
func (e *mediaEnv) addFile(f fileRow) string {
	e.nextTG++
	var sum any
	hx := f.shaOfHex
	if f.data != nil && hx == "" {
		s := sha256.Sum256(f.data)
		hx = hex.EncodeToString(s[:])
	}
	if hx != "" {
		b, _ := hex.DecodeString(hx)
		sum = b
	}
	size := f.size
	if size == 0 {
		size = uint64(len(f.data))
	}
	var alias any
	if f.aliasOf != 0 {
		alias = f.aliasOf
	}
	var ext any
	if f.ext != "" {
		ext = f.ext
	}
	if _, err := e.ro.Exec("INSERT INTO telegram_files (id, tg_file_id, file_type, "+
		"file_size, sha256, file_ext, orig_file_name, on_disk, alias_of) "+
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", f.id, fmt.Sprintf("tg-%d", e.nextTG),
		f.typ, size, sum, ext, f.name, f.onDisk, alias); err != nil {
		e.t.Fatal(err)
	}
	if f.data != nil && !f.noBlob {
		p := media.StorePath(e.store, hx, f.ext)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			e.t.Fatal(err)
		}
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	return e.tokens.Encrypt(f.id)
}

// signIn makes an account and returns its session cookie value.
func (e *mediaEnv) signIn(name string) (string, uint64) {
	uid := addUser(e.t, e.app, name, "x", "admin", true)
	w := httptest.NewRecorder()
	return e.cookies.Issue(w, e.cookies.NewSession(uid, name, "admin", 0)), uid
}

type got struct {
	status int
	h      http.Header
	body   string
}

func (e *mediaEnv) get(path, cookie string, hdr ...string) got {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = "192.0.2.7:4321"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "tgw_session", Value: cookie})
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	res := w.Result()
	b, _ := io.ReadAll(res.Body)
	return got{res.StatusCode, res.Header, string(b)}
}

// reported flushes the access reporter and returns the ids the fake
// daemon got since the last call.
func (e *mediaEnv) reported() []uint64 {
	e.access.Flush(e.t.Context())
	e.fake.mu.Lock()
	defer e.fake.mu.Unlock()
	ids := e.fake.access
	e.fake.access = nil
	return ids
}

func (e *mediaEnv) audits() []auditRow {
	return audits(e.t, e.app)
}

func expect(t *testing.T, g got, status int, body string) {
	t.Helper()
	if g.status != status {
		t.Fatalf("status %d, want %d (body %q)", g.status, status, g.body)
	}
	if body != "" && g.body != body {
		t.Fatalf("body %q, want %q", g.body, body)
	}
}

func expectHeader(t *testing.T, g got, k, v string) {
	t.Helper()
	if g.h.Get(k) != v {
		t.Errorf("%s = %q, want %q", k, g.h.Get(k), v)
	}
}

func TestMediaStored(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	photo := []byte("a photo's bytes, stored")
	tok := e.addFile(fileRow{id: 10, data: photo, ext: "jpg", typ: "photo", onDisk: true})

	g := e.get("/files/"+tok, "")
	expect(t, g, 200, string(photo))
	expectHeader(t, g, "Content-Type", "image/jpeg")
	expectHeader(t, g, "Cache-Control", "public, max-age=604800")
	expectHeader(t, g, "Accept-Ranges", "bytes")
	expectHeader(t, g, "Content-Disposition", "")
	if g.h.Get("Last-Modified") == "" {
		t.Error("no Last-Modified")
	}

	// A single range is a 206; one past the end is a 416.
	g = e.get("/files/"+tok, "", "Range", "bytes=2-6")
	expect(t, g, 206, string(photo[2:7]))
	expectHeader(t, g, "Content-Range", fmt.Sprintf("bytes 2-6/%d", len(photo)))
	g = e.get("/files/"+tok, "", "Range", "bytes=-3")
	expect(t, g, 206, string(photo[len(photo)-3:]))
	g = e.get("/files/"+tok, "", "Range", "bytes=500-")
	expect(t, g, 416, "")
	expectHeader(t, g, "Content-Range", fmt.Sprintf("bytes */%d", len(photo)))

	// Conditional: unchanged since the browser's copy.
	g = e.get("/files/"+tok, "", "If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	expect(t, g, 304, "")

	// Every answer with bytes counts as one access; anonymous requests
	// write no audit row; nothing asked the daemon to fetch.
	if ids := e.reported(); len(ids) != 1 || ids[0] != 10 {
		t.Errorf("reported %v, want [10]", ids)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("anonymous audit rows: %+v", a)
	}
	if f := e.fake.lastFetch(); f != "" {
		t.Errorf("a stored file was fetched: %s", f)
	}
}

func TestMediaTokens(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	e.addFile(fileRow{id: 11, data: []byte("x"), ext: "jpg", typ: "photo", onDisk: true})
	good := e.tokens.Encrypt(11)

	for _, p := range []string{
		"/files/" + e.tokens.Encrypt(999), // a valid token, no row
		"/files/" + strings.Repeat("0", 64),
		"/files/" + good[:63],
		"/files/" + good[:62] + "zz",
		"/files/11",
		"/files/" + auth.NewFileTokens([]byte("another key, another key, 32 by")).Encrypt(11),
	} {
		g := e.get(p, "")
		expect(t, g, 404, "")
		if !strings.Contains(g.body, "File not found") {
			t.Errorf("%s: body %q", p, g.body)
		}
		expectHeader(t, g, "Cache-Control", "")
	}
	// Either case of hex decrypts, as libsodium reads it.
	expect(t, e.get("/files/"+strings.ToUpper(good), ""), 200, "x")
}

func TestMediaNamesAndTypes(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	doc := []byte("%PDF-1.7 a document")
	tok := e.addFile(fileRow{id: 20, data: doc, ext: "pdf", typ: "document",
		name: "Q3 \"report\"/final.pdf", onDisk: true})
	g := e.get("/files/"+tok, "")
	expect(t, g, 200, string(doc))
	expectHeader(t, g, "Content-Disposition", "attachment; filename=Q3 reportfinal.pdf")
	expectHeader(t, g, "Content-Type", "application/pdf")

	// No name: the blob's.
	sum := sha256.Sum256([]byte("unnamed"))
	tok = e.addFile(fileRow{id: 21, data: []byte("unnamed"), ext: "zip", typ: "document", onDisk: true})
	g = e.get("/files/"+tok, "")
	expectHeader(t, g, "Content-Disposition", "attachment; filename="+hex.EncodeToString(sum[:])+".zip")
	expectHeader(t, g, "Content-Type", "application/zip")

	// The download name picks the type, even against the extension.
	tok = e.addFile(fileRow{id: 22, data: []byte("named"), ext: "bin", typ: "document",
		name: "notes.txt", onDisk: true})
	g = e.get("/files/"+tok, "")
	expectHeader(t, g, "Content-Type", "text/plain; charset=utf-8")

	// A video is shown inline, typed by its extension.
	tok = e.addFile(fileRow{id: 23, data: []byte("video"), ext: "mp4", typ: "video",
		name: "clip.mov", onDisk: true})
	g = e.get("/files/"+tok, "")
	expectHeader(t, g, "Content-Disposition", "")
	expectHeader(t, g, "Content-Type", "video/mp4")
}

func TestMediaAlias(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	data := []byte("the same bytes under two rows")
	e.addFile(fileRow{id: 30, data: data, ext: "pdf", typ: "document", name: "holder.pdf", onDisk: true})
	// The alias row holds no sha256 and is not on disk itself.
	tok := e.addFile(fileRow{id: 31, typ: "document", name: "mine.pdf", size: uint64(len(data)),
		aliasOf: 30})

	g := e.get("/files/"+tok, "")
	expect(t, g, 200, string(data))
	// The name is the token's row's, the access the holder's.
	expectHeader(t, g, "Content-Disposition", "attachment; filename=mine.pdf")
	if ids := e.reported(); len(ids) != 1 || ids[0] != 30 {
		t.Errorf("reported %v, want [30]", ids)
	}
	if f := e.fake.lastFetch(); f != "" {
		t.Errorf("fetched: %s", f)
	}
}

func TestMediaFetchStored(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 1 << 20})
	data := []byte("fetched now, stored under another row")
	// Row 41 is not stored; the daemon fetches it, finds the bytes are
	// row 40's, and answers with that row.
	e.addFile(fileRow{id: 40, data: data, ext: "jpg", typ: "photo", onDisk: true})
	tok := e.addFile(fileRow{id: 41, typ: "photo", ext: "jpg", size: uint64(len(data))})
	e.fake.answer(41, 200, "", `{"state":"stored","id":40}`)

	g := e.get("/files/"+tok, "", "Range", "bytes=1-3")
	expect(t, g, 206, string(data[1:4]))
	if f := e.fake.lastFetch(); f != `41 {"length":3,"offset":1}` {
		t.Errorf("fetch %q", f)
	}
	if ids := e.reported(); len(ids) != 1 || ids[0] != 40 {
		t.Errorf("reported %v, want [40]", ids)
	}

	// A stored answer whose blob is not there (yet): ask again soon.
	tok = e.addFile(fileRow{id: 42, typ: "photo", size: 10})
	e.fake.answer(42, 200, "", `{"state":"stored","id":43}`)
	g = e.get("/files/"+tok, "")
	expect(t, g, 503, "")
	expectHeader(t, g, "Retry-After", "1")

	// A stored row whose blob is missing on disk is fetched.
	tok = e.addFile(fileRow{id: 44, data: []byte("lost"), noBlob: true, typ: "photo", onDisk: true})
	e.fake.answer(44, 200, "", `{"state":"stored","id":40}`)
	expect(t, e.get("/files/"+tok, ""), 200, string(data))
	if f := e.fake.lastFetch(); f != `44 {"length":0,"offset":0}` {
		t.Errorf("fetch %q", f)
	}
}

func TestMediaPartial(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	cookie, uid := e.signIn("alice")
	// TDLib's partial file: 64 bytes there, of a 1000-byte file.
	part := filepath.Join(e.tdlib, "videos", "file_7.mp4")
	content := bytes.Repeat([]byte("0123456789abcdef"), 4)
	if err := os.WriteFile(part, content, 0o644); err != nil {
		t.Fatal(err)
	}
	tok := e.addFile(fileRow{id: 50, typ: "video", ext: "mp4", size: 1000})
	ans := func(off, avail int) {
		e.fake.answer(50, 200, "", fmt.Sprintf(
			`{"state":"partial","path":%q,"offset":%d,"available":%d,"size":1000}`,
			part, off, avail))
	}

	// Anonymous: a big file needs a session, and the daemon is not asked.
	ans(0, 64)
	g := e.get("/files/"+tok, "")
	expect(t, g, 403, "")
	if !strings.Contains(g.body, "Sign in to load this file") {
		t.Errorf("body %q", g.body)
	}
	if f := e.fake.lastFetch(); f != "" {
		t.Errorf("fetched anonymously: %s", f)
	}

	// Signed in: the available bytes, Content-Range with the full size.
	g = e.get("/files/"+tok, cookie)
	expect(t, g, 206, string(content))
	expectHeader(t, g, "Content-Range", "bytes 0-63/1000")
	expectHeader(t, g, "Content-Type", "video/mp4")
	expectHeader(t, g, "Content-Length", "64")
	expectHeader(t, g, "Cache-Control", "public, max-age=604800")

	// A range asks from its start; fewer bytes than available if asked.
	ans(16, 48)
	g = e.get("/files/"+tok, cookie, "Range", "bytes=16-19")
	expect(t, g, 206, string(content[16:20]))
	expectHeader(t, g, "Content-Range", "bytes 16-19/1000")
	if f := e.fake.lastFetch(); f != `50 {"length":4,"offset":16}` {
		t.Errorf("fetch %q", f)
	}

	// More available than on disk: not served, asked again soon.
	ans(0, 500)
	g = e.get("/files/"+tok, cookie)
	expect(t, g, 503, "")
	expectHeader(t, g, "Retry-After", "1")

	// The whole file from byte 0 is a 200.
	whole := filepath.Join(e.tdlib, "videos", "file_8.webm")
	if err := os.WriteFile(whole, content, 0o644); err != nil {
		t.Fatal(err)
	}
	e.fake.answer(50, 200, "", fmt.Sprintf(
		`{"state":"partial","path":%q,"offset":0,"available":64,"size":64}`, whole))
	g = e.get("/files/"+tok, cookie)
	expect(t, g, 200, string(content))
	expectHeader(t, g, "Content-Range", "")
	expectHeader(t, g, "Content-Type", "video/webm")

	if ids := e.reported(); len(ids) != 1 || ids[0] != 50 {
		t.Errorf("reported %v, want [50]", ids)
	}
	a := e.audits()
	if len(a) != 3 {
		t.Fatalf("audit rows %+v, want 3", a)
	}
	for _, r := range a {
		if r.action != "media" || r.detail != "file 50" || r.ip != "192.0.2.7" ||
			!r.uid.Valid || uint64(r.uid.Int64) != uid {
			t.Errorf("audit row %+v", r)
		}
	}
}

func TestMediaPartialRefused(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	cookie, _ := e.signIn("bob")
	outside := filepath.Join(filepath.Dir(e.tdlib), "secret.mp4")
	if err := os.WriteFile(outside, []byte("not TDLib's"), 0o644); err != nil {
		t.Fatal(err)
	}
	tok := e.addFile(fileRow{id: 60, typ: "video", ext: "mp4", size: 1000})

	for _, body := range []string{
		fmt.Sprintf(`{"state":"partial","path":%q,"offset":0,"available":5,"size":1000}`, outside),
		fmt.Sprintf(`{"state":"partial","path":%q,"offset":0,"available":5,"size":1000}`,
			filepath.Join(e.tdlib, "..", "secret.mp4")),
		`{"state":"partial","path":"videos/x.mp4","offset":0,"available":5,"size":1000}`,
		fmt.Sprintf(`{"state":"partial","path":%q,"offset":0,"available":0,"size":1000}`, outside),
		`{"state":"partial","offset":0,"available":5,"size":1000}`,
		`{"state":"partial","path":"/x","offset":-1,"available":5,"size":1000}`,
		`{"state":"partial","path":"/x","offset":999,"available":5,"size":1000}`,
		`{"state":"partial","path":"/x","offset":0,"available":5.5,"size":1000}`,
		`{"state":"stored"}`,
		`{"state":"stored","id":0}`,
		`{"state":"elsewhere"}`,
		`not json`,
		``,
	} {
		e.fake.answer(60, 200, "", body)
		g := e.get("/files/"+tok, cookie)
		if g.status != 502 {
			t.Errorf("%s: status %d, want 502", body, g.status)
		}
	}
	if ids := e.reported(); len(ids) != 0 {
		t.Errorf("reported %v for refused answers", ids)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("audit rows for refused answers: %+v", a)
	}
}

func TestMediaDaemonErrors(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	tok := e.addFile(fileRow{id: 70, typ: "photo", size: 10})
	for _, c := range []struct {
		status            int
		retryAfter        string
		want              int
		wantRA, wantRange string
	}{
		{503, "7", 503, "7", ""},
		{503, "", 503, "5", ""},
		{503, "soon", 503, "5", ""},
		{410, "", 410, "", ""},
		{504, "", 504, "", ""},
		{404, "", 404, "", ""},
		{416, "", 416, "", "bytes */10"},
		{400, "", 502, "", ""},
		{401, "", 502, "", ""},
		{500, "", 502, "", ""},
		{429, "3", 502, "", ""},
	} {
		e.fake.answer(70, c.status, c.retryAfter, `{"error":"x"}`)
		g := e.get("/files/"+tok, "")
		if g.status != c.want || g.h.Get("Retry-After") != c.wantRA ||
			g.h.Get("Content-Range") != c.wantRange {
			t.Errorf("daemon %d: got %d Retry-After %q Content-Range %q", c.status,
				g.status, g.h.Get("Retry-After"), g.h.Get("Content-Range"))
		}
		if g.h.Get("Cache-Control") != "" {
			t.Errorf("daemon %d: an error is cacheable", c.status)
		}
	}
	if ids := e.reported(); len(ids) != 0 {
		t.Errorf("reported %v for errors", ids)
	}
}

func TestMediaDaemonAway(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	tok := e.addFile(fileRow{id: 80, typ: "photo", size: 10})
	// A daemon that closes the connection without an answer.
	e.fake.mu.Lock()
	e.fake.answers["80"] = func(w http.ResponseWriter) {
		hj, _ := w.(http.Hijacker)
		c, _, _ := hj.Hijack()
		c.Close()
	}
	e.fake.mu.Unlock()
	g := e.get("/files/"+tok, "")
	expect(t, g, 503, "")
	expectHeader(t, g, "Retry-After", "10")
}

func TestMediaNoDaemon(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{noDaemon: true, cap: 100})
	cookie, _ := e.signIn("carol")
	tok := e.addFile(fileRow{id: 90, typ: "photo", size: 10})
	for _, c := range []string{"", cookie} {
		g := e.get("/files/"+tok, c)
		expect(t, g, 404, "")
		if !strings.Contains(g.body, "File not stored") {
			t.Errorf("body %q", g.body)
		}
	}
	// Stored files are served without the daemon.
	tok = e.addFile(fileRow{id: 91, data: []byte("here"), typ: "photo", onDisk: true})
	expect(t, e.get("/files/"+tok, ""), 200, "here")
}

func TestMediaAnonymousSmallFiles(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 100})
	data := []byte("small enough")
	e.addFile(fileRow{id: 100, data: data, typ: "photo", ext: "png", onDisk: true})

	// Under the cap: an anonymous request may fetch it.
	small := e.addFile(fileRow{id: 101, typ: "photo", size: 99})
	e.fake.answer(101, 200, "", `{"state":"stored","id":100}`)
	expect(t, e.get("/files/"+small, ""), 200, string(data))

	// At the cap: not without a session, and the daemon is not asked.
	big := e.addFile(fileRow{id: 102, typ: "document", size: 100})
	e.fake.answer(102, 200, "", `{"state":"stored","id":100}`)
	expect(t, e.get("/files/"+big, ""), 403, "")
	if f := e.fake.lastFetch(); !strings.HasPrefix(f, "101 ") {
		t.Errorf("last fetch %q", f)
	}

	// A revoked session counts as none.
	cookie, uid := e.signIn("dave")
	expect(t, e.get("/files/"+big, cookie), 200, string(data))
	if _, err := e.app.Exec("UPDATE web_users SET session_epoch = session_epoch + 1 WHERE id = ?", uid); err != nil {
		t.Fatal(err)
	}
	expect(t, e.get("/files/"+big, cookie), 403, "")

	// Unknown size: fetched, but a partial answer is refused.
	unknown := e.addFile(fileRow{id: 103, typ: "video"})
	e.fake.answer(103, 200, "", `{"state":"partial","path":"/x","offset":0,"available":5,"size":1000}`)
	expect(t, e.get("/files/"+unknown, ""), 403, "")
	if f := e.fake.lastFetch(); !strings.HasPrefix(f, "103 ") {
		t.Errorf("last fetch %q", f)
	}

	// Without a cap every file is small.
	e2 := newMediaEnv(t, mediaOpts{cap: 0})
	e2.addFile(fileRow{id: 100, data: data, typ: "photo", ext: "png", onDisk: true})
	huge := e2.addFile(fileRow{id: 104, typ: "photo", size: 1 << 40})
	e2.fake.answer(104, 200, "", `{"state":"stored","id":100}`)
	expect(t, e2.get("/files/"+huge, ""), 200, string(data))
}

func TestMediaAuditSignedIn(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	cookie, uid := e.signIn("erin")
	tok := e.addFile(fileRow{id: 110, data: []byte("seen"), typ: "photo", onDisk: true})
	expect(t, e.get("/files/"+tok, cookie), 200, "seen")
	a := e.audits()
	if len(a) != 1 || a[0].action != "media" || a[0].detail != "file 110" ||
		uint64(a[0].uid.Int64) != uid || a[0].ip != "192.0.2.7" {
		t.Errorf("audit rows %+v", a)
	}
	// Unknown tokens are not audited.
	expect(t, e.get("/files/"+e.tokens.Encrypt(12345), cookie), 404, "")
	if len(e.audits()) != 1 {
		t.Error("a 404 was audited")
	}
}

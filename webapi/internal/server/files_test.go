// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/media"
)

// Unstored files reached from the chat API and the files list, opened
// through /files/<token> against the fake daemon (media_test.go), on
// the "files" fixture: 860001 and 860005 stored, 860002 and 860003
// metadata only (small, big), 860004 evicted (small), under a store
// cap of 1 MiB.

const fetchLab = -1001000000030

// filesStoreCap is the per-file store cap the tests run with.
const filesStoreCap = 1 << 20

func newFilesEnv(t *testing.T, o mediaOpts) *mediaEnv {
	t.Helper()
	o.cap = filesStoreCap
	o.fixtures = []string{"sample", "files"}
	e := newMediaEnv(t, o)
	// The stored files' blobs, where their rows say.
	for _, id := range []uint64{860001, 860005} {
		sum := sha256.Sum256([]byte(fmt.Sprintf("files fixture %d", id)))
		ext := map[uint64]string{860001: "jpg", 860005: "ogg"}[id]
		p := media.StorePath(e.store, hex.EncodeToString(sum[:]), ext)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fmt.Sprintf("blob %d", id)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *mediaEnv) fileURL(id uint64) string {
	return "/files/" + e.tokens.Encrypt(id)
}

// chatMedia is the media of Fetch Lab's messages by row id - 3000, as
// cookie sees them.
func (e *mediaEnv) chatMedia(cookie string) map[int]chatMediaJSON {
	e.t.Helper()
	g := e.get(fmt.Sprintf("/api/chats/group/%d/messages?limit=10", fetchLab), cookie)
	expect(e.t, g, 200, "")
	var w struct {
		Messages []struct {
			ID    int64          `json:"id"`
			Media *chatMediaJSON `json:"media"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(g.body), &w); err != nil {
		e.t.Fatal(err)
	}
	out := map[int]chatMediaJSON{}
	for _, m := range w.Messages {
		if m.Media != nil {
			out[int(m.ID-3000)] = *m.Media
		}
	}
	return out
}

func strp(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// filesPageProps are the parts of FilesProps the tests look at.
type filesPageProps struct {
	Rows  []map[string]any `json:"rows"`
	Conds []listCond       `json:"conds"`
	Sort  string           `json:"sort"`
	Dir   string           `json:"dir"`
}

// filesPageWith is the files page's data, its rows limited to the
// fixture's files, with more filters (a JSON fragment of Conds, each
// preceded by a comma) and query parameters.
func (e *mediaEnv) filesPageWith(cookie, conds string, query ...string) (int, filesPageProps) {
	e.t.Helper()
	q := url.Values{"f": {`[{"field":"id","op":">","value":"860000"},` +
		`{"field":"id","op":"<","value":"860010"}` + conds + `]`}}
	for i := 0; i+1 < len(query); i += 2 {
		q.Set(query[i], query[i+1])
	}
	var b struct {
		Props filesPageProps `json:"props"`
	}
	g := e.get("/api/page/files?"+q.Encode(), cookie)
	if g.status != 200 {
		return g.status, b.Props
	}
	if err := json.Unmarshal([]byte(g.body), &b); err != nil {
		e.t.Fatal(err)
	}
	return 200, b.Props
}

// filesPage is the files page's rows of the fixture's files.
func (e *mediaEnv) filesPage(cookie string, query ...string) (int, []map[string]any) {
	e.t.Helper()
	st, p := e.filesPageWith(cookie, "", query...)
	return st, p.Rows
}

func byFileID(rows []map[string]any) map[string]map[string]any {
	m := map[string]map[string]any{}
	for _, r := range rows {
		m[js(r["id"])] = r
	}
	return m
}

// TestFetchFromChatSignedOut: a public chat gives a visitor without a
// session links to unstored files under the store cap only, and
// opening one fetches it through the daemon; a big one is neither
// linked nor fetched.
func TestFetchFromChatSignedOut(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	md := e.chatMedia("")

	if m := md[1]; !m.Stored || m.Fetchable || strp(m.URL) != e.fileURL(860001) ||
		strp(m.Thumb) != e.fileURL(860001) {
		t.Errorf("stored photo: %+v", m)
	}
	if m := md[2]; m.Stored || !m.Fetchable || strp(m.URL) != e.fileURL(860002) ||
		strp(m.Thumb) != e.fileURL(860002) {
		t.Errorf("metadata-only small photo: %+v", m)
	}
	if m := md[3]; m.Stored || m.Fetchable || m.URL != nil || m.Thumb != nil {
		t.Errorf("metadata-only big video: %+v", m)
	}
	if m := md[4]; m.Stored || !m.Fetchable || strp(m.URL) != e.fileURL(860004) {
		t.Errorf("evicted small document: %+v", m)
	}

	// Opening the small photo's link fetches it; the daemon stores it
	// and names the row that holds the bytes.
	e.fake.answer(860002, 200, "", `{"state":"stored","id":860001}`)
	expect(t, e.get(strp(md[2].URL), ""), 200, "blob 860001")
	if f := e.fake.lastFetch(); !strings.HasPrefix(f, "860002 ") {
		t.Errorf("fetch %q", f)
	}

	// The big video's URL, had the visitor got it, is refused before
	// the daemon is asked.
	before := e.fake.lastFetch()
	expect(t, e.get(e.fileURL(860003), ""), 403, "")
	if e.fake.lastFetch() != before {
		t.Error("a big file was fetched for a visitor without a session")
	}
}

// TestFetchFromChatSignedIn: a signed-in viewer gets every unstored
// file linked, big ones too, and a big one streams from TDLib's cache.
func TestFetchFromChatSignedIn(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	ck, _ := e.signIn("fetcher")
	md := e.chatMedia(ck)

	if m := md[3]; m.Stored || !m.Fetchable || strp(m.URL) != e.fileURL(860003) || m.Thumb != nil {
		t.Errorf("metadata-only big video: %+v", m)
	}
	if m := md[2]; !m.Fetchable || strp(m.URL) != e.fileURL(860002) {
		t.Errorf("metadata-only small photo: %+v", m)
	}
	if m := md[5]; !m.Stored || m.Fetchable || strp(m.URL) != e.fileURL(860005) || m.Thumb != nil {
		t.Errorf("stored audio: %+v", m)
	}

	part := filepath.Join(e.tdlib, "videos", "file_860003.mp4")
	if err := os.WriteFile(part, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.fake.answer(860003, 200, "", fmt.Sprintf(
		`{"state":"partial","path":%q,"offset":0,"available":10,"size":2147483648}`, part))
	g := e.get(strp(md[3].URL), ck)
	expect(t, g, 206, "0123456789")
	expectHeader(t, g, "Content-Range", "bytes 0-9/2147483648")
	if f := e.fake.lastFetch(); !strings.HasPrefix(f, "860003 ") {
		t.Errorf("fetch %q", f)
	}
}

// TestFetchFromChatNoDaemon: without the daemon API nothing unstored
// is linked, signed in or not.
func TestFetchFromChatNoDaemon(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{noDaemon: true})
	ck, _ := e.signIn("nodaemon")
	for _, c := range []string{"", ck} {
		md := e.chatMedia(c)
		for _, n := range []int{2, 3, 4} {
			if m := md[n]; m.Fetchable || m.URL != nil {
				t.Errorf("%q message %d: %+v", c, n, m)
			}
		}
		if md[1].URL == nil {
			t.Errorf("%q: stored photo without a URL", c)
		}
	}
}

// TestFilesListFetch: the files list links every file the viewer may
// have, stored or fetchable, and opening an unstored one fetches it;
// only stored pictures have thumbnails.
func TestFilesListFetch(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	ck, _ := e.signIn("lister")

	st, rows := e.filesPage(ck)
	if st != 200 || len(rows) != 5 {
		t.Fatalf("files page: %d, %d rows", st, len(rows))
	}
	f := byFileID(rows)
	for _, c := range []struct {
		id               uint64
		stored, fetchabl bool
		thumb            bool
	}{
		{860001, true, false, true},
		{860002, false, true, false},
		{860003, false, true, false},
		{860004, false, true, false},
		{860005, true, false, false},
	} {
		r := f[fmt.Sprint(c.id)]
		wantThumb := any(nil)
		if c.thumb {
			wantThumb = e.fileURL(c.id)
		}
		if r["stored"] != c.stored || r["fetchable"] != c.fetchabl ||
			r["url"] != e.fileURL(c.id) || r["thumb"] != wantThumb {
			t.Errorf("file %d: %v", c.id, r)
		}
	}

	e.fake.answer(860004, 200, "", `{"state":"stored","id":860005}`)
	g := e.get(f["860004"]["url"].(string), ck)
	expect(t, g, 200, "blob 860005")
	// Offered as a download under its own name, not the blob's.
	expectHeader(t, g, "Content-Disposition", "attachment; filename=evicted-notes.pdf")
	if l := e.fake.lastFetch(); !strings.HasPrefix(l, "860004 ") {
		t.Errorf("fetch %q", l)
	}

	// Without a session the page is not served at all.
	if st, _ := e.filesPage(""); st != 401 {
		t.Errorf("signed out: %d", st)
	}
	// A member sees none of these files (no granted groups).
	mk, _ := e.signInAs("member1", "member")
	if st, rows := e.filesPage(mk); st != 200 || len(rows) != 0 {
		t.Errorf("member: %d %v", st, rows)
	}
}

// TestFilesListNoDaemon: without the daemon API only stored files are
// linked.
func TestFilesListNoDaemon(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{noDaemon: true})
	ck, _ := e.signIn("lister")
	_, rows := e.filesPage(ck)
	for _, r := range rows {
		stored := r["stored"] == true
		if r["fetchable"] != false || (r["url"] != nil) != stored {
			t.Errorf("file %v: %v", r["id"], r)
		}
	}
	if len(rows) != 5 {
		t.Errorf("%d rows", len(rows))
	}
}

func fixtureSHA(id uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("files fixture %d", id)))
	return hex.EncodeToString(sum[:])
}

func fileIDs(rows []map[string]any) string {
	var ids []string
	for _, r := range rows {
		ids = append(ids, js(r["id"]))
	}
	return strings.Join(ids, ",")
}

// TestFilesListColumns: the SHA-256 and last access columns, as the
// Drogon files list shows them.
func TestFilesListColumns(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	ck, _ := e.signIn("columns")
	_, rows := e.filesPage(ck)
	f := byFileID(rows)
	if r := f["860001"]; r["sha256"] != fixtureSHA(860001) ||
		r["accessed_at"] != "2026-11-17T09:00:00Z" || r["created_at"] != "2026-11-10T10:00:00Z" ||
		r["ext"] != "jpg" || r["tg_file_id"] != "AgACAgUAAxkBAAIfetch01" {
		t.Errorf("stored photo: %v", r)
	}
	// Recorded without its bytes: no digest.
	if r, ok := f["860002"]; !ok || r["sha256"] != nil || r["accessed_at"] != "2026-11-11T10:00:00Z" {
		t.Errorf("metadata-only photo: %v", r)
	}
	// Evicted: the digest is kept.
	if r := f["860004"]; r["sha256"] != fixtureSHA(860004) || r["stored"] != false {
		t.Errorf("evicted document: %v", r)
	}
}

// TestFilesListFilters: the filters by Telegram file id, SHA-256,
// last update and last access, and the sorts by id, extension and
// last access.
func TestFilesListFilters(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	ck, _ := e.signIn("filters")
	cond := func(field, op, value string) string {
		b, _ := json.Marshal(listCond{Field: field, Op: op, Value: strValue(value)})
		return "," + string(b)
	}
	for _, c := range []struct {
		conds string
		ids   string
		kept  int // conds echoed back, the id range's two included
	}{
		{cond("tg_file_id", "=", "BQACAgUAAxkBAAIfetch04"), "860004", 3},
		{cond("tg_file_id", "contains", "Ifetch0"), "860005,860004,860003,860002,860001", 3},
		{cond("sha256", "=", strings.ToUpper(fixtureSHA(860004))), "860004", 3},
		{cond("sha256", "=", fixtureSHA(860001)), "860001", 3},
		// Rows without a digest match neither = nor !=.
		{cond("sha256", "!=", fixtureSHA(860001)), "860005,860004", 3},
		// Not a digest: dropped, not echoed.
		{cond("sha256", "=", "beef"), "860005,860004,860003,860002,860001", 2},
		{cond("sha256", "=", strings.Repeat("zz", 32)), "860005,860004,860003,860002,860001", 2},
		{cond("sha256", "contains", fixtureSHA(860001)), "860005,860004,860003,860002,860001", 2},
		{cond("accessed_at", ">", "2026-11-15"), "860005,860001", 3},
		{cond("accessed_at", "<", "2026-11-12"), "860002", 3},
		{cond("updated_at", ">", "2026-11-15"), "860004", 3},
		{cond("updated_at", "<", "2026-11-11"), "860001", 3},
		{cond("accessed_at", ">", "yesterday"), "860005,860004,860003,860002,860001", 2},
	} {
		st, p := e.filesPageWith(ck, c.conds)
		if st != 200 || fileIDs(p.Rows) != c.ids || len(p.Conds) != c.kept {
			t.Errorf("%s: %d rows %s conds %v, want %s and %d conds", c.conds, st,
				fileIDs(p.Rows), p.Conds, c.ids, c.kept)
		}
	}

	for _, c := range []struct {
		sort, dir string
		ids       string
		exts      string
	}{
		{"id", "asc", "860001,860002,860003,860004,860005", ""},
		{"id", "desc", "860005,860004,860003,860002,860001", ""},
		{"accessed_at", "desc", "860001,860005,860004,860003,860002", ""},
		{"accessed_at", "asc", "860002,860003,860004,860005,860001", ""},
		{"ext", "asc", "", "jpg,jpg,mp4,ogg,pdf"},
		{"ext", "desc", "", "pdf,ogg,mp4,jpg,jpg"},
	} {
		_, p := e.filesPageWith(ck, "", "sort", c.sort, "dir", c.dir)
		var exts []string
		for _, r := range p.Rows {
			exts = append(exts, r["ext"].(string))
		}
		if p.Sort != c.sort || p.Dir != c.dir || (c.ids != "" && fileIDs(p.Rows) != c.ids) ||
			(c.exts != "" && strings.Join(exts, ",") != c.exts) {
			t.Errorf("sort %s %s: %s %s, rows %s exts %v", c.sort, c.dir, p.Sort, p.Dir,
				fileIDs(p.Rows), exts)
		}
	}
}

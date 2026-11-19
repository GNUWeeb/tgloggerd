// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/testdb"
)

// The administration operations and pages, each test on databases of
// its own: the logger's with the "sample" and "forwarding" fixtures,
// the web's with the "accounts" and "admin" fixtures (whose admin,
// gw_admin and researcher sign in here as superadmin, admin and
// member), a fake
// daemon and a fake Discord, both on loopback ports the system picks.

// adminEnv is one test's service and its fakes.
type adminEnv struct {
	t       *testing.T
	h       http.Handler
	ro, app *sql.DB
	signer  *auth.Signer
	cookies map[string]string // by role
	uids    map[string]uint64
	daemon  *joinDaemon
	discord *fakeDiscord
	// auditBase is the newest web_audit id of the fixtures.
	auditBase int64
	deps      Deps
}

// with rebuilds the service with changed dependencies.
func (e *adminEnv) with(f func(d *Deps)) {
	d := e.deps
	f(&d)
	e.h = New(d)
}

// The accounts of the "accounts" fixture each role signs in as; their
// role and federations come from the fixture's rows, as they would on
// any request.
var adminRoles = map[uint64]string{1: "superadmin", 3: "admin", 5: "member"}

// joinDaemon answers the daemon's chat and contact calls from a table
// by path, and records what it was asked.
type joinDaemon struct {
	mu      sync.Mutex
	answers map[string]func(w http.ResponseWriter)
	asked   []string // "<path and query> <body>"
	auth    string
}

func (d *joinDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	d.mu.Lock()
	d.asked = append(d.asked, r.URL.RequestURI()+" "+strings.TrimSpace(string(body)))
	f := d.answers[r.URL.Path]
	ok := r.Header.Get("Authorization") == d.auth
	d.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if f == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f(w)
}

func (d *joinDaemon) answer(path string, status int, retryAfter, body string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers[path] = func(w http.ResponseWriter) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (d *joinDaemon) calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.asked...)
}

// fakeDiscord stands in for Discord's webhook endpoint: the service's
// client is pointed at it whatever host the URL names, and it records
// which host and path were asked for.
type fakeDiscord struct {
	mu     sync.Mutex
	status int
	posts  []string // "<host><path> <body>"
	srv    *httptest.Server
}

func (f *fakeDiscord) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.posts = append(f.posts, r.URL.Host+r.URL.Path+" "+string(body))
	f.mu.Unlock()
	u, _ := url.Parse(f.srv.URL)
	r2 := r.Clone(r.Context())
	r2.URL.Scheme, r2.URL.Host, r2.Host = u.Scheme, u.Host, u.Host
	r2.Body = io.NopCloser(bytes.NewReader(body))
	return http.DefaultTransport.RoundTrip(r2)
}

func (f *fakeDiscord) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.posts...)
}

func newAdminEnv(t *testing.T, loggerFixtures ...string) *adminEnv {
	t.Helper()
	lg := testdb.New(t, testdb.Logger, append([]string{"sample", "forwarding", "bots"}, loggerFixtures...)...)
	web := testdb.New(t, testdb.Web, "accounts", "admin")
	signer := auth.NewSigner(testKey)
	ck := &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true}
	bearer, _ := daemon.Bearer(testKey)
	e := &adminEnv{t: t, ro: lg.DB, app: web.DB, signer: signer,
		cookies: map[string]string{}, uids: map[string]uint64{},
		daemon:  &joinDaemon{answers: map[string]func(http.ResponseWriter){}, auth: bearer},
		discord: &fakeDiscord{status: http.StatusNoContent}}

	dsrv := httptest.NewServer(e.daemon)
	t.Cleanup(dsrv.Close)
	dc, err := daemon.New(dsrv.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	e.discord.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.discord.mu.Lock()
		st := e.discord.status
		e.discord.mu.Unlock()
		w.WriteHeader(st)
		if st >= 300 {
			_, _ = io.WriteString(w, `{"message": "Unknown Webhook", "code": 10015}`)
		}
	}))
	t.Cleanup(e.discord.srv.Close)

	e.deps = Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      lg.DB,
		App:     web.DB,
		Signer:  signer,
		Cookies: ck,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Tokens:  auth.NewFileTokens(testKey),
		Daemon:  dc,
		discord: &http.Client{Transport: e.discord},
	}
	e.h = New(e.deps)
	for uid, r := range adminRoles {
		e.cookies[r] = signer.EncodeSession(ck.NewSession(uid, r, r, 0))
		e.uids[r] = uid
	}
	if err := web.QueryRow("SELECT MAX(id) FROM web_audit").Scan(&e.auditBase); err != nil {
		t.Fatal(err)
	}
	return e
}

// call sends a request as role ("" for no session) with the session's
// CSRF token unless csrf is false, and decodes a JSON answer.
func (e *adminEnv) call(role, method, path string, body any, csrf ...bool) (int, map[string]any, http.Header) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if role != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: e.cookies[role]})
		if len(csrf) == 0 || csrf[0] {
			req.Header.Set(CSRFHeader, e.signer.SessionCSRF(e.cookies[role]))
		}
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			e.t.Fatalf("%s %s: %d, not JSON: %q", method, path, rec.Code, rec.Body.String())
		}
	}
	return rec.Code, out, rec.Header()
}

// want sends a request and wants status; it returns the body.
func (e *adminEnv) want(status int, role, method, path string, body any) map[string]any {
	e.t.Helper()
	st, b, _ := e.call(role, method, path, body)
	if st != status {
		e.t.Fatalf("%s %s %s: %d %v, want %d", role, method, path, st, b, status)
	}
	return b
}

// page is a page's data as role sees it.
func (e *adminEnv) page(role, path string) (int, map[string]any) {
	e.t.Helper()
	st, b, _ := e.call(role, "GET", "/api/page/"+strings.TrimPrefix(path, "/"), nil)
	return st, b
}

// audits are the web_audit rows written since the fixtures', but for
// the sign-in and media rows, as "<uid> <action> <detail>".
func (e *adminEnv) audits() []string {
	e.t.Helper()
	rows, err := e.app.Query("SELECT COALESCE(web_user_id, 0), action, COALESCE(detail, '') "+
		"FROM web_audit WHERE id > ? AND action NOT IN ('login_ok', 'login_fail', 'logout', 'media') "+
		"ORDER BY id", e.auditBase)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uid, action, detail string
		if err := rows.Scan(&uid, &action, &detail); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, uid+" "+action+" "+detail)
	}
	return out
}

// one is a single value of the logger's database.
func (e *adminEnv) one(q string, args ...any) string {
	e.t.Helper()
	var v sql.NullString
	if err := e.ro.QueryRow(q, args...).Scan(&v); err != nil && err != sql.ErrNoRows {
		e.t.Fatal(err)
	}
	return v.String
}

// appOne is a single value of the web's database.
func (e *adminEnv) appOne(q string, args ...any) string {
	e.t.Helper()
	var v sql.NullString
	if err := e.app.QueryRow(q, args...).Scan(&v); err != nil && err != sql.ErrNoRows {
		e.t.Fatal(err)
	}
	return v.String
}

// canon is a JSON text as js writes the same value decoded: keys
// sorted, for comparing with answers decoded into maps.
func canon(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic(err)
	}
	return js(v)
}

// noSecrets fails when an answer carries any of the fixtures' or the
// test's secrets.
func noSecrets(t *testing.T, what string, v any, secrets ...string) {
	t.Helper()
	s := js(v)
	for _, x := range append(secrets, "fixtureSecretToken", "fixtureBotSecret") {
		if strings.Contains(s, x) {
			t.Errorf("%s carries a secret (%s): %s", what, x, s)
		}
	}
}

// Every text the administration files look up has an English text.
func TestAdminTextsExist(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`"((?:federation|fwd|group|join|daemon|lookup|mcp)\.[a-z_]+)"`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllSubmatch(b, -1) {
			n++
			if k := string(m[1]); i18n.T(i18n.Fallback, k) == k {
				t.Errorf("%s: %q has no English text", f, k)
			}
		}
	}
	if n == 0 {
		t.Error("no texts found")
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/render"
	"tgloggerd/webapi/internal/render/rendertest"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/webdb"
)

// The page side: routing and path decoding, the sign-in rules, the
// document through a stand-in renderer and its fallback. Sessions are
// checked against a fake epoch table, so no database is needed.

const testBuild = "b1"

type pagesEnv struct {
	t       *testing.T
	h       http.Handler
	cookies *auth.Cookies
	signer  *auth.Signer
	standin *rendertest.StandIn
	// epochs and roles are the fake web_users rows (session_epoch,
	// role); a missing id is a deleted account, epochErr a database
	// that does not answer.
	epochs   map[uint64]uint32
	roles    map[uint64]string
	epochErr error
}

type pagesOpts struct {
	pages   []pageRoute
	timeout time.Duration
	assets  string // the bundle's assets/ directory, "" for none
}

func newPagesEnv(t *testing.T, o pagesOpts) *pagesEnv {
	t.Helper()
	signer := auth.NewSigner(testKey)
	e := &pagesEnv{
		t:       t,
		signer:  signer,
		cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		standin: rendertest.Start(t, testBuild),
		epochs:  map[uint64]uint32{1: 0, 2: 0},
		roles:   map[uint64]string{1: "superadmin", 2: "member"},
	}
	if o.timeout == 0 {
		o.timeout = 2 * time.Second
	}
	var root *os.Root
	if o.assets != "" {
		var err error
		if root, err = os.OpenRoot(o.assets); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { root.Close() })
	}
	e.h = New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Signer:  signer,
		Cookies: e.cookies,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Render:  render.NewClient(e.standin.Socket, o.timeout, time.Hour),
		Assets: render.Assets{Build: testBuild, CSS: []string{"/app/assets/a.css"},
			Entry: "/app/assets/entry.js"},
		AssetFiles: root,
		pages:      o.pages,
		prefs: func(context.Context, uint64) (settings.Prefs, error) {
			return settings.Prefs{}, nil
		},
		pins: func(context.Context, uint64, scope.Scope) ([]logdb.PinnedGroup, error) {
			return nil, nil
		},
		state: func(_ context.Context, uid uint64, _ [32]byte) (webdb.SessionState, error) {
			if e.epochErr != nil {
				return webdb.SessionState{}, e.epochErr
			}
			ep, ok := e.epochs[uid]
			if !ok {
				return webdb.SessionState{}, webdb.ErrNoUser
			}
			// A recorded, fresh session: nothing to write.
			return webdb.SessionState{Epoch: ep, Role: e.roles[uid], Active: true,
				SessionRow: 1}, nil
		},
	})
	return e
}

// session returns a session cookie value for account uid.
func (e *pagesEnv) session(uid uint64, name, role string) string {
	return e.signer.EncodeSession(e.cookies.NewSession(uid, name, role, e.epochs[uid]))
}

type pageGot struct {
	status int
	header http.Header
	body   string
}

func (e *pagesEnv) get(path, cookie string) pageGot {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: cookie})
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return pageGot{rec.Code, rec.Header(), rec.Body.String()}
}

// page decodes a /api/page answer, with the clock zeroed so answers can
// be compared.
func (g pageGot) page(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(g.body), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, g.body)
	}
	if v, ok := m["viewer"].(map[string]any); ok {
		if _, err := time.Parse(time.RFC3339, v["now"].(string)); err != nil {
			t.Errorf("viewer.now %v", v["now"])
		}
		v["now"] = ""
	}
	return m
}

var embedded = regexp.MustCompile(`<script type="application/json" id="__tgd">(.*?)</script>`)

// docPage is a document's embedded page data, like page.
func (g pageGot) docPage(t *testing.T) map[string]any {
	t.Helper()
	m := embedded.FindStringSubmatch(g.body)
	if m == nil {
		t.Fatalf("no page data in %s", g.body)
	}
	return pageGot{body: m[1]}.page(t)
}

func TestPagePathDecoding(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	ck := e.session(1, "admin", "admin")
	for _, c := range []struct {
		path   string
		status int
		route  string // "" for {"reload": true}
	}{
		{"/api/page/login", 200, "login"},
		{"/api/page/login%2F", 200, "login"},
		{"/api/page/password%2Fforgot", 200, "password_forgot"}, // %2F decoded
		{"/api/page/account/email/verify", 200, "email_verify"}, // plain slashes too
		{"/api/page/password%2Fset%2F", 200, "password_set"},
		{"/api/page/messages%2Fother%2F7", 404, "not_found"},
		{"/api/page/users%2Fabc", 404, "not_found"},
		{"/api/page/users%2F..%2Flogin", 404, "not_found"},
		{"/api/page/users%2F%2F5", 404, "not_found"},
		{"/api/page/users%252F5", 404, "not_found"}, // decoded once only
		{"/api/page/nothing", 404, "not_found"},
	} {
		g := e.get(c.path, ck)
		if g.status != c.status {
			t.Errorf("%s: status %d, want %d: %s", c.path, g.status, c.status, g.body)
			continue
		}
		m := g.page(t)
		if c.route == "" {
			if !reflect.DeepEqual(m, map[string]any{"reload": true}) {
				t.Errorf("%s: %s, want reload", c.path, g.body)
			}
			continue
		}
		if m["route"] != c.route || m["v"] != 1.0 || int(m["status"].(float64)) != c.status {
			t.Errorf("%s: %s", c.path, g.body)
		}
	}
}

func TestPageNotFoundShape(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	g := e.get("/api/page/no%2Fsuch?x=1", e.session(1, "admin", "admin"))
	want := map[string]any{
		"v": 1.0, "route": "not_found", "url": "/no/such?x=1", "status": 404.0,
		"props": map[string]any{},
		"viewer": map[string]any{
			"signed_in": true, "now": "",
			"account": map[string]any{"id": 1.0, "username": "admin", "name": "admin", "role": "superadmin"},
			"nav":     toAny(scope.For(scope.Viewer{ID: 1, Role: scope.Superadmin}).Nav()),
			// The site defaults, as the account set nothing.
			"timezone": "Asia/Jakarta", "date_format": "24h", "lang": "en", "theme": "system",
			"sidebar": "open", "dashboard_numbers": "exact", "rows_per_page": 25.0,
			"dashboard_period": "hour", "dashboard_variant": "bars", "dashboard_last": 172800.0,
			"chat": map[string]any{"auto_load": true, "live": true, "media": "auto",
				"show_deleted": true, "show_edited": true, "show_service": true},
			// No daemon in this env: the logging account is unknown.
			"logger": nil,
		},
	}
	if got := g.page(t); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
	if cc := g.header.Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func TestPageLoginNext(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	for q, want := range map[string]any{
		"":                         nil,
		"?next=%2Fusers%3Fq%3Dx":   "/users?q=x",
		"?next=%2F%2Fevil.example": nil,
		"?next=%2F%5Cevil.example": nil,
		"?next=https%3A%2F%2Fx":    nil,
		"?next=%2Fa%0Ab":           nil,
	} {
		g := e.get("/api/page/login"+q, "")
		props := g.page(t)["props"].(map[string]any)
		if props["next"] != want {
			t.Errorf("%q: next %v, want %v", q, props["next"], want)
		}
	}
}

func TestPageSignedOut(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	for path, status := range map[string]int{
		"/api/page/":                         401,
		"/api/page/users":                    401,
		"/api/page/nothing":                  401, // existing or not
		"/api/page/login":                    200,
		"/api/page/password%2Fforgot":        200,
		"/api/page/account%2Femail%2Fverify": 200,
		"/api/page/password%2Fset":           200,
	} {
		g := e.get(path, "")
		if g.status != status {
			t.Errorf("%s: %d, want %d: %s", path, g.status, status, g.body)
		}
		if status == 401 && !strings.Contains(g.body, `"unauthenticated"`) {
			t.Errorf("%s: %s", path, g.body)
		}
	}
	g := e.get("/api/page/login", "")
	if cc := g.header.Get("Cache-Control"); cc != "public, max-age=60" {
		t.Errorf("public page without a cookie: Cache-Control %q", cc)
	}
	if v := g.page(t)["viewer"].(map[string]any); v["signed_in"] != false || v["account"] != nil || v["nav"] != nil {
		t.Errorf("signed-out viewer %v", v)
	}
}

func TestPageSessionStates(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	ck := e.session(1, "admin", "admin")

	// An epoch bumped since (a password change elsewhere): signed out,
	// and the cookie is cleared.
	e.epochs[1] = 7
	g := e.get("/api/page/users", ck)
	if g.status != 401 || !strings.Contains(g.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Errorf("revoked: %d %q", g.status, g.header.Get("Set-Cookie"))
	}
	e.epochs[1] = 0

	// A database that does not answer: 503, the cookie kept.
	e.epochErr = errors.New("down")
	g = e.get("/api/page/users", ck)
	if g.status != 503 || g.header.Get("Set-Cookie") != "" {
		t.Errorf("unverified: %d %q %s", g.status, g.header.Get("Set-Cookie"), g.body)
	}
	if g = e.get("/login", ck); g.status != 503 {
		t.Errorf("unverified document: %d", g.status)
	}
	e.epochErr = nil

	// A member's viewer role.
	m := e.get("/api/page/login", e.session(2, "researcher", "viewer")).page(t)
	if r := m["viewer"].(map[string]any)["account"].(map[string]any)["role"]; r != "member" {
		t.Errorf("viewer role %v", r)
	}
}

// The document carries the same page data /api/page answers, rendered
// by the render service.
func TestPageDocumentRendered(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	g := e.get("/login?next=%2Fusers", "")
	if g.status != 200 || g.header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("%d %v", g.status, g.header)
	}
	for _, want := range []string{
		`<div id="root" data-ssr="1"><main data-route="login">{"next":"/users"}</main></div>`,
		`<title>login</title>`,
		`<link rel="stylesheet" href="/app/assets/a.css">`,
		`<script type="module" src="/app/assets/entry.js"></script>`,
	} {
		if !strings.Contains(g.body, want) {
			t.Errorf("missing %s in\n%s", want, g.body)
		}
	}
	if strings.Contains(g.body, "csrf-token") {
		t.Error("signed-out document has a CSRF token")
	}
	api := e.get("/api/page/login?next=%2Fusers", "").page(t)
	if doc := g.docPage(t); !reflect.DeepEqual(doc, api) {
		t.Errorf("document data %v\n/api/page    %v", doc, api)
	}
	if e.standin.Calls.Load() != 1 {
		t.Errorf("%d renders", e.standin.Calls.Load())
	}
	var sent map[string]any
	_ = json.Unmarshal(e.standin.Last.Load().([]byte), &sent)
	if sent["build"] != testBuild || sent["route"] != "login" || sent["request_id"] == "" {
		t.Errorf("render request %v", sent)
	}
	if _, ok := sent["csrf"]; ok {
		t.Error("the render service was sent a CSRF token")
	}
}

// The CSRF token is the one of the cookie the request carries: a
// reissued session cookie gets its own.
func TestPageDocumentCSRF(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	ck := e.session(1, "admin", "admin")
	e.epochs[1] = 1
	ck2 := e.session(1, "admin", "admin")
	g := e.get("/login", ck2)
	want := `<meta name="csrf-token" content="` + e.signer.SessionCSRF(ck2) + `">`
	if !strings.Contains(g.body, want) || e.signer.SessionCSRF(ck) == e.signer.SessionCSRF(ck2) {
		t.Errorf("missing %s in\n%s", want, g.body)
	}
	if cc := g.header.Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("signed-in document: Cache-Control %q", cc)
	}
}

// A stopped renderer gives the shell with the same data, not an error.
func TestPageDocumentFallbackDead(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	e.standin.Stop()
	for i := 0; i < 2; i++ { // the second skips the renderer
		g := e.get("/login", "")
		if g.status != 200 || !strings.Contains(g.body, `<div id="root" data-ssr="0"></div>`) ||
			!strings.Contains(g.body, "<title>tgloggerd</title>") {
			t.Fatalf("fallback %d:\n%s", g.status, g.body)
		}
		if g.docPage(t)["route"] != "login" {
			t.Errorf("fallback data %s", g.body)
		}
	}
}

func TestPageDocumentFallbackSlow(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{timeout: 100 * time.Millisecond})
	e.standin.SetDelay(3 * time.Second)
	start := time.Now()
	g := e.get("/login", "")
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("slow renderer held the page for %v", d)
	}
	if g.status != 200 || !strings.Contains(g.body, `data-ssr="0"`) {
		t.Errorf("fallback %d:\n%s", g.status, g.body)
	}
	// Backed off: the next page does not wait at all.
	start = time.Now()
	e.get("/login", "")
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Errorf("backed-off page took %v", d)
	}
}

// A table with a private page whose loader hides id 2, to check the
// redirect and not-found rules on documents.
func secretPages() []pageRoute {
	return []pageRoute{
		{id: "login", pattern: "/login", public: true, load: loadLogin},
		{id: "secret", pattern: "/secret/{id}", load: func(_ context.Context, pr *pageRequest) (any, error) {
			if pr.params["id"] == "2" {
				return nil, errPageNotFound
			}
			return map[string]string{"id": pr.params["id"]}, nil
		}},
		{id: "later", pattern: "/later"},
	}
}

func TestPageRedirects(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{pages: secretPages()})
	for path, loc := range map[string]string{
		"/secret/1?a=b&c=%2F": "/login?next=%2Fsecret%2F1%3Fa%3Db%26c%3D%252F",
		"/secret/2":           "/login?next=%2Fsecret%2F2",
		"/secret/x":           "/login?next=%2Fsecret%2Fx",
	} {
		g := e.get(path, "")
		if g.status != http.StatusFound || g.header.Get("Location") != loc {
			t.Errorf("%s: %d %q, want %q", path, g.status, g.header.Get("Location"), loc)
		}
	}
	// Signed in: the page, and hidden and missing alike answer 404.
	ck := e.session(1, "admin", "admin")
	if g := e.get("/secret/1", ck); g.status != 200 || g.docPage(t)["route"] != "secret" {
		t.Errorf("secret/1: %d %s", g.status, g.body)
	}
	hidden := e.get("/secret/2", ck)
	if hidden.status != 404 || hidden.docPage(t)["route"] != "not_found" {
		t.Errorf("hidden: %d %s", hidden.status, hidden.body)
	}
	a := e.get("/api/page/secret%2F2", ck)
	b := e.get("/api/page/secret%2Fx", ck)
	if a.status != 404 || b.status != 404 {
		t.Errorf("api not found: %d %d", a.status, b.status)
	}
	pa, pb := a.page(t), b.page(t)
	delete(pa, "url")
	delete(pb, "url")
	if !reflect.DeepEqual(pa, pb) {
		t.Errorf("hidden and missing differ:\n%v\n%v", pa, pb)
	}
	// Pages without a loader have no document here.
	if g := e.get("/later", ck); g.status != 404 || strings.Contains(g.body, "__tgd") {
		t.Errorf("/later: %d %s", g.status, g.body)
	}
	if g := e.get("/api/page/later", ck); g.body != "{\"reload\":true}\n" {
		t.Errorf("/api/page/later: %s", g.body)
	}
}

func TestPageDocumentsRegistered(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	ck := e.session(1, "admin", "admin")
	// Every page has its loader now, the link pages included, so each
	// document is served here.
	for _, p := range []string{"/password/forgot", "/password/set", "/account/email/verify"} {
		if g := e.get(p, ck); g.status != 200 || !strings.Contains(g.body, "__tgd") {
			t.Errorf("%s not served here: %d", p, g.status)
		}
	}
}

func TestPageAssets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assets := filepath.Join(dir, "assets")
	if err := os.Mkdir(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "entry-x.js"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newPagesEnv(t, pagesOpts{assets: assets})
	g := e.get("/app/assets/entry-x.js", "")
	if g.status != 200 || g.body != "1" ||
		g.header.Get("Cache-Control") != "public, max-age=31536000, immutable" ||
		!strings.HasPrefix(g.header.Get("Content-Type"), "text/javascript") {
		t.Errorf("asset: %d %v %q", g.status, g.header, g.body)
	}
	for _, p := range []string{"/app/assets/none.js", "/app/assets/%2e%2e%2fsecret", "/app/assets/"} {
		if g := e.get(p, ""); g.status != 404 {
			t.Errorf("%s: %d %q", p, g.status, g.body)
		}
	}
}

// A page opened with a trailing slash is served as without it, in
// place: "/files/" is the files page, not a 404 (and never a redirect,
// which would loop in browsers that kept nginx's old permanent
// redirect from /files to /files/).
func TestPageTrailingSlash(t *testing.T) {
	t.Parallel()
	e := newPagesEnv(t, pagesOpts{})
	g := e.get("/login/", "")
	if g.status != 200 || !strings.Contains(g.body, `<main data-route="login">`) {
		t.Fatalf("/login/: %d %s", g.status, g.body)
	}
	for _, p := range []string{"/users", "/files", "/group_messages", "/settings/preferences"} {
		bare, slashed := e.get(p, ""), e.get(p+"/", "")
		if bare.status == 404 || slashed.status != bare.status {
			t.Errorf("%s: %d, %s/: %d", p, bare.status, p, slashed.status)
		}
		if slashed.status/100 == 3 && !strings.HasPrefix(slashed.header.Get("Location"), "/login") {
			t.Errorf("%s/ redirects to %s", p, slashed.header.Get("Location"))
		}
	}
	if g := e.get("/nothing/", ""); g.status != 404 {
		t.Errorf("/nothing/: %d", g.status)
	}
}

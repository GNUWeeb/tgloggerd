// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/auth"
)

// These tests sign in against a real web schema. They run only when
// WEBAPI_TEST_APP_DSN names a MySQL database whose name contains
// "test", with the web migrations (web/migrations) applied; they
// empty web_users and web_audit in it.

var testKey = []byte("0123456789abcdef0123456789abcdef")

func testDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("WEBAPI_TEST_APP_DSN")
	if dsn == "" {
		t.Skip("WEBAPI_TEST_APP_DSN is not set")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.DBName, "test") {
		t.Fatalf("refusing to empty tables in %q: not a test database", cfg.DBName)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{"DELETE FROM web_mcp_tokens", "DELETE FROM web_audit",
		"DELETE FROM web_users"} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func addUser(t *testing.T, db *sql.DB, name, hash, role string, active bool) uint64 {
	res, err := db.Exec("INSERT INTO web_users (username, password_hash, role, is_active) "+
		"VALUES (?, ?, ?, ?)", name, hash, role, active)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return uint64(id)
}

type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
	addr    string // the connection's peer, "" for httptest's default
}

type answer struct {
	status int
	body   map[string]any
	set    []*http.Cookie
}

// do sends a request with the client's cookies and keeps the cookies
// the answer sets, as a browser would (a cleared cookie is dropped).
func (c *client) do(method, path, csrf string, body any) answer {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rd)
	if c.addr != "" {
		r.RemoteAddr = c.addr
	}
	for k, v := range c.cookies {
		r.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	if csrf != "" {
		r.Header.Set(CSRFHeader, csrf)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	res := w.Result()
	a := answer{status: res.StatusCode, set: res.Cookies()}
	for _, ck := range a.set {
		if ck.MaxAge < 0 {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck.Value
		}
	}
	_ = json.NewDecoder(res.Body).Decode(&a.body)
	return a
}

func newHandler(db *sql.DB, maxAttempts int) http.Handler {
	signer := auth.NewSigner(testKey)
	return New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      db,
		App:     db,
		Signer:  signer,
		Cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		Limiter: auth.NewRateLimiter(maxAttempts, time.Hour),
	})
}

type auditRow struct {
	uid    sql.NullInt64
	action string
	ip     string
	detail string
}

func audits(t *testing.T, db *sql.DB) []auditRow {
	rows, err := db.Query("SELECT web_user_id, action, ip, detail FROM web_audit ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var a auditRow
		if err := rows.Scan(&a.uid, &a.action, &a.ip, &a.detail); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

func (c *client) login(user, pw string) answer {
	tok := c.do("GET", "/api/login", "", nil).body["csrf"].(string)
	return c.do("POST", "/api/login", tok, map[string]string{"username": user, "password": pw})
}

func TestLoginSessionLogout(t *testing.T) {
	db := testDB(t)
	hash, _ := auth.HashPassword("s3cret")
	alice := addUser(t, db, "alice", hash, "member", true)
	addUser(t, db, "bob", hash, "admin", false)
	c := &client{t: t, h: newHandler(db, 5), cookies: map[string]string{}}

	if a := c.do("GET", "/api/session", "", nil); a.status != 401 {
		t.Fatalf("session without a cookie: %d", a.status)
	}

	a := c.do("GET", "/api/login", "", nil)
	tok, _ := a.body["csrf"].(string)
	if a.status != 200 || tok == "" || a.body["signed_in"] != false {
		t.Fatalf("GET /api/login: %d %v", a.status, a.body)
	}
	if len(a.set) != 1 || a.set[0].Name != "tgw_csrf" || a.set[0].Path != "/api/login" ||
		!a.set[0].HttpOnly || !a.set[0].Secure || a.set[0].MaxAge != 1800 {
		t.Fatalf("login cookie %+v", a.set)
	}

	creds := map[string]string{"username": "alice", "password": "s3cret"}
	a = c.do("POST", "/api/login", "", creds)
	if a.status != 403 || a.body["error"] != "csrf" || a.body["csrf"] == "" {
		t.Fatalf("login without CSRF: %d %v", a.status, a.body)
	}
	tok = a.body["csrf"].(string)

	a = c.do("POST", "/api/login", tok, map[string]string{"username": "alice", "password": "nope"})
	if a.status != 401 || a.body["error"] != "invalid_credentials" ||
		a.body["message"] != "Invalid username or password." {
		t.Fatalf("wrong password: %d %v", a.status, a.body)
	}
	tok = a.body["csrf"].(string)

	a = c.do("POST", "/api/login", tok, creds)
	if a.status != 200 {
		t.Fatalf("login: %d %v", a.status, a.body)
	}
	raw := c.cookies["tgw_session"]
	signer := auth.NewSigner(testKey)
	if raw == "" || a.body["csrf"] != signer.SessionCSRF(raw) {
		t.Fatalf("login answer %v, cookie %q", a.body, raw)
	}
	user := a.body["user"].(map[string]any)
	if user["username"] != "alice" || user["role"] != "member" || user["id"] != float64(alice) {
		t.Fatalf("login user %v", user)
	}

	a = c.do("GET", "/api/session", "", nil)
	if a.status != 200 || a.body["csrf"] != signer.SessionCSRF(raw) {
		t.Fatalf("session: %d %v", a.status, a.body)
	}
	if a = c.do("GET", "/api/login", "", nil); a.body["signed_in"] != true {
		t.Fatalf("GET /api/login when signed in: %v", a.body)
	}

	// A password change on either service bumps the epoch: every
	// session issued before it ends, and its cookie is cleared.
	if _, err := db.Exec("UPDATE web_users SET session_epoch = session_epoch + 1 WHERE id = ?", alice); err != nil {
		t.Fatal(err)
	}
	a = c.do("GET", "/api/session", "", nil)
	if a.status != 401 || len(a.set) != 1 || a.set[0].MaxAge >= 0 {
		t.Fatalf("revoked session: %d %+v", a.status, a.set)
	}
	if _, ok := c.cookies["tgw_session"]; ok {
		t.Fatal("the revoked cookie was kept")
	}

	// Usernames compare as the column's collation says, and the
	// session carries the stored name.
	a = c.login("ALICE", "s3cret")
	if a.status != 200 || a.body["user"].(map[string]any)["username"] != "alice" {
		t.Fatalf("login as ALICE: %d %v", a.status, a.body)
	}
	csrf := a.body["csrf"].(string)
	if a = c.do("POST", "/api/logout", "wrong", nil); a.status != 403 {
		t.Fatalf("logout with a wrong token: %d", a.status)
	}
	if a = c.do("POST", "/api/logout", csrf, nil); a.status != 204 {
		t.Fatalf("logout: %d", a.status)
	}
	if _, ok := c.cookies["tgw_session"]; ok {
		t.Fatal("logout kept the cookie")
	}
	if a = c.do("GET", "/api/session", "", nil); a.status != 401 {
		t.Fatalf("session after logout: %d", a.status)
	}

	if a = c.login("bob", "s3cret"); a.status != 401 {
		t.Fatalf("an inactive account signed in: %d", a.status)
	}
	if a = c.login("nobody", "s3cret"); a.status != 401 {
		t.Fatalf("an unknown account signed in: %d", a.status)
	}

	ip := "192.0.2.1" // httptest's peer address
	uid := sql.NullInt64{Int64: int64(alice), Valid: true}
	want := []auditRow{
		{action: "login_fail", ip: ip, detail: "alice"},
		{uid: uid, action: "login_ok", ip: ip, detail: "alice"},
		{uid: uid, action: "login_ok", ip: ip, detail: "alice"},
		{uid: uid, action: "logout", ip: ip, detail: "alice"},
		{action: "login_fail", ip: ip, detail: "bob"},
		{action: "login_fail", ip: ip, detail: "nobody"},
	}
	got := audits(t, db)
	if len(got) != len(want) {
		t.Fatalf("audit rows %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("audit row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	db := testDB(t)
	hash, _ := auth.HashPassword("s3cret")
	addUser(t, db, "alice", hash, "admin", true)
	c := &client{t: t, h: newHandler(db, 2), cookies: map[string]string{}}

	for i := 0; i < 2; i++ {
		if a := c.login("alice", "bad"); a.status != 401 {
			t.Fatalf("failure %d: %d", i, a.status)
		}
	}
	a := c.login("alice", "s3cret")
	if a.status != 429 || a.body["error"] != "rate_limited" {
		t.Fatalf("throttled login: %d %v", a.status, a.body)
	}
	if a := c.login("someone", "bad"); a.status != 401 {
		t.Fatalf("another username is throttled: %d", a.status)
	}
	rows := audits(t, db)
	if len(rows) != 4 || rows[2].action != "login_ratelimited" || rows[2].detail != "alice" ||
		rows[2].uid.Valid {
		t.Fatalf("audit rows %+v", rows)
	}
}

func TestSessionUnverified(t *testing.T) {
	db := testDB(t)
	hash, _ := auth.HashPassword("s3cret")
	addUser(t, db, "alice", hash, "admin", true)
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	if a := c.login("alice", "s3cret"); a.status != 200 {
		t.Fatalf("login: %d", a.status)
	}
	db.Close()
	a := c.do("GET", "/api/session", "", nil)
	if a.status != 503 || len(a.set) != 0 || c.cookies["tgw_session"] == "" {
		t.Fatalf("session with the database away: %d %+v", a.status, a.set)
	}
}

// A username longer than any account's fails like a wrong password and
// is audited cut to the column, instead of failing the audit insert.
func TestLoginLongUsername(t *testing.T) {
	db := testDB(t)
	hash, _ := auth.HashPassword("s3cret")
	addUser(t, db, "alice", hash, "admin", true)
	c := &client{t: t, h: newHandler(db, 2), cookies: map[string]string{}}

	long := strings.Repeat("é", 300)
	for i := 0; i < 2; i++ {
		if a := c.login(long, "bad"); a.status != 401 ||
			a.body["error"] != "invalid_credentials" {
			t.Fatalf("long username %d: %d %v", i, a.status, a.body)
		}
	}
	// The throttle keys on the first maxUsername characters.
	if a := c.login(long[:2*maxUsername]+"x", "bad"); a.status != 429 {
		t.Fatalf("long username is not throttled: %d", a.status)
	}
	if a := c.login("alice", "s3cret"); a.status != 200 {
		t.Fatalf("alice: %d", a.status)
	}
	rows := audits(t, db)
	if len(rows) != 4 {
		t.Fatalf("audit rows %+v", rows)
	}
	if got := []rune(rows[0].detail); len(got) != 255 || string(got) != strings.Repeat("é", 255) {
		t.Fatalf("audit detail is %d characters", len(got))
	}
}

// The account cap stops guesses spread over many addresses, and counts
// the spellings of a name that find the same account together.
func TestLoginAccountCap(t *testing.T) {
	db := testDB(t)
	hash, _ := auth.HashPassword("s3cret")
	addUser(t, db, "alice", hash, "admin", true)
	signer := auth.NewSigner(testKey)
	h := New(Deps{
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:          db,
		App:         db,
		Signer:      signer,
		Cookies:     &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		Limiter:     auth.NewRateLimiter(100, time.Hour),
		AcctLimiter: auth.NewRateLimiter(3, time.Hour),
	})
	from := func(i int) *client {
		return &client{t: t, h: h, cookies: map[string]string{},
			addr: fmt.Sprintf("192.0.2.%d:4000", i)}
	}

	for i, name := range []string{"alice", "ALICE", "\u00e1lice"} {
		if a := from(i+1).login(name, "bad"); a.status != 401 {
			t.Fatalf("failure %d (%s): %d %v", i, name, a.status, a.body)
		}
	}
	if a := from(9).login("alice", "s3cret"); a.status != 429 {
		t.Fatalf("account not capped from a new address: %d %v", a.status, a.body)
	}

	// A name that finds no account is capped on the name itself.
	for i := 0; i < 3; i++ {
		from(20+i).login("nobody", "bad")
	}
	if a := from(30).login("NOBODY", "bad"); a.status != 429 {
		t.Fatalf("unknown name not capped: %d", a.status)
	}
}

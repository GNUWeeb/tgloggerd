// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
)

// The record of each session (web_sessions): sign-in, first sight,
// last use, sign-out and the cookie reissued by a password change.

const (
	firefox = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	android = "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36"
)

type sessionRec struct {
	id             uint64
	epoch          uint32
	expires        int64
	ua, ip         string
	signedIn, last int64
	revoked        bool
}

// sessionRow reads the row of a cookie value; ok is false without one.
func sessionRow(t *testing.T, db *sql.DB, raw string) (sessionRec, bool) {
	t.Helper()
	k := auth.SessionKey(raw)
	var r sessionRec
	err := db.QueryRow("SELECT id, epoch, expires, user_agent, ip, "+
		"UNIX_TIMESTAMP(signed_in_at), UNIX_TIMESTAMP(last_used_at), revoked_at IS NOT NULL "+
		"FROM web_sessions WHERE token_hash = ?", k[:]).
		Scan(&r.id, &r.epoch, &r.expires, &r.ua, &r.ip, &r.signedIn, &r.last, &r.revoked)
	if err == sql.ErrNoRows {
		return r, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return r, true
}

func sessionCount(t *testing.T, db *sql.DB, uid uint64) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_sessions WHERE web_user_id = ?", uid).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSessionRecorded(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("s3cret")
	uid := addUser(t, db, "dewi", hash, "admin", true)
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}, ua: firefox, addr: "198.51.100.4:4000"}
	before := time.Now().Unix()
	signIn(t, c, "dewi", "s3cret")
	raw := c.cookies[auth.SessionCookie]
	r, ok := sessionRow(t, db, raw)
	if !ok || r.ua != firefox || r.ip != "198.51.100.4" || r.revoked || r.epoch != 0 ||
		r.signedIn < before-1 || r.expires < before+3600 {
		t.Fatalf("session after sign-in: %+v %v", r, ok)
	}

	// A use within a minute of the last records nothing; a later one
	// records the time and the address.
	if _, err := db.Exec("UPDATE web_sessions SET last_used_at = NOW() - INTERVAL 1 HOUR, "+
		"ip = 'old' WHERE id = ?", r.id); err != nil {
		t.Fatal(err)
	}
	c.addr = "198.51.100.5:4000"
	if a := c.do("GET", "/api/session", "", nil); a.status != 200 {
		t.Fatalf("session: %d", a.status)
	}
	r2, _ := sessionRow(t, db, raw)
	if r2.ip != "198.51.100.5" || r2.last < before-1 || r2.signedIn != r.signedIn {
		t.Fatalf("after a use: %+v", r2)
	}
	if _, err := db.Exec("UPDATE web_sessions SET ip = 'kept' WHERE id = ?", r.id); err != nil {
		t.Fatal(err)
	}
	c.do("GET", "/api/session", "", nil)
	if r3, _ := sessionRow(t, db, raw); r3.ip != "kept" {
		t.Fatalf("a use within the minute was recorded: %+v", r3)
	}

	// A copy of the cookie stops working once the session signs out,
	// on this service; the Drogon web reads the same row.
	thief := &client{t: t, h: h, cookies: map[string]string{auth.SessionCookie: raw}}
	csrf := auth.NewSigner(testKey).SessionCSRF(raw)
	if a := c.do("POST", "/api/logout", csrf, nil); a.status != 204 {
		t.Fatalf("logout: %d", a.status)
	}
	if a := thief.do("GET", "/api/session", "", nil); a.status != 401 {
		t.Fatalf("a signed-out session's copy: %d", a.status)
	}
	if r, _ := sessionRow(t, db, raw); !r.revoked {
		t.Fatal("the signed-out session is not ended")
	}
	if n := sessionCount(t, db, uid); n != 1 {
		t.Fatalf("%d session rows", n)
	}
}

// A session signed in elsewhere (by the Drogon web, or before sessions
// were recorded) is recorded when first seen, with the start its
// expiry implies; signing it out ends it even before it was seen.
func TestSessionFirstSeen(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	uid := addUser(t, db, "rizky", "x", "admin", true)
	h := newHandler(db, 5)
	signer := auth.NewSigner(testKey)
	cookies := &auth.Cookies{Signer: signer, Timeout: 3600}
	drogon := cookies.NewSession(uid, "rizky", "admin", 0)
	drogon.Exp -= 600 // signed in ten minutes ago
	raw := signer.EncodeSession(drogon)
	c := &client{t: t, h: h, cookies: map[string]string{auth.SessionCookie: raw}, ua: android}
	if a := c.do("GET", "/api/session", "", nil); a.status != 200 {
		t.Fatalf("session: %d %v", a.status, a.body)
	}
	r, ok := sessionRow(t, db, raw)
	if !ok || r.ua != android || r.expires != drogon.Exp {
		t.Fatalf("first seen: %+v %v", r, ok)
	}
	if d := time.Now().Unix() - 600 - r.signedIn; d < -2 || d > 2 {
		t.Fatalf("signed in %d, want about ten minutes ago", r.signedIn)
	}
	// A page request records it too (the same row, once).
	c.do("GET", "/api/page/login", "", nil)
	if n := sessionCount(t, db, uid); n != 1 {
		t.Fatalf("%d rows", n)
	}

	unseen := cookies.NewSession(uid, "rizky", "admin", 0)
	unseen.Exp -= 1200
	raw2 := signer.EncodeSession(unseen)
	c2 := &client{t: t, h: h, cookies: map[string]string{auth.SessionCookie: raw2}}
	if a := c2.do("POST", "/api/logout", signer.SessionCSRF(raw2), nil); a.status != 204 {
		t.Fatalf("logout: %d", a.status)
	}
	c2.cookies[auth.SessionCookie] = raw2
	if a := c2.do("GET", "/api/session", "", nil); a.status != 401 {
		t.Fatalf("an unseen session after its logout: %d", a.status)
	}
}

// Sessions that never expire get distinct cookies too, so each has its
// own row: two sign-ins in the same second differ in their expiry.
func TestSessionsNeverExpiring(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("s3cret")
	uid := addUser(t, db, "sari", hash, "member", true)
	signer := auth.NewSigner(testKey)
	h := New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      db,
		App:     db,
		Signer:  signer,
		Cookies: &auth.Cookies{Signer: signer, Timeout: 0, Secure: true},
		Limiter: auth.NewRateLimiter(5, time.Hour),
	})
	seen := map[string]bool{}
	for i := range 3 {
		c := &client{t: t, h: h, cookies: map[string]string{}}
		signIn(t, c, "sari", "s3cret")
		raw := c.cookies[auth.SessionCookie]
		s, ok := (&auth.Cookies{Signer: signer}).OpenSession(raw)
		if !ok || s.Exp != auth.NeverExpires-int64(i) || seen[raw] {
			t.Fatalf("sign-in %d: exp %d, seen %v", i, s.Exp, seen[raw])
		}
		seen[raw] = true
		if a := c.do("GET", "/api/session", "", nil); a.status != 200 {
			t.Fatalf("session %d: %d", i, a.status)
		}
	}
	if n := sessionCount(t, db, uid); n != 3 {
		t.Fatalf("%d rows for 3 sessions", n)
	}
}

// A password change reissues this session's cookie: the new cookie
// keeps the session's start and device, the old rows go.
func TestSessionPasswordChangeKeepsRecord(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("old-secret")
	uid := addUser(t, db, "budi", hash, "admin", true)
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}, ua: firefox}
	other := &client{t: t, h: h, cookies: map[string]string{}, ua: android}
	tok := signIn(t, c, "budi", "old-secret")
	signIn(t, other, "budi", "old-secret")
	old := c.cookies[auth.SessionCookie]
	if _, err := db.Exec("UPDATE web_sessions SET signed_in_at = NOW() - INTERVAL 3 DAY "+
		"WHERE token_hash = ?", func() []byte { k := auth.SessionKey(old); return k[:] }()); err != nil {
		t.Fatal(err)
	}
	r0, _ := sessionRow(t, db, old)
	c.ua = "something else"
	if a := c.do("POST", "/api/account/password", tok, map[string]string{
		"current_password": "old-secret", "new_password": "new-secret"}); a.status != 200 {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	r, ok := sessionRow(t, db, c.cookies[auth.SessionCookie])
	if !ok || r.signedIn != r0.signedIn || r.ua != firefox || r.epoch != 1 || r.id == r0.id {
		t.Fatalf("reissued session %+v, was %+v", r, r0)
	}
	if n := sessionCount(t, db, uid); n != 1 {
		t.Fatalf("%d rows after the change, want the new one only", n)
	}
	if a := other.do("GET", "/api/session", "", nil); a.status != http.StatusUnauthorized {
		t.Fatalf("other session after the change: %d", a.status)
	}
}

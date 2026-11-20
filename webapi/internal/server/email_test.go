// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/mail"
	"tgloggerd/webapi/internal/mail/mailtest"
	"tgloggerd/webapi/internal/testdb"
)

// These tests send their mail to a mailtest server of their own, on a
// loopback port, and read what it caught; no mail leaves the machine.

const (
	testSite = "https://tgd.example.org"
	mailWait = 10 * time.Second
)

// mailLog collects the mailer's log, to check it never holds a link.
type mailLog struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *mailLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *mailLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// newMailHandler is newHandler with mail on: one worker, so messages
// arrive in the order they were sent. The mailer's log must hold no
// link when the test ends.
func newMailHandler(t *testing.T, db *sql.DB, tweak ...func(*Deps)) (http.Handler, *mailtest.Server) {
	t.Helper()
	srv := mailtest.New(t, mailtest.Plain)
	logs := &mailLog{}
	m, err := mail.New(mail.Config{
		Host: srv.Host(), Port: srv.Port(), From: "tgloggerd <noreply@example.org>",
		Security: mail.None, Workers: 1, Retries: []time.Duration{10 * time.Millisecond},
	}, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		if l := logs.String(); strings.Contains(l, "token=") {
			t.Errorf("a link in the mail log:\n%s", l)
		}
	})
	signer := auth.NewSigner(testKey)
	d := Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      db,
		App:     db,
		Signer:  signer,
		Cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		Limiter: auth.NewRateLimiter(50, time.Hour),
		Mail:    m,
		MailCfg: MailConfig{Site: testSite},
	}
	for _, f := range tweak {
		f(&d)
	}
	return New(d), srv
}

var tokenRe = regexp.MustCompile(`#token=([A-Za-z0-9_-]+)`)

// linkToken is the token of the link in the n-th message to addr
// (from 1), waiting for it.
func linkToken(t *testing.T, mails *mailtest.Server, addr string, n int) string {
	t.Helper()
	m := mails.Wait(addr, n, mailWait)[n-1]
	match := tokenRe.FindStringSubmatch(m.Text)
	if match == nil {
		t.Fatalf("no link in %q: %s", m.Subject, m.Text)
	}
	return match[1]
}

// invite creates an account with "invite": true: no password, the
// invitation by email.
func invite(t *testing.T, s session, body map[string]any) uint64 {
	t.Helper()
	body["invite"] = true
	a := s.call("POST", "/api/accounts", body)
	if a.status != 201 || a.body["invited"] != true {
		t.Fatalf("invite %v: %d %v", body["username"], a.status, a.body)
	}
	if _, ok := a.body["temporary_password"]; ok {
		t.Fatalf("invited: %v", a.body)
	}
	return uint64(a.body["id"].(float64))
}

// linkPassword sets an account's password from the n-th message its
// address received and signs it in, as its owner would.
func linkPassword(t *testing.T, h http.Handler, mails *mailtest.Server, name string, n int,
	own string) session {
	t.Helper()
	tok := linkToken(t, mails, name+"@example.org", n)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	if a := c.do("POST", "/api/password/set", "",
		map[string]string{"token": tok, "new_password": own}); a.status != 204 {
		t.Fatalf("%s: set the password: %d %v", name, a.status, a.body)
	}
	return newSession(t, h, name, own)
}

// anon is a client without a session.
func anon(t *testing.T, h http.Handler) *client {
	return &client{t: t, h: h, cookies: map[string]string{}}
}

func linkState(t *testing.T, db *sql.DB, uid uint64) (unused, used int) {
	t.Helper()
	if err := db.QueryRow("SELECT COALESCE(SUM(used_at IS NULL), 0), COALESCE(SUM(used_at IS NOT NULL), 0) "+
		"FROM web_email_links WHERE web_user_id = ?", uid).Scan(&unused, &used); err != nil {
		t.Fatal(err)
	}
	return unused, used
}

// An invitation: the link is in both parts of the message, in the
// fragment of the set-password page; opening the page (as a mail
// scanner would) uses nothing up; a short password is refused without
// using the link; the link works once; the address it went to is
// verified; and the account signs in with the password it chose. Only
// the token's hash is stored.
func TestInvitationLink(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("root-password")
	addUser(t, db, "root", hash, "superadmin", true)
	h, mails := newMailHandler(t, db)
	root := newSession(t, h, "root", "root-password")
	id := invite(t, root, account("iris", "member"))

	m := mails.Wait("iris@example.org", 1, mailWait)[0]
	tok := linkToken(t, mails, "iris@example.org", 1)
	link := testSite + "/password/set#token=" + tok
	if m.Subject != "Your account on "+testSite || !strings.Contains(m.Text, "Hello IRIS,") ||
		!strings.Contains(m.Text, link+"\n") || !strings.Contains(m.HTML, `<a href="`+link+`">`) {
		t.Fatalf("invitation %q:\n%s\n%s", m.Subject, m.Text, m.HTML)
	}
	var stored int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_email_links WHERE token_hash = UNHEX(SHA2(?, 256)) "+
		"AND email = 'iris@example.org' AND purpose = 'password'", tok).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("stored link: %d %v", stored, err)
	}

	// A scanner opens the page, and its data, as often as it likes.
	c := anon(t, h)
	for _, p := range []string{"/password/set", "/password/set?token=" + tok, "/api/page/password/set"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s: %d", p, w.Code)
		}
	}
	if a := c.do("GET", "/api/page/password/set", "", nil); a.body["route"] != "password_set" {
		t.Fatalf("page %v", a.body)
	}
	if unused, used := linkState(t, db, id); unused != 1 || used != 0 {
		t.Fatalf("after the GETs: %d unused, %d used", unused, used)
	}

	if a := c.do("POST", "/api/password/set", "", map[string]string{"token": tok, "new_password": "short"}); a.status != 400 ||
		a.body["error"] != "weak_password" {
		t.Fatalf("short password: %d %v", a.status, a.body)
	}
	for _, bad := range []map[string]string{{"token": "nope", "new_password": "long-enough"}, {"new_password": "long-enough"}} {
		if a := c.do("POST", "/api/password/set", "", bad); a.status != 400 || a.body["error"] != "bad_link" {
			t.Fatalf("bad token %v: %d %v", bad, a.status, a.body)
		}
	}
	if a := c.do("POST", "/api/password/set", "", map[string]string{"token": tok, "new_password": "iris-password"}); a.status != 204 {
		t.Fatalf("set: %d %v", a.status, a.body)
	}
	if a := c.do("POST", "/api/password/set", "", map[string]string{"token": tok, "new_password": "other-password"}); a.status != 400 ||
		a.body["error"] != "bad_link" {
		t.Fatalf("used twice: %d %v", a.status, a.body)
	}
	iris := newSession(t, h, "iris", "iris-password")
	if a := iris.call("GET", "/api/session", nil); a.status != 200 {
		t.Fatalf("session: %d %v", a.status, a.body)
	}
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.body["email_verified"] != true ||
		a.body["invited"] != false {
		t.Fatalf("row after the link: %v", a.body)
	}
	// The first password is no change to tell about.
	time.Sleep(50 * time.Millisecond)
	if n := len(mails.To("iris@example.org")); n != 1 {
		t.Fatalf("%d messages to iris", n)
	}
}

// Links expire; a link used by two requests at once works for one;
// resend-invite replaces the earlier invitation and is only for
// accounts without a password of their own.
func TestLinkLifecycle(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("root-password")
	addUser(t, db, "root", hash, "superadmin", true)
	h, mails := newMailHandler(t, db)
	root := newSession(t, h, "root", "root-password")
	id := invite(t, root, account("jan", "member"))
	path := fmt.Sprintf("/api/accounts/%d", id)
	first := linkToken(t, mails, "jan@example.org", 1)

	if a := root.call("POST", path+"/resend-invite", nil); a.status != 204 {
		t.Fatalf("resend: %d %v", a.status, a.body)
	}
	second := linkToken(t, mails, "jan@example.org", 2)
	c := anon(t, h)
	set := func(tok, pw string) answer {
		return c.do("POST", "/api/password/set", "", map[string]string{"token": tok, "new_password": pw})
	}
	if a := set(first, "jan-password"); a.status != 400 || a.body["error"] != "bad_link" {
		t.Fatalf("replaced invitation: %d %v", a.status, a.body)
	}

	// Expired.
	if _, err := db.Exec("UPDATE web_email_links SET expires_at = NOW() - INTERVAL 1 SECOND "+
		"WHERE web_user_id = ?", id); err != nil {
		t.Fatal(err)
	}
	if a := set(second, "jan-password"); a.status != 400 || a.body["error"] != "bad_link" {
		t.Fatalf("expired invitation: %d %v", a.status, a.body)
	}

	// Two at once: one wins.
	root.call("POST", path+"/resend-invite", nil)
	third := linkToken(t, mails, "jan@example.org", 3)
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cc := anon(t, h)
			codes[i] = cc.do("POST", "/api/password/set", "",
				map[string]string{"token": third, "new_password": fmt.Sprintf("jan-password-%d", i)}).status
		}(i)
	}
	wg.Wait()
	if (codes[0] == 204) == (codes[1] == 204) {
		t.Fatalf("both or neither: %v", codes)
	}
	if a := root.call("POST", path+"/resend-invite", nil); a.status != 409 || a.body["error"] != "not_invited" {
		t.Fatalf("resend to a signed-up account: %d %v", a.status, a.body)
	}
	if a := root.call("POST", "/api/accounts/999999/resend-invite", nil); a.status != 404 {
		t.Fatalf("resend to nobody: %d", a.status)
	}
}

// The forgotten-password form answers the same whether or not the
// account exists or has a verified address; only a verified address
// gets the link, which resets the password and ends the sessions.
// Clients asking too often are told; an account asked for too often
// silently gets no more links.
func TestForgotPassword(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("old-password")
	kim := addUser(t, db, "kim", hash, "member", true)
	lee := addUser(t, db, "lee", hash, "member", true)
	if _, err := db.Exec("UPDATE web_users SET email_verified_at = NULL WHERE id = ?", lee); err != nil {
		t.Fatal(err)
	}
	h, mails := newMailHandler(t, db)
	kimSession := newSession(t, h, "kim", "old-password")

	c := anon(t, h)
	login := c.do("GET", "/api/login", "", nil)
	tok := login.body["csrf"].(string)
	paths := map[string]bool{}
	for _, ck := range login.set {
		if ck.Name == auth.LoginCookie {
			paths[ck.Path] = true
		}
	}
	if !paths["/api/login"] || !paths["/api/password/forgot"] {
		t.Fatalf("login token cookie paths %v", paths)
	}
	forgot := func(cl *client, csrf, name string) answer {
		return cl.do("POST", "/api/password/forgot", csrf, map[string]string{"username": name})
	}
	if a := forgot(c, "", "kim"); a.status != 403 || a.body["error"] != "csrf" {
		t.Fatalf("without a token: %d %v", a.status, a.body)
	}
	for _, name := range []string{"nobody", "lee", "lee@example.org", "kim@example.org"} {
		if a := forgot(c, tok, name); a.status != 202 || len(a.body) != 0 {
			t.Fatalf("forgot %s: %d %v", name, a.status, a.body)
		}
	}
	// Messages go out in order: by the time kim's arrives, lee's
	// would have.
	link := linkToken(t, mails, "kim@example.org", 1)
	if n := len(mails.To("lee@example.org")); n != 0 {
		t.Fatalf("%d messages to an unverified address", n)
	}
	if m := mails.To("kim@example.org")[0]; m.Subject != "Reset your password on "+testSite ||
		!strings.Contains(m.Text, testSite+"/password/set#token=") {
		t.Fatalf("reset mail %q %s", m.Subject, m.Text)
	}
	// The fifth request from this address passes, the sixth is refused.
	if a := forgot(c, tok, "nobody"); a.status != 202 {
		t.Fatalf("fifth: %d", a.status)
	}
	if a := forgot(c, tok, "kim"); a.status != 429 || a.body["error"] != "rate_limited" {
		t.Fatalf("sixth: %d %v", a.status, a.body)
	}

	// Set from the link: the old password and sessions end; kim is
	// told.
	if a := anon(t, h).do("POST", "/api/password/set", "",
		map[string]string{"token": link, "new_password": "new-password"}); a.status != 204 {
		t.Fatalf("set: %d %v", a.status, a.body)
	}
	if a := kimSession.call("GET", "/api/session", nil); a.status != 401 {
		t.Fatalf("session after a reset: %d", a.status)
	}
	if a := anon(t, h).login("kim", "old-password"); a.status != 401 {
		t.Fatalf("old password: %d", a.status)
	}
	newSession(t, h, "kim", "new-password")
	if m := mails.Wait("kim@example.org", 2, mailWait)[1]; m.Subject != "Your password was changed" {
		t.Fatalf("notice %q", m.Subject)
	}

	// Per account: three links an hour, from any number of addresses;
	// the fourth request is answered the same and sends nothing.
	for i := range 3 {
		cl := anon(t, h)
		cl.addr = fmt.Sprintf("192.0.2.%d:4000", 10+i)
		t2 := cl.do("GET", "/api/login", "", nil).body["csrf"].(string)
		if a := forgot(cl, t2, "kim"); a.status != 202 {
			t.Fatalf("request %d: %d", i, a.status)
		}
	}
	// kim: reset 1, notice, then links 2 and 3 (the cap counts 1-3).
	mails.Wait("kim@example.org", 4, mailWait)
	time.Sleep(100 * time.Millisecond)
	if n := len(mails.To("kim@example.org")); n != 4 {
		t.Fatalf("%d messages to kim, want 4", n)
	}
	var capped int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'password_forgot_ratelimited' "+
		"AND target_user_id = ?", kim).Scan(&capped); err != nil || capped != 1 {
		t.Fatalf("capped requests audited: %d %v", capped, err)
	}
}

// No address is required (decisions/2026-11-18-email-optional.md):
// an account without one uses everything once past the password gate,
// which still comes first for a temporary password. It may add an
// address later; with mail, the address is the account's once
// verified.
func TestNoEmailGate(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("nina-password")
	nina := addUser(t, db, "nina", hash, "superadmin", true)
	if _, err := db.Exec("UPDATE web_users SET email = NULL, email_verified_at = NULL, "+
		"must_change_password = 1 WHERE id = ?", nina); err != nil {
		t.Fatal(err)
	}
	h, mails := newMailHandler(t, db)
	s := newSession(t, h, "nina", "nina-password")
	page := func(path string) map[string]any {
		t.Helper()
		a := s.call("GET", "/api/page/"+path, nil)
		if a.status != 200 {
			t.Fatalf("page %s: %d %v", path, a.status, a.body)
		}
		return a.body
	}

	// The password gate, for the API and the pages; the link pages
	// pass it.
	if a := s.call("GET", "/api/accounts", nil); a.status != 403 || a.body["error"] != "password_change_required" {
		t.Fatalf("accounts behind the password gate: %d %v", a.status, a.body)
	}
	if p := page("users?q=x"); p["route"] != "password_change" ||
		p["props"].(map[string]any)["next"] != "/users?q=x" ||
		p["props"].(map[string]any)["username"] != "nina" {
		t.Fatalf("page behind the password gate: %v", p)
	}
	if p := page("password/set"); p["route"] != "password_set" {
		t.Fatalf("link page behind the password gate: %v", p)
	}
	if a := s.call("POST", "/api/account/email", map[string]string{"email": "nina@example.org",
		"password": "nina-password"}); a.status != 403 || a.body["error"] != "password_change_required" {
		t.Fatalf("email behind the password gate: %d %v", a.status, a.body)
	}
	a := s.call("POST", "/api/account/password", map[string]string{
		"current_password": "nina-password", "new_password": "nina-own-password"})
	if a.status != 200 {
		t.Fatalf("password change: %d %v", a.status, a.body)
	}
	s.csrf = a.body["csrf"].(string)

	// No address, and nothing in the way.
	if a := s.call("GET", "/api/accounts", nil); a.status != 200 {
		t.Fatalf("accounts without an address: %d %v", a.status, a.body)
	}
	if p := page("settings"); p["route"] != "settings" {
		t.Fatalf("settings without an address: %v", p["route"])
	}

	for _, bad := range []struct {
		body map[string]string
		code string
	}{
		{map[string]string{"email": "nina@example.org", "password": "wrong"}, "wrong_password"},
		{map[string]string{"email": "not an address", "password": "nina-own-password"}, "bad_email"},
		{map[string]string{"email": "nina@example.org"}, "bad_request"},
	} {
		if a := s.call("POST", "/api/account/email", bad.body); a.status != 400 || a.body["error"] != bad.code {
			t.Fatalf("email %v: %d %v", bad.body, a.status, a.body)
		}
	}
	if a := s.call("POST", "/api/account/email", map[string]string{"email": "nina@example.org",
		"password": "nina-own-password"}); a.status != 200 || a.body["verification_sent"] != true {
		t.Fatalf("add the address: %d %v", a.status, a.body)
	}
	// Until verified, the account has no address.
	if acct := page("settings")["props"].(map[string]any)["account"].(map[string]any); acct["email"] != nil {
		t.Fatalf("settings before verifying: %v", acct)
	}
	tok := linkToken(t, mails, "nina@example.org", 1)
	if m := mails.To("nina@example.org")[0]; m.Subject != "Verify your email address for "+testSite ||
		!strings.Contains(m.Text, testSite+"/account/email/verify#token="+tok) {
		t.Fatalf("verification mail %q %s", m.Subject, m.Text)
	}
	// Verified in another browser, without a session.
	if a := anon(t, h).do("POST", "/api/account/email/verify", "", map[string]string{"token": tok}); a.status != 204 {
		t.Fatalf("verify: %d %v", a.status, a.body)
	}
	if a := anon(t, h).do("POST", "/api/account/email/verify", "", map[string]string{"token": tok}); a.status != 400 ||
		a.body["error"] != "bad_link" {
		t.Fatalf("verify twice: %d %v", a.status, a.body)
	}
	if acct := page("settings")["props"].(map[string]any)["account"].(map[string]any); acct["email"] != "nina@example.org" ||
		acct["email_verified"] != true {
		t.Fatalf("settings after verifying: %v", acct)
	}
	// A first address replaces none: no change notices.
	time.Sleep(50 * time.Millisecond)
	if n := len(mails.To("nina@example.org")); n != 1 {
		t.Fatalf("%d messages after the first address", n)
	}
}

// Changing an address: the account's own change waits for the link,
// then both addresses are told; another account's address is refused.
// A superadmin may change a signed-in account's address (old one told,
// new one sent a verification link); an admin only an invited one's,
// which sends the invitation to the new address instead.
func TestAddressChange(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("pw-12345678")
	addUser(t, db, "root", hash, "superadmin", true)
	addUser(t, db, "adm", hash, "admin", true)
	ola := addUser(t, db, "ola", hash, "member", true)
	addUser(t, db, "pat", hash, "member", true)
	// adm administers Default, where ola is a member.
	if _, err := db.Exec("INSERT INTO web_federation_admins (federation_id, web_user_id) " +
		"SELECT 0, id FROM web_users WHERE username = 'adm'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO web_federation_members (federation_id, web_user_id) "+
		"VALUES (0, ?)", ola); err != nil {
		t.Fatal(err)
	}
	h, mails := newMailHandler(t, db)
	s := newSession(t, h, "ola", "pw-12345678")

	if a := s.call("POST", "/api/account/email", map[string]string{"email": "pat@example.org",
		"password": "pw-12345678"}); a.status != 409 || a.body["error"] != "email_taken" {
		t.Fatalf("another account's address: %d %v", a.status, a.body)
	}
	if a := s.call("POST", "/api/account/email", map[string]string{"email": "ola@example.org",
		"password": "pw-12345678"}); a.status != 400 || a.body["error"] != "bad_email" {
		t.Fatalf("the same address: %d %v", a.status, a.body)
	}
	if a := s.call("POST", "/api/account/email", map[string]string{"email": "ola@new.example.org",
		"password": "pw-12345678"}); a.status != 200 || a.body["verification_sent"] != true {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	// Until verified, the old address stays the account's.
	if p := s.call("GET", "/api/page/settings", nil).body["props"].(map[string]any)["account"].(map[string]any); p["email"] != "ola@example.org" || p["email_verified"] != true {
		t.Fatalf("settings before verifying: %v", p)
	}
	tok := linkToken(t, mails, "ola@new.example.org", 1)
	if a := anon(t, h).do("POST", "/api/account/email/verify", "", map[string]string{"token": tok}); a.status != 204 {
		t.Fatalf("verify: %d %v", a.status, a.body)
	}
	old := mails.Wait("ola@example.org", 1, mailWait)[0]
	if old.Subject != "Your email address was changed" || !strings.Contains(old.Text, "to ola@new.example.org") {
		t.Fatalf("notice to the old address %q %s", old.Subject, old.Text)
	}
	if n := mails.Wait("ola@new.example.org", 2, mailWait)[1]; n.Subject != "Your email address was changed" {
		t.Fatalf("notice to the new address %q", n.Subject)
	}

	root := newSession(t, h, "root", "pw-12345678")
	adm := newSession(t, h, "adm", "pw-12345678")
	path := fmt.Sprintf("/api/accounts/%d", ola)
	if a := adm.call("PATCH", path, map[string]any{"email": "ola@admin.example.org"}); a.status != 403 {
		t.Fatalf("admin changes a signed-up member's address: %d %v", a.status, a.body)
	}
	a := root.call("PATCH", path, map[string]any{"email": "ola@lost.example.org"})
	if a.status != 200 || a.body["email"] != "ola@lost.example.org" || a.body["email_verified"] != false {
		t.Fatalf("superadmin changes the address: %d %v", a.status, a.body)
	}
	if m := mails.Wait("ola@new.example.org", 3, mailWait)[2]; !strings.Contains(m.Text, "to ola@lost.example.org") {
		t.Fatalf("old address told: %s", m.Text)
	}
	if m := mails.Wait("ola@lost.example.org", 1, mailWait)[0]; !strings.Contains(m.Text, "/account/email/verify#token=") {
		t.Fatalf("verification to the new address: %s", m.Text)
	}

	// An invited member: the admin corrects the address, and the
	// invitation goes to the new one; the first stops working.
	id := invite(t, adm, account("quinn", "member", defaultGrant))
	first := linkToken(t, mails, "quinn@example.org", 1)
	if a := adm.call("PATCH", fmt.Sprintf("/api/accounts/%d", id),
		map[string]any{"email": "quinn@right.example.org"}); a.status != 200 {
		t.Fatalf("admin corrects an invited address: %d %v", a.status, a.body)
	}
	second := linkToken(t, mails, "quinn@right.example.org", 1)
	c := anon(t, h)
	if a := c.do("POST", "/api/password/set", "", map[string]string{"token": first,
		"new_password": "quinn-password"}); a.status != 400 {
		t.Fatalf("invitation to the wrong address: %d", a.status)
	}
	if a := c.do("POST", "/api/password/set", "", map[string]string{"token": second,
		"new_password": "quinn-password"}); a.status != 204 {
		t.Fatalf("invitation to the right address: %d %v", a.status, a.body)
	}
	if a := adm.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.body["email_verified"] != true {
		t.Fatalf("invited address verified by the link: %v", a.body)
	}
	// A reset mails a link to a verified address, and gives a
	// temporary password to an account without one.
	if a := adm.call("POST", fmt.Sprintf("/api/accounts/%d/password-reset", id), nil); a.status != 200 ||
		a.body["link_sent"] != true || a.body["temporary_password"] != nil {
		t.Fatalf("reset with a verified address: %d %v", a.status, a.body)
	}
	if m := mails.Wait("quinn@right.example.org", 2, mailWait)[1]; m.Subject != "Choose a new password for "+testSite {
		t.Fatalf("reset mail %q", m.Subject)
	}
	if _, err := db.Exec("UPDATE web_users SET email_verified_at = NULL WHERE id = ?", id); err != nil {
		t.Fatal(err)
	}
	a = adm.call("POST", fmt.Sprintf("/api/accounts/%d/password-reset", id), nil)
	temp, _ := a.body["temporary_password"].(string)
	if a.status != 200 || a.body["link_sent"] != false || len(temp) != 20 {
		t.Fatalf("reset to an unverified address: %d %v", a.status, a.body)
	}
	q := newSession(t, h, "quinn", temp)
	if a := q.call("GET", "/api/accounts", nil); a.status != 403 || a.body["error"] != "password_change_required" {
		t.Fatalf("after a temporary password: %d %v", a.status, a.body)
	}
}

// The security notices: a password change, a sign-in from a new
// device (not the first device, nor one seen before), a disable and a
// role change, each to the verified address, and none to an
// unverified one.
func TestSecurityNotices(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("pw-12345678")
	addUser(t, db, "root", hash, "superadmin", true)
	rae := addUser(t, db, "rae", hash, "member", true)
	sam := addUser(t, db, "sam", hash, "member", true)
	if _, err := db.Exec("UPDATE web_users SET email_verified_at = NULL WHERE id = ?", sam); err != nil {
		t.Fatal(err)
	}
	h, mails := newMailHandler(t, db)
	const firefox = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	const chrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"
	signIn := func(name, ua string) session {
		t.Helper()
		c := &client{t: t, h: h, cookies: map[string]string{}, ua: ua, addr: "198.51.100.9:5000"}
		return session{c, signIn(t, c, name, "pw-12345678")}
	}
	s := signIn("rae", firefox)
	signIn("rae", firefox)
	signIn("rae", chrome)
	m := mails.Wait("rae@example.org", 1, mailWait)[0]
	if m.Subject != "Sign-in from a new device" || !strings.Contains(m.Text, "Chrome on Windows, from the address 198.51.100.9") {
		t.Fatalf("new device notice %q %s", m.Subject, m.Text)
	}
	signIn("rae", chrome)

	if a := s.call("POST", "/api/account/password", map[string]string{"current_password": "pw-12345678",
		"new_password": "pw-87654321"}); a.status != 200 {
		t.Fatalf("change: %d %v", a.status, a.body)
	}
	root := newSession(t, h, "root", "pw-12345678")
	if a := root.call("PATCH", fmt.Sprintf("/api/accounts/%d", rae), map[string]any{"role": "admin"}); a.status != 200 {
		t.Fatalf("role: %d %v", a.status, a.body)
	}
	if a := root.call("POST", fmt.Sprintf("/api/accounts/%d/disable", rae), nil); a.status != 204 {
		t.Fatalf("disable: %d %v", a.status, a.body)
	}
	// Disabling again changes nothing and tells nothing.
	root.call("POST", fmt.Sprintf("/api/accounts/%d/disable", rae), nil)
	got := mails.Wait("rae@example.org", 4, mailWait)
	var subjects []string
	for _, m := range got {
		subjects = append(subjects, m.Subject)
	}
	if strings.Join(subjects, "|") != "Sign-in from a new device|Your password was changed|"+
		"Your role was changed|Your account was disabled" ||
		!strings.Contains(got[2].Text, "from member to admin") {
		t.Fatalf("notices %v\n%s", subjects, got[2].Text)
	}

	// sam has no verified address: nothing is sent to it.
	signIn("sam", firefox)
	signIn("sam", chrome)
	root.call("POST", fmt.Sprintf("/api/accounts/%d/disable", sam), nil)
	// Enabling tells nothing; disabling again is rae's fifth message,
	// sent after everything above.
	root.call("POST", fmt.Sprintf("/api/accounts/%d/enable", rae), nil)
	root.call("POST", fmt.Sprintf("/api/accounts/%d/disable", rae), nil)
	mails.Wait("rae@example.org", 5, mailWait)
	if n := len(mails.To("sam@example.org")); n != 0 {
		t.Fatalf("%d messages to an unverified address", n)
	}
	if n := len(mails.To("rae@example.org")); n != 5 {
		t.Fatalf("%d messages to rae, want 5", n)
	}
}

// Without mail nothing is sent and nothing waits for it: accounts are
// created with a typed or a generated password, resets give a
// temporary password, an address is recorded unverified, and the
// forgotten-password form answers as ever. Only what is nothing but a
// link (an invitation) is refused, 409 mail_off.
func TestMailOff(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("pw-12345678")
	id := addUser(t, db, "root", hash, "superadmin", true)
	h := newHandler(db, 50)
	root := newSession(t, h, "root", "pw-12345678")

	inv := account("tia", "member")
	inv["invite"] = true
	if a := root.call("POST", "/api/accounts", inv); a.status != 409 || a.body["error"] != "mail_off" {
		t.Fatalf("invite without mail: %d %v", a.status, a.body)
	}
	tia, temp := create(t, root, account("tia", "member"))
	if a := root.call("POST", fmt.Sprintf("/api/accounts/%d/resend-invite", tia), nil); a.status != 409 ||
		a.body["error"] != "mail_off" {
		t.Fatalf("resend without mail: %d %v", a.status, a.body)
	}
	firstPassword(t, h, "tia", temp, "tia-own-password")

	// The verified address is no use without mail: a temporary password.
	a := root.call("POST", fmt.Sprintf("/api/accounts/%d/password-reset", id), nil)
	reset, _ := a.body["temporary_password"].(string)
	if a.status != 200 || a.body["link_sent"] != false || len(reset) != 20 {
		t.Fatalf("reset without mail: %d %v", a.status, a.body)
	}
	root = firstPassword(t, h, "root", reset, "root-own-password")

	if a := root.call("POST", "/api/account/email", map[string]string{"email": "root@new.example.org",
		"password": "root-own-password"}); a.status != 200 || a.body["verification_sent"] != false {
		t.Fatalf("address without mail: %d %v", a.status, a.body)
	}
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.body["email"] != "root@new.example.org" ||
		a.body["email_verified"] != false {
		t.Fatalf("address recorded unverified: %v", a.body)
	}
	if a := root.call("POST", "/api/account/email", map[string]string{"email": "",
		"password": "root-own-password"}); a.status != 200 {
		t.Fatalf("remove the address: %d %v", a.status, a.body)
	}
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%d", id), nil); a.body["email"] != nil {
		t.Fatalf("address removed: %v", a.body)
	}

	c := anon(t, h)
	tok := c.do("GET", "/api/login", "", nil).body["csrf"].(string)
	if a := c.do("POST", "/api/password/forgot", tok, map[string]string{"username": "tia"}); a.status != 202 {
		t.Fatalf("forgot: %d %v", a.status, a.body)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_email_links").Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d links without mail", n)
	}
}

// Account creation: a typed first password or a generated temporary
// one, changed at the first sign-in either way; or, explicitly, an
// invitation, which needs an address (and mail). The address is
// optional.
func TestAccountCreation(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	hash, _ := auth.HashPassword("pw-12345678")
	addUser(t, db, "root", hash, "superadmin", true)
	h, mails := newMailHandler(t, db)
	root := newSession(t, h, "root", "pw-12345678")

	typed := map[string]any{"username": "uma", "name": "Uma", "role": "member", "password": "uma-first-pw"}
	a := root.call("POST", "/api/accounts", typed)
	if a.status != 201 || a.body["invited"] != false || a.body["temporary_password"] != nil {
		t.Fatalf("typed password: %d %v", a.status, a.body)
	}
	uma := newSession(t, h, "uma", "uma-first-pw")
	if a := uma.call("GET", "/api/session", nil); a.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("typed first password must be changed: %v", a.body)
	}
	if a := root.call("GET", fmt.Sprintf("/api/accounts/%v", a.body["id"]), nil); a.body["email"] != nil {
		t.Fatalf("no address: %v", a.body)
	}

	for _, bad := range []struct {
		body   map[string]any
		status int
		code   string
	}{
		{map[string]any{"username": "vic", "name": "", "role": "member", "password": "short"}, 400, "weak_password"},
		{map[string]any{"username": "vic", "name": "", "role": "member", "invite": true}, 409, "no_email"},
		{map[string]any{"username": "vic", "name": "", "role": "member", "invite": true,
			"email": "vic@example.org", "password": "vic-password"}, 400, "bad_request"},
	} {
		if a := root.call("POST", "/api/accounts", bad.body); a.status != bad.status || a.body["error"] != bad.code {
			t.Fatalf("%v: %d %v", bad.body, a.status, a.body)
		}
	}
	// A generated one even with an address and mail, unless an
	// invitation is asked for: nothing is mailed.
	_, temp := create(t, root, account("wes", "member"))
	firstPassword(t, h, "wes", temp, "wes-own-password")
	invite(t, root, account("xia", "member"))
	mails.Wait("xia@example.org", 1, mailWait)
	if n := len(mails.To("wes@example.org")); n != 0 {
		t.Fatalf("%d messages to an account with a generated password", n)
	}
}

// Every message has its subject and body in English, with every value
// filled in and nothing left of a {name}; other languages fall back to
// these.
func TestMailTexts(t *testing.T) {
	t.Parallel()
	vals := []string{"name", "N", "username", "U", "site", testSite, "time", "T", "link", "L",
		"expires", "E", "email", "M", "device", "D", "ip", "I", "old", "O", "new", "W"}
	for _, kind := range mailKinds {
		for _, part := range []string{"subject", "body"} {
			key := "mail." + kind + "." + part
			text := i18n.T(i18n.Fallback, key, vals...)
			if text == key || text == "" || strings.ContainsAny(text, "{}") {
				t.Errorf("%s: %q", key, text)
			}
		}
	}
	if f := i18n.T(i18n.Fallback, "mail.footer", vals...); strings.ContainsAny(f, "{}") || f == "mail.footer" {
		t.Errorf("footer %q", f)
	}
	h := mailHTML("A <b>\n\nGo to https://x/#token=a&b now", "https://x/#token=a&b")
	if !strings.Contains(h, "<p>A &lt;b&gt;</p>") ||
		!strings.Contains(h, `<a href="https://x/#token=a&amp;b">https://x/#token=a&amp;b</a>`) {
		t.Errorf("html %s", h)
	}
}

// The accounts page says whether mail is configured, so it offers
// set-password links only when they can be sent.
func TestAccountsPageMailFlag(t *testing.T) {
	t.Parallel()
	for _, mail := range []bool{false, true} {
		db := freshAppDB(t)
		hash, _ := auth.HashPassword("pw-12345678")
		addUser(t, db, "root", hash, "superadmin", true)
		// The page counts groups, so it needs the logger's schema.
		lg := testdb.New(t, testdb.Logger, "sample")
		h, _ := newMailHandler(t, db, func(d *Deps) {
			d.RO = lg.DB
			if !mail {
				d.Mail = nil
			}
		})
		s := newSession(t, h, "root", "pw-12345678")
		a := s.call("GET", "/api/page/settings/accounts", nil)
		if a.status != 200 || a.body["route"] != "accounts" {
			t.Fatalf("mail %v: page %d %v", mail, a.status, a.body)
		}
		if got := a.body["props"].(map[string]any)["mail"]; got != mail {
			t.Errorf("mail %v: props.mail = %v", mail, got)
		}
	}
}

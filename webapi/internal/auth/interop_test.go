// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testdata/drogon.json holds values the Drogon web's own code made:
// a program linked against web/src/auth and web/src/daemon/Api.cpp,
// run with the test key in "app_key", wrote it. These tests check that
// the Go code reads, makes and refuses the same values.
//
// With WEBAPI_GO_MINTED set to a path, TestGoMinted also writes values
// the Go code made there, for the same program to check in the other
// direction.

type sessionFix struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
	Epoch    uint32 `json:"epoch"`
}

type drogonFixtures struct {
	AppKey       string `json:"app_key"`
	KeySpellings []struct {
		Value         string `json:"value"`
		TokenInit     bool   `json:"token_init"`
		FiletokenInit bool   `json:"filetoken_init"`
		TagABC        string `json:"tag_abc"`
		FileToken1    string `json:"file_token_1"`
	} `json:"key_spellings"`
	Tokens []struct {
		PayloadHex string `json:"payload_hex"`
		Token      string `json:"token"`
	} `json:"tokens"`
	Tags []struct {
		MsgHex string `json:"msg_hex"`
		Tag    string `json:"tag"`
	} `json:"tags"`
	Open []struct {
		Token      string  `json:"token"`
		PayloadHex *string `json:"payload_hex"`
	} `json:"open"`
	Sessions []struct {
		sessionFix
		IssuedBetween [2]int64    `json:"issued_between"`
		Cookie        string      `json:"cookie"`
		SetCookie     string      `json:"set_cookie"`
		Current       *sessionFix `json:"current"`
		CSRF          string      `json:"csrf"`
	} `json:"sessions"`
	SessionClear   string `json:"session_clear"`
	SessionCurrent []struct {
		PayloadHex string      `json:"payload_hex"`
		Cookie     string      `json:"cookie"`
		Current    *sessionFix `json:"current"`
	} `json:"session_current"`
	LoginTokens []struct {
		Token string `json:"token"`
		Check bool   `json:"check"`
	} `json:"login_tokens"`
	LoginCookie string `json:"login_cookie"`
	FileTokens  []struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	} `json:"file_tokens"`
	FileTokenDecrypt []struct {
		Token string  `json:"token"`
		ID    *string `json:"id"`
	} `json:"file_token_decrypt"`
	Passwords []struct {
		PasswordHex string `json:"password_hex"`
		Hash        string `json:"hash"`
		Verify      bool   `json:"verify"`
		VerifyWrong bool   `json:"verify_wrong"`
	} `json:"passwords"`
}

func loadFixtures(t *testing.T) (*drogonFixtures, []byte) {
	b, err := os.ReadFile("testdata/drogon.json")
	if err != nil {
		t.Fatal(err)
	}
	var f drogonFixtures
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	key, err := DecodeAppKey(f.AppKey)
	if err != nil {
		t.Fatal(err)
	}
	return &f, key
}

func unhex(t *testing.T, s string) string {
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func asSession(t *testing.T, f sessionFix) Session {
	uid, err := strconv.ParseUint(f.UID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return Session{UID: uid, Username: f.Username, Role: f.Role, Exp: f.Exp, Epoch: f.Epoch}
}

// cookieAttrs is a Set-Cookie line's attributes, in lower case and in
// any order, without the name and value.
func cookieAttrs(line string) map[string]string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "Set-Cookie:"))
	parts := strings.Split(line, ";")
	out := map[string]string{}
	for _, p := range parts[1:] {
		k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
		out[strings.ToLower(k)] = strings.ToLower(v)
	}
	return out
}

func TestDrogonKeySpellings(t *testing.T) {
	f, _ := loadFixtures(t)
	for _, k := range f.KeySpellings {
		key, err := DecodeAppKey(k.Value)
		if (err == nil) != k.TokenInit || k.TokenInit != k.FiletokenInit {
			t.Errorf("DecodeAppKey(%q) err = %v; Drogon accepts: %v", k.Value, err, k.TokenInit)
			continue
		}
		if err != nil {
			continue
		}
		if got := NewSigner(key).Tag("abc"); got != k.TagABC {
			t.Errorf("key %q: tag %q, Drogon %q", k.Value, got, k.TagABC)
		}
		if got := NewFileTokens(key).Encrypt(1); got != k.FileToken1 {
			t.Errorf("key %q: file token %q, Drogon %q", k.Value, got, k.FileToken1)
		}
	}
}

func TestDrogonTokens(t *testing.T) {
	f, key := loadFixtures(t)
	s := NewSigner(key)
	for _, tk := range f.Tokens {
		p := unhex(t, tk.PayloadHex)
		if got := s.Make(p); got != tk.Token {
			t.Errorf("Make(%q) = %q, Drogon %q", p, got, tk.Token)
		}
		if got, ok := s.Open(tk.Token); !ok || got != p {
			t.Errorf("Open(%q) = %q, %v", tk.Token, got, ok)
		}
	}
	for _, tg := range f.Tags {
		if got := s.Tag(unhex(t, tg.MsgHex)); got != tg.Tag {
			t.Errorf("Tag(%s) = %q, Drogon %q", tg.MsgHex, got, tg.Tag)
		}
	}
	for _, o := range f.Open {
		got, ok := s.Open(o.Token)
		if ok != (o.PayloadHex != nil) || (ok && hex.EncodeToString([]byte(got)) != *o.PayloadHex) {
			t.Errorf("Open(%q) = %q, %v; Drogon %v", o.Token, got, ok, o.PayloadHex)
		}
	}
}

func TestDrogonSessions(t *testing.T) {
	f, key := loadFixtures(t)
	signer := NewSigner(key)
	for _, fs := range f.Sessions {
		if fs.Current == nil {
			t.Fatalf("Drogon did not open its own cookie %q", fs.Cookie)
		}
		want := asSession(t, *fs.Current)
		issued := time.Unix(fs.IssuedBetween[0], 0)
		c := &Cookies{Signer: signer, Timeout: 43200, Secure: true,
			Now: func() time.Time { return issued }}

		got, ok := c.OpenSession(fs.Cookie)
		if !ok || got != want {
			t.Errorf("OpenSession(%q) = %+v, %v; want %+v", fs.Cookie, got, ok, want)
		}
		// The same session encodes to the very same cookie.
		if enc := signer.EncodeSession(want); enc != fs.Cookie {
			t.Errorf("EncodeSession(%+v) = %q, Drogon %q", want, enc, fs.Cookie)
		}
		if got := signer.SessionCSRF(fs.Cookie); got != fs.CSRF {
			t.Errorf("SessionCSRF = %q, Drogon %q", got, fs.CSRF)
		}

		w := httptest.NewRecorder()
		c.Issue(w, want)
		gotAttrs := cookieAttrs(w.Header().Get("Set-Cookie"))
		if wantAttrs := cookieAttrs(fs.SetCookie); !sameAttrs(gotAttrs, wantAttrs) {
			t.Errorf("Set-Cookie attributes %v, Drogon %v", gotAttrs, wantAttrs)
		}
		if !strings.HasPrefix(w.Header().Get("Set-Cookie"), "tgw_session="+fs.Cookie+";") {
			t.Errorf("Set-Cookie %q", w.Header().Get("Set-Cookie"))
		}

		expired := &Cookies{Signer: signer, Now: func() time.Time { return time.Unix(want.Exp, 0) }}
		if _, ok := expired.OpenSession(fs.Cookie); ok {
			t.Errorf("cookie %q opens at its expiry", fs.Cookie)
		}
	}

	c := &Cookies{Signer: signer, Secure: true}
	w := httptest.NewRecorder()
	c.Clear(w)
	if got, want := cookieAttrs(w.Header().Get("Set-Cookie")), cookieAttrs(f.SessionClear); !sameAttrs(got, want) {
		t.Errorf("clear attributes %v, Drogon %v", got, want)
	}

	for _, sc := range f.SessionCurrent {
		got, ok := c.OpenSession(sc.Cookie)
		if ok != (sc.Current != nil) {
			t.Errorf("payload %q: opens %v, Drogon %v", unhex(t, sc.PayloadHex), ok, sc.Current != nil)
			continue
		}
		if ok && got != asSession(t, *sc.Current) {
			t.Errorf("payload %q: %+v, Drogon %+v", unhex(t, sc.PayloadHex), got, *sc.Current)
		}
	}
}

func sameAttrs(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func TestDrogonLoginTokens(t *testing.T) {
	f, key := loadFixtures(t)
	s := NewSigner(key)
	for _, lt := range f.LoginTokens {
		r := httptest.NewRequest("POST", "/", nil)
		r.AddCookie(&http.Cookie{Name: LoginCookie, Value: lt.Token})
		if s.CheckLogin(r, lt.Token) != lt.Check || !lt.Check {
			t.Errorf("CheckLogin(%q) disagrees with Drogon (%v)", lt.Token, lt.Check)
		}
	}

	// The Go login cookie differs from Drogon's in its path only.
	w := httptest.NewRecorder()
	(&Cookies{Signer: s, Secure: true}).SetLoginCookie(w, "tok", "/login")
	if got, want := cookieAttrs(w.Header().Get("Set-Cookie")), cookieAttrs(f.LoginCookie); !sameAttrs(got, want) {
		t.Errorf("login cookie attributes %v, Drogon %v", got, want)
	}
}

func TestDrogonFileTokens(t *testing.T) {
	f, key := loadFixtures(t)
	ft := NewFileTokens(key)
	for _, x := range f.FileTokens {
		id, _ := strconv.ParseUint(x.ID, 10, 64)
		if got := ft.Encrypt(id); got != x.Token {
			t.Errorf("Encrypt(%d) = %q, Drogon %q", id, got, x.Token)
		}
		if got, ok := ft.Decrypt(x.Token); !ok || got != id {
			t.Errorf("Decrypt(%q) = %d, %v; want %d", x.Token, got, ok, id)
		}
	}
	for _, x := range f.FileTokenDecrypt {
		got, ok := ft.Decrypt(x.Token)
		if ok != (x.ID != nil) || (ok && strconv.FormatUint(got, 10) != *x.ID) {
			t.Errorf("Decrypt(%q) = %d, %v; Drogon %v", x.Token, got, ok, x.ID)
		}
	}
}

func TestDrogonPasswords(t *testing.T) {
	f, _ := loadFixtures(t)
	for _, p := range f.Passwords {
		pw := unhex(t, p.PasswordHex)
		if VerifyPassword(p.Hash, pw) != p.Verify || !p.Verify {
			t.Errorf("VerifyPassword(%q) disagrees with libsodium", p.Hash)
		}
		if VerifyPassword(p.Hash, pw+"x") != p.VerifyWrong || p.VerifyWrong {
			t.Errorf("VerifyPassword(%q, wrong) disagrees with libsodium", p.Hash)
		}
	}
}

// TestGoMinted writes values the Go code made for the Drogon code to
// check: tokens, sessions with their CSRF tokens, login tokens, file
// tokens and password hashes.
func TestGoMinted(t *testing.T) {
	out := os.Getenv("WEBAPI_GO_MINTED")
	if out == "" {
		t.Skip("WEBAPI_GO_MINTED is not set")
	}
	f, key := loadFixtures(t)
	s := NewSigner(key)
	c := &Cookies{Signer: s, Timeout: 43200}
	ft := NewFileTokens(key)

	type tok struct {
		PayloadHex string `json:"payload_hex"`
		Token      string `json:"token"`
	}
	type sess struct {
		sessionFix
		Cookie string `json:"cookie"`
		CSRF   string `json:"csrf"`
	}
	type file struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	type pass struct {
		PasswordHex string `json:"password_hex"`
		Hash        string `json:"hash"`
	}
	m := struct {
		AppKey      string              `json:"app_key"`
		Tokens      []tok               `json:"tokens"`
		Sessions    []sess              `json:"sessions"`
		LoginTokens []map[string]string `json:"login_tokens"`
		FileTokens  []file              `json:"file_tokens"`
		Passwords   []pass              `json:"passwords"`
	}{AppKey: f.AppKey}

	for _, p := range []string{"", "go", "\x00\xff binary\n", strings.Repeat("y", 300)} {
		m.Tokens = append(m.Tokens, tok{hex.EncodeToString([]byte(p)), s.Make(p)})
	}
	for _, x := range []Session{
		c.NewSession(1, "admin", "admin", 0),
		c.NewSession(1<<64-1, "we\nird \"go\" ;=", "viewer", 1<<32-1),
		{UID: 3, Username: "never", Role: "viewer", Exp: 0, Epoch: 9},
	} {
		raw := s.EncodeSession(x)
		m.Sessions = append(m.Sessions, sess{
			sessionFix{strconv.FormatUint(x.UID, 10), x.Username, x.Role, x.Exp, x.Epoch},
			raw, s.SessionCSRF(raw)})
	}
	for i := 0; i < 3; i++ {
		m.LoginTokens = append(m.LoginTokens, map[string]string{"token": s.NewLoginToken()})
	}
	for _, id := range []uint64{0, 1, 7, 99999, 1 << 40, 1<<64 - 1} {
		m.FileTokens = append(m.FileTokens, file{strconv.FormatUint(id, 10), ft.Encrypt(id)})
	}
	for _, p := range []string{"hunter2", "", "päss wörd ✓", strings.Repeat("z", 300)} {
		h, err := HashPassword(p)
		if err != nil {
			t.Fatal(err)
		}
		m.Passwords = append(m.Passwords, pass{hex.EncodeToString([]byte(p)), h})
	}
	b, _ := json.MarshalIndent(m, "", "\t")
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

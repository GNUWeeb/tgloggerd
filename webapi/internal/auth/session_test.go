// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestDecodeAppKey(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"MDEyMzQ1Njc4OWFiY2RlZg==", true},
		{"MDEyMzQ1Njc4OWFiY2RlZg", true},
		{"MDEyMzQ1Njc4OWFiY2Rl", false},     // 15 bytes
		{"MDEyMzQ1Njc4OWFiY2RlZh==", false}, // trailing bits set
		{"MDEyMzQ1Njc4OWFiY2RlZg=", false},
		{"MDEyMzQ1Njc4OWFiY2RlZg==\n", false},
		{"", false},
		{strings.Repeat("A", 88), false}, // 66 bytes
	} {
		_, err := DecodeAppKey(c.in)
		if (err == nil) != c.ok {
			t.Errorf("DecodeAppKey(%q) err = %v, want ok=%v", c.in, err, c.ok)
		}
	}
}

func TestTokenRoundTrip(t *testing.T) {
	s := NewSigner(testKey)
	for _, p := range []string{"", "a", "1\n2\n3", "\x00\xff"} {
		tok := s.Make(p)
		got, ok := s.Open(tok)
		if !ok || got != p {
			t.Errorf("Open(Make(%q)) = %q, %v", p, got, ok)
		}
		for _, bad := range []string{tok + "x", "x" + tok, tok + ".x",
			strings.Replace(tok, ".", "", 1)} {
			if _, ok := s.Open(bad); ok {
				t.Errorf("Open(%q) opened", bad)
			}
		}
	}
	if _, ok := NewSigner([]byte("another key of 16+ bytes")).Open(s.Make("x")); ok {
		t.Error("a token opened under another key")
	}
}

func TestParseSessionLikeStoull(t *testing.T) {
	for _, c := range []struct {
		p    string
		ok   bool
		want Session
	}{
		{"5\n0\n3\nadmin\nbob", true, Session{5, "bob", "admin", 0, 3, false}},
		{"5\n0\n3\nadmin\nx\ny", true, Session{5, "x\ny", "admin", 0, 3, false}},
		{"-1\n0\n3\nadmin\nbob", true, Session{1<<64 - 1, "bob", "admin", 0, 3, false}},
		{" 5x\n +0\n+3y\nadmin\nbob", true, Session{5, "bob", "admin", 0, 3, false}},
		{"5\n0\n4294967296\nadmin\nbob", true, Session{5, "bob", "admin", 0, 0, false}},
		{"5\n0\n-1\nadmin\nbob", true, Session{5, "bob", "admin", 0, 1<<32 - 1, false}},
		{"18446744073709551616\n0\n3\nadmin\nbob", false, Session{}},
		{"5\n9223372036854775808\n3\nadmin\nbob", false, Session{}},
		{"\n0\n3\nadmin\nbob", false, Session{}},
		{"abc\n0\n3\nadmin\nbob", false, Session{}},
		{"5\n0\nadmin\nbob", false, Session{}},
	} {
		got, ok := parseSession(c.p)
		if ok != c.ok || got != c.want {
			t.Errorf("parseSession(%q) = %+v, %v; want %+v, %v",
				c.p, got, ok, c.want, c.ok)
		}
	}
}

func TestIssueAndCurrent(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	c := &Cookies{Signer: NewSigner(testKey), Timeout: 43200, Secure: true,
		Now: func() time.Time { return now }}

	w := httptest.NewRecorder()
	v := c.Issue(w, c.NewSession(7, "bob", "viewer", 2))
	sc := w.Header().Get("Set-Cookie")
	for _, want := range []string{"tgw_session=" + v, "Path=/", "Max-Age=43200",
		"HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(sc, want) {
			t.Errorf("Set-Cookie %q lacks %q", sc, want)
		}
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: v})
	s, ok := c.Current(r)
	want := Session{7, "bob", "viewer", now.Unix() + 43200, 2, false}
	if !ok || s != want {
		t.Fatalf("Current = %+v, %v; want %+v", s, ok, want)
	}

	now = now.Add(43200 * time.Second)
	if _, ok := c.Current(r); ok {
		t.Error("an expired session is current")
	}

	w = httptest.NewRecorder()
	c.Clear(w)
	sc = w.Header().Get("Set-Cookie")
	if !strings.HasPrefix(sc, "tgw_session=;") || !strings.Contains(sc, "Max-Age=0") {
		t.Errorf("Clear sent %q", sc)
	}
}

func TestNeverExpiringSession(t *testing.T) {
	c := &Cookies{Signer: NewSigner(testKey), Timeout: 0}
	s := c.NewSession(1, "a", "admin", 0)
	if s.Exp != 0 {
		t.Errorf("Exp = %d, want 0", s.Exp)
	}
	w := httptest.NewRecorder()
	c.Issue(w, s)
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "Max-Age=315360000") || strings.Contains(sc, "Secure") {
		t.Errorf("Set-Cookie %q", sc)
	}
}

func TestCSRF(t *testing.T) {
	s := NewSigner(testKey)
	r := httptest.NewRequest("POST", "/", nil)
	if s.CheckSessionCSRF(r, "") || s.SessionCSRF("") != "" {
		t.Error("CSRF passed without a session")
	}
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "abc.def"})
	tok := s.SessionCSRF("abc.def")
	if !s.CheckSessionCSRF(r, tok) || s.CheckSessionCSRF(r, tok+"x") {
		t.Error("session CSRF check wrong")
	}

	lt := s.NewLoginToken()
	lr := httptest.NewRequest("POST", "/", nil)
	if s.CheckLogin(lr, lt) {
		t.Error("login CSRF passed without its cookie")
	}
	lr.AddCookie(&http.Cookie{Name: LoginCookie, Value: lt})
	if !s.CheckLogin(lr, lt) || s.CheckLogin(lr, s.NewLoginToken()) {
		t.Error("login CSRF check wrong")
	}
	forged := httptest.NewRequest("POST", "/", nil)
	forged.AddCookie(&http.Cookie{Name: LoginCookie, Value: "a.b"})
	if s.CheckLogin(forged, "a.b") {
		t.Error("an unsigned login token passed")
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tgloggerd/webapi/internal/webdb"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewRateLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	l.RecordFailure("k")
	if !l.Allowed("k") {
		t.Fatal("blocked after one failure")
	}
	l.RecordFailure("k")
	if l.Allowed("k") {
		t.Fatal("allowed after two failures")
	}
	if !l.Allowed("other") {
		t.Fatal("another key is blocked")
	}
	now = now.Add(time.Minute)
	if !l.Allowed("k") {
		t.Fatal("blocked after the window")
	}
	l.RecordFailure("k")
	l.RecordFailure("k")
	l.Reset("k")
	if !l.Allowed("k") {
		t.Fatal("blocked after a reset")
	}

	u := NewRateLimiter(-1, time.Minute)
	for i := 0; i < 10; i++ {
		u.RecordFailure("k")
	}
	if !u.Allowed("k") {
		t.Error("a negative limit limits")
	}
	z := NewRateLimiter(0, time.Minute)
	z.RecordFailure("k")
	if z.Allowed("k") || !z.Allowed("fresh") {
		t.Error("a zero limit is wrong")
	}
}

func TestVerifier(t *testing.T) {
	c := &Cookies{Signer: NewSigner(testKey), Timeout: 3600}
	var epoch uint32
	var dbErr error
	active, ended := true, false
	var asked [32]byte
	v := &Verifier{Cookies: c, State: func(_ context.Context, uid uint64, key [32]byte) (webdb.SessionState, error) {
		if uid != 9 {
			return webdb.SessionState{}, webdb.ErrNoUser
		}
		asked = key
		return webdb.SessionState{Epoch: epoch, Role: "member", Active: active,
			MustChangePassword: true, SessionRow: 4, SessionEnded: ended}, dbErr
	}}
	req := func(cookie string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
		}
		return r
	}
	ctx := context.Background()
	good := c.Signer.EncodeSession(c.NewSession(9, "bob", "admin", 0))
	gone := c.Signer.EncodeSession(c.NewSession(8, "eve", "admin", 0))

	for _, tc := range []struct {
		name   string
		cookie string
		epoch  uint32
		err    error
		active bool
		ended  bool
		want   Check
	}{
		{"no cookie", "", 0, nil, true, false, None},
		{"forged", "eA.eA", 0, nil, true, false, None},
		{"valid", good, 0, nil, true, false, Valid},
		{"epoch bumped", good, 1, nil, true, false, Revoked},
		{"account gone", gone, 0, nil, true, false, Revoked},
		{"disabled", good, 0, nil, false, false, Revoked},
		{"session ended", good, 0, nil, true, true, Revoked},
		{"database down", good, 0, errors.New("down"), true, false, Unverified},
	} {
		epoch, dbErr, active, ended = tc.epoch, tc.err, tc.active, tc.ended
		s, got := v.Verify(ctx, req(tc.cookie))
		if got != tc.want {
			t.Errorf("%s: Verify = %v, want %v", tc.name, got, tc.want)
		}
		// The role and the flag are the account's, not the cookie's;
		// the session is looked up by its cookie's key.
		if got == Valid && (s.UID != 9 || s.Username != "bob" ||
			s.Role != "member" || !s.MustChangePassword || s.Row != 4 ||
			asked != SessionKey(good) || s.Key != asked) {
			t.Errorf("%s: session %+v", tc.name, s)
		}
	}
}

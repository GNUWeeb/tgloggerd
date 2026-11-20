// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"strconv"
	"testing"
	"time"
)

func testLimiter(max, keys int) (*RateLimiter, *time.Time) {
	now := time.Unix(1700000000, 0)
	l := NewRateLimiter(max, time.Minute)
	l.now = func() time.Time { return now }
	l.maxKeys = keys
	return l, &now
}

func TestRateLimiterLimits(t *testing.T) {
	l, now := testLimiter(2, 100)
	l.RecordFailure("a")
	if !l.Allowed("a") {
		t.Fatal("throttled after one failure")
	}
	l.RecordFailure("a")
	if l.Allowed("a") {
		t.Fatal("not throttled after two failures")
	}
	if !l.Allowed("b") {
		t.Fatal("another key is throttled")
	}
	*now = now.Add(time.Minute)
	if !l.Allowed("a") {
		t.Fatal("still throttled after the window")
	}
	l.RecordFailure("a")
	l.RecordFailure("a")
	l.Reset("a")
	if !l.Allowed("a") {
		t.Fatal("throttled after a reset")
	}
}

// Failures for ever new keys, as from someone cycling through
// usernames, must not grow the map without bound.
func TestRateLimiterBounded(t *testing.T) {
	l, now := testLimiter(3, 50)
	for i := 0; i < 1000; i++ {
		l.RecordFailure("k" + strconv.Itoa(i))
		*now = now.Add(time.Millisecond)
	}
	if n := len(l.entries); n > 50 {
		t.Fatalf("%d keys kept, want at most 50", n)
	}
	// The newest keys survive, so a throttle in progress keeps working.
	l.RecordFailure("k999")
	l.RecordFailure("k999")
	if l.Allowed("k999") {
		t.Fatal("the newest key lost its count")
	}
	// Once their window has passed, the keys are swept.
	*now = now.Add(time.Minute)
	l.RecordFailure("fresh")
	if n := len(l.entries); n != 1 {
		t.Fatalf("%d keys after the sweep, want 1", n)
	}
}

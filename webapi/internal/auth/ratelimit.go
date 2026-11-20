// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"sync"
	"time"
)

// RateLimiter is the login throttle of web/src/auth/RateLimiter.cpp: a
// fixed-window count of failures per key (the login keys on ip and
// username). Only failures count, and a successful login clears the
// key. Once a key's failures in its window reach the limit, Allowed is
// false until the window ends.
//
// The state is in this process only, as in the Drogon web, so each
// service throttles on its own.
//
// The keys come from requests, so their number is bounded: expired
// entries are swept once per window, and when MaxRateLimitKeys keys
// are still in their window, a new failure replaces the entry whose
// window started first.
type RateLimiter struct {
	max     int
	window  time.Duration
	now     func() time.Time
	maxKeys int

	mu        sync.Mutex
	entries   map[string]rlEntry
	lastSweep time.Time
}

// MaxRateLimitKeys is the most keys a RateLimiter keeps.
const MaxRateLimitKeys = 10000

type rlEntry struct {
	count int
	start time.Time
}

// NewRateLimiter allows max failures per window. A negative max, as
// the C++ reads it into an unsigned size, never limits.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{max: max, window: window, now: time.Now,
		maxKeys: MaxRateLimitKeys, entries: map[string]rlEntry{}}
}

func (l *RateLimiter) expired(e rlEntry, now time.Time) bool {
	return now.Sub(e.start) >= l.window
}

// Allowed reports whether another attempt for key is permitted.
func (l *RateLimiter) Allowed(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		return true
	}
	if l.expired(e, now) {
		delete(l.entries, key)
		return true
	}
	return l.max < 0 || e.count < l.max
}

// RecordFailure counts a failed attempt for key.
func (l *RateLimiter) RecordFailure(key string) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok || l.expired(e, now) {
		if !ok {
			l.makeRoom(now)
		}
		l.entries[key] = rlEntry{count: 1, start: now}
		return
	}
	e.count++
	l.entries[key] = e
}

// Reset clears key's failures (after a successful login).
func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// makeRoom keeps the map bounded before a new key is added: it drops
// expired entries at most once per window, and if the map is still
// full, the entry whose window started first. Called with mu held.
func (l *RateLimiter) makeRoom(now time.Time) {
	if now.Sub(l.lastSweep) >= l.window || len(l.entries) >= l.maxKeys {
		for k, e := range l.entries {
			if l.expired(e, now) {
				delete(l.entries, k)
			}
		}
		l.lastSweep = now
	}
	if len(l.entries) < l.maxKeys {
		return
	}
	var oldest string
	var start time.Time
	first := true
	for k, e := range l.entries {
		if first || e.start.Before(start) {
			oldest, start, first = k, e.start, false
		}
	}
	delete(l.entries, oldest)
}

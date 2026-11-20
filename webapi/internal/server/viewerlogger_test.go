// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Viewer.logger: the logging account, from the daemon's GET /v1/status
// and the sample fixture's users, over the admin tests' databases and
// fake daemon, each test with its own.

// loggerEnv is an admin env whose logger keeper runs on a clock the
// test moves.
type loggerEnv struct {
	*adminEnv
	lg  *loggerAccount
	mu  sync.Mutex
	now time.Time
}

func newLoggerEnv(t *testing.T) *loggerEnv {
	t.Helper()
	e := &loggerEnv{adminEnv: newAdminEnv(t), now: chatNow}
	clock := func() time.Time {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.now
	}
	e.lg = newLoggerAccount(slog.New(slog.NewTextHandler(io.Discard, nil)), e.deps.Daemon, clock)
	e.deps.logger = e.lg
	e.h = New(e.deps)
	// No ask outlives the test's daemon.
	t.Cleanup(e.lg.wait)
	return e
}

func (e *loggerEnv) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

// logger is the viewer's logger on path as role sees it; it fails the
// test when the viewer has no logger key at all (it is required).
func (e *loggerEnv) logger(role, path string) map[string]any {
	e.t.Helper()
	st, p := e.page(role, path)
	if st != 200 {
		e.t.Fatalf("%s %s: %d %v", role, path, st, p)
	}
	v, _ := p["viewer"].(map[string]any)
	l, present := v["logger"]
	if !present {
		e.t.Fatalf("%s %s: viewer without logger: %v", role, path, v)
	}
	m, _ := l.(map[string]any)
	return m
}

// statusAsks is how often the daemon was asked for its status.
func (e *loggerEnv) statusAsks() int {
	return e.daemon.statusCalls()
}

const loggerStatus = `{"tdlib":"ready","since":1763400000,"account":{"user_id":9299000001}}`

func TestViewerLoggerSignedOut(t *testing.T) {
	t.Parallel()
	e := newLoggerEnv(t)
	e.daemon.answer("/v1/status", 200, "", loggerStatus)

	// Unknown at first: the page does not wait for the daemon.
	if l := e.logger("", "/groups/-1001000000001"); l != nil {
		t.Errorf("logger before the daemon answered: %v", l)
	}
	e.lg.wait()

	// A signed-out visitor of an exposed group, and every role.
	want := map[string]any{"id": float64(9299000001), "name": "Dewi Kusuma",
		"username": "dewikusuma", "photo": "/files/" + e.deps.Tokens.Encrypt(880007)}
	for _, role := range []string{"", "member", "admin", "superadmin"} {
		l := e.logger(role, "/groups/-1001000000001")
		if len(l) != len(want) {
			t.Errorf("%q: logger %v, want %v", role, l, want)
			continue
		}
		for k, w := range want {
			if l[k] != w {
				t.Errorf("%q: logger.%s = %v, want %v", role, k, l[k], w)
			}
		}
	}
	// Known once, kept: no more asks.
	if n := e.statusAsks(); n != 1 {
		t.Errorf("status asked %d times, want 1", n)
	}
}

func TestViewerLoggerNoAccount(t *testing.T) {
	t.Parallel()
	e := newLoggerEnv(t)

	// The daemon away: null, and no second ask before the retry.
	e.daemon.answer("/v1/status", 503, "", `{"error":"unavailable","message":"x"}`)
	e.logger("", "/groups/-1001000000001")
	e.lg.wait()
	if l := e.logger("", "/groups/-1001000000001"); l != nil {
		t.Errorf("logger from a failed status: %v", l)
	}
	if n := e.statusAsks(); n != 1 {
		t.Errorf("status asked %d times within the retry, want 1", n)
	}

	// Not logged in yet: account null, still no logger.
	e.daemon.answer("/v1/status", 200, "", `{"tdlib":"waiting","since":5,"account":null}`)
	e.advance(loggerRetry)
	e.logger("member", "/")
	e.lg.wait()
	if l := e.logger("member", "/"); l != nil {
		t.Errorf("logger while the daemon has no account: %v", l)
	}
	if n := e.statusAsks(); n != 2 {
		t.Errorf("status asked %d times, want 2", n)
	}

	// Logged in since: the next ask finds it.
	e.daemon.answer("/v1/status", 200, "", loggerStatus)
	e.advance(loggerRetry)
	e.logger("member", "/")
	e.lg.wait()
	if l := e.logger("member", "/"); l == nil || l["id"] != float64(9299000001) {
		t.Errorf("logger after login: %v", l)
	}
}

func TestViewerLoggerUnknownUser(t *testing.T) {
	t.Parallel()
	e := newLoggerEnv(t)
	// An account the archive has no user row for: null.
	e.daemon.answer("/v1/status", 200, "",
		`{"tdlib":"ready","since":1,"account":{"user_id":9299999999}}`)
	e.logger("", "/groups/-1001000000001")
	e.lg.wait()
	if l := e.logger("", "/groups/-1001000000001"); l != nil {
		t.Errorf("logger without a user row: %v", l)
	}
}

func TestViewerLoggerNoDaemon(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.deps.Daemon = nil
	e.h = New(e.deps)
	st, p := e.page("", "/groups/-1001000000001")
	if st != 200 {
		t.Fatalf("page: %d %v", st, p)
	}
	v, _ := p["viewer"].(map[string]any)
	if l, present := v["logger"]; !present || l != nil {
		t.Errorf("logger without the daemon API: %v (present %v)", l, present)
	}
}

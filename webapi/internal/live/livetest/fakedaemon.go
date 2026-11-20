// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package livetest is a stand-in for the daemon's GET /v1/events, for
// tests. It listens on a loopback port the system picks, and the test
// drives each stream it accepts frame by frame.
package livetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/live"
)

// Daemon is the fake daemon.
type Daemon struct {
	URL    string
	bearer string
	conns  chan *Conn

	mu     sync.Mutex
	refuse int // a status to answer instead of a stream; 0: stream
}

// Conn is one accepted stream.
type Conn struct {
	// Since is the request's since parameter ("" without one).
	Since  string
	frames chan string
}

// endFrame, sent through frames, ends the stream after the frames
// before it.
const endFrame = ""

// New starts a fake daemon accepting the bearer key derived from key,
// stopped when the test ends.
func New(t testing.TB, key []byte) *Daemon {
	t.Helper()
	bearer, err := daemon.Bearer(key)
	if err != nil {
		t.Fatal(err)
	}
	d := &Daemon{bearer: bearer, conns: make(chan *Conn, 16)}
	srv := httptest.NewServer(http.HandlerFunc(d.serve))
	d.URL = srv.URL
	t.Cleanup(srv.Close)
	return d
}

// Refuse makes the daemon answer status (e.g. 503) instead of a
// stream; 0 serves streams again.
func (d *Daemon) Refuse(status int) {
	d.mu.Lock()
	d.refuse = status
	d.mu.Unlock()
}

func (d *Daemon) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/events" || r.Header.Get("Authorization") != d.bearer {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	d.mu.Lock()
	refuse := d.refuse
	d.mu.Unlock()
	if refuse != 0 {
		w.Header().Set("Retry-After", "1")
		http.Error(w, `{"error":"unavailable"}`, refuse)
		return
	}
	c := &Conn{Since: r.URL.Query().Get("since"), frames: make(chan string, 64)}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	d.conns <- c
	for {
		select {
		case f := <-c.frames:
			if f == endFrame {
				return
			}
			if _, err := fmt.Fprint(w, f); err != nil {
				return
			}
			_ = rc.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Accept waits for the next stream to open.
func (d *Daemon) Accept(t testing.TB) *Conn {
	t.Helper()
	select {
	case c := <-d.conns:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no connection to the fake daemon")
		return nil
	}
}

// Send writes one event frame, as the daemon writes it.
func (c *Conn) Send(e live.Event) {
	b, _ := json.Marshal(e)
	c.frames <- fmt.Sprintf("id: %d\ndata: %s\n\n", e.Seq, b)
}

// Keepalive writes the daemon's keepalive comment.
func (c *Conn) Keepalive() {
	c.frames <- ": keepalive\n\n"
}

// Raw writes text as it is.
func (c *Conn) Raw(s string) {
	c.frames <- s
}

// End closes the stream from the daemon's side.
func (c *Conn) End() {
	c.frames <- endFrame
}

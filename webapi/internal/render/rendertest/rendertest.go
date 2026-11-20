// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package rendertest is a stand-in render service for tests.
package rendertest

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// StandIn is a render service for tests: protocol version 1 on a Unix
// socket, answering fixed markup that names the route and echoes the
// props, so a test can tell the page was rendered from its data.
type StandIn struct {
	Socket string
	Build  string
	delay  atomic.Int64
	// Calls counts the render requests that reached it.
	Calls atomic.Int64
	// Last is the last render request's body.
	Last atomic.Value
	srv  *http.Server
}

// Start starts a stand-in in a directory of its own and stops
// it when the test ends.
func Start(t testing.TB, build string) *StandIn {
	t.Helper()
	dir, err := os.MkdirTemp("", "rnd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s := &StandIn{Socket: filepath.Join(dir, "render.sock"), Build: build}
	ln, err := net.Listen("unix", s.Socket)
	if err != nil {
		t.Fatal(err)
	}
	s.srv = &http.Server{Handler: s}
	go s.srv.Serve(ln)
	t.Cleanup(s.Stop)
	return s
}

// SetDelay holds every answer back by d, to play a slow renderer.
func (s *StandIn) SetDelay(d time.Duration) {
	s.delay.Store(int64(d))
}

// Stop closes the socket; later requests are refused.
func (s *StandIn) Stop() {
	_ = s.srv.Close()
}

func (s *StandIn) answer(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *StandIn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d := time.Duration(s.delay.Load()); d > 0 {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return
		}
	}
	if r.Method == http.MethodGet && r.URL.Path == "/health" {
		s.answer(w, 200, map[string]any{"v": 1, "ok": true, "build": s.Build, "uptime_s": 1})
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/render" {
		s.answer(w, 404, map[string]any{"v": 1, "error": "bad_request", "detail": "not served"})
		return
	}
	s.Calls.Add(1)
	var req struct {
		V      int             `json:"v"`
		Build  string          `json:"build"`
		Route  string          `json:"route"`
		URL    string          `json:"url"`
		Status int             `json:"status"`
		Props  json.RawMessage `json:"props"`
		Viewer json.RawMessage `json:"viewer"`
	}
	raw := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		s.answer(w, 400, map[string]any{"v": 1, "error": "bad_request", "detail": err.Error()})
		return
	}
	s.Last.Store([]byte(raw))
	_ = json.Unmarshal(raw, &req)
	switch {
	case req.V != 1:
		s.answer(w, 400, map[string]any{"v": 1, "error": "unknown_version", "detail": "v"})
	case req.Build != s.Build:
		s.answer(w, 409, map[string]any{"v": 1, "error": "build_mismatch", "detail": req.Build})
	case req.Route == "boom":
		s.answer(w, 500, map[string]any{"v": 1, "error": "render_failed", "detail": "threw"})
	default:
		s.answer(w, 200, map[string]any{
			"v": 1, "build": s.Build, "ms": 1,
			"head": "<title>" + req.Route + "</title>",
			"html": "<main data-route=\"" + req.Route + "\">" + string(req.Props) + "</main>",
		})
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/render/rendertest"
)

func testPage() Page {
	return Page{V: 1, Route: "login", URL: "/login", Status: 200,
		Props:  map[string]any{"next": nil},
		Viewer: Viewer{Now: "2026-11-18T00:00:00Z"}}
}

func TestRender(t *testing.T) {
	t.Parallel()
	s := rendertest.Start(t, "b1")
	c := NewClient(s.Socket, time.Second, time.Second)
	r, err := c.Render(context.Background(), testPage(), "rid", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Head != "<title>login</title>" || !strings.Contains(r.HTML, `{"next":null}`) {
		t.Errorf("got %+v", r)
	}
	var sent map[string]any
	_ = json.Unmarshal(s.Last.Load().([]byte), &sent)
	for _, k := range []string{"v", "request_id", "build", "route", "url", "status", "props", "viewer"} {
		if _, ok := sent[k]; !ok {
			t.Errorf("request lacks %s: %v", k, sent)
		}
	}
	h, err := c.Health(context.Background())
	if err != nil || !h.OK || h.Build != "b1" {
		t.Errorf("health %+v, %v", h, err)
	}
}

func TestRenderErrors(t *testing.T) {
	t.Parallel()
	s := rendertest.Start(t, "b1")
	c := NewClient(s.Socket, time.Second, time.Hour)
	_, err := c.Render(context.Background(), testPage(), "", "other")
	var re *Error
	if !errors.As(err, &re) || re.Status != 409 || re.Code != "build_mismatch" {
		t.Errorf("build mismatch: %v", err)
	}
	p := testPage()
	p.Route = "boom"
	if _, err := c.Render(context.Background(), p, "", "b1"); !errors.As(err, &re) || re.Status != 500 {
		t.Errorf("render failure: %v", err)
	}
	// An error answer is not an outage: the next request still goes out.
	if _, err := c.Render(context.Background(), testPage(), "", "b1"); err != nil {
		t.Errorf("after an error answer: %v", err)
	}
}

// A socket nobody listens on fails at once and starts the back-off.
func TestRenderDeadBacksOff(t *testing.T) {
	t.Parallel()
	c := NewClient(filepath.Join(t.TempDir(), "none.sock"), time.Second, time.Hour)
	if _, err := c.Render(context.Background(), testPage(), "", "b1"); err == nil || errors.Is(err, ErrSkipped) {
		t.Fatalf("first call: %v", err)
	}
	if _, err := c.Render(context.Background(), testPage(), "", "b1"); !errors.Is(err, ErrSkipped) {
		t.Errorf("second call not skipped: %v", err)
	}
}

// A slow renderer is given up on after the timeout, and skipped until
// the back-off ends.
func TestRenderSlow(t *testing.T) {
	t.Parallel()
	s := rendertest.Start(t, "b1")
	s.SetDelay(2 * time.Second)
	c := NewClient(s.Socket, 100*time.Millisecond, 200*time.Millisecond)
	start := time.Now()
	_, err := c.Render(context.Background(), testPage(), "", "b1")
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("slow render: %v after %v", err, time.Since(start))
	}
	if _, err := c.Render(context.Background(), testPage(), "", "b1"); !errors.Is(err, ErrSkipped) {
		t.Errorf("not skipped during the back-off: %v", err)
	}
	s.SetDelay(0)
	time.Sleep(250 * time.Millisecond)
	if _, err := c.Render(context.Background(), testPage(), "", "b1"); err != nil {
		t.Errorf("after the back-off: %v", err)
	}
}

// A request whose own client went away does not back off.
func TestRenderCanceledNoBackoff(t *testing.T) {
	t.Parallel()
	s := rendertest.Start(t, "b1")
	s.SetDelay(time.Second)
	c := NewClient(s.Socket, 5*time.Second, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Render(ctx, testPage(), "", "b1"); err == nil {
		t.Fatal("canceled render succeeded")
	}
	if c.skipping() {
		t.Error("a canceled request started the back-off")
	}
}

func TestNilClient(t *testing.T) {
	t.Parallel()
	var c *Client
	if _, err := c.Render(context.Background(), testPage(), "", "b"); !errors.Is(err, ErrSkipped) {
		t.Errorf("nil client: %v", err)
	}
}

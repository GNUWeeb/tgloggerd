// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// Result is the render service's markup for a page.
type Result struct {
	Head string // elements for <head>: title, meta, canonical link
	HTML string // the application root's markup
	MS   int    // render time the service reported
}

// ErrSkipped is returned while the client backs off after the render
// service failed to answer; the caller serves the fallback at once.
var ErrSkipped = errors.New("render service backed off")

// Error is a render service's error answer (400, 409, 422, 500).
type Error struct {
	Status int
	Code   string
	Detail string
}

func (e *Error) Error() string {
	return fmt.Sprintf("render service: %d %s: %s", e.Status, e.Code, e.Detail)
}

// Client talks to the render service over its Unix socket. A nil
// *Client renders nothing: every page is the fallback.
type Client struct {
	socket  string
	timeout time.Duration
	backoff time.Duration
	hc      *http.Client
	now     func() time.Time

	mu        sync.Mutex
	skipUntil time.Time
}

// NewClient returns a client of the render service at socket. timeout
// bounds connect and answer together (RENDER_TIMEOUT); after a
// connection that fails or an answer that does not come in time, the
// service is skipped for backoff (RENDER_BACKOFF), so a dead renderer
// costs no latency.
func NewClient(socket string, timeout, backoff time.Duration) *Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:        8,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	}
	return &Client{
		socket:  socket,
		timeout: timeout,
		backoff: backoff,
		hc:      &http.Client{Transport: tr},
		now:     time.Now,
	}
}

// Socket is the socket path the client dials.
func (c *Client) Socket() string {
	return c.socket
}

func (c *Client) skipping() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Before(c.skipUntil)
}

func (c *Client) backOff() {
	c.mu.Lock()
	c.skipUntil = c.now().Add(c.backoff)
	c.mu.Unlock()
}

// renderRequest is the body of POST /render (web-ssr.md 4.2).
type renderRequest struct {
	Page
	RequestID string `json:"request_id,omitempty"`
	Build     string `json:"build"`
}

type renderAnswer struct {
	V      int    `json:"v"`
	Build  string `json:"build"`
	Head   string `json:"head"`
	HTML   string `json:"html"`
	MS     int    `json:"ms"`
	Error  string `json:"error"`
	Detail string `json:"detail"`
}

// call sends one request and reads the answer, at most MaxBody bytes.
// A failure before an HTTP answer arrives (no socket, refused, timed
// out) starts the back-off, unless ctx itself ended: a client that went
// away says nothing about the renderer.
func (c *Client) call(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	if c.skipping() {
		return 0, nil, ErrSkipped
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(cctx, method, "http://render"+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err == nil {
		defer resp.Body.Close()
		var out []byte
		out, err = io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
		if err == nil {
			if len(out) > MaxBody {
				return resp.StatusCode, nil, fmt.Errorf("render service: answer over %d bytes", MaxBody)
			}
			return resp.StatusCode, out, nil
		}
	}
	if ctx.Err() == nil {
		c.backOff()
	}
	return 0, nil, err
}

// Render asks the render service for a page's markup. build is the id
// of the client assets the document links; markup of another build
// would not hydrate, so the service refuses it (409). Every error
// means the caller serves the fallback.
func (c *Client) Render(ctx context.Context, p Page, requestID, build string) (Result, error) {
	if c == nil {
		return Result{}, ErrSkipped
	}
	body, err := json.Marshal(renderRequest{Page: p, RequestID: requestID, Build: build})
	if err != nil {
		return Result{}, err
	}
	if len(body) > MaxBody {
		return Result{}, fmt.Errorf("render request over %d bytes", MaxBody)
	}
	status, out, err := c.call(ctx, http.MethodPost, "/render", body)
	if err != nil {
		return Result{}, err
	}
	var a renderAnswer
	if err := json.Unmarshal(out, &a); err != nil {
		return Result{}, fmt.Errorf("render service: %d answer is not JSON: %w", status, err)
	}
	if status != http.StatusOK {
		return Result{}, &Error{Status: status, Code: a.Error, Detail: a.Detail}
	}
	if a.V != Protocol {
		return Result{}, fmt.Errorf("render service: protocol version %d", a.V)
	}
	if a.Build != build {
		return Result{}, fmt.Errorf("render service: build %q, want %q", a.Build, build)
	}
	return Result{Head: a.Head, HTML: a.HTML, MS: a.MS}, nil
}

// Health is the render service's GET /health answer.
type Health struct {
	V       int    `json:"v"`
	OK      bool   `json:"ok"`
	Build   string `json:"build"`
	UptimeS int64  `json:"uptime_s"`
}

// Health asks the render service whether it is ready.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var h Health
	if c == nil {
		return h, ErrSkipped
	}
	status, out, err := c.call(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return h, err
	}
	if status != http.StatusOK {
		return h, &Error{Status: status, Code: "health"}
	}
	err = json.Unmarshal(out, &h)
	return h, err
}

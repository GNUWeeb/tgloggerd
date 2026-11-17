// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
)

// The "daemon" cases of ../auth/testdata/drogon.json are calls the
// Drogon web's client (web/src/daemon/Api.cpp) made to a scripted
// server: the request it sent, the raw answer it got and the Reply it
// made of it. Each case is replayed here: the same answer must give
// the same Reply, and the Go client must send the same request.

type daemonCase struct {
	Name    string `json:"name"`
	Request struct {
		RequestLine   string `json:"request_line"`
		Authorization string `json:"authorization"`
		ContentType   string `json:"content-type"`
		Body          string `json:"body"`
	} `json:"request"`
	Response string `json:"response"`
	Reply    struct {
		Sent       bool            `json:"sent"`
		Status     int             `json:"status"`
		Body       json.RawMessage `json:"body"`
		Error      string          `json:"error"`
		RetryAfter string          `json:"retry_after"`
	} `json:"reply"`
}

// calls are the Go calls matching each case's C++ call.
var calls = map[string]func(c *Client, ctx context.Context) Reply{
	"ok-json": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "POST", "/v1/x", map[string]any{"a": 1}, 5*time.Second)
	},
	"404-error": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "GET", "/v1/files/7", nil, 5*time.Second)
	},
	"503-retry-after": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "POST", "/v1/y", map[string]any{}, 5*time.Second)
	},
	"500-not-json": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "GET", "/v1/z", nil, 5*time.Second)
	},
	"204-empty": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "POST", "/v1/w", nil, 5*time.Second)
	},
	"400-error-not-string": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "GET", "/v1/v", nil, 5*time.Second)
	},
	"record-access": func(c *Client, ctx context.Context) Reply {
		return c.RecordAccess(ctx, []uint64{1, 2, 1<<64 - 1})
	},
	"record-access-empty": func(c *Client, ctx context.Context) Reply {
		return c.RecordAccess(ctx, nil)
	},
	"fetch": func(c *Client, ctx context.Context) Reply {
		return c.Fetch(ctx, 5, 10, 0)
	},
	"resolve-chat": func(c *Client, ctx context.Context) Reply {
		return c.ResolveChat(ctx, "@gnu")
	},
	"join-chat": func(c *Client, ctx context.Context) Reply {
		return c.JoinChat(ctx, "https://t.me/x")
	},
	"lookup-name": func(c *Client, ctx context.Context) Reply {
		return c.LookupRealName(ctx, -100123)
	},
	"timeout": func(c *Client, ctx context.Context) Reply {
		return c.Call(ctx, "GET", "/v1/slow", nil, time.Second)
	},
}

type seenRequest struct {
	line, auth, ctype, body string
}

// serveOnce answers one connection with a raw response, or holds it
// open without answering when raw is "", and reports the request.
func serveOnce(t *testing.T, ln net.Listener, raw string, seen chan<- seenRequest) {
	conn, err := ln.Accept()
	if err != nil {
		close(seen)
		return
	}
	defer conn.Close()
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		t.Errorf("reading the request: %v", err)
		close(seen)
		return
	}
	b, _ := io.ReadAll(req.Body)
	seen <- seenRequest{req.Method + " " + req.RequestURI + " " + req.Proto,
		req.Header.Get("Authorization"), req.Header.Get("Content-Type"), string(b)}
	if raw == "" {
		time.Sleep(1500 * time.Millisecond)
		return
	}
	io.WriteString(conn, raw)
}

func TestDrogonReplies(t *testing.T) {
	b, err := os.ReadFile("../auth/testdata/drogon.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		AppKey string       `json:"app_key"`
		Daemon []daemonCase `json:"daemon"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	key, err := auth.DecodeAppKey(f.AppKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Daemon) != len(calls) {
		t.Fatalf("%d cases, %d calls", len(f.Daemon), len(calls))
	}

	for _, dc := range f.Daemon {
		t.Run(dc.Name, func(t *testing.T) {
			call := calls[dc.Name]
			if call == nil {
				t.Fatalf("no Go call for case %q", dc.Name)
			}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			c, err := New("http://"+ln.Addr().String()+"/", key, FetchTimeout("30"))
			if err != nil {
				t.Fatal(err)
			}

			seen := make(chan seenRequest, 1)
			go serveOnce(t, ln, dc.Response, seen)
			r := call(c, context.Background())
			got := <-seen

			want := seenRequest{dc.Request.RequestLine, dc.Request.Authorization,
				dc.Request.ContentType, dc.Request.Body}
			if got != want {
				t.Errorf("request %+v\n Drogon sent %+v", got, want)
			}

			w := dc.Reply
			if r.Sent != w.Sent || r.Status != w.Status || r.RetryAfter != w.RetryAfter {
				t.Errorf("reply %+v, Drogon %+v", r, w)
			}
			wantBody := w.Body
			if bytes.Equal(wantBody, []byte("null")) {
				wantBody = nil
			}
			if !jsonEqual(r.Body, wantBody) {
				t.Errorf("body %s, Drogon %s", r.Body, w.Body)
			}
			// The transport's own wording differs; the wording made from
			// an answer must not.
			if w.Sent && r.Error != w.Error {
				t.Errorf("error %q, Drogon %q", r.Error, w.Error)
			}
			if !w.Sent && r.Error == "" {
				t.Error("an unsent call has no error")
			}
		})
	}
}

func jsonEqual(a, b json.RawMessage) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return bytes.Equal(xb, yb)
}

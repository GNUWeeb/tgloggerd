// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package searchengine is the optional search engine behind the
// advanced search's full-text fields (webapi/README "Search engine",
// docs/design/decisions/2026-11-18-search-engine.md): an OpenSearch
// client over plain HTTP, the index of the group and private messages
// and those of the users' bios and the groups' descriptions
// (profiles.go), the indexers that keep them in step with MySQL, and
// the queries that answer those entities' full-text conditions, the
// request scope always among their filters.
//
// MySQL stays the source of truth. The engine answers which rows
// match; the rows themselves are read from MySQL as before
// (logdb.Engine), so an engine that is down, slow or behind only means
// the search falls back to MySQL.
package searchengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxAnswer bounds an answer read from the engine. A page of ids, its
// highlights and facets are far smaller; the id lists of a search
// (MaxIDs ids) are the largest.
const maxAnswer = 64 << 20

// Client talks to one OpenSearch node over HTTP. It holds no
// credentials: the engine runs with its security plugin off, reachable
// on the compose network only.
type Client struct {
	base string
	hc   *http.Client
}

// NewClient is a client of the engine at base, an http(s) URL such as
// http://opensearch:9200.
func NewClient(base string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not an http(s) URL", base)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%q: give the engine's base URL only", base)
	}
	return &Client{
		base: strings.TrimRight(u.String(), "/"),
		hc: &http.Client{
			// Every call has its own deadline from its context; this
			// only bounds one that was given none.
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				Proxy:               nil,
				MaxIdleConnsPerHost: 16,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

// URL is the engine's base URL, for logs.
func (c *Client) URL() string { return c.base }

// Error is an answer of the engine other than 2xx.
type Error struct {
	Status int
	Type   string // error.type, e.g. index_not_found_exception
	Reason string
}

func (e *Error) Error() string {
	if e.Type == "" {
		return fmt.Sprintf("search engine: HTTP %d", e.Status)
	}
	return fmt.Sprintf("search engine: HTTP %d: %s: %s", e.Status, e.Type, e.Reason)
}

// IsNotFound is an answer 404: a missing index or document.
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

// do sends one request with a JSON body (nil for none; a []byte is sent
// as is, as the bulk body is) and decodes a JSON answer into out (nil:
// read and drop it).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	ctype := "application/json"
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
		ctype = "application/x-ndjson"
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return err
	}
	if rd != nil {
		req.Header.Set("Content-Type", ctype)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return err
	}
	if len(data) > maxAnswer {
		return fmt.Errorf("search engine: answer of %s %s over %d bytes", method, path, maxAnswer)
	}
	if resp.StatusCode/100 != 2 {
		e := &Error{Status: resp.StatusCode}
		var eb struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(data, &eb) == nil && len(eb.Error) > 0 {
			var detail struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			}
			if json.Unmarshal(eb.Error, &detail) == nil {
				e.Type, e.Reason = detail.Type, detail.Reason
			}
		}
		return e
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("search engine: answer of %s %s: %w", method, path, err)
	}
	return nil
}

// Ping is whether the engine answers and its cluster is usable (green
// or yellow), within ctx.
func (c *Client) Ping(ctx context.Context) error {
	var h struct {
		Status string `json:"status"`
	}
	if err := c.do(ctx, http.MethodGet, "/_cluster/health", nil, &h); err != nil {
		return err
	}
	if h.Status != "green" && h.Status != "yellow" {
		return fmt.Errorf("search engine: cluster is %s", h.Status)
	}
	return nil
}

// path escapes one path segment (an index name or document id).
func seg(s string) string { return url.PathEscape(s) }

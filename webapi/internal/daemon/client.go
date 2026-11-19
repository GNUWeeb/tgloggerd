// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package daemon is the client of the tgloggerd daemon's internal API
// (docs/internal-api.md), made as web/src/daemon/Api.cpp makes it: the
// same bearer key, request bodies, timeouts and reading of answers.
//
// Only the daemon holds the TDLib session and writes telegram_files, so
// what the web needs from Telegram or from the file store's bookkeeping
// is asked of it. The API is optional: with WEB_TGLOGGERD_API empty the
// client is off and every call answers "not sent" without touching the
// network. A refused connection or a timeout is an answer like any
// other, never an error the caller has to handle apart.
package daemon

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

// bearerLabel is what the daemon derives its bearer key from.
const bearerLabel = "tgloggerd:internal-api:v1"

// userAgent names this service in the daemon's logs.
const userAgent = "tgloggerd_webapi"

// maxReply bounds how much of an answer is read; the API's answers are
// small JSON objects.
const maxReply = 1 << 20

// Reply is the outcome of one call.
type Reply struct {
	Sent       bool            // an HTTP answer arrived (any status)
	Status     int             // its status; 0 when !Sent
	Body       json.RawMessage // the JSON body; nil if none or not JSON
	Error      string          // for logs: why it failed, if it did
	RetryAfter string          // the Retry-After header, "" if none
}

// OK reports a 2xx answer.
func (r Reply) OK() bool {
	return r.Sent && r.Status >= 200 && r.Status < 300
}

// Client calls the daemon. A nil *Client is a client that is off.
type Client struct {
	base         string
	auth         string
	hc           *http.Client
	fetchTimeout time.Duration
}

// Bearer is the Authorization header value for a decoded WEB_APP_KEY:
// "Bearer " and the lowercase hex of the keyed BLAKE2b-256 of the label.
func Bearer(key []byte) (string, error) {
	h, err := blake2b.New256(key)
	if err != nil {
		return "", err
	}
	h.Write([]byte(bearerLabel))
	return "Bearer " + hex.EncodeToString(h.Sum(nil)), nil
}

// parseBase accepts "http(s)://host[:port]" with at most a trailing
// slash, as the Drogon web does: the API's paths are absolute.
func parseBase(u string) (string, bool) {
	var scheme int
	switch {
	case strings.HasPrefix(u, "http://"):
		scheme = len("http://")
	case strings.HasPrefix(u, "https://"):
		scheme = len("https://")
	default:
		return "", false
	}
	u = strings.TrimSuffix(u, "/")
	if len(u) <= scheme || strings.ContainsAny(u[scheme:], "/?#@ ") {
		return "", false
	}
	return u, true
}

// FetchTimeout is how long a fetch waits: the daemon's own limit,
// TG_FETCH_TIMEOUT_SECS (30 when unset or not positive), plus a margin
// for the round trip, so a slow fetch gets the daemon's 504.
func FetchTimeout(env string) time.Duration {
	secs, err := strconv.ParseInt(env, 10, 64)
	if err != nil || secs <= 0 {
		secs = 30
	}
	return time.Duration(secs)*time.Second + 10*time.Second
}

// New makes the client for WEB_TGLOGGERD_API. An empty base gives a nil
// client (off) and no error; a malformed one gives an error, and the
// caller logs it and goes on without the daemon, as the Drogon web
// does.
func New(base string, key []byte, fetchTimeout time.Duration) (*Client, error) {
	if base == "" {
		return nil, nil
	}
	b, ok := parseBase(base)
	if !ok {
		return nil, errors.New("WEB_TGLOGGERD_API must be http://host:port")
	}
	auth, err := Bearer(key)
	if err != nil {
		return nil, err
	}
	return &Client{
		base: b,
		auth: auth,
		hc: &http.Client{
			// The Drogon client follows no redirects; the API sends none.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		fetchTimeout: fetchTimeout,
	}, nil
}

// Enabled reports whether the client is configured.
func (c *Client) Enabled() bool {
	return c != nil
}

// BaseURL is the configured base URL, for logs ("" when off).
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.base
}

// Call sends one request: path is absolute ("/v1/..."), body is sent as
// JSON unless it is nil. It gives up after timeout.
func (c *Client) Call(ctx context.Context, method, path string, body any,
	timeout time.Duration) Reply {
	var r Reply
	if c == nil {
		r.Error = "WEB_TGLOGGERD_API is not set"
		return r
	}

	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		rd = bytes.NewReader(b)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer resp.Body.Close()
	text, err := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if err != nil {
		// The status arrived but the body did not: not an answer.
		r.Error = err.Error()
		return r
	}

	r.Sent = true
	r.Status = resp.StatusCode
	r.RetryAfter = resp.Header.Get("Retry-After")
	if t := bytes.TrimSpace(text); len(t) > 0 && json.Valid(t) &&
		!bytes.Equal(t, []byte("null")) {
		r.Body = json.RawMessage(t)
	}
	if !r.OK() {
		r.Error = "HTTP " + strconv.Itoa(r.Status)
		var e struct {
			Error *string `json:"error"`
		}
		if r.Body != nil && json.Unmarshal(r.Body, &e) == nil && e.Error != nil {
			r.Error += " " + *e.Error
		}
	}
	return r
}

// RecordAccess is POST /v1/files/access: at most 1000 ids (the daemon
// refuses more). A daemon slower than a few seconds counts as away, and
// the caller drops the batch.
func (c *Client) RecordAccess(ctx context.Context, ids []uint64) Reply {
	if ids == nil {
		ids = []uint64{}
	}
	return c.Call(ctx, http.MethodPost, "/v1/files/access",
		map[string]any{"ids": ids}, 5*time.Second)
}

// ResolveChat is POST /v1/chats/resolve {"ref": ref}, for the Join chat
// page. The daemon waits up to 15 s on Telegram per step and a preview
// takes up to three.
func (c *Client) ResolveChat(ctx context.Context, ref string) Reply {
	return c.Call(ctx, http.MethodPost, "/v1/chats/resolve",
		map[string]any{"ref": ref}, 55*time.Second)
}

// JoinChat is POST /v1/chats/join {"ref": ref}; a join is up to four
// steps.
func (c *Client) JoinChat(ctx context.Context, ref string) Reply {
	return c.Call(ctx, http.MethodPost, "/v1/chats/join",
		map[string]any{"ref": ref}, 70*time.Second)
}

// Fetch is POST /v1/files/{id}/fetch: get a file's bytes from Telegram.
// length 0 asks for one chunk from offset. The answer is
// {state:"stored", id} or {state:"partial", path, offset, available,
// size} (docs/internal-api.md).
func (c *Client) Fetch(ctx context.Context, id, offset, length uint64) Reply {
	var timeout time.Duration
	if c != nil {
		timeout = c.fetchTimeout
	}
	return c.Call(ctx, http.MethodPost,
		"/v1/files/"+strconv.FormatUint(id, 10)+"/fetch",
		map[string]any{"offset": offset, "length": length}, timeout)
}

// LookupRealName is POST /v1/users/{id}/lookup-name: the daemon removes
// the contact, reads the name the account gave itself and adds the
// contact back, sitting out a short flood wait up to twice, so it gets
// two minutes.
func (c *Client) LookupRealName(ctx context.Context, userID int64) Reply {
	return c.Call(ctx, http.MethodPost,
		"/v1/users/"+strconv.FormatInt(userID, 10)+"/lookup-name",
		map[string]any{}, 120*time.Second)
}

// statusTimeout bounds the dashboard's calls: both answer from memory
// or one SUM on the daemon's side, and a daemon slower than this
// counts as away rather than holding the page up.
const statusTimeout = time.Second

// Status is GET /v1/status: the daemon's TDLib client state ("ready",
// "waiting" or "closing") and when it last changed (Unix seconds).
type Status struct {
	TDLib string `json:"tdlib"`
	Since int64  `json:"since"`
}

// Status asks for the TDLib client's state. ok is false when the
// client is off, the daemon does not answer within a second, or its
// answer is not a status.
func (c *Client) Status(ctx context.Context) (st Status, ok bool) {
	r := c.Call(ctx, http.MethodGet, "/v1/status", nil, statusTimeout)
	if !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &st) != nil || st.TDLib == "" {
		return Status{}, false
	}
	return st, true
}

// Storage is the part of GET /v1/storage the dashboard shows: the
// stored files, their bytes, and the cap (TG_STORAGE_MAX_BYTES, 0 for
// none).
type Storage struct {
	Files    int64 `json:"files"`
	Bytes    int64 `json:"bytes"`
	MaxBytes int64 `json:"max_bytes"`
}

// Storage asks for the store's size and cap, failing soft as Status
// does.
func (c *Client) Storage(ctx context.Context) (st Storage, ok bool) {
	var raw struct {
		Files    *int64 `json:"files"`
		Bytes    *int64 `json:"bytes"`
		MaxBytes *int64 `json:"max_bytes"`
	}
	r := c.Call(ctx, http.MethodGet, "/v1/storage", nil, statusTimeout)
	if !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &raw) != nil ||
		raw.Bytes == nil || raw.MaxBytes == nil {
		return Storage{}, false
	}
	st = Storage{Bytes: *raw.Bytes, MaxBytes: *raw.MaxBytes}
	if raw.Files != nil {
		st.Files = *raw.Files
	}
	return st, true
}

// The outcomes of POST /v1/users/resolve.
const (
	ResolveOK        = "ok"
	ResolveNotFound  = "not_found"
	ResolveNotAUser  = "not_a_user"
	ResolveFloodWait = "flood_wait"
	ResolveError     = "error"
)

// ResolvedUser is the user a username resolved to.
type ResolvedUser struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

// UserResolve is the daemon's answer to POST /v1/users/resolve. On
// ResolveOK the daemon has stored the user in telegram_users, with its
// full info and its profile photo (whose file may land a little
// later). RetryAfter is in seconds, for ResolveFloodWait only. A
// malformed username (400) is answered as ResolveError.
type UserResolve struct {
	Outcome    string        `json:"outcome"`
	Message    string        `json:"message"`
	RetryAfter int64         `json:"retry_after"`
	User       *ResolvedUser `json:"user"`
}

// resolveUserTimeout bounds a resolve: the daemon asks Telegram for
// the username and then for the user's full info, each normally in
// well under a second. A daemon slower than this counts as away.
const resolveUserTimeout = 30 * time.Second

// ResolveUser is POST /v1/users/resolve {"username": "@name"}: the
// daemon looks the username up on Telegram and stores the user it
// names. ok is false when there is no answer to go by: the client is
// off, the daemon is away, slow or not ready, or its answer is not a
// resolve's; the caller tries again later. reply is the call itself,
// for logs.
func (c *Client) ResolveUser(ctx context.Context, username string) (res UserResolve, ok bool, reply Reply) {
	return c.resolveUser(ctx, username, resolveUserTimeout)
}

func (c *Client) resolveUser(ctx context.Context, username string, timeout time.Duration) (UserResolve, bool, Reply) {
	if !strings.HasPrefix(username, "@") {
		username = "@" + username
	}
	r := c.Call(ctx, http.MethodPost, "/v1/users/resolve",
		map[string]any{"username": username}, timeout)
	var res UserResolve
	switch {
	case r.Sent && r.Status == http.StatusBadRequest:
		// The daemon refuses the username's shape: asking again would
		// be refused again.
		res = UserResolve{Outcome: ResolveError, Message: "the daemon refused the username"}
		if r.Body != nil {
			var e struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(r.Body, &e) == nil && e.Error != "" {
				res.Message = e.Error
			}
		}
		return res, true, r
	case !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &res) != nil:
		return UserResolve{}, false, r
	}
	switch res.Outcome {
	case ResolveOK:
		if res.User == nil || res.User.UserID == 0 {
			return UserResolve{}, false, r
		}
	case ResolveNotFound, ResolveNotAUser, ResolveFloodWait, ResolveError:
		res.User = nil
	default:
		return UserResolve{}, false, r
	}
	if res.Outcome != ResolveFloodWait {
		res.RetryAfter = 0
	} else if res.RetryAfter < 0 {
		res.RetryAfter = 0
	}
	return res, true, r
}

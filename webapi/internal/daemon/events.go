// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// ErrOff is returned by OpenEvents on a client that is off.
var ErrOff = errors.New("WEB_TGLOGGERD_API is not set")

// StatusError is a stream refused with a status other than 200.
type StatusError struct {
	Status     int
	RetryAfter string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET /v1/events: HTTP %d", e.Status)
}

// OpenEvents opens GET /v1/events (docs/internal-api.md): the daemon's
// stream of Server-Sent Events, starting after since (0: from now). The
// body stays open until ctx ends, the daemon closes it or the caller
// closes it; the caller reads the frames. A refusal is a *StatusError.
func (c *Client) OpenEvents(ctx context.Context, since uint64) (io.ReadCloser, error) {
	if c == nil {
		return nil, ErrOff
	}
	url := c.base + "/v1/events"
	if since != 0 {
		url += "?since=" + strconv.FormatUint(since, 10)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxReply))
		resp.Body.Close()
		return nil, &StatusError{Status: resp.StatusCode,
			RetryAfter: resp.Header.Get("Retry-After")}
	}
	return resp.Body, nil
}

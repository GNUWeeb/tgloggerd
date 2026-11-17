// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// The daemon evicts the least recently accessed files, and only it
// writes telegram_files, so the web tells it which files it serves
// (web/docs/media.md, "Access reporting"), as
// web/src/daemon/AccessReport.cpp does for the Drogon web.

const (
	// accessFlushEvery is how often the served ids are sent.
	accessFlushEvery = 60 * time.Second
	// accessBatch is the most ids the daemon takes per request.
	accessBatch = 1000
	// accessMaxPending bounds the distinct ids kept between flushes. A
	// minute of serving never comes near it; it only bounds memory
	// should a flush stall.
	accessMaxPending = 200000
)

// AccessReporter collects the ids of the files served and sends them to
// POST /v1/files/access once a minute. Nothing on the request path
// waits for the daemon. A nil *AccessReporter (the daemon API off)
// records nothing.
type AccessReporter struct {
	c     *Client
	log   *slog.Logger
	every time.Duration

	mu  sync.Mutex
	ids map[uint64]struct{} // guarded by mu

	// failing: the last flush failed. The next failure is logged only
	// after a success, so a daemon away for a day costs one line, not
	// 1440. Used by the flushing goroutine only.
	failing bool
}

// NewAccessReporter returns the reporter for a client, or nil when the
// client is off.
func NewAccessReporter(c *Client, log *slog.Logger) *AccessReporter {
	if c == nil {
		return nil
	}
	return &AccessReporter{c: c, log: log, every: accessFlushEvery,
		ids: map[uint64]struct{}{}}
}

// Record notes that the bytes of file id were served. A file served
// many times between flushes is reported once.
func (a *AccessReporter) Record(id uint64) {
	if a == nil || id == 0 {
		return
	}
	a.mu.Lock()
	if len(a.ids) < accessMaxPending {
		a.ids[id] = struct{}{}
	}
	a.mu.Unlock()
}

// Run flushes every minute until ctx ends. Flushes run one at a time,
// in this goroutine, so a slow daemon cannot stack them up.
func (a *AccessReporter) Run(ctx context.Context) {
	if a == nil {
		return
	}
	t := time.NewTicker(a.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.Flush(ctx)
		}
	}
}

// Flush sends the ids recorded since the last flush, in batches, and
// forgets them. Best effort: when the daemon is away, slow or refuses,
// this flush's ids are dropped, including the batches not sent yet; a
// file served again is reported again, and accessed_at only needs to
// be roughly right.
func (a *AccessReporter) Flush(ctx context.Context) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if len(a.ids) == 0 {
		a.mu.Unlock()
		return
	}
	ids := make([]uint64, 0, len(a.ids))
	for id := range a.ids {
		ids = append(ids, id)
	}
	a.ids = map[uint64]struct{}{}
	a.mu.Unlock()

	for i := 0; i < len(ids); i += accessBatch {
		r := a.c.RecordAccess(ctx, ids[i:min(len(ids), i+accessBatch)])
		if !r.OK() {
			if !a.failing {
				a.failing = true
				a.log.Warn("access report failed; ids dropped, and no more "+
					"of this until a report succeeds",
					"err", r.Error, "dropped", len(ids)-i)
			}
			return
		}
	}
	if a.failing {
		a.failing = false
		a.log.Info("access reports work again")
	}
}

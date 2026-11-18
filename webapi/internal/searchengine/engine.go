// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tgloggerd/webapi/internal/logdb"
)

// Engine answers searches from the index (logdb.Engine). It never
// waits long: every call has its own time limit, and after a failure
// the engine is skipped for a while, so a search that would wait on a
// dead engine goes to MySQL at once.
type Engine struct {
	x *Index
	o EngineOptions

	mu        sync.Mutex
	state     State
	stateAt   time.Time // when state was read
	downUntil time.Time
}

// EngineOptions are the engine's limits; zero fields take defaults.
type EngineOptions struct {
	// Timeout bounds one query (default 1500 ms).
	Timeout time.Duration
	// MaxLag is how long the index may go without being known in step
	// before searches fall back to MySQL as "behind" (default 5
	// minutes).
	MaxLag time.Duration
	// Backoff is how long the engine is skipped after a failure
	// (default 30 seconds).
	Backoff time.Duration
	Log     *slog.Logger
}

// stateTTL is how long the indexer's state is trusted before it is
// read again.
const stateTTL = 5 * time.Second

// NewEngine is the engine answering from x.
func NewEngine(x *Index, o EngineOptions) *Engine {
	if o.Timeout <= 0 {
		o.Timeout = 1500 * time.Millisecond
	}
	if o.MaxLag <= 0 {
		o.MaxLag = 5 * time.Minute
	}
	if o.Backoff <= 0 {
		o.Backoff = 30 * time.Second
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Engine{x: x, o: o}
}

// Name is the engine's kind, for the search API's capabilities.
func (e *Engine) Name() string { return "opensearch" }

// failed notes a failure: the engine is skipped for the backoff. A
// request that was cancelled by its own client is no failure.
func (e *Engine) failed(parent context.Context, what string, err error) {
	if parent.Err() != nil {
		return
	}
	e.mu.Lock()
	e.downUntil = time.Now().Add(e.o.Backoff)
	e.mu.Unlock()
	e.o.Log.Warn("search engine: failed; searches use MySQL for a while", "what", what,
		"err", err, "for", e.o.Backoff)
}

// Ready implements logdb.Engine: the engine answers, and the index has
// the entity's whole table and was in step within MaxLag.
func (e *Engine) Ready(ctx context.Context, entity string) (bool, string) {
	sp, ok := scopeOf(entity)
	if !ok {
		return false, logdb.FallbackUnavailable
	}
	e.mu.Lock()
	now := time.Now()
	if now.Before(e.downUntil) {
		e.mu.Unlock()
		return false, logdb.FallbackUnavailable
	}
	st, fresh := e.state, now.Sub(e.stateAt) < stateTTL
	e.mu.Unlock()
	if !fresh {
		cctx, cancel := context.WithTimeout(ctx, e.o.Timeout)
		s, err := e.x.LoadState(cctx)
		cancel()
		if err != nil {
			e.failed(ctx, "reading the index state", err)
			return false, logdb.FallbackUnavailable
		}
		e.mu.Lock()
		e.state, e.stateAt, st = s, now, s
		e.mu.Unlock()
	}
	if !st.table(sp).Done || now.Sub(time.Unix(st.SyncedAt, 0)) > e.o.MaxLag {
		return false, logdb.FallbackBehind
	}
	return true, ""
}

// Query implements logdb.Engine.
func (e *Engine) Query(ctx context.Context, q logdb.EngineQuery) (*logdb.EngineResult, error) {
	body, err := searchBody(q)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, e.o.Timeout)
	defer cancel()
	var ans struct {
		Hits struct {
			Total struct {
				Value    int64  `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			Hits []struct {
				ID        string `json:"_id"`
				Highlight struct {
					Text []string `json:"text"`
				} `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations struct {
			Chats struct {
				Buckets []struct {
					Key   int64 `json:"key"`
					Count int64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"chats"`
		} `json:"aggregations"`
	}
	if err := e.x.c.do(cctx, http.MethodPost, "/"+seg(e.x.name)+"/_search", body, &ans); err != nil {
		if cctx.Err() != nil && ctx.Err() == nil {
			err = fmt.Errorf("%w: %v", context.DeadlineExceeded, err)
		}
		e.failed(ctx, "search", err)
		return nil, err
	}
	if ans.Hits.Total.Relation != "eq" {
		// track_total_hits is on: anything else is not this engine.
		err := fmt.Errorf("search engine: total is %s, not exact", ans.Hits.Total.Relation)
		e.failed(ctx, "search", err)
		return nil, err
	}
	sp, _ := scopeOf(q.Entity)
	r := &logdb.EngineResult{Total: ans.Hits.Total.Value, IDs: make([]int64, 0, len(ans.Hits.Hits))}
	if q.Highlight {
		r.Highlights = map[int64][]string{}
	}
	for _, h := range ans.Hits.Hits {
		s, row, ok := parseDocID(h.ID)
		if !ok || s != sp {
			// Cannot be: the scope filter keeps the entity's own.
			return nil, fmt.Errorf("search engine: unexpected document %q", h.ID)
		}
		r.IDs = append(r.IDs, row)
		if q.Highlight && len(h.Highlight.Text) > 0 {
			r.Highlights[row] = h.Highlight.Text
		}
	}
	if q.Facets {
		r.Facets = []logdb.Facet{}
		for _, b := range ans.Aggregations.Chats.Buckets {
			r.Facets = append(r.Facets, logdb.Facet{ChatID: b.Key, Count: b.Count})
		}
	}
	return r, nil
}

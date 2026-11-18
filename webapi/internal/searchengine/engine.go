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
	"tgloggerd/webapi/internal/scope"
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
	warming   bool // StartWarmUp's queries are running
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
	// WarmTimeout bounds one query of the warm-up (StartWarmUp;
	// default 30 seconds).
	WarmTimeout time.Duration
	Log         *slog.Logger
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
	if o.WarmTimeout <= 0 {
		o.WarmTimeout = 30 * time.Second
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
	if e.warming || now.Before(e.downUntil) {
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

// StartWarmUp warms the engine up in the background and returns a
// channel closed when it is done; until then searches go to MySQL
// ("unavailable") without waiting on the engine.
//
// The first queries an engine answers after the service starts are
// much slower than the same queries later (connections, the index's
// files read into memory, the engine's own caches): they ran past
// Timeout, fell back to MySQL, and the failure's backoff then kept
// every search on MySQL for its whole length. The warm-up sends a few
// searches of the shapes the pages send, each allowed WarmTimeout, so
// the first search of a viewer meets a warm engine; MySQL answers
// meanwhile, as it would during the backoff. A warm-up that fails
// leaves the engine to the usual failure handling.
func (e *Engine) StartWarmUp(ctx context.Context) <-chan struct{} {
	e.mu.Lock()
	e.warming = true
	e.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		start := time.Now()
		err := e.warmUp(ctx)
		e.mu.Lock()
		e.warming = false
		e.mu.Unlock()
		if err != nil {
			e.o.Log.Warn("search engine: warm-up failed", "err", err, "took", time.Since(start))
			return
		}
		e.o.Log.Info("search engine: warmed up", "took", time.Since(start))
	}()
	return done
}

// warmQueries are the warm-up's searches: relevance and date order,
// a fuzzy word, highlights and facets, for both entities, as a viewer
// who sees everything.
func warmQueries() []logdb.EngineQuery {
	all := scope.For(scope.Viewer{Role: scope.Superadmin})
	var out []logdb.EngineQuery
	for _, ent := range []string{logdb.GroupMessages.Name, logdb.PrivateMessages.Name} {
		out = append(out,
			logdb.EngineQuery{Entity: ent, Scope: all, Match: []string{"the"}, Size: 50,
				Highlight: true, Facets: true},
			logdb.EngineQuery{Entity: ent, Scope: all, Match: []string{"message~"},
				Sort: "date", Desc: true, Size: 50, Highlight: true})
	}
	return out
}

func (e *Engine) warmUp(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, e.o.WarmTimeout)
	s, err := e.x.LoadState(cctx)
	cancel()
	if err != nil {
		return fmt.Errorf("reading the index state: %w", err)
	}
	e.mu.Lock()
	e.state, e.stateAt = s, time.Now()
	e.mu.Unlock()
	if s == (State{}) {
		// Nothing indexed yet: searches go to MySQL as behind, and the
		// index may not even exist.
		return nil
	}
	for _, q := range warmQueries() {
		body, err := searchBody(q)
		if err != nil {
			return err
		}
		cctx, cancel := context.WithTimeout(ctx, e.o.WarmTimeout)
		err = e.x.c.do(cctx, http.MethodPost, "/"+seg(e.x.name)+"/_search", body, nil)
		cancel()
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
	}
	return nil
}

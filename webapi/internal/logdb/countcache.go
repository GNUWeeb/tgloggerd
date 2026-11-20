// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// CountCache keeps the answers of whole-table and list counts (COUNT,
// SUM) for a short time, so a page that shows the archive's totals
// does not count millions of rows on every load: the dashboard's
// cards, the lists' totals, the store's size.
//
// An answer is kept by its exact SQL and arguments and by the pool it
// ran on, so it is the same for every viewer who would run the same
// query: the request scope is in the SQL (its conditions and values),
// never beside it. Requests for a count being computed wait for it
// instead of running it again. A failed query (a cancelled request, a
// bad statement) is not kept, so the caller's fallback still runs and
// the next request tries again.
//
// A count that runs out of its statement time limit (MAX_EXECUTION_TIME)
// is different: it would only run out again, costing every request the
// whole limit before the caller falls back to its estimate. Such a
// count is remembered as slow for slowTTL, and requests for it answer
// errCountDeferred at once, so their callers estimate without waiting.
// The count runs once more in the background, one at a time across the
// cache and with the longer limit slowLimit, and its answer is kept for
// slowTTL. A background count that fails keeps the key deferred until
// slowTTL has passed. A slow count whose time is up is counted again in
// the background while requests keep getting its last answer, so a
// known slow count never holds up a request again.
//
// The totals it gives are at most TTL old, or slowTTL plus the time
// a background count takes for a slow one, which the pages can afford:
// they are counts of an archive that grows by the second anyway.
type CountCache struct {
	ttl time.Duration
	max int
	// slowTTL is how long a timed-out count stays deferred and how
	// long a background count's answer is kept; slowLimit is the
	// background count's statement time limit.
	slowTTL, slowLimit time.Duration
	// query runs one count: queryInt, but for the tests.
	query func(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error)

	bg chan struct{} // one slot per background count running
	wg sync.WaitGroup

	mu sync.Mutex
	m  map[string]*countEntry
}

type countEntry struct {
	done chan struct{} // closed once n, err, at, keep and slow are set
	at   time.Time
	keep time.Duration // how long the answer (or the deferral) holds
	n    int64
	err  error
	// slow marks a count that ran out of its time limit: it is
	// counted in the background only. refreshing (c.mu held) is set
	// once a background count is under way for an expired slow entry.
	slow       bool
	refreshing bool
}

// errCountDeferred answers a count known to be slow while it has no
// answer: the caller falls back to its estimate without waiting.
var errCountDeferred = errors.New("count deferred to the background")

// countCacheMax bounds the answers kept: filtered list totals are one
// answer per filter, and an expired answer is dropped only when the map
// is full.
const countCacheMax = 2048

const (
	// countSlowTTL is how long a timed-out count is answered by
	// estimate and a background count's answer is kept.
	countSlowTTL = 10 * time.Minute
	// countSlowLimit is a background count's statement time limit.
	countSlowLimit = 60 * time.Second
	// countBackgroundMax is how many background counts run at once:
	// each holds a connection of the pool for up to countSlowLimit,
	// so they must not take the pool from the requests.
	countBackgroundMax = 1
)

// NewCountCache is a cache keeping answers for ttl; nil (no cache) for
// ttl <= 0.
func NewCountCache(ttl time.Duration) *CountCache {
	if ttl <= 0 {
		return nil
	}
	return &CountCache{ttl: ttl, max: countCacheMax, slowTTL: countSlowTTL,
		slowLimit: countSlowLimit, query: queryInt,
		bg: make(chan struct{}, countBackgroundMax), m: map[string]*countEntry{}}
}

type countCacheKey struct{}

// WithCountCache gives the requests of ctx the cache c (nil: none).
func WithCountCache(ctx context.Context, c *CountCache) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, countCacheKey{}, c)
}

// countInt runs a query answering one integer (NULL reads as 0),
// through the request's count cache when it has one.
func countInt(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error) {
	c, _ := ctx.Value(countCacheKey{}).(*CountCache)
	if c == nil {
		return queryInt(ctx, db, q, args...)
	}
	return c.get(ctx, db, q, args)
}

func queryInt(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error) {
	var n sql.NullInt64
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n.Int64, err
}

func (c *CountCache) get(ctx context.Context, db *sql.DB, q string, args []any) (int64, error) {
	key := fmt.Sprintf("%p\x00%s\x00%#v", db, q, args)
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.m[key]; ok {
		select {
		case <-e.done:
			fresh := now.Sub(e.at) < e.keep
			if e.slow {
				if !fresh && !e.refreshing {
					// Count it again, never inline; until then
					// the last answer (or the estimate) stands.
					e.refreshing = true
					c.background(key, e, db, q, args)
				}
				c.mu.Unlock()
				return e.n, e.err
			}
			if e.err == nil && fresh {
				c.mu.Unlock()
				return e.n, nil
			}
		default:
			// Being computed: wait for it.
			c.mu.Unlock()
			select {
			case <-e.done:
				if e.err == nil || e.slow {
					// An answer, or a count that ran out of
					// time: running it again would too.
					return e.n, e.err
				}
				// Its request failed (or went away); run our own.
				return c.query(ctx, db, q, args...)
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}
	if len(c.m) >= c.max {
		c.prune(now)
	}
	e := &countEntry{done: make(chan struct{}), keep: c.ttl}
	c.m[key] = e
	c.mu.Unlock()

	n, err := c.query(ctx, db, q, args...)
	c.mu.Lock()
	e.n, e.err, e.at = n, err, time.Now()
	if isTimeout(err) {
		// Remember it, and count it in the background instead.
		e.n, e.err, e.slow, e.keep = 0, errCountDeferred, true, c.slowTTL
		if c.m[key] == e {
			c.background(key, e, db, q, args)
		}
	} else if err != nil && c.m[key] == e {
		delete(c.m, key)
	}
	close(e.done)
	c.mu.Unlock()
	return e.n, e.err
}

// hintLimit is a statement's MAX_EXECUTION_TIME hint.
var hintLimit = regexp.MustCompile(`MAX_EXECUTION_TIME\([0-9]+\)`)

// background counts q for the slow entry old, which stays the key's
// answer until the count is done: then the count's answer replaces it,
// or a deferral when the count fails too, both kept for slowTTL. It
// does not block: the count waits for a free background slot in its
// own goroutine. c.mu is held.
func (c *CountCache) background(key string, old *countEntry, db *sql.DB, q string, args []any) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.bg <- struct{}{}
		defer func() { <-c.bg }()
		ctx, cancel := context.WithTimeout(context.Background(), c.slowLimit+5*time.Second)
		defer cancel()
		limit := "MAX_EXECUTION_TIME(" + strconv.FormatInt(c.slowLimit.Milliseconds(), 10) + ")"
		n, err := c.query(ctx, db, hintLimit.ReplaceAllLiteralString(q, limit), args...)

		e := &countEntry{done: make(chan struct{}), at: time.Now(), keep: c.slowTTL,
			n: n, slow: true}
		if err != nil {
			e.n, e.err = 0, errCountDeferred
		}
		close(e.done)
		c.mu.Lock()
		if c.m[key] == old {
			// Not pruned meanwhile.
			c.m[key] = e
		}
		c.mu.Unlock()
	}()
}

// prune drops the expired answers, or every answer when none has
// expired; c.mu is held.
func (c *CountCache) prune(now time.Time) {
	for k, e := range c.m {
		select {
		case <-e.done:
			if now.Sub(e.at) >= e.keep && !e.refreshing {
				delete(c.m, k)
			}
		default:
		}
	}
	if len(c.m) >= c.max {
		clear(c.m)
	}
}

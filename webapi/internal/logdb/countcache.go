// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"fmt"
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
// instead of running it again. A failed query (a time limit, a
// cancelled request) is not kept, so the caller's fallback still runs
// and the next request tries again.
//
// The totals it gives are at most TTL old, which the pages can afford:
// they are counts of an archive that grows by the second anyway.
type CountCache struct {
	ttl time.Duration
	max int

	mu sync.Mutex
	m  map[string]*countEntry
}

type countEntry struct {
	done chan struct{} // closed once n and err are set
	at   time.Time
	n    int64
	err  error
}

// countCacheMax bounds the answers kept: filtered list totals are one
// answer per filter, and an expired answer is dropped only when the map
// is full.
const countCacheMax = 2048

// NewCountCache is a cache keeping answers for ttl; nil (no cache) for
// ttl <= 0.
func NewCountCache(ttl time.Duration) *CountCache {
	if ttl <= 0 {
		return nil
	}
	return &CountCache{ttl: ttl, max: countCacheMax, m: map[string]*countEntry{}}
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
			if e.err == nil && now.Sub(e.at) < c.ttl {
				c.mu.Unlock()
				return e.n, nil
			}
		default:
			// Being computed: wait for it.
			c.mu.Unlock()
			select {
			case <-e.done:
				if e.err == nil {
					return e.n, nil
				}
				// Its request failed (or went away); run our own.
				return queryInt(ctx, db, q, args...)
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}
	if len(c.m) >= c.max {
		c.prune(now)
	}
	e := &countEntry{done: make(chan struct{})}
	c.m[key] = e
	c.mu.Unlock()

	n, err := queryInt(ctx, db, q, args...)
	c.mu.Lock()
	e.n, e.err, e.at = n, err, time.Now()
	if err != nil && c.m[key] == e {
		delete(c.m, key)
	}
	close(e.done)
	c.mu.Unlock()
	return n, err
}

// prune drops the expired answers, or every answer when none has
// expired; c.mu is held.
func (c *CountCache) prune(now time.Time) {
	for k, e := range c.m {
		select {
		case <-e.done:
			if now.Sub(e.at) >= c.ttl {
				delete(c.m, k)
			}
		default:
		}
	}
	if len(c.m) >= c.max {
		clear(c.m)
	}
}

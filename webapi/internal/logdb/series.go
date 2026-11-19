// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"container/list"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"tgloggerd/webapi/internal/scope"
)

// The dashboard's message series: the messages logged per period over
// a range, counted as the dashboard's per-day bars count them (the
// group messages in the scope and, when the scope sees them, the
// private messages), in the viewer's timezone.

// Period is a series' step.
type Period string

const (
	Minute  Period = "minute"
	Hour    Period = "hour"
	Hours4  Period = "4h"
	Day     Period = "day"
	Week    Period = "week"
	Month   Period = "month"
	Quarter Period = "quarter"
	Year    Period = "year"
)

// periodLimits are the longest ranges a series may ask for, in
// seconds, so a chart has at most about a thousand points and a query
// stays cheap; 0 is unlimited. Days are counted as 86400 seconds: the
// limit is about the cost, not the calendar.
var periodLimits = map[Period]int64{
	Minute:  86400,
	Hour:    45 * 86400,
	Hours4:  180 * 86400,
	Day:     1096 * 86400,
	Week:    5479 * 86400,
	Month:   0,
	Quarter: 0,
	Year:    0,
}

// ParsePeriod is the period named s.
func ParsePeriod(s string) (Period, bool) {
	p := Period(s)
	_, ok := periodLimits[p]
	return p, ok
}

// Limit is the period's longest range in seconds; 0 is unlimited.
func (p Period) Limit() int64 { return periodLimits[p] }

// MaxSeriesTime is the latest time a series may name: the end of the
// year 9999 UTC, so every boundary stays a valid calendar date.
const MaxSeriesTime = 253402300799

// floor is the start of the period of p in loc holding the unix time t.
//
// A minute or an hour is cut by the instant's own UTC offset, so the
// hour repeated when daylight saving time ends is two hours, as the
// clock showed them. Longer periods start at a wall-clock time
// (00:00, 04:00, ... for 4h, midnight for the rest); a day across a
// change is 23 or 25 hours long.
func (p Period) floor(t int64, loc *time.Location) int64 {
	switch p {
	case Minute, Hour:
		step := int64(60)
		if p == Hour {
			step = 3600
		}
		_, off := time.Unix(t, 0).In(loc).Zone()
		return t - mod(t+int64(off), step)
	}
	lt := time.Unix(t, 0).In(loc)
	y, m, d := lt.Date()
	var f time.Time
	switch p {
	case Hours4:
		f = time.Date(y, m, d, lt.Hour()-lt.Hour()%4, 0, 0, 0, loc)
	case Day:
		f = time.Date(y, m, d, 0, 0, 0, 0, loc)
	case Week:
		f = time.Date(y, m, d-(int(lt.Weekday())+6)%7, 0, 0, 0, 0, loc)
	case Month:
		f = time.Date(y, m, 1, 0, 0, 0, 0, loc)
	case Quarter:
		f = time.Date(y, m-(m-1)%3, 1, 0, 0, 0, 0, loc)
	default:
		f = time.Date(y, 1, 1, 0, 0, 0, 0, loc)
	}
	// A wall time that does not exist, or exists twice, can land after
	// t; the period before it holds t then.
	for f.Unix() > t {
		f = time.Unix(p.floor(f.Unix()-1, loc), 0)
	}
	return f.Unix()
}

// next is the start of the period after the one starting at b (a
// floor).
func (p Period) next(b int64, loc *time.Location) int64 {
	var n int64
	switch p {
	case Minute:
		n = p.floor(b+60, loc)
	case Hour:
		n = p.floor(b+3600, loc)
	default:
		lt := time.Unix(b, 0).In(loc)
		y, m, d := lt.Date()
		h := lt.Hour()
		var t time.Time
		switch p {
		case Hours4:
			t = time.Date(y, m, d, h+4, 0, 0, 0, loc)
		case Day:
			t = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		case Week:
			t = time.Date(y, m, d+7, 0, 0, 0, 0, loc)
		case Month:
			t = time.Date(y, m+1, 1, 0, 0, 0, 0, loc)
		case Quarter:
			t = time.Date(y, m+3, 1, 0, 0, 0, 0, loc)
		default:
			t = time.Date(y+1, 1, 1, 0, 0, 0, 0, loc)
		}
		n = p.floor(t.Unix(), loc)
	}
	if n <= b {
		// Only a zone change odder than any in use gets here; move on
		// by at least a minute so the series ends.
		n = p.floor(b+60, loc)
		for n <= b {
			n += 60
		}
	}
	return n
}

func mod(a, b int64) int64 {
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}

// Series is a series' answer: the range snapped to the period's
// boundaries and one count per period, oldest first.
type Series struct {
	Period     Period
	Start, End int64
	Points     []SeriesPoint
}

// SeriesPoint is one period: its start (unix) and its messages.
type SeriesPoint struct {
	At int64 `json:"at"`
	N  int64 `json:"n"`
}

// Errors of LoadSeries' arguments.
var (
	ErrSeriesRange   = errors.New("series: start not before end, or outside 0..year 9999")
	ErrSeriesTooLong = errors.New("series: range over the period's limit")
)

// seriesGrace is how long after its end a period is still counted
// afresh: a message is logged a moment after it was sent, and its date
// is the send time.
const seriesGrace = 5 * time.Minute

// LoadSeries counts the messages in the scope per period p in loc over
// [start, end), end being now when live. The limit applies to the
// range as asked; the answer covers it snapped outwards to whole
// periods, and a live one runs to the end of the period holding now.
// Finished periods come from c when it holds them (c may be nil);
// periods that start after now are zero without a query.
func LoadSeries(ctx context.Context, db *sql.DB, c *SeriesCache, sc scope.Scope, p Period,
	start, end int64, live bool, now int64, loc *time.Location) (Series, error) {
	if live {
		end = now
	}
	if start < 0 || end > MaxSeriesTime || start >= end {
		return Series{}, ErrSeriesRange
	}
	if l := p.Limit(); l > 0 && end-start > l {
		return Series{}, ErrSeriesTooLong
	}
	bounds := []int64{p.floor(start, loc)}
	fe := p.floor(end, loc)
	if live || fe != end {
		fe = p.next(fe, loc)
	}
	for b := bounds[0]; b < fe; {
		b = p.next(b, loc)
		bounds = append(bounds, b)
	}
	s := Series{Period: p, Start: bounds[0], End: bounds[len(bounds)-1],
		Points: make([]SeriesPoint, len(bounds)-1)}
	for i := range s.Points {
		s.Points[i].At = bounds[i]
	}
	var counts []int64
	var err error
	switch p {
	case Week, Month, Quarter, Year:
		counts, err = sumDays(ctx, db, c, sc, bounds, now, loc)
	default:
		counts, err = countPeriods(ctx, db, c, sc, p, bounds, now, loc)
	}
	if err != nil {
		return Series{}, err
	}
	for i := range s.Points {
		s.Points[i].N = counts[i]
	}
	return s, nil
}

// sumDays counts the periods between bounds, which start at midnight
// in loc (a week, month, quarter or year), as sums of days: every such
// period is a whole number of days, so these periods share the days
// cached for the day series, and the current period costs a count of
// today alone. The days before the archive's first message are zeros
// without being counted or cached.
func sumDays(ctx context.Context, db *sql.DB, c *SeriesCache, sc scope.Scope, bounds []int64,
	now int64, loc *time.Location) ([]int64, error) {
	out := make([]int64, len(bounds)-1)
	first, ok, err := firstMessage(ctx, db, sc)
	if err != nil || !ok {
		return out, err
	}
	from := max(bounds[0], Day.floor(first, loc))
	to := min(bounds[len(bounds)-1], Day.next(Day.floor(now, loc), loc))
	if from >= to {
		return out, nil
	}
	days := []int64{from}
	for b := from; b < to; {
		b = Day.next(b, loc)
		days = append(days, b)
	}
	n, err := countPeriods(ctx, db, c, sc, Day, days, now, loc)
	if err != nil {
		return nil, err
	}
	for d, v := range n {
		// The period holding the day: the last bound not after it.
		i := sort.Search(len(bounds), func(i int) bool { return bounds[i] > days[d] }) - 1
		if i >= 0 && i < len(out) {
			out[i] += v
		}
	}
	return out, nil
}

// firstMessage is the date of the oldest message the scope may count,
// read off the date indexes; ok is false when there is none.
func firstMessage(ctx context.Context, db *sql.DB, sc scope.Scope) (int64, bool, error) {
	qs := []string{"SELECT MIN(date) FROM telegram_group_messages"}
	if sc.SeesPrivate() {
		qs = append(qs, "SELECT MIN(date) FROM telegram_private_messages")
	}
	var first int64
	ok := false
	for _, q := range qs {
		var v sql.NullInt64
		if err := db.QueryRowContext(ctx, q).Scan(&v); err != nil {
			return 0, false, err
		}
		if v.Valid && (!ok || v.Int64 < first) {
			first, ok = v.Int64, true
		}
	}
	return max(first, 0), ok, nil
}

// countPeriods counts the periods p between bounds: the finished ones
// from c when it holds them (c may be nil), the others with one query
// over the span from the first period not cached to the last that has
// started; periods that start after now are zero without a query.
func countPeriods(ctx context.Context, db *sql.DB, c *SeriesCache, sc scope.Scope, p Period,
	bounds []int64, now int64, loc *time.Location) ([]int64, error) {
	out := make([]int64, len(bounds)-1)
	key := seriesKey(sc, p, loc)
	finished := now - int64(seriesGrace/time.Second)
	var hit map[int64]int64
	if c != nil {
		hit = c.get(key, bounds[0], bounds[len(bounds)-1])
	}
	lo, hi := -1, -1
	for i := range out {
		if bounds[i] > now {
			break
		}
		if n, ok := hit[bounds[i]]; ok && bounds[i+1] <= finished {
			out[i] = n
			continue
		}
		if lo < 0 {
			lo = i
		}
		hi = i
	}
	if lo < 0 {
		return out, nil
	}
	counts, err := countSpan(ctx, db, sc, bounds[lo:hi+2])
	if err != nil {
		return nil, err
	}
	done := map[int64]int64{}
	for i := lo; i <= hi; i++ {
		out[i] = counts[i-lo]
		if bounds[i+1] <= finished {
			done[bounds[i]] = counts[i-lo]
		}
	}
	if c != nil && len(done) > 0 {
		c.put(key, done)
	}
	return out, nil
}

// countSpan counts the messages in the scope in each period between
// consecutive bounds.
//
// The periods are found by grouping on (date - first bound) DIV unit,
// unit being the greatest common divisor of the bounds' distances from
// the first (a day in Jakarta, an hour across a daylight saving change,
// a minute for minutes), so every bound is a whole number of units from
// the first and each group falls in one period, which the bounds' list
// gives. Measured on ten million messages, this is twice as fast as
// placing each row with INTERVAL() over the bounds, and nearly as fast
// as a bare COUNT over the date index.
//
// An admin with hidden groups would read every row for its chat_id
// (no index has date and chat_id in that order), so its counts are
// every group's less the hidden groups', both read from indexes alone:
// the date index, and the (chat_id, date) one for the hidden groups.
func countSpan(ctx context.Context, db *sql.DB, sc scope.Scope, bounds []int64) ([]int64, error) {
	out := make([]int64, len(bounds)-1)
	b0, bn := bounds[0], bounds[len(bounds)-1]
	unit := int64(0)
	for _, b := range bounds[1:] {
		unit = gcd(unit, b-b0)
	}
	head := "SELECT (m.date - ?) DIV ? AS k, COUNT(*) FROM "
	where := " m WHERE m.date >= ? AND m.date < ?"
	args := []any{b0, unit, b0, bn}
	type part struct {
		q    string
		args []any
		sign int64
	}
	var parts []part
	group := head + "telegram_group_messages" + where
	if hidden := sc.HiddenGroups(); len(hidden) > 0 && !sc.SeesAllGroups() {
		in := make([]any, len(hidden))
		for i, g := range hidden {
			in[i] = g
		}
		parts = append(parts, part{group + " GROUP BY k", args, 1},
			part{group + " AND m.chat_id IN (" + strings.TrimSuffix(strings.Repeat("?, ", len(in)), ", ") +
				") GROUP BY k", append(args[:len(args):len(args)], in...), -1})
	} else if cond, cargs := sc.GroupFilter("m.chat_id"); cond != "1 = 0" {
		parts = append(parts, part{group + " AND " + cond + " GROUP BY k",
			append(args[:len(args):len(args)], cargs...), 1})
	}
	if sc.SeesPrivate() {
		parts = append(parts, part{head + "telegram_private_messages" + where + " GROUP BY k", args, 1})
	}
	for _, pt := range parts {
		rows, err := db.QueryContext(ctx, pt.q, pt.args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var k, n int64
			if err := rows.Scan(&k, &n); err != nil {
				rows.Close()
				return nil, err
			}
			at := b0 + k*unit
			// The period holding at: the last bound not after it.
			i := sort.Search(len(bounds), func(i int) bool { return bounds[i] > at }) - 1
			if i >= 0 && i < len(out) {
				out[i] += pt.sign * n
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// seriesKey names a series in the cache: what the scope's queries
// read (its group condition and values, and whether it sees private
// messages), the timezone and the period. Two viewers who see the same
// messages share their cached periods.
func seriesKey(sc scope.Scope, p Period, loc *time.Location) string {
	cond, args := sc.GroupFilter("m.chat_id")
	return fmt.Sprintf("%s|%v|%t|%s|%s", cond, args, sc.SeesPrivate(), loc.String(), p)
}

// SeriesCache keeps the counts of finished periods, which only change
// when older history is logged late. It holds at most max periods,
// dropping the least recently used series first, and forgets a series
// ttl after it was first filled, so history logged late shows up
// within that time.
type SeriesCache struct {
	max int
	ttl time.Duration
	now func() time.Time

	mu    sync.Mutex
	n     int // periods held
	lru   *list.List
	byKey map[string]*list.Element
}

type seriesEntry struct {
	key    string
	at     time.Time
	counts map[int64]int64
}

// NewSeriesCache keeps at most max periods for ttl.
func NewSeriesCache(max int, ttl time.Duration) *SeriesCache {
	return &SeriesCache{max: max, ttl: ttl, now: time.Now, lru: list.New(),
		byKey: map[string]*list.Element{}}
}

// Default sizes of the series cache: a period costs some 50 bytes.
const (
	SeriesCachePeriods = 200000
	SeriesCacheTTL     = time.Hour
)

// get answers the cached periods of key starting in [from, to).
func (c *SeriesCache) get(key string, from, to int64) map[int64]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.byKey[key]
	if !ok {
		return nil
	}
	e := el.Value.(*seriesEntry)
	if c.now().Sub(e.at) > c.ttl {
		c.drop(el)
		return nil
	}
	c.lru.MoveToFront(el)
	out := map[int64]int64{}
	for at, n := range e.counts {
		if at >= from && at < to {
			out[at] = n
		}
	}
	return out
}

// put adds finished periods to key's series.
func (c *SeriesCache) put(key string, counts map[int64]int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.byKey[key]
	if ok && c.now().Sub(el.Value.(*seriesEntry).at) > c.ttl {
		c.drop(el)
		ok = false
	}
	if !ok {
		el = c.lru.PushFront(&seriesEntry{key: key, at: c.now(), counts: map[int64]int64{}})
		c.byKey[key] = el
	}
	c.lru.MoveToFront(el)
	e := el.Value.(*seriesEntry)
	for at, n := range counts {
		if _, had := e.counts[at]; !had {
			c.n++
		}
		e.counts[at] = n
	}
	// Over the bound: drop the least recently used series, the one
	// just filled last of all.
	for c.n > c.max {
		c.drop(c.lru.Back())
	}
}

func (c *SeriesCache) drop(el *list.Element) {
	e := el.Value.(*seriesEntry)
	c.n -= len(e.counts)
	delete(c.byKey, e.key)
	c.lru.Remove(el)
}

// Len is the number of periods held.
func (c *SeriesCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

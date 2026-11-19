// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/testdb"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// Every period's boundaries are wall-clock boundaries in the zone, on
// both sides of Berlin's changes: 29 March 2026 (02:00 becomes 03:00)
// and 25 October 2026 (03:00 becomes 02:00).
func TestSeriesPeriodsAcrossDST(t *testing.T) {
	t.Parallel()
	be := mustZone(t, "Europe/Berlin")
	at := func(y int, m time.Month, d, h, min int) int64 {
		return time.Date(y, m, d, h, min, 0, 0, be).Unix()
	}
	// The starts of consecutive periods from the period holding from,
	// as wall times in Berlin.
	walk := func(p Period, from int64, n int) []string {
		var out []string
		b := p.floor(from, be)
		for range n {
			out = append(out, time.Unix(b, 0).In(be).Format("2006-01-02 15:04 MST"))
			b = p.next(b, be)
		}
		return out
	}
	for _, tc := range []struct {
		p    Period
		from int64
		want []string
	}{
		{Minute, at(2026, 3, 29, 1, 59) + 30, []string{"2026-03-29 01:59 CET",
			"2026-03-29 03:00 CEST", "2026-03-29 03:01 CEST"}},
		{Hour, at(2026, 3, 29, 1, 30), []string{"2026-03-29 01:00 CET",
			"2026-03-29 03:00 CEST", "2026-03-29 04:00 CEST"}},
		// The hour repeated in October is two hours, as the clock shows.
		{Hour, at(2026, 10, 25, 1, 30), []string{"2026-10-25 01:00 CEST",
			"2026-10-25 02:00 CEST", "2026-10-25 02:00 CET", "2026-10-25 03:00 CET"}},
		{Hours4, at(2026, 3, 29, 3, 30), []string{"2026-03-29 00:00 CET",
			"2026-03-29 04:00 CEST", "2026-03-29 08:00 CEST"}},
		{Hours4, at(2026, 10, 25, 2, 30) + 3600, []string{"2026-10-25 00:00 CEST",
			"2026-10-25 04:00 CET"}},
		{Day, at(2026, 3, 28, 12, 0), []string{"2026-03-28 00:00 CET",
			"2026-03-29 00:00 CET", "2026-03-30 00:00 CEST"}},
		{Day, at(2026, 10, 25, 23, 59), []string{"2026-10-25 00:00 CEST",
			"2026-10-26 00:00 CET"}},
		// 29 March 2026 is a Sunday: its week started on Monday 23.
		{Week, at(2026, 3, 29, 12, 0), []string{"2026-03-23 00:00 CET",
			"2026-03-30 00:00 CEST", "2026-04-06 00:00 CEST"}},
		{Month, at(2026, 10, 31, 23, 0), []string{"2026-10-01 00:00 CEST",
			"2026-11-01 00:00 CET", "2026-12-01 00:00 CET"}},
		{Quarter, at(2026, 3, 31, 23, 59), []string{"2026-01-01 00:00 CET",
			"2026-04-01 00:00 CEST", "2026-07-01 00:00 CEST", "2026-10-01 00:00 CEST",
			"2027-01-01 00:00 CET"}},
		{Year, at(2026, 12, 31, 23, 59), []string{"2026-01-01 00:00 CET",
			"2027-01-01 00:00 CET"}},
	} {
		got := walk(tc.p, tc.from, len(tc.want))
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s from %s: %v, want %v", tc.p,
					time.Unix(tc.from, 0).In(be), got, tc.want)
				break
			}
		}
	}
	// The lengths of the changed days and hours.
	if d := Day.next(at(2026, 3, 29, 0, 0), be) - at(2026, 3, 29, 0, 0); d != 23*3600 {
		t.Errorf("29 March: %d s", d)
	}
	if d := Day.next(at(2026, 10, 25, 0, 0), be) - at(2026, 10, 25, 0, 0); d != 25*3600 {
		t.Errorf("25 October: %d s", d)
	}
	// A quarter-hour zone: Kathmandu's hours start at :15 UTC.
	ka := mustZone(t, "Asia/Kathmandu")
	h := Hour.floor(time.Date(2026, 11, 18, 10, 20, 0, 0, time.UTC).Unix(), ka)
	if got := time.Unix(h, 0).UTC().Format("15:04"); got != "10:15" {
		t.Errorf("Kathmandu's hour starts at %s UTC", got)
	}
}

// seriesDB is a logger database whose group messages are only the
// ones the test adds.
func seriesDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testdb.New(t, testdb.Logger, "sample").DB
	for _, q := range []string{"DELETE FROM telegram_group_messages",
		"DELETE FROM telegram_private_messages"} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

var seriesMsgID atomic.Int64

func addGroupMsg(t *testing.T, db *sql.DB, chat, at int64) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO telegram_group_messages (chat_id, message_id, date, "+
		"content_type, text) VALUES (?, ?, ?, 'text', 'x')", chat, seriesMsgID.Add(1)<<20,
		at); err != nil {
		t.Fatal(err)
	}
}

func addPrivateMsg(t *testing.T, db *sql.DB, at int64) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO telegram_private_messages (chat_id, message_id, date, "+
		"content_type, text) VALUES (9299000001, ?, ?, 'text', 'x')", seriesMsgID.Add(1)<<20,
		at); err != nil {
		t.Fatal(err)
	}
}

func counts(s Series) []int64 {
	out := make([]int64, len(s.Points))
	for i, p := range s.Points {
		out[i] = p.N
	}
	return out
}

func sameInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The counts per period across Berlin's October change, snapped
// outwards, zeros included, in the scope, and the limits.
func TestLoadSeries(t *testing.T) {
	t.Parallel()
	db := seriesDB(t)
	ctx := context.Background()
	be := mustZone(t, "Europe/Berlin")
	at := func(d, h, min int) int64 { return time.Date(2026, 10, d, h, min, 0, 0, be).Unix() }
	const g1, g2 = -1001000000001, -1001000000003
	// 02:30 CEST and 02:30 CET are an hour apart.
	// (time.Date may pick either for the wall time 02:30 itself.)
	cest230 := at(25, 0, 30) + 2*3600
	cet230 := cest230 + 3600
	for _, m := range []int64{at(24, 23, 59), at(25, 0, 0), cest230, cet230, cet230 + 60,
		at(25, 23, 59)} {
		addGroupMsg(t, db, g1, m)
	}
	addGroupMsg(t, db, g2, at(25, 12, 0))
	addPrivateMsg(t, db, at(25, 12, 0))
	now := at(28, 12, 0)
	all := scope.Everything()

	// Hours: start and end inside hours are snapped outwards; the
	// repeated 02:00 hour is two points.
	s, err := LoadSeries(ctx, db, nil, all, Hour, at(25, 0, 10), cet230+600, false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if s.Start != at(25, 0, 0) || s.End != cet230-1800+3600 {
		t.Errorf("hours: %d..%d", s.Start, s.End)
	}
	if want := []int64{1, 0, 1, 2}; !sameInts(counts(s), want) {
		t.Errorf("hours: %v, want %v", counts(s), want)
	}

	// Days: the 25 hours of 25 October are one day, the private
	// message is counted, and the empty days are zeros.
	s, err = LoadSeries(ctx, db, nil, all, Day, at(23, 12, 0), at(27, 0, 0), false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{0, 1, 7, 0}; !sameInts(counts(s), want) {
		t.Errorf("days: %v, want %v", counts(s), want)
	}
	if s.End != at(27, 0, 0) {
		t.Errorf("days end at %v, an exact boundary is kept", time.Unix(s.End, 0).In(be))
	}

	// A member sees its granted group only, and no private messages.
	mem := scope.For(scope.Viewer{ID: 5, Role: scope.Member, GrantedGroups: []int64{g2}})
	s, err = LoadSeries(ctx, db, nil, mem, Day, at(23, 12, 0), at(27, 0, 0), false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{0, 0, 1, 0}; !sameInts(counts(s), want) {
		t.Errorf("member days: %v, want %v", counts(s), want)
	}
	// An admin with private chats hidden misses the private message.
	adm := scope.For(scope.Viewer{ID: 3, Role: scope.Admin, HidePrivate: true})
	s, err = LoadSeries(ctx, db, nil, adm, Day, at(25, 0, 0), at(26, 0, 0), false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{6}; !sameInts(counts(s), want) {
		t.Errorf("admin without private: %v, want %v", counts(s), want)
	}

	// Every other period over the same messages: all 8 in one point.
	for _, p := range []Period{Week, Month, Quarter, Year} {
		s, err = LoadSeries(ctx, db, nil, all, p, at(20, 0, 0), at(26, 0, 0), false, now, be)
		if err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, n := range counts(s) {
			sum += n
		}
		if sum != 8 || len(s.Points) != 1 {
			t.Errorf("%s: %v", p, counts(s))
		}
	}
	// Four hours: 00:00-04:00 holds the 00:00 and both 02:30s.
	s, err = LoadSeries(ctx, db, nil, all, Hours4, at(25, 0, 0), at(25, 8, 0), false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{4, 0}; !sameInts(counts(s), want) {
		t.Errorf("4h: %v, want %v", counts(s), want)
	}
	// Minutes: 02:30 and 02:31 CET.
	s, err = LoadSeries(ctx, db, nil, all, Minute, cet230, cet230+120, false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{1, 1}; !sameInts(counts(s), want) {
		t.Errorf("minutes: %v, want %v", counts(s), want)
	}

	// Limits: the longest range is allowed, a second more is not; month
	// and longer are unlimited.
	for _, p := range []Period{Minute, Hour, Hours4, Day, Week} {
		l := p.Limit()
		if _, err := LoadSeries(ctx, db, nil, all, p, now-l, now, false, now, be); err != nil {
			t.Errorf("%s over its limit: %v", p, err)
		}
		if _, err := LoadSeries(ctx, db, nil, all, p, now-l-1, now, false, now, be); !errors.Is(err, ErrSeriesTooLong) {
			t.Errorf("%s past its limit: %v", p, err)
		}
		if _, err := LoadSeries(ctx, db, nil, all, p, now-l-1, 0, true, now, be); !errors.Is(err, ErrSeriesTooLong) {
			t.Errorf("%s live past its limit: %v", p, err)
		}
	}
	for _, p := range []Period{Month, Quarter, Year} {
		if p.Limit() != 0 {
			t.Errorf("%s is limited", p)
		}
		s, err := LoadSeries(ctx, db, nil, all, p, 0, now, false, now, be)
		if err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, n := range counts(s) {
			sum += n
		}
		if sum != 8 || s.Start != time.Date(1970, 1, 1, 0, 0, 0, 0, be).Unix() {
			t.Errorf("%s all time: %d messages from %d", p, sum, s.Start)
		}
	}
	for _, r := range [][2]int64{{now, now}, {now, now - 1}, {-1, now}, {0, MaxSeriesTime + 1}} {
		if _, err := LoadSeries(ctx, db, nil, all, Year, r[0], r[1], false, now, be); !errors.Is(err, ErrSeriesRange) {
			t.Errorf("%v: %v", r, err)
		}
	}

	// Live: the end is the end of the period holding now; periods
	// after now are zero.
	addGroupMsg(t, db, g1, now-10)
	s, err = LoadSeries(ctx, db, nil, all, Hour, now-7200, 0, true, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if s.End != now+3600 || !sameInts(counts(s), []int64{0, 1, 0}) {
		t.Errorf("live: end %d (now %d), %v", s.End, now, counts(s))
	}
	s, err = LoadSeries(ctx, db, nil, all, Hour, now-3600, now+7200, false, now, be)
	if err != nil {
		t.Fatal(err)
	}
	if !sameInts(counts(s), []int64{1, 0, 0}) {
		t.Errorf("into the future: %v", counts(s))
	}
}

// Finished periods come from the cache: a message added to one later
// is not seen until the series expires, while the current period is
// counted afresh; a scope or a zone of its own is a series of its own.
func TestSeriesCache(t *testing.T) {
	t.Parallel()
	db := seriesDB(t)
	ctx := context.Background()
	be := mustZone(t, "Europe/Berlin")
	const g1 = -1001000000001
	now := time.Date(2026, 11, 18, 10, 30, 0, 0, be).Unix()
	hour := Hour.floor(now, be)
	addGroupMsg(t, db, g1, hour-7200)
	addGroupMsg(t, db, g1, hour+60)

	c := NewSeriesCache(100, time.Hour)
	clock := time.Unix(now, 0)
	c.now = func() time.Time { return clock }
	all := scope.Everything()
	load := func(sc scope.Scope, loc *time.Location) []int64 {
		t.Helper()
		s, err := LoadSeries(ctx, db, c, sc, Hour, hour-3*3600, 0, true, now, loc)
		if err != nil {
			t.Fatal(err)
		}
		return counts(s)
	}
	if got := load(all, be); !sameInts(got, []int64{0, 1, 0, 1}) {
		t.Fatalf("first: %v", got)
	}
	// The three finished hours are cached, the current one is not.
	if c.Len() != 3 {
		t.Errorf("cached %d periods, want 3", c.Len())
	}
	addGroupMsg(t, db, g1, hour-7200+5) // late history, in a cached hour
	addGroupMsg(t, db, g1, hour+120)    // the current hour
	if got := load(all, be); !sameInts(got, []int64{0, 1, 0, 2}) {
		t.Errorf("cached: %v", got)
	}
	// Another scope and another zone do not share it.
	mem := scope.For(scope.Viewer{ID: 5, Role: scope.Member, GrantedGroups: []int64{g1}})
	if got := load(mem, be); !sameInts(got, []int64{0, 2, 0, 2}) {
		t.Errorf("member: %v", got)
	}
	if got := load(all, time.UTC); !sameInts(got, []int64{0, 2, 0, 2}) {
		t.Errorf("UTC: %v", got)
	}
	// After the TTL the late message shows.
	clock = clock.Add(time.Hour + time.Second)
	if got := load(all, be); !sameInts(got, []int64{0, 2, 0, 2}) {
		t.Errorf("expired: %v", got)
	}
	// The bound: filling more periods than it holds drops the least
	// recently used series.
	small := NewSeriesCache(5, time.Hour)
	small.put("a", map[int64]int64{1: 1, 2: 1, 3: 1})
	small.put("b", map[int64]int64{1: 1, 2: 1})
	small.get("a", 0, 10)
	small.put("c", map[int64]int64{1: 1})
	if small.Len() != 4 || small.get("b", 0, 10) != nil || small.get("a", 0, 10) == nil {
		t.Errorf("bound: %d periods held, b %v", small.Len(), small.get("b", 0, 10))
	}
}

// A period that ended only a moment ago is not cached yet: a message
// is logged a little after its send time.
func TestSeriesGrace(t *testing.T) {
	t.Parallel()
	db := seriesDB(t)
	ctx := context.Background()
	const g1 = -1001000000001
	now := time.Date(2026, 11, 18, 10, 1, 0, 0, time.UTC).Unix()
	c := NewSeriesCache(100, time.Hour)
	all := scope.Everything()
	if _, err := LoadSeries(ctx, db, c, all, Hour, now-3600, 0, true, now, time.UTC); err != nil {
		t.Fatal(err)
	}
	addGroupMsg(t, db, g1, now-120) // 09:59, logged at 10:01
	s, err := LoadSeries(ctx, db, c, all, Hour, now-3600, 0, true, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !sameInts(counts(s), []int64{1, 0}) {
		t.Errorf("%v", counts(s))
	}
}

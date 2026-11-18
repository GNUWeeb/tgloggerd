// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"testing"
	"time"

	"tgloggerd/webapi/internal/testdb"
)

// The dashboard's days are calendar days in the viewer's timezone:
// a zone east or west of UTC, one with a quarter-hour offset, and one
// whose daylight saving time makes a day 23 or 25 hours long.
func TestPerLocalDay(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample").DB
	if _, err := db.Exec("DELETE FROM telegram_group_messages"); err != nil {
		t.Fatal(err)
	}
	add := func(at time.Time) {
		t.Helper()
		if _, err := db.Exec("INSERT INTO telegram_group_messages (chat_id, message_id, date, "+
			"content_type, text) VALUES (-1001000000001, ?, ?, 'text', 'x')",
			at.UnixNano()>>10, at.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	zone := func(name string) *time.Location {
		l, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	q := quarterIndex + "FROM telegram_group_messages m WHERE m.date >= ? GROUP BY 1"
	ctx := context.Background()

	for _, tc := range []struct {
		zone      string
		now       time.Time // in the zone
		msgs      []time.Time
		start     string
		last, day int64 // counts of the last day and the day before
	}{
		// Midnight boundaries, not UTC's: 23:59 is yesterday, 00:00 today.
		{"Asia/Jakarta", time.Date(2026, 11, 18, 10, 0, 0, 0, zone("Asia/Jakarta")),
			[]time.Time{
				time.Date(2026, 11, 17, 23, 59, 0, 0, zone("Asia/Jakarta")),
				time.Date(2026, 11, 18, 0, 0, 0, 0, zone("Asia/Jakarta")),
				time.Date(2026, 11, 18, 9, 0, 0, 0, zone("Asia/Jakarta")),
			}, "2026-10-20", 2, 1},
		{"UTC", time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC),
			[]time.Time{time.Date(2026, 11, 17, 23, 59, 0, 0, time.UTC),
				time.Date(2026, 11, 18, 0, 0, 1, 0, time.UTC)}, "2026-10-20", 1, 1},
		// UTC+5:45: a day starts at 18:15 UTC.
		{"Asia/Kathmandu", time.Date(2026, 11, 18, 12, 0, 0, 0, zone("Asia/Kathmandu")),
			[]time.Time{
				time.Date(2026, 11, 17, 23, 50, 0, 0, zone("Asia/Kathmandu")),
				time.Date(2026, 11, 18, 0, 10, 0, 0, zone("Asia/Kathmandu")),
			}, "2026-10-20", 1, 1},
		// New York leaves daylight saving time on 1 November 2026: that day
		// has 25 hours, and its 23:30 is still that day.
		{"America/New_York", time.Date(2026, 11, 2, 12, 0, 0, 0, zone("America/New_York")),
			[]time.Time{
				time.Date(2026, 11, 1, 23, 30, 0, 0, zone("America/New_York")),
				time.Date(2026, 11, 1, 0, 30, 0, 0, zone("America/New_York")),
				time.Date(2026, 11, 2, 0, 30, 0, 0, zone("America/New_York")),
			}, "2026-10-04", 1, 2},
	} {
		if _, err := db.Exec("DELETE FROM telegram_group_messages"); err != nil {
			t.Fatal(err)
		}
		for _, m := range tc.msgs {
			add(m)
		}
		loc := zone(tc.zone)
		got, start, err := perLocalDay(ctx, db, tc.now.Unix(), loc, q)
		if err != nil {
			t.Fatal(err)
		}
		s := time.Unix(start, 0).In(loc)
		if s.Format(time.DateOnly) != tc.start || s.Hour() != 0 || s.Minute() != 0 {
			t.Errorf("%s: starts %v, want %s 00:00", tc.zone, s, tc.start)
		}
		if len(got) != Days || got[Days-1] != tc.last || got[Days-2] != tc.day {
			t.Errorf("%s: %v, want ... %d %d", tc.zone, got, tc.day, tc.last)
		}
	}
	// A day across the change is 25 hours long.
	ny := zone("America/New_York")
	days := localDays(time.Date(2026, 11, 2, 12, 0, 0, 0, ny).Unix(), ny)
	if d := days[Days-1] - days[Days-2]; d != 25*3600 {
		t.Errorf("1 November in New York: %d s", d)
	}
}

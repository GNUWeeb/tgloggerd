// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"testing"
	"time"
)

// The dashboard's today (its KPIs) is a calendar day in the viewer's
// timezone: midnight there, not UTC's, and a day across a daylight
// saving change is 23 or 25 hours long.
func TestLocalDays(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		zone  string
		now   time.Time
		today string // the start of the last day, in the zone
	}{
		{"Asia/Jakarta", time.Date(2026, 11, 17, 17, 0, 0, 0, time.UTC), "2026-11-18 00:00"},
		{"Asia/Jakarta", time.Date(2026, 11, 17, 16, 59, 0, 0, time.UTC), "2026-11-17 00:00"},
		{"Asia/Kathmandu", time.Date(2026, 11, 17, 18, 15, 0, 0, time.UTC), "2026-11-18 00:00"},
		{"UTC", time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), "2026-11-18 00:00"},
	} {
		loc, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Fatal(err)
		}
		days := localDays(tc.now.Unix(), loc)
		got := time.Unix(days[Days-1], 0).In(loc).Format("2006-01-02 15:04")
		if len(days) != Days+1 || got != tc.today {
			t.Errorf("%s at %v: today starts %s, want %s", tc.zone, tc.now, got, tc.today)
		}
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	days := localDays(time.Date(2026, 11, 2, 12, 0, 0, 0, ny).Unix(), ny)
	if d := days[Days-1] - days[Days-2]; d != 25*3600 {
		t.Errorf("1 November in New York: %d s", d)
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"

	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/testdb"
)

// The dashboard chart's preferences: the viewer carries the choice in
// effect, the app's three kinds of save (a range ending now, a fixed
// range, the variant alone) are stored, and a range that does not
// hold together or exceeds its period's limit is refused, checked
// over the stored keys for a PATCH.
func TestDashboardPrefs(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts").DB
	h := newHandler(db, 5)
	c := &client{t: t, h: h, cookies: map[string]string{}}
	tok := signIn(t, c, "gw_admin", "password")
	patch := func(body map[string]any) (int, map[string]any) {
		t.Helper()
		a := c.do("PATCH", "/api/account/preferences", tok, body)
		return a.status, a.body
	}

	v := viewerOf(t, c)
	if v["dashboard_period"] != "hour" || v["dashboard_variant"] != "bars" ||
		v["dashboard_last"] != 172800.0 || v["dashboard_start"] != nil || v["dashboard_end"] != nil {
		t.Fatalf("default viewer %v", v)
	}
	// A range ending now.
	if st, b := patch(map[string]any{"dashboard_period": "day", "dashboard_last": 30 * 86400,
		"dashboard_start": nil, "dashboard_end": nil}); st != 200 {
		t.Fatalf("now range: %d %v", st, b)
	}
	v = viewerOf(t, c)
	if v["dashboard_period"] != "day" || v["dashboard_last"] != float64(30*86400) || v["dashboard_start"] != nil {
		t.Fatalf("now range viewer %v", v)
	}
	// The variant alone keeps the range.
	if st, b := patch(map[string]any{"dashboard_variant": "total"}); st != 200 {
		t.Fatalf("variant: %d %v", st, b)
	}
	v = viewerOf(t, c)
	if v["dashboard_variant"] != "total" || v["dashboard_period"] != "day" {
		t.Fatalf("variant viewer %v", v)
	}
	// A fixed range.
	if st, b := patch(map[string]any{"dashboard_period": "month", "dashboard_start": 0,
		"dashboard_end": 1794931200, "dashboard_last": nil}); st != 200 {
		t.Fatalf("fixed range: %d %v", st, b)
	}
	v = viewerOf(t, c)
	if v["dashboard_start"] != 0.0 || v["dashboard_end"] != 1794931200.0 || v["dashboard_last"] != nil {
		t.Fatalf("fixed viewer %v", v)
	}

	// Refusals, each naming its key; the stored keys fill in for the
	// ones left out: a period of minute under the stored fixed range
	// of all time is over its limit.
	for _, tc := range []struct {
		body  map[string]any
		field string
		limit bool
	}{
		{map[string]any{"dashboard_period": "minute"}, "dashboard_start", true},
		{map[string]any{"dashboard_period": "hour", "dashboard_last": 46 * 86400,
			"dashboard_start": nil, "dashboard_end": nil}, "dashboard_last", true},
		{map[string]any{"dashboard_end": nil}, "dashboard_end", false},
		{map[string]any{"dashboard_start": nil}, "dashboard_start", false},
		{map[string]any{"dashboard_start": 1794931200}, "dashboard_end", false},
		{map[string]any{"dashboard_period": "fortnight"}, "dashboard_period", false},
		{map[string]any{"dashboard_variant": "pie"}, "dashboard_variant", false},
		{map[string]any{"dashboard_last": 0}, "dashboard_last", false},
		{map[string]any{"dashboard_start": -1}, "dashboard_start", false},
	} {
		st, b := patch(tc.body)
		msg, _ := b["message"].(string)
		if st != 400 || b["error"] != "bad_request" || b["field"] != tc.field ||
			tc.limit != strings.Contains(msg, "at most") {
			t.Errorf("%v: %d %v", tc.body, st, b)
		}
	}
	// PUT checks the body as a whole.
	if a := c.do("PUT", "/api/account/preferences", tok, map[string]any{"dashboard_period": "minute",
		"dashboard_last": 86401}); a.status != 400 || a.body["field"] != "dashboard_last" {
		t.Errorf("put over the limit: %d %v", a.status, a.body)
	}
	if a := c.do("PUT", "/api/account/preferences", tok, map[string]any{"dashboard_last": 3600}); a.status != 200 {
		t.Errorf("put: %d %v", a.status, a.body)
	}
	v = viewerOf(t, c)
	if v["dashboard_period"] != "hour" || v["dashboard_last"] != 3600.0 || v["dashboard_variant"] != "bars" {
		t.Errorf("after put %v", v)
	}
}

// A stored range that no longer fits gives way to one ending now, cut
// to the period's limit.
func TestDashChoiceOf(t *testing.T) {
	t.Parallel()
	s := func(v string) *string { return &v }
	n := func(v int64) *int64 { return &v }
	for _, tc := range []struct {
		p                settings.Prefs
		period           string
		start, end, last int64 // 0 for absent
	}{
		{settings.Prefs{}, "hour", 0, 0, 172800},
		{settings.Prefs{DashPeriod: s("minute")}, "minute", 0, 0, 86400},
		{settings.Prefs{DashPeriod: s("day"), DashStart: n(100), DashEnd: n(200)}, "day", 100, 200, 0},
		{settings.Prefs{DashPeriod: s("minute"), DashStart: n(100), DashEnd: n(100 + 86401)},
			"minute", 0, 0, 86400},
		{settings.Prefs{DashStart: n(200), DashEnd: n(100), DashLast: n(3600)}, "hour", 0, 0, 3600},
		{settings.Prefs{DashPeriod: s("year"), DashLast: n(1 << 40)}, "year", 0, 0, 1 << 40},
	} {
		c := dashChoiceOf(tc.p)
		val := func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		}
		if string(c.Period) != tc.period || val(c.Start) != tc.start || val(c.End) != tc.end ||
			val(c.Last) != tc.last {
			t.Errorf("%+v: %s %d %d %d", tc.p, c.Period, val(c.Start), val(c.End), val(c.Last))
		}
	}
}

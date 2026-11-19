// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// GET /api/dashboard/series counts as the dashboard's per-day bars
// count, for each role: its day series over the bars' 30 days is the
// bars. It refuses an unknown period, a bad range and one over the
// limit, and needs a session.
func TestDashboardSeries(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	// The sample's messages are from the hours before this.
	now := time.Unix(1794931200+5*3600, 0)
	e.with(func(d *Deps) { d.clock = func() time.Time { return now } })
	for _, who := range []string{"superadmin", "admin", "member"} {
		_, p := e.page(who, "/")
		props := p["props"].(map[string]any)
		// The bars' first day, a date in the viewer's zone.
		_, b0, _ := e.call(who, "GET", fmt.Sprintf("/api/dashboard/series?period=day&start=%d&end=now",
			now.Unix()-60), nil)
		loc, err := time.LoadLocation(b0["tz"].(string))
		if err != nil {
			t.Fatal(err)
		}
		day, err := time.ParseInLocation(time.DateOnly, props["per_day_start"].(string), loc)
		if err != nil {
			t.Fatal(err)
		}
		start := day.Unix()
		st, b, h := e.call(who, "GET", fmt.Sprintf("/api/dashboard/series?period=day&start=%d&end=now",
			start), nil)
		if st != 200 || h.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %v", who, st, b)
		}
		var ns []any
		for _, pt := range b["points"].([]any) {
			ns = append(ns, pt.(map[string]any)["n"])
		}
		if js(ns) != js(props["per_day"]) {
			t.Errorf("%s: %s, want the bars %s", who, js(ns), js(props["per_day"]))
		}
		var sum float64
		for _, n := range ns {
			sum += n.(float64)
		}
		if who == "superadmin" && sum == 0 {
			t.Errorf("superadmin: no messages counted")
		}
		if b["live"] != true || b["period"] != "day" || b["limit"] != float64(1096*86400) ||
			b["start"] != float64(start) || b["tz"] == "" {
			t.Errorf("%s: %v", who, b)
		}
	}
	st, b, _ := e.call("superadmin", "GET", "/api/dashboard/series?period=year&start=0&end=now", nil)
	if st != 200 || b["limit"] != nil || b["live"] != true {
		t.Errorf("year: %d %v", st, b)
	}
	st, b, _ = e.call("superadmin", "GET", fmt.Sprintf("/api/dashboard/series?period=hour&start=%d&end=%d",
		now.Unix()-7200, now.Unix()-3600), nil)
	if st != 200 || b["live"] != false || len(b["points"].([]any)) != 1 {
		t.Errorf("fixed hour: %d %v", st, b)
	}
	for _, q := range []string{"period=fortnight&start=0&end=now", "period=day&start=x&end=now",
		"period=day&start=10&end=10", "period=day&start=10&end=soon", "period=day&end=now",
		fmt.Sprintf("period=minute&start=%d&end=now", now.Unix()-86401)} {
		st, b, _ := e.call("superadmin", "GET", "/api/dashboard/series?"+q, nil)
		msg, _ := b["message"].(string)
		if st != 400 || b["error"] != "bad_request" || msg == "" {
			t.Errorf("%s: %d %v", q, st, b)
		}
		if strings.HasPrefix(q, "period=minute") && !strings.Contains(msg, "24 hours") {
			t.Errorf("over the limit: %q", msg)
		}
	}
	if st, _, _ := e.call("", "GET", "/api/dashboard/series?period=day&start=0&end=now", nil); st != 401 {
		t.Errorf("signed out: %d", st)
	}
}

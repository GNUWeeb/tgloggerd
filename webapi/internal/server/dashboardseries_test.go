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

// GET /api/dashboard/series counts as the dashboard counts, for each
// role: today's point of a day series is the sum of the cards' today
// counts of group and private messages, and the page opens with the
// series of the default choice. It refuses an unknown period, a bad
// range and one over the limit, and needs a session.
func TestDashboardSeries(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	// 23:59 in Jakarta: the sample's messages are from the hours before.
	now := time.Unix(1794931200-60, 0)
	e.with(func(d *Deps) { d.clock = func() time.Time { return now } })
	for _, who := range []string{"superadmin", "admin", "member"} {
		_, p := e.page(who, "/")
		props := p["props"].(map[string]any)
		var today float64
		for _, k := range props["kpis"].([]any) {
			k := k.(map[string]any)
			if k["key"] == "group_messages" || k["key"] == "private_messages" {
				today += k["today"].(float64)
			}
		}
		st, b, h := e.call(who, "GET", fmt.Sprintf("/api/dashboard/series?period=day&start=%d&end=now",
			now.Unix()-3*86400), nil)
		if st != 200 || h.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %v", who, st, b)
		}
		pts := b["points"].([]any)
		if n := pts[len(pts)-1].(map[string]any)["n"]; n != today {
			t.Errorf("%s: today %v, the cards %v", who, n, today)
		}
		if who == "superadmin" && today == 0 {
			t.Errorf("superadmin: no messages today")
		}
		if b["live"] != true || b["period"] != "day" || b["limit"] != float64(1096*86400) ||
			b["tz"] == "" || len(pts) != 4 {
			t.Errorf("%s: %v", who, b)
		}
		// The page's series is the default: per hour, the last 48 hours.
		_, d, _ := e.call(who, "GET", fmt.Sprintf("/api/dashboard/series?period=hour&start=%d&end=now",
			now.Unix()-48*3600), nil)
		if js(props["series"]) != js(d) {
			t.Errorf("%s: page series %s, want %s", who, js(props["series"]), js(d))
		}
	}
	st, b, _ := e.call("superadmin", "GET", "/api/dashboard/series?period=year&start=0&end=now", nil)
	if st != 200 || b["limit"] != nil || b["live"] != true {
		t.Errorf("year: %d %v", st, b)
	}
	st, b, _ = e.call("superadmin", "GET", fmt.Sprintf("/api/dashboard/series?period=hour&start=%d&end=%d",
		1794931200-7200, 1794931200-3600), nil)
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

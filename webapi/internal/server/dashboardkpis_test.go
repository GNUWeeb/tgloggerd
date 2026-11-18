// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import "testing"

// GET /api/dashboard/kpis answers the dashboard page's cards, the same
// for each role, and needs a session.
func TestDashboardKPIs(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, who := range []string{"superadmin", "admin", "member"} {
		_, p := e.page(who, "/")
		want := js(p["props"].(map[string]any)["kpis"])
		st, b, _ := e.call(who, "GET", "/api/dashboard/kpis", nil)
		if st != 200 || js(b["kpis"]) != want {
			t.Errorf("%s: %d %s, want the page's %s", who, st, js(b["kpis"]), want)
		}
	}
	if st, _, _ := e.call("", "GET", "/api/dashboard/kpis", nil); st != 401 {
		t.Errorf("signed out: %d", st)
	}
}

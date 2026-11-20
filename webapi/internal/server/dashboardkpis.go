// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/http"

	"tgloggerd/webapi/internal/logdb"
)

// dashboardKPIs answers GET /api/dashboard/kpis: the dashboard's cards
// alone, as its page data has them, for a page that keeps them current
// without loading the rest of the dashboard again. Its counts come
// from the count cache, so asking more often than its TTL changes
// nothing.
func (s *Server) dashboardKPIs(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	sess, _ := SessionFrom(r.Context())
	loc, err := s.locationOf(r.Context(), sess.UID)
	if err != nil {
		s.internalError(w, "dashboard kpis", err)
		return
	}
	ks, err := logdb.LoadKPIs(r.Context(), s.ro, sc, s.clock().Unix(), loc)
	if err != nil {
		s.internalError(w, "dashboard kpis", err)
		return
	}
	out := []kpiJSON{}
	for _, k := range ks {
		out = append(out, kpiJSON(k))
	}
	writeJSON(w, http.StatusOK, map[string]any{"kpis": out})
}

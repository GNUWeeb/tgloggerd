// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"net/http"
	"strconv"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
)

// dashboardSeries answers GET /api/dashboard/series: the messages per
// period over a range, in the viewer's timezone and scope
// (logdb.LoadSeries).
func (s *Server) dashboardSeries(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	sess, _ := SessionFrom(r.Context())
	lang := s.textLang(r.Context(), sess.UID)
	q := r.URL.Query()
	p, ok := logdb.ParsePeriod(q.Get("period"))
	if !ok {
		badRequest(w, "bad_request", i18n.T(lang, "dashboard.bad_period"))
		return
	}
	start, err := strconv.ParseInt(q.Get("start"), 10, 64)
	live := q.Get("end") == "now"
	var end int64
	if err == nil && !live {
		end, err = strconv.ParseInt(q.Get("end"), 10, 64)
	}
	if err != nil {
		badRequest(w, "bad_request", i18n.T(lang, "dashboard.bad_range"))
		return
	}
	loc, err := s.locationOf(r.Context(), sess.UID)
	if err != nil {
		s.internalError(w, "dashboard series", err)
		return
	}
	se, err := logdb.LoadSeries(r.Context(), s.ro, s.series, sc, p, start, end, live,
		s.clock().Unix(), loc)
	switch {
	case errors.Is(err, logdb.ErrSeriesRange):
		badRequest(w, "bad_request", i18n.T(lang, "dashboard.bad_range"))
		return
	case errors.Is(err, logdb.ErrSeriesTooLong):
		badRequest(w, "bad_request", i18n.T(lang, "dashboard.range_too_long",
			"limit", rangeText(lang, p.Limit())))
		return
	case err != nil:
		s.internalError(w, "dashboard series", err)
		return
	}
	var limit *int64
	if l := p.Limit(); l > 0 {
		limit = &l
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"period": se.Period, "start": se.Start, "end": se.End, "live": live,
		"tz": loc.String(), "limit": limit, "points": se.Points,
	})
}

// rangeText is a limit for the refusal's sentence: hours under two
// days, days above.
func rangeText(lang string, secs int64) string {
	if secs < 2*86400 {
		return i18n.T(lang, "dashboard.limit_hours", "n", strconv.FormatInt(secs/3600, 10))
	}
	return i18n.T(lang, "dashboard.limit_days", "n", strconv.FormatInt(secs/86400, 10))
}

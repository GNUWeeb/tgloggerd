// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
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
	writeJSON(w, http.StatusOK, newSeriesJSON(se, live, loc))
}

// seriesJSON is the DashboardSeries schema.
type seriesJSON struct {
	Period logdb.Period        `json:"period"`
	Start  int64               `json:"start"`
	End    int64               `json:"end"`
	Live   bool                `json:"live"`
	TZ     string              `json:"tz"`
	Limit  *int64              `json:"limit"`
	Points []logdb.SeriesPoint `json:"points"`
}

func newSeriesJSON(se logdb.Series, live bool, loc *time.Location) seriesJSON {
	out := seriesJSON{Period: se.Period, Start: se.Start, End: se.End, Live: live,
		TZ: loc.String(), Points: se.Points}
	if l := se.Period.Limit(); l > 0 {
		out.Limit = &l
	}
	return out
}

// dashboardFirstSeries is the series the dashboard page opens with:
// the one the viewer's preferences ask for (dashChoiceOf).
func (s *Server) dashboardFirstSeries(ctx context.Context, uid uint64, sc scope.Scope,
	now int64, loc *time.Location) (seriesJSON, error) {
	var prefs settings.Prefs
	if uid != 0 {
		var err error
		if prefs, err = s.viewerPrefs(ctx, uid); err != nil {
			return seriesJSON{}, err
		}
	}
	c := dashChoiceOf(prefs)
	var se logdb.Series
	var err error
	if c.Last != nil {
		se, err = logdb.LoadSeries(ctx, s.ro, s.series, sc, c.Period, max(now-*c.Last, 0), 0, true, now, loc)
	} else {
		se, err = logdb.LoadSeries(ctx, s.ro, s.series, sc, c.Period, *c.Start, *c.End, false, now, loc)
	}
	if err != nil {
		return seriesJSON{}, err
	}
	return newSeriesJSON(se, c.Last != nil, loc), nil
}

// rangeText is a limit for the refusal's sentence: hours under two
// days, days above.
func rangeText(lang string, secs int64) string {
	if secs < 2*86400 {
		return i18n.T(lang, "dashboard.limit_hours", "n", strconv.FormatInt(secs/3600, 10))
	}
	return i18n.T(lang, "dashboard.limit_days", "n", strconv.FormatInt(secs/86400, 10))
}

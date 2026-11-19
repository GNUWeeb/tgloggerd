// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/http"
	"slices"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/settings"
)

// The dashboard chart's preferences: its period, and its range, fixed
// (dashboard_start and dashboard_end) or ending now (dashboard_last).

// DefaultDashLast is the length of a range that ends now when the
// account has not chosen one: 48 hours.
const DefaultDashLast = 48 * 3600

// dashChoice is the chart the viewer's preferences ask for: a fixed
// range (Start and End set) or one ending now (Last set).
type dashChoice struct {
	Period           logdb.Period
	Start, End, Last *int64
}

// dashChoiceOf is the chart in effect under p. A stored range that no
// longer fits (a period changed by a save racing another, a limit
// lowered since) gives way to the range ending now, its length cut to
// the period's limit, so the page always has a chart to draw.
func dashChoiceOf(p settings.Prefs) dashChoice {
	c := dashChoice{Period: logdb.Hour}
	if p.DashPeriod != nil {
		if per, ok := logdb.ParsePeriod(*p.DashPeriod); ok {
			c.Period = per
		}
	}
	limit := c.Period.Limit()
	if p.DashStart != nil && p.DashEnd != nil && *p.DashStart < *p.DashEnd &&
		(limit == 0 || *p.DashEnd-*p.DashStart <= limit) {
		c.Start, c.End = p.DashStart, p.DashEnd
		return c
	}
	last := int64(DefaultDashLast)
	if p.DashLast != nil {
		last = *p.DashLast
	}
	if limit > 0 && last > limit {
		last = limit
	}
	c.Last = &last
	return c
}

// dashRangeKeys are the keys checked together by checkDashRange.
var dashRangeKeys = []string{"dashboard_period", "dashboard_start", "dashboard_end", "dashboard_last"}

// checkDashRange refuses a dashboard range that does not hold together
// (each key alone already passed settings' checks): start and end set
// together, the start before the end, and the fixed range or the
// length of one ending now within the limit of the period in effect.
// It answers the key to name and whether the fault is the limit; ""
// when the range is fine.
func checkDashRange(p settings.Prefs) (key string, overLimit bool) {
	per := logdb.Hour
	if p.DashPeriod != nil {
		per, _ = logdb.ParsePeriod(*p.DashPeriod)
	}
	limit := per.Limit()
	switch {
	case p.DashStart == nil && p.DashEnd != nil:
		return "dashboard_start", false
	case p.DashStart != nil && p.DashEnd == nil:
		return "dashboard_end", false
	case p.DashStart != nil && *p.DashStart >= *p.DashEnd:
		return "dashboard_end", false
	case p.DashStart != nil && limit > 0 && *p.DashEnd-*p.DashStart > limit:
		return "dashboard_start", true
	case p.DashLast != nil && limit > 0 && *p.DashLast > limit:
		return "dashboard_last", true
	}
	return "", false
}

// refuseDashRange answers 400 for checkDashRange's fault, if there is
// one, and reports whether it did.
func refuseDashRange(w http.ResponseWriter, lang string, p settings.Prefs) bool {
	key, overLimit := checkDashRange(p)
	switch {
	case key == "":
		return false
	case overLimit:
		per := logdb.Hour
		if p.DashPeriod != nil {
			per, _ = logdb.ParsePeriod(*p.DashPeriod)
		}
		writeError(w, http.StatusBadRequest, "bad_request", map[string]any{
			"message": i18n.T(lang, "dashboard.range_too_long", "limit", rangeText(lang, per.Limit())),
			"field":   key,
		})
	default:
		badField(w, lang, "prefs.invalid", key)
	}
	return true
}

// namesDashRange is whether a PATCH body names a key of the range.
func namesDashRange(keys []string) bool {
	return slices.ContainsFunc(keys, func(k string) bool { return slices.Contains(dashRangeKeys, k) })
}

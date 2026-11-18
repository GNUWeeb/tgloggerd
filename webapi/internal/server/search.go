// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The advanced search API, GET /api/search/{entity}, the port of the
// Drogon web's GET /v1/search/{entity} (web/docs/search-api.md): the
// same parameters and the same answer, read through the request scope.
// It lives under /api/ with the rest of this service's API; the Drogon
// pages keep calling their own /v1/search until they move here.
//
// And the chat search of the pickers, GET /api/chats/search.

// photoURL is a file's /files/<token> URL.
func (s *Server) photoURL(id uint64) string {
	return "/files/" + s.tokens.Encrypt(id)
}

// atoi reads a number as C's atoi does, which the Drogon web's
// clampedIntParam uses: leading blanks, a sign, the digits up to the
// first other byte; nothing readable is 0.
func atoi(s string) int {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	n := 0
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		if n < math.MaxInt32 {
			n = n*10 + int(s[i]-'0')
		}
	}
	n = min(n, math.MaxInt32)
	if neg {
		return -n
	}
	return n
}

// clampedParam is a numeric query parameter as Drogon reads it: the
// default when absent or empty, else atoi's value clamped to lo..hi.
func clampedParam(r *http.Request, name string, def, lo, hi int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	return min(max(atoi(v), lo), hi)
}

// mayDebug is whether the viewer may ask for a search's SQL and plan:
// the roles of Drogon's admin, who see every row anyway.
func mayDebug(sc scope.Scope) bool {
	return sc.Role() == scope.Superadmin || sc.Role() == scope.Admin
}

// searchAnswer is the API's answer (SearchAnswer).
type searchAnswer struct {
	Fields     []logdb.FieldInfo  `json:"fields"`
	Cols       []logdb.Col        `json:"cols"`
	Rows       [][]any            `json:"rows"`
	Total      int64              `json:"total"`
	TotalExact bool               `json:"total_exact"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	MaxOffset  int                `json:"max_offset"`
	Sort       string             `json:"sort"`
	Order      string             `json:"order"`
	Debug      *logdb.SearchDebug `json:"debug,omitempty"`
	// The search engine's part (searchengine.go): present only with
	// an engine configured.
	Backend    *logdb.Backend `json:"backend,omitempty"`
	Highlights [][]string     `json:"highlights,omitempty"`
	Facets     []logdb.Facet  `json:"facets,omitempty"`
}

// searchFailed answers a search's error: hidden or unknown entities
// 404, refused requests and searches that ran out of time 400 with
// Drogon's message.
func (s *Server) searchFailed(w http.ResponseWriter, what string, err error) {
	var se *logdb.SearchError
	switch {
	case errors.Is(err, logdb.ErrNotFound):
		notFound(w)
	case errors.As(err, &se):
		badRequest(w, "bad_search", se.Msg)
	case errors.Is(err, logdb.ErrSearchTimeout):
		badRequest(w, "search_timeout", err.Error())
	default:
		s.internalError(w, what, err)
	}
}

// searchEntity is GET /api/search/{entity}.
func (s *Server) searchEntity(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	e := logdb.Entities[r.PathValue("entity")]
	if e == nil {
		notFound(w)
		return
	}
	q := r.URL.Query()
	req := logdb.SearchRequest{
		Limit:  clampedParam(r, "limit", 10, 1, logdb.MaxLimit),
		Offset: clampedParam(r, "offset", 0, 0, logdb.MaxOffset),
		Sort:   q.Get("sort"),
		Order:  q.Get("order"),
		Debug:  q.Get("debug") == "1" && mayDebug(sc),
	}
	conds, err := logdb.ParseConditions(q.Get("search"))
	if err != nil {
		s.searchFailed(w, "search", err)
		return
	}
	req.Conds = conds
	s.withEngine(r, &req)
	res, err := logdb.Search(r.Context(), s.ro, sc, e, req)
	if err != nil {
		s.searchFailed(w, "search "+e.Name, err)
		return
	}
	writeJSON(w, http.StatusOK, searchAnswer{
		Fields: res.Fields, Cols: res.Cols, Rows: res.Positional(s.photoURL),
		Total: res.Total, TotalExact: res.TotalExact, Limit: res.Limit,
		Offset: res.Offset, MaxOffset: logdb.MaxOffset, Sort: res.Sort,
		Order: res.Order, Debug: res.Debug,
		Backend: res.Backend, Highlights: res.Highlights, Facets: res.Facets,
	})
}

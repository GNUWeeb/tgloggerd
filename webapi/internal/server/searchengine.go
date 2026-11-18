// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/http"
	"slices"

	"tgloggerd/webapi/internal/logdb"
)

// The optional search engine's part of the search API
// (web/docs/search-api.md, "Search engine"): the options a search
// takes for its full-text conditions, and GET /api/search/capabilities,
// which tells a client whether to offer them. Without an engine the
// options are ignored, the answers carry nothing new, and the
// capabilities name no engine.

// SearchEngine is a search engine for the message searches
// (internal/searchengine.Engine).
type SearchEngine interface {
	logdb.Engine
	// Name is the engine's kind, as the capabilities name it.
	Name() string
}

// engineOptions are the options the engine offers, in the order a
// client lists them.
var engineOptions = []string{"phrase", "proximity", "prefix", "fuzzy", "highlight", "facets"}

// withEngine gives a search the engine and the request's options for
// it: fuzzy=1, highlight=1, facets=1, and engine=mysql to keep it on
// MySQL.
func (s *Server) withEngine(r *http.Request, req *logdb.SearchRequest) {
	if s.engine == nil {
		return
	}
	q := r.URL.Query()
	req.Engine = s.engine
	req.Text = logdb.TextOptions{
		Fuzzy:     q.Get("fuzzy") == "1",
		Highlight: q.Get("highlight") == "1",
		Facets:    q.Get("facets") == "1",
		MySQL:     q.Get("engine") == "mysql",
	}
}

// engineCaps is what the engine offers for one entity.
type engineCaps struct {
	// Fields are the full-text fields it answers.
	Fields []string `json:"fields"`
	// Options are the options it offers for them.
	Options []string `json:"options"`
	// Available is whether it answers now; otherwise Fallback says why
	// searches go to MySQL (a logdb.Fallback* reason).
	Available bool    `json:"available"`
	Fallback  *string `json:"fallback"`
}

// searchCaps is GET /api/search/capabilities's answer.
type searchCaps struct {
	// Engine is the engine's kind, null without one.
	Engine *string `json:"engine"`
	// Entities are the entities the engine answers that the viewer
	// may search, by name; empty without an engine.
	Entities map[string]engineCaps `json:"entities"`
}

// searchCapabilities is GET /api/search/capabilities.
func (s *Server) searchCapabilities(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	ans := searchCaps{Entities: map[string]engineCaps{}}
	if s.engine == nil {
		writeJSON(w, http.StatusOK, ans)
		return
	}
	name := s.engine.Name()
	ans.Engine = &name
	names := []string{}
	for n, e := range logdb.Entities {
		if logdb.EngineEntity(e) && sc.Page(n) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	for _, n := range names {
		e := logdb.Entities[n]
		c := engineCaps{Fields: []string{}, Options: engineOptions}
		for _, f := range e.FieldInfos(sc) {
			if slices.Contains(f.Operators, "matches") {
				c.Fields = append(c.Fields, f.Key)
			}
		}
		ok, why := s.engine.Ready(r.Context(), n)
		c.Available = ok
		if !ok {
			c.Fallback = &why
		}
		ans.Entities[n] = c
	}
	writeJSON(w, http.StatusOK, ans)
}

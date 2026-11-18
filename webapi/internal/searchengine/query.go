// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The engine's queries: a full-text value read in the syntax of
// MySQL's boolean mode, so a search means the same on both backends,
// and the request scope as a filter of every query.
//
// The syntax (web/docs/search-api.md, "Search engine"):
//
//	word        a row with the word ranks higher; with no +word, a row
//	            needs at least one of these
//	+word       a row must have the word
//	-word       a row must not have the word
//	word*       a word starting so
//	"a phrase"  the words in this order, next to each other
//	"a b" @3    the words within 3 positions of each other (proximity)
//	word~       the word or one within a few typos (fuzzy; word~1,
//	            word~2 for that many edits); the fuzzy option does it
//	            for every plain word
//
// +, - and the phrase forms combine: +"a b" -c. MySQL's other
// operators (~ < > before a word, parentheses) are read as spaces. A
// value with no word to look for matches nothing, as in MySQL.

// clause is one term of a full-text value.
type clause struct {
	occur  byte // '+' must, '-' must not, 0 should
	phrase bool
	text   string
	slop   int    // a phrase's proximity
	prefix bool   // word*
	fuzzy  string // "" none, "AUTO", "1", "2"
}

func isSpace(r rune) bool {
	return unicode.IsSpace(r) || r == '(' || r == ')'
}

// parseText reads a full-text value into its clauses.
func parseText(v string) []clause {
	rs := []rune(v)
	var out []clause
	for i := 0; i < len(rs); {
		r := rs[i]
		if isSpace(r) {
			i++
			continue
		}
		var c clause
		// Operators before a word: + and -; MySQL's ~ < > are read
		// as no operator.
		for i < len(rs) && strings.ContainsRune("+-~<>", rs[i]) {
			if rs[i] == '+' || rs[i] == '-' {
				c.occur = byte(rs[i])
			}
			i++
		}
		if i >= len(rs) {
			break
		}
		if rs[i] == '"' {
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			c.phrase, c.text = true, string(rs[i+1:min(j, len(rs))])
			i = min(j+1, len(rs))
			// "a b" @N: the proximity.
			k := i
			for k < len(rs) && unicode.IsSpace(rs[k]) {
				k++
			}
			if k < len(rs) && rs[k] == '@' {
				d := k + 1
				for d < len(rs) && rs[d] >= '0' && rs[d] <= '9' {
					d++
				}
				if n, err := strconv.Atoi(string(rs[k+1 : d])); err == nil {
					c.slop, i = min(n, 100), d
				}
			}
			if strings.TrimSpace(c.text) != "" {
				out = append(out, c)
			}
			continue
		}
		if isSpace(rs[i]) || rs[i] == '@' {
			i++
			continue
		}
		j := i
		for j < len(rs) && !isSpace(rs[j]) && rs[j] != '"' {
			j++
		}
		w := string(rs[i:j])
		i = j
		if t, n, ok := strings.Cut(w, "~"); ok && t != "" {
			switch n {
			case "":
				c.fuzzy = "AUTO"
			case "0", "1", "2":
				c.fuzzy = n
			default:
				c.fuzzy = "AUTO"
			}
			w = t
		}
		if strings.HasSuffix(w, "*") {
			c.prefix, w = true, strings.TrimRight(w, "*")
		}
		if w == "" {
			continue
		}
		c.text = w
		out = append(out, c)
	}
	return out
}

// textQuery is a full-text value as an engine query; fuzzy matches
// every plain word with typos.
func textQuery(v string, fuzzy bool) map[string]any {
	var must, mustNot, should []any
	for _, c := range parseText(v) {
		var q map[string]any
		switch {
		case c.phrase:
			q = map[string]any{"match_phrase": map[string]any{"text": map[string]any{
				"query": c.text, "slop": c.slop}}}
		case c.prefix:
			q = map[string]any{"match_bool_prefix": map[string]any{"text": map[string]any{
				"query": c.text, "operator": "and"}}}
		default:
			m := map[string]any{"query": c.text, "operator": "and"}
			switch {
			case c.fuzzy != "":
				m["fuzziness"] = c.fuzzy
			case fuzzy:
				m["fuzziness"] = "AUTO"
			}
			q = map[string]any{"match": map[string]any{"text": m}}
		}
		switch c.occur {
		case '+':
			must = append(must, q)
		case '-':
			mustNot = append(mustNot, q)
		default:
			should = append(should, q)
		}
	}
	if len(must) == 0 && len(should) == 0 {
		return map[string]any{"match_none": map[string]any{}}
	}
	b := map[string]any{}
	if len(must) > 0 {
		b["must"] = must
	}
	if len(should) > 0 {
		b["should"] = should
		if len(must) == 0 {
			b["minimum_should_match"] = 1
		}
	}
	if len(mustNot) > 0 {
		b["must_not"] = mustNot
	}
	return map[string]any{"bool": b}
}

var matchNone = map[string]any{"match_none": map[string]any{}}

func terms(field string, ids []int64) map[string]any {
	return map[string]any{"terms": map[string]any{field: ids}}
}

func term(field string, v any) map[string]any {
	return map[string]any{"term": map[string]any{field: v}}
}

// scopeOf is the document scope of an entity the engine answers.
func scopeOf(entity string) (string, bool) {
	switch entity {
	case logdb.GroupMessages.Name:
		return ScopeGroup, true
	case logdb.PrivateMessages.Name:
		return ScopePrivate, true
	}
	return "", false
}

// scopeFilter is the request scope as the query's filter and must_not
// clauses (web-accounts.md section 9), the engine's GroupFilter: an
// entity's own documents only; for group messages every group, all but
// an admin's hidden ones, or a member's granted ones; for private
// messages all of them to a scope that sees private chats, none
// otherwise. It is part of every query, so hits, totals, facets and
// highlights count only what the viewer may see.
func scopeFilter(entity string, sc scope.Scope) (filter, mustNot []any, err error) {
	sp, ok := scopeOf(entity)
	if !ok {
		return nil, nil, fmt.Errorf("search engine: no entity %q", entity)
	}
	filter = []any{term("scope", sp)}
	switch {
	case sp == ScopePrivate && !sc.SeesPrivate():
		filter = append(filter, matchNone)
	case sp == ScopePrivate, sc.SeesAllGroups():
	case len(sc.HiddenGroups()) > 0:
		mustNot = append(mustNot, terms("chat_id", sc.HiddenGroups()))
	case len(sc.GrantedGroups()) > 0:
		filter = append(filter, terms("chat_id", sc.GrantedGroups()))
	default:
		filter = append(filter, matchNone)
	}
	return filter, mustNot, nil
}

// The document fields a column filter may name, by kind.
var (
	numberFields = map[string]bool{"chat_id": true, "sender_id": true, "date": true}
	boolFields   = map[string]bool{"outgoing": true}
)

// columnFilter is a column condition as a filter clause, with MySQL's
// meaning: a missing value (a NULL sender) matches no comparison, !=
// included.
func columnFilter(f logdb.EngineFilter) (map[string]any, error) {
	var v any
	switch {
	case f.Field == "deleted":
		switch f.Op {
		case logdb.OpIsNull:
			return term("deleted", false), nil
		case logdb.OpIsNotNull:
			return term("deleted", true), nil
		}
		return nil, fmt.Errorf("search engine: deleted takes IS [NOT] NULL only")
	case numberFields[f.Field]:
		n, err := strconv.ParseInt(f.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("search engine: %s: %q is not a number", f.Field, f.Value)
		}
		v = n
	case boolFields[f.Field]:
		n, err := strconv.ParseInt(f.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("search engine: %s: %q is not a number", f.Field, f.Value)
		}
		v = n != 0
	case f.Field == "content_type":
		v = f.Value
	default:
		return nil, fmt.Errorf("search engine: no field %q", f.Field)
	}
	rng := func(op string) map[string]any {
		return map[string]any{"range": map[string]any{f.Field: map[string]any{op: v}}}
	}
	switch f.Op {
	case logdb.OpEQ:
		return term(f.Field, v), nil
	case logdb.OpNE:
		return map[string]any{"bool": map[string]any{
			"filter":   []any{map[string]any{"exists": map[string]any{"field": f.Field}}},
			"must_not": []any{term(f.Field, v)},
		}}, nil
	case logdb.OpLT:
		return rng("lt"), nil
	case logdb.OpGT:
		return rng("gt"), nil
	case logdb.OpLE:
		return rng("lte"), nil
	case logdb.OpGE:
		return rng("gte"), nil
	}
	return nil, fmt.Errorf("search engine: %s: operator not taken", f.Field)
}

// facetSize is how many chats a facet answer lists, busiest first.
const facetSize = 10

// searchBody is q as the engine's search request.
func searchBody(q logdb.EngineQuery) (map[string]any, error) {
	filter, mustNot, err := scopeFilter(q.Entity, q.Scope)
	if err != nil {
		return nil, err
	}
	for _, f := range q.Filters {
		c, err := columnFilter(f)
		if err != nil {
			return nil, err
		}
		filter = append(filter, c)
	}
	if q.IDs != nil {
		filter = append(filter, terms("row_id", q.IDs))
	}
	var must []any
	for _, v := range q.Match {
		must = append(must, textQuery(v, q.Text.Fuzzy))
	}
	b := map[string]any{"filter": filter}
	if len(must) > 0 {
		b["must"] = must
	}
	if len(mustNot) > 0 {
		b["must_not"] = mustNot
	}
	order := "asc"
	if q.Desc {
		order = "desc"
	}
	sort := []any{map[string]any{"_score": "desc"}}
	if q.Sort != "" {
		if !numberFields[q.Sort] {
			return nil, fmt.Errorf("search engine: cannot sort by %q", q.Sort)
		}
		sort = []any{map[string]any{q.Sort: order}}
	}
	// Ties as MySQL breaks them: the newest row first.
	sort = append(sort, map[string]any{"row_id": "desc"})
	body := map[string]any{
		"query":            map[string]any{"bool": b},
		"from":             max(q.From, 0),
		"size":             max(q.Size, 0),
		"sort":             sort,
		"track_total_hits": true,
		"_source":          false,
	}
	if q.Highlight {
		body["highlight"] = map[string]any{
			"encoder":   "html",
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
			"fields": map[string]any{"text": map[string]any{
				"number_of_fragments": 3, "fragment_size": 150,
			}},
		}
	}
	if q.Facets {
		body["aggs"] = map[string]any{"chats": map[string]any{
			"terms": map[string]any{"field": "chat_id", "size": facetSize},
		}}
	}
	return body, nil
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/scope"
)

// The search engine's part of a search (webapi/README "Search engine"):
// the full-text conditions of the message entities answered by an
// optional engine (internal/searchengine) instead of MySQL's FULLTEXT
// indexes. The engine answers which rows match, always within the
// request scope, which is a filter of its query; MySQL reads those rows
// as before, with every other condition and the scope applied again, so
// the rows keep their shape and a stale document can only drop a row,
// never add one the viewer may not see.
//
// Two ways, by what the search holds:
//
//   - Whole in the engine: every condition is a full-text one or a
//     column the documents carry (engineFields), ANDed, and the sort is
//     relevance or one the documents carry. The engine pages and counts;
//     MySQL reads the page's rows by id.
//   - Ids from the engine: anything else. Each full-text condition
//     becomes the list of the rows the engine finds for it, narrowed by
//     the columns of the search that every row must match; MySQL counts
//     and pages as usual. More than EngineMaxIDs rows for one condition
//     cannot be listed, and MySQL answers the search alone.
//
// Whatever the engine cannot answer now (not configured for the
// entity, down, slow, behind the archive, or asked not to) falls back to
// MySQL, and the answer says so (Backend).

// Engine is a search engine answering the full-text conditions of the
// entities in engineFields.
type Engine interface {
	// Ready is whether the engine answers searches of the entity now;
	// otherwise reason is the fallback's (FallbackUnavailable,
	// FallbackBehind).
	Ready(ctx context.Context, entity string) (ok bool, reason string)
	// Query answers q, within q.Scope.
	Query(ctx context.Context, q EngineQuery) (*EngineResult, error)
}

// TextOptions are a search's options for its full-text conditions.
type TextOptions struct {
	// Fuzzy matches every plain word with a few typos, as word~ does
	// for one word.
	Fuzzy bool
	// Highlight asks for the matching fragments of each row's text.
	Highlight bool
	// Facets asks for the hits per chat.
	Facets bool
	// MySQL keeps the search on MySQL even with an engine.
	MySQL bool
}

// The fallback reasons, Backend.Fallback.
const (
	FallbackAsked       = "asked"         // TextOptions.MySQL
	FallbackUnavailable = "unavailable"   // the engine fails or is down
	FallbackTimeout     = "timeout"       // the engine took too long
	FallbackBehind      = "behind"        // the index is behind the archive
	FallbackTooMany     = "too_many_hits" // a condition matches over EngineMaxIDs
)

// EngineMaxIDs is how many rows one full-text condition may match when
// MySQL counts and pages the search (the ids way).
const EngineMaxIDs = 10000

// Backend is which backend answered a search.
type Backend struct {
	// Used is "engine" or "mysql".
	Used string `json:"used"`
	// Fallback is why MySQL answered a search with full-text
	// conditions: one of the Fallback* reasons; "" when the engine
	// answered or there was nothing for it to answer.
	Fallback string `json:"fallback,omitempty"`
	// Ignored are the options asked for that the answer does not
	// give: fuzzy, highlight, facets.
	Ignored []string `json:"ignored,omitempty"`
}

// EngineFilter is a column condition the engine applies: a document
// field, an operator (OpEQ to OpGE, OpIsNull, OpIsNotNull) and the
// value as given.
type EngineFilter struct {
	Field string
	Op    Op
	Value string
}

// EngineQuery is one question to the engine.
type EngineQuery struct {
	Entity string
	Scope  scope.Scope
	// Match are full-text values, each in MySQL's boolean syntax (with
	// word~ for typos): a row must match every one.
	Match []string
	Text  TextOptions
	// Filters must all hold.
	Filters []EngineFilter
	// IDs, when set, limits the answer to these rows.
	IDs []int64
	// Sort is "" for relevance or a document field; Desc its order.
	Sort string
	Desc bool
	// From and Size are the page.
	From, Size int
	// Highlight and Facets ask for them with the page.
	Highlight, Facets bool
}

// EngineResult is the engine's answer: the page's row ids in order,
// the exact number of rows that match, and what was asked for.
type EngineResult struct {
	IDs        []int64
	Total      int64
	Highlights map[int64][]string
	Facets     []Facet
}

// Facet is the hits in one chat.
type Facet struct {
	ChatID int64  `json:"chat_id"`
	Name   string `json:"name"`
	Count  int64  `json:"count"`
}

// engineField is a column the documents carry: the document field and
// the operators the engine applies exactly as MySQL does.
type engineField struct {
	doc string
	ops Op
}

const engineIntOps = OpEQ | OpNE | OpLT | OpGT | OpLE | OpGE

// engineFields are the entities the engine knows and, by field key, the
// columns their documents carry; engineSorts the sorts they carry.
var (
	engineFields = map[*Entity]map[string]engineField{
		GroupMessages: {
			"chat_id":        {"chat_id", engineIntOps},
			"sender_user_id": {"sender_id", engineIntOps},
			"date":           {"date", engineIntOps},
			"content_type":   {"content_type", OpEQ | OpNE},
			"deleted":        {"deleted", OpIsNull | OpIsNotNull},
		},
		PrivateMessages: {
			"chat_id":      {"chat_id", engineIntOps},
			"sender_id":    {"sender_id", engineIntOps},
			"date":         {"date", engineIntOps},
			"content_type": {"content_type", OpEQ | OpNE},
			"deleted":      {"deleted", OpIsNull | OpIsNotNull},
			"is_outgoing":  {"outgoing", OpEQ | OpNE},
		},
	}
	engineSorts = map[string]string{"date": "date"}
)

// EngineEntity is whether the engine may answer the entity's full-text
// fields.
func EngineEntity(e *Entity) bool { return engineFields[e] != nil }

// enginePlan is how the engine can take part in a search.
type enginePlan struct {
	match   []*Cond        // the full-text conditions, in the copies of the chains
	filters []EngineFilter // the carried columns every row must match
	whole   bool           // the engine can answer the whole search
	chains  [][]Cond       // copies of the request's chains
}

// plan reads the search's conditions. ok is false for a request the
// SQL path refuses (an unknown field, a bad operator or value): the SQL
// path answers it, with its error.
func (e *Entity) plan(sc scope.Scope, req SearchRequest) (p enginePlan, ok bool) {
	fields := engineFields[e]
	p.whole = true
	for _, ch := range append([][]Cond{req.Conds}, req.And...) {
		cp := append([]Cond(nil), ch...)
		p.chains = append(p.chains, cp)
		allAnd := true
		for i := range cp[:max(len(cp)-1, 0)] {
			if cp[i].N != "AND" {
				allAnd = false
			}
		}
		for i := range cp {
			c := &cp[i]
			f := e.field(c.C, req.Internal)
			if f == nil || !e.fieldVisible(sc, f) {
				return p, false
			}
			op, _, known := opLookup(c.O)
			if !known || f.Ops&op == 0 || c.IsList != (op == OpIn) {
				return p, false
			}
			if op == OpMatch && !f.Exists {
				if !c.HasV || validValue(f, c.V) != nil {
					return p, false
				}
				p.match = append(p.match, c)
				continue
			}
			ef, carried := fields[c.C]
			if !carried || ef.ops&op == 0 || !engineValue(f, op, c) {
				p.whole = false
				continue
			}
			if allAnd {
				p.filters = append(p.filters, EngineFilter{Field: ef.doc, Op: op, Value: c.V})
			} else {
				p.whole = false
			}
		}
		if !allAnd {
			p.whole = false
		}
	}
	return p, true
}

// engineValue is whether a carried column's value is one the engine
// takes as MySQL does.
func engineValue(f *Field, op Op, c *Cond) bool {
	if op == OpIsNull || op == OpIsNotNull {
		return true
	}
	if !c.HasV || validValue(f, c.V) != nil {
		return false
	}
	if f.Type == Int || f.Type == Bool {
		_, err := strconv.ParseInt(c.V, 10, 64)
		return err == nil
	}
	return true
}

// inIDs is the condition that col is one of ids; ids are the
// engine's, parsed as numbers.
func inIDs(col string, ids []int64) string {
	if len(ids) == 0 {
		return "1 = 0"
	}
	var b strings.Builder
	b.WriteString(col + " IN (")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatInt(id, 10))
	}
	b.WriteByte(')')
	return b.String()
}

// rankExpr orders rows by their place in ids, best first, as DESC: the
// list is reversed so the best row has the highest FIELD(), and a row
// not in it (0) comes last.
func rankExpr(col string, ids []int64) string {
	var b strings.Builder
	b.WriteString("FIELD(" + col)
	for i := len(ids) - 1; i >= 0; i-- {
		b.WriteString(", " + strconv.FormatInt(ids[i], 10))
	}
	b.WriteByte(')')
	return b.String()
}

// fallbackReason is the reason of an engine error.
func fallbackReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return FallbackTimeout
	}
	return FallbackUnavailable
}

// searchEngine runs a search of an entity the engine knows.
func searchEngine(ctx context.Context, db *sql.DB, sc scope.Scope, e *Entity, req SearchRequest) (*SearchResult, error) {
	if !e.visible(sc) {
		return nil, ErrNotFound
	}
	p, ok := e.plan(sc, req)
	if !ok || len(p.match) == 0 {
		// A refused search, or nothing for the engine: MySQL as
		// before, and the answer says MySQL answered.
		res, err := searchSQL(ctx, db, sc, e, req)
		if err == nil {
			res.Backend = &Backend{Used: "mysql", Ignored: ignored(req.Text)}
		}
		return res, err
	}
	fallback := func(reason string) (*SearchResult, error) {
		res, err := searchSQL(ctx, db, sc, e, req)
		if err == nil {
			res.Backend = &Backend{Used: "mysql", Fallback: reason, Ignored: ignored(req.Text)}
		}
		return res, err
	}
	if req.Text.MySQL {
		return fallback(FallbackAsked)
	}
	if ok, why := req.Engine.Ready(ctx, e.Name); !ok {
		return fallback(why)
	}
	var (
		res *SearchResult
		err error
	)
	if p.whole {
		res, err = e.engineWhole(ctx, db, sc, req, p)
	} else {
		res, err = e.engineIDs(ctx, db, sc, req, p)
	}
	var fe fallbackError
	if errors.As(err, &fe) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return fallback(fe.reason)
	}
	return res, err
}

// fallbackError is an engine failure that sends the search to MySQL.
type fallbackError struct{ reason string }

func (f fallbackError) Error() string { return "search engine: " + f.reason }

func engineFailed(err error) error { return fallbackError{fallbackReason(err)} }

// ignored are the options asked for that a MySQL answer does not give.
func ignored(t TextOptions) []string {
	var out []string
	if t.Fuzzy {
		out = append(out, "fuzzy")
	}
	if t.Highlight {
		out = append(out, "highlight")
	}
	if t.Facets {
		out = append(out, "facets")
	}
	return out
}

// sortOf is the engine's sort for the request, as build orders it:
// relevance without one, a carried sortable field, else the entity's
// default; ok is false for a sort the documents do not carry.
func (e *Entity) sortOf(sc scope.Scope, req SearchRequest) (field string, desc, ok bool) {
	if req.Sort == "" {
		return "", true, true
	}
	key := "date" // the default sort of both message entities (m.date)
	if f := e.field(req.Sort, req.Internal); f != nil && f.Sortable && e.fieldVisible(sc, f) {
		key = f.Key
	}
	doc, carried := engineSorts[key]
	switch strings.ToLower(req.Order) {
	case "asc":
		desc = false
	case "desc":
		desc = true
	default:
		desc = e.defOrder == "DESC"
	}
	return doc, desc, carried
}

// values are the full-text conditions' values.
func (p *enginePlan) values() []string {
	out := make([]string, len(p.match))
	for i, c := range p.match {
		out[i] = c.V
	}
	return out
}

// request is req with the plan's chains, whose full-text conditions
// are now the engine's id lists.
func (p *enginePlan) request(req SearchRequest) SearchRequest {
	req.Conds, req.And = p.chains[0], p.chains[1:]
	return req
}

// engineWhole has the engine page and count the search, and reads the
// page's rows from MySQL.
func (e *Entity) engineWhole(ctx context.Context, db *sql.DB, sc scope.Scope, req SearchRequest,
	p enginePlan) (*SearchResult, error) {
	sortField, desc, ok := e.sortOf(sc, req)
	if !ok {
		return e.engineIDs(ctx, db, sc, req, p)
	}
	limit := min(max(req.Limit, 1), MaxLimit)
	offset := min(max(req.Offset, 0), MaxOffset)
	r, err := req.Engine.Query(ctx, EngineQuery{
		Entity: e.Name, Scope: sc, Match: p.values(), Text: req.Text, Filters: p.filters,
		Sort: sortField, Desc: desc, From: offset, Size: limit,
		Highlight: req.Text.Highlight, Facets: req.Text.Facets,
	})
	if err != nil {
		return nil, engineFailed(err)
	}
	for _, c := range p.match {
		c.engine, c.engineIDs = true, r.IDs
	}
	page := p.request(req)
	page.Offset, page.Limit = 0, limit
	if sortField == "" {
		page.rank = r.IDs
	}
	res, err := e.pageOnly(ctx, db, sc, page, len(r.IDs) == 0)
	if err != nil {
		return nil, err
	}
	res.Offset, res.Total, res.TotalExact = offset, r.Total, true
	res.Backend = &Backend{Used: "engine"}
	if req.Text.Highlight {
		res.Highlights = highlightsOf(res.Rows, r.Highlights)
	}
	if req.Text.Facets {
		res.Facets = r.Facets
		if err := e.facetNames(ctx, db, res.Facets); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// pageOnly runs a search's page query without counting; empty skips
// MySQL altogether.
func (e *Entity) pageOnly(ctx context.Context, db *sql.DB, sc scope.Scope, req SearchRequest,
	empty bool) (*SearchResult, error) {
	b, err := e.build(sc, req)
	if err != nil {
		return nil, err
	}
	res := &SearchResult{
		Entity: e, Scope: sc, Fields: e.fieldInfos(sc), Cols: e.columns(sc),
		Limit: b.limit, Sort: b.sort, Order: strings.ToLower(b.order),
	}
	if !empty {
		rs, err := db.QueryContext(ctx, b.pageSQL, b.pageArgs()...)
		if err == nil {
			res.Rows, err = scanRows(rs)
		}
		if err != nil {
			if isTimeout(err) {
				return nil, ErrSearchTimeout
			}
			return nil, err
		}
	}
	if req.Debug {
		if res.Debug, err = explain(ctx, db, b); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// engineIDs has the engine list the rows of each full-text condition,
// and MySQL count and page the search over those lists.
func (e *Entity) engineIDs(ctx context.Context, db *sql.DB, sc scope.Scope, req SearchRequest,
	p enginePlan) (*SearchResult, error) {
	for i, c := range p.match {
		r, err := req.Engine.Query(ctx, EngineQuery{
			Entity: e.Name, Scope: sc, Match: []string{c.V}, Text: req.Text,
			Filters: p.filters, Size: EngineMaxIDs,
		})
		if err != nil {
			return nil, engineFailed(err)
		}
		if r.Total > int64(len(r.IDs)) {
			return nil, fallbackError{FallbackTooMany}
		}
		c.engine, c.engineIDs = true, r.IDs
		if i == 0 {
			req.rank = r.IDs
		}
	}
	res, err := searchSQL(ctx, db, sc, e, p.request(req))
	if err != nil {
		return nil, err
	}
	res.Backend = &Backend{Used: "engine"}
	if req.Text.Facets {
		// Counted by MySQL over conditions the engine does not see,
		// the engine's per-chat totals would not add up to the total.
		res.Backend.Ignored = append(res.Backend.Ignored, "facets")
	}
	if req.Text.Highlight && len(res.Rows) == 0 {
		res.Highlights = [][]string{}
	} else if req.Text.Highlight {
		ids := make([]int64, len(res.Rows))
		for i, r := range res.Rows {
			ids[i] = r.Int("id")
		}
		h, err := req.Engine.Query(ctx, EngineQuery{
			Entity: e.Name, Scope: sc, Match: p.values(), Text: req.Text, IDs: ids,
			Size: len(ids), Highlight: true,
		})
		if err != nil {
			// The rows are there; only their highlights are not.
			res.Backend.Ignored = append(res.Backend.Ignored, "highlight")
		} else {
			res.Highlights = highlightsOf(res.Rows, h.Highlights)
		}
	}
	return res, nil
}

// highlightsOf aligns the engine's highlights to the rows. A service
// message gets none: its stored text may name what the viewer's
// rendering of it leaves out.
func highlightsOf(rows []Row, h map[int64][]string) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = []string{}
		if r.Str("content_type") == "service" {
			continue
		}
		if f := h[r.Int("id")]; f != nil {
			out[i] = f
		}
	}
	return out
}

// facetNames fills in the chats' names: group titles, or the other
// side's name of a private chat. The chats are the engine's answer
// within the scope, so the viewer sees each.
func (e *Entity) facetNames(ctx context.Context, db *sql.DB, fs []Facet) error {
	if len(fs) == 0 {
		return nil
	}
	ids := make([]int64, len(fs))
	for i, f := range fs {
		ids[i] = f.ChatID
	}
	q := "SELECT id, title, '' FROM `telegram_groups` WHERE " + inIDs("id", ids)
	if e == PrivateMessages {
		q = "SELECT id, first_name, last_name FROM telegram_users WHERE " + inIDs("id", ids)
	}
	rs, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rs.Close()
	names := map[int64]string{}
	for rs.Next() {
		var id int64
		var a, b sql.NullString
		if err := rs.Scan(&id, &a, &b); err != nil {
			return err
		}
		if e == PrivateMessages {
			names[id] = esc(DisplayName(a.String, b.String))
		} else {
			names[id] = esc(GroupTitle(a.String))
		}
	}
	if err := rs.Err(); err != nil {
		return err
	}
	for i := range fs {
		fs[i].Name = names[fs[i].ChatID]
	}
	return nil
}

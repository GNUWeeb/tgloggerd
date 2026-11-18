// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/testdb"
)

// The engine behind logdb.Search, on the logger fixtures "sample" and
// "search_engine", each test with a database and an index of its own.

const (
	gnuWeeb = int64(-1001000000001)
	kernel  = int64(-1001000000002) // hidden from the restricted admin
	rust    = int64(-1001000000003)
)

var (
	superadmin   = scope.For(scope.Viewer{ID: 1, Role: scope.Superadmin})
	hidingAdmin  = scope.For(scope.Viewer{ID: 2, Role: scope.Admin, HideGroups: []int64{kernel}})
	privateAdmin = scope.For(scope.Viewer{ID: 4, Role: scope.Admin, HidePrivate: true})
	member       = scope.For(scope.Viewer{ID: 3, Role: scope.Member, GrantedGroups: []int64{gnuWeeb}})
)

type searchEnv struct {
	db  *testdb.DB
	x   *Index
	eng *Engine
}

// newSearchEnv indexes the fixtures and returns an engine on them.
func newSearchEnv(t *testing.T) *searchEnv {
	t.Helper()
	db := testdb.New(t, testdb.Logger, "sample", "search_engine")
	x := testIndex(t)
	ix := NewIndexer(db.DB, x, nil, IndexerOptions{Log: quiet, Pause: time.Millisecond})
	ctx := context.Background()
	if err := ix.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if err := x.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	return &searchEnv{db: db, x: x, eng: NewEngine(x, EngineOptions{Log: quiet})}
}

func conds(t *testing.T, raw string) []logdb.Cond {
	t.Helper()
	cs, err := logdb.ParseConditions(raw)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func matches(v string) string {
	return fmt.Sprintf(`[{"c":"text","o":"matches","v":%q}]`, v)
}

func (e *searchEnv) search(t *testing.T, sc scope.Scope, ent *logdb.Entity, raw string,
	text logdb.TextOptions) *logdb.SearchResult {
	t.Helper()
	res, err := logdb.Search(context.Background(), e.db.DB, sc, ent, logdb.SearchRequest{
		Conds: conds(t, raw), Limit: 100, Engine: e.eng, Text: text,
	})
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return res
}

func rowIDs(res *logdb.SearchResult) []int64 {
	out := []int64{}
	for _, r := range res.Rows {
		out = append(out, r.Int("id"))
	}
	return out
}

func sorted(ids []int64) []int64 { return slices.Sorted(slices.Values(ids)) }

func wantRows(t *testing.T, what string, res *logdb.SearchResult, ids ...int64) {
	t.Helper()
	got := sorted(rowIDs(res))
	if !slices.Equal(got, sorted(ids)) || res.Total != int64(len(ids)) || !res.TotalExact {
		t.Errorf("%s: rows %v total %d exact %v, want %v", what, got, res.Total, res.TotalExact,
			sorted(ids))
	}
}

func wantEngine(t *testing.T, what string, res *logdb.SearchResult) {
	t.Helper()
	if res.Backend == nil || res.Backend.Used != "engine" {
		t.Errorf("%s: answered by %+v, want the engine", what, res.Backend)
	}
}

// Phrase, proximity, fuzzy, prefix and the boolean operators, as the
// syntax says, and the column conditions beside them.
func TestEngineSyntax(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	gm := logdb.GroupMessages
	for _, c := range []struct {
		raw  string
		text logdb.TextOptions
		want []int64
	}{
		{matches(`"quick fox"`), logdb.TextOptions{}, []int64{301}},
		{matches(`"quick fox" @2`), logdb.TextOptions{}, []int64{300, 301}},
		{matches(`kernal`), logdb.TextOptions{}, []int64{305}},
		{matches(`kernal~`), logdb.TextOptions{}, []int64{2, 301, 303, 304, 305}},
		{matches(`kernal`), logdb.TextOptions{Fuzzy: true}, []int64{2, 301, 303, 304, 305}},
		{matches(`kern*`), logdb.TextOptions{}, []int64{2, 301, 303, 304, 305}},
		{matches(`+kernel -panic`), logdb.TextOptions{}, []int64{2, 301}},
		{matches(`+kernel +panic`), logdb.TextOptions{}, []int64{303, 304}},
		{matches(`kernel lifetimes`), logdb.TextOptions{}, []int64{2, 301, 303, 304, 306}},
		{matches(`-kernel`), logdb.TextOptions{}, nil},
		{matches("K\u00e9rnel PANIC"), logdb.TextOptions{}, []int64{2, 301, 303, 304}},
		// Column conditions the documents carry, in the engine.
		{`[{"c":"text","o":"matches","v":"quick","n":"AND"},` +
			`{"c":"chat_id","o":"=","v":"-1001000000001"}]`, logdb.TextOptions{}, []int64{300, 302}},
		{`[{"c":"text","o":"matches","v":"deleted","n":"AND"},{"c":"deleted","o":"IS NULL"}]`,
			logdb.TextOptions{}, nil},
		{matches(`deleted`), logdb.TextOptions{}, []int64{6}},
		{`[{"c":"text","o":"matches","v":"quick","n":"AND"},` +
			`{"c":"sender_user_id","o":"!=","v":"9299000001"}]`, logdb.TextOptions{}, []int64{300, 306}},
		// An OR with a column: the engine lists the ids, MySQL pages.
		{`[{"c":"text","o":"matches","v":"quick","n":"OR"},` +
			`{"c":"chat_id","o":"=","v":"-1001000000005"}]`, logdb.TextOptions{},
			[]int64{300, 301, 302, 306, 14, 15}},
	} {
		res := e.search(t, superadmin, gm, c.raw, c.text)
		wantEngine(t, c.raw, res)
		wantRows(t, c.raw, res, c.want...)
	}

	// Relevance: the row with one of the words only comes last.
	res := e.search(t, superadmin, gm, matches(`quick fox`), logdb.TextOptions{})
	if ids := rowIDs(res); len(ids) != 4 || ids[3] != 306 {
		t.Errorf("relevance order: %v", ids)
	}
	// A sort the documents carry, in the engine.
	r, err := logdb.Search(context.Background(), e.db.DB, superadmin, gm, logdb.SearchRequest{
		Conds: conds(t, matches("quick")), Sort: "date", Order: "asc", Limit: 2, Offset: 1,
		Engine: e.eng,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ids := rowIDs(r); !slices.Equal(ids, []int64{301, 302}) || r.Total != 4 || r.Offset != 1 {
		t.Errorf("by date, second page: %v of %d at %d", ids, r.Total, r.Offset)
	}
	wantEngine(t, "by date", r)
}

// The scope is in the engine's query: a restricted admin and a member
// get no hits, totals, facets or highlights from what they may not
// see, whichever way the engine answers.
func TestEngineScope(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	gm := logdb.GroupMessages
	opts := logdb.TextOptions{Highlight: true, Facets: true}
	all := e.search(t, superadmin, gm, matches("quick"), opts)
	wantRows(t, "superadmin", all, 300, 301, 302, 306)
	facets := func(res *logdb.SearchResult) map[int64]int64 {
		out := map[int64]int64{}
		for _, f := range res.Facets {
			out[f.ChatID] = f.Count
		}
		return out
	}
	if f := facets(all); len(f) != 3 || f[gnuWeeb] != 2 || f[kernel] != 1 || f[rust] != 1 {
		t.Errorf("superadmin facets: %v", all.Facets)
	}
	if all.Facets[0].Name != "GNU/Weeb" {
		t.Errorf("facet names: %+v", all.Facets)
	}
	if len(all.Highlights) != 4 || !strings.Contains(strings.Join(all.Highlights[0], " "), "<mark>quick</mark>") {
		t.Errorf("highlights: %q", all.Highlights)
	}

	for _, c := range []struct {
		who   string
		sc    scope.Scope
		rows  []int64
		chats map[int64]int64
	}{
		{"admin hiding the kernel group", hidingAdmin, []int64{300, 302, 306},
			map[int64]int64{gnuWeeb: 2, rust: 1}},
		{"member of GNU/Weeb", member, []int64{300, 302}, map[int64]int64{gnuWeeb: 2}},
	} {
		res := e.search(t, c.sc, gm, matches("quick"), opts)
		wantEngine(t, c.who, res)
		wantRows(t, c.who, res, c.rows...)
		if f := facets(res); fmt.Sprint(f) != fmt.Sprint(c.chats) {
			t.Errorf("%s: facets %v, want %v", c.who, f, c.chats)
		}
		for i, h := range res.Highlights {
			if strings.Contains(strings.Join(h, " "), "kernel group") {
				t.Errorf("%s: highlight of row %d from the hidden group: %q", c.who, i, h)
			}
		}
		// A search only the hidden group's rows match.
		none := e.search(t, c.sc, gm, matches(`"kernel group"`), opts)
		wantRows(t, c.who+", the hidden group's text", none)
		if len(none.Facets) != 0 || len(none.Highlights) != 0 {
			t.Errorf("%s: %v %v", c.who, none.Facets, none.Highlights)
		}
		// The ids way, too.
		or := e.search(t, c.sc, gm, `[{"c":"text","o":"matches","v":"kernel","n":"OR"},`+
			`{"c":"chat_id","o":"=","v":"-1001000000005"}]`, opts)
		wantEngine(t, c.who+", ids", or)
		for _, id := range rowIDs(or) {
			if id == 301 || id == 303 {
				t.Errorf("%s, ids: row %d of the hidden group", c.who, id)
			}
		}
		// And the engine's own answer, before MySQL reads anything.
		r, err := e.eng.Query(context.Background(), logdb.EngineQuery{Entity: gm.Name, Scope: c.sc,
			Match: []string{"kernel"}, Size: 100, Facets: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range r.IDs {
			if id == 301 || id == 303 {
				t.Errorf("%s: the engine answered row %d of the hidden group", c.who, id)
			}
		}
		if _, ok := facets(&logdb.SearchResult{Facets: r.Facets})[kernel]; ok {
			t.Errorf("%s: a facet of the hidden group", c.who)
		}
	}

	// Private messages: all of them to a superadmin, none to an admin
	// they are hidden from or a member, neither in the search (404) nor
	// in the engine's answer.
	pm := logdb.PrivateMessages
	wantRows(t, "superadmin, private", e.search(t, superadmin, pm, matches("quick"), opts), 300)
	for _, c := range []struct {
		who string
		sc  scope.Scope
	}{{"admin without private chats", privateAdmin}, {"member", member}} {
		_, err := logdb.Search(context.Background(), e.db.DB, c.sc, pm, logdb.SearchRequest{
			Conds: conds(t, matches("quick")), Engine: e.eng})
		if !errors.Is(err, logdb.ErrNotFound) {
			t.Errorf("%s: private search: %v", c.who, err)
		}
		r, err := e.eng.Query(context.Background(), logdb.EngineQuery{Entity: pm.Name, Scope: c.sc,
			Match: []string{"quick"}, Size: 100, Facets: true, Highlight: true})
		if err != nil {
			t.Fatal(err)
		}
		if r.Total != 0 || len(r.IDs) != 0 || len(r.Facets) != 0 || len(r.Highlights) != 0 {
			t.Errorf("%s: the engine answered %+v", c.who, r)
		}
	}
	// A member with no groups sees nothing.
	none := scope.For(scope.Viewer{ID: 5, Role: scope.Member})
	if r := e.search(t, none, gm, matches("quick"), opts); r.Total != 0 || len(r.Rows) != 0 {
		t.Errorf("member without groups: %d %v", r.Total, rowIDs(r))
	}
}

// slowEngine is a proxy to the engine that holds every search for
// delay.
func slowEngine(t *testing.T, target string, delay time.Duration) string {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	p := httputil.NewSingleHostReverseProxy(u)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		p.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Search falls back to MySQL when the engine is down, too slow, behind
// the archive or asked not to answer, and says so; the rows are
// MySQL's own.
func TestEngineFallback(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	gm := logdb.GroupMessages
	raw := matches("kernel")
	mysqlOnly, err := logdb.Search(context.Background(), e.db.DB, superadmin, gm,
		logdb.SearchRequest{Conds: conds(t, raw), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if mysqlOnly.Backend != nil {
		t.Errorf("without an engine the answer names a backend: %+v", mysqlOnly.Backend)
	}
	engineOn := func(base string, o EngineOptions) *Engine {
		c, err := NewClient(base)
		if err != nil {
			t.Fatal(err)
		}
		x, err := c.Index(e.x.Name())
		if err != nil {
			t.Fatal(err)
		}
		o.Log = quiet
		return NewEngine(x, o)
	}
	check := func(what string, eng *Engine, text logdb.TextOptions, reason string) {
		t.Helper()
		start := time.Now()
		res, err := logdb.Search(context.Background(), e.db.DB, superadmin, gm,
			logdb.SearchRequest{Conds: conds(t, raw), Limit: 100, Engine: eng, Text: text})
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if res.Backend == nil || res.Backend.Used != "mysql" || res.Backend.Fallback != reason {
			t.Errorf("%s: backend %+v, want mysql for %s", what, res.Backend, reason)
		}
		if !slices.Equal(rowIDs(res), rowIDs(mysqlOnly)) || res.Total != mysqlOnly.Total {
			t.Errorf("%s: rows %v, MySQL's %v", what, rowIDs(res), rowIDs(mysqlOnly))
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("%s: took %v", what, d)
		}
	}

	check("asked", e.eng, logdb.TextOptions{MySQL: true}, logdb.FallbackAsked)
	// Nothing listens on port 1 of the loopback.
	down := engineOn("http://127.0.0.1:1", EngineOptions{Timeout: time.Second})
	check("engine down", down, logdb.TextOptions{}, logdb.FallbackUnavailable)

	slow := engineOn(slowEngine(t, e.x.c.URL(), 2*time.Second),
		EngineOptions{Timeout: 200 * time.Millisecond, Backoff: time.Minute})
	check("engine slow", slow, logdb.TextOptions{}, logdb.FallbackTimeout)
	// Skipped for its backoff now, without waiting on it again.
	check("engine skipped", slow, logdb.TextOptions{}, logdb.FallbackUnavailable)

	// Options MySQL cannot give are listed as ignored.
	res, err := logdb.Search(context.Background(), e.db.DB, superadmin, gm, logdb.SearchRequest{
		Conds: conds(t, raw), Engine: down, Text: logdb.TextOptions{Fuzzy: true, Highlight: true}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend == nil || strings.Join(res.Backend.Ignored, ",") != "fuzzy,highlight" {
		t.Errorf("ignored options: %+v", res.Backend)
	}

	// An index whose backfill has not finished is behind, and so is
	// one not known in step for longer than MaxLag.
	fresh := testIndex(t)
	check("backfill running", NewEngine(fresh, EngineOptions{Log: quiet}), logdb.TextOptions{},
		logdb.FallbackBehind)
	st, err := e.x.LoadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st.SyncedAt = time.Now().Add(-time.Hour).Unix()
	if err := e.x.SaveState(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	check("stale", NewEngine(e.x, EngineOptions{Log: quiet, MaxLag: time.Minute}),
		logdb.TextOptions{}, logdb.FallbackBehind)

	// A search without a full-text condition stays on MySQL and says
	// which backend answered, without a fallback.
	res, err = logdb.Search(context.Background(), e.db.DB, superadmin, gm, logdb.SearchRequest{
		Conds: conds(t, `[{"c":"chat_id","o":"=","v":"-1001000000003"}]`), Engine: e.eng})
	if err != nil || res.Backend == nil || res.Backend.Used != "mysql" || res.Backend.Fallback != "" {
		t.Errorf("no full-text condition: %+v %v", res, err)
	}
	// A refused search is refused as without the engine.
	_, err = logdb.Search(context.Background(), e.db.DB, superadmin, gm, logdb.SearchRequest{
		Conds: conds(t, `[{"c":"text","o":"=","v":"x"}]`), Engine: e.eng})
	var se *logdb.SearchError
	if !errors.As(err, &se) {
		t.Errorf("a bad operator: %v", err)
	}
}

func TestParseText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   string
		want string
	}{
		{`a +b -c`, `{0 false a 0 false } {43 false b 0 false } {45 false c 0 false }`},
		{`"x y" @3 z*`, `{0 true x y 3 false } {0 false z 0 true }`},
		{`w~ v~2 u~9`, `{0 false w 0 false AUTO} {0 false v 0 false 2} {0 false u 0 false AUTO}`},
		{`(a) ~b <c >d`, `{0 false a 0 false } {0 false b 0 false } {0 false c 0 false } {0 false d 0 false }`},
		{`"unclosed`, `{0 true unclosed 0 false }`},
		{`"" * - + @5`, `{0 false 5 0 false }`},
	} {
		var got []string
		for _, cl := range parseText(c.in) {
			got = append(got, fmt.Sprint(cl))
		}
		if s := strings.Join(got, " "); s != c.want {
			t.Errorf("parseText(%q) = %s, want %s", c.in, s, c.want)
		}
	}
}

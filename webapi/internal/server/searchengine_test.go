// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/searchengine"
	"tgloggerd/webapi/internal/testdb"
	"tgloggerd/webapi/internal/webdb"
)

// The search API with the optional search engine: the capabilities,
// the options and the answers' new parts, and the request scope over
// them; and without an engine, answers exactly as before. The engine
// tests run with the test engine (scripts/test-web.sh --search-engine)
// and skip without it.

// engineEnv is a searchEnv whose service has a search engine (nil:
// none), indexed from the fixtures "sample" and "search_engine". 1 is
// a superadmin, 2 an admin the kernel group and private chats are
// hidden from, 3 a member granted GNU/Weeb.
func engineEnv(t *testing.T, withEngine bool) *searchEnv {
	t.Helper()
	d := testdb.New(t, testdb.Logger, "sample", "search_engine")
	db := interpolating(t, d)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	var engine SearchEngine
	if withEngine {
		u := os.Getenv("WEBAPI_TEST_SEARCH_ENGINE_URL")
		if u == "" {
			t.Skip("WEBAPI_TEST_SEARCH_ENGINE_URL is not set (scripts/test-web.sh --search-engine)")
		}
		c, err := searchengine.NewClient(u)
		if err != nil {
			t.Fatal(err)
		}
		var rnd [6]byte
		_, _ = rand.Read(rnd[:])
		x, err := c.Index("test-server-" + hex.EncodeToString(rnd[:]))
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		t.Cleanup(func() { _ = x.Drop(context.Background()) })
		ix := searchengine.NewIndexer(d.DB, x, nil, searchengine.IndexerOptions{
			Log: quiet, Pause: time.Millisecond})
		if err := ix.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		if err := x.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
		engine = searchengine.NewEngine(x, searchengine.EngineOptions{Log: quiet})
	}
	signer := auth.NewSigner(testKey)
	ck := &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true}
	e := &searchEnv{t: t, tokens: auth.NewFileTokens(testKey), cookies: map[string]string{}}
	states := map[uint64]webdb.SessionState{
		1: {Role: "superadmin", Active: true},
		2: {Role: "admin", Active: true, HidePrivate: true, HideGroups: []int64{-1001000000002}},
		3: {Role: "member", Active: true, GrantedGroups: []int64{-1001000000001}},
	}
	e.h = New(Deps{
		Log:     quiet,
		RO:      db,
		App:     testdb.New(t, testdb.Web, "accounts").DB,
		Signer:  signer,
		Cookies: ck,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Tokens:  e.tokens,
		state: func(_ context.Context, uid uint64, _ [32]byte) (webdb.SessionState, error) {
			st, ok := states[uid]
			if !ok {
				return webdb.SessionState{}, webdb.ErrNoUser
			}
			return st, nil
		},
		SearchEngine: engine,
	})
	for uid, st := range states {
		e.cookies[st.Role] = signer.EncodeSession(ck.NewSession(uid, st.Role, st.Role, 0))
	}
	return e
}

const quickSearch = `[{"c":"text","o":"matches","v":"quick"}]`

// Without an engine nothing changes: no capabilities, and the options
// change nothing in the answer.
func TestSearchWithoutEngine(t *testing.T) {
	t.Parallel()
	e := engineEnv(t, false)
	b := e.ok("superadmin", "/api/search/capabilities")
	if js(b) != `{"engine":null,"entities":{}}` {
		t.Errorf("capabilities: %s", js(b))
	}
	plain := e.ok("superadmin", search("group_messages", quickSearch))
	opts := e.ok("superadmin", search("group_messages", quickSearch, "highlight", "1",
		"facets", "1", "fuzzy", "1", "engine", "mysql"))
	if js(plain) != js(opts) {
		t.Errorf("the options changed the answer:\n%s\n%s", js(plain), js(opts))
	}
	for _, k := range []string{"backend", "highlights", "facets"} {
		if _, ok := plain[k]; ok {
			t.Errorf("answer has %s without an engine", k)
		}
	}
	if st, _ := e.get("", "/api/search/capabilities"); st != 401 {
		t.Errorf("signed out: %d", st)
	}
}

func TestSearchCapabilities(t *testing.T) {
	t.Parallel()
	e := engineEnv(t, true)
	// js re-encodes the decoded answer, its keys in order.
	opts := `["phrase","proximity","prefix","fuzzy","highlight","facets"]`
	entity := `{"available":true,"fallback":null,"fields":["text"],"options":` + opts + `}`
	for role, want := range map[string]string{
		"superadmin": `{"engine":"opensearch","entities":{"group_messages":` + entity +
			`,"private_messages":` + entity + `}}`,
		// Private chats are hidden from the admin and never a member's.
		"admin":  `{"engine":"opensearch","entities":{"group_messages":` + entity + `}}`,
		"member": `{"engine":"opensearch","entities":{"group_messages":` + entity + `}}`,
	} {
		if b := e.ok(role, "/api/search/capabilities"); js(b) != want {
			t.Errorf("%s: %s\nwant %s", role, js(b), want)
		}
	}
}

func TestSearchEngineAnswers(t *testing.T) {
	t.Parallel()
	e := engineEnv(t, true)
	q := search("group_messages", quickSearch, "highlight", "1", "facets", "1")
	for _, c := range []struct {
		role   string
		total  int64
		facets string
	}{
		// Busiest first, ties by chat id.
		{"superadmin", 4, `-1001000000001:2 -1001000000003:1 -1001000000002:1`},
		{"admin", 3, `-1001000000001:2 -1001000000003:1`},
		{"member", 2, `-1001000000001:2`},
	} {
		b := e.ok(c.role, q)
		wantTotal(t, c.role, b, c.total, true)
		if js(b["backend"]) != `{"used":"engine"}` {
			t.Errorf("%s: backend %s", c.role, js(b["backend"]))
		}
		hs := b["highlights"].([]any)
		if len(hs) != len(rows(b)) || !strings.Contains(fmt.Sprint(hs), "<mark>quick</mark>") {
			t.Errorf("%s: highlights %s", c.role, js(hs))
		}
		var fs []string
		for _, f := range b["facets"].([]any) {
			m := f.(map[string]any)
			fs = append(fs, js(m["chat_id"])+":"+js(m["count"]))
		}
		if strings.Join(fs, " ") != c.facets {
			t.Errorf("%s: facets %v, want %s", c.role, fs, c.facets)
		}
		if strings.Contains(js(b), "kernel group") && c.role != "superadmin" {
			t.Errorf("%s: the hidden group's text in %s", c.role, js(b))
		}
	}

	// engine=mysql: MySQL answers, says so, and gives no highlights.
	b := e.ok("superadmin", search("group_messages", quickSearch, "engine", "mysql", "highlight", "1"))
	if js(b["backend"]) != `{"fallback":"asked","ignored":["highlight"],"used":"mysql"}` ||
		b["highlights"] != nil {
		t.Errorf("engine=mysql: %s %v", js(b["backend"]), b["highlights"])
	}
	wantTotal(t, "engine=mysql", b, 4, true)

	// The list pages' search text goes to the engine too: kernal~ finds
	// the kernel messages, which MySQL's FULLTEXT would not.
	p := e.props("superadmin", "group_messages", "q", "kernal~")
	if p["total"] != float64(5) {
		t.Errorf("list page search: total %v, want 5", p["total"])
	}
}

// The message lists' pages take the same options from their query and
// answer the engine's part as the search API does, within the scope,
// with their entity's capabilities; without an engine they answer
// none of it.
func TestListPagesEngineParts(t *testing.T) {
	t.Parallel()
	e := engineEnv(t, true)
	for _, c := range []struct {
		role   string
		facets string
	}{
		{"superadmin", `-1001000000001:2 -1001000000003:1 -1001000000002:1`},
		{"admin", `-1001000000001:2 -1001000000003:1`},
		{"member", `-1001000000001:2`},
	} {
		p := e.props(c.role, "group_messages", "q", "quick", "highlight", "1", "facets", "1")
		if js(p["backend"]) != `{"used":"engine"}` {
			t.Errorf("%s: backend %s", c.role, js(p["backend"]))
		}
		hs, _ := p["highlights"].([]any)
		rs, _ := p["rows"].([]any)
		if len(rs) == 0 || len(hs) != len(rs) || !strings.Contains(fmt.Sprint(hs), "<mark>quick</mark>") {
			t.Errorf("%s: %d rows, highlights %s", c.role, len(rs), js(hs))
		}
		var fs []string
		for _, f := range p["facets"].([]any) {
			m := f.(map[string]any)
			fs = append(fs, js(m["chat_id"])+":"+js(m["count"]))
		}
		if strings.Join(fs, " ") != c.facets {
			t.Errorf("%s: facets %v, want %s", c.role, fs, c.facets)
		}
		if strings.Contains(js(p), "kernel group") && c.role != "superadmin" {
			t.Errorf("%s: the hidden group's text in %s", c.role, js(p))
		}
		want := `{"available":true,"fallback":null,"fields":["text"],"options":` +
			`["phrase","proximity","prefix","fuzzy","highlight","facets"]}`
		if js(p["engine"]) != want {
			t.Errorf("%s: engine %s", c.role, js(p["engine"]))
		}
	}
	// engine=mysql on the page: MySQL answers and says so.
	p := e.props("superadmin", "group_messages", "q", "quick", "engine", "mysql", "highlight", "1")
	if js(p["backend"]) != `{"fallback":"asked","ignored":["highlight"],"used":"mysql"}` ||
		p["highlights"] != nil {
		t.Errorf("engine=mysql: %s %v", js(p["backend"]), p["highlights"])
	}

	plain := engineEnv(t, false).props("superadmin", "group_messages", "q", "quick",
		"highlight", "1", "facets", "1")
	for _, k := range []string{"backend", "highlights", "facets", "engine"} {
		if _, ok := plain[k]; ok {
			t.Errorf("list page has %s without an engine", k)
		}
	}
}

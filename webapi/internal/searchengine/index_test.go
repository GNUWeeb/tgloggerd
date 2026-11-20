// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"testing"
	"time"
)

// EnvURL names the variable holding the test engine's URL
// (docker-compose.test.yml, scripts/test-web.sh --search-engine).
const EnvURL = "WEBAPI_TEST_SEARCH_ENGINE_URL"

// testIndex is an index of the test's own on the test engine, dropped
// when the test ends. It skips the test when no engine is configured.
func testIndex(t *testing.T) *Index {
	t.Helper()
	u := os.Getenv(EnvURL)
	if u == "" {
		t.Skip(EnvURL + " is not set (scripts/test-web.sh --search-engine)")
	}
	c, err := NewClient(u)
	if err != nil {
		t.Fatal(err)
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	x, err := c.Index("test-" + hex.EncodeToString(rnd[:]))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := x.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := x.Drop(ctx); err != nil {
			t.Errorf("dropping %s: %v", x.Name(), err)
		}
	})
	return x
}

func count(t *testing.T, x *Index, query map[string]any) int64 {
	t.Helper()
	var ans struct {
		Count int64 `json:"count"`
	}
	if err := x.c.do(context.Background(), http.MethodPost, "/"+seg(x.name)+"/_count",
		map[string]any{"query": query}, &ans); err != nil {
		t.Fatal(err)
	}
	return ans.Count
}

func TestNewClient(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "opensearch:9200", "ftp://x", "http://", "http://u:p@x:9200",
		"http://x:9200/?a=1"} {
		if _, err := NewClient(bad); err == nil {
			t.Errorf("NewClient(%q) accepted", bad)
		}
	}
	c, err := NewClient("http://opensearch:9200/")
	if err != nil || c.URL() != "http://opensearch:9200" {
		t.Fatalf("%v %v", c, err)
	}
	for _, bad := range []string{"", "Upper", "-x", "a b", "x-state", "a/b"} {
		if _, err := c.Index(bad); err == nil {
			t.Errorf("Index(%q) accepted", bad)
		}
	}
}

func TestDocID(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		scope string
		row   int64
		id    string
	}{{ScopeGroup, 7, "g-7"}, {ScopePrivate, 12, "p-12"}} {
		if got := DocID(c.scope, c.row); got != c.id {
			t.Errorf("DocID(%s, %d) = %s", c.scope, c.row, got)
		}
		s, r, ok := parseDocID(c.id)
		if !ok || s != c.scope || r != c.row {
			t.Errorf("parseDocID(%s) = %s %d %v", c.id, s, r, ok)
		}
	}
	for _, bad := range []string{"", "g", "g-", "x-1", "g-0", "g--1", "g-1a"} {
		if _, _, ok := parseDocID(bad); ok {
			t.Errorf("parseDocID(%q) accepted", bad)
		}
	}
}

// The index is made once, its writes are idempotent, and its state
// survives a reload.
func TestIndexWritesAndState(t *testing.T) {
	t.Parallel()
	x := testIndex(t)
	ctx := context.Background()
	if err := x.Ensure(ctx); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	sender := int64(9299000001)
	docs := []Op{
		{ID: DocID(ScopeGroup, 1), Doc: &Doc{Scope: ScopeGroup, RowID: 1, ChatID: -100,
			MessageID: 1 << 20, SenderID: &sender, Date: 1000, ContentType: "text",
			Text: "K\u00e9rnel build broken"}},
		{ID: DocID(ScopePrivate, 1), Doc: &Doc{Scope: ScopePrivate, RowID: 1, ChatID: sender,
			MessageID: 2 << 20, Date: 1001, ContentType: "text", Outgoing: true,
			Text: "kernel meeting"}},
	}
	for range 2 {
		if err := x.Bulk(ctx, docs); err != nil {
			t.Fatal(err)
		}
	}
	// Deleting a document that is not there is not an error.
	if err := x.Bulk(ctx, []Op{{ID: DocID(ScopeGroup, 99), Delete: true}}); err != nil {
		t.Fatal(err)
	}
	if err := x.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	match := map[string]any{"match": map[string]any{"text": "kernel"}}
	if n := count(t, x, match); n != 2 {
		t.Errorf("kernel: %d documents, want 2 (folded and lowercased)", n)
	}
	if err := x.Bulk(ctx, []Op{{ID: DocID(ScopeGroup, 1), Delete: true}}); err != nil {
		t.Fatal(err)
	}
	if err := x.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(t, x, match); n != 1 {
		t.Errorf("after the delete: %d documents, want 1", n)
	}

	// A document of another shape is refused (the mapping is strict).
	err := x.c.do(ctx, http.MethodPut, "/"+seg(x.name)+"/_doc/x", map[string]any{"other": 1}, nil)
	if err == nil {
		t.Errorf("a document with an unknown field was accepted")
	}

	st, err := x.LoadState(ctx)
	if err != nil || st != (State{}) {
		t.Fatalf("fresh state: %+v %v", st, err)
	}
	want := State{Group: TableState{NextID: 10, Done: true}, Private: TableState{NextID: 3},
		SyncedAt: 1700000000, ReconciledAt: 1690000000}
	if err := x.SaveState(ctx, want); err != nil {
		t.Fatal(err)
	}
	if st, err = x.LoadState(ctx); err != nil || st != want {
		t.Fatalf("state: %+v %v, want %+v", st, err, want)
	}
}

// An index made with another layout version is refused, not written to.
func TestIndexOtherVersion(t *testing.T) {
	t.Parallel()
	x := testIndex(t)
	ctx := context.Background()
	y, err := x.c.Index(x.name + "-old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = y.Drop(context.Background()) })
	body := indexSettings()
	body["mappings"].(map[string]any)["_meta"] = map[string]any{"version": mappingVersion + 1}
	if err := x.c.do(ctx, http.MethodPut, "/"+seg(y.name), body, nil); err != nil {
		t.Fatal(err)
	}
	if err := y.Ensure(ctx); err == nil {
		t.Fatal("an index of another layout version was accepted")
	}
}

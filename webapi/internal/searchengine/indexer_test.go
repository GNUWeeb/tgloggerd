// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"tgloggerd/webapi/internal/live"
	"tgloggerd/webapi/internal/testdb"
)

// The indexer on the logger fixture "sample" (internal/testdb), each
// test with a database and an index of its own. The sample's group
// messages with text are rows 1-6 and 9-16 (7 has an empty text, 8
// none), its private messages with text rows 1-4 and 6.

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// getDoc is the document of id, if there is one.
func getDoc(t *testing.T, x *Index, id string) (Doc, bool) {
	t.Helper()
	var d struct {
		Found  bool `json:"found"`
		Source Doc  `json:"_source"`
	}
	err := x.c.do(context.Background(), http.MethodGet, "/"+seg(x.name)+"/_doc/"+seg(id), nil, &d)
	if IsNotFound(err) {
		return Doc{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return d.Source, d.Found
}

// waitFor polls cond until it holds, failing the test after limit.
func waitFor(t *testing.T, what string, limit time.Duration, cond func() bool) {
	t.Helper()
	end := time.Now().Add(limit)
	for !cond() {
		if time.Now().After(end) {
			t.Fatalf("%s: not within %v", what, limit)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func allDocs(t *testing.T, x *Index, scope string) int64 {
	t.Helper()
	if err := x.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	return count(t, x, map[string]any{"term": map[string]any{"scope": scope}})
}

func exec(t *testing.T, db *testdb.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// The backfill reads the tables in id order a batch at a time, keeps
// its place in the state, and a new indexer resumes from there.
func TestIndexerBackfillResumes(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample")
	x := testIndex(t)
	ctx := context.Background()
	o := IndexerOptions{Batch: 4, Pause: time.Millisecond, Log: quiet}

	ix := NewIndexer(db.DB, x, nil, o)
	for range 2 {
		if err := ix.catchUpStep(ctx); err != nil {
			t.Fatal(err)
		}
	}
	st, err := x.LoadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Group != (TableState{NextID: 8}) || st.Private != (TableState{NextID: 6, Done: true}) {
		t.Fatalf("after two batches: %+v", st)
	}
	if n := allDocs(t, x, ScopeGroup); n != 6 {
		t.Errorf("group documents after rows 1-8: %d, want 6", n)
	}

	// A new indexer, as after a restart, goes on from row 9.
	ix = NewIndexer(db.DB, x, nil, o)
	if ix.st, err = x.LoadState(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ix.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ = x.LoadState(ctx); !st.Group.Done || st.Group.NextID != 16 || st.SyncedAt == 0 {
		t.Errorf("caught up: %+v", st)
	}
	if g, p := allDocs(t, x, ScopeGroup), allDocs(t, x, ScopePrivate); g != 14 || p != 5 {
		t.Errorf("documents: %d group, %d private, want 14 and 5", g, p)
	}
	for _, id := range []string{"g-7", "g-8", "p-5"} {
		if _, ok := getDoc(t, x, id); ok {
			t.Errorf("%s has no text but a document", id)
		}
	}
	d, ok := getDoc(t, x, "g-3")
	if !ok || d.ChatID != -1001000000001 || d.MessageID != 410003<<20 || d.SenderID == nil ||
		*d.SenderID != 9299000002 || d.EditDate == 0 || d.ContentType != "text" ||
		d.Text != "the build is broken on arm64 again, see the log" {
		t.Errorf("g-3: %+v", d)
	}
	if d, _ := getDoc(t, x, "g-6"); !d.Deleted {
		t.Errorf("g-6 is deleted: %+v", d)
	}
	if d, _ := getDoc(t, x, "p-2"); !d.Outgoing || d.SenderID != nil || d.ChatID != 9299000001 {
		t.Errorf("p-2: %+v", d)
	}
}

// chanSource is a fake event stream: a hello, then what the test sends.
type chanSource struct{ ch chan live.Event }

func (s chanSource) Stream(ctx context.Context, since uint64, emit func(live.Event)) error {
	emit(live.Event{Kind: live.KindHello, Seq: 1})
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-s.ch:
			emit(e)
		}
	}
}

// runIndexer runs ix until the test ends, and waits for the backfill.
func runIndexer(t *testing.T, ix *Indexer) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ix.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Error("the indexer did not stop")
		}
	})
	waitFor(t, "the backfill", 30*time.Second, func() bool {
		st, err := ix.x.LoadState(context.Background())
		return err == nil && st.Group.Done && st.Private.Done
	})
}

// New, edited and deleted messages reach the index from the event
// stream within seconds.
func TestIndexerEvents(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample")
	x := testIndex(t)
	src := chanSource{ch: make(chan live.Event, 16)}
	ix := NewIndexer(db.DB, x, src, IndexerOptions{Log: quiet, Flush: 50 * time.Millisecond})
	runIndexer(t, ix)

	const chat, msg = int64(-1001000000001), int64(490001 << 20)
	send := func(kind string) {
		src.ch <- live.Event{Kind: kind, ChatID: chat, MessageID: msg}
	}
	doc := func(want func(Doc, bool) bool) func() bool {
		return func() bool { return want(getDoc(t, x, "g-100")) }
	}

	exec(t, db, "INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, "+
		"date, content_type, text) VALUES (100, ?, ?, 9299000001, 1794931200, 'text', "+
		"'fresh pancake recipe')", chat, msg)
	send(live.KindMessageNew)
	waitFor(t, "the new message", 5*time.Second, doc(func(d Doc, ok bool) bool {
		return ok && d.Text == "fresh pancake recipe" && !d.Deleted
	}))

	exec(t, db, "UPDATE telegram_group_messages SET text = 'fresh waffle recipe', "+
		"edit_date = 1794931300 WHERE id = 100")
	send(live.KindMessageEdited)
	waitFor(t, "the edit", 5*time.Second, doc(func(d Doc, ok bool) bool {
		return ok && d.Text == "fresh waffle recipe" && d.EditDate == 1794931300
	}))

	exec(t, db, "UPDATE telegram_group_messages SET deleted_at = NOW() WHERE id = 100")
	send(live.KindMessageDeleted)
	waitFor(t, "the deletion", 5*time.Second, doc(func(d Doc, ok bool) bool {
		return ok && d.Deleted
	}))

	// An edit that leaves no text leaves no document.
	exec(t, db, "UPDATE telegram_group_messages SET text = NULL, content_type = 'photo' "+
		"WHERE id = 100")
	send(live.KindMessageEdited)
	waitFor(t, "the document's removal", 5*time.Second, doc(func(_ Doc, ok bool) bool {
		return !ok
	}))

	// A private message, by the other side's user id.
	exec(t, db, "INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, "+
		"is_outgoing, date, content_type, text) VALUES (100, 9299000002, 690001 << 20, NULL, "+
		"1, 1794931200, 'text', 'see you at the meetup')")
	src.ch <- live.Event{Kind: live.KindMessageNew, ChatID: 9299000002, MessageID: 690001 << 20}
	waitFor(t, "the private message", 5*time.Second, func() bool {
		d, ok := getDoc(t, x, "p-100")
		return ok && d.Outgoing && d.Text == "see you at the meetup"
	})
}

// The reconcile repairs what the stream missed: a row inserted, one
// edited, one deleted and one emptied without any event.
func TestIndexerReconcileRepairs(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample")
	x := testIndex(t)
	ctx := context.Background()
	ix := NewIndexer(db.DB, x, nil, IndexerOptions{Log: quiet, Pause: time.Millisecond})
	if err := ix.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	exec(t, db, "INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, "+
		"date, content_type, text) VALUES (101, -1001000000002, 490101 << 20, 9299000001, "+
		"?, 'text', 'missed while the service was down')", now)
	exec(t, db, "UPDATE telegram_group_messages SET text = 'edited without an event', "+
		"edit_date = ? WHERE id = 2", now)
	exec(t, db, "UPDATE telegram_group_messages SET deleted_at = NOW() WHERE id = 9")
	exec(t, db, "UPDATE telegram_group_messages SET text = NULL, edit_date = ? WHERE id = 10", now)
	exec(t, db, "UPDATE telegram_private_messages SET text = 'edited too', edit_date = ? "+
		"WHERE id = 3", now)

	stale := func() []string {
		var out []string
		if _, ok := getDoc(t, x, "g-101"); !ok {
			out = append(out, "g-101 missing")
		}
		if d, _ := getDoc(t, x, "g-2"); d.Text != "edited without an event" {
			out = append(out, "g-2 not edited")
		}
		if d, _ := getDoc(t, x, "g-9"); !d.Deleted {
			out = append(out, "g-9 not deleted")
		}
		if _, ok := getDoc(t, x, "g-10"); ok {
			out = append(out, "g-10 still there")
		}
		if d, _ := getDoc(t, x, "p-3"); d.Text != "edited too" {
			out = append(out, "p-3 not edited")
		}
		return out
	}
	if s := stale(); len(s) != 5 {
		t.Fatalf("before the reconcile, only %v are stale", s)
	}
	if err := ix.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if s := stale(); len(s) != 0 {
		t.Errorf("after the reconcile: %v", s)
	}
}

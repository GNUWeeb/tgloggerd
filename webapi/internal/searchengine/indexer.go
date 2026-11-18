// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"tgloggerd/webapi/internal/live"
)

// The indexer keeps the index in step with the message tables, from
// three sides:
//
//   - The daemon's event stream (message.new, .edited, .deleted): each
//     event names a row by chat and message id; the indexer reads the
//     row again by the tables' unique (chat_id, message_id) keys and
//     writes its document, gathering the events of half a second into
//     one read and one bulk.
//   - The catch-up pass: the rows above the state's NextID, in primary
//     key order, a batch at a time with a pause between batches, so
//     the live MySQL is never starved. From an empty state it is the
//     backfill of the whole archive; once done it picks up the rows the
//     stream missed. It never filters on created_at, which has no
//     index.
//   - The reconcile, every Reconcile (more often while no stream
//     delivers): the catch-up pass, then the rows deleted
//     (deleted_at's index) and edited (the (chat_id, edit_date)
//     indexes, a batch of chats at a time) since the last reconcile
//     began, less a margin. It repairs what the stream missed: a
//     disconnect, a resync, a downtime of the service.
//
// Every write is idempotent (document id from scope and row id), so a
// row written twice, by an event and by the reconcile, is harmless.
// One indexer per index: run it in one instance of the service only.

// IndexerOptions are the indexer's limits; zero fields take defaults.
type IndexerOptions struct {
	// Batch is the rows read per query of the catch-up pass and the
	// reconcile (default 1000); Pause the wait between two (default
	// 200 ms).
	Batch int
	Pause time.Duration
	// Reconcile is the time between two reconciles (default 10
	// minutes), NoStream the time while no event stream delivers
	// (default 1 minute), Margin how far before the last reconcile's
	// start the next one looks for edits and deletions (default 5
	// minutes).
	Reconcile time.Duration
	NoStream  time.Duration
	Margin    time.Duration
	// Flush is how long events are gathered before their rows are read
	// (default 500 ms).
	Flush time.Duration
	Log   *slog.Logger
}

func (o *IndexerOptions) defaults() {
	if o.Batch <= 0 {
		o.Batch = 1000
	}
	if o.Pause <= 0 {
		o.Pause = 200 * time.Millisecond
	}
	if o.Reconcile <= 0 {
		o.Reconcile = 10 * time.Minute
	}
	if o.NoStream <= 0 {
		o.NoStream = time.Minute
	}
	if o.Margin <= 0 {
		o.Margin = 5 * time.Minute
	}
	if o.Flush <= 0 {
		o.Flush = 500 * time.Millisecond
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
}

// Indexer keeps one index in step with the message tables.
type Indexer struct {
	db  *sql.DB
	x   *Index
	src live.Source // nil: no event stream, the reconcile alone
	o   IndexerOptions
	now func() time.Time

	events   chan live.Event
	up       atomic.Bool // the stream delivers
	overflow atomic.Bool // events were dropped: reconcile

	// Owned by Run's goroutine.
	st        State
	saved     time.Time // when st was last written
	pending   map[msgKey]struct{}
	reconcile time.Time // when the next reconcile is due
}

// NewIndexer is an indexer reading db (the logger's schema) into x, fed
// by src (nil for none).
func NewIndexer(db *sql.DB, x *Index, src live.Source, o IndexerOptions) *Indexer {
	o.defaults()
	return &Indexer{
		db: db, x: x, src: src, o: o, now: time.Now,
		events:  make(chan live.Event, 4096),
		pending: map[msgKey]struct{}{},
	}
}

// msgKey names a row as the events do.
type msgKey struct{ chat, msg int64 }

// table is one message table as the indexer reads it.
type table struct {
	scope, name string
	sel         string // the columns scanDocs reads, in its order
}

var tables = []table{
	{ScopeGroup, "telegram_group_messages", "m.id, m.chat_id, m.message_id, " +
		"m.sender_user_id, m.date, m.edit_date, m.deleted_at IS NOT NULL, " +
		"m.content_type, COALESCE(m.text, ''), 0"},
	{ScopePrivate, "telegram_private_messages", "m.id, m.chat_id, m.message_id, " +
		"m.sender_id, m.date, m.edit_date, m.deleted_at IS NOT NULL, " +
		"m.content_type, COALESCE(m.text, ''), m.is_outgoing"},
}

// tableOf is the table of a chat: groups have negative chat ids.
func tableOf(chat int64) *table {
	if chat < 0 {
		return &tables[0]
	}
	return &tables[1]
}

// hint bounds every statement of the indexer, so a slow one gives up
// instead of holding a connection the pages need.
const hint = "SELECT /*+ MAX_EXECUTION_TIME(10000) */ "

func scanDocs(rs *sql.Rows, scope string) ([]Doc, error) {
	defer rs.Close()
	var out []Doc
	for rs.Next() {
		d := Doc{Scope: scope}
		var sender sql.NullInt64
		var ctype sql.NullString
		if err := rs.Scan(&d.RowID, &d.ChatID, &d.MessageID, &sender, &d.Date, &d.EditDate,
			&d.Deleted, &ctype, &d.Text, &d.Outgoing); err != nil {
			return nil, err
		}
		if sender.Valid {
			d.SenderID = &sender.Int64
		}
		d.ContentType = ctype.String
		out = append(out, d)
	}
	return out, rs.Err()
}

func (ix *Indexer) query(ctx context.Context, t *table, where string, args ...any) ([]Doc, error) {
	rs, err := ix.db.QueryContext(ctx, hint+t.sel+" FROM "+t.name+" m WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	return scanDocs(rs, t.scope)
}

// ops are the writes for rows read again: a row without text has no
// document (it may have had one before an edit).
func ops(docs []Doc) []Op {
	out := make([]Op, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		if strings.TrimSpace(d.Text) == "" {
			out = append(out, Op{ID: DocID(d.Scope, d.RowID), Delete: true})
			continue
		}
		out = append(out, Op{ID: DocID(d.Scope, d.RowID), Doc: d})
	}
	return out
}

// Run keeps the index in step until ctx ends. It returns only then:
// an engine or database that fails is retried.
func (ix *Indexer) Run(ctx context.Context) {
	log := ix.o.Log
	if !ix.retry(ctx, "preparing the index", func() error {
		if err := ix.x.Ensure(ctx); err != nil {
			return err
		}
		st, err := ix.x.LoadState(ctx)
		ix.st = st
		return err
	}) {
		return
	}
	if ix.st == (State{}) {
		// A new index: the first reconcile looks for edits and
		// deletions from the backfill's start on.
		ix.st.ReconciledAt = ix.now().Unix()
		log.Info("search index: starting the backfill", "index", ix.x.Name())
	}
	if ix.src != nil {
		go ix.stream(ctx)
	}
	// Once the catch-up is done, reconcile at once: what happened while
	// the service was down.
	ix.reconcile = ix.now()

	flush := time.NewTicker(ix.o.Flush)
	defer flush.Stop()
	step := time.NewTimer(0)
	defer step.Stop()
	for {
		select {
		case <-ctx.Done():
			ix.save(context.Background(), true)
			return
		case e := <-ix.events:
			ix.event(e)
			if len(ix.pending) >= 500 {
				ix.flush(ctx)
			}
		case <-flush.C:
			ix.flush(ctx)
			ix.heartbeat(ctx)
			if ix.done() && !ix.now().Before(ix.reconcile) {
				ix.runReconcile(ctx)
			}
		case <-step.C:
			if ix.done() {
				continue
			}
			wait := ix.o.Pause
			if err := ix.catchUpStep(ctx); err != nil && ctx.Err() == nil {
				log.Warn("search index: catch-up", "err", err)
				wait = 10 * time.Second
			}
			if !ix.done() {
				step.Reset(wait)
			} else {
				log.Info("search index: caught up", "index", ix.x.Name(),
					"group_next_id", ix.st.Group.NextID, "private_next_id", ix.st.Private.NextID)
			}
		}
	}
}

// retry runs f until it succeeds or ctx ends (false).
func (ix *Indexer) retry(ctx context.Context, what string, f func() error) bool {
	wait := time.Second
	for {
		err := f()
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		ix.o.Log.Warn("search index: "+what, "err", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
		wait = min(wait*2, time.Minute)
	}
}

// done is whether the catch-up pass reached the end of both tables.
func (ix *Indexer) done() bool { return ix.st.Group.Done && ix.st.Private.Done }

// stream keeps the event source connected, resuming after the last seq
// seen, until ctx ends.
func (ix *Indexer) stream(ctx context.Context) {
	var since uint64
	wait := time.Second
	for ctx.Err() == nil {
		got := false
		err := ix.src.Stream(ctx, since, func(e live.Event) {
			got = true
			if e.Seq > since {
				since = e.Seq
			}
			switch {
			case e.Kind == live.KindHello || e.Kind == live.KindResync:
				ix.up.Store(true)
			case !isMessageEvent(e.Kind):
				return
			}
			select {
			case ix.events <- e:
			default:
				// The indexer is behind (a reconcile or a slow
				// engine): the reconcile repairs what is dropped.
				ix.overflow.Store(true)
			}
		})
		ix.up.Store(false)
		if ctx.Err() != nil {
			return
		}
		if got {
			wait = time.Second
		}
		ix.o.Log.Debug("search index: event stream ended", "err", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, 30*time.Second)
	}
}

// event takes one event of the stream.
func (ix *Indexer) event(e live.Event) {
	switch e.Kind {
	case live.KindResync:
		// Events were missed: reconcile soon.
		ix.reconcile = ix.now()
	case live.KindMessageNew, live.KindMessageEdited, live.KindMessageDeleted:
		if e.ChatID != 0 && e.MessageID != 0 {
			ix.pending[msgKey{e.ChatID, e.MessageID}] = struct{}{}
		}
	}
}

// maxPending bounds the events kept while the engine or MySQL fails;
// past it they are dropped and the reconcile repairs them.
const maxPending = 100000

// flush reads the rows of the pending events and writes their
// documents. On failure they stay pending for the next flush.
func (ix *Indexer) flush(ctx context.Context) {
	if ix.overflow.Swap(false) {
		ix.reconcile = ix.now()
	}
	if len(ix.pending) == 0 {
		return
	}
	by := map[*table][]msgKey{}
	for k := range ix.pending {
		t := tableOf(k.chat)
		by[t] = append(by[t], k)
	}
	var all []Op
	for t, keys := range by {
		for len(keys) > 0 {
			n := min(len(keys), 500)
			args := make([]any, 0, 2*n)
			for _, k := range keys[:n] {
				args = append(args, k.chat, k.msg)
			}
			docs, err := ix.query(ctx, t, "(m.chat_id, m.message_id) IN ("+
				strings.TrimSuffix(strings.Repeat("(?, ?), ", n), ", ")+")", args...)
			if err != nil {
				ix.flushFailed(err)
				return
			}
			all = append(all, ops(docs)...)
			keys = keys[n:]
		}
	}
	if err := ix.x.Bulk(ctx, all); err != nil {
		ix.flushFailed(err)
		return
	}
	clear(ix.pending)
}

func (ix *Indexer) flushFailed(err error) {
	ix.o.Log.Warn("search index: writing the events' rows", "err", err, "pending", len(ix.pending))
	if len(ix.pending) > maxPending {
		clear(ix.pending)
		ix.reconcile = ix.now()
	}
}

// heartbeat records that the index is in step while the stream
// delivers and nothing waits: searches check SyncedAt.
func (ix *Indexer) heartbeat(ctx context.Context) {
	if ix.up.Load() && len(ix.pending) == 0 && ix.done() {
		ix.st.SyncedAt = ix.now().Unix()
	}
	ix.save(ctx, false)
}

// stateEvery is how often the state is written when only SyncedAt
// moved.
const stateEvery = 30 * time.Second

// save writes the state when forced or when stateEvery has passed.
func (ix *Indexer) save(ctx context.Context, force bool) {
	if !force && ix.now().Sub(ix.saved) < stateEvery {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ix.x.SaveState(ctx, ix.st); err != nil {
		ix.o.Log.Warn("search index: saving the state", "err", err)
		return
	}
	ix.saved = ix.now()
}

// catchUpStep reads one batch of each table not caught up yet, above
// its NextID, and writes their documents. A row without text is
// skipped: it has no document, and the reconcile removes one an edit
// left behind.
func (ix *Indexer) catchUpStep(ctx context.Context) error {
	for i := range tables {
		t := &tables[i]
		ts := ix.st.table(t.scope)
		if ts.Done {
			continue
		}
		n, err := ix.catchUp(ctx, t, ts)
		if err != nil {
			return err
		}
		if n < ix.o.Batch {
			ts.Done = true
		}
		ix.save(ctx, true)
	}
	return nil
}

// catchUp reads and writes one batch of t above ts.NextID, moving it
// on, and returns the rows read.
func (ix *Indexer) catchUp(ctx context.Context, t *table, ts *TableState) (int, error) {
	docs, err := ix.query(ctx, t, "m.id > ? ORDER BY m.id LIMIT ?", ts.NextID, ix.o.Batch)
	if err != nil {
		return 0, err
	}
	var writes []Op
	for _, op := range ops(docs) {
		if !op.Delete {
			writes = append(writes, op)
		}
	}
	if err := ix.x.Bulk(ctx, writes); err != nil {
		return 0, err
	}
	if len(docs) > 0 {
		ts.NextID = docs[len(docs)-1].RowID
	}
	return len(docs), nil
}

// runReconcile reconciles and schedules the next one.
func (ix *Indexer) runReconcile(ctx context.Context) {
	if err := ix.Reconcile(ctx); err != nil && ctx.Err() == nil {
		ix.o.Log.Warn("search index: reconcile", "err", err)
		ix.reconcile = ix.now().Add(time.Minute)
		return
	}
	every := ix.o.Reconcile
	if !ix.up.Load() {
		every = min(every, ix.o.NoStream)
	}
	ix.reconcile = ix.now().Add(every)
}

// Reconcile repairs what the stream missed: the rows above NextID, and
// the rows deleted or edited since the last reconcile began (less the
// margin). Run's goroutine calls it; tests call it directly instead
// of Run.
func (ix *Indexer) Reconcile(ctx context.Context) error {
	start := ix.now()
	for i := range tables {
		t := &tables[i]
		ts := ix.st.table(t.scope)
		for {
			n, err := ix.catchUp(ctx, t, ts)
			if err != nil {
				return err
			}
			if n < ix.o.Batch {
				break
			}
			if err := ix.pause(ctx); err != nil {
				return err
			}
		}
		ts.Done = true
	}
	since := ix.st.ReconciledAt - int64(ix.o.Margin/time.Second)
	window := max(start.Unix()-since, 1)
	for i := range tables {
		t := &tables[i]
		// Deleted: deleted_at is when the daemon saw it, in the
		// server's clock, so the window is counted there too.
		if err := ix.repair(ctx, t, "m.deleted_at >= NOW() - INTERVAL ? SECOND", window); err != nil {
			return fmt.Errorf("%s, deleted rows: %w", t.name, err)
		}
		if err := ix.repairEdits(ctx, t, since); err != nil {
			return fmt.Errorf("%s, edited rows: %w", t.name, err)
		}
	}
	// What the stream sent meanwhile is written by the next flush; a
	// dropped event asks for another reconcile (flush).
	ix.st.ReconciledAt = start.Unix()
	ix.st.SyncedAt = start.Unix()
	ix.o.Log.Debug("search index: reconciled", "index", ix.x.Name())
	ix.save(ctx, true)
	return nil
}

func (ix *Indexer) pause(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(ix.o.Pause):
		return nil
	}
}

// repair writes the documents of the rows matching cond, a batch at a
// time in id order.
func (ix *Indexer) repair(ctx context.Context, t *table, cond string, args ...any) error {
	var after int64
	for {
		a := append(append([]any{}, args...), after, ix.o.Batch)
		docs, err := ix.query(ctx, t, cond+" AND m.id > ? ORDER BY m.id LIMIT ?", a...)
		if err != nil {
			return err
		}
		if err := ix.x.Bulk(ctx, ops(docs)); err != nil {
			return err
		}
		if len(docs) < ix.o.Batch {
			return nil
		}
		after = docs[len(docs)-1].RowID
		if err := ix.pause(ctx); err != nil {
			return err
		}
	}
}

// chatsPerProbe is how many chats one edit probe asks for, each a
// range of the (chat_id, edit_date) index.
const chatsPerProbe = 200

// repairEdits rewrites the rows of t edited at or after since (Unix
// seconds), chat by chat through the (chat_id, edit_date) index: there
// is none on edit_date alone.
func (ix *Indexer) repairEdits(ctx context.Context, t *table, since int64) error {
	q := "SELECT id FROM telegram_groups"
	if t.scope == ScopePrivate {
		q = "SELECT DISTINCT chat_id FROM telegram_private_messages"
	}
	rs, err := ix.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	var chats []int64
	for rs.Next() {
		var c int64
		if err := rs.Scan(&c); err != nil {
			rs.Close()
			return err
		}
		chats = append(chats, c)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for len(chats) > 0 {
		n := min(len(chats), chatsPerProbe)
		in := make([]string, n)
		for i, c := range chats[:n] {
			// Ids read from the database itself, as numbers.
			in[i] = strconv.FormatInt(c, 10)
		}
		cond := "m.chat_id IN (" + strings.Join(in, ", ") + ") AND m.edit_date >= ?"
		if err := ix.repair(ctx, t, cond, since); err != nil {
			return err
		}
		chats = chats[n:]
	}
	return nil
}

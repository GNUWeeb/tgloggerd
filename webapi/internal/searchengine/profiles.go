// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"tgloggerd/webapi/internal/live"
)

// The profile indices: users' bios and groups' descriptions, an index
// each beside the messages' one, so adding or rebuilding them never
// touches the messages index (webapi/README, "Search engine").
//
// A document is one row with a non-empty text: its id (the user id, or
// the group's chat id) and the text, under the same analyzers as the
// messages' text, so a search means the same in every index. Its
// document id is the row id in decimal.
//
// Unlike the message tables, the profile tables are small (a few tens
// of thousands of rows with a text) and their rows change in place:
// a new bio is an old user's row rewritten, and a user whose extra
// info falls back to its defaults loses the row. So the catch-up pass
// of the messages, which reads the rows above the last id seen, does
// not apply; the profile indexer instead compares, a batch of ids at a
// time, MySQL's rows with the index's documents of the same id range
// and writes only what differs: a missing or changed document is
// written, one whose row has no text any more is deleted. Such a pass
// over the whole table is the backfill of an empty index and, every
// reconcile, the repair of whatever the event stream missed.

// The profile kinds, by the entity each answers.
const (
	ProfileUsers  = "users"
	ProfileGroups = "groups"
)

// profileVersion is the profile indices' layout version (_meta), as
// mappingVersion is the messages index's.
const profileVersion = 1

// ProfileDoc is the document of one user or group.
type ProfileDoc struct {
	RowID int64  `json:"row_id"`
	Text  string `json:"text"`
}

// profileDocID is the document id of a row.
func profileDocID(row int64) string { return strconv.FormatInt(row, 10) }

// profileSettings is a profile index's settings and mapping: the
// messages' analyzers (indexSettings) over a row id and a text.
func profileSettings() map[string]any {
	return textIndexSettings(profileVersion, map[string]any{
		"row_id": map[string]any{"type": "long"},
		"text": map[string]any{"type": "text", "analyzer": "message_text",
			"search_analyzer": "message_search"},
	})
}

// ProfileIndex is the index of this name for kind (ProfileUsers or
// ProfileGroups), named as Index names one.
func (c *Client) ProfileIndex(name, kind string) (*Index, error) {
	if kind != ProfileUsers && kind != ProfileGroups {
		return nil, fmt.Errorf("no profile kind %q", kind)
	}
	x, err := c.Index(name)
	if err != nil {
		return nil, err
	}
	x.kind = kind
	return x, nil
}

// ProfileState is a profile indexer's progress, its state index's one
// document.
type ProfileState struct {
	// InPass: a pass over the table is under way; Next is the last row
	// id it has compared, so a restart resumes it there.
	InPass bool  `json:"in_pass"`
	Next   int64 `json:"next"`
	// PassStart is when the pass under way began (Unix seconds).
	PassStart int64 `json:"pass_start"`
	// Done: a pass has reached the table's end at least once, so every
	// row is in the index.
	Done bool `json:"done"`
	// SyncedAt is when the index was last known in step with MySQL
	// (Unix seconds): the event stream delivering, or a pass ending
	// (as of its start). Searches fall back to MySQL when it is too old.
	SyncedAt int64 `json:"synced_at"`
}

// LoadProfileState reads a profile index's state; a missing one is the
// zero ProfileState.
func (x *Index) LoadProfileState(ctx context.Context) (ProfileState, error) {
	var s ProfileState
	err := x.loadState(ctx, &s)
	return s, err
}

// SaveProfileState writes a profile index's state.
func (x *Index) SaveProfileState(ctx context.Context, s ProfileState) error {
	return x.saveState(ctx, s)
}

// profileTable is the table of a profile kind as the indexer reads it.
type profileTable struct {
	name, id, text string
	// fieldsOf is whether an update event naming these fields may have
	// changed the text; nil fields (no list) may have changed anything.
	fields string
	// event is the update event's kind, and its id the row's.
	event string
	idOf  func(live.Event) int64
}

var profileTables = map[string]*profileTable{
	ProfileUsers: {name: "telegram_user_extra_info", id: "user_id", text: "bio",
		fields: "bio", event: live.KindUserUpdated,
		idOf: func(e live.Event) int64 { return e.UserID }},
	ProfileGroups: {name: "`telegram_groups`", id: "id", text: "description",
		event: live.KindGroupUpdated,
		idOf:  func(e live.Event) int64 { return e.ChatID }},
}

// wants is whether e may have changed a row's text, and which row.
func (t *profileTable) wants(e live.Event) (int64, bool) {
	if e.Kind != t.event {
		return 0, false
	}
	id := t.idOf(e)
	if id == 0 {
		return 0, false
	}
	if e.Fields == nil {
		return id, true
	}
	for _, f := range e.Fields {
		if t.fields != "" && f == t.fields {
			return id, true
		}
	}
	return 0, false
}

// ProfileEvent is whether the profile indexers act on events of kind;
// a HubSource for them filters by it.
func ProfileEvent(kind string) bool {
	return kind == live.KindUserUpdated || kind == live.KindGroupUpdated
}

// ProfileIndexer keeps one profile index in step with its table.
type ProfileIndexer struct {
	db  *sql.DB
	x   *Index
	t   *profileTable
	src live.Source // nil: no event stream, the passes alone
	o   IndexerOptions
	now func() time.Time

	events   chan live.Event
	up       atomic.Bool
	overflow atomic.Bool

	// Owned by Run's goroutine.
	st      ProfileState
	saved   time.Time
	pending map[int64]struct{}
	next    time.Time // when the next pass is due
}

// NewProfileIndexer is an indexer reading db (the logger's schema) into
// x, a ProfileIndex, fed by src (nil for none). Of o, Batch, Pause,
// Reconcile (the time between two passes), NoStream, Flush and Log
// apply.
func NewProfileIndexer(db *sql.DB, x *Index, src live.Source, o IndexerOptions) (*ProfileIndexer, error) {
	t := profileTables[x.kind]
	if t == nil {
		return nil, fmt.Errorf("index %s is no profile index", x.Name())
	}
	o.defaults()
	return &ProfileIndexer{
		db: db, x: x, t: t, src: src, o: o, now: time.Now,
		events:  make(chan live.Event, 4096),
		pending: map[int64]struct{}{},
	}, nil
}

// Run keeps the index in step until ctx ends; failures are retried.
func (px *ProfileIndexer) Run(ctx context.Context) {
	log := px.o.Log
	if !retry(ctx, log, "preparing the index", func() error {
		if err := px.x.Ensure(ctx); err != nil {
			return err
		}
		st, err := px.x.LoadProfileState(ctx)
		px.st = st
		return err
	}) {
		return
	}
	if !px.st.Done && !px.st.InPass {
		log.Info("search index: starting the backfill", "index", px.x.Name())
	}
	if px.src != nil {
		go follow(ctx, px.src, log, ProfileEvent, &px.up, func(e live.Event) {
			select {
			case px.events <- e:
			default:
				px.overflow.Store(true)
			}
		})
	}
	// A pass at once: the backfill, or what changed while the service
	// was down.
	px.next = px.now()

	flush := time.NewTicker(px.o.Flush)
	defer flush.Stop()
	step := time.NewTimer(0)
	defer step.Stop()
	for {
		select {
		case <-ctx.Done():
			px.save(context.Background(), true)
			return
		case e := <-px.events:
			px.event(e)
			if len(px.pending) >= 500 {
				px.flush(ctx)
			}
		case <-flush.C:
			px.flush(ctx)
			px.heartbeat(ctx)
		case <-step.C:
			wait := px.o.Pause
			if px.st.InPass || !px.now().Before(px.next) {
				if err := px.passStep(ctx); err != nil && ctx.Err() == nil {
					log.Warn("search index: pass", "index", px.x.Name(), "err", err)
					wait = 10 * time.Second
				}
			}
			if !px.st.InPass {
				wait = max(min(px.next.Sub(px.now()), time.Second), px.o.Pause)
			}
			step.Reset(wait)
		}
	}
}

// event takes one event of the stream.
func (px *ProfileIndexer) event(e live.Event) {
	if e.Kind == live.KindResync {
		// Events were missed: a pass soon.
		px.next = px.now()
		return
	}
	if id, ok := px.t.wants(e); ok {
		px.pending[id] = struct{}{}
	}
}

// flush reads the rows of the pending events and writes their
// documents; a row without text, or gone, loses its document. On
// failure they stay pending.
func (px *ProfileIndexer) flush(ctx context.Context) {
	if px.overflow.Swap(false) {
		px.next = px.now()
	}
	if len(px.pending) == 0 {
		return
	}
	ids := make([]int64, 0, len(px.pending))
	for id := range px.pending {
		ids = append(ids, id)
	}
	var ops []Op
	for len(ids) > 0 {
		n := min(len(ids), 500)
		rows, err := px.byIDs(ctx, ids[:n])
		if err != nil {
			px.flushFailed(err)
			return
		}
		for _, id := range ids[:n] {
			if text := rows[id]; strings.TrimSpace(text) != "" {
				ops = append(ops, Op{ID: profileDocID(id), Profile: &ProfileDoc{RowID: id, Text: text}})
			} else {
				ops = append(ops, Op{ID: profileDocID(id), Delete: true})
			}
		}
		ids = ids[n:]
	}
	if err := px.x.Bulk(ctx, ops); err != nil {
		px.flushFailed(err)
		return
	}
	clear(px.pending)
}

func (px *ProfileIndexer) flushFailed(err error) {
	px.o.Log.Warn("search index: writing the events' rows", "index", px.x.Name(), "err", err,
		"pending", len(px.pending))
	if len(px.pending) > maxPending {
		clear(px.pending)
		px.next = px.now()
	}
}

// byIDs reads the texts of these rows; a row missing from the answer
// has none.
func (px *ProfileIndexer) byIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	in := make([]string, len(ids))
	for i, id := range ids {
		// Ids of events, parsed as numbers.
		in[i] = strconv.FormatInt(id, 10)
	}
	rs, err := px.db.QueryContext(ctx, hint+px.t.id+", "+px.t.text+" FROM "+px.t.name+
		" WHERE "+px.t.id+" IN ("+strings.Join(in, ", ")+")")
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]string{}
	for rs.Next() {
		var id int64
		var text sql.NullString
		if err := rs.Scan(&id, &text); err != nil {
			return nil, err
		}
		out[id] = text.String
	}
	return out, rs.Err()
}

// heartbeat records that the index is in step while the stream
// delivers and nothing waits.
func (px *ProfileIndexer) heartbeat(ctx context.Context) {
	if px.up.Load() && len(px.pending) == 0 && px.st.Done {
		px.st.SyncedAt = px.now().Unix()
	}
	px.save(ctx, false)
}

func (px *ProfileIndexer) save(ctx context.Context, force bool) {
	if !force && px.now().Sub(px.saved) < stateEvery {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := px.x.SaveProfileState(ctx, px.st); err != nil {
		px.o.Log.Warn("search index: saving the state", "index", px.x.Name(), "err", err)
		return
	}
	px.saved = px.now()
}

// Pass runs one whole pass over the table, as the reconcile does; for
// tests, instead of Run.
func (px *ProfileIndexer) Pass(ctx context.Context) error {
	if err := px.passStep(ctx); err != nil {
		return err
	}
	for px.st.InPass {
		if err := sleep(ctx, px.o.Pause); err != nil {
			return err
		}
		if err := px.passStep(ctx); err != nil {
			return err
		}
	}
	return nil
}

// passStep compares one batch of rows with the index's documents of
// the same id range and writes the difference, starting a pass if
// none is under way. The range runs from the last id compared to the
// batch's last row, or to the end at the table's end, so a document
// whose row lost its text or went away is in some batch's range.
func (px *ProfileIndexer) passStep(ctx context.Context) error {
	if !px.st.InPass {
		px.st = ProfileState{InPass: true, Next: math.MinInt64, PassStart: px.now().Unix(),
			Done: px.st.Done, SyncedAt: px.st.SyncedAt}
	}
	t := px.t
	rs, err := px.db.QueryContext(ctx, hint+t.id+", "+t.text+" FROM "+t.name+" WHERE "+t.id+
		" > ? AND "+t.text+" <> '' ORDER BY "+t.id+" LIMIT ?", px.st.Next, px.o.Batch)
	if err != nil {
		return err
	}
	var rows []ProfileDoc
	for rs.Next() {
		var d ProfileDoc
		if err := rs.Scan(&d.RowID, &d.Text); err != nil {
			rs.Close()
			return err
		}
		rows = append(rows, d)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	end := len(rows) < px.o.Batch
	hi := int64(math.MaxInt64)
	if !end {
		hi = rows[len(rows)-1].RowID
	}
	have, err := px.x.profileTexts(ctx, px.st.Next, hi)
	if err != nil {
		return err
	}
	var ops []Op
	for i := range rows {
		d := &rows[i]
		text, ok := have[d.RowID]
		delete(have, d.RowID)
		if strings.TrimSpace(d.Text) == "" {
			if ok {
				ops = append(ops, Op{ID: profileDocID(d.RowID), Delete: true})
			}
			continue
		}
		if !ok || text != d.Text {
			ops = append(ops, Op{ID: profileDocID(d.RowID), Profile: d})
		}
	}
	for id := range have {
		// A document whose row has no text any more, or no row.
		ops = append(ops, Op{ID: profileDocID(id), Delete: true})
	}
	if err := px.x.Bulk(ctx, ops); err != nil {
		return err
	}
	px.st.Next = hi
	if end {
		px.st.InPass, px.st.Done = false, true
		px.st.SyncedAt = px.st.PassStart
		every := px.o.Reconcile
		if px.src == nil || !px.up.Load() {
			every = min(every, px.o.NoStream)
		}
		px.next = px.now().Add(every)
		px.o.Log.Debug("search index: pass done", "index", px.x.Name())
	}
	px.save(ctx, true)
	return nil
}

// profileTexts are the documents of a profile index with a row id in
// (after, upTo], by row id.
func (x *Index) profileTexts(ctx context.Context, after, upTo int64) (map[int64]string, error) {
	out := map[int64]string{}
	var from []any
	for {
		body := map[string]any{
			"query": map[string]any{"range": map[string]any{"row_id": map[string]any{
				"gt": after, "lte": upTo}}},
			"sort":             []any{map[string]any{"row_id": "asc"}},
			"size":             1000,
			"_source":          []string{"row_id", "text"},
			"track_total_hits": false,
		}
		if from != nil {
			body["search_after"] = from
		}
		var ans struct {
			Hits struct {
				Hits []struct {
					Source ProfileDoc `json:"_source"`
					Sort   []any      `json:"sort"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if err := x.c.do(ctx, http.MethodPost, "/"+seg(x.name)+"/_search", body, &ans); err != nil {
			return nil, err
		}
		for _, h := range ans.Hits.Hits {
			out[h.Source.RowID] = h.Source.Text
		}
		if len(ans.Hits.Hits) < 1000 {
			return out, nil
		}
		from = ans.Hits.Hits[len(ans.Hits.Hits)-1].Sort
	}
}

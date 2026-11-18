// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// The index: one document per message row that has text, group and
// private messages side by side, told apart by Scope. A document
// carries what the scope filter needs (scope, chat_id) and the few
// columns a search often adds to its text (sender, date, content type,
// deleted), so the engine can narrow its answer by them too; MySQL
// still applies every one of them to the rows it reads.
//
// Beside it, <name>-state holds one document, the indexer's progress
// (State), so a restart resumes the backfill where it stopped, and
// losing the engine's data restarts it from the beginning, as it
// should.

// The scopes of a document.
const (
	ScopeGroup   = "group"
	ScopePrivate = "private"
)

// mappingVersion is the index layout's version, in the mapping's
// _meta. An index made with another one is not used: delete it (and its
// -state index) and the indexer rebuilds it.
const mappingVersion = 1

// maxResultWindow lets a search page as deep as the MySQL search may
// (logdb.MaxOffset plus a page).
const maxResultWindow = 501000

// Doc is the document of one message row.
type Doc struct {
	Scope     string `json:"scope"`
	RowID     int64  `json:"row_id"`
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	// SenderID is the sending user: sender_user_id of a group message,
	// sender_id of a private one; absent when there is none.
	SenderID    *int64 `json:"sender_id,omitempty"`
	Date        int64  `json:"date"`
	EditDate    int64  `json:"edit_date"`
	Deleted     bool   `json:"deleted"`
	ContentType string `json:"content_type"`
	Outgoing    bool   `json:"outgoing"`
	Text        string `json:"text"`
}

// DocID is the document id of a row: the scope's letter and the row
// id, so writing the same row again replaces its document.
func DocID(scope string, rowID int64) string {
	if scope == ScopePrivate {
		return "p-" + strconv.FormatInt(rowID, 10)
	}
	return "g-" + strconv.FormatInt(rowID, 10)
}

// parseDocID is DocID's inverse.
func parseDocID(id string) (scope string, rowID int64, ok bool) {
	p, n, found := strings.Cut(id, "-")
	if !found {
		return "", 0, false
	}
	row, err := strconv.ParseInt(n, 10, 64)
	if err != nil || row <= 0 {
		return "", 0, false
	}
	switch p {
	case "g":
		return ScopeGroup, row, true
	case "p":
		return ScopePrivate, row, true
	}
	return "", 0, false
}

// indexSettings is the messages index's settings and mapping. Text is
// split on Unicode word boundaries, lowercased and folded to ASCII, so
// "Kernel", "kernel" and an accented "kernel" match; nothing language-specific, as
// the archive mixes languages.
func indexSettings() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
			"max_result_window":  maxResultWindow,
			"analysis": map[string]any{
				"analyzer": map[string]any{
					"message_text": map[string]any{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "asciifolding"},
					},
				},
			},
		},
		"mappings": map[string]any{
			"dynamic": "strict",
			"_meta":   map[string]any{"version": mappingVersion},
			"properties": map[string]any{
				"scope":        map[string]any{"type": "keyword"},
				"row_id":       map[string]any{"type": "long"},
				"chat_id":      map[string]any{"type": "long"},
				"message_id":   map[string]any{"type": "long"},
				"sender_id":    map[string]any{"type": "long"},
				"date":         map[string]any{"type": "long"},
				"edit_date":    map[string]any{"type": "long"},
				"deleted":      map[string]any{"type": "boolean"},
				"content_type": map[string]any{"type": "keyword"},
				"outgoing":     map[string]any{"type": "boolean"},
				"text":         map[string]any{"type": "text", "analyzer": "message_text"},
			},
		},
	}
}

// Index is the messages index of one name, and its state.
type Index struct {
	c    *Client
	name string
}

var indexName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,200}$`)

// Index is the index of this name: lowercase letters, digits, '_', '.'
// and '-', not starting with a symbol.
func (c *Client) Index(name string) (*Index, error) {
	if !indexName.MatchString(name) || strings.HasSuffix(name, "-state") {
		return nil, fmt.Errorf("%q is not a usable index name", name)
	}
	return &Index{c: c, name: name}, nil
}

// Name is the index's name.
func (x *Index) Name() string { return x.name }

func (x *Index) stateName() string { return x.name + "-state" }

// Ensure creates the index and its state index when they are missing,
// and refuses an index made with another layout.
func (x *Index) Ensure(ctx context.Context) error {
	var m map[string]struct {
		Mappings struct {
			Meta struct {
				Version int `json:"version"`
			} `json:"_meta"`
		} `json:"mappings"`
	}
	err := x.c.do(ctx, http.MethodGet, "/"+seg(x.name)+"/_mapping", nil, &m)
	switch {
	case IsNotFound(err):
		if err := x.create(ctx, x.name, indexSettings()); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if v := m[x.name].Mappings.Meta.Version; v != mappingVersion {
			return fmt.Errorf("index %s has layout version %d, not %d: delete it and %s "+
				"to have it rebuilt", x.name, v, mappingVersion, x.stateName())
		}
	}
	err = x.c.do(ctx, http.MethodHead, "/"+seg(x.stateName()), nil, nil)
	if IsNotFound(err) {
		return x.create(ctx, x.stateName(), map[string]any{
			"settings": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
			"mappings": map[string]any{"dynamic": false},
		})
	}
	return err
}

func (x *Index) create(ctx context.Context, name string, body any) error {
	err := x.c.do(ctx, http.MethodPut, "/"+seg(name), body, nil)
	var e *Error
	if errors.As(err, &e) && e.Type == "resource_already_exists_exception" {
		// Another instance made it first.
		return nil
	}
	return err
}

// Drop deletes the index and its state, for tests.
func (x *Index) Drop(ctx context.Context) error {
	for _, n := range []string{x.name, x.stateName()} {
		if err := x.c.do(ctx, http.MethodDelete, "/"+seg(n), nil, nil); err != nil && !IsNotFound(err) {
			return err
		}
	}
	return nil
}

// Refresh makes what was written searchable now instead of within the
// index's refresh interval (a second), for tests.
func (x *Index) Refresh(ctx context.Context) error {
	return x.c.do(ctx, http.MethodPost, "/"+seg(x.name)+"/_refresh", nil, nil)
}

// Op is one write of a bulk: index Doc under ID, or delete ID.
type Op struct {
	ID     string
	Doc    *Doc // nil: delete
	Delete bool
}

// bulkItem is one write's result in a bulk answer.
type bulkItem struct {
	Status int             `json:"status"`
	Error  json.RawMessage `json:"error"`
}

// Bulk applies the writes in one request. Every write is idempotent
// (index replaces, delete of a missing document is fine), so a bulk
// that failed in part is simply sent again.
func (x *Index) Bulk(ctx context.Context, ops []Op) error {
	if len(ops) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, op := range ops {
		meta := map[string]any{"_index": x.name, "_id": op.ID}
		if op.Delete || op.Doc == nil {
			if err := enc.Encode(map[string]any{"delete": meta}); err != nil {
				return err
			}
			continue
		}
		if err := enc.Encode(map[string]any{"index": meta}); err != nil {
			return err
		}
		if err := enc.Encode(op.Doc); err != nil {
			return err
		}
	}
	var ans struct {
		Errors bool                  `json:"errors"`
		Items  []map[string]bulkItem `json:"items"`
	}
	if err := x.c.do(ctx, http.MethodPost, "/_bulk", buf.Bytes(), &ans); err != nil {
		return err
	}
	if !ans.Errors {
		return nil
	}
	for _, it := range ans.Items {
		for action, r := range it {
			if action == "delete" && r.Status == http.StatusNotFound {
				continue
			}
			if r.Status/100 != 2 {
				return fmt.Errorf("search engine: bulk %s: HTTP %d: %s", action, r.Status, r.Error)
			}
		}
	}
	return nil
}

// TableState is the indexer's progress through one message table.
type TableState struct {
	// NextID: rows with an id above it are still to be read by the
	// catch-up pass (the backfill, then the reconcile's new rows).
	NextID int64 `json:"next_id"`
	// Done: the catch-up pass reached the table's end at least once,
	// so every row up to NextID is in the index.
	Done bool `json:"done"`
}

// State is the indexer's progress, the state index's one document.
type State struct {
	Group   TableState `json:"group"`
	Private TableState `json:"private"`
	// SyncedAt is when the index was last known to be in step with
	// MySQL (Unix seconds): the event stream delivering, or a
	// reconcile finishing. Searches fall back to MySQL when it is too
	// old (Engine.MaxLag).
	SyncedAt int64 `json:"synced_at"`
	// ReconciledAt is when the last reconcile started (Unix seconds);
	// the next one looks for edits and deletions since then.
	ReconciledAt int64 `json:"reconciled_at"`
}

// table is the state of one scope's table.
func (s *State) table(scope string) *TableState {
	if scope == ScopePrivate {
		return &s.Private
	}
	return &s.Group
}

const stateID = "state"

// LoadState reads the indexer's state; a missing one is the zero State
// (nothing indexed yet).
func (x *Index) LoadState(ctx context.Context) (State, error) {
	var d struct {
		Found  bool  `json:"found"`
		Source State `json:"_source"`
	}
	err := x.c.do(ctx, http.MethodGet, "/"+seg(x.stateName())+"/_doc/"+stateID, nil, &d)
	if IsNotFound(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	return d.Source, nil
}

// SaveState writes the indexer's state.
func (x *Index) SaveState(ctx context.Context, s State) error {
	return x.c.do(ctx, http.MethodPut, "/"+seg(x.stateName())+"/_doc/"+stateID, s, nil)
}

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

	"tgloggerd/webapi/internal/live"
	"tgloggerd/webapi/internal/testdb"
)

// The profile indices on the logger fixture "sample": bios of the users
// 9299000001, 9299000003 and 9299000007, and the descriptions of its
// groups.

// testProfileIndex is a profile index of the test's own, as testIndex.
func testProfileIndex(t *testing.T, kind string) *Index {
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
	x, err := c.ProfileIndex("test-"+kind+"-"+hex.EncodeToString(rnd[:]), kind)
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

// profileDoc is the document of a row and its version, if there is one.
func profileDoc(t *testing.T, x *Index, row int64) (ProfileDoc, int, bool) {
	t.Helper()
	var d struct {
		Found   bool       `json:"found"`
		Version int        `json:"_version"`
		Source  ProfileDoc `json:"_source"`
	}
	err := x.c.do(context.Background(), http.MethodGet, "/"+seg(x.name)+"/_doc/"+
		profileDocID(row), nil, &d)
	if IsNotFound(err) {
		return ProfileDoc{}, 0, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return d.Source, d.Version, d.Found
}

func profileCount(t *testing.T, x *Index) int64 {
	t.Helper()
	if err := x.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	return count(t, x, map[string]any{"match_all": map[string]any{}})
}

func TestProfileIndexKinds(t *testing.T) {
	t.Parallel()
	c, err := NewClient("http://opensearch:9200")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ProfileIndex("x", "messages"); err == nil {
		t.Error("a profile index of no kind")
	}
	x, err := c.Index("x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProfileIndexer(nil, x, nil, IndexerOptions{}); err == nil {
		t.Error("a profile indexer on the messages index")
	}
	if !ProfileEvent(live.KindUserUpdated) || !ProfileEvent(live.KindGroupUpdated) ||
		ProfileEvent(live.KindMessageNew) {
		t.Error("ProfileEvent")
	}
}

// A pass, a batch of ids at a time, writes what differs from the
// table: the backfill of an empty index, then the repair of changes
// made without an event; a document already in step is not written.
func TestProfilePass(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample")
	ctx := context.Background()
	o := IndexerOptions{Batch: 2, Pause: time.Millisecond, Log: quiet}
	ux := testProfileIndex(t, ProfileUsers)
	gx := testProfileIndex(t, ProfileGroups)
	uix, err := NewProfileIndexer(db.DB, ux, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	gix, err := NewProfileIndexer(db.DB, gx, nil, o)
	if err != nil {
		t.Fatal(err)
	}

	// The backfill stops after a batch and resumes in a new indexer.
	if err := uix.passStep(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := ux.LoadProfileState(ctx)
	if err != nil || !st.InPass || st.Done || st.Next != 9299000003 {
		t.Fatalf("after a batch: %+v %v", st, err)
	}
	if uix, err = NewProfileIndexer(db.DB, ux, nil, o); err != nil {
		t.Fatal(err)
	}
	if uix.st, err = ux.LoadProfileState(ctx); err != nil {
		t.Fatal(err)
	}
	for _, ix := range []*ProfileIndexer{uix, gix} {
		if err := ix.Pass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if st, _ = ux.LoadProfileState(ctx); st.InPass || !st.Done || st.SyncedAt == 0 {
		t.Errorf("after the pass: %+v", st)
	}
	var groups int64
	if err := db.QueryRow("SELECT COUNT(*) FROM telegram_groups WHERE description <> ''").
		Scan(&groups); err != nil {
		t.Fatal(err)
	}
	if n := profileCount(t, ux); n != 3 {
		t.Errorf("users: %d documents, want 3", n)
	}
	if n := profileCount(t, gx); n != groups {
		t.Errorf("groups: %d documents, want %d", n, groups)
	}
	d, _, ok := profileDoc(t, gx, -1001000000002)
	if !ok || d.Text != "Discussing kernel development and patches." {
		t.Errorf("the kernel group: %+v %v", d, ok)
	}

	// Without events: a bio changed, a row gone, a new bio, a
	// description emptied.
	exec(t, db, "UPDATE telegram_user_extra_info SET bio = 'Kernel hacker. Tea now.' "+
		"WHERE user_id = 9299000001")
	exec(t, db, "DELETE FROM telegram_user_extra_info WHERE user_id = 9299000003")
	exec(t, db, "INSERT INTO telegram_user_extra_info (user_id, bio) VALUES "+
		"(9299000002, 'Campus network admin')")
	exec(t, db, "UPDATE telegram_groups SET description = '' WHERE id = -1001000000003")
	_, v7, _ := profileDoc(t, ux, 9299000007)
	for _, ix := range []*ProfileIndexer{uix, gix} {
		if err := ix.Pass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if d, _, _ := profileDoc(t, ux, 9299000001); d.Text != "Kernel hacker. Tea now." {
		t.Errorf("changed bio: %+v", d)
	}
	if _, _, ok := profileDoc(t, ux, 9299000003); ok {
		t.Error("the gone row's document is left")
	}
	if d, _, _ := profileDoc(t, ux, 9299000002); d.Text != "Campus network admin" {
		t.Errorf("new bio: %+v", d)
	}
	if _, v, _ := profileDoc(t, ux, 9299000007); v != v7 {
		t.Errorf("an unchanged document was written again: version %d, was %d", v, v7)
	}
	if _, _, ok := profileDoc(t, gx, -1001000000003); ok {
		t.Error("the emptied description's document is left")
	}
	if n := profileCount(t, gx); n != groups-1 {
		t.Errorf("groups after: %d documents, want %d", n, groups-1)
	}
}

// Update events reach the index within seconds: a user's bio by
// user.updated (with no fields or "bio", not "photo"), a group's
// description by group.updated without fields.
func TestProfileEvents(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample")
	ux := testProfileIndex(t, ProfileUsers)
	gx := testProfileIndex(t, ProfileGroups)
	o := IndexerOptions{Log: quiet, Flush: 50 * time.Millisecond, Pause: time.Millisecond}
	usrc := chanSource{ch: make(chan live.Event, 16)}
	gsrc := chanSource{ch: make(chan live.Event, 16)}
	for _, p := range []struct {
		x   *Index
		src chanSource
	}{{ux, usrc}, {gx, gsrc}} {
		ix, err := NewProfileIndexer(db.DB, p.x, p.src, o)
		if err != nil {
			t.Fatal(err)
		}
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
		x := p.x
		waitFor(t, "the backfill", 30*time.Second, func() bool {
			st, err := x.LoadProfileState(context.Background())
			return err == nil && st.Done
		})
	}
	bio := func(want string) func() bool {
		return func() bool {
			d, _, ok := profileDoc(t, ux, 9299000001)
			return ok && d.Text == want
		}
	}

	exec(t, db, "UPDATE telegram_user_extra_info SET bio = 'first' WHERE user_id = 9299000001")
	usrc.ch <- live.Event{Kind: live.KindUserUpdated, UserID: 9299000001, Fields: []string{"photo"}}
	usrc.ch <- live.Event{Kind: live.KindUserUpdated, UserID: 9299000003, Fields: []string{"bio"}}
	time.Sleep(500 * time.Millisecond)
	if bio("first")() {
		t.Error("a photo event rewrote the bio")
	}
	usrc.ch <- live.Event{Kind: live.KindUserUpdated, UserID: 9299000001,
		Fields: []string{"bio", "birthday", "personal_chat"}}
	waitFor(t, "the bio event", 5*time.Second, bio("first"))

	exec(t, db, "UPDATE telegram_user_extra_info SET bio = 'second' WHERE user_id = 9299000001")
	usrc.ch <- live.Event{Kind: live.KindUserUpdated, UserID: 9299000001}
	waitFor(t, "the event without fields", 5*time.Second, bio("second"))

	// A row gone: its document goes.
	exec(t, db, "DELETE FROM telegram_user_extra_info WHERE user_id = 9299000001")
	usrc.ch <- live.Event{Kind: live.KindUserUpdated, UserID: 9299000001, Fields: []string{"bio"}}
	waitFor(t, "the document's removal", 5*time.Second, func() bool {
		_, _, ok := profileDoc(t, ux, 9299000001)
		return !ok
	})

	exec(t, db, "UPDATE telegram_groups SET description = 'Now about compilers.' "+
		"WHERE id = -1001000000003")
	gsrc.ch <- live.Event{Kind: live.KindGroupUpdated, ChatID: -1001000000003,
		Fields: []string{"admins"}}
	gsrc.ch <- live.Event{Kind: live.KindGroupUpdated, ChatID: -1001000000003}
	waitFor(t, "the group event", 5*time.Second, func() bool {
		d, _, ok := profileDoc(t, gx, -1001000000003)
		return ok && d.Text == "Now about compilers."
	})
}

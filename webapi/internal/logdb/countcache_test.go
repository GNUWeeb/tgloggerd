// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/testdb"
)

const countGroups = "SELECT COUNT(*) FROM telegram_groups"

// A count goes through the cache only when the request has one; with
// one, it is kept for the TTL, separately per pool, and a failed query
// is never kept.
func TestCountCache(t *testing.T) {
	t.Parallel()
	a := testdb.New(t, testdb.Logger, "sample").DB
	b := testdb.New(t, testdb.Logger).DB
	insert := func(id int64) {
		t.Helper()
		if _, err := a.Exec("INSERT INTO telegram_groups (id, title, type) VALUES (?, 'cache test', 'supergroup')", id); err != nil {
			t.Fatal(err)
		}
	}
	bg := context.Background()
	count := func(ctx context.Context, db *sql.DB) int64 {
		t.Helper()
		n, err := countInt(ctx, db, countGroups)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	// No cache: every call counts.
	n0 := count(bg, a)
	insert(-1009000000001)
	if n := count(bg, a); n != n0+1 {
		t.Fatalf("uncached count %d, want %d", n, n0+1)
	}

	c := NewCountCache(300 * time.Millisecond)
	ctx := WithCountCache(bg, c)
	n1 := count(ctx, a)
	insert(-1009000000002)
	if n := count(ctx, a); n != n1 {
		t.Errorf("cached count %d, want the kept %d", n, n1)
	}
	// The same SQL on another pool is its own answer.
	if n := count(ctx, b); n != 0 {
		t.Errorf("other pool %d, want its own 0", n)
	}
	time.Sleep(350 * time.Millisecond)
	if n := count(ctx, a); n != n1+1 {
		t.Errorf("after the TTL %d, want %d", n, n1+1)
	}

	// A failing query answers its error each time and is not kept.
	for i := 0; i < 2; i++ {
		if _, err := countInt(ctx, a, "SELECT COUNT(*) FROM no_such_table"); err == nil {
			t.Fatal("no error for a missing table")
		}
	}
	c.mu.Lock()
	kept := len(c.m)
	c.mu.Unlock()
	if kept != 2 {
		t.Errorf("%d answers kept, want 2 (one per pool, no failures)", kept)
	}

	// Concurrent requests all get the one answer.
	time.Sleep(350 * time.Millisecond)
	var wg sync.WaitGroup
	got := make([]int64, 16)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], _ = countInt(ctx, a, countGroups)
		}()
	}
	wg.Wait()
	for i, n := range got {
		if n != n1+1 {
			t.Errorf("concurrent %d: %d, want %d", i, n, n1+1)
		}
	}

	if NewCountCache(0) != nil {
		t.Error("a zero TTL makes a cache")
	}
}

// The users count leaves out the extra-info join unless a condition
// names it, and counts the same either way.
func TestUsersCountFrom(t *testing.T) {
	t.Parallel()
	for where, want := range map[string]string{
		"":                                     "telegram_users u",
		"u.first_name LIKE ?":                  "telegram_users u",
		"e.bio LIKE ?":                         Users.from,
		"(u.id = ? OR e.personal_chat_id = ?)": Users.from,
		"u.type = 'regular'":                   "telegram_users u",
	} {
		if got := Users.countFromFor(where); got != want {
			t.Errorf("%q: %q, want %q", where, got, want)
		}
	}
	db := testdb.New(t, testdb.Logger, "sample").DB
	ctx := context.Background()
	bare, err := countInt(ctx, db, "SELECT COUNT(*) FROM "+Users.countFromFor(""))
	if err != nil {
		t.Fatal(err)
	}
	joined, err := countInt(ctx, db, "SELECT COUNT(*) FROM "+Users.from)
	if err != nil {
		t.Fatal(err)
	}
	if bare != joined || bare == 0 {
		t.Errorf("bare %d, joined %d", bare, joined)
	}
}

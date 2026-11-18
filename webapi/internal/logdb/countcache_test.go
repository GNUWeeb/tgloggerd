// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"tgloggerd/webapi/internal/scope"
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

// A scope's totals are joined once, right after the query's own users,
// and only into queries that read the total.
func TestScopedTotalsJoin(t *testing.T) {
	t.Parallel()
	sc := scope.For(scope.Viewer{ID: 3, Role: scope.Admin, HidePrivate: true, HideGroups: []int64{-10}})
	q := scopedTotals(sc, "SELECT u.id, u.msg_count AS msg_count FROM telegram_users u WHERE u.msg_count > 1 ORDER BY u.msg_count DESC", nil)
	if strings.Count(q, " hg ON hg.uid = u.id") != 1 || strings.Count(q, " hp ON hp.uid = u.id") != 1 ||
		!strings.Contains(q, "FROM telegram_users u LEFT JOIN (SELECT sender_user_id") ||
		strings.Contains(q, "SELECT COUNT(*) FROM telegram_private_messages hp WHERE") ||
		!strings.Contains(q, " AS msg_count FROM") {
		t.Errorf("rewritten: %s", q)
	}
	if q := scopedTotals(sc, "SELECT COUNT(*) FROM telegram_users u", nil); strings.Contains(q, "JOIN") {
		t.Errorf("a query without the total got the joins: %s", q)
	}
	if q := scopedTotals(scope.For(scope.Viewer{ID: 1, Role: scope.Superadmin}), "SELECT u.msg_count FROM telegram_users u", nil); q != "SELECT u.msg_count FROM telegram_users u" {
		t.Errorf("a superadmin's query changed: %s", q)
	}
}

// A lookup by id counts only the looked-up users' messages in the
// derived tables, so a page of a chat does not count every sender of
// a big group.
func TestScopedTotalsOnlyIDs(t *testing.T) {
	t.Parallel()
	q := "SELECT u.id, u.msg_count FROM telegram_users u WHERE u.id IN (?, ?)"
	admin := scope.For(scope.Viewer{ID: 3, Role: scope.Admin, HidePrivate: true, HideGroups: []int64{-10}})
	got := scopedTotals(admin, q, []int64{5, 7})
	if strings.Count(got, "sender_user_id IN (5,7)") != 1 || strings.Count(got, "sender_id IN (5,7)") != 1 ||
		strings.Contains(got, "IS NOT NULL") {
		t.Errorf("admin lookup: %s", got)
	}
	member := scope.For(scope.Viewer{ID: 4, Role: scope.Member, GrantedGroups: []int64{-20, -21}})
	got = scopedTotals(member, q, []int64{5})
	if !strings.Contains(got, "chat_id IN (-21,-20) AND sender_user_id IN (5) GROUP BY") {
		t.Errorf("member lookup: %s", got)
	}
	if got := scopedTotals(admin, q, nil); !strings.Contains(got, "sender_user_id IS NOT NULL") {
		t.Errorf("list query: %s", got)
	}
}

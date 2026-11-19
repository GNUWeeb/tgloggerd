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

	"github.com/go-sql-driver/mysql"

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

// slowCounts stands in for the database in the count cache: a count
// with the inline limit runs out of time, a background one (with the
// cache's slowLimit) waits for release and answers n or fails.
type slowCounts struct {
	mu         sync.Mutex
	inline, bg int
	bgSQL      string
	started    chan struct{}
	release    chan struct{}
	n          int64
	fail       bool
}

func newSlowCounts(n int64, fail bool) *slowCounts {
	return &slowCounts{started: make(chan struct{}, 16), release: make(chan struct{}), n: n, fail: fail}
}

func (s *slowCounts) query(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error) {
	if !strings.Contains(q, "MAX_EXECUTION_TIME(3000)") {
		s.mu.Lock()
		s.bg++
		s.bgSQL = q
		s.mu.Unlock()
		s.started <- struct{}{}
		<-s.release
		if s.fail {
			return 0, &mysql.MySQLError{Number: 3024, Message: "timed out"}
		}
		return s.n, nil
	}
	s.mu.Lock()
	s.inline++
	s.mu.Unlock()
	return 0, &mysql.MySQLError{Number: 3024, Message: "Query execution was interrupted, maximum statement execution time exceeded"}
}

func (s *slowCounts) calls() (inline, bg int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inline, s.bg
}

const slowCountSQL = "SELECT /*+ MAX_EXECUTION_TIME(3000) */ COUNT(*) AS n FROM telegram_files f WHERE f.id > ?"

// A count that runs out of time is remembered: later requests answer
// at once without counting, one background count with the longer limit
// runs for the key, and its answer is then the requests' total.
func TestCountCacheSlowCount(t *testing.T) {
	t.Parallel()
	c := NewCountCache(time.Minute)
	c.slowTTL, c.slowLimit = 200*time.Millisecond, 7*time.Second
	sc := newSlowCounts(706879, false)
	c.query = sc.query
	ctx := WithCountCache(context.Background(), c)

	if _, err := countInt(ctx, nil, slowCountSQL, 5); err != errCountDeferred {
		t.Fatalf("timed-out count: %v, want errCountDeferred", err)
	}
	<-sc.started
	// While the background count runs, every request answers at once.
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := countInt(ctx, nil, slowCountSQL, 5); err != errCountDeferred {
				t.Errorf("deferred count: %v", err)
			}
		}()
	}
	wg.Wait()
	if in, bg := sc.calls(); in != 1 || bg != 1 {
		t.Errorf("%d inline and %d background counts, want 1 and 1", in, bg)
	}
	if !strings.Contains(sc.bgSQL, "MAX_EXECUTION_TIME(7000)") || strings.Contains(sc.bgSQL, "(3000)") {
		t.Errorf("background SQL %q, want the longer limit", sc.bgSQL)
	}
	close(sc.release)
	c.wg.Wait()
	if n, err := countInt(ctx, nil, slowCountSQL, 5); err != nil || n != 706879 {
		t.Errorf("after the background count: %d, %v; want 706879", n, err)
	}
	if in, _ := sc.calls(); in != 1 {
		t.Errorf("%d inline counts, want 1", in)
	}

	// Once its time is up, the last answer stands while one
	// background count runs again; never inline.
	time.Sleep(250 * time.Millisecond)
	// No background count runs now (c.wg is done).
	sc.n, sc.release = 706900, make(chan struct{})
	for range 3 {
		if n, err := countInt(ctx, nil, slowCountSQL, 5); err != nil || n != 706879 {
			t.Errorf("expired slow count: %d, %v; want the last answer", n, err)
		}
	}
	close(sc.release)
	c.wg.Wait()
	if n, err := countInt(ctx, nil, slowCountSQL, 5); err != nil || n != 706900 {
		t.Errorf("refreshed count: %d, %v; want 706900", n, err)
	}
	if in, bg := sc.calls(); in != 1 || bg != 2 {
		t.Errorf("%d inline and %d background counts, want 1 and 2", in, bg)
	}
}

// A background count that fails too keeps the estimate, and is not
// tried again before slowTTL has passed.
func TestCountCacheSlowCountFails(t *testing.T) {
	t.Parallel()
	c := NewCountCache(time.Minute)
	c.slowTTL = 300 * time.Millisecond
	sc := newSlowCounts(0, true)
	close(sc.release)
	c.query = sc.query
	ctx := WithCountCache(context.Background(), c)

	if _, err := countInt(ctx, nil, slowCountSQL, 7); err != errCountDeferred {
		t.Fatalf("timed-out count: %v", err)
	}
	c.wg.Wait()
	for range 3 {
		if _, err := countInt(ctx, nil, slowCountSQL, 7); err != errCountDeferred {
			t.Errorf("after a failed background count: %v", err)
		}
	}
	if in, bg := sc.calls(); in != 1 || bg != 1 {
		t.Errorf("%d inline and %d background counts, want 1 and 1", in, bg)
	}
	time.Sleep(350 * time.Millisecond)
	if _, err := countInt(ctx, nil, slowCountSQL, 7); err != errCountDeferred {
		t.Errorf("expired failure: %v", err)
	}
	c.wg.Wait()
	if in, bg := sc.calls(); in != 1 || bg != 2 {
		t.Errorf("%d inline and %d background counts after slowTTL, want 1 and 2", in, bg)
	}
}

// MySQL's own time limit is what marks a count slow: a statement that
// runs out of MAX_EXECUTION_TIME is answered by deferral from then on.
func TestCountCacheMySQLTimeout(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger).DB
	c := NewCountCache(time.Minute)
	// The background count runs out of time as well.
	c.slowLimit = time.Millisecond
	ctx := WithCountCache(context.Background(), c)
	q := "SELECT /*+ MAX_EXECUTION_TIME(1) */ COUNT(*) FROM information_schema.columns a, " +
		"information_schema.columns b, information_schema.columns c"
	if _, err := countInt(ctx, db, q); err != errCountDeferred {
		t.Fatalf("first count: %v, want errCountDeferred", err)
	}
	c.wg.Wait()
	start := time.Now()
	if _, err := countInt(ctx, db, q); err != errCountDeferred {
		t.Errorf("second count: %v, want errCountDeferred", err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("second count took %v, want no query", d)
	}
}

// A list whose count times out answers an estimate at once afterwards,
// and the exact total once the background count is done.
func TestSearchCountDeferred(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Logger, "sample").DB
	sc := scope.For(scope.Viewer{ID: 3, Role: scope.Admin, HidePrivate: true,
		HideGroups: []int64{-1009999999999}})
	req := SearchRequest{Internal: true, Limit: 1}
	bg := context.Background()
	want, err := Search(bg, db, sc, Files, req)
	if err != nil {
		t.Fatal(err)
	}
	if !want.TotalExact || want.Total < 2 {
		t.Fatalf("uncached total %d (exact %v), want an exact total above one page",
			want.Total, want.TotalExact)
	}

	c := NewCountCache(time.Minute)
	var mu sync.Mutex
	inline := 0
	release := make(chan struct{})
	c.query = func(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error) {
		if strings.Contains(q, "MAX_EXECUTION_TIME(3000)") {
			mu.Lock()
			inline++
			mu.Unlock()
			return 0, &mysql.MySQLError{Number: 3024, Message: "timed out"}
		}
		<-release
		return queryInt(ctx, db, q, args...)
	}
	ctx := WithCountCache(bg, c)
	for i := range 2 {
		res, err := Search(ctx, db, sc, Files, req)
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalExact || res.Total != 2 {
			t.Errorf("request %d: total %d (exact %v), want the estimate 2", i, res.Total, res.TotalExact)
		}
	}
	close(release)
	c.wg.Wait()
	res, err := Search(ctx, db, sc, Files, req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TotalExact || res.Total != want.Total {
		t.Errorf("after the background count: total %d (exact %v), want %d exact",
			res.Total, res.TotalExact, want.Total)
	}
	mu.Lock()
	defer mu.Unlock()
	if inline != 1 {
		t.Errorf("%d inline counts, want 1", inline)
	}
}

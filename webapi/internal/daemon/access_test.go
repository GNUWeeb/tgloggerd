// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAccess is a daemon that takes access reports, or refuses them
// while down is set.
type fakeAccess struct {
	mu      sync.Mutex
	batches [][]uint64
	down    bool
}

func (f *fakeAccess) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []uint64 `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	f.batches = append(f.batches, in.IDs)
	w.Write([]byte(`{"ids":1,"updated":1}`))
}

func (f *fakeAccess) all() []uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []uint64
	for _, b := range f.batches {
		out = append(out, b...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func newReporter(t *testing.T, h http.Handler, log *bytes.Buffer) *AccessReporter {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, testKey, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return NewAccessReporter(c, slog.New(slog.NewTextHandler(log, nil)))
}

func TestAccessReporterBatches(t *testing.T) {
	f := &fakeAccess{}
	var log bytes.Buffer
	a := newReporter(t, f, &log)

	for id := uint64(1); id <= 2500; id++ {
		a.Record(id)
		a.Record(id) // served twice, reported once
	}
	a.Record(0) // never a file id
	a.Flush(context.Background())

	if len(f.batches) != 3 || len(f.batches[0]) > accessBatch ||
		len(f.batches[1]) > accessBatch || len(f.batches[2]) > accessBatch {
		t.Fatalf("batch sizes wrong: %d batches", len(f.batches))
	}
	got := f.all()
	if len(got) != 2500 || got[0] != 1 || got[2499] != 2500 {
		t.Fatalf("reported %d ids", len(got))
	}

	// Emptied: nothing is sent again.
	a.Flush(context.Background())
	if len(f.batches) != 3 {
		t.Error("ids sent twice")
	}
}

func TestAccessReporterDropsAndLogsOnce(t *testing.T) {
	f := &fakeAccess{down: true}
	var log bytes.Buffer
	a := newReporter(t, f, &log)

	for i := 0; i < 3; i++ {
		a.Record(uint64(i + 1))
		a.Flush(context.Background())
	}
	if n := strings.Count(log.String(), "access report failed"); n != 1 {
		t.Errorf("failure logged %d times:\n%s", n, log.String())
	}

	// Back: the dropped ids are not retried, and recovery is logged.
	f.mu.Lock()
	f.down = false
	f.mu.Unlock()
	a.Record(9)
	a.Flush(context.Background())
	if got := f.all(); len(got) != 1 || got[0] != 9 {
		t.Errorf("after recovery reported %v, want [9]", got)
	}
	if !strings.Contains(log.String(), "access reports work again") {
		t.Errorf("recovery not logged:\n%s", log.String())
	}
}

func TestAccessReporterRun(t *testing.T) {
	f := &fakeAccess{}
	a := newReporter(t, f, &bytes.Buffer{})
	a.every = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	a.Record(42)
	deadline := time.Now().Add(5 * time.Second)
	for len(f.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if got := f.all(); len(got) != 1 || got[0] != 42 {
		t.Errorf("Run reported %v, want [42]", got)
	}
}

func TestAccessReporterOff(t *testing.T) {
	var a *AccessReporter = NewAccessReporter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if a != nil {
		t.Fatal("a reporter without a client")
	}
	a.Record(1)
	a.Flush(context.Background())
	a.Run(context.Background())
}

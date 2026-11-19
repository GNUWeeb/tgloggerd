// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/render"
	"tgloggerd/webapi/internal/scope"
)

// loggerAccount keeps the logging account's Telegram user id, from the
// daemon's GET /v1/status (daemon.Client.Status), for Viewer.logger.
//
// The id never changes for a running daemon, so once known it is kept.
// Until then the daemon is asked in the background, never in a page's
// path: at most one ask in flight, and none sooner than loggerRetry
// after a failed one or one that had no account yet (before login).
// A page asked for meanwhile gets no logger.
type loggerAccount struct {
	log   *slog.Logger
	dc    *daemon.Client
	clock func() time.Time
	retry time.Duration

	mu    sync.Mutex
	known int64     // the account's id, 0 while unknown
	next  time.Time // no ask before
	busy  bool
	wg    sync.WaitGroup
}

// loggerRetry is the least time between two asks while the account is
// unknown: short, as the account appears once the daemon logs in.
const loggerRetry = 15 * time.Second

func newLoggerAccount(log *slog.Logger, dc *daemon.Client, clock func() time.Time) *loggerAccount {
	return &loggerAccount{log: log, dc: dc, clock: clock, retry: loggerRetry}
}

// id is the account's user id, or 0 while unknown; it returns at once,
// starting an ask when one is due. A nil keeper, or one without the
// daemon API, knows none.
func (l *loggerAccount) id() int64 {
	if l == nil || !l.dc.Enabled() {
		return 0
	}
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.known != 0 || l.busy || now.Before(l.next) {
		return l.known
	}
	l.busy = true
	l.next = now.Add(l.retry)
	l.wg.Add(1)
	go l.ask()
	return 0
}

// ask is one status call; the client's timeout bounds it.
func (l *loggerAccount) ask() {
	defer l.wg.Done()
	st, ok := l.dc.Status(context.Background())
	l.mu.Lock()
	l.busy = false
	if ok && st.AccountID > 0 {
		l.known = st.AccountID
	}
	l.mu.Unlock()
	if !ok {
		l.log.Debug("logger account: no status from the daemon")
	}
}

// wait returns once no ask is in flight, for tests.
func (l *loggerAccount) wait() {
	l.wg.Wait()
}

// pageLogger is the logging account as the viewer object names it to
// every viewer, signed in or not: its user row, with the photo as
// other user references build it. nil while the account is unknown or
// has no row yet; a failed read is logged and also gives nil, as the
// page does not depend on it.
func (s *Server) pageLogger(ctx context.Context, sc scope.Scope, signedIn bool) *render.Logger {
	id := s.logger.id()
	if id == 0 {
		return nil
	}
	u, ok, err := logdb.LoggerUser(ctx, s.ro, id)
	if err != nil {
		s.log.Warn("logger account lookup", "id", id, "err", err)
		return nil
	}
	if !ok {
		return nil
	}
	lg := &render.Logger{ID: u.ID, Name: u.Name(), Photo: s.viewFor(sc, signedIn).photo(u.Photo)}
	if u.Username != "" {
		lg.Username = &u.Username
	}
	return lg
}

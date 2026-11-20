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
)

// photoRefresher has the daemon list a user's or a group's photos
// from Telegram when its page opens (daemon.Client.UserPhotos,
// ChatPhotos). The history records only the photo changes the logger
// saw; listing them, the daemon adds the photos and history entries it
// lacked, so the photo list (logdb.UserPhotos, GroupPhotos) is whole
// on the next load.
//
// The ask runs in the background, never in the page's path: at most one
// in flight per user or group, and none sooner than photoRefreshEvery
// after the last one for it (a flood wait's retry_after if longer),
// whatever came of it. Failures are only logged; the page shows what
// the archive has. The times are kept in memory: a restart may ask once
// more.
type photoRefresher struct {
	log   *slog.Logger
	dc    *daemon.Client
	clock func() time.Time
	every time.Duration

	mu   sync.Mutex
	next map[photoOwner]time.Time // no ask before
	busy map[photoOwner]bool
	wg   sync.WaitGroup
}

// photoOwner is a user (group false) or a group, by its Telegram id.
type photoOwner struct {
	group bool
	id    int64
}

const (
	// photoRefreshEvery is the least time between two asks for the
	// same user or group; the daemon caches its answers about as long.
	photoRefreshEvery = 10 * time.Minute
	// photoRefreshKeep is how many owners the refresher remembers
	// before it forgets those whose wait is over.
	photoRefreshKeep = 10000
)

func newPhotoRefresher(log *slog.Logger, dc *daemon.Client, clock func() time.Time) *photoRefresher {
	return &photoRefresher{log: log, dc: dc, clock: clock, every: photoRefreshEvery,
		next: map[photoOwner]time.Time{}, busy: map[photoOwner]bool{}}
}

// user asks for a user's photos, group for a group's; both return at
// once. A nil refresher, or one without the daemon API, asks nothing.
func (p *photoRefresher) user(id int64)  { p.ask(photoOwner{false, id}) }
func (p *photoRefresher) group(id int64) { p.ask(photoOwner{true, id}) }

func (p *photoRefresher) ask(o photoOwner) {
	if p == nil || !p.dc.Enabled() {
		return
	}
	now := p.clock()
	p.mu.Lock()
	if p.busy[o] || now.Before(p.next[o]) {
		p.mu.Unlock()
		return
	}
	if len(p.next) >= photoRefreshKeep {
		for k, t := range p.next {
			if !p.busy[k] && !now.Before(t) {
				delete(p.next, k)
			}
		}
	}
	p.busy[o] = true
	p.next[o] = now.Add(p.every)
	p.wg.Add(1)
	p.mu.Unlock()
	go p.run(o, now)
}

// run is one ask; its own call bounds it (the client's timeout), as
// the page that started it is long answered.
func (p *photoRefresher) run(o photoOwner, at time.Time) {
	defer p.wg.Done()
	var (
		l     daemon.PhotoList
		ok    bool
		reply daemon.Reply
	)
	if o.group {
		l, ok, reply = p.dc.ChatPhotos(context.Background(), o.id)
	} else {
		l, ok, reply = p.dc.UserPhotos(context.Background(), o.id)
	}
	p.mu.Lock()
	delete(p.busy, o)
	if ok && l.Outcome == daemon.PhotosFloodWait {
		if t := at.Add(time.Duration(l.RetryAfter) * time.Second); t.After(p.next[o]) {
			p.next[o] = t
		}
	}
	p.mu.Unlock()
	switch {
	case !ok:
		p.log.Debug("photo list: no answer", "group", o.group, "id", o.id,
			"status", reply.Status, "err", reply.Error)
	case l.Outcome != daemon.PhotosOK:
		p.log.Info("photo list: not listed", "group", o.group, "id", o.id,
			"outcome", l.Outcome, "message", l.Message, "retry_after", l.RetryAfter)
	default:
		p.log.Debug("photo list: listed", "group", o.group, "id", o.id,
			"photos", len(l.Photos), "total", l.Total)
	}
}

// wait waits for the asks in flight, for tests.
func (p *photoRefresher) wait() {
	p.wg.Wait()
}

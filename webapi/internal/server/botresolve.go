// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
)

// BotResolver gets the Telegram bots' own users into the archive, so a
// bot can link to its user page (BotRef.profile). A bot writes into no
// logged chat, so the logger seldom sees it; once discordd has logged
// a bot in, its identity (user id and username) is known, and the
// daemon resolves the username and stores the user
// (daemon.Client.ResolveUser).
//
// It runs in the background, never in a page request's path: a pass
// over the bots every few minutes, and, for a bot just created or
// given a new token (Nudge), a pass every few seconds for a while, as
// its identity comes only once discordd reports it. Each pass asks
// only about bots whose user row is still missing. A bot whose resolve
// failed waits before it is asked again, hours after an answer that
// would not change soon, the daemon's wait after a flood wait. The
// attempts are kept in memory: a restart may ask once more.
type BotResolver struct {
	log   *slog.Logger
	ro    *sql.DB
	dc    *daemon.Client
	clock func() time.Time

	every      time.Duration // the pass over every bot
	watchEvery time.Duration // the passes over the nudged bots
	watchFor   time.Duration // how long a nudged bot is watched

	wake chan struct{}

	mu    sync.Mutex
	tries map[uint64]botTry    // by bot id
	watch map[uint64]time.Time // nudged bots, until when
	// pause is until when no bot is asked about: a flood wait is the
	// daemon's account's, whichever username it came for.
	pause time.Time
}

// botTry is a bot's last resolve: for which identity, and when the bot
// may be asked about again.
type botTry struct {
	userID   int64
	username string
	until    time.Time
}

const (
	botResolveEvery      = 5 * time.Minute
	botResolveWatchEvery = 15 * time.Second
	botResolveWatchFor   = 10 * time.Minute
	// After not_found, not_a_user or error: the username or the
	// account will not change soon.
	botRetryFailed = 6 * time.Hour
	// After a resolve the daemon said it stored, if the row is still
	// not there.
	botRetryResolved = time.Hour
	// When the daemon gave no answer to go by, and the least wait
	// after a flood wait.
	botRetryAway = time.Minute
)

// NewBotResolver makes the resolver; Run starts it. With the daemon
// API off it does nothing.
func NewBotResolver(log *slog.Logger, ro *sql.DB, dc *daemon.Client) *BotResolver {
	return &BotResolver{
		log: log, ro: ro, dc: dc, clock: time.Now,
		every: botResolveEvery, watchEvery: botResolveWatchEvery, watchFor: botResolveWatchFor,
		wake:  make(chan struct{}, 1),
		tries: map[uint64]botTry{},
		watch: map[uint64]time.Time{},
	}
}

// Nudge asks for the bot to be resolved as soon as its identity is
// known: a Telegram bot just created or given a new token, which
// forgets the bot's earlier attempts (not a flood wait). A nil
// resolver ignores it.
func (b *BotResolver) Nudge(id uint64) {
	if b == nil || logdb.KindOf(id) != logdb.KindTelegram {
		return
	}
	b.mu.Lock()
	b.watch[id] = b.clock().Add(b.watchFor)
	delete(b.tries, id)
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Run resolves until ctx ends: a pass over every bot at once and every
// b.every, and over the watched bots when nudged and every
// b.watchEvery while any is watched.
func (b *BotResolver) Run(ctx context.Context) {
	if b == nil || !b.dc.Enabled() {
		return
	}
	b.pass(ctx, false)
	sweep := time.NewTicker(b.every)
	defer sweep.Stop()
	watch := time.NewTicker(b.watchEvery)
	defer watch.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			b.pass(ctx, false)
		case <-b.wake:
			b.pass(ctx, true)
		case <-watch.C:
			if b.watching() {
				b.pass(ctx, true)
			}
		}
	}
}

// watching is whether any nudged bot is still watched, forgetting
// those whose time is up.
func (b *BotResolver) watching() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock()
	for id, until := range b.watch {
		if !now.Before(until) {
			delete(b.watch, id)
		}
	}
	return len(b.watch) > 0
}

// due is whether bot u may be asked about now: no flood wait is on,
// and it was never tried for this identity or its wait is over.
func (b *BotResolver) due(u logdb.UnresolvedBot, watchedOnly bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock()
	if now.Before(b.pause) {
		return false
	}
	if watchedOnly {
		until, ok := b.watch[u.ID]
		if !ok || !now.Before(until) {
			return false
		}
	}
	t, ok := b.tries[u.ID]
	return !ok || t.userID != u.UserID || t.username != u.Username || !now.Before(t.until)
}

func (b *BotResolver) tried(u logdb.UnresolvedBot, wait time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tries[u.ID] = botTry{userID: u.UserID, username: u.Username, until: b.clock().Add(wait)}
	delete(b.watch, u.ID)
}

// pass asks the daemon about every bot due whose user row is missing,
// or, with watchedOnly, about the watched ones only.
func (b *BotResolver) pass(ctx context.Context, watchedOnly bool) {
	bots, err := logdb.UnresolvedTelegramBots(ctx, b.ro)
	if err != nil {
		if ctx.Err() == nil {
			b.log.Warn("bot users: listing the bots failed", "err", err)
		}
		return
	}
	for _, u := range bots {
		if ctx.Err() != nil {
			return
		}
		if b.due(u, watchedOnly) {
			b.resolve(ctx, u)
		}
	}
}

func (b *BotResolver) resolve(ctx context.Context, u logdb.UnresolvedBot) {
	res, ok, reply := b.dc.ResolveUser(ctx, u.Username)
	attrs := []any{"bot", u.ID, "username", u.Username}
	if !ok {
		if ctx.Err() != nil {
			return
		}
		// Keep the bot watched: the daemon may be back on the next pass.
		b.mu.Lock()
		b.tries[u.ID] = botTry{userID: u.UserID, username: u.Username, until: b.clock().Add(botRetryAway)}
		b.mu.Unlock()
		b.log.Info("bot user: no answer from the daemon", append(attrs, "err", reply.Error)...)
		return
	}
	attrs = append(attrs, "outcome", res.Outcome)
	if res.Message != "" {
		attrs = append(attrs, "message", res.Message)
	}
	switch res.Outcome {
	case daemon.ResolveOK:
		if res.User.UserID != u.UserID {
			// The username now names another account: the bot's own
			// is not what was stored.
			b.tried(u, botRetryFailed)
			b.log.Warn("bot user: the username names another user",
				append(attrs, "user_id", u.UserID, "resolved", res.User.UserID)...)
			return
		}
		b.tried(u, botRetryResolved)
		b.log.Info("bot user resolved", append(attrs, "user_id", u.UserID)...)
	case daemon.ResolveFloodWait:
		wait := max(time.Duration(res.RetryAfter)*time.Second, botRetryAway)
		b.tried(u, wait)
		b.mu.Lock()
		b.pause = b.clock().Add(wait)
		b.mu.Unlock()
		b.log.Info("bot user: Telegram asks to wait", append(attrs, "retry_after", res.RetryAfter)...)
	default:
		b.tried(u, botRetryFailed)
		b.log.Info("bot user not resolved", attrs...)
	}
}

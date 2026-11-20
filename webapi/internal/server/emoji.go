// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
)

// Custom emoji and message effects (webapi/openapi.yaml: CustomEmoji,
// MessageEffect; web/docs/message-kinds.md, "Custom emoji"). A page's
// messages name custom emoji by id only, in text entities, reactions
// and forum topic icons, and their effects by meta.effect_id. The ids
// of a page are resolved in one batch (emojiResolver.resolve): read
// from telegram_custom_emoji and telegram_message_effects, and those
// the tables lack asked of the daemon (POST /v1/custom-emoji and
// /v1/message-effects), which fills the tables from Telegram.
//
// A page never waits on the daemon beyond emojiPageBudget: an ask it
// started that has not answered by then goes on in the background, the
// ids it could not resolve keep their fallback (null), and a later page
// finds the rows the ask stored. An id already being asked for by
// another page's ask is not asked again, nor waited for. The resolver
// remembers in memory, so a restart may ask once more:
//   - the ids Telegram does not know, not asked again for
//     emojiMissingRetry;
//   - a pause per kind after a flood wait (its retry_after) or after an
//     ask without an answer or that Telegram failed (emojiAwayRetry),
//     in which no page asks, so a daemon that is away costs no page
//     any time.

const (
	// emojiPageBudget is how long a page waits for the asks it
	// started.
	emojiPageBudget = 400 * time.Millisecond
	// emojiMissingRetry is how long an id Telegram does not know is
	// not asked for again.
	emojiMissingRetry = time.Hour
	// emojiAwayRetry is how long no page asks after an ask that got
	// no answer, or whose answer was not ok (but a flood wait's).
	emojiAwayRetry = 30 * time.Second
	// emojiMissingKeep is how many unknown ids the resolver remembers
	// before it forgets those whose wait is over.
	emojiMissingKeep = 10000
)

// emojiKind is a custom emoji or a message effect.
type emojiKind int

const (
	kindCustomEmoji emojiKind = iota
	kindEffect
)

func (k emojiKind) String() string {
	if k == kindEffect {
		return "message effect"
	}
	return "custom emoji"
}

type emojiKey struct {
	kind emojiKind
	id   int64
}

// emojiResolver resolves a page's custom emoji and message effects; one
// per service.
type emojiResolver struct {
	log    *slog.Logger
	dc     *daemon.Client
	clock  func() time.Time
	budget time.Duration

	mu      sync.Mutex
	asking  map[emojiKey]bool      // in an ask now
	missing map[emojiKey]time.Time // not asked for before
	pause   [2]time.Time           // by kind: no asks before
	wg      sync.WaitGroup
}

func newEmojiResolver(log *slog.Logger, dc *daemon.Client, clock func() time.Time) *emojiResolver {
	return &emojiResolver{log: log, dc: dc, clock: clock, budget: emojiPageBudget,
		asking: map[emojiKey]bool{}, missing: map[emojiKey]time.Time{}}
}

// pageEmoji are the custom emoji and message effects a page resolved,
// by id; an id it could not resolve is in neither.
type pageEmoji struct {
	emoji   map[int64]logdb.CustomEmoji
	effects map[int64]logdb.MessageEffect
}

// resolve reads the rows of the custom emoji and effects ids name, and
// asks the daemon for those without one, waiting for that answer at
// most r.budget (see above). Only reading the tables can fail.
func (r *emojiResolver) resolve(ctx context.Context, db *sql.DB, emoji, effects []int64) (pageEmoji, error) {
	var (
		p   pageEmoji
		err error
	)
	if p.emoji, err = logdb.CustomEmojiByID(ctx, db, emoji); err != nil {
		return p, err
	}
	if p.effects, err = logdb.MessageEffectsByID(ctx, db, effects); err != nil {
		return p, err
	}
	askEmoji := r.start(kindCustomEmoji, unknownIDs(emoji, p.emoji))
	askEffects := r.start(kindEffect, unknownIDs(effects, p.effects))
	if askEmoji == nil && askEffects == nil {
		return p, nil
	}
	t := time.NewTimer(r.budget)
	defer t.Stop()
	for _, a := range []*emojiAsk{askEmoji, askEffects} {
		if a == nil {
			continue
		}
		select {
		case <-a.done:
		case <-t.C:
			return p, nil
		case <-ctx.Done():
			return p, nil
		}
		// The rows the ask stored, with their files as the tables
		// have them.
		if a.kind == kindCustomEmoji {
			more, err := logdb.CustomEmojiByID(ctx, db, a.ids)
			if err != nil {
				return p, err
			}
			for id, e := range more {
				p.emoji[id] = e
			}
		} else {
			more, err := logdb.MessageEffectsByID(ctx, db, a.ids)
			if err != nil {
				return p, err
			}
			for id, e := range more {
				p.effects[id] = e
			}
		}
	}
	return p, nil
}

// unknownIDs are the ids, once each, that rows lacks.
func unknownIDs[T any](ids []int64, rows map[int64]T) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, id := range ids {
		if _, ok := rows[id]; ok || id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// emojiAsk is one ask in flight; done is closed when it is over.
type emojiAsk struct {
	kind emojiKind
	ids  []int64
	done chan struct{}
}

// start asks the daemon for the ids of kind that no ask has now, that
// are not known to be missing, up to the daemon's limit, unless asks of
// kind are paused; nil when it asks nothing.
func (r *emojiResolver) start(kind emojiKind, ids []int64) *emojiAsk {
	if r == nil || len(ids) == 0 || !r.dc.Enabled() {
		return nil
	}
	limit := daemon.CustomEmojiLimit
	if kind == kindEffect {
		limit = daemon.MessageEffectLimit
	}
	now := r.clock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Before(r.pause[kind]) {
		return nil
	}
	a := &emojiAsk{kind: kind, done: make(chan struct{})}
	for _, id := range ids {
		k := emojiKey{kind, id}
		if len(a.ids) == limit {
			break
		}
		if r.asking[k] || now.Before(r.missing[k]) {
			continue
		}
		a.ids = append(a.ids, id)
	}
	if len(a.ids) == 0 {
		return nil
	}
	for _, id := range a.ids {
		r.asking[emojiKey{kind, id}] = true
	}
	r.wg.Add(1)
	go r.run(a, now)
	return a
}

// run is one ask; the client's timeout bounds it, as the page that
// started it may be long answered.
func (r *emojiResolver) run(a *emojiAsk, at time.Time) {
	defer r.wg.Done()
	defer close(a.done)
	var (
		l     daemon.EmojiLookup
		ok    bool
		reply daemon.Reply
	)
	if a.kind == kindEffect {
		l, ok, reply = r.dc.MessageEffects(context.Background(), a.ids)
	} else {
		l, ok, reply = r.dc.CustomEmojiStickers(context.Background(), a.ids)
	}
	r.mu.Lock()
	for _, id := range a.ids {
		delete(r.asking, emojiKey{a.kind, id})
	}
	switch {
	case !ok:
		r.pause[a.kind] = at.Add(emojiAwayRetry)
	case l.Outcome == daemon.EmojiFloodWait:
		r.pause[a.kind] = at.Add(time.Duration(l.RetryAfter) * time.Second)
	case l.Outcome != daemon.EmojiOK:
		r.pause[a.kind] = at.Add(emojiAwayRetry)
	}
	if ok && len(l.Missing) > 0 {
		if len(r.missing) >= emojiMissingKeep {
			for k, t := range r.missing {
				if !at.Before(t) {
					delete(r.missing, k)
				}
			}
		}
		for _, id := range l.Missing {
			if len(r.missing) < emojiMissingKeep {
				r.missing[emojiKey{a.kind, id}] = at.Add(emojiMissingRetry)
			}
		}
	}
	r.mu.Unlock()
	switch {
	case !ok:
		r.log.Debug("emoji lookup: no answer", "kind", a.kind.String(), "ids", len(a.ids),
			"status", reply.Status, "err", reply.Error)
	case l.Outcome != daemon.EmojiOK:
		r.log.Info("emoji lookup: not resolved", "kind", a.kind.String(), "ids", len(a.ids),
			"outcome", l.Outcome, "message", l.Message, "retry_after", l.RetryAfter)
	default:
		r.log.Debug("emoji lookup: resolved", "kind", a.kind.String(), "ids", len(a.ids),
			"asked", l.Asked, "missing", len(l.Missing))
	}
}

// wait waits for the asks in flight, for tests.
func (r *emojiResolver) wait() {
	r.wg.Wait()
}

// customEmojiJSON is a CustomEmoji.
type customEmojiJSON struct {
	Emoji           string  `json:"emoji"`
	Format          *string `json:"format"`
	URL             *string `json:"url"`
	NeedsRepainting bool    `json:"needs_repainting"`
}

// messageEffectJSON is a MessageEffect.
type messageEffectJSON struct {
	Emoji  string  `json:"emoji"`
	Format *string `json:"format"`
	URL    *string `json:"url"`
}

// stickerURL is a sticker's URL and format for the viewer: both nil
// when its format is not known, or there is no URL to give.
func (v view) stickerURL(format string, f logdb.FileRef) (*string, *string) {
	if format == "" {
		return nil, nil
	}
	u := v.photo(f)
	if u == nil {
		return nil, nil
	}
	return &format, u
}

// customEmoji is the custom emoji id (a decimal string, as entities
// and reactions carry it) names, as the page resolved it; nil when it
// did not, or id is not one.
func (v view) customEmoji(k kinds, id string) *customEmojiJSON {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil
	}
	return v.customEmojiOf(k, n)
}

func (v view) customEmojiOf(k kinds, id int64) *customEmojiJSON {
	e, ok := k.emoji.emoji[id]
	if !ok || id == 0 {
		return nil
	}
	j := &customEmojiJSON{Emoji: e.Emoji, NeedsRepainting: e.NeedsRepainting}
	j.Format, j.URL = v.stickerURL(e.Format, e.File)
	return j
}

// effect is a message's effect, as the page resolved it; nil for none
// and when it did not.
func (v view) effect(k kinds, id int64) *messageEffectJSON {
	e, ok := k.emoji.effects[id]
	if !ok || id == 0 {
		return nil
	}
	j := &messageEffectJSON{Emoji: e.Emoji}
	j.Format, j.URL = v.stickerURL(e.Format, e.File)
	return j
}

// jsonOrNull is p's JSON, null for nil.
func jsonOrNull[T any](p *T) json.RawMessage {
	if p == nil {
		return json.RawMessage("null")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// looseID is a 64-bit id stored as a decimal string or a number; 0
// for anything else, which does not fail the value around it.
type looseID int64

func (l *looseID) UnmarshalJSON(b []byte) error {
	*l = 0
	if id, ok := entityEmojiID(b); ok {
		n, _ := strconv.ParseInt(id, 10, 64)
		*l = looseID(n)
	}
	return nil
}

// idString is a 64-bit id as a string, nil for 0.
func idString(id int64) *string {
	if id == 0 {
		return nil
	}
	s := strconv.FormatInt(id, 10)
	return &s
}

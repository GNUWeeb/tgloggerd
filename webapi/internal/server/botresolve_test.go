// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/daemon"
)

// The resolver of the Telegram bots' own users, against a fake daemon
// that answers POST /v1/users/resolve by username and, on ok, stores
// the user as the daemon does. Bots of the "forwarding", "bots" and
// "fwd_photos" fixtures: Telegram bot 1 has its user row, bot 2 has
// no identity yet, the Discord bots are never asked about.

// resolveDaemon is the fake daemon.
type resolveDaemon struct {
	t  *testing.T
	db *sql.DB

	mu      sync.Mutex
	answers map[string]string // by "@username": the JSON answer
	status  int               // when not 0, the status of every answer
	asked   []string
}

func (d *resolveDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
	}
	b, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(b, &in)
	d.mu.Lock()
	d.asked = append(d.asked, in.Username)
	a, st := d.answers[in.Username], d.status
	d.mu.Unlock()
	if want, _ := daemon.Bearer(testKey); r.Header.Get("Authorization") != want ||
		r.URL.Path != "/v1/users/resolve" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if st != 0 {
		w.WriteHeader(st)
		return
	}
	if a == "" {
		a = `{"outcome":"not_found","message":"","user":null}`
	}
	var res daemon.UserResolve
	_ = json.Unmarshal([]byte(a), &res)
	if res.Outcome == daemon.ResolveOK {
		if _, err := d.db.Exec("INSERT INTO telegram_users (id, first_name, last_name, type, "+
			"msg_count, created_at, updated_at) VALUES (?, 'Resolved', '', 'bot', 0, NOW(), NOW())",
			res.User.UserID); err != nil {
			d.t.Error(err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, a)
}

func (d *resolveDaemon) answer(username, body string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers[username] = body
}

func (d *resolveDaemon) setStatus(st int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status = st
}

// took is what was asked since the last call, sorted.
func (d *resolveDaemon) took() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.asked
	d.asked = nil
	slices.Sort(out)
	return out
}

func okAnswer(uid int64, username string) string {
	return fmt.Sprintf(`{"outcome":"ok","message":"","user":{"user_id":%d,"username":%q,"is_bot":true}}`,
		uid, username)
}

// clockAt is a clock a test moves.
type clockAt struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clockAt) get() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clockAt) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newResolver is a resolver of e's archive, against a fake daemon, on
// a clock the test moves.
func newResolver(t *testing.T, e *adminEnv) (*BotResolver, *resolveDaemon, *clockAt) {
	t.Helper()
	fd := &resolveDaemon{t: t, db: e.ro, answers: map[string]string{}}
	srv := httptest.NewServer(fd)
	t.Cleanup(srv.Close)
	dc, err := daemon.New(srv.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	clk := &clockAt{now: time.Unix(1794931200, 0)}
	r := NewBotResolver(slog.New(slog.NewTextHandler(io.Discard, nil)), e.ro, dc)
	r.clock = clk.get
	return r, fd, clk
}

// addTelegramBot adds a Telegram bot with an identity and no user row.
func addTelegramBot(t *testing.T, db *sql.DB, id uint64, uid int64, username string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO telegram_bots (id, name, token, bot_user_id, username, enabled) "+
		"VALUES (?, '', ?, ?, ?, 1)", id, fmt.Sprintf("%d:resolveFixtureSecret%030d", uid, id),
		uid, username); err != nil {
		t.Fatal(err)
	}
}

func TestBotResolverPasses(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	r, fd, clk := newResolver(t, e)
	ctx := context.Background()

	addTelegramBot(t, e.ro, 100, 7000000100, "new_bot")
	addTelegramBot(t, e.ro, 101, 7000000101, "gone_bot")
	addTelegramBot(t, e.ro, 102, 7000000102, "busy_bot")
	// A Discord bot with an identity and no Telegram user of its id.
	if _, err := e.ro.Exec("INSERT INTO discord_bots (id, name, token, enabled, status, bot_user_id, " +
		"username) VALUES (1000000100, 'Reader', 'MTAwMDAwMDAwMDAwMDAwMDEwMA.GfixC.resolveFixtureDiscord', " +
		"1, 'connected', 1000000000000000099, 'discord_only')"); err != nil {
		t.Fatal(err)
	}
	fd.answer("@new_bot", okAnswer(7000000100, "new_bot"))
	fd.answer("@busy_bot", `{"outcome":"flood_wait","message":"","retry_after":600,"user":null}`)

	// The bots with an identity and no user row, and only those: not
	// bot 1, whose row exists, nor bot 2, not logged in, nor any
	// Discord bot.
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@busy_bot", "@gone_bot", "@new_bot"}) {
		t.Fatalf("first pass asked %v", got)
	}
	var n int
	if err := e.ro.QueryRow("SELECT COUNT(*) FROM telegram_users WHERE id = 7000000100").Scan(&n); err != nil || n != 1 {
		t.Fatalf("new_bot's row %d %v", n, err)
	}

	// At once again: new_bot is resolved, gone_bot backs off, and the
	// flood wait holds every bot.
	r.pass(ctx, false)
	if got := fd.took(); len(got) != 0 {
		t.Errorf("second pass asked %v", got)
	}

	// After the flood wait only busy_bot is due; gone_bot waits hours.
	fd.answer("@busy_bot", "")
	clk.add(11 * time.Minute)
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@busy_bot"}) {
		t.Errorf("after the flood wait asked %v", got)
	}
	clk.add(botRetryFailed - 11*time.Minute + time.Second)
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@gone_bot"}) {
		t.Errorf("hours later asked %v", got)
	}

	// A new identity is a new bot to ask about, whatever the old one's
	// wait.
	if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = 7000000201, username = 'renamed_bot' " +
		"WHERE id = 101"); err != nil {
		t.Fatal(err)
	}
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@renamed_bot"}) {
		t.Errorf("after a new identity asked %v", got)
	}

	// The daemon away: no wait of hours, a minute.
	fd.setStatus(http.StatusServiceUnavailable)
	clk.add(botRetryFailed + time.Minute)
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@busy_bot", "@renamed_bot"}) {
		t.Errorf("with the daemon away asked %v", got)
	}
	r.pass(ctx, false)
	if got := fd.took(); len(got) != 0 {
		t.Errorf("right after the daemon was away asked %v", got)
	}
	fd.setStatus(0)
	clk.add(botRetryAway + time.Second)
	r.pass(ctx, false)
	if got := fd.took(); !slices.Equal(got, []string{"@busy_bot", "@renamed_bot"}) {
		t.Errorf("once the daemon is back asked %v", got)
	}

	// A username that now names another account is not taken for the
	// bot's, and waits.
	fd.answer("@busy_bot", okAnswer(7000000999, "busy_bot"))
	clk.add(11 * time.Minute) // past the flood wait busy_bot answered last
	r.Nudge(102)
	r.pass(ctx, true)
	if got := fd.took(); !slices.Equal(got, []string{"@busy_bot"}) {
		t.Errorf("nudged asked %v", got)
	}
	r.pass(ctx, false)
	if got := fd.took(); len(got) != 0 {
		t.Errorf("after another account's answer asked %v", got)
	}
}

// A nudge watches the bot: the watched passes ask about it alone,
// and once it is tried, or its time is up, no more.
func TestBotResolverNudge(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	r, fd, clk := newResolver(t, e)
	ctx := context.Background()
	addTelegramBot(t, e.ro, 100, 7000000100, "new_bot")
	addTelegramBot(t, e.ro, 101, 7000000101, "other_bot")

	r.pass(ctx, true)
	if got := fd.took(); len(got) != 0 {
		t.Errorf("a watched pass with no bot watched asked %v", got)
	}
	// Nudged before discordd reported its identity: watched, nothing
	// to ask yet.
	if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = 0, username = '' WHERE id = 100"); err != nil {
		t.Fatal(err)
	}
	r.Nudge(100)
	r.Nudge(1000000001) // a Discord bot: ignored
	if !r.watching() {
		t.Fatal("the nudged bot is not watched")
	}
	r.pass(ctx, true)
	if got := fd.took(); len(got) != 0 {
		t.Errorf("a bot with no identity asked %v", got)
	}
	if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = 7000000100, username = 'new_bot' " +
		"WHERE id = 100"); err != nil {
		t.Fatal(err)
	}
	clk.add(time.Minute)
	r.pass(ctx, true)
	if got := fd.took(); !slices.Equal(got, []string{"@new_bot"}) {
		t.Errorf("the watched pass asked %v", got)
	}
	if r.watching() {
		t.Error("a tried bot is still watched")
	}
	// A nudge forgets the bot's wait.
	r.Nudge(100)
	r.pass(ctx, true)
	if got := fd.took(); !slices.Equal(got, []string{"@new_bot"}) {
		t.Errorf("nudged again asked %v", got)
	}
	// A watch ends when its time is up.
	r.Nudge(1)
	clk.add(botResolveWatchFor)
	if r.watching() {
		t.Error("a watch outlived its time")
	}
}

// Run resolves at once and when nudged, and stops with its context;
// with the daemon API off it does nothing.
func TestBotResolverRun(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, fwdPhotosFixtr)
	r, fd, _ := newResolver(t, e)
	r.clock = time.Now
	r.every, r.watchEvery = time.Hour, 20*time.Millisecond
	addTelegramBot(t, e.ro, 100, 7000000100, "first_bot")
	fd.answer("@first_bot", okAnswer(7000000100, "first_bot"))
	fd.answer("@second_bot", okAnswer(7000000101, "second_bot"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	waitAsked := func(want string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		var got []string
		for time.Now().Before(deadline) {
			got = append(got, fd.took()...)
			if slices.Contains(got, want) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("%s never asked: %v", want, got)
	}
	waitAsked("@first_bot")

	// Created now, identity reported a moment later.
	addTelegramBot(t, e.ro, 101, 0, "")
	r.Nudge(101)
	time.Sleep(50 * time.Millisecond)
	if _, err := e.ro.Exec("UPDATE telegram_bots SET bot_user_id = 7000000101, username = 'second_bot' " +
		"WHERE id = 101"); err != nil {
		t.Fatal(err)
	}
	waitAsked("@second_bot")

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not stop")
	}

	off := NewBotResolver(slog.New(slog.NewTextHandler(io.Discard, nil)), e.ro, nil)
	stopped := make(chan struct{})
	go func() {
		off.Run(context.Background())
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("an off resolver runs")
	}
	var nilR *BotResolver
	nilR.Nudge(1)
}

// Creating a Telegram bot, or giving one a new token, nudges the
// resolver; a Discord bot, or a change of name, does not.
func TestBotChangesNudgeResolver(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	r, _, _ := newResolver(t, e)
	e.with(func(d *Deps) { d.Bots = r })
	watched := func() []uint64 {
		r.mu.Lock()
		defer r.mu.Unlock()
		var out []uint64
		for id := range r.watch {
			out = append(out, id)
		}
		slices.Sort(out)
		r.watch = map[uint64]time.Time{}
		return out
	}

	b := e.want(201, "admin", "POST", "/api/bots", map[string]any{"kind": "telegram", "name": "New",
		"token": "7000000300:nudgeFixtureSecretTelegramBotOne"})
	id := uint64(b["id"].(float64))
	if got := watched(); !slices.Equal(got, []uint64{id}) {
		t.Errorf("after creating a Telegram bot watched %v", got)
	}
	e.want(201, "admin", "POST", "/api/bots", map[string]any{"kind": "discord", "name": "Reader",
		"token": "MTAwMDAwMDAwMDAwMDAwMDMwMA.GfixD.nudgeFixtureDiscordBot"})
	if got := watched(); len(got) != 0 {
		t.Errorf("after creating a Discord bot watched %v", got)
	}
	path := fmt.Sprintf("/api/bots/%d", id)
	e.want(200, "admin", "PATCH", path, map[string]any{"name": "Renamed"})
	if got := watched(); len(got) != 0 {
		t.Errorf("after a rename watched %v", got)
	}
	e.want(200, "admin", "PATCH", path, map[string]any{"token": "7000000301:nudgeFixtureSecretTelegramBotTwo"})
	if got := watched(); !slices.Equal(got, []uint64{id}) {
		t.Errorf("after a new token watched %v", got)
	}
}

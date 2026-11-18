// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/daemon"
)

// The dashboard's logger status card (DashboardProps.status), over the
// admin tests' databases and fake daemon, each test with its own.

// dashStatus is the dashboard's status as role sees it, nil when absent.
func dashStatus(e *adminEnv, role string) map[string]any {
	e.t.Helper()
	st, p := e.page(role, "/")
	if st != 200 {
		e.t.Fatalf("%s dashboard: %d %v", role, st, p)
	}
	s, _ := p["props"].(map[string]any)["status"].(map[string]any)
	return s
}

// statusEnv is an admin env whose store file system is stubbed at
// 8 TiB, and which records the paths it was asked about.
func statusEnv(t *testing.T) (*adminEnv, func() []string) {
	t.Helper()
	e := newAdminEnv(t)
	var mu sync.Mutex
	var asked []string
	// Kept in e.deps, so later e.with calls keep them.
	e.deps.Media.StorageDir = "/store/files"
	e.deps.statfs = func(p string) (uint64, error) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, p)
		return 8 << 40, nil
	}
	e.h = New(e.deps)
	return e, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), asked...)
	}
}

func TestDashboardStatusWho(t *testing.T) {
	t.Parallel()
	e, _ := statusEnv(t)
	e.daemon.answer("/v1/status", 200, "", `{"tdlib":"ready","since":1763400000}`)

	for _, role := range []string{"superadmin", "admin"} {
		if dashStatus(e, role) == nil {
			t.Errorf("%s gets no status", role)
		}
	}
	if s := dashStatus(e, "member"); s != nil {
		t.Errorf("a member gets a status: %v", s)
	}

	// An admin with a hidden group, then with private chats hidden,
	// sees less than the whole archive: no card.
	restrict := func(priv bool, groups ...any) {
		t.Helper()
		if groups == nil {
			groups = []any{}
		}
		e.want(200, "superadmin", "PATCH", "/api/accounts/3", map[string]any{
			"restrict": map[string]any{"hide_private": priv, "hide_groups": groups,
				"join_chat": false}})
	}
	restrict(false, kernelID)
	if s := dashStatus(e, "admin"); s != nil {
		t.Errorf("an admin with a hidden group gets a status: %v", s)
	}
	restrict(true)
	if s := dashStatus(e, "admin"); s != nil {
		t.Errorf("an admin without private chats gets a status: %v", s)
	}
	restrict(false)
	if dashStatus(e, "admin") == nil {
		t.Error("a lifted restriction brings no status back")
	}
}

func TestDashboardStatusSession(t *testing.T) {
	t.Parallel()
	e, _ := statusEnv(t)
	session := func() any { return dashStatus(e, "superadmin")["session"] }

	for state, want := range map[string]string{
		"ready": "connected", "waiting": "disconnected", "closing": "disconnected",
	} {
		e.daemon.answer("/v1/status", 200, "", `{"tdlib":"`+state+`","since":1763400000}`)
		if got := session(); got != want {
			t.Errorf("tdlib %s: session %v, want %s", state, got, want)
		}
	}
	e.daemon.answer("/v1/status", 503, "5", `{"error":"unavailable","message":"x"}`)
	if got := session(); got != "disconnected" {
		t.Errorf("a 503: session %v", got)
	}

	// A daemon that does not answer, and none configured.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	dc, err := daemon.New(dead.URL, testKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	e.with(func(d *Deps) { d.Daemon = dc })
	if got := session(); got != "disconnected" {
		t.Errorf("unreachable: session %v", got)
	}
	e.with(func(d *Deps) { d.Daemon = nil })
	s := dashStatus(e, "superadmin")
	if s == nil || s["session"] != "disconnected" {
		t.Errorf("API off: %v", s)
	}
}

func TestDashboardStatusLastMessage(t *testing.T) {
	t.Parallel()
	e, _ := statusEnv(t)
	set := func(table, id, at string) {
		t.Helper()
		if _, err := e.ro.Exec("UPDATE "+table+" SET created_at = ? WHERE id = "+id, at); err != nil {
			t.Fatal(err)
		}
	}
	gTop := e.one("SELECT MAX(id) FROM telegram_group_messages")
	gMin := e.one("SELECT MIN(id) FROM telegram_group_messages")
	pTop := e.one("SELECT MAX(id) FROM telegram_private_messages")
	pMin := e.one("SELECT MIN(id) FROM telegram_private_messages")
	if gTop == gMin || pTop == pMin {
		t.Fatal("the fixture has too few messages")
	}
	last := func() any { return dashStatus(e, "superadmin")["last_message_at"] }

	// The newest row by id counts, not a later created_at on an
	// older row.
	set("telegram_group_messages", gTop, "2026-11-17 15:59:56")
	set("telegram_group_messages", gMin, "2026-11-18 09:00:00")
	set("telegram_private_messages", pTop, "2026-11-17 10:00:00")
	set("telegram_private_messages", pMin, "2026-11-18 09:00:00")
	if got := last(); got != "2026-11-17T15:59:56Z" {
		t.Errorf("group newest: %v", got)
	}
	set("telegram_private_messages", pTop, "2026-11-17 16:30:00")
	if got := last(); got != "2026-11-17T16:30:00Z" {
		t.Errorf("private newest: %v", got)
	}
}

func TestDashboardStatusForwarding(t *testing.T) {
	t.Parallel()
	e, _ := statusEnv(t)
	active := func() int64 { return num(dashStatus(e, "superadmin")["forwarding_active"]) }
	index := func() int64 {
		_, p := e.page("superadmin", "/platform-fwd")
		pr := p["props"].(map[string]any)
		return num(pr["t2d"].(map[string]any)["active"]) + num(pr["d2t"].(map[string]any)["active"])
	}

	// Webhooks 1 and 2, and route 1 (route 2's bots are disabled).
	if got := active(); got != 3 || got != index() {
		t.Errorf("forwarding_active %d, want 3 (the index says %d)", got, index())
	}
	exec := func(q string) {
		t.Helper()
		if _, err := e.ro.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE telegram_discord_webhooks SET enabled = 0 WHERE id = 2")
	if got := active(); got != 2 || got != index() {
		t.Errorf("a disabled webhook: %d (the index says %d)", got, index())
	}
	exec("UPDATE discord_bots SET enabled = 0 WHERE id = 1000000001")
	if got := active(); got != 1 || got != index() {
		t.Errorf("a disabled Discord bot: %d (the index says %d)", got, index())
	}
	exec("UPDATE discord_bots SET enabled = 1 WHERE id = 1000000001")
	exec("UPDATE telegram_bots SET enabled = 0 WHERE id = 1")
	if got := active(); got != 1 || got != index() {
		t.Errorf("a disabled Telegram bot: %d (the index says %d)", got, index())
	}
}

func TestDashboardStatusStore(t *testing.T) {
	t.Parallel()
	e, asked := statusEnv(t)
	store := func() (any, any, float64) {
		s := dashStatus(e, "superadmin")
		r, _ := s["store_ratio"].(float64)
		return s["store_used"], s["store_total"], r
	}

	// A cap: the daemon's bytes of its cap.
	e.daemon.answer("/v1/storage", 200, "", `{"files":2,"bytes":1073741824,"max_bytes":4294967296}`)
	if u, tot, r := store(); u != "1.0 GiB" || tot != "4.0 GiB" || r != 0.25 {
		t.Errorf("capped: %v of %v, %v", u, tot, r)
	}
	if a := asked(); len(a) != 0 {
		t.Errorf("a cap asked statfs about %v", a)
	}

	// No cap: of the store's file system.
	e.daemon.answer("/v1/storage", 200, "", `{"files":2,"bytes":2199023255552,"max_bytes":0}`)
	if u, tot, r := store(); u != "2.0 TiB" || tot != "8.0 TiB" || r != 0.25 {
		t.Errorf("no cap: %v of %v, %v", u, tot, r)
	}
	if a := asked(); len(a) != 1 || a[0] != "/store/files" {
		t.Errorf("statfs asked about %v", a)
	}

	// Over the cap: the ratio stops at 1.
	e.daemon.answer("/v1/storage", 200, "", `{"files":2,"bytes":5368709120,"max_bytes":4294967296}`)
	if u, tot, r := store(); u != "5.0 GiB" || tot != "4.0 GiB" || r != 1 {
		t.Errorf("over the cap: %v of %v, %v", u, tot, r)
	}

	// No daemon: the stored files' sum, of the file system.
	sum, _ := strconv.ParseInt(e.one("SELECT SUM(file_size) FROM telegram_files WHERE on_disk = 1"), 10, 64)
	if sum <= 0 {
		t.Fatal("the fixture stores no bytes")
	}
	e.with(func(d *Deps) { d.Daemon = nil })
	if u, tot, r := store(); u != humanBytes(sum) || tot != "8.0 TiB" ||
		r != float64(sum)/float64(8<<40) {
		t.Errorf("no daemon: %v of %v, %v (sum %d)", u, tot, r, sum)
	}

	// No file system size either: a total of 0 and a ratio of 0.
	e.with(func(d *Deps) {
		d.Daemon = nil
		d.statfs = func(string) (uint64, error) { return 0, errors.New("no such file") }
	})
	if u, tot, r := store(); u != humanBytes(sum) || tot != "0.0 KiB" || r != 0 {
		t.Errorf("no statfs: %v of %v, %v", u, tot, r)
	}
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()
	for n, want := range map[int64]string{
		0:                        "0.0 KiB",
		-5:                       "0.0 KiB",
		512:                      "0.5 KiB",
		1024:                     "1.0 KiB",
		1048524:                  "1023.9 KiB",
		1048575:                  "1.0 MiB",
		183406:                   "179.1 KiB",
		349*1<<30 + 1<<30/5:      "349.2 GiB",
		1 << 40:                  "1.0 TiB",
		2048 << 40:               "2048.0 TiB",
		5*1<<40 + 3*1<<40/10 + 1: "5.3 TiB",
	} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
	for _, c := range []struct {
		used, total int64
		want        float64
	}{{0, 0, 0}, {5, 0, 0}, {-1, 10, 0}, {1, 4, 0.25}, {9, 4, 1}} {
		if got := storeRatio(c.used, c.total); got != c.want {
			t.Errorf("storeRatio(%d, %d) = %v, want %v", c.used, c.total, got, c.want)
		}
	}
}

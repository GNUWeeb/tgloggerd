// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"testing"

	"tgloggerd/webapi/internal/live"
	"tgloggerd/webapi/internal/scope"
)

// bot.updated reaches the viewers who see every bot, on the bots topic.
func TestBotEventScope(t *testing.T) {
	t.Parallel()
	e := live.Event{Seq: 1, Kind: live.KindBotUpdated, BotID: discordBot}
	for role, want := range map[scope.Role]bool{scope.Superadmin: true, scope.Admin: true, scope.Member: false} {
		if _, ok := eventInScope(scope.For(scope.Viewer{ID: 9, Role: role}), e); ok != want {
			t.Errorf("%s: %v", role, ok)
		}
	}
	if _, ok := eventInScope(scope.Anonymous(), e); ok {
		t.Error("anonymous")
	}
	tp, err := parseTopics("bots")
	if err != nil || !tp.wants(e) {
		t.Errorf("bots topic %v %v", tp, err)
	}
	if tp, _ := parseTopics("stats"); tp.wants(e) {
		t.Error("stats topic wants bot.updated")
	}
	if !live.Known(live.KindBotUpdated) {
		t.Error("bot.updated unknown to the hub")
	}
}

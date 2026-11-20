// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"net/url"
	"testing"
)

// A user's message total counts every group and private message the
// user sent (telegram_users.msg_count). An admin from whom a group or
// the private chats are hidden sees it less those messages, in the
// users list, its sort and its filters, and on the user page; a
// superadmin sees the stored total.
func TestHiddenTotals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	const (
		poster  = 9299000007 // two messages in kernelID
		private = 9299000001 // sends private messages too
	)
	stored := func(id int64) int64 {
		t.Helper()
		var n int64
		if err := e.ro.QueryRow("SELECT msg_count FROM telegram_users WHERE id = ?", id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	sent := func(q string, id int64) int64 {
		t.Helper()
		var n int64
		if err := e.ro.QueryRow(q, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// What the admin may not see of each user's messages: those in the
	// hidden group and the private ones.
	hidden := func(id int64) int64 {
		return sent("SELECT COUNT(*) FROM telegram_group_messages WHERE sender_user_id = ? AND chat_id = "+
			fmt.Sprint(kernelID), id) +
			sent("SELECT COUNT(*) FROM telegram_private_messages WHERE sender_id = ?", id)
	}
	if hidden(poster) == 0 || sent("SELECT COUNT(*) FROM telegram_private_messages WHERE sender_id = ?", private) == 0 {
		t.Fatal("fixture: the users send nothing the admin may not see")
	}
	// The stored totals must hold those messages for the test to mean
	// anything; the sample's are counted from its messages.
	if _, err := e.ro.Exec("UPDATE telegram_users SET msg_count = msg_count + 100 WHERE id IN (?, ?)",
		poster, private); err != nil {
		t.Skip("the read-only pool cannot write the fixture: ", err)
	}

	listed := func(who string, id int64, extra string) (int64, bool) {
		t.Helper()
		f := url.QueryEscape(fmt.Sprintf(`[{"field":"id","op":"=","value":"%d"}%s]`, id, extra))
		_, p := e.page(who, "/users?f="+f)
		rows := p["props"].(map[string]any)["rows"].([]any)
		if len(rows) == 0 {
			return 0, false
		}
		return int64(rows[0].(map[string]any)["msg_count"].(float64)), true
	}
	onPage := func(who string, id int64) int64 {
		t.Helper()
		_, p := e.page(who, fmt.Sprintf("/users/%d", id))
		return int64(p["props"].(map[string]any)["user"].(map[string]any)["msg_count"].(float64))
	}

	e.want(200, "superadmin", "PATCH", "/api/accounts/3", map[string]any{"restrict": map[string]any{
		"hide_private": true, "hide_groups": []any{kernelID}, "join_chat": false}})

	for _, c := range []struct{ id int64 }{{poster}, {private}} {
		want := stored(c.id) - hidden(c.id)
		if n, ok := listed("admin", c.id, ""); !ok || n != want {
			t.Errorf("%d listed to the admin: %d, want %d", c.id, n, want)
		}
		if n := onPage("admin", c.id); n != want {
			t.Errorf("%d on the admin's user page: %d, want %d", c.id, n, want)
		}
		if n, _ := listed("superadmin", c.id, ""); n != stored(c.id) {
			t.Errorf("%d listed to a superadmin: %d, want the stored %d", c.id, n, stored(c.id))
		}
		// A filter on the total uses the admin's total: the stored one
		// no longer matches, the admin's does.
		if _, ok := listed("admin", c.id, fmt.Sprintf(`,{"field":"msg_count","op":"=","value":"%d"}`, stored(c.id))); ok {
			t.Errorf("%d matched the stored total for the admin", c.id)
		}
		if _, ok := listed("admin", c.id, fmt.Sprintf(`,{"field":"msg_count","op":"=","value":"%d"}`, want)); !ok {
			t.Errorf("%d did not match the admin's total", c.id)
		}
	}
	// The sort is by the admin's totals as well.
	_, p := e.page("admin", "/users?sort=msg_count&dir=desc&per=100")
	prev := int64(1 << 62)
	for _, r := range p["props"].(map[string]any)["rows"].([]any) {
		n := int64(r.(map[string]any)["msg_count"].(float64))
		if n > prev {
			t.Fatalf("sorted rows go up: %d after %d", n, prev)
		}
		prev = n
	}
}

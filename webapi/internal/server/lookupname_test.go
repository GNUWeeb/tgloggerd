// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"

	"tgloggerd/webapi/internal/i18n"
)

// "Look up real name" against the fake daemon (adminEnv), on the
// sample's contacts: Dewi Kusuma (9299000001) is one, Andi Santoso
// (9299000004) is not.

const (
	dewiLookup = "/api/users/9299000001/lookup-name"
	dewiDaemon = "/v1/users/9299000001/lookup-name"
)

func TestLookupName(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer(dewiDaemon, 200, "", `{"user_id": 9299000001, "first_name": "Dewi",
		"last_name": "K.", "contact_first_name": "Dewi", "contact_last_name": "(kernel)",
		"checked_at": 1794931200}`)
	b := e.want(200, "superadmin", "POST", dewiLookup, nil)
	if js(b) != canon(`{"user_id": 9299000001, "first_name": "Dewi", "last_name": "K.",
		"contact_first_name": "Dewi", "contact_last_name": "(kernel)", "checked_at": 1794931200}`) {
		t.Errorf("answer %v", b)
	}

	// The daemon's refusals keep their code; Retry-After passes on.
	for _, c := range []struct {
		status     int
		retryAfter string
		code       string
		want       int
	}{
		{429, "42", "rate_limited", 429},
		{429, "10", "flood_wait", 429},
		{409, "", "not_a_contact", 409},
		{409, "", "unsupported", 409},
		{503, "60", "restore_pending", 503},
		{503, "5", "unavailable", 503},
		{504, "", "timeout", 504},
		{502, "", "telegram_error", 502},
		{404, "", "not_found", 404},
		{500, "", "internal", 502},
	} {
		e.daemon.answer(dewiDaemon, c.status, c.retryAfter, `{"error": "`+c.code+`", "message": "x"}`)
		st, b, h := e.call("superadmin", "POST", dewiLookup, nil)
		code := c.code
		if c.want == 502 && c.code == "internal" {
			code = "daemon"
		}
		if st != c.want || b["error"] != code || b["message"] == "" || h.Get("Retry-After") != c.retryAfter {
			t.Errorf("%s: %d %v %q", c.code, st, b, h.Get("Retry-After"))
		}
	}

	want := []string{"1 contact_lookup user 9299000001: ok"}
	for _, c := range []string{"rate_limited", "flood_wait", "not_a_contact", "unsupported",
		"restore_pending", "unavailable", "timeout", "telegram_error", "not_found", "internal"} {
		want = append(want, "1 contact_lookup user 9299000001: "+c)
	}
	if got := e.audits(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestLookupNameRefusals(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	e.daemon.answer("/v1/users/9299000004/lookup-name", 200, "", `{}`)
	e.daemon.answer(dewiDaemon, 200, "", `{}`)
	for _, c := range []struct {
		role, path string
		status     int
		code       string
	}{
		// For superadmins only: to an admin or a member it does not
		// exist.
		{"admin", dewiLookup, 404, "not_found"},
		{"member", dewiLookup, 404, "not_found"},
		// A user the logger never saw, and nonsense ids.
		{"superadmin", "/api/users/9299999999/lookup-name", 404, "not_found"},
		{"superadmin", "/api/users/-5/lookup-name", 404, "not_found"},
		{"superadmin", "/api/users/x/lookup-name", 404, "not_found"},
		// Not a contact: refused before the daemon is asked.
		{"superadmin", "/api/users/9299000004/lookup-name", 409, "not_a_contact"},
	} {
		st, b, _ := e.call(c.role, "POST", c.path, nil)
		if st != c.status || b["error"] != c.code {
			t.Errorf("%s %s: %d %v, want %d %s", c.role, c.path, st, b, c.status, c.code)
		}
	}
	if st, b, _ := e.call("superadmin", "POST", dewiLookup, nil, false); st != 403 || b["error"] != "csrf" {
		t.Errorf("no csrf %d %v", st, b)
	}
	if st, _, _ := e.call("", "POST", dewiLookup, nil); st != 401 {
		t.Errorf("anonymous %d", st)
	}
	if c := e.daemon.calls(); len(c) != 0 {
		t.Errorf("the daemon was asked: %q", c)
	}
	if a := e.audits(); len(a) != 0 {
		t.Errorf("refusals audited: %v", a)
	}

	// Without the daemon's API: 502, audited as off.
	e.with(func(d *Deps) { d.Daemon = nil })
	if st, b, _ := e.call("superadmin", "POST", dewiLookup, nil); st != 502 || b["error"] != "daemon" {
		t.Errorf("off %d %v", st, b)
	}
	if a := e.audits(); len(a) != 1 || a[0] != "1 contact_lookup user 9299000001: off" {
		t.Errorf("audit %v", a)
	}
}

// Every daemon error passed on has its sentence.
func TestLookupTexts(t *testing.T) {
	t.Parallel()
	for code := range lookupErrors {
		if k := "lookup." + code; i18n.T(i18n.Fallback, k) == k {
			t.Errorf("%s has no English text", k)
		}
	}
}

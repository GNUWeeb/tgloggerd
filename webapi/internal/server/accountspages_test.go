// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"strings"
	"testing"
)

// The accounts and federations pages (adminEnv, with the "accounts"
// fixture's accounts: gw_admin and sec_lead administer Default, the
// members are filed in it). Their federation data is
// federationprops.go's, tested in federations_test.go; these check the
// loaders' wiring and who gets the pages.

func TestAccountsPage(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)

	st, b := e.page("superadmin", "/settings/accounts")
	if st != 200 || b["route"] != "accounts" {
		t.Fatalf("page %d %v", st, b)
	}
	p := b["props"].(map[string]any)
	if js(p["editable"]) != canon(js(accountsEditable())) {
		t.Errorf("editable %v", p["editable"])
	}
	if js(p["federations"]) != `[{"default":true,"group_count":6,"id":0,"name":"Default"}]` ||
		js(p["all_federations"]) != `[{"default":true,"id":0,"name":"Default"}]` {
		t.Errorf("federations %v %v", p["federations"], p["all_federations"])
	}
	// The same rows as GET /api/accounts.
	_, list, _ := e.call("superadmin", "GET", "/api/accounts", nil)
	if js(p["accounts"]) != js(list["accounts"]) || len(p["accounts"].([]any)) != 7 {
		t.Errorf("accounts %s\nwant %s", js(p["accounts"]), js(list["accounts"]))
	}

	// An admin manages the members of its federations only.
	_, b = e.page("admin", "/settings/accounts")
	var names []string
	for _, a := range b["props"].(map[string]any)["accounts"].([]any) {
		names = append(names, a.(map[string]any)["username"].(string))
	}
	if strings.Join(names, ",") != "intern2025,mod_kernel,researcher" ||
		js(b["props"].(map[string]any)["federations"]) != `[{"default":true,"group_count":6,"id":0,"name":"Default"}]` {
		t.Errorf("admin's page %s", js(b["props"]))
	}

	if st, b := e.page("member", "/settings/accounts"); st != 404 || b["route"] != "not_found" {
		t.Errorf("member %d %v", st, b)
	}
}

func TestFederationsPage(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t)
	for _, c := range []struct {
		role     string
		editable bool
		options  string
	}{
		{"superadmin", true, "3,4"},
		{"admin", false, ""},
	} {
		st, b := e.page(c.role, "/settings/federations")
		if st != 200 || b["route"] != "federations" {
			t.Fatalf("%s page %d %v", c.role, st, b)
		}
		p := b["props"].(map[string]any)
		if p["editable"] != c.editable || idsOf(p["admin_options"]) != c.options {
			t.Errorf("%s props %s", c.role, js(p))
		}
		feds := p["federations"].([]any)
		if len(feds) != 1 {
			t.Fatalf("%s federations %v", c.role, feds)
		}
		f := feds[0].(map[string]any)
		if f["id"] != float64(0) || f["default"] != true || f["group_count"] != float64(6) ||
			len(f["groups"].([]any)) != 6 || idsOf(f["admins"]) != "3,4" {
			t.Errorf("%s Default %s", c.role, js(f))
		}
	}
	if st, b := e.page("member", "/settings/federations"); st != 404 || b["route"] != "not_found" {
		t.Errorf("member %d %v", st, b)
	}
}

// idsOf is the ids of a list of objects, comma-separated.
func idsOf(v any) string {
	var out []string
	l, _ := v.([]any)
	for _, x := range l {
		out = append(out, js(x.(map[string]any)["id"]))
	}
	return strings.Join(out, ",")
}

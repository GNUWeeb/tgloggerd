// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package scope

import (
	"slices"
	"testing"
)

func TestParseRole(t *testing.T) {
	for _, r := range []string{"superadmin", "admin", "member"} {
		if got, ok := ParseRole(r); !ok || string(got) != r {
			t.Errorf("ParseRole(%q) = %q, %v", r, got, ok)
		}
	}
	// The roles before web migration 000004 are not roles any more.
	for _, r := range []string{"viewer", "", "Admin", "root"} {
		if _, ok := ParseRole(r); ok {
			t.Errorf("ParseRole(%q) accepted", r)
		}
	}
}

func TestVisibility(t *testing.T) {
	super := For(Viewer{ID: 1, Role: Superadmin, EmailVerified: true})
	admin := For(Viewer{ID: 2, Role: Admin, EmailVerified: true})
	member := For(Viewer{ID: 3, Role: Member, EmailVerified: true})
	for _, tc := range []struct {
		name               string
		s                  Scope
		groups, priv, user bool
		filter             string
	}{
		{"superadmin", super, true, true, true, "1 = 1"},
		{"admin", admin, true, true, true, "1 = 1"},
		{"member", member, false, false, false, "1 = 0"},
		{"anonymous", Anonymous(), false, false, false, "1 = 0"},
		{"zero", Scope{}, false, false, false, "1 = 0"},
		{"everything", Everything(), true, true, true, "1 = 1"},
		{"unknown role", For(Viewer{ID: 4, Role: "viewer"}), false, false, false, "1 = 0"},
	} {
		if tc.s.SeesAllGroups() != tc.groups || tc.s.SeesGroup(-100) != tc.groups ||
			tc.s.SeesPrivate() != tc.priv || tc.s.SeesAllUsers() != tc.user ||
			tc.s.SeesChat(-100123) != tc.groups || tc.s.SeesChat(42) != tc.priv ||
			tc.s.SeesUser(42) != tc.user {
			t.Errorf("%s: groups %v private %v users %v", tc.name,
				tc.s.SeesAllGroups(), tc.s.SeesPrivate(), tc.s.SeesAllUsers())
		}
		if f, args := tc.s.GroupFilter("m.chat_id"); f != tc.filter || len(args) != 0 {
			t.Errorf("%s: GroupFilter = %q %v", tc.name, f, args)
		}
	}
}

func TestAbilitiesAndPages(t *testing.T) {
	super := For(Viewer{ID: 1, Role: Superadmin, EmailVerified: true})
	admin := For(Viewer{ID: 2, Role: Admin, EmailVerified: true})
	member := For(Viewer{ID: 3, Role: Member, EmailVerified: true})
	all := []Ability{JoinChat, LookupName, ManageMCP, StoreAudit, Forward, FetchFiles}
	for _, a := range all {
		if !super.Can(a) {
			t.Errorf("superadmin cannot %s", a)
		}
		if admin.Can(a) == (a == JoinChat || a == LookupName) {
			t.Errorf("admin Can(%s) = %v", a, admin.Can(a))
		}
		if member.Can(a) || Everything().Can(a) || Anonymous().Can(a) {
			t.Errorf("%s allowed without a grant", a)
		}
	}
	for _, tc := range []struct {
		s    Scope
		want []string
	}{
		{super, []string{"dashboard", "users", "groups", "group_messages",
			"private_messages", "files", "platform_fwd", "join_chat", "mcp",
			"accounts", "federations", "settings"}},
		{admin, []string{"dashboard", "users", "groups", "group_messages",
			"private_messages", "files", "platform_fwd", "mcp", "accounts",
			"federations", "settings"}},
		{member, []string{"dashboard", "users", "groups", "group_messages",
			"files", "settings"}},
		{Anonymous(), []string{}},
		{Everything(), []string{}},
	} {
		if got := tc.s.Nav(); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Nav = %v, want %v", tc.s, got, tc.want)
		}
	}
	if member.Page("nonsense") || super.Page("nonsense") {
		t.Error("an unknown page key is visible")
	}
}

func TestManagingAccounts(t *testing.T) {
	super := For(Viewer{ID: 1, Role: Superadmin, EmailVerified: true})
	admin := For(Viewer{ID: 2, Role: Admin, EmailVerified: true})
	member := For(Viewer{ID: 3, Role: Member, EmailVerified: true})
	roles := []Role{Superadmin, Admin, Member}

	if !super.ManagesAccounts() || !admin.ManagesAccounts() || member.ManagesAccounts() ||
		Everything().ManagesAccounts() {
		t.Error("ManagesAccounts")
	}
	for _, r := range roles {
		target := Account{ID: 9, Role: r}
		if !super.SeesAccount(target) || !super.MayGiveRole(r) {
			t.Errorf("superadmin and a %s", r)
		}
		if admin.SeesAccount(target) != (r == Member) || admin.MayGiveRole(r) != (r == Member) {
			t.Errorf("admin and a %s", r)
		}
		if member.SeesAccount(target) || member.MayGiveRole(r) {
			t.Errorf("member and a %s", r)
		}
	}
	// An admin does not manage itself.
	if admin.SeesAccount(Account{ID: 2, Role: Admin}) {
		t.Error("admin sees itself")
	}
	if !super.AdministersFederation(DefaultFederation) ||
		!admin.AdministersFederation(DefaultFederation) ||
		member.AdministersFederation(DefaultFederation) || admin.AdministersFederation(5) {
		t.Error("AdministersFederation")
	}
}

func TestGate(t *testing.T) {
	for _, tc := range []struct {
		v    Viewer
		want Gate
	}{
		{Viewer{ID: 1, Role: Admin, EmailVerified: true}, None},
		{Viewer{ID: 1, Role: Admin, EmailVerified: true, MustChangePassword: true}, PasswordChange},
		// The password comes first: changing the address needs it.
		{Viewer{ID: 1, Role: Member, MustChangePassword: true}, PasswordChange},
		{Viewer{ID: 1, Role: Member}, EmailRequired},
	} {
		if got := For(tc.v).Gate(); got != tc.want {
			t.Errorf("%+v: Gate = %q, want %q", tc.v, got, tc.want)
		}
	}
	if Anonymous().Gate() != None || Everything().Gate() != None {
		t.Error("a gate without an account")
	}
}

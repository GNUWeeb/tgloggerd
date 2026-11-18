// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package scope

import (
	"reflect"
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
	admin := For(Viewer{ID: 2, Role: Admin, EmailVerified: true, AdminOf: []uint64{DefaultFederation}})
	member := For(Viewer{ID: 3, Role: Member, EmailVerified: true})
	roles := []Role{Superadmin, Admin, Member}

	if !super.ManagesAccounts() || !admin.ManagesAccounts() || member.ManagesAccounts() ||
		Everything().ManagesAccounts() {
		t.Error("ManagesAccounts")
	}
	for _, r := range roles {
		target := Account{ID: 9, Role: r, Federations: []uint64{DefaultFederation}}
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

// Federations limit what an admin manages (web-accounts.md section 3):
// the members filed in a federation it administers, whichever others
// they are in, and nothing else; never what it sees.
func TestFederations(t *testing.T) {
	super := For(Viewer{ID: 1, Role: Superadmin, EmailVerified: true})
	// Unsorted, with a repeat: For keeps it sorted and unique.
	fedA := For(Viewer{ID: 2, Role: Admin, EmailVerified: true, AdminOf: []uint64{7, 3, 7}})
	defAdmin := For(Viewer{ID: 3, Role: Admin, EmailVerified: true, AdminOf: []uint64{DefaultFederation}})
	none := For(Viewer{ID: 4, Role: Admin, EmailVerified: true})
	// A member's AdminOf means nothing.
	member := For(Viewer{ID: 5, Role: Member, EmailVerified: true, AdminOf: []uint64{3}})

	for _, tc := range []struct {
		name string
		s    Scope
		feds map[uint64]bool // federation -> administers
		ids  []uint64
		all  bool
	}{
		{"superadmin", super, map[uint64]bool{0: true, 3: true, 7: true, 99: true}, nil, true},
		{"admin of 3 and 7", fedA, map[uint64]bool{0: false, 3: true, 7: true, 99: false}, []uint64{3, 7}, false},
		{"admin of Default", defAdmin, map[uint64]bool{0: true, 3: false}, []uint64{0}, false},
		{"admin of none", none, map[uint64]bool{0: false, 3: false}, nil, false},
		{"member", member, map[uint64]bool{0: false, 3: false}, nil, false},
		{"anonymous", Anonymous(), map[uint64]bool{0: false}, nil, false},
		{"everything", Everything(), map[uint64]bool{0: false}, nil, false},
	} {
		for f, want := range tc.feds {
			if got := tc.s.AdministersFederation(f); got != want {
				t.Errorf("%s: AdministersFederation(%d) = %v", tc.name, f, got)
			}
		}
		if ids, all := tc.s.AdministeredFederations(); !slices.Equal(ids, tc.ids) || all != tc.all {
			t.Errorf("%s: AdministeredFederations = %v %v", tc.name, ids, all)
		}
		if tc.s.ManagesFederations() != (tc.name == "superadmin") {
			t.Errorf("%s: ManagesFederations", tc.name)
		}
		// Federations change no visibility: every admin sees all.
		if tc.s.SeesAllGroups() != (tc.s.Role() == Superadmin || tc.s.Role() == Admin ||
			tc.name == "everything") {
			t.Errorf("%s: SeesAllGroups = %v", tc.name, tc.s.SeesAllGroups())
		}
	}

	for _, tc := range []struct {
		a                 Account
		super, fedA, def  bool
		noneSees, memSees bool
	}{
		{Account{ID: 10, Role: Member, Federations: []uint64{3}}, true, true, false, false, false},
		{Account{ID: 11, Role: Member, Federations: []uint64{0, 7}}, true, true, true, false, false},
		{Account{ID: 12, Role: Member, Federations: []uint64{0}}, true, false, true, false, false},
		{Account{ID: 13, Role: Member, Federations: []uint64{5}}, true, false, false, false, false},
		// A member in no federation is the superadmins' alone.
		{Account{ID: 14, Role: Member}, true, false, false, false, false},
		// Admins and superadmins, whatever their rows say, are never
		// an admin's.
		{Account{ID: 15, Role: Admin, Federations: []uint64{3}}, true, false, false, false, false},
		{Account{ID: 16, Role: Superadmin, Federations: []uint64{0}}, true, false, false, false, false},
	} {
		if super.SeesAccount(tc.a) != tc.super || fedA.SeesAccount(tc.a) != tc.fedA ||
			defAdmin.SeesAccount(tc.a) != tc.def || none.SeesAccount(tc.a) != tc.noneSees ||
			member.SeesAccount(tc.a) != tc.memSees {
			t.Errorf("%+v: superadmin %v, admin of 3 and 7 %v, admin of Default %v, "+
				"admin of none %v", tc.a, super.SeesAccount(tc.a), fedA.SeesAccount(tc.a),
				defAdmin.SeesAccount(tc.a), none.SeesAccount(tc.a))
		}
	}

	// The same rows build equal scopes, in any order.
	if !reflect.DeepEqual(fedA, For(Viewer{ID: 2, Role: Admin, EmailVerified: true, AdminOf: []uint64{3, 7}})) ||
		!reflect.DeepEqual(none, For(Viewer{ID: 4, Role: Admin, EmailVerified: true, AdminOf: []uint64{}})) {
		t.Error("scopes of the same rows differ")
	}
}

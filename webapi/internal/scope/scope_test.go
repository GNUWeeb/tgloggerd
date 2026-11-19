// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package scope

import (
	"reflect"
	"slices"
	"strings"
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
	super := For(Viewer{ID: 1, Role: Superadmin})
	admin := For(Viewer{ID: 2, Role: Admin})
	member := For(Viewer{ID: 3, Role: Member})
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
			tc.s.SeesChat(-100123) != tc.groups || tc.s.SeesChat(42) != tc.priv {
			t.Errorf("%s: groups %v private %v users %v", tc.name,
				tc.s.SeesAllGroups(), tc.s.SeesPrivate(), tc.s.SeesAllUsers())
		}
		if f, args := tc.s.GroupFilter("m.chat_id"); f != tc.filter || len(args) != 0 {
			t.Errorf("%s: GroupFilter = %q %v", tc.name, f, args)
		}
		want := map[bool]string{true: "1 = 1", false: "1 = 0"}[tc.user]
		if f, args := tc.s.UserFilter("u.id"); f != want || len(args) != 0 {
			t.Errorf("%s: UserFilter = %q %v", tc.name, f, args)
		}
	}
}

func TestAbilitiesAndPages(t *testing.T) {
	super := For(Viewer{ID: 1, Role: Superadmin})
	admin := For(Viewer{ID: 2, Role: Admin})
	member := For(Viewer{ID: 3, Role: Member})
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
	super := For(Viewer{ID: 1, Role: Superadmin})
	admin := For(Viewer{ID: 2, Role: Admin, AdminOf: []uint64{DefaultFederation}})
	member := For(Viewer{ID: 3, Role: Member})
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
		{Viewer{ID: 1, Role: Admin}, None},
		{Viewer{ID: 1, Role: Admin, MustChangePassword: true}, PasswordChange},
		// No address is required.
		{Viewer{ID: 1, Role: Member}, None},
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
	super := For(Viewer{ID: 1, Role: Superadmin})
	// Unsorted, with a repeat: For keeps it sorted and unique.
	fedA := For(Viewer{ID: 2, Role: Admin, AdminOf: []uint64{7, 3, 7}})
	defAdmin := For(Viewer{ID: 3, Role: Admin, AdminOf: []uint64{DefaultFederation}})
	none := For(Viewer{ID: 4, Role: Admin})
	// A member's AdminOf means nothing.
	member := For(Viewer{ID: 5, Role: Member, AdminOf: []uint64{3}})

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
			// Its admins rename and describe a federation.
			if got := tc.s.DescribesFederation(f); got != want {
				t.Errorf("%s: DescribesFederation(%d) = %v", tc.name, f, got)
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
	if !reflect.DeepEqual(fedA, For(Viewer{ID: 2, Role: Admin, AdminOf: []uint64{3, 7}})) ||
		!reflect.DeepEqual(none, For(Viewer{ID: 4, Role: Admin, AdminOf: []uint64{}})) {
		t.Error("scopes of the same rows differ")
	}
}

// An admin's hidden groups (web-admin-restrictions): every other group
// stays visible, the filter names the hidden ones, and the list means
// nothing for the other roles.
func TestHiddenGroups(t *testing.T) {
	t.Parallel()
	a := For(Viewer{ID: 3, Role: Admin, HideGroups: []int64{-12, -10, -12}})
	if a.SeesAllGroups() || a.SeesGroup(-12) || a.SeesGroup(-10) || !a.SeesGroup(-11) ||
		a.SeesChat(-10) || !a.SeesChat(-11) {
		t.Error("an admin's hidden groups")
	}
	if cond, args := a.GroupFilter("m.chat_id"); cond != "m.chat_id NOT IN (?, ?)" ||
		len(args) != 2 || args[0] != int64(-12) || args[1] != int64(-10) {
		t.Errorf("filter %q %v", cond, args)
	}
	for _, r := range []Role{Superadmin, Member} {
		s := For(Viewer{ID: 1, Role: r, HideGroups: []int64{-10}})
		if s.v.HideGroups != nil {
			t.Errorf("%s keeps hidden groups", r)
		}
	}
	if s := For(Viewer{ID: 1, Role: Superadmin, HideGroups: []int64{-10}}); !s.SeesGroup(-10) || !s.SeesAllGroups() {
		t.Error("a superadmin is never restricted")
	}
	if s := For(Viewer{ID: 3, Role: Admin}); !s.SeesAllGroups() {
		t.Error("an unrestricted admin")
	}
}

// A member's grants (web-member-access): it sees exactly its granted
// groups, the users who appear in them (by the filter's subqueries),
// no private chat, and may do exactly the abilities it was given. The
// grants mean nothing for the other roles.
func TestMemberGrants(t *testing.T) {
	t.Parallel()
	m := For(Viewer{ID: 5, Role: Member, GrantedGroups: []int64{-12, -10, -12},
		Abilities: []Ability{FetchFiles, JoinChat, FetchFiles}})
	if m.SeesAllGroups() || !m.SeesGroup(-12) || !m.SeesGroup(-10) || m.SeesGroup(-11) ||
		!m.SeesChat(-10) || m.SeesChat(-11) || m.SeesChat(42) || m.SeesPrivate() || m.SeesAllUsers() {
		t.Error("a member's granted groups")
	}
	if cond, args := m.GroupFilter("m.chat_id"); cond != "m.chat_id IN (?, ?)" ||
		!reflect.DeepEqual(args, []any{int64(-12), int64(-10)}) {
		t.Errorf("group filter %q %v", cond, args)
	}
	cond, args := m.UserFilter("u.id")
	if !strings.Contains(cond, "u.id IN (SELECT uf_m.sender_user_id FROM telegram_group_messages uf_m "+
		"WHERE uf_m.chat_id IN (?, ?))") ||
		!strings.Contains(cond, "u.id IN (SELECT uf_a.user_id FROM telegram_group_admins uf_a "+
			"WHERE uf_a.group_id IN (?, ?))") ||
		!reflect.DeepEqual(args, []any{int64(-12), int64(-10), int64(-12), int64(-10)}) {
		t.Errorf("user filter %q %v", cond, args)
	}
	if !slices.Equal(m.GrantedGroups(), []int64{-12, -10}) {
		t.Errorf("GrantedGroups = %v", m.GrantedGroups())
	}
	for _, a := range []Ability{JoinChat, LookupName, ManageMCP, StoreAudit, Forward, MCPTokens} {
		if m.Can(a) {
			t.Errorf("member Can(%s) without it", a)
		}
	}
	if !m.Can(FetchFiles) {
		t.Error("member cannot fetch_files it was given")
	}
	f := For(Viewer{ID: 5, Role: Member, Abilities: []Ability{Forward}})
	if !f.Can(Forward) || f.Can(FetchFiles) || !f.Page("platform_fwd") || f.SeesAllForwarding() ||
		f.SeesAllBots() {
		t.Error("a member with forward")
	}
	mt := For(Viewer{ID: 5, Role: Member, Abilities: []Ability{MCPTokens}})
	if !mt.Can(MCPTokens) || mt.Can(ManageMCP) || mt.Can(Forward) {
		t.Error("a member with mcp_tokens")
	}
	if a := For(Viewer{ID: 3, Role: Admin, Abilities: []Ability{MCPTokens}}); !a.Can(MCPTokens) {
		t.Error("an admin without mcp_tokens")
	}
	// Without grants: nothing.
	none := For(Viewer{ID: 6, Role: Member})
	if c, a := none.GroupFilter("g.id"); c != "1 = 0" || a != nil {
		t.Errorf("no grants: %q", c)
	}
	if c, _ := none.UserFilter("u.id"); c != "1 = 0" {
		t.Errorf("no grants: user filter %q", c)
	}
	// The same rows build equal scopes, in any order (the live
	// streams' recheck).
	if !reflect.DeepEqual(m, For(Viewer{ID: 5, Role: Member, GrantedGroups: []int64{-10, -12},
		Abilities: []Ability{FetchFiles}})) {
		t.Error("scopes of the same grants differ")
	}
	for _, r := range []Role{Superadmin, Admin} {
		s := For(Viewer{ID: 1, Role: r, GrantedGroups: []int64{-10}, Abilities: []Ability{Forward}})
		if s.v.GrantedGroups != nil || s.v.Abilities != nil || s.GrantedGroups() != nil {
			t.Errorf("%s keeps a member's grants", r)
		}
	}
}

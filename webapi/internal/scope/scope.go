// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package scope is what one request may see and do: the request scope
// of docs/design/web-accounts.md section 9. It is built once per
// request (Server.requestScope) and passed to every query over the
// logger's schema and to every permission check, so visibility is
// decided in one place and a query without a scope stands out in
// review. Something outside the scope answers 404, never 403, so its
// existence does not leak; 403 is for a forbidden action on something
// the request may see.
//
// The scope is built from the account's role and, for an admin, the
// federations it administers and its restrictions (hidden groups,
// hidden private messages), for a member the groups granted to it and
// its abilities. The scope never runs a query itself: what needs one
// (whether a member sees a user) is a condition it hands to the query
// (UserFilter), which the logdb package runs.
package scope

import (
	"slices"
	"strconv"
	"strings"
)

// Role is an account's role (web-accounts.md section 2).
type Role string

// The roles, as web_users.role stores them.
const (
	Superadmin Role = "superadmin"
	Admin      Role = "admin"
	Member     Role = "member"
)

// ParseRole reads a role as stored or as an API request names it.
func ParseRole(s string) (Role, bool) {
	switch r := Role(s); r {
	case Superadmin, Admin, Member:
		return r, true
	}
	return "", false
}

// Ability is something a request may do beyond seeing.
type Ability string

// The abilities. Forward and FetchFiles are the ones a member can be
// given (web-accounts.md section 5); the others are admin actions on
// the shared Telegram account (section 4).
const (
	JoinChat   Ability = "join_chat"   // join groups (Join chat page)
	LookupName Ability = "lookup_name" // "Look up real name": changes contacts
	ManageMCP  Ability = "mcp"         // MCP exposure and tokens
	StoreAudit Ability = "store_audit" // the store audit
	Forward    Ability = "forward"     // set up forwarding
	FetchFiles Ability = "fetch_files" // fetch unstored files from Telegram
)

// Gate is a step a signed-in account must take before anything else.
// While one applies, the API answers 403 with the gate as the error
// code, except for the operations that take the step (and session and
// logout), and page data answers the gate's route instead of the page.
type Gate string

// The gates; None is "".
const (
	None Gate = ""
	// PasswordChange: the account signed in with a temporary password
	// and must choose its own
	// (decisions/2026-11-18-temporary-passwords.md).
	PasswordChange Gate = "password_change_required"
	// An address is optional, so there is no gate on a missing or
	// unverified one (decisions/2026-11-18-email-optional.md).
)

// DefaultFederation is the id of the Default federation, the groups in
// no other federation (web-accounts.md section 3), in the API and in
// web_federations.
const DefaultFederation uint64 = 0

// Viewer is the signed-in account behind a request, as its account row
// says now (not as its cookie said at sign-in).
type Viewer struct {
	ID                 uint64
	Role               Role
	MustChangePassword bool
	// AdminOf is the federations an admin administers
	// (web_federation_admins), in ascending order; nil for the other
	// roles, as a superadmin administers every federation without
	// being listed.
	AdminOf []uint64
	// HidePrivate is an admin's restriction: private chats and their
	// messages are hidden from it (web-admin-restrictions). For clears
	// it for the other roles.
	HidePrivate bool
	// HideGroups are the groups hidden from an admin, by chat id. For
	// sorts them and clears them for the other roles.
	HideGroups []int64
	// GrantedGroups are the groups granted to a member, by chat id:
	// the union over its federations (web-member-access). Abilities
	// are the abilities given to a member (Forward, FetchFiles). For
	// sorts both and clears them for the other roles.
	GrantedGroups []int64
	Abilities     []Ability
}

// Account is an account the request acts on, as far as the rules of
// who manages whom need it.
type Account struct {
	ID   uint64
	Role Role
	// Federations is the federations a member is filed in
	// (web_federation_members); unused for the other roles.
	Federations []uint64
}

type kind int

const (
	anonymous kind = iota // the zero Scope: sees nothing
	everything
	viewer
)

// Scope is the visibility of one request. The zero value is Anonymous.
type Scope struct {
	kind kind
	v    Viewer
}

// Everything is the scope of a request authorised by something other
// than an account, such as a file token: it sees every row and may do
// nothing.
func Everything() Scope {
	return Scope{kind: everything}
}

// Anonymous is the scope of a request without a valid session: it
// sees nothing and may do nothing. Public pages (groups exposed
// through MCP) are decided by their own rule, not by this scope.
func Anonymous() Scope {
	return Scope{}
}

// For is the scope of a signed-in account. An unknown role gets the
// anonymous scope.
func For(v Viewer) Scope {
	if _, ok := ParseRole(string(v.Role)); !ok {
		return Anonymous()
	}
	// Only an admin is limited to the federations listed; keep the
	// list sorted and nil when empty, so two scopes built from the
	// same rows compare equal (the live streams' recheck).
	if v.Role == Admin && len(v.AdminOf) > 0 {
		v.AdminOf = slices.Compact(slices.Sorted(slices.Values(v.AdminOf)))
	} else {
		v.AdminOf = nil
	}
	if v.Role != Admin {
		v.HidePrivate = false
	}
	if v.Role == Admin && len(v.HideGroups) > 0 {
		v.HideGroups = slices.Compact(slices.Sorted(slices.Values(v.HideGroups)))
	} else {
		v.HideGroups = nil
	}
	if v.Role == Member && len(v.GrantedGroups) > 0 {
		v.GrantedGroups = slices.Compact(slices.Sorted(slices.Values(v.GrantedGroups)))
	} else {
		v.GrantedGroups = nil
	}
	// Only the member abilities mean anything for a member.
	var abilities []Ability
	if v.Role == Member {
		for _, a := range v.Abilities {
			if a == Forward || a == FetchFiles {
				abilities = append(abilities, a)
			}
		}
	}
	if len(abilities) > 0 {
		v.Abilities = slices.Compact(slices.Sorted(slices.Values(abilities)))
	} else {
		v.Abilities = nil
	}
	return Scope{kind: viewer, v: v}
}

// Viewer is the account behind the request, if there is one.
func (s Scope) Viewer() (Viewer, bool) {
	return s.v, s.kind == viewer
}

// Role is the viewer's role, or "" without one.
func (s Scope) Role() Role {
	if s.kind != viewer {
		return ""
	}
	return s.v.Role
}

func (s Scope) is(roles ...Role) bool {
	if s.kind != viewer {
		return false
	}
	for _, r := range roles {
		if s.v.Role == r {
			return true
		}
	}
	return false
}

// Gate is the step the viewer must take first, or None.
func (s Scope) Gate() Gate {
	switch {
	case s.kind != viewer:
		return None
	case s.v.MustChangePassword:
		return PasswordChange
	}
	return None
}

// SeesAllGroups is whether every group is visible, so a query needs no
// group filter. Superadmins see every group, and so do admins except
// those a superadmin hid from them (web-admin-restrictions); a member
// sees only its granted groups (web-member-access).
func (s Scope) SeesAllGroups() bool {
	return s.kind == everything || s.is(Superadmin) || (s.is(Admin) && len(s.v.HideGroups) == 0)
}

// GroupFilter is the condition, with its arguments, that a query over
// rows of groups adds to its WHERE so only visible groups come back.
// column is the query's own name for the group's chat id column, such
// as "m.chat_id", a constant of the caller's, never input. The
// condition is "1 = 1" when every group is visible, "column NOT IN
// (...)" over an admin's hidden groups, "column IN (...)" over a
// member's granted groups, and "1 = 0" when none is. An IN list keeps
// the chat_id indexes usable, so a member's queries read its groups'
// rows only.
func (s Scope) GroupFilter(column string) (string, []any) {
	switch {
	case s.SeesAllGroups():
		return "1 = 1", nil
	case s.is(Admin):
		return column + " NOT IN (" + marks(len(s.v.HideGroups)) + ")", groupArgs(s.v.HideGroups)
	case s.is(Member) && len(s.v.GrantedGroups) > 0:
		return column + " IN (" + marks(len(s.v.GrantedGroups)) + ")", groupArgs(s.v.GrantedGroups)
	}
	return "1 = 0", nil
}

func marks(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func groupArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, g := range ids {
		args[i] = g
	}
	return args
}

// UserFilter is the condition, with its arguments, that a query over
// users adds to its WHERE so only visible users come back. column is
// the query's own name for the user id column, such as "u.id", a
// constant of the caller's, never input. A viewer that sees users in
// full sees every user ("1 = 1"); a member sees a user only as it
// appears in its granted groups (web-accounts.md section 5): as the
// sender of a message there or as one of their admins. The two EXISTS
// are answered by the (sender_user_id, chat_id) index of the group
// messages and the (user_id) index of the group admins. Anyone else
// sees no user ("1 = 0").
func (s Scope) UserFilter(column string) (string, []any) {
	switch {
	case s.SeesAllUsers():
		return "1 = 1", nil
	case s.is(Member) && len(s.v.GrantedGroups) > 0:
		in := marks(len(s.v.GrantedGroups))
		args := append(groupArgs(s.v.GrantedGroups), groupArgs(s.v.GrantedGroups)...)
		return "(" + column + " IN (SELECT uf_m.sender_user_id FROM telegram_group_messages uf_m " +
			"WHERE uf_m.chat_id IN (" + in + ")) OR " + column + " IN (SELECT uf_a.user_id FROM " +
			"telegram_group_admins uf_a WHERE uf_a.group_id IN (" + in + ")))", args
	}
	return "1 = 0", nil
}

// HiddenGroups are the groups a superadmin hid from this admin,
// ascending; nil for everyone else. Visibility goes through
// GroupFilter and SeesGroup; this is for the queries that subtract the
// hidden groups' messages from a stored total.
func (s Scope) HiddenGroups() []int64 { return s.v.HideGroups }

// GrantedGroups are the groups granted to this member, ascending; nil
// for everyone else. Visibility goes through GroupFilter, SeesGroup and
// UserFilter; this is for the queries that inline the groups, as a
// member's message totals do.
func (s Scope) GrantedGroups() []int64 { return s.v.GrantedGroups }

// SeesGroup is whether one group is visible.
func (s Scope) SeesGroup(chatID int64) bool {
	switch {
	case s.SeesAllGroups():
		return true
	case s.is(Admin):
		_, hidden := slices.BinarySearch(s.v.HideGroups, chatID)
		return !hidden
	case s.is(Member):
		_, granted := slices.BinarySearch(s.v.GrantedGroups, chatID)
		return granted
	}
	return false
}

// SeesChat is whether a chat is visible, by the key the logger uses:
// a group by its (negative) chat id, a private chat by the other
// user's (positive) id. It answers from the scope alone, without a
// query, so it can filter a stream of events.
func (s Scope) SeesChat(chatID int64) bool {
	if chatID < 0 {
		return s.SeesGroup(chatID)
	}
	return s.SeesPrivate()
}

// SeesPrivate is whether private chats and their messages are
// visible: never to members (web-accounts.md section 5), and to admins
// unless a superadmin hides them (web-admin-restrictions).
func (s Scope) SeesPrivate() bool {
	return s.kind == everything || s.is(Superadmin) || (s.is(Admin) && !s.v.HidePrivate)
}

// SeesAllUsers is whether users are visible in full (bio, name
// history, global counts, their other groups). A member sees users
// only as they appear in its granted groups: whether it sees one takes
// a query (UserFilter; logdb.VisibleUsers).
func (s Scope) SeesAllUsers() bool {
	return s.kind == everything || s.is(Superadmin, Admin)
}

// Can is whether the viewer may do a. Superadmins may do everything.
// Admins may do all but what is withheld from them: join_chat is off
// by default and granted per admin (web-admin-restrictions), and "Look
// up real name" is for superadmins only. Members may do Forward and
// FetchFiles when given them (web-member-access), and nothing else.
func (s Scope) Can(a Ability) bool {
	switch {
	case s.is(Superadmin):
		return true
	case s.is(Admin):
		return a != JoinChat && a != LookupName
	case s.is(Member):
		_, given := slices.BinarySearch(s.v.Abilities, a)
		return given
	}
	return false
}

// SeesAllBots is whether every forwarding bot, with its owner, is the
// viewer's to see and use (decisions/2026-11-18-ownership-and-bots.md):
// superadmins' and admins'. A member with Forward sees and uses only
// the bots it owns, which the caller asks the ownership records about.
func (s Scope) SeesAllBots() bool {
	return s.is(Superadmin, Admin)
}

// SeesAllForwarding is whether the forwarding configurations of other
// accounts are the viewer's to see and change (web-accounts.md section
// 6): superadmins' and admins', within the groups they see. A member
// with Forward sees and changes only the configurations it owns, which
// the caller asks the ownership records about.
func (s Scope) SeesAllForwarding() bool {
	return s.is(Superadmin, Admin)
}

// The menu's page keys (decisions/2026-11-18-web-accounts-details.md),
// in menu order.
var pageKeys = []string{
	"dashboard", "users", "groups", "group_messages", "private_messages",
	"files", "platform_fwd", "join_chat", "mcp", "accounts", "federations",
	"settings",
}

// Page is whether the page with this menu key is the viewer's to open;
// one that is not answers 404. settings is every account's.
func (s Scope) Page(key string) bool {
	if s.kind != viewer {
		return false
	}
	switch key {
	case "dashboard", "users", "groups", "group_messages", "files", "settings":
		return true
	case "private_messages":
		return s.SeesPrivate()
	case "platform_fwd":
		return s.Can(Forward)
	case "join_chat":
		return s.Can(JoinChat)
	case "mcp":
		return s.Can(ManageMCP)
	case "accounts":
		return s.ManagesAccounts()
	case "federations":
		// Admins see the federations; only superadmins change them.
		return s.is(Superadmin, Admin)
	}
	return false
}

// Nav is the menu keys of the pages the viewer may open, in menu order.
func (s Scope) Nav() []string {
	nav := []string{}
	for _, k := range pageKeys {
		if s.Page(k) {
			nav = append(nav, k)
		}
	}
	return nav
}

// ManagesAccounts is whether the accounts administration is the
// viewer's: superadmins and admins. To a member it does not exist.
func (s Scope) ManagesAccounts() bool {
	return s.is(Superadmin, Admin)
}

// SeesAccount is whether the accounts administration shows a, which is
// what the viewer may manage (web-accounts.md sections 2 and 3): a
// superadmin every account, an admin the members filed in at least one
// federation it administers, and never an admin or a superadmin,
// itself included. A member in no federation is a superadmin's alone.
func (s Scope) SeesAccount(a Account) bool {
	switch {
	case s.is(Superadmin):
		return true
	case s.is(Admin):
		if a.Role != Member {
			return false
		}
		for _, f := range a.Federations {
			if s.AdministersFederation(f) {
				return true
			}
		}
	}
	return false
}

// MayGiveRole is whether the viewer may create an account with role r
// or change a visible account to it: a superadmin any role, an admin
// member only.
func (s Scope) MayGiveRole(r Role) bool {
	switch {
	case s.is(Superadmin):
		return true
	case s.is(Admin):
		return r == Member
	}
	return false
}

// AdministersFederation is whether the viewer manages the federation
// with this id: the members filed in it, their grants there and its
// forwarding. A superadmin administers every federation (whether id
// exists is the caller's to check), an admin those it is an admin of,
// Default included only when it is listed there.
func (s Scope) AdministersFederation(id uint64) bool {
	switch {
	case s.is(Superadmin):
		return true
	case s.is(Admin):
		_, found := slices.BinarySearch(s.v.AdminOf, id)
		return found
	}
	return false
}

// AdministeredFederations is the federations the viewer administers:
// all true for a superadmin (every federation, ids nil), else the ids
// of an admin's in ascending order (nil for none, and for every other
// scope). The caller must not change the slice.
func (s Scope) AdministeredFederations() (ids []uint64, all bool) {
	switch {
	case s.is(Superadmin):
		return nil, true
	case s.is(Admin):
		return s.v.AdminOf, false
	}
	return nil, false
}

// ManagesFederations is whether the viewer defines federations:
// creates and deletes them, renames any, sets their groups and admins,
// and says which federations an admin administers. Superadmins only
// (web-accounts.md section 3); an admin sees the federations it
// administers and only renames and describes them
// (DescribesFederation).
func (s Scope) ManagesFederations() bool {
	return s.is(Superadmin)
}

// DescribesFederation is whether the viewer may rename the federation
// with this id and change its description: whoever administers it, a
// superadmin any (whether id exists is the caller's to check). Its
// groups and admins stay the superadmins' (ManagesFederations), and
// Default's name is nobody's to change.
func (s Scope) DescribesFederation(id uint64) bool {
	return s.AdministersFederation(id)
}

// String describes the scope for logs.
func (s Scope) String() string {
	switch s.kind {
	case everything:
		return "everything"
	case viewer:
		return string(s.v.Role) + ":" + strconv.FormatUint(s.v.ID, 10)
	}
	return "anonymous"
}

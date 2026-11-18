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
// federations it administers. Later items add to Viewer and fill in
// the methods below without changing their callers:
// web-admin-restrictions an admin's hidden groups, hidden private
// messages and join_chat, web-member-access a member's granted groups
// and abilities.
package scope

import (
	"slices"
	"strconv"
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
// group filter. Superadmins and admins see every group (until
// web-admin-restrictions hides some from an admin); a member sees only
// its granted groups, none until web-member-access grants any.
func (s Scope) SeesAllGroups() bool {
	return s.kind == everything || s.is(Superadmin, Admin)
}

// GroupFilter is the condition, with its arguments, that a query over
// rows of groups adds to its WHERE so only visible groups come back.
// column is the query's own name for the group's chat id column, such
// as "m.chat_id", a constant of the caller's, never input. The
// condition is "1 = 1" when every group is visible and "1 = 0" when
// none is; later an IN list over column, which the chat_id indexes
// serve.
func (s Scope) GroupFilter(column string) (string, []any) {
	_ = column
	if s.SeesAllGroups() {
		return "1 = 1", nil
	}
	return "1 = 0", nil
}

// SeesGroup is whether one group is visible.
func (s Scope) SeesGroup(chatID int64) bool {
	_ = chatID
	return s.SeesAllGroups()
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

// SeesUser is whether a user is visible at all. A member will see the
// users of its granted groups, which takes a query; until grants exist
// it sees none. Like SeesChat, it needs no query today, and the
// version with grants will answer from a set loaded with the scope.
func (s Scope) SeesUser(userID int64) bool {
	_ = userID
	return s.SeesAllUsers()
}

// SeesPrivate is whether private chats and their messages are
// visible: never to members (web-accounts.md section 5), and to admins
// unless a superadmin hides them (web-admin-restrictions).
func (s Scope) SeesPrivate() bool {
	return s.kind == everything || s.is(Superadmin, Admin)
}

// SeesAllUsers is whether users are visible in full (bio, name
// history, global counts, their other groups). A member sees users
// only as they appear in its granted groups.
func (s Scope) SeesAllUsers() bool {
	return s.kind == everything || s.is(Superadmin, Admin)
}

// Can is whether the viewer may do a. Superadmins may do everything.
// Admins may do all but what is withheld from them: join_chat is off
// by default and granted per admin (web-admin-restrictions), and "Look
// up real name" is for superadmins only. Members get Forward and
// FetchFiles when granted (web-member-access), none so far.
func (s Scope) Can(a Ability) bool {
	switch {
	case s.is(Superadmin):
		return true
	case s.is(Admin):
		return a != JoinChat && a != LookupName
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
// creates, renames and deletes them, sets their groups and admins, and
// says which federations an admin administers. Superadmins only
// (web-accounts.md section 3); an admin sees the federations it
// administers without changing them.
func (s Scope) ManagesFederations() bool {
	return s.is(Superadmin)
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

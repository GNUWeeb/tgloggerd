// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The accounts administration (docs/design/web-accounts.md sections 2
// and 3): list, create, change, disable, enable, reset the password,
// send the invitation again and sign out everywhere. New accounts and
// resets get a set-password link by email (email.go). Who may manage whom is the request
// scope's (Scope.ManagesAccounts, SeesAccount, MayGiveRole,
// AdministersFederation): an account the viewer may not manage answers
// 404 like a missing one, and a change it may not make to one it
// manages answers 403. The contract is in webapi/openapi.yaml (tag
// accounts); web/docs/accounts.md explains it.

// accountAuditRows is how many audited changes an account row carries.
const accountAuditRows = 20

// Field limits: the columns' lengths, in characters.
const (
	maxDisplayName = 128
	maxEmail       = 254
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type auditJSON struct {
	What string `json:"what"`
	By   string `json:"by"`
	At   string `json:"at"`
}

type restrictJSON struct {
	HidePrivate bool       `json:"hide_private"`
	HideGroups  []pickItem `json:"hide_groups"`
	JoinChat    bool       `json:"join_chat"`
}

type grantJSON struct {
	Fed    uint64     `json:"fed"`
	Groups []pickItem `json:"groups"`
}

// accountJSON is the AccountRow schema.
type accountJSON struct {
	ID             uint64        `json:"id"`
	Username       string        `json:"username"`
	Name           string        `json:"name"`
	Email          *string       `json:"email"`
	EmailVerified  bool          `json:"email_verified"`
	Invited        bool          `json:"invited"`
	Role           string        `json:"role"`
	Active         bool          `json:"active"`
	LastSignin     *string       `json:"last_signin"`
	LastSuperadmin bool          `json:"last_superadmin"`
	AdminOf        []uint64      `json:"admin_of"`
	Restrict       *restrictJSON `json:"restrict"`
	Grants         []grantJSON   `json:"grants"`
	Abilities      []string      `json:"abilities"`
	Audit          []auditJSON   `json:"audit"`
}

// accountJSONOf is a row as the API shows it: an admin with the
// federations it administers and its restrictions, a member with every
// federation it is filed in (also those the viewer does not
// administer), the groups granted to it in each and its abilities.
// groups names the groups (groupPicks). The viewer sc sees only the
// granted groups it may see and change (grantShown); the others are
// left out, and their ids are cut out of the audit lines, so an admin
// learns nothing of groups granted in other federations or hidden
// from it.
func accountJSONOf(sc scope.Scope, a webdb.Account, supers int, audits []webdb.AccountAudit,
	groups map[int64]pickItem) accountJSON {
	j := accountJSON{
		ID:            a.ID,
		Username:      a.Username,
		Name:          a.DisplayName,
		EmailVerified: a.EmailVerified,
		// Has not chosen its own password yet.
		Invited:        invited(a),
		Role:           a.Role,
		Active:         a.Active,
		LastSuperadmin: a.Role == string(scope.Superadmin) && a.Active && supers == 1,
		AdminOf:        []uint64{},
		Grants:         []grantJSON{},
		Abilities:      []string{},
		Audit:          []auditJSON{},
	}
	if a.Email != "" {
		e := a.Email
		j.Email = &e
	}
	if a.LastSignin != nil {
		t := a.LastSignin.Format(time.RFC3339)
		j.LastSignin = &t
	}
	switch scope.Role(a.Role) {
	case scope.Admin:
		j.Restrict = &restrictJSON{HidePrivate: a.HidePrivate, HideGroups: picksOf(a.HideGroups, groups)}
		j.AdminOf = append(j.AdminOf, a.AdminOf...)
	case scope.Member:
		for _, f := range a.Federations {
			j.Grants = append(j.Grants, grantJSON{Fed: f, Groups: picksOf(shownGrants(sc, f, a.Grants[f]), groups)})
		}
		j.Abilities = append(j.Abilities, a.Abilities...)
	}
	for _, e := range audits {
		what := e.Action
		if e.Detail != "" {
			what += ": " + hideGroupIDs(sc, e.Detail)
		}
		j.Audit = append(j.Audit, auditJSON{what, e.By, e.At.Format(time.RFC3339)})
	}
	return j
}

// grantShown is whether the viewer sees a group granted in federation
// fed: one it administers and a group it sees. A superadmin sees all.
func grantShown(sc scope.Scope, fed uint64, g int64) bool {
	return sc.AdministersFederation(fed) && sc.SeesGroup(g)
}

// shownGrants is the groups of ids granted in fed that the viewer sees.
func shownGrants(sc scope.Scope, fed uint64, ids []int64) []int64 {
	out := []int64{}
	for _, g := range ids {
		if grantShown(sc, fed, g) {
			out = append(out, g)
		}
	}
	return out
}

// groupIDRe is a group id in an audit line (negative chat ids), and
// grantsRe the start of a grants part ("grants <fed>: ...").
var (
	groupIDRe = regexp.MustCompile(`-\d+`)
	grantsRe  = regexp.MustCompile(`^grants (\d+):`)
)

// hideGroupIDs replaces with "hidden" the group ids of an audit line
// that the viewer may not see: every group of a grants part of a
// federation it does not administer, and any group hidden from it.
func hideGroupIDs(sc scope.Scope, detail string) string {
	parts := strings.Split(detail, "; ")
	for i, p := range parts {
		all := false
		if m := grantsRe.FindStringSubmatch(p); m != nil {
			fed, err := strconv.ParseUint(m[1], 10, 64)
			all = err != nil || !sc.AdministersFederation(fed)
		}
		parts[i] = groupIDRe.ReplaceAllStringFunc(p, func(m string) string {
			if id, err := strconv.ParseInt(m, 10, 64); err != nil || all || !sc.SeesGroup(id) {
				return "hidden"
			}
			return m
		})
	}
	return strings.Join(parts, "; ")
}

// invited is whether an account has no password of its own yet: none
// (an invitation or a reset waits for its link), or a temporary one
// from before set-password links.
func invited(a webdb.Account) bool {
	return !a.HasPassword || a.MustChangePassword
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", nil)
}

func forbidden(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusForbidden, "forbidden", map[string]any{"message": msg})
}

func badRequest(w http.ResponseWriter, code, msg string) {
	writeError(w, http.StatusBadRequest, code, map[string]any{"message": msg})
}

// actor is the signed-in viewer behind an accounts operation. A scope
// without one never gets here (RequireSession), so a missing one is a
// bug and answers as not found.
func actor(w http.ResponseWriter, r *http.Request) (scope.Scope, scope.Viewer, bool) {
	sc := ScopeFrom(r.Context())
	v, ok := sc.Viewer()
	if !ok || !sc.ManagesAccounts() {
		// To a member the accounts administration does not exist.
		notFound(w)
		return sc, v, false
	}
	return sc, v, true
}

// target loads the account in the path, answering 404 itself when
// there is none or it is not the viewer's to manage.
func (s *Server) target(w http.ResponseWriter, r *http.Request, sc scope.Scope) (webdb.Account, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return webdb.Account{}, false
	}
	a, err := webdb.GetAccount(r.Context(), s.app, id)
	if errors.Is(err, webdb.ErrNoUser) {
		notFound(w)
		return a, false
	}
	if err != nil {
		s.internalError(w, "account lookup", err)
		return a, false
	}
	if !sc.SeesAccount(scopeAccount(a)) {
		notFound(w)
		return a, false
	}
	return a, true
}

// scopeAccount is an account as the scope's rules of who manages whom
// take it.
func scopeAccount(a webdb.Account) scope.Account {
	return scope.Account{ID: a.ID, Role: scope.Role(a.Role), Federations: a.Federations}
}

// row answers one account as the accounts page shows it.
func (s *Server) row(w http.ResponseWriter, r *http.Request, id uint64, status int) {
	ctx := r.Context()
	a, err := webdb.GetAccount(ctx, s.app, id)
	if err != nil {
		s.internalError(w, "account lookup", err)
		return
	}
	supers, err := webdb.ActiveSuperadmins(ctx, s.app)
	if err != nil {
		s.internalError(w, "superadmin count", err)
		return
	}
	audits, err := webdb.AccountAudits(ctx, s.app, id, accountAuditRows)
	if err != nil {
		s.internalError(w, "account audit", err)
		return
	}
	sc := ScopeFrom(ctx)
	writeJSON(w, status, accountJSONOf(sc, a, supers, audits, s.groupPicks(ctx, accountGroups(sc, a))))
}

// listAccounts answers GET /api/accounts: the accounts the viewer
// manages, optionally of one role.
func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc, _, ok := actor(w, r)
	if !ok {
		return
	}
	var only scope.Role
	if q := r.URL.Query().Get("role"); q != "" {
		if only, ok = scope.ParseRole(q); !ok {
			badRequest(w, "bad_request", "role must be superadmin, admin or member.")
			return
		}
	}
	out, err := s.managedAccounts(r.Context(), sc, only)
	if err != nil {
		s.internalError(w, "accounts", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

// managedAccounts is the accounts the viewer manages, optionally of
// one role, as the accounts list and its page show them.
func (s *Server) managedAccounts(ctx context.Context, sc scope.Scope, only scope.Role) ([]accountJSON, error) {
	all, err := webdb.ListAccounts(ctx, s.app)
	if err != nil {
		return nil, err
	}
	supers, err := webdb.ActiveSuperadmins(ctx, s.app)
	if err != nil {
		return nil, err
	}
	out := []accountJSON{}
	for _, a := range all {
		role := scope.Role(a.Role)
		if !sc.SeesAccount(scopeAccount(a)) || (only != "" && role != only) {
			continue
		}
		audits, err := webdb.AccountAudits(ctx, s.app, a.ID, accountAuditRows)
		if err != nil {
			return nil, err
		}
		out = append(out, accountJSONOf(sc, a, supers, audits, s.groupPicks(ctx, accountGroups(sc, a))))
	}
	return out, nil
}

// getAccount answers GET /api/accounts/{id}.
func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc, _, ok := actor(w, r)
	if !ok {
		return
	}
	a, ok := s.target(w, r, sc)
	if !ok {
		return
	}
	s.row(w, r, a.ID, http.StatusOK)
}

// temporaryPassword is a new random password, 20 characters from an
// alphabet without look-alikes (about 115 bits). It is answered once
// and never stored or logged; only its hash is kept.
func temporaryPassword() (string, error) {
	const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// 256 is not a multiple of len(alphabet); reject the high bytes so
	// every character is equally likely.
	out := make([]byte, 0, len(b))
	limit := byte(256 - 256%len(alphabet))
	for len(out) < len(b) {
		for _, c := range b {
			if c < limit && len(out) < len(b) {
				out = append(out, alphabet[int(c)%len(alphabet)])
			}
		}
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
	}
	return string(out), nil
}

// newPassword is the hash of a password an account must change at its
// next sign-in: the one typed, which must be long enough, or a
// generated temporary one, returned to be answered once. It answers
// the request itself and reports false when it cannot.
func (s *Server) newPassword(w http.ResponseWriter, typed *string, lang string) (hash, temp string, ok bool) {
	pw := ""
	if typed != nil {
		pw = *typed
	}
	if pw == "" {
		var err error
		if temp, err = temporaryPassword(); err != nil {
			s.internalError(w, "temporary password", err)
			return "", "", false
		}
		pw = temp
	} else if len(pw) < MinPasswordLen {
		badRequest(w, "weak_password", i18n.T(lang, "password.weak"))
		return "", "", false
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		s.internalError(w, "hash", err)
		return "", "", false
	}
	return hash, temp, true
}

// cleanName checks a display name: valid UTF-8, no control
// characters, at most maxDisplayName characters; spaces around it go.
func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxDisplayName {
		return "", false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return "", false
		}
	}
	return s, true
}

// cleanEmail checks an address: one plain addr-spec, no display name,
// at most maxEmail characters.
func cleanEmail(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxEmail || strings.ContainsAny(s, " <>\"") {
		return "", false
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || a.Name != "" {
		return "", false
	}
	return s, true
}

// grantWrite is the GrantWrite schema.
type grantWrite struct {
	Fed      *uint64 `json:"fed"`
	GroupIDs []int64 `json:"group_ids"`
}

// grantFederations reads a member's grants: the federations it is to
// be filed in and, for those the viewer administers, the groups
// granted to it there. A grant naming a federation is what files the
// member in it, and leaving one out unfiles it, which drops the
// groups granted there. keep is the federations the member is in now
// (nil for a new one): a grant of one of those passes even when the
// viewer does not administer it, as the accounts page sends the
// member's whole list back, and the change leaves it and its groups
// alone (FederationChange.Within, GrantChange). Any other grant must
// be of a federation the viewer administers, else 403. The groups are
// checked by checkGrantGroups. It answers the request itself when the
// grants do not pass.
func grantFederations(w http.ResponseWriter, sc scope.Scope, grants []grantWrite,
	keep []uint64) ([]uint64, map[uint64][]int64, bool) {
	feds := []uint64{}
	groups := map[uint64][]int64{}
	for _, g := range grants {
		if g.Fed == nil {
			badRequest(w, "bad_request", "Every grant names its federation (fed).")
			return nil, nil, false
		}
		if !sc.AdministersFederation(*g.Fed) {
			if !slices.Contains(keep, *g.Fed) {
				forbidden(w, "That federation is not one you administer.")
				return nil, nil, false
			}
		} else {
			groups[*g.Fed] = append(groups[*g.Fed], g.GroupIDs...)
		}
		feds = append(feds, *g.Fed)
	}
	return feds, groups, true
}

// checkGrantGroups checks the groups of grants: each must be a group of
// its federation (its stored groups, or for Default a group of the
// archive that no other federation holds) and one the viewer sees, as
// a viewer never grants what it cannot see. A federation that does not
// exist answers bad_federation, a group that is not the federation's
// 400 bad_request with field "grants". A group granted already in
// that federation (granted) passes as it is: it was checked when it
// was granted, and one hidden from the viewer is kept for it. It
// answers the request itself when they do not pass.
func (s *Server) checkGrantGroups(w http.ResponseWriter, r *http.Request, sc scope.Scope,
	groups, granted map[uint64][]int64) bool {
	ctx := r.Context()
	for fed, ids := range groups {
		if len(ids) == 0 {
			continue
		}
		var (
			held, federated []int64
			err             error
		)
		if fed == scope.DefaultFederation {
			if federated, err = webdb.FederatedGroups(ctx, s.app); err != nil {
				s.internalError(w, "federated groups", err)
				return false
			}
		} else {
			held, err = webdb.FederationGroups(ctx, s.app, fed)
			if errors.Is(err, webdb.ErrNoFederation) {
				s.badFederation(w, r)
				return false
			}
			if err != nil {
				s.internalError(w, "federation groups", err)
				return false
			}
		}
		for _, g := range ids {
			if slices.Contains(granted[fed], g) {
				continue
			}
			ok := g < 0 && sc.SeesGroup(g)
			if ok && fed == scope.DefaultFederation {
				ok = !slices.Contains(federated, g)
				if ok {
					if _, ok, err = logdb.ResolveChat(ctx, s.ro, sc, g); err != nil {
						s.internalError(w, "granted group", err)
						return false
					}
				}
			} else if ok {
				ok = slices.Contains(held, g)
			}
			if !ok {
				writeError(w, http.StatusBadRequest, "bad_request", map[string]any{
					"message": fmt.Sprintf("Group %d is not a group of federation %d.", g, fed),
					"field":   "grants",
				})
				return false
			}
		}
	}
	return true
}

// abilitiesOfRequest checks the abilities a request gives a member,
// answering 400 itself for one that is not a member ability.
func abilitiesOfRequest(w http.ResponseWriter, as []string) ([]string, bool) {
	out := []string{}
	for _, a := range as {
		if !webdb.ValidAbility(a) {
			writeError(w, http.StatusBadRequest, "bad_request", map[string]any{
				"message": "abilities are forward and fetch_files.",
				"field":   "abilities",
			})
			return nil, false
		}
		out = append(out, a)
	}
	return out, true
}

// federationChange is the change a viewer makes to an account's
// federations when it sends feds: a superadmin sets them all, an admin
// only those it administers.
func federationChange(sc scope.Scope, feds []uint64) *webdb.FederationChange {
	within, all := sc.AdministeredFederations()
	return &webdb.FederationChange{Set: feds, Within: within, WithinAll: all}
}

func (s *Server) badFederation(w http.ResponseWriter, r *http.Request) {
	badRequest(w, "bad_federation", s.text(r, "federation.unknown"))
}

// createAccount answers POST /api/accounts
// (decisions/2026-11-18-email-optional.md). The account gets the first
// password its creator typed, or a generated temporary one answered
// once (temporary_password), and must change it at its first sign-in.
// With "invite": true it gets no password; a set-password link goes to
// its address instead, which needs an address and mail (409 otherwise).
// The address is optional.
func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := actor(w, r)
	if !ok {
		return
	}
	var in struct {
		Username *string      `json:"username"`
		Name     *string      `json:"name"`
		Email    *string      `json:"email"`
		Role     *string      `json:"role"`
		Grants   []grantWrite `json:"grants"`
		Password *string      `json:"password"`
		Invite   bool         `json:"invite"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Username == nil || in.Name == nil || in.Role == nil {
		badRequest(w, "bad_request", "username, name and role are required.")
		return
	}
	if !usernameRe.MatchString(*in.Username) {
		badRequest(w, "bad_username", "A username is 1 to 64 letters, digits, dots, "+
			"dashes or underscores, starting with a letter or digit.")
		return
	}
	name, ok := cleanName(*in.Name)
	if !ok {
		badRequest(w, "bad_name", "A name is at most 128 characters, without control characters.")
		return
	}
	email := ""
	if in.Email != nil && strings.TrimSpace(*in.Email) != "" {
		if email, ok = cleanEmail(*in.Email); !ok {
			badRequest(w, "bad_email", "That is not an email address.")
			return
		}
	}
	role, ok := scope.ParseRole(*in.Role)
	if !ok {
		badRequest(w, "bad_request", "role must be superadmin, admin or member.")
		return
	}
	if !sc.MayGiveRole(role) {
		forbidden(w, "You cannot create an account with that role.")
		return
	}
	if role != scope.Member && len(in.Grants) != 0 {
		badRequest(w, "bad_request", "Only members get grants.")
		return
	}
	// An admin manages only the members of its federations, so a
	// member it creates must be filed in one at once.
	if v.Role == scope.Admin && len(in.Grants) == 0 {
		badRequest(w, "grants_required", "Choose at least one federation for the member.")
		return
	}
	feds, groups, ok := grantFederations(w, sc, in.Grants, nil)
	if !ok || !s.checkGrantGroups(w, r, sc, groups, nil) {
		return
	}
	if role == scope.Member && in.Grants == nil {
		// A superadmin's member without grants goes where today's
		// members went (web migration 000006): into Default.
		feds = []uint64{scope.DefaultFederation}
	}

	ctx := r.Context()
	lang := s.textLang(ctx, v.ID)
	var hash, temp string
	switch {
	case in.Invite && in.Password != nil && *in.Password != "":
		badRequest(w, "bad_request", i18n.T(lang, "account.invite_or_password"))
		return
	case in.Invite && email == "":
		writeError(w, http.StatusConflict, "no_email",
			map[string]any{"message": i18n.T(lang, "account.invite_no_email")})
		return
	case in.Invite && !s.mailOn():
		s.mailOff(w, lang)
		return
	case !in.Invite:
		if hash, temp, ok = s.newPassword(w, in.Password, lang); !ok {
			return
		}
	}
	id, err := webdb.CreateAccount(ctx, s.app, webdb.NewAccount{
		Username:     *in.Username,
		DisplayName:  name,
		Email:        email,
		Role:         string(role),
		PasswordHash: hash,
		CreatedBy:    v.ID,
		Federations:  feds,
		Grants:       webdb.GrantChange{Groups: groups},
	}, clientIP(r))
	switch {
	case errors.Is(err, webdb.ErrNoFederation):
		s.badFederation(w, r)
		return
	case errors.Is(err, webdb.ErrTaken):
		writeError(w, http.StatusConflict, "taken",
			map[string]any{"message": "That username is taken."})
		return
	case errors.Is(err, webdb.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken",
			map[string]any{"message": "Another account has that email address."})
		return
	case err != nil:
		s.internalError(w, "create account", err)
		return
	}
	out := map[string]any{"id": id, "invited": in.Invite}
	if in.Invite {
		a, err := webdb.GetAccount(ctx, s.app, id)
		if err == nil {
			err = s.sendLink(ctx, recipientOf(a, email), webdb.LinkPassword, "invite", s.mailCfg.InviteTTL)
		}
		if err != nil {
			// The account exists; resend-invite sends the link again.
			s.internalError(w, "invitation link", err)
			return
		}
	}
	if temp != "" {
		out["temporary_password"] = temp
	}
	writeJSON(w, http.StatusCreated, out)
}

// change applies one change to the account in the path and answers
// 204, or the account row when withRow. ErrLastSuperadmin answers 409.
// after, when not nil, runs once the change is made, if it changed
// anything, before the answer (the notices).
func (s *Server) change(w http.ResponseWriter, r *http.Request, a webdb.Account, v scope.Viewer,
	c webdb.AccountChange, action string, withRow bool, after func()) {
	changed, err := webdb.UpdateAccount(r.Context(), s.app, v.ID, a.ID, c, action, clientIP(r))
	switch {
	case errors.Is(err, webdb.ErrNoFederation):
		s.badFederation(w, r)
		return
	case errors.Is(err, webdb.ErrLastSuperadmin):
		writeError(w, http.StatusConflict, "last_superadmin", map[string]any{
			"message": "This is the last active superadmin; make another one first."})
		return
	case errors.Is(err, webdb.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken",
			map[string]any{"message": "Another account has that email address."})
		return
	case errors.Is(err, webdb.ErrNoUser):
		notFound(w)
		return
	case err != nil:
		s.internalError(w, "update account", err)
		return
	}
	if changed && after != nil {
		after()
	}
	if withRow {
		s.row(w, r, a.ID, http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateAccount answers PATCH /api/accounts/{id}. It changes the name,
// the address, the role, an admin's federations (admin_of) and
// restrictions (superadmins only), and a member's grants (each viewer
// within the federations it administers: the federations it is filed
// in and the groups granted there) and abilities. join_chat is
// accepted only off, until it is built, so the accounts page can send
// its whole form.
func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := actor(w, r)
	if !ok {
		return
	}
	a, ok := s.target(w, r, sc)
	if !ok {
		return
	}
	var in struct {
		Name     *string `json:"name"`
		Role     *string `json:"role"`
		Email    *string `json:"email"`
		AdminOf  []int64 `json:"admin_of"`
		Restrict *struct {
			HidePrivate bool    `json:"hide_private"`
			HideGroups  []int64 `json:"hide_groups"`
			JoinChat    bool    `json:"join_chat"`
		} `json:"restrict"`
		Grants    []grantWrite `json:"grants"`
		Abilities []string     `json:"abilities"`
	}
	if !decodeBody(w, r, &in) {
		return
	}

	var c webdb.AccountChange
	if in.Name != nil {
		name, ok := cleanName(*in.Name)
		if !ok {
			badRequest(w, "bad_name", "A name is at most 128 characters, without control characters.")
			return
		}
		c.DisplayName = &name
	}
	emailChanged := false
	if in.Email != nil {
		email, ok := cleanEmail(*in.Email)
		if !ok {
			badRequest(w, "bad_email", "That is not an email address.")
			return
		}
		// An account's address is its own to change (POST
		// /api/account/email). An admin may correct it only while the
		// account is invited; a superadmin may change any, for an
		// account that lost its mailbox (decisions/2026-11-18-web-email.md).
		if email != a.Email {
			if v.Role != scope.Superadmin && !invited(a) {
				forbidden(w, i18n.T(s.textLang(r.Context(), v.ID), "account.email_admin"))
				return
			}
			emailChanged = true
		}
		c.Email = &email
	}
	newRole := scope.Role(a.Role)
	if in.Role != nil {
		role, ok := scope.ParseRole(*in.Role)
		if !ok {
			badRequest(w, "bad_request", "role must be superadmin, admin or member.")
			return
		}
		if role != newRole && !sc.MayGiveRole(role) {
			forbidden(w, "You cannot give that role.")
			return
		}
		rs := string(role)
		c.Role, newRole = &rs, role
	}
	if (in.AdminOf != nil || in.Restrict != nil) && v.Role != scope.Superadmin {
		forbidden(w, "Only a superadmin sets an admin's federations and restrictions.")
		return
	}
	if in.AdminOf != nil {
		feds := make([]uint64, 0, len(in.AdminOf))
		for _, f := range in.AdminOf {
			if f < 0 {
				s.badFederation(w, r)
				return
			}
			feds = append(feds, uint64(f))
		}
		if len(feds) != 0 && newRole != scope.Admin {
			badRequest(w, "bad_request", s.text(r, "federation.admins_only"))
			return
		}
		c.AdminOf = &webdb.FederationChange{Set: feds, WithinAll: true}
	}
	if in.Restrict != nil {
		// Only hiding private chats is built so far
		// (docs/todo/web-admin-restrictions.md).
		if in.Restrict.JoinChat {
			badRequest(w, "not_supported", "Granting Join chat cannot be set yet.")
			return
		}
		if (in.Restrict.HidePrivate || len(in.Restrict.HideGroups) != 0) && newRole != scope.Admin {
			badRequest(w, "bad_request", "Only an admin can be restricted.")
			return
		}
		groups := []int64{}
		for _, g := range in.Restrict.HideGroups {
			if _, ok, err := logdb.ResolveChat(r.Context(), s.ro, scope.Everything(), g); err != nil {
				s.internalError(w, "hidden group", err)
				return
			} else if !ok || g >= 0 {
				writeError(w, http.StatusBadRequest, "bad_request", map[string]any{
					"message": "hide_groups names a group the archive does not have.",
					"field":   "hide_groups",
				})
				return
			}
			groups = append(groups, g)
		}
		hide := in.Restrict.HidePrivate
		c.HidePrivate, c.HideGroups = &hide, &groups
	}
	if len(in.Grants) != 0 && newRole != scope.Member {
		badRequest(w, "bad_request", "Only members get grants.")
		return
	}
	keep := a.Federations
	if scope.Role(a.Role) != scope.Member {
		keep = nil
	}
	feds, groups, ok := grantFederations(w, sc, in.Grants, keep)
	if !ok {
		return
	}
	// The groups granted in an administered federation that the viewer
	// cannot see (hidden from it) are not its to change: they are not
	// in the row it was shown, so they stay whatever it sends.
	if scope.Role(a.Role) == scope.Member {
		for f := range groups {
			kept := []int64{}
			for _, g := range groups[f] {
				if !slices.Contains(a.Grants[f], g) || grantShown(sc, f, g) {
					kept = append(kept, g)
				}
			}
			for _, g := range a.Grants[f] {
				if !grantShown(sc, f, g) {
					kept = append(kept, g)
				}
			}
			groups[f] = kept
		}
	}
	if !s.checkGrantGroups(w, r, sc, groups, a.Grants) {
		return
	}
	switch {
	case in.Grants != nil && newRole == scope.Member:
		c.Federations = federationChange(sc, feds)
		c.Grants = &webdb.GrantChange{Groups: groups}
	case newRole == scope.Member && scope.Role(a.Role) != scope.Member:
		// Made a member without grants: into Default, as a new one.
		c.Federations = &webdb.FederationChange{
			Set: []uint64{scope.DefaultFederation}, WithinAll: true}
	}
	if in.Abilities != nil {
		as, ok := abilitiesOfRequest(w, in.Abilities)
		if !ok {
			return
		}
		if len(as) != 0 && newRole != scope.Member {
			badRequest(w, "bad_request", "Only members get abilities.")
			return
		}
		c.Abilities = &as
	}
	s.change(w, r, a, v, c, "account_update", true, func() {
		s.afterUpdate(r.Context(), a, c, emailChanged)
	})
}

// afterUpdate tells an account what an update changed, at the address
// it had verified (web-accounts.md section 8): a new role, and a new
// address (also told to the old one). The new address gets the link
// that verifies it: the invitation again for an invited account, else
// a verification link.
func (s *Server) afterUpdate(ctx context.Context, a webdb.Account, c webdb.AccountChange, emailChanged bool) {
	if c.Role != nil && *c.Role != a.Role {
		s.notice(ctx, a, "role", "old", a.Role, "new", *c.Role)
	}
	if !emailChanged {
		return
	}
	s.notice(ctx, a, "email_old", "email", *c.Email)
	if *c.Email == "" || !s.mailOn() {
		return
	}
	p, kind, ttl := webdb.LinkVerify, "verify", s.mailCfg.VerifyTTL
	if invited(a) {
		p, kind, ttl = webdb.LinkPassword, "invite", s.mailCfg.InviteTTL
	}
	if err := s.sendLink(ctx, recipientOf(a, *c.Email), p, kind, ttl); err != nil {
		s.log.Warn("link to the new address not sent", "uid", a.ID, "err", err)
	}
}

// disableAccount answers POST /api/accounts/{id}/disable. Its sessions
// end on both services, and the account is told.
func (s *Server) disableAccount(w http.ResponseWriter, r *http.Request) {
	s.simpleChange(w, r, "account_disable", func(c *webdb.AccountChange) {
		f := false
		c.Active = &f
	}, func(a webdb.Account) {
		s.notice(r.Context(), a, "disabled")
	})
}

// enableAccount answers POST /api/accounts/{id}/enable.
func (s *Server) enableAccount(w http.ResponseWriter, r *http.Request) {
	s.simpleChange(w, r, "account_enable", func(c *webdb.AccountChange) {
		t := true
		c.Active = &t
	}, nil)
}

// signOutAccountEverywhere answers POST
// /api/accounts/{id}/sign-out-everywhere: every session of the
// account ends on both services, the viewer's own too when it is the
// viewer's account.
func (s *Server) signOutAccountEverywhere(w http.ResponseWriter, r *http.Request) {
	s.simpleChange(w, r, "account_signout_everywhere", func(c *webdb.AccountChange) {
		c.EndSessions = true
	}, nil)
}

// simpleChange makes the change set and answers 204; after, when not
// nil, runs with the account as it was if anything changed.
func (s *Server) simpleChange(w http.ResponseWriter, r *http.Request, action string,
	set func(*webdb.AccountChange), after func(webdb.Account)) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := actor(w, r)
	if !ok {
		return
	}
	a, ok := s.target(w, r, sc)
	if !ok {
		return
	}
	var c webdb.AccountChange
	set(&c)
	var done func()
	if after != nil {
		done = func() { after(a) }
	}
	s.change(w, r, a, v, c, action, false, done)
}

// resetAccountPassword answers POST /api/accounts/{id}/password-reset
// (decisions/2026-11-18-email-optional.md). Every session of the
// account ends. When the account has a verified address and mail is
// set up, the password is removed and a set-password link goes to the
// address ({"link_sent": true}); otherwise the account gets a random
// temporary password, answered once, which it must change at its next
// sign-in ({"link_sent": false, "temporary_password": ...}).
func (s *Server) resetAccountPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := actor(w, r)
	if !ok {
		return
	}
	a, ok := s.target(w, r, sc)
	if !ok {
		return
	}
	ctx := r.Context()
	if a.EmailVerified && a.Email != "" && s.mailOn() {
		_, err := webdb.UpdateAccount(ctx, s.app, v.ID, a.ID,
			webdb.AccountChange{ClearPassword: true}, "account_password_reset", clientIP(r))
		if err != nil {
			s.internalError(w, "reset password", err)
			return
		}
		if err := s.sendLink(ctx, recipientOf(a, a.Email), webdb.LinkPassword, "admin_reset", s.mailCfg.InviteTTL); err != nil {
			s.internalError(w, "reset link", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"link_sent": true})
		return
	}
	hash, temp, ok := s.newPassword(w, nil, "")
	if !ok {
		return
	}
	_, err := webdb.UpdateAccount(ctx, s.app, v.ID, a.ID,
		webdb.AccountChange{PasswordHash: &hash}, "account_password_reset", clientIP(r))
	if err != nil {
		s.internalError(w, "reset password", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"link_sent": false, "temporary_password": temp})
}

// accountGroups is every group an account row names to the viewer: an
// admin's hidden groups and the granted ones the viewer sees.
func accountGroups(sc scope.Scope, a webdb.Account) []int64 {
	ids := append([]int64{}, a.HideGroups...)
	for f, gs := range a.Grants {
		ids = append(ids, shownGrants(sc, f, gs)...)
	}
	return ids
}

// picksOf is the groups ids as the pickers show them, from groups
// (groupPicks); one the archive no longer has is named by its id.
func picksOf(ids []int64, groups map[int64]pickItem) []pickItem {
	out := []pickItem{}
	for _, g := range ids {
		p, ok := groups[g]
		if !ok {
			p = pickItem{ID: g, Title: "#" + strconv.FormatInt(g, 10), Kind: "group"}
		}
		out = append(out, p)
	}
	return out
}

// groupPicks is the groups of an account as the pickers show them, by
// id; a group the archive no longer has is left out (picksOf names it
// by its id).
func (s *Server) groupPicks(ctx context.Context, ids []int64) map[int64]pickItem {
	out := map[int64]pickItem{}
	for _, g := range ids {
		if c, ok, err := logdb.ResolveChat(ctx, s.ro, scope.Everything(), g); err == nil && ok {
			out[g] = s.fwdPick(c)
		}
	}
	return out
}

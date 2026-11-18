// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The federations' part of the accounts and federations pages' data
// (AccountsProps and FederationsProps in webapi/openapi.yaml). The
// pages' loaders call these; they assume the loader has checked that
// the page is the viewer's (Scope.Page "accounts" or "federations").

// federationPreview is how many groups a federation's card shows.
const federationPreview = 12

// accountsEditableJSON is AccountsProps.editable: what the accounts
// page can change today.
type accountsEditableJSON struct {
	Federations  bool `json:"federations"`
	GroupGrants  bool `json:"group_grants"`
	Restrictions bool `json:"restrictions"`
	// RestrictPrivate and RestrictGroups: the hide_private and
	// hide_groups restrictions can be set, before join_chat is built.
	RestrictPrivate bool `json:"restrict_private"`
	RestrictGroups  bool `json:"restrict_groups"`
	Abilities       bool `json:"abilities"`
}

// accountsEditable is AccountsProps.editable as far as the items built
// so far: federations (admin_of, and the federations of grants) are
// real, and so is an admin's hide_private; the groups of grants, the
// other restrictions and abilities are not yet. Each item that builds
// one turns its flag on here.
func accountsEditable() accountsEditableJSON {
	return accountsEditableJSON{Federations: true, RestrictPrivate: true, RestrictGroups: true}
}

// fedJSON is the Fed schema.
type fedJSON struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

// fedCountedJSON is an item of AccountsProps.federations.
type fedCountedJSON struct {
	fedJSON
	GroupCount int `json:"group_count"`
}

// fedPersonJSON is the FedPerson schema.
type fedPersonJSON struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// federationRowJSON is the FederationRow schema.
type federationRowJSON struct {
	fedJSON
	Created    *string            `json:"created"`
	Groups     []groupSummaryJSON `json:"groups"`
	GroupCount int                `json:"group_count"`
	Admins     []fedPersonJSON    `json:"admins"`
	Members    []fedPersonJSON    `json:"members"`
}

// federationsPropsJSON is the FederationsProps schema.
type federationsPropsJSON struct {
	Federations  []federationRowJSON `json:"federations"`
	Editable     bool                `json:"editable"`
	AdminOptions []fedPersonJSON     `json:"admin_options"`
}

// federationSets is every federation with its groups as a set for the
// logger's queries: Default's are every group in no other federation.
func (s *Server) federationSets(ctx context.Context) ([]webdb.Federation, map[uint64]logdb.GroupSet, error) {
	all, err := webdb.ListFederations(ctx, s.app)
	if err != nil {
		return nil, nil, err
	}
	sets := map[uint64]logdb.GroupSet{}
	var federated []int64
	for _, f := range all {
		if !f.Default() {
			sets[f.ID] = logdb.GroupSet{IDs: f.Groups}
			federated = append(federated, f.Groups...)
		}
	}
	sets[scope.DefaultFederation] = logdb.GroupSet{IDs: uniqInt64(federated), Not: true}
	return all, sets, nil
}

func uniqInt64(vs []int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, v := range vs {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func fedOf(f webdb.Federation) fedJSON {
	return fedJSON{ID: f.ID, Name: f.Name, Default: f.Default()}
}

// accountsFederations is AccountsProps.federations, the federations
// the viewer administers with how many of their groups it sees, and
// AllFederations, every federation (to name a member's others).
func (s *Server) accountsFederations(ctx context.Context, sc scope.Scope) (
	managed []fedCountedJSON, all []fedJSON, err error) {
	feds, sets, err := s.federationSets(ctx)
	if err != nil {
		return nil, nil, err
	}
	managed, all = []fedCountedJSON{}, []fedJSON{}
	for _, f := range feds {
		all = append(all, fedOf(f))
		if !sc.AdministersFederation(f.ID) {
			continue
		}
		n, err := logdb.CountGroups(ctx, s.ro, sc, sets[f.ID])
		if err != nil {
			return nil, nil, err
		}
		managed = append(managed, fedCountedJSON{fedOf(f), n})
	}
	return managed, all, nil
}

func people(ps []webdb.FedPerson) []fedPersonJSON {
	out := []fedPersonJSON{}
	for _, p := range ps {
		out = append(out, fedPersonJSON{p.ID, p.Username, p.DisplayName})
	}
	return out
}

// federationsProps is the federations page's data: for a superadmin
// every federation, editable, with the admins it may assign; for an
// admin the federations it administers, read-only. Each row carries a
// preview of the groups the viewer sees, busiest first, and the count
// of all its groups.
func (s *Server) federationsProps(ctx context.Context, sc scope.Scope) (federationsPropsJSON, error) {
	p := federationsPropsJSON{Federations: []federationRowJSON{}, Editable: sc.ManagesFederations(),
		AdminOptions: []fedPersonJSON{}}
	feds, sets, err := s.federationSets(ctx)
	if err != nil {
		return p, err
	}
	v := s.viewFor(sc, true)
	for _, f := range feds {
		if !sc.AdministersFederation(f.ID) {
			continue
		}
		set := sets[f.ID]
		// Counted within the scope, so groups hidden from an admin are
		// not counted either.
		total, err := logdb.CountGroups(ctx, s.ro, sc, set)
		if err != nil {
			return p, err
		}
		gs, err := logdb.PreviewGroups(ctx, s.ro, sc, set, federationPreview)
		if err != nil {
			return p, err
		}
		row := federationRowJSON{fedJSON: fedOf(f), Groups: []groupSummaryJSON{}, GroupCount: total,
			Admins: people(f.Admins), Members: people(f.Members)}
		if f.CreatedAt != nil {
			c := f.CreatedAt.Format(time.RFC3339)
			row.Created = &c
		}
		for _, g := range gs {
			row.Groups = append(row.Groups, v.group(g))
		}
		p.Federations = append(p.Federations, row)
	}
	if p.Editable {
		accounts, err := webdb.ListAccounts(ctx, s.app)
		if err != nil {
			return p, err
		}
		for _, a := range accounts {
			if a.Role == string(scope.Admin) && a.Active {
				p.AdminOptions = append(p.AdminOptions, fedPersonJSON{a.ID, a.Username, a.DisplayName})
			}
		}
	}
	return p, nil
}

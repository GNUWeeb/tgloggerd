// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// A federation's details (FederationDetailProps in webapi/openapi.yaml):
// the page /settings/federations/{id} (route federation), the same data
// at GET /api/federations/{id}, and its groups page by page at GET
// /api/federations/{id}/groups. A superadmin sees every federation, an
// admin those it administers; to anyone else, and for any other
// federation, they do not exist (404). The groups are the ones the
// request scope shows, so groups hidden from an admin are neither
// listed nor counted.

// The groups pages' sizes.
const (
	fedGroupsDefault = 50
	fedGroupsMax     = 200
)

// federationDetailJSON is FederationDetail.
type federationDetailJSON struct {
	fedJSON
	Description string         `json:"description"`
	Created     *string        `json:"created"`
	CreatedBy   *fedPersonJSON `json:"created_by"`
	GroupCount  int            `json:"group_count"`
}

// federationCanEditJSON is FederationCanEdit: what the viewer may
// change of the federation.
type federationCanEditJSON struct {
	Name        bool `json:"name"`
	Description bool `json:"description"`
	Groups      bool `json:"groups"`
	Admins      bool `json:"admins"`
	Delete      bool `json:"delete"`
}

// fedGroupsPageJSON is FederationGroupsPage.
type fedGroupsPageJSON struct {
	Items      []groupSummaryJSON `json:"items"`
	NextCursor *string            `json:"next_cursor"`
	Total      int                `json:"total"`
}

// federationDetailPropsJSON is FederationDetailProps.
type federationDetailPropsJSON struct {
	Federation federationDetailJSON  `json:"federation"`
	Admins     []fedPersonJSON       `json:"admins"`
	Members    []fedPersonJSON       `json:"members"`
	Groups     fedGroupsPageJSON     `json:"groups"`
	CanEdit    federationCanEditJSON `json:"can_edit"`
}

// errBadFedCursor: a groups cursor this service did not give.
var errBadFedCursor = errors.New("bad federation groups cursor")

// The groups cursor is opaque to clients; it holds the last group's
// message count and id.
func fedGroupsCursor(k logdb.FedGroupKey) string {
	return base64.RawURLEncoding.EncodeToString([]byte("k" + strconv.FormatUint(k.MsgCount, 10) +
		":" + strconv.FormatInt(k.ID, 10)))
}

func parseFedGroupsCursor(c string) (logdb.FedGroupKey, bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || len(b) < 2 || b[0] != 'k' {
		return logdb.FedGroupKey{}, false
	}
	n, id, ok := strings.Cut(string(b[1:]), ":")
	if !ok {
		return logdb.FedGroupKey{}, false
	}
	var k logdb.FedGroupKey
	if k.MsgCount, err = strconv.ParseUint(n, 10, 64); err != nil {
		return logdb.FedGroupKey{}, false
	}
	if k.ID, err = strconv.ParseInt(id, 10, 64); err != nil {
		return logdb.FedGroupKey{}, false
	}
	return k, true
}

// visibleFederation is the federation with this id if the viewer sees
// it (it administers it, a superadmin every one), else
// webdb.ErrNoFederation, as for one that does not exist.
func (s *Server) visibleFederation(ctx context.Context, sc scope.Scope, id uint64) (webdb.Federation, error) {
	if !sc.AdministersFederation(id) {
		return webdb.Federation{}, webdb.ErrNoFederation
	}
	return webdb.GetFederation(ctx, s.app, id)
}

// federationSet is the federation's groups as a set for the logger's
// queries: Default's are every group in no other federation.
func (s *Server) federationSet(ctx context.Context, f webdb.Federation) (logdb.GroupSet, error) {
	if f.Default() {
		ids, err := webdb.FederatedGroups(ctx, s.app)
		return logdb.GroupSet{IDs: ids, Not: true}, err
	}
	return logdb.GroupSet{IDs: f.Groups}, nil
}

// federationGroupsPage is a page of the set's groups the scope sees,
// after cursor ("" for the first page); errBadFedCursor for a cursor
// that does not open.
func (s *Server) federationGroupsPage(ctx context.Context, sc scope.Scope, set logdb.GroupSet,
	q, cursor string, limit int) (fedGroupsPageJSON, error) {
	p := fedGroupsPageJSON{Items: []groupSummaryJSON{}}
	fq := logdb.FedGroupQuery{Set: set, Q: q, Limit: limit}
	if cursor != "" {
		k, ok := parseFedGroupsCursor(cursor)
		if !ok {
			return p, errBadFedCursor
		}
		fq.After = &k
	}
	gs, total, err := logdb.FederationGroupPage(ctx, s.ro, sc, fq)
	if err != nil {
		return p, err
	}
	p.Total = total
	if len(gs) > limit {
		gs = gs[:limit]
		last := gs[len(gs)-1]
		c := fedGroupsCursor(logdb.FedGroupKey{MsgCount: last.MsgCount, ID: last.ID})
		p.NextCursor = &c
	}
	v := s.viewFor(sc, true)
	for _, g := range gs {
		p.Items = append(p.Items, v.group(g))
	}
	return p, nil
}

// federationDetail is the federation's details for the viewer, or
// webdb.ErrNoFederation when it does not see it.
func (s *Server) federationDetail(ctx context.Context, sc scope.Scope, id uint64) (federationDetailPropsJSON, error) {
	var p federationDetailPropsJSON
	f, err := s.visibleFederation(ctx, sc, id)
	if err != nil {
		return p, err
	}
	set, err := s.federationSet(ctx, f)
	if err != nil {
		return p, err
	}
	// Counted within the scope, so groups hidden from an admin are not
	// counted either.
	n, err := logdb.CountGroups(ctx, s.ro, sc, set)
	if err != nil {
		return p, err
	}
	p.Federation = federationDetailJSON{fedJSON: fedOf(f), Description: f.Description, GroupCount: n}
	if f.CreatedAt != nil {
		c := f.CreatedAt.Format(time.RFC3339)
		p.Federation.Created = &c
	}
	if f.CreatedBy != nil {
		p.Federation.CreatedBy = &fedPersonJSON{f.CreatedBy.ID, f.CreatedBy.Username, f.CreatedBy.DisplayName}
	}
	p.Admins, p.Members = people(f.Admins), people(f.Members)
	if p.Groups, err = s.federationGroupsPage(ctx, sc, set, "", "", fedGroupsDefault); err != nil {
		return p, err
	}
	all := sc.ManagesFederations()
	p.CanEdit = federationCanEditJSON{
		Name:        sc.DescribesFederation(id) && !f.Default(),
		Description: sc.DescribesFederation(id),
		Groups:      all && !f.Default(),
		Admins:      all,
		Delete:      all && !f.Default(),
	}
	return p, nil
}

// loadFederation is the federation page's loader.
func loadFederation(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "federations"); err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(pr.params["id"], 10, 64)
	if err != nil {
		return nil, errPageNotFound
	}
	p, err := pr.s.federationDetail(ctx, pr.scope, id)
	if errors.Is(err, webdb.ErrNoFederation) {
		return nil, errPageNotFound
	}
	return p, err
}

// getFederation answers GET /api/federations/{id} with the federation
// page's props.
func (s *Server) getFederation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	id, ok := federationID(w, r)
	if !ok {
		return
	}
	p, err := s.federationDetail(r.Context(), ScopeFrom(r.Context()), id)
	if errors.Is(err, webdb.ErrNoFederation) {
		notFound(w)
		return
	}
	if err != nil {
		s.internalError(w, "federation", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// getFederationGroups answers GET /api/federations/{id}/groups.
func (s *Server) getFederationGroups(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	ctx := r.Context()
	sc := ScopeFrom(ctx)
	id, ok := federationID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := fedGroupsDefault
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > fedGroupsMax {
			badRequest(w, "bad_request", s.text(r, "federation.groups_limit", "max", strconv.Itoa(fedGroupsMax)))
			return
		}
		limit = n
	}
	text := q.Get("q")
	if len(text) > logdb.MaxValueLen {
		badRequest(w, "bad_request", s.text(r, "federation.search_long",
			"max", strconv.Itoa(logdb.MaxValueLen)))
		return
	}
	f, err := s.visibleFederation(ctx, sc, id)
	if errors.Is(err, webdb.ErrNoFederation) {
		notFound(w)
		return
	}
	if err != nil {
		s.internalError(w, "federation", err)
		return
	}
	set, err := s.federationSet(ctx, f)
	if err != nil {
		s.internalError(w, "federation groups", err)
		return
	}
	p, err := s.federationGroupsPage(ctx, sc, set, text, q.Get("cursor"), limit)
	if errors.Is(err, errBadFedCursor) {
		badRequest(w, "bad_request", s.text(r, "federation.bad_cursor"))
		return
	}
	if err != nil {
		s.internalError(w, "federation groups", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

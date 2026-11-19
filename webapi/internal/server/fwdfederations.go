// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The federation of a forwarding configuration (web-accounts.md
// section 6, webdb resfeds.go). It is chosen at creation among the
// federations holding the configuration's chat that are the creator's
// own: every one for a superadmin, those it administers for an admin,
// those in which the group is granted to it for a member. When none is
// its own, every federation holding the chat is a choice, so an admin
// may still set up forwarding for a group of a federation it does not
// administer. The form asks only when there is more than one choice
// (GET /api/forwarding/federations); with one, the configuration goes
// there by itself.

// fwdChoices is the federations a new configuration of chat may go
// into, ascending (Default first), never empty.
func (s *Server) fwdChoices(ctx context.Context, sc scope.Scope, chat int64) ([]uint64, error) {
	holders, err := webdb.GroupFederations(ctx, s.app, chat)
	if err != nil {
		return nil, err
	}
	var own []uint64
	switch sc.Role() {
	case scope.Superadmin:
		own = holders
	case scope.Admin:
		for _, f := range holders {
			if sc.AdministersFederation(f) {
				own = append(own, f)
			}
		}
	case scope.Member:
		v, _ := sc.Viewer()
		acc, err := webdb.GetAccount(ctx, s.app, v.ID)
		if err != nil {
			return nil, err
		}
		for _, f := range holders {
			if slices.Contains(acc.Grants[f], chat) {
				own = append(own, f)
			}
		}
	}
	if len(own) == 0 {
		return holders, nil
	}
	return own, nil
}

// fedNames is the name of every federation by id, Default's in the
// viewer's language.
func (s *Server) fedNames(ctx context.Context, v scope.Viewer) (map[uint64]string, error) {
	all, err := webdb.ListFederations(ctx, s.app)
	if err != nil {
		return nil, err
	}
	m := make(map[uint64]string, len(all)+1)
	for _, f := range all {
		m[f.ID] = f.Name
	}
	m[scope.DefaultFederation] = s.textFor(ctx, v, "federation.default")
	return m, nil
}

// fwdFederation picks the federation of a new configuration of chat:
// the one choice there is, or fed, which must be a choice. It answers
// 400 itself, federation_required when there are several choices and
// fed is nil, bad_federation for a fed that is not one.
func (s *Server) fwdFederation(w http.ResponseWriter, r *http.Request, sc scope.Scope,
	v scope.Viewer, chat int64, fed *uint64) (uint64, bool) {
	choices, err := s.fwdChoices(r.Context(), sc, chat)
	if err != nil {
		s.internalError(w, "forwarding federations", err)
		return 0, false
	}
	switch {
	case fed != nil && slices.Contains(choices, *fed):
		return *fed, true
	case fed != nil:
		s.refuse(w, r, v, http.StatusBadRequest, "bad_federation", "fwd.bad_federation")
		return 0, false
	case len(choices) > 1:
		s.refuse(w, r, v, http.StatusBadRequest, "federation_required", "fwd.federation_required")
		return 0, false
	}
	return choices[0], true
}

// fileConfig puts a configuration, just created, into its federation.
// As with its owner (ownership.go), a failure leaves it in Default and
// is logged; the configuration exists by then.
func (s *Server) fileConfig(r *http.Request, res webdb.Resource, fed uint64) {
	if err := webdb.FileResource(r.Context(), s.app, res, fed); err != nil {
		s.log.Error("file configuration", "resource", res.String(), "federation", fed, "err", err)
	}
}

// fwdFederations answers GET /api/forwarding/federations?chat_id=:
// the choices for a new configuration of the chat.
func (s *Server) fwdFederations(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := adminActor(w, r, scope.Forward)
	if !ok {
		return
	}
	chat, err := strconv.ParseInt(r.URL.Query().Get("chat_id"), 10, 64)
	if err != nil || chat == 0 {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.chat_required")
		return
	}
	c, ok, err := logdb.ResolveChat(r.Context(), s.ro, sc, chat)
	if err != nil {
		s.internalError(w, "forwarding chat", err)
		return
	}
	if !ok {
		notFound(w)
		return
	}
	choices, err := s.fwdChoices(r.Context(), sc, c.ID)
	if err != nil {
		s.internalError(w, "forwarding federations", err)
		return
	}
	names, err := s.fedNames(r.Context(), v)
	if err != nil {
		s.internalError(w, "federations", err)
		return
	}
	out := []fedRef{}
	for _, f := range choices {
		out = append(out, fedRef{f, names[f]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"federations": out})
}

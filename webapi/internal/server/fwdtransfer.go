// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// Giving a forwarding configuration to another account
// (web-accounts.md section 6): a superadmin gives any configuration to
// any active account; an admin gives the configurations of the
// federations it administers to that federation's admins and to its
// members holding forward that are granted the configuration's group
// there, so the new owner can see it. A member transfers nothing. The
// configuration keeps forwarding and stays in its federation; a
// disabled owner's configurations are given away the same way, which
// is what the "owner disabled" flag is for.

// fwdCandidates is who sc may give a configuration of chat in
// federation fed, owned by owner (0 for shared), to, from accounts.
func fwdCandidates(sc scope.Scope, accounts []webdb.Account, fed uint64, chat int64,
	owner uint64) []fwdPerson {
	out := []fwdPerson{}
	if !sc.SeesAllForwarding() {
		return out
	}
	super := sc.Role() == scope.Superadmin
	if !super && !sc.AdministersFederation(fed) {
		return out
	}
	for _, a := range accounts {
		if !a.Active || a.ID == owner {
			continue
		}
		switch {
		case super:
		case a.Role == string(scope.Admin) && slices.Contains(a.AdminOf, fed):
		case a.Role == string(scope.Member) && slices.Contains(a.Federations, fed) &&
			slices.Contains(a.Abilities, string(scope.Forward)) && slices.Contains(a.Grants[fed], chat):
		default:
			continue
		}
		out = append(out, personOf(a))
	}
	return out
}

// transferFwd gives configuration res, of chat, to the account the
// body names, when it is one of the viewer's candidates. bots are the
// bots the configuration uses, which an account given it must own when
// it is a member (409 bot_not_owned). It answers the request itself.
func (s *Server) transferFwd(w http.ResponseWriter, r *http.Request, sc scope.Scope, v scope.Viewer,
	res webdb.Resource, chat int64, bots []uint64, action, what string) {
	var in struct {
		OwnerID *uint64 `json:"owner_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.OwnerID == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.bad_owner")
		return
	}
	ctx := r.Context()
	m, err := s.fwdMetaOf(ctx, sc, v, res.Type, []uint64{uint64(res.ID)})
	if err != nil {
		s.internalError(w, "transfer candidates", err)
		return
	}
	o := s.ownership(ctx, v, uint64(res.ID), chat, m)
	var to *fwdPerson
	for i, c := range o.TransferCandidates {
		if c.ID == *in.OwnerID {
			to = &o.TransferCandidates[i]
		}
	}
	if to == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.bad_owner")
		return
	}
	if to.Role == string(scope.Member) {
		for _, b := range bots {
			bo, ok, err := webdb.OwnerOf(ctx, s.app, botResource(b))
			if err != nil {
				s.internalError(w, "bot owner", err)
				return
			}
			if !ok || bo.ID != to.ID {
				s.refuse(w, r, v, http.StatusConflict, "bot_not_owned", "fwd.bot_not_owned")
				return
			}
		}
	}
	by := webdb.By(v.ID, clientIP(r))
	from, err := webdb.TransferOwner(ctx, s.app, res, to.ID, by)
	if errors.Is(err, webdb.ErrNotOwned) {
		// It counted as the superadmins': it gets its first owner.
		err = webdb.SetOwner(ctx, s.app, res, to.ID, by)
	}
	if err != nil {
		s.internalError(w, "transfer "+what, err)
		return
	}
	s.auditAdmin(r, v.ID, action, fmt.Sprintf("%s %d chat %d from %d to %d", what, res.ID, chat, from, to.ID))
	w.WriteHeader(http.StatusNoContent)
}

// transferT2D answers POST /api/forwarding/telegram-discord/{id}/transfer.
func (s *Server) transferT2D(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	if !sc.SeesAllForwarding() {
		notFound(w)
		return
	}
	h, ok := s.t2dOf(w, r, sc)
	if !ok {
		return
	}
	s.transferFwd(w, r, sc, v, webdb.Resource{Type: webdb.ResT2DWebhook, ID: int64(h.ID)}, h.Chat.ID,
		nil, "fwd_t2d_transfer", "webhook")
}

// transferD2T answers POST /api/forwarding/discord-telegram/{id}/transfer.
func (s *Server) transferD2T(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	if !sc.SeesAllForwarding() {
		notFound(w)
		return
	}
	d, ok := s.d2tOf(w, r, sc)
	if !ok {
		return
	}
	bots := []uint64{d.TelegramBotID}
	if d.DiscordBotID != 0 {
		bots = append(bots, d.DiscordBotID)
	}
	s.transferFwd(w, r, sc, v, webdb.Resource{Type: webdb.ResD2TRoute, ID: int64(d.ID)}, d.Chat.ID,
		bots, "fwd_d2t_transfer", "route")
}

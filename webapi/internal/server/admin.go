// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// What the administration operations share: forwarding (forwarding.go),
// Join chat (joinchat.go), "Look up real name" (lookupname.go) and the
// MCP administration (mcpadmin.go), each with its pages.
//
// Each is behind an ability of the request scope (Scope.Can): a viewer
// without it gets 404, as if the operation did not exist, and so does
// a viewer asking about something the scope hides. Every change writes
// a web_audit row with who made it; secrets (webhook URLs, bot tokens,
// MCP tokens) are never in it, nor in any answer.

// adminActor is the viewer of an administration operation that needs
// the ability a. Without it the operation does not exist for the
// viewer: 404, answered here.
func adminActor(w http.ResponseWriter, r *http.Request, a scope.Ability) (scope.Scope, scope.Viewer, bool) {
	sc := ScopeFrom(r.Context())
	v, ok := sc.Viewer()
	if !ok || !sc.Can(a) {
		notFound(w)
		return sc, v, false
	}
	return sc, v, true
}

// adminWrite starts a state-changing administration operation: the
// CSRF token, then the ability. It answers the request itself when
// either fails.
func (s *Server) adminWrite(w http.ResponseWriter, r *http.Request, a scope.Ability) (scope.Scope, scope.Viewer, bool) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return scope.Scope{}, scope.Viewer{}, false
	}
	return adminActor(w, r, a)
}

// auditAdmin writes an administration change to web_audit. The change
// has happened by then, so a lost row is logged and does not turn the
// answer into a failure.
func (s *Server) auditAdmin(r *http.Request, uid uint64, action, detail string) {
	if err := webdb.Audit(r.Context(), s.app, &uid, action, clientIP(r), detail); err != nil {
		s.log.Error("audit", "action", action, "err", err)
	}
}

// textFor is a user-facing sentence in the language of viewer v.
func (s *Server) textFor(ctx context.Context, v scope.Viewer, key string, kv ...string) string {
	return i18n.T(s.textLang(ctx, v.ID), key, kv...)
}

// refuse answers status with an error code and the sentence of key.
func (s *Server) refuse(w http.ResponseWriter, r *http.Request, v scope.Viewer, status int,
	code, key string, kv ...string) {
	writeError(w, status, code, map[string]any{"message": s.textFor(r.Context(), v, key, kv...)})
}

// pathID reads the {id} of the path as an unsigned id.
func pathID(r *http.Request, name string) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// pageOf is the viewer of a page the scope must allow, or
// errPageNotFound.
func pageOf(pr *pageRequest, key string) (scope.Viewer, error) {
	v, ok := pr.scope.Viewer()
	if !ok || !pr.scope.Page(key) {
		return v, errPageNotFound
	}
	return v, nil
}

// fedRef is a federation as the forwarding and Join chat pages name
// one (FedRef).
type fedRef struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// fileFederations are the federations the viewer may file a
// configuration or a joined group into: those that exist and that it
// administers (Scope.AdministersFederation), Default first. This is
// the one place that lists them for the administration pages and
// checks a chosen one.
func (s *Server) fileFederations(ctx context.Context, sc scope.Scope) ([]fedRef, error) {
	all, err := webdb.ListFederations(ctx, s.app)
	if err != nil {
		return nil, err
	}
	out := []fedRef{}
	for _, f := range all {
		if sc.AdministersFederation(f.ID) {
			out = append(out, fedRef{f.ID, f.Name})
		}
	}
	return out, nil
}

// filesInto is whether id is one of fileFederations.
func (s *Server) filesInto(ctx context.Context, sc scope.Scope, id uint64) (bool, error) {
	feds, err := s.fileFederations(ctx, sc)
	if err != nil {
		return false, err
	}
	for _, f := range feds {
		if f.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// clip cuts s to n characters, never inside one, after trimming the
// spaces around it.
func clip(s string, n int) string {
	s = strings.ToValidUTF8(strings.TrimSpace(s), "�")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

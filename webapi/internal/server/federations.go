// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The federations editor (docs/design/web-accounts.md section 3):
// superadmins create, change and delete federations, their groups and
// their admins. To anyone else these operations do not exist (404);
// admins see the federations they administer on the federations page,
// read-only. The contract is in webapi/openapi.yaml (tag accounts);
// web/docs/accounts.md explains it.

// Limits of a federation write: the name's length in characters
// (web_federations.name), how many groups one may hold, and the body,
// which must take that many ids.
const (
	maxFederationName   = 64
	maxFederationGroups = 50000
	maxFederationBody   = 1 << 20
)

// text is a user-facing text in the viewer's language.
func (s *Server) text(r *http.Request, key string, kv ...string) string {
	var uid uint64
	if v, ok := ScopeFrom(r.Context()).Viewer(); ok {
		uid = v.ID
	}
	return i18n.T(s.textLang(r.Context(), uid), key, kv...)
}

// federationWrite is the FederationWrite schema; absent fields stay.
type federationWrite struct {
	Name     *string   `json:"name"`
	GroupIDs *[]int64  `json:"group_ids"`
	AdminIDs *[]uint64 `json:"admin_ids"`
}

// federationEditor is the superadmin behind a federation write, after
// the CSRF check; anyone else gets 404, as the operation does not
// exist for it.
func (s *Server) federationEditor(w http.ResponseWriter, r *http.Request) (scope.Scope, scope.Viewer, bool) {
	sc := ScopeFrom(r.Context())
	v, ok := sc.Viewer()
	if !ok || !sc.ManagesFederations() {
		notFound(w)
		return sc, v, false
	}
	return sc, v, true
}

// readFederation decodes and checks a federation write, answering the
// request itself when it does not pass: the name, and the groups,
// which must be groups the logger knows.
func (s *Server) readFederation(w http.ResponseWriter, r *http.Request, sc scope.Scope) (webdb.FederationWrite, bool) {
	var in federationWrite
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFederationBody))
	if err := dec.Decode(&in); err != nil {
		badRequest(w, "bad_request", s.text(r, "federation.malformed"))
		return webdb.FederationWrite{}, false
	}
	out := webdb.FederationWrite{Groups: in.GroupIDs, Admins: in.AdminIDs}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		ok := name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= maxFederationName
		for _, c := range name {
			ok = ok && !unicode.IsControl(c)
		}
		if !ok {
			badRequest(w, "bad_name", s.text(r, "federation.name",
				"max", strconv.Itoa(maxFederationName)))
			return out, false
		}
		out.Name = &name
	}
	if in.GroupIDs != nil {
		if len(*in.GroupIDs) > maxFederationGroups {
			badRequest(w, "bad_request", s.text(r, "federation.too_many_groups",
				"max", strconv.Itoa(maxFederationGroups)))
			return out, false
		}
		missing, err := logdb.MissingGroups(r.Context(), s.ro, sc, *in.GroupIDs)
		if err != nil {
			s.internalError(w, "federation groups", err)
			return out, false
		}
		if len(missing) > 0 {
			writeError(w, http.StatusBadRequest, "bad_group", map[string]any{
				"message": s.text(r, "federation.bad_group",
					"id", strconv.FormatInt(missing[0], 10))})
			return out, false
		}
	}
	return out, true
}

// federationError answers the errors a federation write can meet, and
// reports whether err was one.
func (s *Server) federationError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, webdb.ErrNoFederation):
		notFound(w)
	case errors.Is(err, webdb.ErrFederationTaken):
		writeError(w, http.StatusConflict, "taken",
			map[string]any{"message": s.text(r, "federation.taken")})
	case errors.Is(err, webdb.ErrNotAdmin):
		badRequest(w, "bad_admin", s.text(r, "federation.bad_admin"))
	case errors.Is(err, webdb.ErrDefaultFixed):
		badRequest(w, "bad_request", s.text(r, "federation.default_fixed"))
	default:
		s.internalError(w, "federation write", err)
	}
	return true
}

// createFederation answers POST /api/federations.
func (s *Server) createFederation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := s.federationEditor(w, r)
	if !ok {
		return
	}
	fw, ok := s.readFederation(w, r, sc)
	if !ok {
		return
	}
	if fw.Name == nil {
		badRequest(w, "bad_name", s.text(r, "federation.name", "max", strconv.Itoa(maxFederationName)))
		return
	}
	id, err := webdb.CreateFederation(r.Context(), s.app, v.ID, fw, clientIP(r))
	if s.federationError(w, r, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// federationID is the federation in the path; ok false answers 404.
func federationID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return 0, false
	}
	return id, true
}

// updateFederation answers PATCH /api/federations/{id}. Default takes
// admins only: its groups are derived and its name is fixed.
func (s *Server) updateFederation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := s.federationEditor(w, r)
	if !ok {
		return
	}
	id, ok := federationID(w, r)
	if !ok {
		return
	}
	fw, ok := s.readFederation(w, r, sc)
	if !ok {
		return
	}
	if id == scope.DefaultFederation && fw.Groups != nil {
		badRequest(w, "bad_request", s.text(r, "federation.default_fixed"))
		return
	}
	_, err := webdb.UpdateFederation(r.Context(), s.app, v.ID, id, fw, clientIP(r))
	if s.federationError(w, r, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// deleteFederation answers DELETE /api/federations/{id}. Its groups
// fall back to Default (unless another federation holds them), and a
// member it was the only federation of is filed in Default.
func (s *Server) deleteFederation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	_, v, ok := s.federationEditor(w, r)
	if !ok {
		return
	}
	id, ok := federationID(w, r)
	if !ok {
		return
	}
	if id == scope.DefaultFederation {
		badRequest(w, "bad_request", s.text(r, "federation.default_fixed"))
		return
	}
	err := webdb.DeleteFederation(r.Context(), s.app, v.ID, id, clientIP(r))
	if s.federationError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/webdb"
)

// Pinned groups and saved searches (web/docs/settings.md). A pin is
// only made of a group the request scope sees, and only the pins the
// scope sees are shown, so losing access to a group hides its pin
// without removing it.

// pinnedGroups are the account's pins that the scope sees, in the
// order pinned.
func (s *Server) pinnedGroups(ctx context.Context, uid uint64, sc scope.Scope) ([]logdb.PinnedGroup, error) {
	ids, err := webdb.Pins(ctx, s.app, uid)
	if err != nil {
		return nil, err
	}
	return logdb.PinnedGroups(ctx, s.ro, sc, ids)
}

// groupParam is the group_id path parameter: a group's chat id is
// negative.
func groupParam(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("group_id"), 10, 64)
	return id, err == nil && id < 0
}

// pinGroup answers PUT /api/account/pinned/{group_id}.
func (s *Server) pinGroup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	id, ok := groupParam(r)
	if ok {
		var err error
		if ok, err = logdb.GroupVisible(ctx, s.ro, ScopeFrom(ctx), id); err != nil {
			s.internalError(w, "group lookup", err)
			return
		}
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", nil)
		return
	}
	err := webdb.Pin(ctx, s.app, sess.UID, id)
	if errors.Is(err, webdb.ErrLimit) {
		lang := s.textLang(ctx, sess.UID)
		writeError(w, http.StatusBadRequest, "too_many", map[string]any{
			"message": i18n.T(lang, "pins.limit", "max", strconv.Itoa(webdb.MaxPins))})
		return
	}
	if err != nil {
		s.internalError(w, "pin", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unpinGroup answers DELETE /api/account/pinned/{group_id}. A pin of
// a group the scope hides answers 404 like one that does not exist.
func (s *Server) unpinGroup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	id, ok := groupParam(r)
	if ok {
		var err error
		if ok, err = logdb.GroupVisible(ctx, s.ro, ScopeFrom(ctx), id); err != nil {
			s.internalError(w, "group lookup", err)
			return
		}
	}
	if ok {
		var err error
		if ok, err = webdb.Unpin(ctx, s.app, sess.UID, id); err != nil {
			s.internalError(w, "unpin", err)
			return
		}
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// saveSearch answers POST /api/account/saved-searches.
func (s *Server) saveSearch(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	lang := s.textLang(ctx, sess.UID)
	var in struct {
		Title *string `json:"title"`
		URL   *string `json:"url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	switch {
	case in.Title == nil || strings.TrimSpace(*in.Title) == "" ||
		!utf8.ValidString(*in.Title) ||
		utf8.RuneCountInString(strings.TrimSpace(*in.Title)) > webdb.MaxSearchTitle:
		badField(w, lang, "search.title", "title")
		return
	case in.URL == nil || len(*in.URL) > webdb.MaxSearchURL || !settings.SameSitePath(*in.URL):
		badField(w, lang, "search.url", "url")
		return
	}
	id, err := webdb.SaveSearch(ctx, s.app, sess.UID, *in.Title, *in.URL)
	if errors.Is(err, webdb.ErrLimit) {
		writeError(w, http.StatusBadRequest, "too_many", map[string]any{
			"message": i18n.T(lang, "search.limit", "max", strconv.Itoa(webdb.MaxSavedSearches))})
		return
	}
	if err != nil {
		s.internalError(w, "save search", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// deleteSavedSearch answers DELETE /api/account/saved-searches/{id}.
func (s *Server) deleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	ctx := r.Context()
	sess, _ := SessionFrom(ctx)
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	ok := err == nil
	if ok {
		if ok, err = webdb.DeleteSearch(ctx, s.app, sess.UID, id); err != nil {
			s.internalError(w, "delete search", err)
			return
		}
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

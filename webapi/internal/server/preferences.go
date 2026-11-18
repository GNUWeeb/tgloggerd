// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/settings"
)

// The account's preferences (internal/settings; web/docs/settings.md).

// textLang is the language of the texts answered to an account: its
// own choice, else the site's.
func (s *Server) textLang(ctx context.Context, uid uint64) string {
	p, err := s.viewerPrefs(ctx, uid)
	if err != nil {
		p = settings.Prefs{}
	}
	return settings.Resolve(p, s.defaults).Lang
}

// landing is the account's landing page, its own or the default; a
// preference that cannot be read gives the default.
func (s *Server) landing(ctx context.Context, uid uint64) string {
	p, err := s.viewerPrefs(ctx, uid)
	if err != nil {
		p = settings.Prefs{}
	}
	return settings.Resolve(p, s.defaults).Landing
}

// badField answers 400 bad_request for a refused key.
func badField(w http.ResponseWriter, lang, msgKey, key string) {
	writeError(w, http.StatusBadRequest, "bad_request", map[string]any{
		"message": i18n.T(lang, msgKey, "key", key),
		"field":   key,
	})
}

// putPreferences answers PUT /api/account/preferences: the body
// replaces every preference, a key left out or null going back to the
// site default. A key that is not a preference, or a value its key
// does not take, refuses the whole body.
func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	ctx := r.Context()
	lang := s.textLang(ctx, sess.UID)

	var p settings.Prefs
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		var te *json.UnmarshalTypeError
		switch {
		case errors.As(err, &te) && te.Field != "":
			badField(w, lang, "prefs.invalid", te.Field)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			key := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			badField(w, lang, "prefs.unknown", key)
		default:
			writeError(w, http.StatusBadRequest, "bad_request",
				map[string]any{"message": i18n.T(lang, "prefs.malformed")})
		}
		return
	}
	var fe *settings.FieldError
	if err := p.Check(); errors.As(err, &fe) {
		badField(w, lang, "prefs.invalid", fe.Key)
		return
	}
	if err := settings.Save(ctx, s.app, sess.UID, p); err != nil {
		s.internalError(w, "save preferences", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefs": p})
}

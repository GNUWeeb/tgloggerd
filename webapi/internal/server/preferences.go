// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

// decodePrefs reads a preferences body: the preferences it gives and
// the keys it names (null included). A key that is not a preference, a
// value of the wrong type or a value its key does not take refuses the
// whole body; the answer is written and ok is false.
func decodePrefs(w http.ResponseWriter, r *http.Request, lang string) (p settings.Prefs, keys []string, ok bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request",
			map[string]any{"message": i18n.T(lang, "prefs.malformed")})
		return p, nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
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
		return p, nil, false
	}
	var fe *settings.FieldError
	if err := p.Check(); errors.As(err, &fe) {
		badField(w, lang, "prefs.invalid", fe.Key)
		return p, nil, false
	}
	// The object decoded above, so this cannot fail.
	var named map[string]json.RawMessage
	_ = json.Unmarshal(body, &named)
	for k := range named {
		keys = append(keys, k)
	}
	return p, keys, true
}

// putPreferences answers PUT /api/account/preferences: the body
// replaces every preference, a key left out or null going back to the
// site default.
func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	ctx := r.Context()
	p, _, ok := decodePrefs(w, r, s.textLang(ctx, sess.UID))
	if !ok {
		return
	}
	if err := settings.Save(ctx, s.app, sess.UID, p); err != nil {
		s.internalError(w, "save preferences", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefs": p})
}

// patchPreferences answers PATCH /api/account/preferences: only the
// keys the body names change (null going back to the site default),
// so a toggle saving one key cannot undo another page's save of the
// others. The answer is every preference as now stored.
func (s *Server) patchPreferences(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	ctx := r.Context()
	p, keys, ok := decodePrefs(w, r, s.textLang(ctx, sess.UID))
	if !ok {
		return
	}
	if err := settings.SaveKeys(ctx, s.app, sess.UID, p, keys); err != nil {
		s.internalError(w, "save preferences", err)
		return
	}
	all, err := settings.Load(ctx, s.app, sess.UID)
	if err != nil {
		s.internalError(w, "load preferences", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefs": all})
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// "Look up real name" (web/docs/contact-names.md), the Drogon
// UsersController's lookupName as a JSON operation: the daemon removes
// the contact, reads the user's own name and adds the contact back
// (POST /v1/users/{id}/lookup-name, docs/internal-api.md). It changes
// the logging account's contacts, so it is for superadmins only (the
// lookup_name ability); to anyone else it does not exist.

// lookupAnswer is the daemon's success answer, passed on (LookupName).
type lookupAnswer struct {
	UserID       int64  `json:"user_id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ContactFirst string `json:"contact_first_name"`
	ContactLast  string `json:"contact_last_name"`
	CheckedAt    int64  `json:"checked_at"`
}

// lookupErrors are the daemon's error codes this answer passes on,
// with the status it answers them with; any other failure is 502
// daemon.
var lookupErrors = map[string]int{
	"not_found":       http.StatusNotFound,
	"not_a_contact":   http.StatusConflict,
	"unsupported":     http.StatusConflict,
	"rate_limited":    http.StatusTooManyRequests,
	"flood_wait":      http.StatusTooManyRequests,
	"unavailable":     http.StatusServiceUnavailable,
	"restore_pending": http.StatusServiceUnavailable,
	"timeout":         http.StatusGatewayTimeout,
	"telegram_error":  http.StatusBadGateway,
}

// lookupName answers POST /api/users/{id}/lookup-name.
func (s *Server) lookupName(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.LookupName)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	exists, contact, err := logdb.ContactOf(r.Context(), s.ro, sc, id)
	if err != nil {
		s.internalError(w, "contact", err)
		return
	}
	if !exists {
		notFound(w)
		return
	}
	// Refused before the daemon is asked, so a stale page cannot start
	// a cycle for a user who is no contact any more; the daemon checks
	// again.
	if !contact {
		s.refuse(w, r, v, http.StatusConflict, "not_a_contact", "lookup.not_a_contact")
		return
	}

	rep := s.daemon.LookupRealName(r.Context(), id)
	code := ""
	switch {
	case rep.OK():
	case !s.daemon.Enabled():
		code = "off"
	case !rep.Sent:
		code = "unreachable"
	default:
		var e struct {
			Error string `json:"error"`
		}
		if rep.Body != nil {
			_ = json.Unmarshal(rep.Body, &e)
		}
		code = e.Error
		if code == "" {
			code = "error"
		}
	}
	outcome := code
	if outcome == "" {
		outcome = "ok"
	}
	s.auditAdmin(r, v.ID, "contact_lookup", "user "+strconv.FormatInt(id, 10)+": "+outcome)

	if code == "" {
		var a lookupAnswer
		if rep.Body == nil || json.Unmarshal(rep.Body, &a) != nil {
			s.refuse(w, r, v, http.StatusBadGateway, "daemon", "lookup.bad_answer")
			return
		}
		writeJSON(w, http.StatusOK, a)
		return
	}
	status, known := lookupErrors[code]
	if !known {
		s.log.Warn("lookup-name failed", "user", id, "err", rep.Error)
		key := "lookup.failed"
		switch code {
		case "off":
			key = "daemon.off"
		case "unreachable":
			key = "daemon.unreachable"
		}
		s.refuse(w, r, v, http.StatusBadGateway, "daemon", key)
		return
	}
	if rep.RetryAfter != "" && (status == http.StatusTooManyRequests ||
		status == http.StatusServiceUnavailable) {
		w.Header().Set("Retry-After", rep.RetryAfter)
	}
	s.refuse(w, r, v, status, code, "lookup."+code)
}

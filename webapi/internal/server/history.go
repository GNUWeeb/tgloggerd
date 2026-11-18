// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"errors"
	"net/http"
	"strconv"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The change timelines of users and groups, page by page: GET
// /api/users/{id}/history and GET /api/groups/{id}/history. The user
// and group pages carry the first page and its next cursor
// (history_next); these answer the following ones, as the Drogon
// history page did with Newer and Older, by cursor instead of offset.

// historyAnswer is a page of a timeline (UserHistoryPage,
// GroupHistoryPage).
type historyAnswer struct {
	Entries []historyJSON `json:"entries"`
	Next    *string       `json:"next"`
}

// groupHistory turns a group's timeline into JSON, the admin changes
// with their admin.
func (v view) groupHistory(es []logdb.HistoryEntry, users map[int64]logdb.User) []historyJSON {
	out := []historyJSON{}
	for _, e := range es {
		h := v.history(e)
		if e.Kind == "admin" {
			u := v.userOf(users, e.UserID)
			h.User = &u
		}
		out = append(out, h)
	}
	return out
}

// userHistory turns an entry of a user's timeline into JSON, a photo
// change with its photo_id (the item of GET /api/users/{id}/photos).
func (v view) userHistory(e logdb.HistoryEntry) historyJSON {
	h := v.history(e)
	if e.Kind == "photo" && e.Photo.ID != 0 {
		h.PhotoID = ptrTo(e.RowID())
	}
	return h
}

// historyRequest reads the path's id and the before and limit
// parameters; false when it has answered already.
func historyRequest(w http.ResponseWriter, r *http.Request) (id int64, before string, limit int, ok bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return 0, "", 0, false
	}
	limit = logdb.HistoryPageSize
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > logdb.MaxHistoryLimit {
			badRequest(w, "bad_request", "limit must be 1 to "+
				strconv.Itoa(logdb.MaxHistoryLimit))
			return 0, "", 0, false
		}
		limit = n
	}
	return id, r.URL.Query().Get("before"), limit, true
}

func (s *Server) historyFailed(w http.ResponseWriter, err error) {
	if errors.Is(err, logdb.ErrBadCursor) {
		badRequest(w, "bad_request", "before is not a cursor of this timeline")
		return
	}
	s.internalError(w, "history", err)
}

// getUserHistory answers GET /api/users/{id}/history: for a viewer who
// sees users in full, as a member sees no one's history
// (web-accounts.md section 5); to anyone else the user does not exist.
func (s *Server) getUserHistory(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	id, before, limit, ok := historyRequest(w, r)
	if !ok {
		return
	}
	if _, ok := s.userInFull(w, r, sc, id, "history"); !ok {
		return
	}
	h, err := logdb.UserHistory(r.Context(), s.ro, id, before, limit)
	if err != nil {
		s.historyFailed(w, err)
		return
	}
	v := s.viewFor(sc, true)
	a := historyAnswer{Entries: []historyJSON{}, Next: nextCursor(h)}
	for _, e := range h.Entries {
		a.Entries = append(a.Entries, v.userHistory(e))
	}
	writeJSON(w, http.StatusOK, a)
}

// userInFull is the user for a viewer who sees users in full, as a
// member sees no one's history (web-accounts.md section 5); to anyone
// else, as for a user the scope hides or the logger never saw, it
// answers 404 and false. what names the answer in a failure's log.
func (s *Server) userInFull(w http.ResponseWriter, r *http.Request, sc scope.Scope, id int64,
	what string) (logdb.User, bool) {
	if !sc.SeesAllUsers() {
		notFound(w)
		return logdb.User{}, false
	}
	us, err := logdb.VisibleUsers(r.Context(), s.ro, sc, []int64{id})
	if err != nil {
		s.internalError(w, what, err)
		return logdb.User{}, false
	}
	if len(us) == 0 {
		notFound(w)
		return logdb.User{}, false
	}
	return us[0], true
}

// userPhotoJSON is one of a user's recorded profile photos
// (UserPhoto).
type userPhotoJSON struct {
	ID     uint64  `json:"id"`
	At     string  `json:"at"`
	URL    *string `json:"url"`
	FileID *uint64 `json:"file_id"`
}

// userPhotosAnswer is UserPhotos: the photos, and which is the current
// one.
type userPhotosAnswer struct {
	Items     []userPhotoJSON `json:"items"`
	CurrentID *uint64         `json:"current_id"`
}

// getUserPhotos answers GET /api/users/{id}/photos: every recorded
// profile photo of the user, newest first, for the user page's
// carousel. The ids are the history's photo_id, so a photo change on
// the timeline opens the carousel at its photo. The same access as
// the history.
func (s *Server) getUserPhotos(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	u, ok := s.userInFull(w, r, sc, id, "photos")
	if !ok {
		return
	}
	ps, err := logdb.UserPhotos(r.Context(), s.ro, id)
	if err != nil {
		s.internalError(w, "photos", err)
		return
	}
	v := s.viewFor(sc, true)
	a := userPhotosAnswer{Items: []userPhotoJSON{}}
	for _, p := range ps {
		a.Items = append(a.Items, userPhotoJSON{ID: p.ID, At: iso(p.At), URL: v.photo(p.File),
			FileID: ptrTo(p.File.ID)})
		// The current photo is the newest time its file was set; the
		// same file may have been the photo before.
		if a.CurrentID == nil && u.Photo.ID != 0 && p.File.ID == u.Photo.ID {
			a.CurrentID = ptrTo(p.ID)
		}
	}
	writeJSON(w, http.StatusOK, a)
}

// getGroupHistory answers GET /api/groups/{id}/history, for a group the
// scope sees.
func (s *Server) getGroupHistory(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	id, before, limit, ok := historyRequest(w, r)
	if !ok {
		return
	}
	gs, err := logdb.VisibleGroups(r.Context(), s.ro, sc, []int64{id})
	if err != nil {
		s.internalError(w, "history", err)
		return
	}
	if len(gs) == 0 {
		notFound(w)
		return
	}
	h, err := logdb.GroupHistory(r.Context(), s.ro, id, before, limit)
	if err != nil {
		s.historyFailed(w, err)
		return
	}
	users, err := logdb.HistoryUsers(r.Context(), s.ro, sc, h)
	if err != nil {
		s.internalError(w, "history", err)
		return
	}
	v := s.viewFor(sc, true)
	writeJSON(w, http.StatusOK, historyAnswer{Entries: v.groupHistory(h.Entries, users),
		Next: nextCursor(h)})
}

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

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The chat search of the pickers, GET /api/chats/search: the groups
// and private chats the viewer may see, busiest first, paged by an
// opaque cursor.

// pickItem is a chat as the pickers show it (PickItem).
type pickItem struct {
	ID    int64   `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Sub   string  `json:"sub,omitempty"`
	Photo *string `json:"photo"`
}

// chatSearchPage is GET /api/chats/search's answer (ChatSearchPage).
type chatSearchPage struct {
	Items      []pickItem `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

// The chat search's limits: a page, and how deep the cursor goes.
const (
	chatSearchDefault = 20
	chatSearchMax     = 200
	chatSearchDepth   = 100000
)

// groupTypeLabel is a group type as the pickers' second line says it.
func groupTypeLabel(t string) string {
	if t == "basic_group" {
		return "basic group"
	}
	return t
}

// The cursor is opaque to clients; it holds the offset of the next
// page.
func chatCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o" + strconv.Itoa(offset)))
}

func parseChatCursor(c string) (int, bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || len(b) < 2 || b[0] != 'o' {
		return 0, false
	}
	n, err := strconv.Atoi(string(b[1:]))
	if err != nil || n < 0 || n > chatSearchDepth {
		return 0, false
	}
	return n, true
}

// searchChats is GET /api/chats/search.
func (s *Server) searchChats(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	sc := ScopeFrom(r.Context())
	q := r.URL.Query()
	cs := logdb.ChatSearch{Q: q.Get("q"), Kind: q.Get("kind"), Limit: chatSearchDefault}
	if len(cs.Q) > logdb.MaxValueLen {
		badRequest(w, "bad_request", "The search text is too long.")
		return
	}
	if cs.Kind != "" && cs.Kind != "group" && cs.Kind != "user" {
		badRequest(w, "bad_request", "kind is group or user.")
		return
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > chatSearchMax {
			badRequest(w, "bad_request", "limit is 1 to 200.")
			return
		}
		cs.Limit = n
	}
	if v := q.Get("federation"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			badRequest(w, "bad_request", "federation is a federation id.")
			return
		}
		g, err := s.federationGroups(r.Context(), id)
		if err != nil {
			s.internalError(w, "federation groups", err)
			return
		}
		cs.Groups = &g
	}
	if v := q.Get("cursor"); v != "" {
		off, ok := parseChatCursor(v)
		if !ok {
			badRequest(w, "bad_request", "The cursor is not one this service gave.")
			return
		}
		cs.Offset = off
	}
	picks, err := logdb.SearchChats(r.Context(), s.ro, sc, cs)
	if err != nil {
		s.searchFailed(w, "chat search", err)
		return
	}
	page := chatSearchPage{Items: []pickItem{}}
	if len(picks) > cs.Limit {
		picks = picks[:cs.Limit]
		if next := cs.Offset + cs.Limit; next <= chatSearchDepth {
			c := chatCursor(next)
			page.NextCursor = &c
		}
	}
	for _, p := range picks {
		it := pickItem{ID: p.ID, Title: p.Title, Kind: p.Kind}
		switch {
		case p.Kind == "group":
			it.Sub = groupTypeLabel(p.Type)
		case p.Username != "":
			it.Sub = "@" + p.Username
		default:
			it.Sub = "private chat"
		}
		if p.Photo != 0 {
			u := s.photoURL(p.Photo)
			it.Photo = &u
		}
		page.Items = append(page.Items, it)
	}
	writeJSON(w, http.StatusOK, page)
}

// federationGroups is the groups of a federation as a set for the
// logger's queries: Default's are every group in no other federation,
// another's its stored groups; a federation that does not exist holds
// none. The groups are read from the web's own schema; the scope still
// decides which of them a viewer sees.
func (s *Server) federationGroups(ctx context.Context, id uint64) (logdb.GroupSet, error) {
	if id == scope.DefaultFederation {
		ids, err := webdb.FederatedGroups(ctx, s.app)
		return logdb.GroupSet{IDs: ids, Not: true}, err
	}
	ids, err := webdb.FederationGroups(ctx, s.app, id)
	if errors.Is(err, webdb.ErrNoFederation) {
		return logdb.GroupSet{}, nil
	}
	return logdb.GroupSet{IDs: ids}, err
}

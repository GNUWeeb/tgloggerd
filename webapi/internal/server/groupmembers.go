// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// Group members: Telegram's member count of a group, which the daemon
// keeps in telegram_groups (member_count, member_count_at), refreshed
// in the background when a page shows a missing or old one; and the
// member list, asked of the daemon page by page.

const (
	// memberCountMaxAge is how old a stored count may be before a page
	// that shows it asks the daemon for a fresh one.
	memberCountMaxAge = 10 * time.Minute
	// memberRefreshRetry is how long after asking the daemon about a
	// group the pages leave it alone: the daemon asks Telegram at most
	// once a minute per chat and answers from the stored row
	// otherwise, and a daemon that failed is not asked again on every
	// page load.
	memberRefreshRetry = time.Minute
	// memberRefreshTimeout bounds one background refresh: the daemon
	// gives Telegram 15 s.
	memberRefreshTimeout = 25 * time.Second
)

// memberRefresher asks the daemon to refresh the member counts the
// pages show, at most one call in flight per group, never in a page's
// path.
type memberRefresher struct {
	mu       sync.Mutex
	inflight map[int64]bool
	asked    map[int64]time.Time // when each group was last asked about
	wg       sync.WaitGroup      // the calls in flight, for tests
}

// refreshMemberCount asks the daemon, in the background, for a fresh
// member count of group g when the stored one is missing or older than
// memberCountMaxAge. The page shows the stored count; the daemon
// stores the new one, which the next page load shows. A failure keeps
// the stored count and is only logged.
func (s *Server) refreshMemberCount(g logdb.Group) {
	if !s.daemon.Enabled() || g.ID >= 0 {
		return
	}
	now := s.clock()
	if g.MemberCount != nil && now.Sub(time.Unix(g.MemberCountAt, 0)) < memberCountMaxAge {
		return
	}
	m := &s.members
	m.mu.Lock()
	if m.inflight[g.ID] || now.Sub(m.asked[g.ID]) < memberRefreshRetry {
		m.mu.Unlock()
		return
	}
	if m.inflight == nil {
		m.inflight, m.asked = map[int64]bool{}, map[int64]time.Time{}
	}
	m.inflight[g.ID], m.asked[g.ID] = true, now
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), memberRefreshTimeout)
		defer cancel()
		ref, r, ok := s.daemon.RefreshChat(ctx, g.ID)
		switch {
		case !ok:
			s.log.Info("member count refresh failed", "chat", g.ID, "err", r.Error)
		case ref.Error != "":
			s.log.Info("member count refresh: Telegram failed, stored count kept",
				"chat", g.ID, "err", ref.Error)
		}
		m.mu.Lock()
		delete(m.inflight, g.ID)
		m.mu.Unlock()
	}()
}

// mayHideMemberList is whether the viewer may hide group g's member
// list or show it again: superadmins, on channels only (a group's
// members see each other in Telegram anyway).
func mayHideMemberList(sc scope.Scope, g logdb.Group) bool {
	return sc.Role() == scope.Superadmin && g.Type == "channel"
}

// groupWriteJSON is PATCH /api/groups/{id}'s answer.
type groupWriteJSON struct {
	ID                int64 `json:"id"`
	HideMemberList    bool  `json:"hide_member_list"`
	CanHideMemberList bool  `json:"can_hide_member_list"`
}

// updateGroup answers PATCH /api/groups/{id} {"hide_member_list":
// bool}: a superadmin hides a channel's member list from everyone but
// the superadmins, or shows it again. A group the viewer does not see
// is 404; one it sees but may not change, 403; a group that is not a
// channel, 400.
func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc := ScopeFrom(r.Context())
	v, _ := sc.Viewer()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id >= 0 {
		notFound(w)
		return
	}
	ctx := r.Context()
	gs, err := logdb.VisibleGroups(ctx, s.ro, sc, []int64{id})
	if err != nil {
		s.internalError(w, "group", err)
		return
	}
	if len(gs) == 0 {
		notFound(w)
		return
	}
	g := gs[0]
	if sc.Role() != scope.Superadmin {
		s.refuse(w, r, v, http.StatusForbidden, "forbidden", "group.superadmins_only")
		return
	}
	var in struct {
		HideMemberList *bool `json:"hide_member_list"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.HideMemberList == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "group.write_fields")
		return
	}
	if !mayHideMemberList(sc, g) {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "group.channels_only")
		return
	}
	changed, err := webdb.SetMemberListHidden(ctx, s.app, id, *in.HideMemberList, v.ID)
	if err != nil {
		s.internalError(w, "hide member list", err)
		return
	}
	if changed {
		action := "member_list_show"
		if *in.HideMemberList {
			action = "member_list_hide"
		}
		s.auditAdmin(r, v.ID, action, "group "+strconv.FormatInt(id, 10))
	}
	writeJSON(w, http.StatusOK, groupWriteJSON{ID: id, HideMemberList: *in.HideMemberList,
		CanHideMemberList: true})
}

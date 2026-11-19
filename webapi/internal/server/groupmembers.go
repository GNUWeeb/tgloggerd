// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"tgloggerd/webapi/internal/daemon"
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

// memberState is the service's group member bookkeeping: the count
// refreshes in flight (at most one per group, never in a page's path)
// and the cached first pages of member lists.
type memberState struct {
	mu       sync.Mutex
	inflight map[int64]bool
	asked    map[int64]time.Time // when each group was last asked about
	wg       sync.WaitGroup      // the calls in flight, for tests
	pages    map[memberPageKey]memberPageEntry
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

const (
	// memberPageDefault and memberPageMax bound a page of members, as
	// the daemon does; memberOffsetMax is the daemon's highest offset.
	memberPageDefault = 50
	memberPageMax     = 200
	memberOffsetMax   = 1000000
	// memberPageTTL is how long a page of a group's members (or of a
	// search in them) is kept, so the page's background prefetch makes
	// the Members tab instant and paging back and forth in a search
	// does not ask Telegram again. The daemon keeps its own answers as
	// long.
	memberPageTTL = 2 * time.Minute
	// memberQueryMax is the longest member search, in characters, as
	// the daemon takes it.
	memberQueryMax = 64
	// memberPagesMax bounds the cache; a full cache drops what has
	// expired, then everything.
	memberPagesMax = 1000
)

type memberPageKey struct {
	group  int64
	q      string
	offset int
	limit  int
}

type memberPageEntry struct {
	at time.Time
	m  daemon.Members
}

// cachedPage is a page of a group's members as the daemon answered it
// within memberPageTTL.
func (m *memberState) cachedPage(k memberPageKey, now time.Time) (daemon.Members, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.pages[k]
	if !ok || now.Sub(e.at) >= memberPageTTL {
		return daemon.Members{}, false
	}
	return e.m, true
}

func (m *memberState) keepPage(k memberPageKey, now time.Time, page daemon.Members) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pages == nil {
		m.pages = map[memberPageKey]memberPageEntry{}
	}
	if len(m.pages) >= memberPagesMax {
		for key, e := range m.pages {
			if now.Sub(e.at) >= memberPageTTL {
				delete(m.pages, key)
			}
		}
		if len(m.pages) >= memberPagesMax {
			clear(m.pages)
		}
	}
	m.pages[k] = memberPageEntry{at: now, m: page}
}

// memberJSON is one member of a group's list (GroupMember); MatchedBy
// only in a search.
type memberJSON struct {
	User      userSummaryJSON `json:"user"`
	Status    string          `json:"status"`
	JoinedAt  *string         `json:"joined_at"`
	MatchedBy string          `json:"matched_by,omitempty"`
}

// membersPageJSON is a page of a group's members (GroupMembersPage).
type membersPageJSON struct {
	Available  bool         `json:"available"`
	Reason     string       `json:"reason"`
	RetryAfter *int64       `json:"retry_after,omitempty"`
	Total      *int64       `json:"total"`
	Members    []memberJSON `json:"members"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
}

// seesMemberList is whether the viewer may have group lists of
// members at all: those who see users in full (superadmins and
// admins). Members and signed-out visitors get the count only.
func seesMemberList(sc scope.Scope) bool {
	return sc.SeesAllUsers()
}

// memberPage is one page of group g's members, or of those matching
// the search q (memberQuery's), for a viewer who sees g and
// seesMemberList. The daemon lists them from Telegram; a list a
// superadmin hid answers reason hidden to everyone but superadmins, and
// a daemon that is off or does not answer, reason unavailable. The
// members are named from telegram_users in one query; one not stored
// yet is named by its id. A search's total is the daemon's count of
// matches, never the group's member count.
func (s *Server) memberPage(ctx context.Context, sc scope.Scope, g logdb.Group,
	q string, offset, limit int) (membersPageJSON, error) {
	out := membersPageJSON{Members: []memberJSON{}, Offset: offset, Limit: limit}
	if g.MemberCount != nil && q == "" {
		n := int64(*g.MemberCount)
		out.Total = &n
	}
	hidden, err := webdb.MemberListHidden(ctx, s.app, g.ID)
	if err != nil {
		return out, err
	}
	if hidden && sc.Role() != scope.Superadmin {
		out.Reason = "hidden"
		return out, nil
	}

	k := memberPageKey{group: g.ID, q: q, offset: offset, limit: limit}
	page, ok := s.members.cachedPage(k, s.clock())
	if !ok {
		var r daemon.Reply
		page, r, ok = s.daemon.ChatMembers(ctx, g.ID, q, offset, limit)
		if !ok {
			s.log.Info("member list unavailable", "chat", g.ID, "err", r.Error)
			out.Reason = "unavailable"
			return out, nil
		}
		// A rate limit or a failure is asked again next time, as the
		// daemon does: its wait may be shorter than the cache's.
		if page.Available || (page.Reason != "flood_wait" && page.Reason != "error") {
			s.members.keepPage(k, s.clock(), page)
		}
	}
	if page.Total != nil {
		out.Total = page.Total
	}
	if !page.Available {
		out.Reason = page.Reason
		if out.Reason == "" {
			out.Reason = "error"
		}
		if out.Reason == "flood_wait" && page.RetryAfter > 0 {
			ra := page.RetryAfter
			out.RetryAfter = &ra
		}
		return out, nil
	}
	out.Available = true
	ids := make([]int64, 0, len(page.Members))
	for _, m := range page.Members {
		ids = append(ids, m.UserID)
	}
	us, err := logdb.VisibleUsers(ctx, s.ro, sc, ids)
	if err != nil {
		return out, err
	}
	users := make(map[int64]logdb.User, len(us))
	for _, u := range us {
		users[u.ID] = u
	}
	v := s.viewFor(sc, true)
	for _, m := range page.Members {
		j := memberJSON{User: v.userOf(users, m.UserID), Status: m.Status}
		if q != "" {
			j.MatchedBy = m.MatchedBy
		}
		if m.JoinedAt != nil {
			j.JoinedAt = isoPtr(*m.JoinedAt)
		}
		out.Members = append(out.Members, j)
	}
	return out, nil
}

// memberPageArgs reads ?offset= and ?limit=: offset 0 to
// memberOffsetMax, limit 1 to memberPageMax (memberPageDefault when
// absent).
func memberPageArgs(q url.Values) (offset, limit int, ok bool) {
	offset, limit = 0, memberPageDefault
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > memberOffsetMax {
			return 0, 0, false
		}
		offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > memberPageMax {
			return 0, 0, false
		}
		limit = n
	}
	return offset, limit, true
}

// memberQuery reads ?q=, a member search: trimmed, a leading @
// dropped (a username), at most memberQueryMax characters of UTF-8.
// An empty one, or a lone @, is the plain list (""). ok is false for
// one that is too long or not UTF-8.
func memberQuery(v url.Values) (q string, ok bool) {
	q = strings.TrimSpace(v.Get("q"))
	q = strings.TrimSpace(strings.TrimPrefix(q, "@"))
	if !utf8.ValidString(q) || utf8.RuneCountInString(q) > memberQueryMax {
		return "", false
	}
	return q, true
}

// getGroupMembers answers GET /api/groups/{id}/members?offset=&limit=
// &q=: a page of the group's members, or of those matching the search
// q, for a viewer who sees the group and users in full; for anyone
// else, signed out included, the group's list does not exist (404),
// and its count is on the group's pages.
func (s *Server) getGroupMembers(w http.ResponseWriter, r *http.Request) {
	sess, sc, ok := s.optionalSession(w, r)
	if !ok {
		return
	}
	noStore(w)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if sess == nil || err != nil || id >= 0 || !seesMemberList(sc) {
		notFound(w)
		return
	}
	offset, limit, ok := memberPageArgs(r.URL.Query())
	if !ok {
		badRequest(w, "bad_request", "offset is 0 to "+strconv.Itoa(memberOffsetMax)+
			" and limit 1 to "+strconv.Itoa(memberPageMax)+".")
		return
	}
	q, ok := memberQuery(r.URL.Query())
	if !ok {
		badRequest(w, "bad_request", "q is at most "+strconv.Itoa(memberQueryMax)+" characters.")
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
	p, err := s.memberPage(ctx, sc, gs[0], q, offset, limit)
	if err != nil {
		s.internalError(w, "members", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// groupMembersProps are the group_members page's (GroupMembersProps):
// the group's page as its other tabs have it, and a page of its
// members.
type groupMembersProps struct {
	groupProps
	membersPageJSON
}

// loadGroupMembers is the group_members route, /groups/{id}/members:
// for a viewer who sees the group and seesMemberList, the group's page
// and the page of members its ?offset= names (?limit= and the search
// ?q= as the API's; out of range, the first page and no search);
// anyone else gets not_found.
func loadGroupMembers(ctx context.Context, pr *pageRequest) (any, error) {
	if !seesMemberList(pr.scope) {
		return nil, errPageNotFound
	}
	offset, limit, ok := memberPageArgs(pr.query)
	if !ok {
		offset, limit = 0, memberPageDefault
	}
	q, ok := memberQuery(pr.query)
	if !ok {
		q = ""
	}
	gp, g, err := groupPage(ctx, pr)
	if err != nil {
		return nil, err
	}
	mp, err := pr.s.memberPage(ctx, pr.scope, g, q, offset, limit)
	if err != nil {
		return nil, err
	}
	return groupMembersProps{groupProps: gp, membersPageJSON: mp}, nil
}

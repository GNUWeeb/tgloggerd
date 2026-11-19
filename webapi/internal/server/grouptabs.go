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

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The group page's Files and Polls tabs (webapi/openapi.yaml:
// GroupFilesProps, GroupPollsProps): the routes group_files and
// group_polls carry the group page's props (GroupProps), as its other
// tabs do, and beside them the newest page of the group's files or
// polls; GET /api/groups/{id}/files and /api/groups/{id}/polls answer
// the same lists, for the following pages and for other clients. For a
// signed-in viewer whose scope sees the group: unlike its overview and
// chat, an exposed group's tabs are not public.

// tabSenderJSON is who sent a tab's message (TabSender): a user, or a
// chat posting in its own name, named only when the viewer sees it.
type tabSenderJSON struct {
	ID       *int64  `json:"id"`
	Name     string  `json:"name"`
	Username string  `json:"username"`
	Photo    *string `json:"photo"`
}

// tabSender is a message's sender as the chat shows it (view.sender),
// nil for the logging account's own messages; a chat the viewer may
// not see and a sender known only by its signature have no id.
func (v view) tabSender(n names, m logdb.Message) *tabSenderJSON {
	s := v.sender(n, m)
	if s == nil {
		return nil
	}
	j := tabSenderJSON{Name: s.Name, Username: s.Username, Photo: s.Photo}
	if s.ID != 0 {
		j.ID = ptrTo(s.ID)
	}
	return &j
}

// tabMessageJSON are the fields of a tab's row that point at its
// message.
type tabMessageJSON struct {
	MessageID    int64          `json:"message_id"`
	MessageRowID uint64         `json:"message_row_id"`
	Date         string         `json:"date"`
	Sender       *tabSenderJSON `json:"sender"`
	ChatLink     string         `json:"chat_link"`
}

func (v view) tabMessage(n names, chat int64, m logdb.Message) tabMessageJSON {
	return tabMessageJSON{MessageID: m.MessageID, MessageRowID: m.RowID, Date: iso(m.Date),
		Sender: v.tabSender(n, m),
		ChatLink: "/groups/" + strconv.FormatInt(chat, 10) + "/chat?at=" +
			strconv.FormatInt(m.MessageID, 10)}
}

// groupFileRowJSON is GroupFileRow: the file as the files list shows it
// (FileRow, without the message that carried it there) and its
// message.
type groupFileRowJSON struct {
	ID        uint64  `json:"id"`
	FileType  string  `json:"file_type"`
	Name      string  `json:"name"`
	Ext       string  `json:"ext"`
	Size      uint64  `json:"size"`
	Stored    bool    `json:"stored"`
	Fetchable bool    `json:"fetchable"`
	URL       *string `json:"url"`
	Thumb     *string `json:"thumb"`
	Minithumb string  `json:"minithumb,omitempty"`
	tabMessageJSON
}

type kindCountJSON struct {
	Kind  string `json:"kind"`
	Count *int64 `json:"count"`
}

// groupFilesJSON is GroupFilesPage, the answer of GET
// /api/groups/{id}/files and the files tab's own props.
type groupFilesJSON struct {
	Kinds      []kindCountJSON    `json:"kinds"`
	Kind       string             `json:"kind"`
	Rows       []groupFileRowJSON `json:"rows"`
	NextBefore *int64             `json:"next_before"`
}

type groupFilesProps struct {
	groupProps
	groupFilesJSON
}

// groupPollRowJSON is GroupPollRow: a poll message and its poll, as the
// chat shows it (PollDetail); nil when the poll is not stored.
type groupPollRowJSON struct {
	tabMessageJSON
	Poll any `json:"poll"`
}

// groupPollsJSON is GroupPollsPage.
type groupPollsJSON struct {
	Rows       []groupPollRowJSON `json:"rows"`
	NextBefore *int64             `json:"next_before"`
}

type groupPollsProps struct {
	groupProps
	groupPollsJSON
}

// tabQuery is a tab's query string: ?before= the message id the page
// lies below, and for the files ?kind=. ok is false for one that does
// not parse, with the parameter it is about.
type tabQuery struct {
	before int64
	kind   string // "" for all
}

func parseTabQuery(q map[string][]string, files bool) (tabQuery, string, bool) {
	var t tabQuery
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	if b := get("before"); b != "" {
		n, err := strconv.ParseInt(b, 10, 64)
		if err != nil || n <= 0 {
			return t, "before", false
		}
		t.before = n
	}
	if files {
		switch k := get("kind"); {
		case k == "" || k == "all":
		case logdb.IsFileKind(k):
			t.kind = k
		default:
			return t, "kind", false
		}
	}
	return t, "", true
}

func nextBefore(p logdb.TabPage) *int64 {
	if p.NextBefore == 0 {
		return nil
	}
	return ptrTo(p.NextBefore)
}

// groupFiles is the files tab's list of a group the scope sees.
func (s *Server) groupFiles(ctx context.Context, sc scope.Scope, signedIn bool, chat int64,
	t tabQuery) (groupFilesJSON, error) {
	kinds, err := logdb.GroupFileKinds(ctx, s.ro, chat)
	if err != nil {
		return groupFilesJSON{}, err
	}
	p, err := logdb.GroupFiles(ctx, s.ro, sc, chat, t.kind, t.before, logdb.TabPageSize)
	if err != nil {
		return groupFilesJSON{}, err
	}
	rows := make([]logdb.Row, 0, len(p.Items))
	for _, it := range p.Items {
		rows = append(rows, it.File)
	}
	stills, err := s.fileStills(ctx, sc, rows)
	if err != nil {
		return groupFilesJSON{}, err
	}
	v := s.viewFor(sc, signedIn)
	n := names{p.Users, p.Groups}
	out := groupFilesJSON{Kinds: []kindCountJSON{}, Kind: "all", Rows: []groupFileRowJSON{},
		NextBefore: nextBefore(p)}
	if t.kind != "" {
		out.Kind = t.kind
	}
	for _, k := range kinds {
		out.Kinds = append(out.Kinds, kindCountJSON{Kind: k.Kind, Count: k.Count})
	}
	for _, it := range p.Items {
		f := s.fileRow(sc, signedIn, it.File, stills)
		out.Rows = append(out.Rows, groupFileRowJSON{ID: f.ID, FileType: f.FileType, Name: f.Name,
			Ext: f.Ext, Size: f.Size, Stored: f.Stored, Fetchable: f.Fetchable, URL: f.URL,
			Thumb: f.Thumb, Minithumb: f.Minithumb, tabMessageJSON: v.tabMessage(n, chat, it.Message)})
	}
	return out, nil
}

// groupPolls is the polls tab's list of a group the scope sees, each
// poll as the chat's detail of it, the polls read in one batch.
func (s *Server) groupPolls(ctx context.Context, sc scope.Scope, signedIn bool, chat int64,
	t tabQuery) (groupPollsJSON, error) {
	p, err := logdb.GroupPolls(ctx, s.ro, sc, chat, t.before, logdb.TabPageSize)
	if err != nil {
		return groupPollsJSON{}, err
	}
	ms := make([]logdb.Message, 0, len(p.Items))
	for _, it := range p.Items {
		ms = append(ms, it.Message)
	}
	k, err := s.loadKinds(ctx, sc, chat, ms)
	if err != nil {
		return groupPollsJSON{}, err
	}
	v := s.viewFor(sc, signedIn)
	n := names{p.Users, p.Groups}
	out := groupPollsJSON{Rows: []groupPollRowJSON{}, NextBefore: nextBefore(p)}
	for _, m := range ms {
		row := groupPollRowJSON{tabMessageJSON: v.tabMessage(n, chat, m)}
		if d := v.detail(k, m); d != nil {
			row.Poll = d
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

// groupHead is the group page's own props (GroupProps), as its other
// tabs carry them, for a viewer that sees the group; errPageNotFound
// otherwise. The tab's ?before= is a message id, not the history's
// cursor, so the overview is read from its newest history.
func groupHead(ctx context.Context, pr *pageRequest) (groupProps, error) {
	gp := *pr
	gp.query = url.Values{}
	p, err := loadGroup(ctx, &gp)
	if err != nil {
		return groupProps{}, err
	}
	return p.(groupProps), nil
}

// tabQueryOf is the page's tab query; one that does not parse lists
// from the newest, of every kind, as a page does not answer 400.
func (pr *pageRequest) tabQueryOf(files bool) tabQuery {
	t, _, ok := parseTabQuery(pr.query, files)
	if !ok {
		return tabQuery{}
	}
	return t
}

func loadGroupFiles(ctx context.Context, pr *pageRequest) (any, error) {
	g, err := groupHead(ctx, pr)
	if err != nil {
		return nil, err
	}
	f, err := pr.s.groupFiles(ctx, pr.scope, pr.sess != nil, g.Group.ID, pr.tabQueryOf(true))
	if err != nil {
		return nil, err
	}
	return groupFilesProps{groupProps: g, groupFilesJSON: f}, nil
}

func loadGroupPolls(ctx context.Context, pr *pageRequest) (any, error) {
	g, err := groupHead(ctx, pr)
	if err != nil {
		return nil, err
	}
	p, err := pr.s.groupPolls(ctx, pr.scope, pr.sess != nil, g.Group.ID, pr.tabQueryOf(false))
	if err != nil {
		return nil, err
	}
	return groupPollsProps{groupProps: g, groupPollsJSON: p}, nil
}

// tabRequest reads a tab API request: the group, which the scope must
// see (404 otherwise, as for one that does not exist), and its query
// (400 for one that does not parse).
func (s *Server) tabRequest(w http.ResponseWriter, r *http.Request, files bool) (int64, tabQuery, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return 0, tabQuery{}, false
	}
	t, field, ok := parseTabQuery(r.URL.Query(), files)
	if !ok {
		msg := "before must be a positive message id"
		if field == "kind" {
			msg = "kind must be all or one of the file kinds"
		}
		badRequest(w, "bad_request", msg)
		return 0, tabQuery{}, false
	}
	gs, err := logdb.VisibleGroups(r.Context(), s.ro, ScopeFrom(r.Context()), []int64{id})
	if err != nil {
		s.internalError(w, "group tab", err)
		return 0, tabQuery{}, false
	}
	if len(gs) == 0 {
		notFound(w)
		return 0, tabQuery{}, false
	}
	return id, t, true
}

// getGroupFiles answers GET /api/groups/{id}/files.
func (s *Server) getGroupFiles(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	id, t, ok := s.tabRequest(w, r, true)
	if !ok {
		return
	}
	out, err := s.groupFiles(r.Context(), ScopeFrom(r.Context()), true, id, t)
	if err != nil {
		s.internalError(w, "group files", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// getGroupPolls answers GET /api/groups/{id}/polls.
func (s *Server) getGroupPolls(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	id, t, ok := s.tabRequest(w, r, false)
	if !ok {
		return
	}
	out, err := s.groupPolls(r.Context(), ScopeFrom(r.Context()), true, id, t)
	if err != nil {
		s.internalError(w, "group polls", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

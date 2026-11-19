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
	"time"
	"unicode/utf8"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/mcp"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The MCP administration (web/docs/mcp.md), the Drogon
// McpAdminController as JSON operations: which groups the MCP endpoint
// exposes (telegram_public_groups, which also makes their chats
// public), and its bearer tokens (web_mcp_tokens). They need the mcp
// ability; a group the scope hides can be neither exposed nor
// withdrawn, as if it did not exist (web-accounts.md section 4).

// Column lengths, in characters.
const (
	mcpNoteMax      = 255
	mcpTokenNameMax = 64
)

// mintMCPToken is a new token and the SHA-256 it is stored as, made
// by the MCP endpoint's own rules (mcp.NewToken), so every token the
// pages mint is one the endpoint accepts.
func mintMCPToken() (string, []byte, error) {
	tok, sum, err := mcp.NewToken()
	return tok, sum[:], err
}

// exposeMcpGroup answers POST /api/mcp/groups.
func (s *Server) exposeMcpGroup(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.ManageMCP)
	if !ok {
		return
	}
	var in struct {
		GroupID *int64  `json:"group_id"`
		Note    *string `json:"note"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.GroupID == nil || *in.GroupID >= 0 {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "mcp.group_required")
		return
	}
	ctx := r.Context()
	visible, err := logdb.GroupVisible(ctx, s.ro, sc, *in.GroupID)
	if err != nil {
		s.internalError(w, "group", err)
		return
	}
	if !visible {
		notFound(w)
		return
	}
	note := ""
	if in.Note != nil {
		note = clip(*in.Note, mcpNoteMax)
	}
	added, err := logdb.ExposeGroup(ctx, s.ro, *in.GroupID, note, v.ID)
	if err != nil {
		s.internalError(w, "expose group", err)
		return
	}
	if !added {
		// Its note changes by PATCH, which keeps who exposed it and
		// when.
		s.refuse(w, r, v, http.StatusConflict, "already_exposed", "mcp.already_exposed")
		return
	}
	s.auditAdmin(r, v.ID, "mcp_expose", "group "+strconv.FormatInt(*in.GroupID, 10)+" "+note)
	s.recordOwner(r, webdb.Resource{Type: webdb.ResMCPGroup, ID: *in.GroupID}, v.ID)
	writeJSON(w, http.StatusCreated, map[string]int64{"group_id": *in.GroupID})
}

// unexposeMcpGroup answers DELETE /api/mcp/groups/{group_id}. Taking a
// group off can only reduce what the endpoint shows, so it asks for
// nothing more than the page's confirmation.
func (s *Server) unexposeMcpGroup(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.ManageMCP)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("group_id"), 10, 64)
	if err != nil || id >= 0 {
		notFound(w)
		return
	}
	ctx := r.Context()
	visible, err := logdb.GroupVisible(ctx, s.ro, sc, id)
	if err != nil {
		s.internalError(w, "group", err)
		return
	}
	if !visible {
		notFound(w)
		return
	}
	removed, err := logdb.UnexposeGroup(ctx, s.ro, id)
	if err != nil {
		s.internalError(w, "unexpose group", err)
		return
	}
	if !removed {
		notFound(w)
		return
	}
	s.auditAdmin(r, v.ID, "mcp_unexpose", "group "+strconv.FormatInt(id, 10))
	s.dropOwner(r, webdb.Resource{Type: webdb.ResMCPGroup, ID: id}, v.ID)
	w.WriteHeader(http.StatusNoContent)
}

// setMcpGroupNote answers PATCH /api/mcp/groups/{group_id}: the note
// of an exposed group is reworded in place, under the same permission
// as exposing it. Who exposed the group and when stay as they are; the
// audit row keeps the old and the new note, so the earlier reason is
// not lost. An unchanged note answers 204 and audits nothing.
func (s *Server) setMcpGroupNote(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.ManageMCP)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("group_id"), 10, 64)
	if err != nil || id >= 0 {
		notFound(w)
		return
	}
	var in struct {
		Note *string `json:"note"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Note == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "mcp.note_required")
		return
	}
	ctx := r.Context()
	visible, err := logdb.GroupVisible(ctx, s.ro, sc, id)
	if err != nil {
		s.internalError(w, "group", err)
		return
	}
	if !visible {
		notFound(w)
		return
	}
	note := clip(*in.Note, mcpNoteMax)
	old, found, err := logdb.SetExposedNote(ctx, s.ro, id, note)
	if err != nil {
		s.internalError(w, "group note", err)
		return
	}
	if !found {
		notFound(w)
		return
	}
	if old != note {
		s.auditAdmin(r, v.ID, "mcp_note", noteChange("group "+strconv.FormatInt(id, 10)+" ", old, note))
	}
	w.WriteHeader(http.StatusNoContent)
}

// noteChange is the audit detail of a reworded note, prefix then
// "old -> new". The detail column holds webdb.MaxAuditDetail
// characters; when both notes do not fit, each is cut to its share
// (a short one leaves the rest to the other), so neither is lost.
func noteChange(prefix, old, note string) string {
	const sep = " -> "
	room := webdb.MaxAuditDetail - utf8.RuneCountInString(prefix) - len(sep)
	no, nn := utf8.RuneCountInString(old), utf8.RuneCountInString(note)
	if no+nn > room {
		half := room / 2
		switch {
		case no <= half:
			nn = room - no
		case nn <= half:
			no = room - nn
		default:
			no, nn = half, room-half
		}
	}
	return prefix + clip(old, no) + sep + clip(note, nn)
}

// createMcpToken answers POST /api/mcp/tokens. The token is in this
// answer only: it is stored as its SHA-256, and neither the log nor the
// audit row sees it.
func (s *Server) createMcpToken(w http.ResponseWriter, r *http.Request) {
	_, v, ok := s.adminWrite(w, r, scope.ManageMCP)
	if !ok {
		return
	}
	var in struct {
		Name *string `json:"name"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	name := ""
	if in.Name != nil {
		name = clip(*in.Name, mcpTokenNameMax)
	}
	if name == "" {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "mcp.name_required")
		return
	}
	tok, sum, err := mintMCPToken()
	if err != nil {
		s.internalError(w, "mint token", err)
		return
	}
	id, err := webdb.CreateMCPToken(r.Context(), s.app, v.ID, name, sum)
	if err != nil {
		s.internalError(w, "store token", err)
		return
	}
	s.auditAdmin(r, v.ID, "mcp_token_create", "token "+strconv.FormatUint(id, 10)+" "+name)
	s.recordOwner(r, webdb.Resource{Type: webdb.ResMCPToken, ID: int64(id)}, v.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "token": tok})
}

// revokeMcpToken answers POST /api/mcp/tokens/{id}/revoke: revoked_at
// is stamped, the row stays. Revoking a revoked token changes nothing
// and answers 204 as well.
func (s *Server) revokeMcpToken(w http.ResponseWriter, r *http.Request) {
	_, v, ok := s.adminWrite(w, r, scope.ManageMCP)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	ctx := r.Context()
	name, err := webdb.MCPTokenName(ctx, s.app, id)
	if errors.Is(err, webdb.ErrNoToken) {
		notFound(w)
		return
	}
	if err != nil {
		s.internalError(w, "token", err)
		return
	}
	revoked, err := webdb.RevokeMCPToken(ctx, s.app, id)
	if err != nil {
		s.internalError(w, "revoke token", err)
		return
	}
	if revoked {
		s.auditAdmin(r, v.ID, "mcp_token_revoke", "token "+strconv.FormatUint(id, 10)+" "+name)
	}
	w.WriteHeader(http.StatusNoContent)
}

// mcpIndexProps are the MCP page's (McpIndexProps).
type mcpIndexProps struct {
	ExposedGroups   int     `json:"exposed_groups"`
	ExposedMessages int64   `json:"exposed_messages"`
	ActiveTokens    int     `json:"active_tokens"`
	LastUsed        *string `json:"last_used"`
	Endpoint        string  `json:"endpoint"`
}

// mcpEndpoint is the MCP endpoint's URL on the site the page was asked
// on: nginx passes the Host on, and the scheme in X-Forwarded-Proto.
func mcpEndpoint(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "https" || p == "http" {
		scheme = p
	}
	return scheme + "://" + r.Host + "/mcp"
}

func timePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func loadMcp(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "mcp"); err != nil {
		return nil, err
	}
	s := pr.s
	var p mcpIndexProps
	var err error
	if p.ExposedGroups, p.ExposedMessages, err = logdb.ExposedStats(ctx, s.ro, pr.scope); err != nil {
		return nil, err
	}
	n, last, err := webdb.MCPTokenStats(ctx, s.app)
	if err != nil {
		return nil, err
	}
	p.ActiveTokens, p.LastUsed, p.Endpoint = n, timePtr(last), mcpEndpoint(pr.r)
	return p, nil
}

type mcpGroupJSON struct {
	Group     groupSummaryJSON `json:"group"`
	PublicNow bool             `json:"public_now"`
	Note      string           `json:"note"`
	Added     string           `json:"added"`
}

type mcpGroupsProps struct {
	Groups []mcpGroupJSON `json:"groups"`
}

func loadMcpGroups(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "mcp"); err != nil {
		return nil, err
	}
	s := pr.s
	gs, err := logdb.ExposedGroups(ctx, s.ro, pr.scope)
	if err != nil {
		return nil, err
	}
	v := s.viewFor(pr.scope, true)
	p := mcpGroupsProps{Groups: []mcpGroupJSON{}}
	for _, g := range gs {
		j := v.group(g.Group)
		// GroupCounted: the count is always there on this page, which
		// only viewers that see groups in full reach.
		n := g.Group.MsgCount
		j.MsgCount = &n
		p.Groups = append(p.Groups, mcpGroupJSON{Group: j, PublicNow: g.PublicNow,
			Note: g.Note, Added: iso(g.Added)})
	}
	return p, nil
}

type mcpTokenJSON struct {
	ID       uint64  `json:"id"`
	Name     string  `json:"name"`
	Owner    string  `json:"owner"`
	Created  string  `json:"created"`
	LastUsed *string `json:"last_used"`
	Revoked  *string `json:"revoked"`
}

type mcpTokensProps struct {
	Tokens []mcpTokenJSON `json:"tokens"`
}

func loadMcpTokens(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "mcp"); err != nil {
		return nil, err
	}
	ts, err := webdb.ListMCPTokens(ctx, pr.s.app)
	if err != nil {
		return nil, err
	}
	p := mcpTokensProps{Tokens: []mcpTokenJSON{}}
	for _, t := range ts {
		p.Tokens = append(p.Tokens, mcpTokenJSON{ID: t.ID, Name: t.Name, Owner: t.Owner,
			Created: t.Created.Format(time.RFC3339), LastUsed: timePtr(t.LastUsed),
			Revoked: timePtr(t.Revoked)})
	}
	return p, nil
}

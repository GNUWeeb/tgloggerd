// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// Join chat (web/docs/join-chat.md), the Drogon JoinChatController as
// JSON operations: preview the chat behind an invite link or a public
// username, and make the logging account join it. Only the daemon
// holds the TDLib session, so both ask it (POST /v1/chats/resolve and
// /v1/chats/join, docs/internal-api.md). They need the join_chat
// ability: superadmins, and admins it is granted to
// (web-admin-restrictions); for anyone else they do not exist.

// maxJoinRef is the longest reference passed to the daemon, in bytes.
const maxJoinRef = 256

// joinRecent is how many attempts the page lists.
const joinRecent = 20

// joinChatJSON is the daemon's chat object (JoinChat). Its chat_id is
// null for a chat the account cannot see before joining; that is
// answered as 0, which is no chat's id.
type joinChatJSON struct {
	ChatID        int64  `json:"chat_id"`
	Title         string `json:"title"`
	Type          string `json:"type"`
	MemberCount   int64  `json:"member_count"`
	Username      string `json:"username"`
	IsMember      bool   `json:"is_member"`
	NeedsApproval bool   `json:"needs_approval"`
	Banned        bool   `json:"banned"`
	Paid          bool   `json:"paid"`
}

// joinAnswer is the daemon's answer, and this service's (JoinOutcome).
type joinAnswer struct {
	Outcome    string        `json:"outcome"`
	Message    string        `json:"message"`
	Chat       *joinChatJSON `json:"chat"`
	Ref        string        `json:"ref"`
	RetryAfter *int64        `json:"retry_after,omitempty"`
	// FederationIDs and FilingPending answer a join only: where the
	// group was filed, or is held for (filing_pending).
	FederationIDs *[]uint64 `json:"federation_ids,omitempty"`
	FilingPending *bool     `json:"filing_pending,omitempty"`
}

// joinedGroup is the chat id of the group or channel a answers about,
// 0 when it names none.
func (a joinAnswer) joinedGroup() int64 {
	if a.Chat == nil || a.Chat.ChatID >= 0 {
		return 0
	}
	return a.Chat.ChatID
}

// daemonJoin reads a daemon answer: a 200 with an outcome, else false.
func daemonJoin(r daemon.Reply) (joinAnswer, bool) {
	var a struct {
		Outcome    *string       `json:"outcome"`
		Message    string        `json:"message"`
		Ref        string        `json:"ref"`
		RetryAfter int64         `json:"retry_after"`
		Chat       *joinChatJSON `json:"chat"`
	}
	if !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &a) != nil || a.Outcome == nil {
		return joinAnswer{}, false
	}
	out := joinAnswer{Outcome: *a.Outcome, Message: a.Message, Ref: a.Ref, Chat: a.Chat}
	if out.Outcome == "flood_wait" {
		ra := a.RetryAfter
		out.RetryAfter = &ra
	}
	return out, true
}

// outcomeText is the sentence an outcome is shown with, as the Drogon
// page words it; empty for ok.
func (s *Server) outcomeText(ctx context.Context, v scope.Viewer, a joinAnswer) string {
	switch a.Outcome {
	case "ok":
		return ""
	case "joined", "already_member", "request_sent", "approval_required", "declined",
		"not_a_group", "not_found", "link_invalid", "banned", "too_many_chats":
		return s.textFor(ctx, v, "join."+a.Outcome)
	case "flood_wait":
		sec := int64(0)
		if a.RetryAfter != nil {
			sec = *a.RetryAfter
		}
		switch {
		case sec <= 0:
			return s.textFor(ctx, v, "join.flood_wait")
		case sec < 120:
			return s.textFor(ctx, v, "join.flood_wait_seconds", "n", strconv.FormatInt(sec, 10))
		case sec < 7200:
			return s.textFor(ctx, v, "join.flood_wait_minutes", "n", strconv.FormatInt((sec+59)/60, 10))
		}
		return s.textFor(ctx, v, "join.flood_wait_hours", "n", strconv.FormatInt((sec+3599)/3600, 10))
	}
	m := a.Message
	if m == "" {
		m = s.textFor(ctx, v, "join.unknown_error")
	}
	return s.textFor(ctx, v, "join.refused", "message", m)
}

// daemonFailure answers a daemon call that gave no outcome: 400 when
// the daemon refused the reference, 502 daemon otherwise, with what to
// do. joining adds that a join that timed out may have happened.
func (s *Server) daemonFailure(w http.ResponseWriter, r *http.Request, v scope.Viewer,
	rep daemon.Reply, joining bool) {
	var e struct {
		Message string `json:"message"`
	}
	if rep.Body != nil {
		_ = json.Unmarshal(rep.Body, &e)
	}
	key, kv := "", []string{}
	switch {
	case !s.daemon.Enabled():
		key = "daemon.off"
	case !rep.Sent && joining:
		key = "daemon.no_answer_join"
	case !rep.Sent:
		key = "daemon.unreachable"
	case rep.Status == http.StatusBadRequest:
		if e.Message == "" {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "join.daemon_refused")
		} else {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "join.not_accepted",
				"message", e.Message)
		}
		return
	case rep.Status == http.StatusUnauthorized:
		key = "daemon.bad_key"
	case rep.Status == http.StatusServiceUnavailable:
		key, kv = "daemon.not_ready", []string{"message", e.Message}
	case rep.Status == http.StatusGatewayTimeout && joining:
		key = "daemon.timeout_join"
	case rep.Status == http.StatusGatewayTimeout:
		key = "daemon.timeout"
	default:
		key, kv = "daemon.http", []string{"status", strconv.Itoa(rep.Status), "message", e.Message}
	}
	s.log.Warn("daemon call failed", "err", rep.Error)
	s.refuse(w, r, v, http.StatusBadGateway, "daemon", key, kv...)
}

// joinRef reads the body's reference: trimmed and bounded; the daemon
// does the real parsing. It answers the request itself when the body
// does not pass.
func (s *Server) joinRef(w http.ResponseWriter, r *http.Request, v scope.Viewer, fedIDs *[]uint64) (string, bool) {
	var in struct {
		Ref         *string   `json:"ref"`
		Federations *[]uint64 `json:"federation_ids"`
	}
	if !decodeBody(w, r, &in) {
		return "", false
	}
	ref := ""
	if in.Ref != nil {
		ref = strings.Trim(*in.Ref, " \t\r\n")
	}
	if ref == "" {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "join.empty")
		return "", false
	}
	if len(ref) > maxJoinRef {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "join.too_long")
		return "", false
	}
	if fedIDs != nil && in.Federations != nil {
		*fedIDs = *in.Federations
	}
	return ref, true
}

func (s *Server) joinReply(ctx context.Context, w http.ResponseWriter, v scope.Viewer, a joinAnswer) {
	a.Message = s.outcomeText(ctx, v, a)
	if a.Chat != nil && a.Chat.Username != "" {
		a.Chat.Username = strings.TrimPrefix(a.Chat.Username, "@")
	}
	writeJSON(w, http.StatusOK, a)
}

// previewJoin answers POST /api/join-chat/preview. A preview changes
// nothing and is not audited; it is behind CSRF all the same, since
// each one costs a rate-limited Telegram lookup.
func (s *Server) previewJoin(w http.ResponseWriter, r *http.Request) {
	_, v, ok := s.adminWrite(w, r, scope.JoinChat)
	if !ok {
		return
	}
	ref, ok := s.joinRef(w, r, v, nil)
	if !ok {
		return
	}
	rep := s.daemon.ResolveChat(r.Context(), ref)
	a, ok := daemonJoin(rep)
	if !ok {
		s.daemonFailure(w, r, v, rep, false)
		return
	}
	// A preview that finds the account in a chat it asked to join
	// files the group where that request said.
	if a.Outcome == "ok" && a.Chat != nil && a.Chat.IsMember {
		s.settleHeld(r, a)
	}
	s.joinReply(r.Context(), w, v, a)
}

// settleHeld files a group the account is now a member of into the
// federations held for it (by its ref or its id). A failure is logged:
// the sweep or the next look settles it.
func (s *Server) settleHeld(r *http.Request, a joinAnswer) {
	g := a.joinedGroup()
	if g == 0 {
		return
	}
	if _, err := webdb.SettleJoinFilings(r.Context(), s.app, nil, a.Ref, g, clientIP(r)); err != nil {
		s.log.Error("settle join filings", "chat", g, "err", err)
	}
}

// fileJoin files the group a join answered about, as its outcome
// says: into feds once joined; held for feds while the chat approves
// the request; nowhere otherwise, also when the account was a member
// already, since moving a group the logger has is the federation
// editor's. It sets the answer's federation_ids and filing_pending.
func (s *Server) fileJoin(r *http.Request, v scope.Viewer, a *joinAnswer, feds []uint64) error {
	ctx := r.Context()
	ids, pending := []uint64{}, false
	switch a.Outcome {
	case "joined":
		if g := a.joinedGroup(); g != 0 {
			var err error
			if ids, err = webdb.FileJoinedGroup(ctx, s.app, v.ID, feds, g, clientIP(r)); err != nil {
				return err
			}
		}
		s.settleHeld(r, *a)
	case "already_member":
		s.settleHeld(r, *a)
	case "request_sent", "approval_required":
		if a.Chat == nil || a.Chat.ChatID > 0 {
			break
		}
		// The group's counter now, so the sweep can tell when the
		// account starts receiving its messages.
		var count uint64
		if g := a.joinedGroup(); g != 0 {
			counts, err := logdb.GroupMsgCounts(ctx, s.ro, []int64{g})
			if err != nil {
				return err
			}
			count = counts[g]
		}
		var err error
		if ids, err = webdb.HoldJoinFilings(ctx, s.app, v.ID, feds, a.Ref, a.Chat.ChatID,
			a.Chat.Title, count, clientIP(r)); err != nil {
			return err
		}
		pending = len(ids) > 0
	}
	a.FederationIDs, a.FilingPending = &ids, &pending
	return nil
}

// joinChat answers POST /api/join-chat/join. Every attempt that passes
// the checks writes one chat_join row, whatever came of it, worded as
// the Drogon page words it, so the page's list reads both.
func (s *Server) joinChat(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.JoinChat)
	if !ok {
		return
	}
	var feds []uint64
	ref, ok := s.joinRef(w, r, v, &feds)
	if !ok {
		return
	}
	// The chosen federations must exist and be the viewer's
	// (fileFederations, from Scope.AdministersFederation), checked
	// before anything is joined; fileJoin files the group into them.
	for _, f := range feds {
		ok, err := s.filesInto(r.Context(), sc, f)
		if err != nil {
			s.internalError(w, "join federation", err)
			return
		}
		if !ok {
			notFound(w)
			return
		}
	}
	rep := s.daemon.JoinChat(r.Context(), ref)
	a, answered := daemonJoin(rep)
	var detail string
	if answered {
		detail = a.Outcome + " " + a.Ref
		if a.Chat != nil && a.Chat.Title != "" {
			detail += " \"" + a.Chat.Title + "\""
		}
		if a.RetryAfter != nil {
			detail += " retry_after=" + strconv.FormatInt(*a.RetryAfter, 10)
		}
		// The chat's id, last, so the page can leave out the
		// attempts at chats a viewer's scope hides.
		if a.Chat != nil && a.Chat.ChatID != 0 {
			detail += " chat=" + strconv.FormatInt(a.Chat.ChatID, 10)
		}
	} else {
		switch {
		case !s.daemon.Enabled():
			detail = "api_off"
		case !rep.Sent:
			detail = "no_answer"
		default:
			detail = "http_" + strconv.Itoa(rep.Status)
		}
		detail += " " + ref
	}
	s.auditAdmin(r, v.ID, "chat_join", detail)
	if !answered {
		s.daemonFailure(w, r, v, rep, true)
		return
	}
	if err := s.fileJoin(r, v, &a, feds); err != nil {
		// The chat is joined; only the filing failed, and the
		// answer must still say what Telegram did.
		s.log.Error("file joined group", "ref", a.Ref, "err", err)
		none, no := []uint64{}, false
		a.FederationIDs, a.FilingPending = &none, &no
	}
	s.joinReply(r.Context(), w, v, a)
}

// cancelJoinFiling answers DELETE /api/join-chat/pending/{id}: an
// admin drops its own held filings, a superadmin any.
func (s *Server) cancelJoinFiling(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.JoinChat)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return
	}
	var by *uint64
	if sc.Role() != scope.Superadmin {
		by = &v.ID
	}
	done, err := webdb.CancelJoinFiling(r.Context(), s.app, v.ID, id, by, clientIP(r))
	if err != nil {
		s.internalError(w, "cancel join filing", err)
		return
	}
	if !done {
		notFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// joinAttempt is one row of the page's list of recent attempts.
type joinAttempt struct {
	At      string `json:"at"`
	By      string `json:"by"`
	Ref     string `json:"ref"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
	// chatID is the chat the attempt named, 0 when the row does not
	// say (the Drogon page's rows, and chats not resolved).
	chatID int64
}

// parseJoinDetail reads a chat_join row's detail, "<outcome> <ref>",
// then the chat's title in quotes, the flood wait and the chat's id
// when known.
func parseJoinDetail(e webdb.AuditEntry) joinAttempt {
	j := joinAttempt{At: e.At.Format("2006-01-02T15:04:05Z"), By: e.By}
	outcome, rest, _ := strings.Cut(e.Detail, " ")
	j.Outcome = outcome
	if i := strings.LastIndex(rest, " chat="); i >= 0 {
		if id, err := strconv.ParseInt(rest[i+len(" chat="):], 10, 64); err == nil {
			j.chatID = id
			rest = rest[:i]
		}
	}
	if i := strings.LastIndex(rest, " retry_after="); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.Index(rest, " \""); i >= 0 && strings.HasSuffix(rest, "\"") && len(rest) > i+2 {
		j.Title = rest[i+2 : len(rest)-1]
		rest = rest[:i]
	}
	j.Ref = rest
	return j
}

// joinFilingJSON is JoinFiling.
type joinFilingJSON struct {
	ID         uint64 `json:"id"`
	Ref        string `json:"ref"`
	Title      string `json:"title"`
	ChatID     int64  `json:"chat_id"`
	Federation fedRef `json:"federation"`
	By         string `json:"by"`
	At         string `json:"at"`
}

type joinChatProps struct {
	Federations []fedRef         `json:"federations"`
	Recent      []joinAttempt    `json:"recent"`
	Pending     []joinFilingJSON `json:"pending"`
	// APIConfigured is whether this service reaches the daemon's API;
	// without it no chat can be joined, and the page says so up front.
	APIConfigured bool `json:"api_configured"`
}

func loadJoinChat(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "join_chat"); err != nil {
		return nil, err
	}
	s := pr.s
	// A superadmin sees every account's attempts; an admin granted
	// join_chat only its own, and none at a chat its scope hides. A
	// row that does not name its chat cannot be checked, so it is
	// shown to an admin without the title.
	all := pr.scope.Role() == scope.Superadmin
	var by *uint64
	if !all {
		v, _ := pr.scope.Viewer()
		by = &v.ID
	}
	rows, err := webdb.AuditOfBy(ctx, s.app, "chat_join", by, joinRecent)
	if err != nil {
		return nil, err
	}
	feds, err := s.fileFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	held, err := webdb.ListJoinFilings(ctx, s.app, by)
	if err != nil {
		return nil, err
	}
	p := joinChatProps{Federations: feds, Recent: []joinAttempt{}, Pending: []joinFilingJSON{},
		APIConfigured: s.daemon != nil}
	for _, j := range held {
		p.Pending = append(p.Pending, joinFilingJSON{ID: j.ID, Ref: j.Ref, Title: j.Title,
			ChatID: j.ChatID, Federation: fedRef{j.FedID, j.FedName}, By: j.ByName,
			At: j.At.Format("2006-01-02T15:04:05Z")})
	}
	for _, e := range rows {
		j := parseJoinDetail(e)
		if !all {
			if j.chatID != 0 && !pr.scope.SeesChat(j.chatID) {
				continue
			}
			if j.chatID == 0 {
				j.Title = ""
			}
		}
		p.Recent = append(p.Recent, j)
	}
	return p, nil
}

// joinFilingSweep is how often JoinFiler looks at the held filings.
const joinFilingSweep = time.Minute

// JoinFiler settles held filings in the background: a group whose
// join waited for approval is filed once the logger counts messages of
// it beyond those it had at the request, which it only receives as a
// member. Filings of chats unknown at the request (a private group
// behind an invite link) settle when the chat is previewed or joined
// again instead. Each pass reads the held filings with one query and
// stops there when none has a known chat.
type JoinFiler struct {
	log     *slog.Logger
	ro, app *sql.DB
	every   time.Duration
}

// NewJoinFiler makes the sweep; Run starts it.
func NewJoinFiler(log *slog.Logger, ro, app *sql.DB) *JoinFiler {
	return &JoinFiler{log: log, ro: ro, app: app, every: joinFilingSweep}
}

// Run sweeps every f.every until ctx ends.
func (f *JoinFiler) Run(ctx context.Context) {
	t := time.NewTicker(f.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := f.pass(ctx); err != nil && ctx.Err() == nil {
				f.log.Error("join filings", "err", err)
			}
		}
	}
}

// pass settles the held filings whose group now has more messages.
func (f *JoinFiler) pass(ctx context.Context) error {
	held, err := webdb.KnownJoinFilings(ctx, f.app)
	if err != nil || len(held) == 0 {
		return err
	}
	var groups []int64
	for _, j := range held {
		groups = append(groups, j.ChatID)
	}
	counts, err := logdb.GroupMsgCounts(ctx, f.ro, groups)
	if err != nil {
		return err
	}
	due := map[int64][]uint64{}
	for _, j := range held {
		if n, ok := counts[j.ChatID]; ok && n > j.MsgCount {
			due[j.ChatID] = append(due[j.ChatID], j.ID)
		}
	}
	for g, ids := range due {
		n, err := webdb.SettleJoinFilings(ctx, f.app, ids, "", g, "")
		if err != nil {
			return err
		}
		f.log.Info("filed an approved join", "chat", g, "federations", n)
	}
	return nil
}

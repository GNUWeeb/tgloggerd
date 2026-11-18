// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// The bots of Discord to Telegram forwarding (web/docs/platform-
// forwarding.md, Bots; decisions/2026-11-18-ownership-and-bots.md):
// Telegram bots that send into Telegram chats and Discord bots that
// read Discord channels, each owned by an account. They need the
// forward ability; a member with it sees and uses only its own bots
// (webdb.OwnedBy), superadmins and admins every bot, with its owner. A
// bot the viewer may not see does not exist for it (404).
//
// A token is write-only: it is checked in shape, stored, and never
// answered, logged or audited. Whether it works is discordd's to find
// out; the status says what it found.

// botIdentity is who the bot is on its platform once logged in
// (BotIdentity).
type botIdentity struct {
	UserID   string  `json:"user_id"`
	Username *string `json:"username"`
}

// botRef is a bot as pickers and routes show it (BotRef).
type botRef struct {
	ID       uint64       `json:"id"`
	Kind     string       `json:"kind"`
	Name     string       `json:"name"`
	Identity *botIdentity `json:"identity"`
	Status   string       `json:"status"`
	Enabled  bool         `json:"enabled"`
}

// botJSON is a bot on the bots page (Bot).
type botJSON struct {
	botRef
	Owner              fwdPerson   `json:"owner"`
	TransferCandidates []fwdPerson `json:"transfer_candidates"`
	StatusAt           *string     `json:"status_at"`
	StatusDetail       *string     `json:"status_detail"`
	Routes             int         `json:"routes"`
}

func botResource(id uint64) webdb.Resource {
	t := webdb.ResTelegramBot
	if logdb.KindOf(id) == logdb.KindDiscord {
		t = webdb.ResDiscordBot
	}
	return webdb.Resource{Type: t, ID: int64(id)}
}

// botStatus is the status answered: a bot switched off is disabled,
// unless discordd switched it off for a refusal, which says why.
func botStatus(b logdb.BotRow) string {
	switch {
	case !b.Enabled && b.Status != "token_refused" && b.Status != "intent_missing":
		return "disabled"
	case b.Enabled && b.Status == "disabled":
		return "connecting"
	}
	return b.Status
}

func (s *Server) botRefOf(ctx context.Context, v scope.Viewer, b logdb.BotRow) botRef {
	name := b.Name
	switch {
	case name != "":
	case b.Username != "":
		name = "@" + b.Username
	default:
		name = s.textFor(ctx, v, "bots.unnamed", "id", strconv.FormatUint(b.ID, 10))
	}
	ref := botRef{ID: b.ID, Kind: b.Kind, Name: name, Status: botStatus(b), Enabled: b.Enabled}
	if b.UserID != "" {
		ref.Identity = &botIdentity{UserID: b.UserID}
		if b.Username != "" {
			u := b.Username
			ref.Identity.Username = &u
		}
	}
	return ref
}

// botAccess is which bots a viewer may see and use: every one, or
// those it owns.
type botAccess struct {
	all   bool
	owned map[webdb.Resource]bool
}

func (a botAccess) sees(id uint64) bool {
	return a.all || a.owned[botResource(id)]
}

func (s *Server) botAccessOf(ctx context.Context, sc scope.Scope) (botAccess, error) {
	if sc.SeesAllBots() {
		return botAccess{all: true}, nil
	}
	a := botAccess{owned: map[webdb.Resource]bool{}}
	v, ok := sc.Viewer()
	if !ok || !sc.Can(scope.Forward) {
		return a, nil
	}
	for _, t := range []webdb.ResourceType{webdb.ResTelegramBot, webdb.ResDiscordBot} {
		ids, err := webdb.OwnedBy(ctx, s.app, t, v.ID)
		if err != nil {
			return a, err
		}
		for _, id := range ids {
			a.owned[webdb.Resource{Type: t, ID: id}] = true
		}
	}
	return a, nil
}

// visibleBots is the bots of kind ("" both) the viewer may see.
func (s *Server) visibleBots(ctx context.Context, sc scope.Scope, kind string) ([]logdb.BotRow, error) {
	acc, err := s.botAccessOf(ctx, sc)
	if err != nil {
		return nil, err
	}
	all, err := logdb.ListBots(ctx, s.ro, kind)
	if err != nil {
		return nil, err
	}
	out := []logdb.BotRow{}
	for _, b := range all {
		if acc.sees(b.ID) {
			out = append(out, b)
		}
	}
	return out, nil
}

// usableBot is a bot of kind the viewer may use in a route: one it
// sees; any other id answers 404, one of the other kind 400.
func (s *Server) usableBot(w http.ResponseWriter, r *http.Request, sc scope.Scope, v scope.Viewer,
	id uint64, kind string) (logdb.BotRow, bool) {
	b, ok := s.loadVisibleBot(w, r, sc, id)
	if !ok {
		return b, false
	}
	if b.Kind != kind {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.bot_kind", "kind", kind)
		return b, false
	}
	return b, true
}

func (s *Server) loadVisibleBot(w http.ResponseWriter, r *http.Request, sc scope.Scope,
	id uint64) (logdb.BotRow, bool) {
	acc, err := s.botAccessOf(r.Context(), sc)
	if err != nil {
		s.internalError(w, "bot access", err)
		return logdb.BotRow{}, false
	}
	if !acc.sees(id) {
		notFound(w)
		return logdb.BotRow{}, false
	}
	b, ok, err := logdb.GetBot(r.Context(), s.ro, id)
	if err != nil {
		s.internalError(w, "bot", err)
		return b, false
	}
	if !ok {
		notFound(w)
	}
	return b, ok
}

// botOf loads the bot in the path, answering 404 itself for one that
// does not exist or that the viewer may not see.
func (s *Server) botOf(w http.ResponseWriter, r *http.Request, sc scope.Scope) (logdb.BotRow, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return logdb.BotRow{}, false
	}
	return s.loadVisibleBot(w, r, sc, id)
}

// transferCandidates is who the viewer may give a bot to, beside its
// owner: for a superadmin any active account, for an admin the active
// admins and superadmins and the members it manages. A member
// transfers nothing.
func (s *Server) transferCandidates(ctx context.Context, sc scope.Scope) ([]webdb.Account, error) {
	if !sc.SeesAllBots() {
		return nil, nil
	}
	all, err := webdb.ListAccounts(ctx, s.app)
	if err != nil {
		return nil, err
	}
	var out []webdb.Account
	for _, a := range all {
		if !a.Active {
			continue
		}
		if sc.Role() == scope.Superadmin || a.Role != string(scope.Member) ||
			sc.SeesAccount(scopeAccount(a)) {
			out = append(out, a)
		}
	}
	return out, nil
}

func personOf(a webdb.Account) fwdPerson {
	name := a.DisplayName
	if name == "" {
		name = a.Username
	}
	return fwdPerson{ID: a.ID, Name: name, Role: a.Role, Active: a.Active}
}

// botsJSON answers bots as the page shows them, with their owners.
func (s *Server) botsJSON(ctx context.Context, sc scope.Scope, v scope.Viewer,
	bots []logdb.BotRow) ([]botJSON, error) {
	owners := map[webdb.Resource]webdb.OwnerInfo{}
	for _, t := range []webdb.ResourceType{webdb.ResTelegramBot, webdb.ResDiscordBot} {
		var ids []int64
		for _, b := range bots {
			if botResource(b.ID).Type == t {
				ids = append(ids, int64(b.ID))
			}
		}
		m, err := webdb.Owners(ctx, s.app, t, ids)
		if err != nil {
			return nil, err
		}
		for id, o := range m {
			owners[webdb.Resource{Type: t, ID: id}] = o
		}
	}
	cands, err := s.transferCandidates(ctx, sc)
	if err != nil {
		return nil, err
	}
	out := []botJSON{}
	for _, b := range bots {
		j := botJSON{botRef: s.botRefOf(ctx, v, b), TransferCandidates: []fwdPerson{},
			Routes: b.Routes,
			Owner:  fwdPerson{ID: 0, Name: s.textFor(ctx, v, "fwd.shared"), Active: true}}
		if o, ok := owners[botResource(b.ID)]; ok {
			j.Owner = fwdPerson{ID: o.ID, Name: o.Name(), Role: o.Role, Active: o.Active}
		}
		for _, a := range cands {
			if a.ID != j.Owner.ID {
				j.TransferCandidates = append(j.TransferCandidates, personOf(a))
			}
		}
		if b.StatusAt.Valid {
			j.StatusAt = isoPtr(b.StatusAt.Int64)
		}
		if b.StatusDetail.Valid && b.StatusDetail.String != "" {
			d := b.StatusDetail.String
			j.StatusDetail = &d
		}
		out = append(out, j)
	}
	return out, nil
}

// botAnswer answers one bot as it is now.
func (s *Server) botAnswer(w http.ResponseWriter, r *http.Request, sc scope.Scope, v scope.Viewer,
	status int, id uint64) {
	b, ok, err := logdb.GetBot(r.Context(), s.ro, id)
	if err != nil || !ok {
		s.internalError(w, "bot answer", fmt.Errorf("bot %d: %v", id, err))
		return
	}
	js, err := s.botsJSON(r.Context(), sc, v, []logdb.BotRow{b})
	if err != nil {
		s.internalError(w, "bot answer", err)
		return
	}
	writeJSON(w, status, js[0])
}

// validBotName is a name of 1 to MaxBotName characters, trimmed.
func validBotName(n string) (string, bool) {
	n = strings.TrimSpace(n)
	return n, n != "" && utf8.ValidString(n) && utf8.RuneCountInString(n) <= logdb.MaxBotName
}

// checkBotToken checks a token of kind, answering 400 bad_token itself.
func (s *Server) checkBotToken(w http.ResponseWriter, r *http.Request, v scope.Viewer,
	kind, token string) bool {
	if kind == logdb.KindDiscord && logdb.ValidDiscordToken(token) ||
		kind == logdb.KindTelegram && logdb.ValidTelegramToken(token) {
		return true
	}
	s.refuse(w, r, v, http.StatusBadRequest, "bad_token", "bots.bad_token_"+kind)
	return false
}

// listBots answers GET /api/bots?kind=: the bots the viewer may use.
func (s *Server) listBots(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := adminActor(w, r, scope.Forward)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != logdb.KindTelegram && kind != logdb.KindDiscord {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_kind")
		return
	}
	bots, err := s.visibleBots(r.Context(), sc, kind)
	if err != nil {
		s.internalError(w, "bots", err)
		return
	}
	refs := []botRef{}
	for _, b := range bots {
		refs = append(refs, s.botRefOf(r.Context(), v, b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"bots": refs})
}

// createBot answers POST /api/bots.
func (s *Server) createBot(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	var in struct {
		Kind  *string `json:"kind"`
		Name  *string `json:"name"`
		Token *string `json:"token"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Kind == nil || in.Name == nil || in.Token == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.required")
		return
	}
	kind := *in.Kind
	if kind != logdb.KindTelegram && kind != logdb.KindDiscord {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_kind")
		return
	}
	name, ok := validBotName(*in.Name)
	if !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_name",
			"max", strconv.Itoa(logdb.MaxBotName))
		return
	}
	if !s.checkBotToken(w, r, v, kind, *in.Token) {
		return
	}
	ctx := r.Context()
	// A token stored already: the viewer's own bot is answered; one of
	// another account is refused without saying whose.
	if id, found, err := logdb.BotByToken(ctx, s.ro, kind, *in.Token); err != nil {
		s.internalError(w, "bot by token", err)
		return
	} else if found {
		o, owned, err := webdb.OwnerOf(ctx, s.app, botResource(id))
		if err != nil {
			s.internalError(w, "bot owner", err)
			return
		}
		if owned && o.ID == v.ID || !owned && sc.Role() == scope.Superadmin {
			s.botAnswer(w, r, sc, v, http.StatusCreated, id)
			return
		}
		s.refuse(w, r, v, http.StatusConflict, "token_in_use", "bots.token_in_use")
		return
	}
	id, err := logdb.CreateBot(ctx, s.ro, kind, name, *in.Token)
	if errors.Is(err, logdb.ErrTokenInUse) {
		s.refuse(w, r, v, http.StatusConflict, "token_in_use", "bots.token_in_use")
		return
	}
	if err != nil {
		s.internalError(w, "create bot", err)
		return
	}
	s.auditAdmin(r, v.ID, "bot_create", fmt.Sprintf("bot %d kind %s name %q", id, kind, name))
	s.recordOwner(r, botResource(id), v.ID)
	s.botAnswer(w, r, sc, v, http.StatusCreated, id)
}

// updateBot answers PATCH /api/bots/{id}.
func (s *Server) updateBot(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	b, ok := s.botOf(w, r, sc)
	if !ok {
		return
	}
	var in struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
		Token   *string `json:"token"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var c logdb.BotChange
	var changed []string
	if in.Name != nil {
		name, ok := validBotName(*in.Name)
		if !ok {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_name",
				"max", strconv.Itoa(logdb.MaxBotName))
			return
		}
		c.Name = &name
		changed = append(changed, fmt.Sprintf("name %q", name))
	}
	if in.Token != nil {
		if !s.checkBotToken(w, r, v, b.Kind, *in.Token) {
			return
		}
		c.Token = in.Token
		changed = append(changed, "token")
	}
	if in.Enabled != nil {
		c.Enabled = in.Enabled
		changed = append(changed, "enabled="+strconv.FormatBool(*in.Enabled))
	}
	err := logdb.UpdateBot(r.Context(), s.ro, b.ID, c)
	if errors.Is(err, logdb.ErrTokenInUse) {
		s.refuse(w, r, v, http.StatusConflict, "token_in_use", "bots.token_in_use")
		return
	}
	if err != nil {
		s.internalError(w, "update bot", err)
		return
	}
	if len(changed) > 0 {
		s.auditAdmin(r, v.ID, "bot_update", fmt.Sprintf("bot %d %s", b.ID, strings.Join(changed, " ")))
	}
	s.botAnswer(w, r, sc, v, http.StatusOK, b.ID)
}

// deleteBot answers DELETE /api/bots/{id}. A bot routes use is refused
// (409 bot_in_use): they would be left without it.
func (s *Server) deleteBot(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	b, ok := s.botOf(w, r, sc)
	if !ok {
		return
	}
	if b.Routes > 0 {
		s.refuse(w, r, v, http.StatusConflict, "bot_in_use", "bots.in_use",
			"count", strconv.Itoa(b.Routes))
		return
	}
	if err := logdb.DeleteBot(r.Context(), s.ro, b.ID); err != nil {
		s.internalError(w, "delete bot", err)
		return
	}
	s.auditAdmin(r, v.ID, "bot_delete", fmt.Sprintf("bot %d kind %s", b.ID, b.Kind))
	s.dropOwner(r, botResource(b.ID), v.ID)
	w.WriteHeader(http.StatusNoContent)
}

// transferBot answers POST /api/bots/{id}/transfer: admins and
// superadmins give a bot to one of its transfer candidates. To a
// member it does not exist. Routes using the bot keep working; giving
// a route to a member needs its bots to be the member's.
func (s *Server) transferBot(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	if !sc.SeesAllBots() {
		notFound(w)
		return
	}
	b, ok := s.botOf(w, r, sc)
	if !ok {
		return
	}
	var in struct {
		OwnerID *uint64 `json:"owner_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.OwnerID == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_owner")
		return
	}
	ctx := r.Context()
	cands, err := s.transferCandidates(ctx, sc)
	if err != nil {
		s.internalError(w, "transfer candidates", err)
		return
	}
	found := false
	for _, a := range cands {
		found = found || a.ID == *in.OwnerID
	}
	if !found {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "bots.bad_owner")
		return
	}
	res := botResource(b.ID)
	by := webdb.By(v.ID, clientIP(r))
	from, err := webdb.TransferOwner(ctx, s.app, res, *in.OwnerID, by)
	if errors.Is(err, webdb.ErrNotOwned) {
		// It counted as the superadmins': it gets its first owner.
		err = webdb.SetOwner(ctx, s.app, res, *in.OwnerID, by)
	}
	if err != nil {
		s.internalError(w, "transfer bot", err)
		return
	}
	s.auditAdmin(r, v.ID, "bot_transfer", fmt.Sprintf("bot %d from %d to %d", b.ID, from, *in.OwnerID))
	w.WriteHeader(http.StatusNoContent)
}

// botsProps is the bots page's data (BotsProps).
type botsProps struct {
	Bots      []botJSON `json:"bots"`
	ShowOwner bool      `json:"show_owner"`
}

func loadBots(ctx context.Context, pr *pageRequest) (any, error) {
	v, err := pageOf(pr, "platform_fwd")
	if err != nil {
		return nil, err
	}
	s := pr.s
	bots, err := s.visibleBots(ctx, pr.scope, "")
	if err != nil {
		return nil, err
	}
	js, err := s.botsJSON(ctx, pr.scope, v, bots)
	if err != nil {
		return nil, err
	}
	return botsProps{Bots: js, ShowOwner: pr.scope.SeesAllBots()}, nil
}

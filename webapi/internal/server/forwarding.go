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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/webdb"
)

// Platform forwarding (web/docs/platform-forwarding.md), the Drogon
// TelegramDiscordController and DiscordTelegramController as JSON
// operations: Telegram to Discord webhooks and Discord to Telegram
// routes, created, changed, deleted and, for a webhook, tested. They
// need the forward ability (Scope.Can); a configuration whose Telegram
// chat the scope hides does not exist for the request.
//
// Each configuration is owned by the account that created it
// (ownership.go), as is each bot (bots.go); one without an owner, made before
// ownership existed or by the Drogon pages, counts as the superadmins'
// and is answered as shared. The federation of a configuration is not
// stored yet: every one is in Default, and transferring is not served
// (forwarding-per-user). An admin or superadmin with the ability sees
// every configuration of the chats it sees; a member with forward sees
// and changes only those it owns (web-accounts.md section 6): any other
// does not exist for it (fwdVisible).
//
// The webhook URL is write-only: an answer carries it with its secret
// part cut off. A webhook URL must be Discord's, on save and on test,
// so the service cannot be made to post to another host. A Discord to
// Telegram route names its two bots by id (bots.go); no token is ever
// typed on a route.

// webhookMax is the column's length.
const webhookMax = 512

// parseWebhook checks a Discord webhook URL as the Drogon parseWebhook
// does: https, one of Discord's hosts, a /api/webhooks/<id>/<token>
// path. It returns the host and the path with its query.
func parseWebhook(u string) (host, pathq string, ok bool) {
	const pfx = "https://"
	if len(u) > webhookMax || !strings.HasPrefix(u, pfx) {
		return "", "", false
	}
	slash := strings.IndexByte(u[len(pfx):], '/')
	if slash < 0 {
		return "", "", false
	}
	host, pathq = u[len(pfx):len(pfx)+slash], u[len(pfx)+slash:]
	switch host {
	case "discord.com", "discordapp.com", "ptb.discord.com", "canary.discord.com":
	default:
		return "", "", false
	}
	const hooks = "/api/webhooks/"
	if !strings.HasPrefix(pathq, hooks) {
		return "", "", false
	}
	rest := pathq[len(hooks):]
	idEnd := strings.IndexByte(rest, '/')
	if idEnd <= 0 || idEnd+1 >= len(rest) {
		return "", "", false
	}
	return host, pathq, true
}

// maskWebhook is a webhook URL without its token: the host and the
// webhook's id, which identify it to an admin and grant nothing.
func maskWebhook(u string) string {
	host, pathq, ok := parseWebhook(u)
	if !ok {
		return "****"
	}
	rest := strings.TrimPrefix(pathq, "/api/webhooks/")
	id := rest[:strings.IndexByte(rest, '/')]
	return "https://" + host + "/api/webhooks/" + id + "/****"
}

// parseSnowflake reads a Discord id: 1 to 20 digits, not 0.
func parseSnowflake(s string) (uint64, bool) {
	if s == "" || len(s) > 20 || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil && n != 0
}

// fwdPerson is an account as the forwarding pages show one (FwdPerson).
type fwdPerson struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role,omitempty"`
	Active bool   `json:"active"`
}

// owned is what every configuration carries about its ownership
// (Owned).
type owned struct {
	ID                 uint64      `json:"id"`
	Owner              fwdPerson   `json:"owner"`
	Federation         fedRef      `json:"federation"`
	TransferCandidates []fwdPerson `json:"transfer_candidates"`
}

// fwdOwners is the owners of a list's configurations of type t, read
// in one query.
func (s *Server) fwdOwners(ctx context.Context, t webdb.ResourceType, ids []uint64) (map[int64]webdb.OwnerInfo, error) {
	keys := make([]int64, len(ids))
	for i, id := range ids {
		keys[i] = int64(id)
	}
	return webdb.Owners(ctx, s.app, t, keys)
}

// ownership is a configuration's owner and federation: its owner from
// owners, or shared (id 0, active) when it has none. The federation is
// Default and there is nobody to transfer to until forwarding-per-user
// stores federations and serves transfers.
func (s *Server) ownership(ctx context.Context, v scope.Viewer, id uint64,
	owners map[int64]webdb.OwnerInfo) owned {
	owner := fwdPerson{ID: 0, Name: s.textFor(ctx, v, "fwd.shared"), Active: true}
	if o, ok := owners[int64(id)]; ok {
		owner = fwdPerson{ID: o.ID, Name: o.Name(), Role: o.Role, Active: o.Active}
	}
	return owned{
		ID:                 id,
		Owner:              owner,
		Federation:         fedRef{scope.DefaultFederation, s.textFor(ctx, v, "federation.default")},
		TransferCandidates: []fwdPerson{},
	}
}

// fwdPick is a configuration's chat as the pickers show it, with its
// photo when stored. Every caller has the chat from a query filtered
// by the request's scope, so the viewer may see the chat and its
// photo.
func (s *Server) fwdPick(c logdb.FwdChat) pickItem {
	p := pickItem{ID: c.ID, Title: c.Title, Kind: c.Kind, Photo: s.storedPhoto(c.Photo)}
	if c.Kind == "group" {
		p.Sub = groupTypeLabel(c.Type)
	}
	return p
}

// t2dRoute is a webhook as the page shows it (T2DRoute).
type t2dRoute struct {
	owned
	Chat      pickItem `json:"chat"`
	URLMasked string   `json:"url_masked"`
	Types     []string `json:"types"`
	Enabled   bool     `json:"enabled"`
	Sent      int64    `json:"sent"`
	LastSent  *string  `json:"last_sent"`
}

func (s *Server) t2dJSON(ctx context.Context, v scope.Viewer, h logdb.T2DWebhook,
	owners map[int64]webdb.OwnerInfo) t2dRoute {
	return t2dRoute{owned: s.ownership(ctx, v, h.ID, owners), Chat: s.fwdPick(h.Chat),
		URLMasked: maskWebhook(h.URL), Types: h.Types.Keys(logdb.T2DTypeCount),
		Enabled: h.Enabled, Sent: h.Sent, LastSent: isoPtr(h.LastSent)}
}

// d2tRoute is a route as the page shows it (D2TRoute): its bots with
// their state, so the page can tell a route whose bot is switched off
// or refused. DiscordBot is null for a route made before the Discord
// bots were stored, until the deployment's import gives it one.
type d2tRoute struct {
	owned
	ChannelID   string   `json:"channel_id"`
	ChannelName string   `json:"channel_name"`
	Chat        pickItem `json:"chat"`
	DiscordBot  *botRef  `json:"discord_bot"`
	TelegramBot botRef   `json:"telegram_bot"`
	Types       []string `json:"types"`
	Enabled     bool     `json:"enabled"`
	Sent        int64    `json:"sent"`
}

// botsByID is every bot by id, for the routes' answers.
func (s *Server) botsByID(ctx context.Context) (map[uint64]logdb.BotRow, error) {
	all, err := logdb.ListBots(ctx, s.ro, "")
	if err != nil {
		return nil, err
	}
	m := make(map[uint64]logdb.BotRow, len(all))
	for _, b := range all {
		m[b.ID] = b
	}
	return m, nil
}

func (s *Server) d2tJSON(ctx context.Context, v scope.Viewer, d logdb.D2TRoute,
	owners map[int64]webdb.OwnerInfo, bots map[uint64]logdb.BotRow) d2tRoute {
	out := d2tRoute{owned: s.ownership(ctx, v, d.ID, owners),
		// A snowflake does not fit a JavaScript number exactly.
		ChannelID: strconv.FormatUint(d.ChannelID, 10), ChannelName: d.ChannelName,
		Chat:        s.fwdPick(d.Chat),
		TelegramBot: s.botRefOf(ctx, v, bots[d.TelegramBotID]),
		Types:       d.Types.Keys(logdb.D2TTypeCount), Enabled: d.Enabled, Sent: d.Sent}
	if b, ok := bots[d.DiscordBotID]; ok && d.DiscordBotID != 0 {
		ref := s.botRefOf(ctx, v, b)
		out.DiscordBot = &ref
	}
	return out
}

// d2tLive is whether a route forwards: it and both its bots enabled.
func d2tLive(d logdb.D2TRoute, bots map[uint64]logdb.BotRow) bool {
	return d.Enabled && bots[d.TelegramBotID].Enabled && d.DiscordBotID != 0 &&
		bots[d.DiscordBotID].Enabled
}

// fwdVisible is which configurations of type t the viewer may see and
// change, besides the scope's chat filter: every one for a viewer that
// sees all forwarding (scope.SeesAllForwarding), else those it owns.
func (s *Server) fwdVisible(ctx context.Context, sc scope.Scope, t webdb.ResourceType) (func(uint64) bool, error) {
	if sc.SeesAllForwarding() {
		return func(uint64) bool { return true }, nil
	}
	v, ok := sc.Viewer()
	if !ok {
		return func(uint64) bool { return false }, nil
	}
	ids, err := webdb.OwnedBy(ctx, s.app, t, v.ID)
	if err != nil {
		return nil, err
	}
	owned := map[uint64]bool{}
	for _, id := range ids {
		owned[uint64(id)] = true
	}
	return func(id uint64) bool { return owned[id] }, nil
}

// listT2D is the webhooks the viewer sees (logdb.ListT2D, fwdVisible).
func (s *Server) listT2D(ctx context.Context, sc scope.Scope) ([]logdb.T2DWebhook, error) {
	ws, err := logdb.ListT2D(ctx, s.ro, sc)
	if err != nil {
		return nil, err
	}
	vis, err := s.fwdVisible(ctx, sc, webdb.ResT2DWebhook)
	if err != nil {
		return nil, err
	}
	out := ws[:0]
	for _, w := range ws {
		if vis(w.ID) {
			out = append(out, w)
		}
	}
	return out, nil
}

// getT2D is one webhook the viewer sees.
func (s *Server) getT2D(ctx context.Context, sc scope.Scope, id uint64) (logdb.T2DWebhook, bool, error) {
	h, ok, err := logdb.GetT2D(ctx, s.ro, sc, id)
	if err != nil || !ok {
		return h, false, err
	}
	vis, err := s.fwdVisible(ctx, sc, webdb.ResT2DWebhook)
	if err != nil {
		return h, false, err
	}
	return h, vis(h.ID), nil
}

// listD2T is the routes the viewer sees (logdb.ListD2T, fwdVisible).
func (s *Server) listD2T(ctx context.Context, sc scope.Scope) ([]logdb.D2TRoute, error) {
	rs, err := logdb.ListD2T(ctx, s.ro, sc)
	if err != nil {
		return nil, err
	}
	vis, err := s.fwdVisible(ctx, sc, webdb.ResD2TRoute)
	if err != nil {
		return nil, err
	}
	out := rs[:0]
	for _, r := range rs {
		if vis(r.ID) {
			out = append(out, r)
		}
	}
	return out, nil
}

// getD2T is one route the viewer sees.
func (s *Server) getD2T(ctx context.Context, sc scope.Scope, id uint64) (logdb.D2TRoute, bool, error) {
	d, ok, err := logdb.GetD2T(ctx, s.ro, sc, id)
	if err != nil || !ok {
		return d, false, err
	}
	vis, err := s.fwdVisible(ctx, sc, webdb.ResD2TRoute)
	if err != nil {
		return d, false, err
	}
	return d, vis(d.ID), nil
}

// fwdTarget checks the chat and federation of a new configuration: the
// chat must be one the logger has seen and the scope sees, the
// federation one the viewer administers. Either missing answers 404.
func (s *Server) fwdTarget(w http.ResponseWriter, r *http.Request, sc scope.Scope,
	chat int64, fed *uint64) (logdb.FwdChat, bool) {
	if fed != nil {
		ok, err := s.filesInto(r.Context(), sc, *fed)
		if err != nil {
			s.internalError(w, "forwarding federation", err)
			return logdb.FwdChat{}, false
		}
		if !ok {
			notFound(w)
			return logdb.FwdChat{}, false
		}
	}
	c, ok, err := logdb.ResolveChat(r.Context(), s.ro, sc, chat)
	if err != nil {
		s.internalError(w, "forwarding chat", err)
		return c, false
	}
	if !ok {
		notFound(w)
	}
	return c, ok
}

// fwdTypes reads a types list of a direction with n types.
func (s *Server) fwdTypes(w http.ResponseWriter, r *http.Request, v scope.Viewer,
	keys []string, n int) (logdb.FwdTypes, bool) {
	t, ok := logdb.ParseFwdTypes(keys, n)
	if !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.bad_types",
			"types", strings.Join(logdb.FwdKeys[:n], ", "))
	}
	return t, ok
}

// createT2D answers POST /api/forwarding/telegram-discord.
func (s *Server) createT2D(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	var in struct {
		ChatID     *int64    `json:"chat_id"`
		ChatKind   *string   `json:"chat_kind"`
		WebhookURL *string   `json:"webhook_url"`
		Types      *[]string `json:"types"`
		Enabled    *bool     `json:"enabled"`
		Federation *uint64   `json:"federation_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.ChatID == nil || in.ChatKind == nil || in.WebhookURL == nil || in.Types == nil ||
		in.Enabled == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.t2d_required")
		return
	}
	if (*in.ChatKind != "group" || *in.ChatID >= 0) && (*in.ChatKind != "user" || *in.ChatID <= 0) {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.chat_kind")
		return
	}
	if _, _, ok := parseWebhook(*in.WebhookURL); !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_webhook", "fwd.bad_webhook")
		return
	}
	types, ok := s.fwdTypes(w, r, v, *in.Types, logdb.T2DTypeCount)
	if !ok {
		return
	}
	chat, ok := s.fwdTarget(w, r, sc, *in.ChatID, in.Federation)
	if !ok {
		return
	}
	id, err := logdb.CreateT2D(r.Context(), s.ro, chat.ID, *in.WebhookURL, *in.Enabled, types)
	if err != nil {
		s.internalError(w, "create webhook", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_t2d_create", fmt.Sprintf("webhook %d chat %d enabled=%t types=%s",
		id, chat.ID, *in.Enabled, strings.Join(types.Keys(logdb.T2DTypeCount), ",")))
	s.recordOwner(r, webdb.Resource{Type: webdb.ResT2DWebhook, ID: int64(id)}, v.ID)
	writeJSON(w, http.StatusCreated, map[string]uint64{"id": id})
}

// t2dOf loads the webhook in the path, answering 404 itself for one
// that does not exist or that the scope hides.
func (s *Server) t2dOf(w http.ResponseWriter, r *http.Request, sc scope.Scope) (logdb.T2DWebhook, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return logdb.T2DWebhook{}, false
	}
	h, ok, err := s.getT2D(r.Context(), sc, id)
	if err != nil {
		s.internalError(w, "webhook", err)
		return h, false
	}
	if !ok {
		notFound(w)
	}
	return h, ok
}

// updateT2D answers PATCH /api/forwarding/telegram-discord/{id}: the
// fields given change, the others stay; the chat stays.
func (s *Server) updateT2D(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	h, ok := s.t2dOf(w, r, sc)
	if !ok {
		return
	}
	var in struct {
		ChatID     *int64    `json:"chat_id"`
		ChatKind   *string   `json:"chat_kind"`
		Types      *[]string `json:"types"`
		Enabled    *bool     `json:"enabled"`
		WebhookURL *string   `json:"webhook_url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var changed []string
	if in.ChatID != nil || in.ChatKind != nil {
		// The chat moves with its kind, as on creation.
		if in.ChatID == nil || in.ChatKind == nil ||
			(*in.ChatKind != "group" || *in.ChatID >= 0) && (*in.ChatKind != "user" || *in.ChatID <= 0) {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.chat_kind")
			return
		}
		chat, ok := s.fwdTarget(w, r, sc, *in.ChatID, nil)
		if !ok {
			return
		}
		if chat.ID != h.Chat.ID {
			changed = append(changed, fmt.Sprintf("chat=%d", chat.ID))
		}
		h.Chat = chat
	}
	if in.WebhookURL != nil {
		if _, _, ok := parseWebhook(*in.WebhookURL); !ok {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_webhook", "fwd.bad_webhook")
			return
		}
		h.URL = *in.WebhookURL
		changed = append(changed, "url")
	}
	if in.Types != nil {
		t, ok := s.fwdTypes(w, r, v, *in.Types, logdb.T2DTypeCount)
		if !ok {
			return
		}
		h.Types = t
		changed = append(changed, "types="+strings.Join(t.Keys(logdb.T2DTypeCount), ","))
	}
	if in.Enabled != nil {
		h.Enabled = *in.Enabled
		changed = append(changed, "enabled="+strconv.FormatBool(h.Enabled))
	}
	if err := logdb.UpdateT2D(r.Context(), s.ro, h.ID, h.Chat.ID, h.URL, h.Enabled, h.Types); err != nil {
		s.internalError(w, "update webhook", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_t2d_update", fmt.Sprintf("webhook %d chat %d %s",
		h.ID, h.Chat.ID, strings.Join(changed, " ")))
	w.WriteHeader(http.StatusNoContent)
}

// deleteT2D answers DELETE /api/forwarding/telegram-discord/{id}.
func (s *Server) deleteT2D(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	h, ok := s.t2dOf(w, r, sc)
	if !ok {
		return
	}
	if err := logdb.DeleteT2D(r.Context(), s.ro, h.ID); err != nil {
		s.internalError(w, "delete webhook", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_t2d_delete", fmt.Sprintf("webhook %d chat %d", h.ID, h.Chat.ID))
	s.dropOwner(r, webdb.Resource{Type: webdb.ResT2DWebhook, ID: int64(h.ID)}, v.ID)
	w.WriteHeader(http.StatusNoContent)
}

// discordTimeout bounds a test message, as the Drogon page's.
const discordTimeout = 10 * time.Second

// testT2D answers POST /api/forwarding/telegram-discord/test: a test
// message to a saved webhook ({id}) or to a URL being entered
// ({webhook_url}). It changes nothing, so it is not audited.
func (s *Server) testT2D(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	var in struct {
		ID         *uint64 `json:"id"`
		WebhookURL *string `json:"webhook_url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var target string
	switch {
	case in.WebhookURL != nil:
		target = *in.WebhookURL
	case in.ID != nil:
		h, ok, err := s.getT2D(r.Context(), sc, *in.ID)
		if err != nil {
			s.internalError(w, "webhook", err)
			return
		}
		if !ok {
			notFound(w)
			return
		}
		target = h.URL
	default:
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.test_required")
		return
	}
	// A stored URL is checked again: rows written before the check
	// existed, or by hand, must not reach another host either.
	host, pathq, ok := parseWebhook(target)
	if !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_webhook", "fwd.bad_webhook")
		return
	}
	body, _ := json.Marshal(map[string]string{
		"username": "tgloggerd",
		"content":  s.textFor(r.Context(), v, "fwd.test_message"),
	})
	ctx, cancel := context.WithTimeout(r.Context(), discordTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+pathq,
		bytes.NewReader(body))
	if err != nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_webhook", "fwd.bad_webhook")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.discord.Do(req)
	if err != nil {
		// The error names the URL, whose token is a secret: log only
		// what went wrong.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		s.log.Warn("webhook test: Discord not reached", "err", err)
		s.refuse(w, r, v, http.StatusBadGateway, "send_failed", "fwd.test_unreachable")
		return
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail := strings.ToValidUTF8(strings.TrimSpace(string(text)), "")
		s.refuse(w, r, v, http.StatusBadGateway, "send_failed", "fwd.test_refused",
			"status", strconv.Itoa(resp.StatusCode), "body", detail)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createD2T answers POST /api/forwarding/discord-telegram: a route
// from a channel to a chat, read by a Discord bot and sent through a
// Telegram bot, both ones the viewer may use (bots.go).
func (s *Server) createD2T(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	var in struct {
		Channel     *string   `json:"discord_channel_id"`
		ChatID      *int64    `json:"chat_id"`
		DiscordBot  *uint64   `json:"discord_bot_id"`
		TelegramBot *uint64   `json:"telegram_bot_id"`
		Types       *[]string `json:"types"`
		Enabled     *bool     `json:"enabled"`
		Federation  *uint64   `json:"federation_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Channel == nil || in.ChatID == nil || in.DiscordBot == nil || in.TelegramBot == nil ||
		in.Types == nil || in.Enabled == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.d2t_required")
		return
	}
	channel, ok := parseSnowflake(*in.Channel)
	if !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_channel", "fwd.bad_channel")
		return
	}
	types, ok := s.fwdTypes(w, r, v, *in.Types, logdb.D2TTypeCount)
	if !ok {
		return
	}
	if _, ok := s.usableBot(w, r, sc, v, *in.DiscordBot, logdb.KindDiscord); !ok {
		return
	}
	if _, ok := s.usableBot(w, r, sc, v, *in.TelegramBot, logdb.KindTelegram); !ok {
		return
	}
	chat, ok := s.fwdTarget(w, r, sc, *in.ChatID, in.Federation)
	if !ok {
		return
	}
	id, err := logdb.CreateD2T(r.Context(), s.ro, channel, chat.ID, *in.TelegramBot, *in.DiscordBot,
		*in.Enabled, types)
	if errors.Is(err, logdb.ErrDuplicateRoute) {
		s.refuse(w, r, v, http.StatusConflict, "duplicate", "fwd.duplicate")
		return
	}
	if err != nil {
		s.internalError(w, "create route", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_d2t_create", fmt.Sprintf("route %d channel %d chat %d "+
		"discord_bot %d telegram_bot %d enabled=%t types=%s", id, channel, chat.ID,
		*in.DiscordBot, *in.TelegramBot, *in.Enabled,
		strings.Join(types.Keys(logdb.D2TTypeCount), ",")))
	s.recordOwner(r, webdb.Resource{Type: webdb.ResD2TRoute, ID: int64(id)}, v.ID)
	writeJSON(w, http.StatusCreated, map[string]uint64{"id": id})
}

// d2tOf loads the route in the path, answering 404 itself for one that
// does not exist or that the scope hides.
func (s *Server) d2tOf(w http.ResponseWriter, r *http.Request, sc scope.Scope) (logdb.D2TRoute, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		notFound(w)
		return logdb.D2TRoute{}, false
	}
	d, ok, err := s.getD2T(r.Context(), sc, id)
	if err != nil {
		s.internalError(w, "route", err)
		return d, false
	}
	if !ok {
		notFound(w)
	}
	return d, ok
}

// updateD2T answers PATCH /api/forwarding/discord-telegram/{id}: the
// fields given change, the others stay. A bot given must be one the
// viewer may use; the bots the route had stay, for other routes.
func (s *Server) updateD2T(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	d, ok := s.d2tOf(w, r, sc)
	if !ok {
		return
	}
	var in struct {
		Channel     *string   `json:"discord_channel_id"`
		ChatID      *int64    `json:"chat_id"`
		DiscordBot  *uint64   `json:"discord_bot_id"`
		TelegramBot *uint64   `json:"telegram_bot_id"`
		Types       *[]string `json:"types"`
		Enabled     *bool     `json:"enabled"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var changed []string
	if in.Channel != nil {
		channel, ok := parseSnowflake(*in.Channel)
		if !ok {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_channel", "fwd.bad_channel")
			return
		}
		if channel != d.ChannelID {
			changed = append(changed, fmt.Sprintf("channel=%d", channel))
		}
		d.ChannelID = channel
	}
	if in.Types != nil {
		t, ok := s.fwdTypes(w, r, v, *in.Types, logdb.D2TTypeCount)
		if !ok {
			return
		}
		d.Types = t
		changed = append(changed, "types="+strings.Join(t.Keys(logdb.D2TTypeCount), ","))
	}
	if in.Enabled != nil {
		d.Enabled = *in.Enabled
		changed = append(changed, "enabled="+strconv.FormatBool(d.Enabled))
	}
	if in.DiscordBot != nil {
		if _, ok := s.usableBot(w, r, sc, v, *in.DiscordBot, logdb.KindDiscord); !ok {
			return
		}
		d.DiscordBotID = *in.DiscordBot
		changed = append(changed, fmt.Sprintf("discord_bot=%d", d.DiscordBotID))
	}
	if in.TelegramBot != nil {
		if _, ok := s.usableBot(w, r, sc, v, *in.TelegramBot, logdb.KindTelegram); !ok {
			return
		}
		d.TelegramBotID = *in.TelegramBot
		changed = append(changed, fmt.Sprintf("telegram_bot=%d", d.TelegramBotID))
	}
	if in.ChatID != nil {
		chat, ok := s.fwdTarget(w, r, sc, *in.ChatID, nil)
		if !ok {
			return
		}
		if chat.ID != d.Chat.ID {
			changed = append(changed, fmt.Sprintf("chat=%d", chat.ID))
		}
		d.Chat = chat
	}
	err := logdb.UpdateD2T(r.Context(), s.ro, d)
	if errors.Is(err, logdb.ErrDuplicateRoute) {
		s.refuse(w, r, v, http.StatusConflict, "duplicate", "fwd.duplicate")
		return
	}
	if err != nil {
		s.internalError(w, "update route", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_d2t_update", fmt.Sprintf("route %d chat %d %s",
		d.ID, d.Chat.ID, strings.Join(changed, " ")))
	w.WriteHeader(http.StatusNoContent)
}

// deleteD2T answers DELETE /api/forwarding/discord-telegram/{id}. A
// route that has sent messages is refused (409 in_use): those rows map
// Discord messages to the Telegram ones; disabling it is the way.
func (s *Server) deleteD2T(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	d, ok := s.d2tOf(w, r, sc)
	if !ok {
		return
	}
	n, err := logdb.D2TSent(r.Context(), s.ro, d.ID)
	if err != nil {
		s.internalError(w, "route sent", err)
		return
	}
	if n > 0 {
		s.refuse(w, r, v, http.StatusConflict, "in_use", "fwd.in_use",
			"count", strconv.FormatInt(n, 10))
		return
	}
	if err := logdb.DeleteD2T(r.Context(), s.ro, d.ID); err != nil {
		s.internalError(w, "delete route", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_d2t_delete", fmt.Sprintf("route %d channel %d chat %d",
		d.ID, d.ChannelID, d.Chat.ID))
	s.dropOwner(r, webdb.Resource{Type: webdb.ResD2TRoute, ID: int64(d.ID)}, v.ID)
	w.WriteHeader(http.StatusNoContent)
}

// fwdCounts are a direction's numbers on the forwarding index
// (FwdCounts).
type fwdCounts struct {
	Active int   `json:"active"`
	Total  int   `json:"total"`
	Sent   int64 `json:"sent"`
}

type forwardingIndexProps struct {
	T2D fwdCounts `json:"t2d"`
	D2T fwdCounts `json:"d2t"`
}

func loadPlatformFwd(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "platform_fwd"); err != nil {
		return nil, err
	}
	s := pr.s
	ws, err := s.listT2D(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	rs, err := s.listD2T(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	bots, err := s.botsByID(ctx)
	if err != nil {
		return nil, err
	}
	var p forwardingIndexProps
	for _, w := range ws {
		p.T2D.Total++
		p.T2D.Sent += w.Sent
		if w.Enabled {
			p.T2D.Active++
		}
	}
	for _, r := range rs {
		p.D2T.Total++
		p.D2T.Sent += r.Sent
		// A route forwards only while its bots are enabled too.
		if d2tLive(r, bots) {
			p.D2T.Active++
		}
	}
	return p, nil
}

type t2dProps struct {
	Routes      []t2dRoute `json:"routes"`
	Federations []fedRef   `json:"federations"`
}

func loadT2D(ctx context.Context, pr *pageRequest) (any, error) {
	v, err := pageOf(pr, "platform_fwd")
	if err != nil {
		return nil, err
	}
	s := pr.s
	ws, err := s.listT2D(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	feds, err := s.fileFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(ws))
	for i, w := range ws {
		ids[i] = w.ID
	}
	owners, err := s.fwdOwners(ctx, webdb.ResT2DWebhook, ids)
	if err != nil {
		return nil, err
	}
	p := t2dProps{Routes: []t2dRoute{}, Federations: feds}
	for _, w := range ws {
		p.Routes = append(p.Routes, s.t2dJSON(ctx, v, w, owners))
	}
	return p, nil
}

type d2tProps struct {
	Routes      []d2tRoute `json:"routes"`
	Federations []fedRef   `json:"federations"`
}

func loadD2T(ctx context.Context, pr *pageRequest) (any, error) {
	v, err := pageOf(pr, "platform_fwd")
	if err != nil {
		return nil, err
	}
	s := pr.s
	rs, err := s.listD2T(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	feds, err := s.fileFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	owners, err := s.fwdOwners(ctx, webdb.ResD2TRoute, ids)
	if err != nil {
		return nil, err
	}
	bots, err := s.botsByID(ctx)
	if err != nil {
		return nil, err
	}
	p := d2tProps{Routes: []d2tRoute{}, Federations: feds}
	for _, r := range rs {
		p.Routes = append(p.Routes, s.d2tJSON(ctx, v, r, owners, bots))
	}
	return p, nil
}

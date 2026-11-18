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
)

// Platform forwarding (web/docs/platform-forwarding.md), the Drogon
// TelegramDiscordController and DiscordTelegramController as JSON
// operations: Telegram to Discord webhooks and Discord to Telegram
// routes, created, changed, deleted and, for a webhook, tested. They
// need the forward ability (Scope.Can); a configuration whose Telegram
// chat the scope hides does not exist for the request.
//
// Ownership is not stored yet (web-ownership): until it is, every
// configuration is shared, owned by nobody, in the Default federation,
// and visible to every viewer with the ability; transferring is not
// served.
//
// The webhook URL and the bot token are write-only: an answer carries
// the URL with its secret part cut off and the bot's name, never the
// token. A webhook URL must be Discord's, on save and on test, so the
// service cannot be made to post to another host.

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

// looksLikeBotToken checks a Telegram bot token's shape,
// <bot user id>:<secret>, as the Drogon page does; whether it works is
// discordd's to find out when it logs the bot in.
func looksLikeBotToken(t string) bool {
	colon := strings.IndexByte(t, ':')
	if colon <= 0 || colon+1 >= len(t) || len(t) > 255 {
		return false
	}
	for _, c := range t[:colon] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return !strings.ContainsAny(t, " \t\r\n")
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

// ownership is a configuration's owner and federation. Until
// web-ownership stores them, every configuration is shared (id 0) and
// in Default, and there is nobody to transfer it to; that item fills
// this in.
func (s *Server) ownership(ctx context.Context, v scope.Viewer, id uint64) owned {
	return owned{
		ID:                 id,
		Owner:              fwdPerson{ID: 0, Name: s.textFor(ctx, v, "fwd.shared"), Active: true},
		Federation:         fedRef{scope.DefaultFederation, s.textFor(ctx, v, "federation.default")},
		TransferCandidates: []fwdPerson{},
	}
}

func fwdPick(c logdb.FwdChat) pickItem {
	p := pickItem{ID: c.ID, Title: c.Title, Kind: c.Kind}
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

func (s *Server) t2dJSON(ctx context.Context, v scope.Viewer, h logdb.T2DWebhook) t2dRoute {
	return t2dRoute{owned: s.ownership(ctx, v, h.ID), Chat: fwdPick(h.Chat),
		URLMasked: maskWebhook(h.URL), Types: h.Types.Keys(logdb.T2DTypeCount),
		Enabled: h.Enabled, Sent: h.Sent, LastSent: isoPtr(h.LastSent)}
}

// d2tRoute is a route as the page shows it (D2TRoute).
type d2tRoute struct {
	owned
	ChannelID   string   `json:"channel_id"`
	ChannelName string   `json:"channel_name"`
	Chat        pickItem `json:"chat"`
	Bot         string   `json:"bot"`
	Types       []string `json:"types"`
	Enabled     bool     `json:"enabled"`
	Sent        int64    `json:"sent"`
}

// botLabel names a bot without its token: @username once discordd has
// logged it in, its user id, or its row while it has not.
func (s *Server) botLabel(ctx context.Context, v scope.Viewer, d logdb.D2TRoute) string {
	switch {
	case d.BotUsername != "":
		return "@" + d.BotUsername
	case d.BotUserID != 0:
		return strconv.FormatInt(d.BotUserID, 10)
	}
	return s.textFor(ctx, v, "fwd.bot_pending", "id", strconv.FormatUint(d.BotID, 10))
}

func (s *Server) d2tJSON(ctx context.Context, v scope.Viewer, d logdb.D2TRoute) d2tRoute {
	return d2tRoute{owned: s.ownership(ctx, v, d.ID),
		// A snowflake does not fit a JavaScript number exactly.
		ChannelID: strconv.FormatUint(d.ChannelID, 10), ChannelName: d.ChannelName,
		Chat: fwdPick(d.Chat), Bot: s.botLabel(ctx, v, d),
		Types: d.Types.Keys(logdb.D2TTypeCount), Enabled: d.Enabled, Sent: d.Sent}
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
	h, ok, err := logdb.GetT2D(r.Context(), s.ro, sc, id)
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
		Types      *[]string `json:"types"`
		Enabled    *bool     `json:"enabled"`
		WebhookURL *string   `json:"webhook_url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var changed []string
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
	if err := logdb.UpdateT2D(r.Context(), s.ro, h.ID, h.URL, h.Enabled, h.Types); err != nil {
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
		h, ok, err := logdb.GetT2D(r.Context(), s.ro, sc, *in.ID)
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

// createD2T answers POST /api/forwarding/discord-telegram.
func (s *Server) createD2T(w http.ResponseWriter, r *http.Request) {
	sc, v, ok := s.adminWrite(w, r, scope.Forward)
	if !ok {
		return
	}
	var in struct {
		Channel    *string   `json:"discord_channel_id"`
		ChatID     *int64    `json:"chat_id"`
		BotToken   *string   `json:"bot_token"`
		Types      *[]string `json:"types"`
		Enabled    *bool     `json:"enabled"`
		Federation *uint64   `json:"federation_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Channel == nil || in.ChatID == nil || in.BotToken == nil || in.Types == nil ||
		in.Enabled == nil {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_request", "fwd.d2t_required")
		return
	}
	channel, ok := parseSnowflake(*in.Channel)
	if !ok {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_channel", "fwd.bad_channel")
		return
	}
	if !looksLikeBotToken(*in.BotToken) {
		s.refuse(w, r, v, http.StatusBadRequest, "bad_token", "fwd.bad_token")
		return
	}
	types, ok := s.fwdTypes(w, r, v, *in.Types, logdb.D2TTypeCount)
	if !ok {
		return
	}
	chat, ok := s.fwdTarget(w, r, sc, *in.ChatID, in.Federation)
	if !ok {
		return
	}
	id, err := logdb.CreateD2T(r.Context(), s.ro, channel, chat.ID, *in.BotToken, *in.Enabled, types)
	if errors.Is(err, logdb.ErrDuplicateRoute) {
		s.refuse(w, r, v, http.StatusConflict, "duplicate", "fwd.duplicate")
		return
	}
	if err != nil {
		s.internalError(w, "create route", err)
		return
	}
	s.auditAdmin(r, v.ID, "fwd_d2t_create", fmt.Sprintf("route %d channel %d chat %d enabled=%t types=%s",
		id, channel, chat.ID, *in.Enabled, strings.Join(types.Keys(logdb.D2TTypeCount), ",")))
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
	d, ok, err := logdb.GetD2T(r.Context(), s.ro, sc, id)
	if err != nil {
		s.internalError(w, "route", err)
		return d, false
	}
	if !ok {
		notFound(w)
	}
	return d, ok
}

// updateD2T answers PATCH /api/forwarding/discord-telegram/{id}; a bot
// token replaces the route's bot.
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
		Types    *[]string `json:"types"`
		Enabled  *bool     `json:"enabled"`
		BotToken *string   `json:"bot_token"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var changed []string
	if in.BotToken != nil {
		if !looksLikeBotToken(*in.BotToken) {
			s.refuse(w, r, v, http.StatusBadRequest, "bad_token", "fwd.bad_token")
			return
		}
		changed = append(changed, "bot")
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
	if err := logdb.UpdateD2T(r.Context(), s.ro, d.ID, d.Enabled, d.Types, in.BotToken); err != nil {
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
	ws, err := logdb.ListT2D(ctx, s.ro, pr.scope)
	if err != nil {
		return nil, err
	}
	rs, err := logdb.ListD2T(ctx, s.ro, pr.scope)
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
		// A route forwards only while its bot is enabled too.
		if r.Enabled && r.BotEnabled {
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
	ws, err := logdb.ListT2D(ctx, s.ro, pr.scope)
	if err != nil {
		return nil, err
	}
	feds, err := s.fileFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	p := t2dProps{Routes: []t2dRoute{}, Federations: feds}
	for _, w := range ws {
		p.Routes = append(p.Routes, s.t2dJSON(ctx, v, w))
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
	rs, err := logdb.ListD2T(ctx, s.ro, pr.scope)
	if err != nil {
		return nil, err
	}
	feds, err := s.fileFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	p := d2tProps{Routes: []d2tRoute{}, Federations: feds}
	for _, r := range rs {
		p.Routes = append(p.Routes, s.d2tJSON(ctx, v, r))
	}
	return p, nil
}

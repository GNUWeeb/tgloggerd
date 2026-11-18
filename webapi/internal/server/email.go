// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/i18n"
	"tgloggerd/webapi/internal/mail"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/webdb"
)

// Account email (docs/design/web-accounts.md section 8): verified
// addresses, the one-time links sent to them, and the security
// notices. The contract is in webapi/openapi.yaml; web/docs/accounts.md
// explains the flows.
//
// A link is <site>/password/set#token=<token> or
// <site>/account/email/verify#token=<token>. The token is in the
// fragment, so it reaches no server log and no Referer header; the
// page posts it only when the person acts on it, so a mail scanner
// that opens the link uses nothing up. Only its SHA-256 is stored.

// MailConfig is what the links and messages are made with.
type MailConfig struct {
	// Site is the public base URL the links point to, without a
	// trailing slash (WEB_PUBLIC_URL).
	Site string
	// InviteTTL is how long an invitation, or a reset an admin made,
	// works; ResetTTL a reset asked for on the forgotten-password
	// page; VerifyTTL an address verification.
	InviteTTL time.Duration
	ResetTTL  time.Duration
	VerifyTTL time.Duration
}

// The defaults of MailConfig's lifetimes.
const (
	DefaultInviteTTL = 72 * time.Hour
	DefaultResetTTL  = time.Hour
	DefaultVerifyTTL = 24 * time.Hour
)

// The forgotten-password throttle: requests per client address, and
// links sent per account, within forgotWindow. A client past its cap
// is told (429), which says nothing about any account; an account
// past its cap silently gets no more links, since telling would say
// that it exists.
const (
	forgotPerAddress = 5
	forgotPerAccount = 3
	forgotWindow     = time.Hour
)

// forgotPath is where the forgotten-password form posts; the pre-login
// CSRF cookie is set for it as well as for the login.
const forgotPath = "/api/password/forgot"

// mailOn is whether links and notices can be sent: a relay and the
// site's URL are configured. Mail is optional
// (decisions/2026-11-18-email-optional.md): without it nothing is sent
// and nothing waits for it.
func (s *Server) mailOn() bool {
	return s.mail.Enabled() && s.mailCfg.Site != ""
}

// mailOff refuses an operation whose only point is to send a link (an
// invitation asked for explicitly) while mail is off: 409 mail_off.
func (s *Server) mailOff(w http.ResponseWriter, lang string) {
	writeError(w, http.StatusConflict, "mail_off",
		map[string]any{"message": i18n.T(lang, "mail.off")})
}

// mailKinds are the messages sent, by the key of their texts
// (mail.<kind>.subject and .body).
var mailKinds = []string{
	"invite", "admin_reset", "reset", "verify",
	"notice.password", "notice.email_old", "notice.email_new",
	"notice.new_device", "notice.disabled", "notice.role",
}

// recipient is who a message goes to.
type recipient struct {
	uid      uint64
	username string
	name     string // the display name, or the username without one
	addr     string
}

func recipientOf(a webdb.Account, addr string) recipient {
	name := a.DisplayName
	if name == "" {
		name = a.Username
	}
	return recipient{uid: a.ID, username: a.Username, name: name, addr: addr}
}

// locale is the language and time zone an account reads its mail in:
// its own settings, else the site's.
func (s *Server) locale(ctx context.Context, uid uint64) (string, *time.Location) {
	p, err := s.viewerPrefs(ctx, uid)
	if err != nil {
		p = settings.Prefs{}
	}
	e := settings.Resolve(p, s.defaults)
	loc, err := time.LoadLocation(e.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return i18n.Pick(e.Lang), loc
}

// mailTime is how a time reads in mail.
func mailTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02 15:04 MST")
}

// send queues the message kind (its texts are mail.<kind>.subject and
// mail.<kind>.body) to rc, in the account's language. link, when not
// empty, is the one-time link, valid until expires. kv are the
// message's own values; name, username, site, time, link and expires
// are filled in for every message. It never blocks: a message that
// cannot be queued is logged and dropped.
func (s *Server) send(ctx context.Context, rc recipient, kind, link string, expires time.Time, kv ...string) {
	lang, loc := s.locale(ctx, rc.uid)
	vals := append([]string{
		"name", rc.name, "username", rc.username, "site", s.mailCfg.Site,
		"time", mailTime(s.clock(), loc), "link", link,
	}, kv...)
	if !expires.IsZero() {
		vals = append(vals, "expires", mailTime(expires, loc))
	}
	text := i18n.T(lang, "mail."+kind+".body", vals...) + "\n\n" +
		i18n.T(lang, "mail.footer", vals...)
	s.mail.Send(mail.Message{
		To:      rc.addr,
		Subject: i18n.T(lang, "mail."+kind+".subject", vals...),
		Text:    text + "\n",
		HTML:    mailHTML(text, link),
		Kind:    kind,
	})
}

// mailHTML is the simple HTML version of a message's text: one
// paragraph per blank-line-separated block, the link clickable.
func mailHTML(text, link string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><body style="font-family: sans-serif; line-height: 1.5">`)
	el := html.EscapeString(link)
	for _, para := range strings.Split(text, "\n\n") {
		p := html.EscapeString(para)
		if link != "" {
			p = strings.ReplaceAll(p, el, `<a href="`+el+`">`+el+`</a>`)
		}
		b.WriteString("<p>" + strings.ReplaceAll(p, "\n", "<br>") + "</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// sendLink issues a link of purpose p to addr and mails it as kind.
// Earlier unused links of the same purpose stop working.
func (s *Server) sendLink(ctx context.Context, rc recipient, p webdb.LinkPurpose, kind string,
	ttl time.Duration) error {
	token, err := webdb.IssueLink(ctx, s.app, rc.uid, p, rc.addr, ttl)
	if err != nil {
		return err
	}
	path := "/password/set"
	if p == webdb.LinkVerify {
		path = "/account/email/verify"
	}
	s.send(ctx, rc, kind, s.mailCfg.Site+path+"#token="+token, s.clock().Add(ttl))
	return nil
}

// notice sends a security notice to the account's verified address. An
// account without one gets none: an address is used for notices only
// once verified.
func (s *Server) notice(ctx context.Context, a webdb.Account, kind string, kv ...string) {
	if !a.EmailVerified || a.Email == "" {
		return
	}
	s.send(ctx, recipientOf(a, a.Email), "notice."+kind, "", time.Time{}, kv...)
}

// noticeByID is notice for an account by id; a failed lookup only goes
// to the log.
func (s *Server) noticeByID(ctx context.Context, uid uint64, kind string, kv ...string) {
	a, err := webdb.GetAccount(ctx, s.app, uid)
	if err != nil {
		s.log.Warn("notice not sent", "kind", kind, "uid", uid, "err", err)
		return
	}
	s.notice(ctx, a, kind, kv...)
}

// changeEmail answers POST /api/account/email: the current password
// confirms it is the account's owner. With mail, a verification link
// goes to the new address, which becomes the account's only once
// verified; without mail, the address is recorded at once, unverified,
// since it could never be verified otherwise. An empty address removes
// the account's address.
func (s *Server) changeEmail(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	ctx := r.Context()
	lang := s.textLang(ctx, sess.UID)
	var in struct {
		Email    *string `json:"email"`
		Password *string `json:"password"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Email == nil || in.Password == nil {
		badRequest(w, "bad_request", i18n.T(lang, "email.required"))
		return
	}
	// The same throttle as a password change: both take the password.
	key := "password\n" + strconv.FormatUint(sess.UID, 10)
	if !s.limiter.Allowed(key) {
		writeError(w, http.StatusTooManyRequests, "rate_limited",
			map[string]any{"message": i18n.T(lang, "throttled")})
		return
	}
	u, err := webdb.FindByID(ctx, s.app, sess.UID)
	if err != nil {
		s.internalError(w, "account lookup", err)
		return
	}
	ip := clientIP(r)
	if !auth.VerifyPassword(u.PasswordHash, *in.Password) {
		s.limiter.RecordFailure(key)
		if err := webdb.Audit(ctx, s.app, &u.ID, "email_change_fail", ip, u.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		badRequest(w, "wrong_password", i18n.T(lang, "email.wrong_password"))
		return
	}
	s.limiter.Reset(key)
	email := ""
	if strings.TrimSpace(*in.Email) != "" {
		var ok bool
		if email, ok = cleanEmail(*in.Email); !ok {
			badRequest(w, "bad_email", i18n.T(lang, "email.bad"))
			return
		}
	}
	a, err := webdb.GetAccount(ctx, s.app, sess.UID)
	if err != nil {
		s.internalError(w, "account lookup", err)
		return
	}
	mailOn := s.mailOn() && email != ""
	if strings.EqualFold(a.Email, email) && (a.EmailVerified || !mailOn) {
		badRequest(w, "bad_email", i18n.T(lang, "email.same"))
		return
	}
	if !mailOn {
		// Recorded at once, unverified (or removed); the old address,
		// if verified, is told when mail is on.
		_, err := webdb.UpdateAccount(ctx, s.app, a.ID, a.ID, webdb.AccountChange{Email: &email},
			"email_change", ip)
		switch {
		case errors.Is(err, webdb.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email_taken",
				map[string]any{"message": i18n.T(lang, "email.taken")})
			return
		case err != nil:
			s.internalError(w, "email change", err)
			return
		}
		s.notice(ctx, a, "email_old", "email", email)
		writeJSON(w, http.StatusOK, map[string]any{"verification_sent": false})
		return
	}
	switch taken, err := webdb.EmailInUse(ctx, s.app, email, a.ID); {
	case err != nil:
		s.internalError(w, "email lookup", err)
		return
	case taken:
		writeError(w, http.StatusConflict, "email_taken",
			map[string]any{"message": i18n.T(lang, "email.taken")})
		return
	}
	if err := webdb.Audit(ctx, s.app, &a.ID, "email_change_request", ip, a.Username); err != nil {
		s.internalError(w, "audit", err)
		return
	}
	if err := s.sendLink(ctx, recipientOf(a, email), webdb.LinkVerify, "verify", s.mailCfg.VerifyTTL); err != nil {
		s.internalError(w, "verification link", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification_sent": true})
}

// linkBody is the token of a link operation, and the new password of
// a set-password one.
type linkBody struct {
	Token       *string `json:"token"`
	NewPassword *string `json:"new_password"`
}

// verifyEmail answers POST /api/account/email/verify. It needs no
// session: the link may be opened in another browser. The address the
// link went to becomes the account's, verified; when it replaces
// another, both are told.
func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	lang := s.defaults.Lang
	var in linkBody
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Token == nil || *in.Token == "" {
		badRequest(w, "bad_link", i18n.T(lang, "link.bad"))
		return
	}
	ctx := r.Context()
	v, err := webdb.VerifyEmailFromLink(ctx, s.app, *in.Token, clientIP(r))
	switch {
	case errors.Is(err, webdb.ErrBadLink):
		badRequest(w, "bad_link", i18n.T(lang, "link.bad"))
		return
	case errors.Is(err, webdb.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken",
			map[string]any{"message": i18n.T(lang, "email.taken")})
		return
	case err != nil:
		s.internalError(w, "verify email", err)
		return
	}
	if v.Old != "" {
		a, err := webdb.GetAccount(ctx, s.app, v.UID)
		if err != nil {
			s.log.Warn("address change notices not sent", "uid", v.UID, "err", err)
		} else {
			s.notice(ctx, a, "email_new")
			if v.OldVerified {
				s.send(ctx, recipientOf(a, v.Old), "notice.email_old", "", time.Time{},
					"email", v.Email)
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// setPassword answers POST /api/password/set: the token of an
// invitation or a reset, and the new password. The password is checked
// first, so a short one does not use the link up. Every session of the
// account ends; the account then signs in with the new password.
func (s *Server) setPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	lang := s.defaults.Lang
	var in linkBody
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Token == nil || *in.Token == "" {
		badRequest(w, "bad_link", i18n.T(lang, "link.bad"))
		return
	}
	if in.NewPassword == nil || len(*in.NewPassword) < MinPasswordLen {
		badRequest(w, "weak_password", i18n.T(lang, "password.weak"))
		return
	}
	hash, err := auth.HashPassword(*in.NewPassword)
	if err != nil {
		s.internalError(w, "hash", err)
		return
	}
	ctx := r.Context()
	res, err := webdb.SetPasswordFromLink(ctx, s.app, *in.Token, hash, clientIP(r))
	switch {
	case errors.Is(err, webdb.ErrBadLink):
		badRequest(w, "bad_link", i18n.T(lang, "link.bad"))
		return
	case err != nil:
		s.internalError(w, "set password", err)
		return
	}
	// An account's first password is no change to tell it about.
	if !res.First {
		s.noticeByID(ctx, res.UID, "password")
	}
	w.WriteHeader(http.StatusNoContent)
}

// forgotPassword answers POST /api/password/forgot: a reset link to
// the verified address of the account named, by username or address.
// The answer is 202 whether or not there is such an account, and
// whether or not mail is set up (without it nothing is sent); only a
// client asking too often is told (429).
func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	lang := s.defaults.Lang
	if !s.signer.CheckLogin(r, r.Header.Get(CSRFHeader)) {
		writeError(w, http.StatusForbidden, "csrf", map[string]any{"message": msgExpired})
		return
	}
	var in struct {
		Username *string `json:"username"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Username == nil {
		badRequest(w, "bad_request", i18n.T(lang, "forgot.required"))
		return
	}
	ctx := r.Context()
	ip := clientIP(r)
	key := "forgot\n" + ip
	if !s.forgotLimiter.Allowed(key) {
		writeError(w, http.StatusTooManyRequests, "rate_limited",
			map[string]any{"message": i18n.T(lang, "throttled")})
		return
	}
	s.forgotLimiter.RecordFailure(key)

	name := strings.TrimSpace(*in.Username)
	a, err := webdb.Account{}, webdb.ErrNoUser
	if name != "" && len(name) <= maxEmail {
		a, err = webdb.FindForReset(ctx, s.app, name)
	}
	switch {
	case errors.Is(err, webdb.ErrNoUser):
		cut, _ := cutRunes(name, maxUsername)
		if err := webdb.Audit(ctx, s.app, nil, "password_forgot_unknown", ip, cut); err != nil {
			s.internalError(w, "audit", err)
			return
		}
	case err != nil:
		s.internalError(w, "account lookup", err)
		return
	default:
		akey := "forgot\n" + strconv.FormatUint(a.ID, 10)
		if !s.forgotAcct.Allowed(akey) {
			if err := webdb.AuditAccount(ctx, s.app, a.ID, a.ID, "password_forgot_ratelimited",
				ip, a.Username); err != nil {
				s.internalError(w, "audit", err)
				return
			}
			break
		}
		s.forgotAcct.RecordFailure(akey)
		if err := webdb.AuditAccount(ctx, s.app, a.ID, a.ID, "password_forgot", ip, a.Username); err != nil {
			s.internalError(w, "audit", err)
			return
		}
		if !s.mailOn() {
			break
		}
		if err := s.sendLink(ctx, recipientOf(a, a.Email), webdb.LinkPassword, "reset", s.mailCfg.ResetTTL); err != nil {
			s.internalError(w, "reset link", err)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// noteDevice records the device of a sign-in and sends the new device
// notice when the account had signed in from others before. Failures
// only go to the log; the sign-in goes on.
func (s *Server) noteDevice(r *http.Request, uid uint64) {
	ctx := r.Context()
	dev := deviceName(i18n.Fallback, r.UserAgent())
	isNew, err := webdb.NoteDevice(ctx, s.app, uid, dev)
	if err != nil {
		s.log.Warn("device not recorded", "uid", uid, "err", err)
		return
	}
	if isNew {
		lang, _ := s.locale(ctx, uid)
		s.noticeByID(ctx, uid, "new_device", "device", deviceName(lang, r.UserAgent()),
			"ip", clientIP(r))
	}
}

// loadLinkPage loads the email link pages and the forgotten-password
// page. Their props are empty and their GETs change nothing: the
// token is in the fragment, which the server never sees.
func loadLinkPage(context.Context, *pageRequest) (any, error) {
	return struct{}{}, nil
}

// resendInvite answers POST /api/accounts/{id}/resend-invite: a new
// set-password link to an account that has not set its own password
// yet; earlier ones stop working.
func (s *Server) resendInvite(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.checkCSRF(w, r) {
		return
	}
	sc, v, ok := actor(w, r)
	if !ok {
		return
	}
	a, ok := s.target(w, r, sc)
	if !ok {
		return
	}
	ctx := r.Context()
	lang := s.textLang(ctx, v.ID)
	switch {
	case a.HasPassword && !a.MustChangePassword:
		writeError(w, http.StatusConflict, "not_invited",
			map[string]any{"message": i18n.T(lang, "account.not_invited")})
		return
	case a.Email == "":
		writeError(w, http.StatusConflict, "no_email",
			map[string]any{"message": i18n.T(lang, "account.no_email")})
		return
	case !s.mailOn():
		s.mailOff(w, lang)
		return
	}
	if err := webdb.AuditAccount(ctx, s.app, v.ID, a.ID, "account_invite_resend", clientIP(r), a.Email); err != nil {
		s.internalError(w, "audit", err)
		return
	}
	kind := "invite"
	if a.LastSignin != nil {
		// It signed in before: an admin's reset, sent again.
		kind = "admin_reset"
	}
	if err := s.sendLink(ctx, recipientOf(a, a.Email), webdb.LinkPassword, kind, s.mailCfg.InviteTTL); err != nil {
		s.internalError(w, "invitation link", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

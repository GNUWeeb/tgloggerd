// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import "net/http"

// CSRF protection as web/src/auth/Csrf.cpp does it, with no server-side
// store. A signed-in request's token is a keyed tag over its session
// cookie, so it is the same in both services and changes with every
// new session. The login form has no session yet; it uses a signed
// random token, set as a cookie and echoed in the request (double
// submit).

// LoginCookie is the pre-login CSRF cookie's name.
const LoginCookie = "tgw_csrf"

// loginCookieAge gives a sign-in half an hour.
const loginCookieAge = 1800

// SessionCSRF is the CSRF token for a session cookie value, or "" when
// there is none (csrf::forSession).
func (s *Signer) SessionCSRF(rawSession string) string {
	if rawSession == "" {
		return ""
	}
	return s.Tag("csrf\n" + rawSession)
}

// CheckSessionCSRF compares a submitted token with the request's in
// constant time (csrf::checkSession). It looks only at the cookie's
// presence, not at its validity, as the C++ does; the session itself is
// checked separately.
func (s *Signer) CheckSessionCSRF(r *http.Request, submitted string) bool {
	return nonEmptyEqual(s.SessionCSRF(RawSession(r)), submitted)
}

// NewLoginToken is a fresh signed login token (csrf::newLoginToken).
func (s *Signer) NewLoginToken() string {
	return s.Make(RandomToken())
}

// SetLoginCookie sets the pre-login cookie for path. The Drogon web
// scopes it to its form's path, /login; the Go service uses its own
// path, so the two cookies never meet.
func (c *Cookies) SetLoginCookie(w http.ResponseWriter, tok, path string) {
	http.SetCookie(w, c.cookie(LoginCookie, tok, path, loginCookieAge))
}

// CheckLogin checks a submitted login token: the cookie must carry a
// valid signature and equal the submitted value (csrf::checkLogin).
func (s *Signer) CheckLogin(r *http.Request, submitted string) bool {
	ck := cookieValue(r, LoginCookie)
	if _, ok := s.Open(ck); !ok {
		return false
	}
	return nonEmptyEqual(ck, submitted)
}

// nonEmptyEqual is ctEqual of Csrf.cpp: false when either side is empty
// or the lengths differ.
func nonEmptyEqual(a, b string) bool {
	return a != "" && equal(a, b)
}

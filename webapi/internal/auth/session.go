// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"crypto/sha256"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SessionCookie is the session cookie's name in both services.
const SessionCookie = "tgw_session"

// neverMaxAge is the cookie lifetime when sessions never expire: ten
// years, so the browser still keeps the cookie on disk.
const neverMaxAge = 315360000

// Session is the identity a session cookie carries
// (web/src/auth/Session.hpp).
type Session struct {
	UID      uint64
	Username string
	Role     string // "superadmin", "admin" or "member"; see Verifier
	Exp      int64  // absolute Unix expiry; 0 = never
	Epoch    uint32 // the account's session_epoch when it was issued
	// MustChangePassword is not in the cookie: Verifier sets it, and
	// the role, from the account.
	MustChangePassword bool

	// Key identifies the session in web_sessions: the SHA-256 of the
	// cookie value (SessionKey). Row is the session's web_sessions.id,
	// 0 while it is not recorded yet; Stale says its recorded last use
	// is old enough to be recorded again. Verifier sets all three.
	Key   [32]byte
	Row   uint64
	Stale bool
}

// NeverExpires is the expiry this service gives a session when
// sessions never expire (WEB_SESSION_TIMEOUT <= 0), instead of 0: the
// last second of the year 9999. Sessions are told apart by their
// cookie, so each new one needs an expiry of its own, which a constant
// 0 cannot give; one second less is taken for each session that
// already has this one (see the server's startSession).
const NeverExpires int64 = 253402300799

// SessionKey is the key of the session a cookie value carries. The
// value is canonical (Signer.Open accepts one spelling of a token
// only), so a cookie cannot be respelled to dodge its row.
func SessionKey(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

// Cookies issues and reads the session and login CSRF cookies with the
// Drogon web's names and flags.
type Cookies struct {
	Signer *Signer
	// Timeout is WEB_SESSION_TIMEOUT in seconds; <= 0 means sessions
	// never expire.
	Timeout int
	// Secure is WEB_SECURE_COOKIE != "0".
	Secure bool
	// Now returns the current time; nil means time.Now.
	Now func() time.Time
}

func (c *Cookies) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// EncodeSession signs "<uid>\n<exp>\n<epoch>\n<role>\n<username>".
func (s *Signer) EncodeSession(sess Session) string {
	payload := strconv.FormatUint(sess.UID, 10) + "\n" +
		strconv.FormatInt(sess.Exp, 10) + "\n" +
		strconv.FormatUint(uint64(sess.Epoch), 10) + "\n" +
		sess.Role + "\n" + sess.Username
	return s.Make(payload)
}

// parseSession reads a payload as session::parse does. The username is
// the rest, so it may hold anything; a payload with fewer than five
// fields (the format before session epochs) does not parse.
func parseSession(p string) (Session, bool) {
	f := strings.SplitN(p, "\n", 5)
	if len(f) != 5 {
		return Session{}, false
	}
	var s Session
	var ok bool
	if s.UID, ok = stoull(f[0]); !ok {
		return Session{}, false
	}
	if s.Exp, ok = stoll(f[1]); !ok {
		return Session{}, false
	}
	epoch, ok := stoul(f[2])
	if !ok {
		return Session{}, false
	}
	s.Epoch = uint32(epoch) // the C++ casts unsigned long to uint32_t
	s.Role = f[3]
	s.Username = f[4]
	return s, true
}

// OpenSession verifies a session cookie's signature and expiry. It does
// not check the epoch; Verifier does.
func (c *Cookies) OpenSession(raw string) (Session, bool) {
	if raw == "" {
		return Session{}, false
	}
	p, ok := c.Signer.Open(raw)
	if !ok {
		return Session{}, false
	}
	s, ok := parseSession(p)
	if !ok {
		return Session{}, false
	}
	if s.Exp != 0 && c.now().Unix() >= s.Exp {
		return Session{}, false
	}
	return s, true
}

// RawSession is the request's session cookie value, or "".
func RawSession(r *http.Request) string {
	return cookieValue(r, SessionCookie)
}

func cookieValue(r *http.Request, name string) string {
	ck, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return ck.Value
}

// Current is the request's signed, unexpired session (session::current).
func (c *Cookies) Current(r *http.Request) (Session, bool) {
	return c.OpenSession(RawSession(r))
}

// NewSession is the session issued at login: it expires Timeout
// seconds from now, or never.
func (c *Cookies) NewSession(uid uint64, username, role string, epoch uint32) Session {
	var exp int64
	if c.Timeout > 0 {
		exp = c.now().Unix() + int64(c.Timeout)
	}
	return Session{UID: uid, Username: username, Role: role, Exp: exp, Epoch: epoch}
}

// Issue sets the session cookie (session::issue) and returns its value.
func (c *Cookies) Issue(w http.ResponseWriter, s Session) string {
	v := c.Signer.EncodeSession(s)
	age := c.Timeout
	if age <= 0 {
		age = neverMaxAge
	}
	http.SetCookie(w, c.cookie(SessionCookie, v, "/", age))
	return v
}

// Clear expires the session cookie (session::clear).
func (c *Cookies) Clear(w http.ResponseWriter) {
	http.SetCookie(w, c.cookie(SessionCookie, "", "/", -1))
}

// cookie has the flags both services set on every cookie. maxAge < 0
// sends Max-Age=0.
func (c *Cookies) cookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

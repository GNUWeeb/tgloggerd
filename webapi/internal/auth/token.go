// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

// b64url is libsodium's URLSAFE_NO_PADDING variant, the one every
// signed token uses.
var b64url = base64.RawURLEncoding.Strict()

// Signer makes and opens signed tokens as web/src/auth/Token.cpp does:
// "<b64url(payload)>.<b64url(HMAC-SHA256(key, b64url(payload)))>".
// Anyone can read the payload; only a holder of the key can make a
// token that opens.
type Signer struct {
	key []byte
}

// NewSigner returns a signer over the decoded WEB_APP_KEY.
func NewSigner(key []byte) *Signer {
	return &Signer{key: append([]byte(nil), key...)}
}

func (s *Signer) mac(msg string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

// Tag is b64url(HMAC-SHA256(key, msg)): a keyed, unforgeable tag.
func (s *Signer) Tag(msg string) string {
	return b64url.EncodeToString(s.mac(msg))
}

// Make signs a payload into a token.
func (s *Signer) Make(payload string) string {
	body := b64url.EncodeToString([]byte(payload))
	return body + "." + s.Tag(body)
}

// Open checks a token's signature in constant time and returns its
// payload. The signature is everything after the first dot, so a token
// with a second dot never opens.
func (s *Signer) Open(tok string) (string, bool) {
	body, sig, ok := strings.Cut(tok, ".")
	if !ok {
		return "", false
	}
	if !equal(s.Tag(body), sig) {
		return "", false
	}
	raw, err := b64url.DecodeString(body)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// RandomToken is 32 random bytes, b64url-encoded (a CSRF nonce).
func RandomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("auth: no randomness: " + err.Error())
	}
	return b64url.EncodeToString(b[:])
}

// equal compares in constant time; lengths may leak, as in the C++.
func equal(a, b string) bool {
	return len(a) == len(b) &&
		subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

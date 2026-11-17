// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package auth is the Go service's side of the authentication the
// Drogon web does in web/src/auth: signed session cookies, CSRF tokens,
// file tokens, password hashes and the login throttle.
//
// Every value here is made exactly as the C++ code makes it, so a
// cookie, token or hash minted by one service is accepted by the other
// while both run (docs/design/web-go.md section 3). The formats are
// copied, not redesigned; a new scheme can come once Drogon is gone.
package auth

import (
	"encoding/base64"
	"errors"
	"strings"
)

// MinKeyLen is the shortest WEB_APP_KEY, after decoding, that the
// Drogon web accepts.
const MinKeyLen = 16

// MaxKeyLen is the longest key keyed BLAKE2b takes. The Drogon web does
// not check it and, given a longer key, derives its file token and
// daemon keys from no key at all; refusing such a key here keeps the
// two services from disagreeing silently.
const MaxKeyLen = 64

// keyEncodings are the spellings the Drogon web tries, in its order:
// libsodium's ORIGINAL, ORIGINAL_NO_PADDING, URLSAFE and
// URLSAFE_NO_PADDING variants. Strict matches libsodium in refusing
// non-zero trailing bits.
var keyEncodings = []*base64.Encoding{
	base64.StdEncoding.Strict(),
	base64.RawStdEncoding.Strict(),
	base64.URLEncoding.Strict(),
	base64.RawURLEncoding.Strict(),
}

// DecodeAppKey decodes WEB_APP_KEY as the Drogon web does
// (b64decodeAny in web/src/auth/Token.cpp): base64 in any of four
// spellings, at least MinKeyLen bytes. Unlike Go's decoder, libsodium
// skips no newlines, so a key with one is refused here too.
func DecodeAppKey(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("WEB_APP_KEY is not set")
	}
	if strings.ContainsAny(s, "\r\n") {
		return nil, errors.New("WEB_APP_KEY is not valid base64")
	}
	for _, enc := range keyEncodings {
		k, err := enc.DecodeString(s)
		if err != nil {
			continue
		}
		if len(k) < MinKeyLen {
			return nil, errors.New("WEB_APP_KEY is shorter than 16 bytes")
		}
		if len(k) > MaxKeyLen {
			return nil, errors.New("WEB_APP_KEY is longer than 64 bytes")
		}
		return k, nil
	}
	return nil, errors.New("WEB_APP_KEY is not valid base64")
}

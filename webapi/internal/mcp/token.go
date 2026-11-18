// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

// MCP bearer tokens, as the Drogon web mints and checks them
// (web/src/mcp/Token.cpp, web/src/dao/Mcp.cpp): "tgmcp_" and 32 random
// bytes in lowercase hex, stored in web_mcp_tokens (web migration
// 000003) only as the SHA-256 of the whole token. A fast hash is enough
// for 256 random bits; there is nothing to guess.

// TokenPrefix starts every token, so one is recognisable in a paste.
const TokenPrefix = "tgmcp_"

// TokenQueryParam carries a token in the query string, for clients
// that accept only a URL. Weaker than the header: a query string ends
// up in proxy logs and browser history (web/docs/mcp.md).
const TokenQueryParam = "key"

const tokenRandomBytes = 32

// LooksLikeToken checks the shape only, not whether the token exists.
func LooksLikeToken(s string) bool {
	if len(s) != len(TokenPrefix)+2*tokenRandomBytes || !strings.HasPrefix(s, TokenPrefix) {
		return false
	}
	for _, c := range s[len(TokenPrefix):] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// TokenDigest is the stored form: the raw SHA-256 of the token.
func TokenDigest(tok string) [32]byte { return sha256.Sum256([]byte(tok)) }

// NewToken mints a token, as web/src/mcp/Token.cpp does: the prefix
// and tokenRandomBytes random bytes in lowercase hex. It returns the
// token, to be shown once, and the digest it is stored as.
func NewToken() (string, [32]byte, error) {
	b := make([]byte, tokenRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", [32]byte{}, err
	}
	tok := TokenPrefix + hex.EncodeToString(b)
	return tok, TokenDigest(tok), nil
}

// FromAuthorizationHeader returns the credential of a "Bearer <token>"
// header, the scheme matched case-insensitively and blanks around the
// token dropped; "" for anything else.
func FromAuthorizationHeader(h string) string {
	const scheme = "bearer"
	if len(h) <= len(scheme) || !strings.EqualFold(h[:len(scheme)], scheme) {
		return ""
	}
	if h[len(scheme)] != ' ' && h[len(scheme)] != '\t' {
		return ""
	}
	return strings.TrimRight(strings.TrimLeft(h[len(scheme):], " \t"), " \t\r\n")
}

// TokenOwner is the account a token belongs to.
type TokenOwner struct {
	TokenID   uint64
	WebUserID uint64
	Username  string
	Role      string
}

// ErrNoToken is an unknown digest, a revoked token, or a token of a
// disabled account.
var ErrNoToken = errors.New("mcp: invalid or revoked token")

// ResolveToken finds the token with this digest in the web's schema.
// A revoked token and a disabled account are each enough to refuse.
func ResolveToken(ctx context.Context, app *sql.DB, digest [32]byte) (TokenOwner, error) {
	var o TokenOwner
	err := app.QueryRowContext(ctx,
		"SELECT t.id, t.web_user_id, u.username, u.role "+
			"FROM web_mcp_tokens t "+
			"JOIN web_users u ON u.id = t.web_user_id "+
			"WHERE t.token_sha256 = ? AND t.revoked_at IS NULL "+
			"  AND u.is_active = 1 LIMIT 1",
		digest[:]).Scan(&o.TokenID, &o.WebUserID, &o.Username, &o.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return o, ErrNoToken
	}
	return o, err
}

// TouchToken stamps the token's last use. Callers ignore its failure: a
// missed stamp must not fail a request.
func TouchToken(ctx context.Context, app *sql.DB, id uint64) error {
	_, err := app.ExecContext(ctx,
		"UPDATE web_mcp_tokens SET last_used_at = NOW() WHERE id = ?", id)
	return err
}

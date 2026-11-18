// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// The email side of the accounts (docs/design/web-accounts.md section
// 8): one-time links, verified addresses and known devices. A link's
// token goes into the email only; web_email_links keeps its SHA-256,
// so a copy of the database opens no account.

// LinkPurpose is what a link does.
type LinkPurpose string

const (
	// LinkPassword sets the account's password: an invitation or a
	// reset. Using one also verifies the address it was sent to.
	LinkPassword LinkPurpose = "password"
	// LinkVerify verifies (and makes current) the address it was sent
	// to.
	LinkVerify LinkPurpose = "verify"
)

// ErrBadLink: the token is unknown, used, replaced, expired, of
// another purpose, or its account is disabled.
var ErrBadLink = errors.New("bad link")

// LinkToken is a new random token: 32 bytes, base64url without
// padding, so it fits a URL fragment as it is.
func LinkToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func tokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// IssueLink stores a new link for the account and returns its token.
// The account's earlier unused links of the same purpose stop working,
// so only the newest one sent does.
func IssueLink(ctx context.Context, db *sql.DB, uid uint64, p LinkPurpose, email string,
	ttl time.Duration) (string, error) {
	token, err := LinkToken()
	if err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := endLinks(ctx, tx, uid, p); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO web_email_links (web_user_id, purpose, token_hash, email, expires_at) "+
			"VALUES (?, ?, ?, ?, NOW() + INTERVAL ? SECOND)",
		uid, string(p), tokenHash(token), email, int64(ttl/time.Second)); err != nil {
		return "", err
	}
	return token, tx.Commit()
}

// endLinks ends the account's unused links of purpose p.
func endLinks(ctx context.Context, db Execer, uid uint64, p LinkPurpose) error {
	_, err := db.ExecContext(ctx, "UPDATE web_email_links SET used_at = NOW() "+
		"WHERE web_user_id = ? AND purpose = ? AND used_at IS NULL", uid, string(p))
	return err
}

// Link is a link being used.
type Link struct {
	ID    uint64
	UID   uint64
	Email string // the address it was sent to
}

// useLink takes the link of token for purpose p in tx: it must exist,
// be unused, unexpired and belong to an active account. It is marked
// used in the same transaction, so two requests with one token cannot
// both pass.
func useLink(ctx context.Context, tx *sql.Tx, token string, p LinkPurpose) (Link, error) {
	var (
		l  Link
		ok bool
	)
	err := tx.QueryRowContext(ctx,
		"SELECT l.id, l.web_user_id, l.email, "+
			"l.used_at IS NULL AND l.expires_at > NOW() AND u.is_active = 1 "+
			"FROM web_email_links l JOIN web_users u ON u.id = l.web_user_id "+
			"WHERE l.token_hash = ? AND l.purpose = ? FOR UPDATE",
		tokenHash(token), string(p)).Scan(&l.ID, &l.UID, &l.Email, &ok)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !ok {
		return Link{}, ErrBadLink
	}
	if err != nil {
		return Link{}, err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE web_email_links SET used_at = NOW() WHERE id = ?", l.ID); err != nil {
		return Link{}, err
	}
	return l, nil
}

// PasswordSet is what SetPasswordFromLink did.
type PasswordSet struct {
	UID      uint64
	Username string
	// First: the account had never signed in, so this was its
	// invitation, not a reset.
	First bool
	// Email is the account's address now, Verified whether it is
	// verified (it is when the link went to it).
	Email    string
	Verified bool
}

// SetPasswordFromLink uses a set-password link: the account's password
// becomes hash, a forced change ends, every session ends (the epoch
// moves on), the address the link went to is verified if it is still
// the account's, and the account's other set-password links stop
// working. ErrBadLink when the link does not work.
func SetPasswordFromLink(ctx context.Context, db *sql.DB, token, hash, ip string) (PasswordSet, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PasswordSet{}, err
	}
	defer tx.Rollback()
	l, err := useLink(ctx, tx, token, LinkPassword)
	if err != nil {
		return PasswordSet{}, err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE web_users SET password_hash = ?, must_change_password = 0, "+
			"session_epoch = session_epoch + 1, "+
			"email_verified_at = IF(email = ?, COALESCE(email_verified_at, NOW()), email_verified_at) "+
			"WHERE id = ?", hash, l.Email, l.UID); err != nil {
		return PasswordSet{}, err
	}
	if err := endLinks(ctx, tx, l.UID, LinkPassword); err != nil {
		return PasswordSet{}, err
	}
	var r PasswordSet
	err = tx.QueryRowContext(ctx,
		"SELECT id, username, COALESCE(email, ''), email_verified_at IS NOT NULL, "+
			"NOT EXISTS (SELECT 1 FROM web_audit a WHERE a.web_user_id = u.id "+
			"AND a.action = 'login_ok') FROM web_users u WHERE id = ?", l.UID).
		Scan(&r.UID, &r.Username, &r.Email, &r.Verified, &r.First)
	if err != nil {
		return PasswordSet{}, err
	}
	if err := Audit(ctx, tx, &l.UID, "password_set_link", ip, r.Username); err != nil {
		return PasswordSet{}, err
	}
	return r, tx.Commit()
}

// EmailVerified is what VerifyEmailFromLink did.
type EmailVerified struct {
	UID      uint64
	Username string
	Email    string // the address now verified
	// Old is the address it replaced, "" when it was the same or
	// none; OldVerified whether that one was verified.
	Old         string
	OldVerified bool
}

// VerifyEmailFromLink uses a verification link: the address it went to
// becomes the account's, verified, and the account's other
// verification links stop working. ErrBadLink when the link does not
// work, ErrEmailTaken when another account has the address meanwhile
// (the link is then used up all the same).
func VerifyEmailFromLink(ctx context.Context, db *sql.DB, token, ip string) (EmailVerified, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return EmailVerified{}, err
	}
	defer tx.Rollback()
	l, err := useLink(ctx, tx, token, LinkVerify)
	if err != nil {
		return EmailVerified{}, err
	}
	r := EmailVerified{UID: l.UID, Email: l.Email}
	if err := tx.QueryRowContext(ctx,
		"SELECT username, COALESCE(email, ''), email_verified_at IS NOT NULL "+
			"FROM web_users WHERE id = ? FOR UPDATE", l.UID).
		Scan(&r.Username, &r.Old, &r.OldVerified); err != nil {
		return EmailVerified{}, err
	}
	if strings.EqualFold(r.Old, l.Email) {
		r.Old, r.OldVerified = "", false
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE web_users SET email = ?, email_verified_at = NOW() WHERE id = ?",
		l.Email, l.UID); err != nil {
		if errors.Is(duplicate(err), ErrEmailTaken) {
			// Keep the link used: it can never work now.
			if _, err := tx.ExecContext(ctx,
				"UPDATE web_email_links SET used_at = NOW() WHERE id = ?", l.ID); err != nil {
				return EmailVerified{}, err
			}
			if err := tx.Commit(); err != nil {
				return EmailVerified{}, err
			}
			return EmailVerified{}, ErrEmailTaken
		}
		return EmailVerified{}, err
	}
	if err := endLinks(ctx, tx, l.UID, LinkVerify); err != nil {
		return EmailVerified{}, err
	}
	if err := Audit(ctx, tx, &l.UID, "email_verified", ip, r.Username); err != nil {
		return EmailVerified{}, err
	}
	return r, tx.Commit()
}

// PendingEmail is the address of the account's newest verification
// link still working, if any.
func PendingEmail(ctx context.Context, db *sql.DB, uid uint64) (*string, error) {
	var e string
	err := db.QueryRowContext(ctx,
		"SELECT email FROM web_email_links WHERE web_user_id = ? AND purpose = 'verify' "+
			"AND used_at IS NULL AND expires_at > NOW() ORDER BY id DESC LIMIT 1", uid).Scan(&e)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// EmailInUse is whether an account other than uid has the address.
func EmailInUse(ctx context.Context, db *sql.DB, email string, uid uint64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM web_users WHERE email = ? AND id <> ?", email, uid).Scan(&n)
	return n > 0, err
}

// FindForReset is the active account with a verified address that a
// forgotten-password request names: by its address when the name has
// an @, else by its username. ErrNoUser when there is none, which the
// caller does not tell apart from a link sent.
func FindForReset(ctx context.Context, db *sql.DB, name string) (Account, error) {
	col := "u.username"
	if strings.Contains(name, "@") {
		col = "u.email"
	}
	a, err := scanAccount(db.QueryRowContext(ctx, accountColumns+"WHERE "+col+" = ? "+
		"AND u.is_active = 1 AND u.email_verified_at IS NOT NULL", name))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNoUser
	}
	return a, err
}

// NoteDevice records a sign-in from a device and reports whether the
// device is new to an account that had signed in from others before.
// The first device an account signs in from is not news.
func NoteDevice(ctx context.Context, db *sql.DB, uid uint64, device string) (bool, error) {
	h := sha256.Sum256([]byte(device))
	var known, same int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(SUM(device_hash = ?), 0) FROM web_known_devices "+
			"WHERE web_user_id = ?", h[:], uid).Scan(&known, &same); err != nil {
		return false, err
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO web_known_devices (web_user_id, device_hash) VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE last_seen_at = NOW()", uid, h[:]); err != nil {
		return false, err
	}
	return known > 0 && same == 0, nil
}

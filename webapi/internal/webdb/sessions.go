// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// The record of each session cookie, web_sessions
// (decisions/2026-11-18-sessions-list.md). A session is its cookie: the
// key is the SHA-256 of the cookie value, so the cookie format both web
// services read stays as it is. Both services refuse a cookie whose
// row is ended (revoked_at set); rows of an older session_epoch are
// dead through the epoch already and are pruned.

// TouchInterval is how stale a session's last use may get before a
// request records it again, so a busy session writes once a minute,
// not on every request.
const TouchInterval = time.Minute

// maxUserAgent and maxIP are the lengths of their columns.
const (
	maxUserAgent = 512
	maxIP        = 45
)

// ErrSessionExists: a session with this key is recorded already.
var ErrSessionExists = errors.New("session key exists")

// NewSession is a session to record.
type NewSession struct {
	UID       uint64
	Key       [32]byte
	Epoch     uint32
	Expires   int64 // the cookie's expiry, Unix seconds; 0 = never
	UserAgent string
	IP        string
	// SignedIn is when the session started; zero is now.
	SignedIn time.Time
	// From is the row of the session this one replaces (a cookie
	// reissued after a password change), whose start and device it
	// keeps; 0 for a new sign-in.
	From uint64
}

func fit(s string, n int) string {
	s = strings.ToValidUTF8(s, "�")
	c := 0
	for i := range s {
		if c == n {
			return s[:i]
		}
		c++
	}
	return s
}

func signedIn(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Unix()
}

// RecordSession records a session the service signs in. It answers
// ErrSessionExists when the key is taken, so the caller can make
// another cookie.
func RecordSession(ctx context.Context, db *sql.DB, n NewSession) error {
	ua, ip := fit(n.UserAgent, maxUserAgent), fit(n.IP, maxIP)
	var (
		res sql.Result
		err error
	)
	if n.From != 0 {
		res, err = db.ExecContext(ctx,
			"INSERT INTO web_sessions (web_user_id, token_hash, epoch, expires, "+
				"user_agent, ip, signed_in_at) SELECT ?, ?, ?, ?, user_agent, ?, "+
				"signed_in_at FROM web_sessions WHERE id = ? AND web_user_id = ?",
			n.UID, n.Key[:], n.Epoch, n.Expires, ip, n.From, n.UID)
		if err == nil {
			if k, _ := res.RowsAffected(); k == 1 {
				return nil
			}
		}
	}
	if err == nil {
		_, err = db.ExecContext(ctx,
			"INSERT INTO web_sessions (web_user_id, token_hash, epoch, expires, "+
				"user_agent, ip, signed_in_at) VALUES (?, ?, ?, ?, ?, ?, "+
				"COALESCE(FROM_UNIXTIME(?), NOW()))",
			n.UID, n.Key[:], n.Epoch, n.Expires, ua, ip, signedIn(n.SignedIn))
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return ErrSessionExists
	}
	return err
}

// SeenSession records a valid session the first time the service sees
// it, one signed in before sessions were recorded or by the Drogon
// web. A session recorded meanwhile, by a request running at the same
// time, is left as it is.
func SeenSession(ctx context.Context, db *sql.DB, n NewSession) error {
	_, err := db.ExecContext(ctx,
		"INSERT IGNORE INTO web_sessions (web_user_id, token_hash, epoch, expires, "+
			"user_agent, ip, signed_in_at) VALUES (?, ?, ?, ?, ?, ?, "+
			"LEAST(COALESCE(FROM_UNIXTIME(?), NOW()), NOW()))",
		n.UID, n.Key[:], n.Epoch, n.Expires, fit(n.UserAgent, maxUserAgent),
		fit(n.IP, maxIP), signedIn(n.SignedIn))
	return err
}

// TouchSession records a use of the session: now, from ip.
func TouchSession(ctx context.Context, db *sql.DB, row uint64, ip string) error {
	_, err := db.ExecContext(ctx,
		"UPDATE web_sessions SET last_used_at = NOW(), ip = ? WHERE id = ?",
		fit(ip, maxIP), row)
	return err
}

// EndSessionKey ends the session with this cookie key (a sign-out),
// recording it ended when it had no row yet, so the cookie stops
// working on both services even if it was copied.
func EndSessionKey(ctx context.Context, db *sql.DB, n NewSession) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO web_sessions (web_user_id, token_hash, epoch, expires, "+
			"user_agent, ip, revoked_at) VALUES (?, ?, ?, ?, ?, ?, NOW()) "+
			"ON DUPLICATE KEY UPDATE revoked_at = COALESCE(revoked_at, NOW())",
		n.UID, n.Key[:], n.Epoch, n.Expires, fit(n.UserAgent, maxUserAgent),
		fit(n.IP, maxIP))
	return err
}

// EndSession ends one of the account's live sessions by its row id.
// It reports false when the account has no such live session.
func EndSession(ctx context.Context, db *sql.DB, uid, row uint64) (bool, error) {
	res, err := db.ExecContext(ctx,
		"UPDATE web_sessions s JOIN web_users u ON u.id = s.web_user_id "+
			"SET s.revoked_at = NOW() WHERE s.id = ? AND s.web_user_id = ? "+
			"AND s.revoked_at IS NULL AND s.epoch = u.session_epoch "+
			"AND (s.expires = 0 OR s.expires > UNIX_TIMESTAMP())", row, uid)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// PruneSessions removes the account's rows that can never be used
// again: those of an older epoch than epoch, and expired ones.
func PruneSessions(ctx context.Context, db *sql.DB, uid uint64, epoch uint32) error {
	_, err := db.ExecContext(ctx,
		"DELETE FROM web_sessions WHERE web_user_id = ? AND (epoch <> ? "+
			"OR (expires <> 0 AND expires <= UNIX_TIMESTAMP()))", uid, epoch)
	return err
}

// SessionRow is a live session as the sessions page lists it.
type SessionRow struct {
	ID        uint64
	Key       [32]byte
	UserAgent string
	IP        string
	SignedIn  time.Time
	LastUsed  time.Time
}

// ListSessions is the account's live sessions (of its current epoch,
// not ended, not expired), the most recently used first. It prunes the
// dead ones first.
func ListSessions(ctx context.Context, db *sql.DB, uid uint64) ([]SessionRow, error) {
	var epoch uint32
	err := db.QueryRowContext(ctx, "SELECT session_epoch FROM web_users WHERE id = ?", uid).
		Scan(&epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoUser
	}
	if err != nil {
		return nil, err
	}
	if err := PruneSessions(ctx, db, uid, epoch); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, token_hash, user_agent, ip, UNIX_TIMESTAMP(signed_in_at), "+
			"UNIX_TIMESTAMP(last_used_at) FROM web_sessions "+
			"WHERE web_user_id = ? AND epoch = ? AND revoked_at IS NULL "+
			"ORDER BY last_used_at DESC, id DESC", uid, epoch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionRow{}
	for rows.Next() {
		var (
			r        SessionRow
			key      []byte
			in, last float64
		)
		if err := rows.Scan(&r.ID, &key, &r.UserAgent, &r.IP, &in, &last); err != nil {
			return nil, err
		}
		copy(r.Key[:], key)
		r.SignedIn = time.Unix(int64(in), 0).UTC()
		r.LastUsed = time.Unix(int64(last), 0).UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

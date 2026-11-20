// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// Filing joined groups into federations (join-chat-federations, web
// migration 000017). A group joined from the Join chat page goes into
// the federations its admin chose; when the chat must approve the join
// first, the choice is held in web_join_filings until the account is a
// member. Who may choose which federation is the caller's, from the
// request scope; a held filing is checked again when it is settled,
// since the account may have lost the federation meanwhile.

// JoinFiling is a held filing.
type JoinFiling struct {
	ID       uint64
	By       uint64 // the account that asked
	ByName   string // its username
	FedID    uint64
	FedName  string
	Ref      string
	ChatID   int64 // 0 when unknown
	Title    string
	MsgCount uint64 // the group's msg_count when it was asked
	At       time.Time
}

// realFeds is ids without Default (whose groups are derived) and
// repeats, ascending.
func realFeds(ids []uint64) []uint64 {
	var out []uint64
	for _, f := range uniq(ids) {
		if f != DefaultFederation {
			out = append(out, f)
		}
	}
	return out
}

// fileGroup adds group to federation fed for actor and audits it, if
// actor still may file into fed (a superadmin, or an admin of fed) and
// fed exists. It reports whether the group is in fed afterwards.
func fileGroup(ctx context.Context, tx *sql.Tx, actor, fed uint64, group int64, ip,
	why string) (bool, error) {
	var may int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM web_users u "+
		"JOIN web_federations f ON f.id = ? "+
		"WHERE u.id = ? AND u.is_active = 1 AND (u.role = 'superadmin' OR (u.role = 'admin' AND "+
		"EXISTS (SELECT 1 FROM web_federation_admins fa WHERE fa.federation_id = f.id "+
		"AND fa.web_user_id = u.id))) FOR SHARE", fed, actor).Scan(&may); err != nil {
		return false, err
	}
	if may == 0 {
		return false, nil
	}
	res, err := tx.ExecContext(ctx, "INSERT IGNORE INTO web_federation_groups "+
		"(federation_id, group_id) VALUES (?, ?)", fed, group)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err == nil, err
	}
	// As the federation editor does: the group is no longer in Default,
	// so the members' Default grants of it go.
	if err := dropGrantsOfGroups(ctx, tx, fed, []int64{group}, nil); err != nil {
		return false, err
	}
	return true, Audit(ctx, tx, &actor, "federation_update", ip, "federation "+
		strconv.FormatUint(fed, 10)+"; groups +1 -0; "+why+" "+strconv.FormatInt(group, 10))
}

// FileJoinedGroup files group, just joined by actor, into the
// federations feds (Default and repeats skipped) and returns those it
// is in afterwards, ascending. A federation actor may not file into is
// skipped; the caller checked them before the join.
func FileJoinedGroup(ctx context.Context, db *sql.DB, actor uint64, feds []uint64, group int64,
	ip string) ([]uint64, error) {
	feds = realFeds(feds)
	out := []uint64{}
	if len(feds) == 0 {
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, f := range feds {
		ok, err := fileGroup(ctx, tx, actor, f, group, ip, "joined")
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, f)
		}
	}
	return out, tx.Commit()
}

// HoldJoinFilings records that the chat behind ref goes into feds
// once the account is a member, and audits it (chat_join_hold). A
// federation already held for ref keeps its first request. It returns
// the federations held, ascending.
func HoldJoinFilings(ctx context.Context, db *sql.DB, actor uint64, feds []uint64, ref string,
	chatID int64, title string, msgCount uint64, ip string) ([]uint64, error) {
	feds = realFeds(feds)
	out := []uint64{}
	if len(feds) == 0 {
		return out, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := checkFederations(ctx, tx, feds); err != nil {
		return nil, err
	}
	title = clipText(title, 255)
	for _, f := range feds {
		if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO web_join_filings (web_user_id, "+
			"federation_id, ref, chat_id, title, msg_count) VALUES (?, ?, ?, ?, ?, ?)",
			actor, f, ref, chatID, title, msgCount); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	detail := ref + " federations " + idsText(out)
	if chatID != 0 {
		detail += " chat=" + strconv.FormatInt(chatID, 10)
	}
	if err := Audit(ctx, tx, &actor, "chat_join_hold", ip, detail); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

// joinFilingColumns are what scanJoinFilings reads, from
// web_join_filings j with its federation f and account u.
const joinFilingColumns = "j.id, j.web_user_id, COALESCE(u.username, ''), j.federation_id, " +
	"COALESCE(f.name, ''), j.ref, j.chat_id, j.title, j.msg_count, UNIX_TIMESTAMP(j.created_at) " +
	"FROM web_join_filings j LEFT JOIN web_users u ON u.id = j.web_user_id " +
	"LEFT JOIN web_federations f ON f.id = j.federation_id "

func scanJoinFilings(rows *sql.Rows, err error) ([]JoinFiling, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []JoinFiling{}
	for rows.Next() {
		var j JoinFiling
		var at float64
		if err := rows.Scan(&j.ID, &j.By, &j.ByName, &j.FedID, &j.FedName, &j.Ref, &j.ChatID,
			&j.Title, &j.MsgCount, &at); err != nil {
			return nil, err
		}
		j.At = time.Unix(int64(at), 0).UTC()
		out = append(out, j)
	}
	return out, rows.Err()
}

// ListJoinFilings is the held filings, oldest first: every one, or
// only those of account by when it is not nil.
func ListJoinFilings(ctx context.Context, db Querier, by *uint64) ([]JoinFiling, error) {
	if by == nil {
		return scanJoinFilings(db.QueryContext(ctx, "SELECT "+joinFilingColumns+"ORDER BY j.id"))
	}
	return scanJoinFilings(db.QueryContext(ctx, "SELECT "+joinFilingColumns+
		"WHERE j.web_user_id = ? ORDER BY j.id", *by))
}

// KnownJoinFilings is the held filings whose chat is known: those the
// background sweep can settle from the logger's counters.
func KnownJoinFilings(ctx context.Context, db Querier) ([]JoinFiling, error) {
	return scanJoinFilings(db.QueryContext(ctx, "SELECT "+joinFilingColumns+
		"WHERE j.chat_id <> 0 ORDER BY j.id"))
}

// SettleJoinFilings files the chat chatID, now that the account is a
// member, into the federations held for it: the filings named by ids,
// and every one held for ref or for chatID when ids is nil. Each
// settled filing is dropped, also one whose account may no longer
// file into its federation (audited as chat_join_unheld). It returns
// how many groups were filed.
func SettleJoinFilings(ctx context.Context, db *sql.DB, ids []uint64, ref string, chatID int64,
	ip string) (int, error) {
	if chatID >= 0 || (ids != nil && len(ids) == 0) {
		return 0, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var held []JoinFiling
	if ids != nil {
		ph, args := placeholders(ids)
		held, err = scanJoinFilings(tx.QueryContext(ctx, "SELECT "+joinFilingColumns+
			"WHERE j.id IN ("+ph+") ORDER BY j.id FOR UPDATE OF j", args...))
	} else {
		held, err = scanJoinFilings(tx.QueryContext(ctx, "SELECT "+joinFilingColumns+
			"WHERE j.ref = ? OR j.chat_id = ? ORDER BY j.id FOR UPDATE OF j", ref, chatID))
	}
	if err != nil || len(held) == 0 {
		return 0, err
	}
	n := 0
	for _, j := range held {
		filed, err := fileGroup(ctx, tx, j.By, j.FedID, chatID, ip, "approved")
		if err != nil {
			return 0, err
		}
		if filed {
			n++
		} else if err := Audit(ctx, tx, &j.By, "chat_join_unheld", ip, j.Ref+" federation "+
			strconv.FormatUint(j.FedID, 10)+" chat="+strconv.FormatInt(chatID, 10)+
			" no longer the account's"); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM web_join_filings WHERE id = ?", j.ID); err != nil {
			return 0, err
		}
	}
	return n, tx.Commit()
}

// CancelJoinFiling drops held filing id, if it is held and, when by is
// not nil, account by's, and audits it (chat_join_unheld). It reports
// whether there was one.
func CancelJoinFiling(ctx context.Context, db *sql.DB, actor uint64, id uint64, by *uint64,
	ip string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	held, err := scanJoinFilings(tx.QueryContext(ctx, "SELECT "+joinFilingColumns+
		"WHERE j.id = ? FOR UPDATE OF j", id))
	if err != nil || len(held) == 0 || (by != nil && held[0].By != *by) {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM web_join_filings WHERE id = ?", id); err != nil {
		return false, err
	}
	if err := Audit(ctx, tx, &actor, "chat_join_unheld", ip, held[0].Ref+" federation "+
		strconv.FormatUint(held[0].FedID, 10)+" cancelled"); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// clipText cuts s to n characters, never inside one.
func clipText(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

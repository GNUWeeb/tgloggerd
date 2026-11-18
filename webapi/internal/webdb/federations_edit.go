// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// The federations' own reads and writes: the superadmins' federation
// editor and the pages that list federations.

var (
	// ErrFederationTaken: another federation has the name.
	ErrFederationTaken = errors.New("federation name taken")
	// ErrNotAdmin: an account named as a federation's admin is not an
	// admin.
	ErrNotAdmin = errors.New("not an admin")
	// ErrDefaultFixed: the change would rename the Default federation
	// or set its groups, which are derived.
	ErrDefaultFixed = errors.New("the Default federation's name and groups are fixed")
)

// FedPerson is an admin or member of a federation.
type FedPerson struct {
	ID          uint64
	Username    string
	DisplayName string
}

// Federation is a web_federations row with its stored groups, admins
// and members. Default has no stored groups.
type Federation struct {
	ID        uint64
	Name      string
	CreatedAt *time.Time // nil for Default
	Groups    []int64    // ascending; never for Default
	Admins    []FedPerson
	Members   []FedPerson
}

// Default is whether f is the Default federation.
func (f Federation) Default() bool { return f.ID == DefaultFederation }

// ListFederations is every federation, Default first, then by name,
// with their groups, admins and members (active or not, by username).
func ListFederations(ctx context.Context, db Querier) ([]Federation, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, UNIX_TIMESTAMP(created_at) "+
		"FROM web_federations ORDER BY id <> 0, name, id")
	if err != nil {
		return nil, err
	}
	var out []Federation
	at := map[uint64]int{}
	for rows.Next() {
		var (
			f       Federation
			created sql.NullFloat64
		)
		if err := rows.Scan(&f.ID, &f.Name, &created); err != nil {
			rows.Close()
			return nil, err
		}
		if created.Valid && f.ID != DefaultFederation {
			t := time.Unix(int64(created.Float64), 0).UTC()
			f.CreatedAt = &t
		}
		at[f.ID] = len(out)
		out = append(out, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.QueryContext(ctx, "SELECT federation_id, group_id FROM web_federation_groups "+
		"ORDER BY federation_id, group_id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			fid uint64
			gid int64
		)
		if err := rows.Scan(&fid, &gid); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := at[fid]; ok && fid != DefaultFederation {
			out[i].Groups = append(out[i].Groups, gid)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, part := range []struct {
		table string
		admin bool
	}{{"web_federation_admins", true}, {"web_federation_members", false}} {
		rows, err := db.QueryContext(ctx, "SELECT x.federation_id, u.id, u.username, "+
			"u.display_name FROM "+part.table+" x JOIN web_users u ON u.id = x.web_user_id "+
			"ORDER BY x.federation_id, u.username")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				fid uint64
				p   FedPerson
			)
			if err := rows.Scan(&fid, &p.ID, &p.Username, &p.DisplayName); err != nil {
				rows.Close()
				return nil, err
			}
			if i, ok := at[fid]; ok {
				if part.admin {
					out[i].Admins = append(out[i].Admins, p)
				} else {
					out[i].Members = append(out[i].Members, p)
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FederationWrite is a federation's new state; nil fields stay.
// Groups are never Default's.
type FederationWrite struct {
	Name   *string
	Groups *[]int64
	Admins *[]uint64 // web_users.id of admins
}

// fedName maps a duplicate-name insert or update to ErrFederationTaken.
func fedName(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 && strings.Contains(me.Message, "uq_web_federations_name") {
		return ErrFederationTaken
	}
	return err
}

// errNextID: another creation took the id first, or the two deadlocked
// over it; the creation is tried again.
var errNextID = errors.New("federation id taken meanwhile")

// CreateFederation makes a federation with the next id, MAX(id) + 1,
// and audits it, in one transaction. Two creations at once may pick
// the same id, or deadlock over the end of the table; the loser is
// rolled back and tried again with the next id.
func CreateFederation(ctx context.Context, db *sql.DB, actor uint64, w FederationWrite,
	ip string) (uint64, error) {
	if w.Name == nil {
		return 0, errors.New("a new federation needs a name")
	}
	for try := 0; ; try++ {
		id, err := createFederation(ctx, db, actor, w, ip)
		if !errors.Is(err, errNextID) || try == 9 {
			return id, err
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Duration(5+mrand.IntN(20*(try+1))) * time.Millisecond):
		}
	}
}

func createFederation(ctx context.Context, db *sql.DB, actor uint64, w FederationWrite,
	ip string) (uint64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id uint64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM web_federations").
		Scan(&id); err != nil {
		return 0, nextID(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO web_federations (id, name, created_by) "+
		"VALUES (?, ?, ?)", id, *w.Name, actor); err != nil {
		return 0, nextID(fedName(err))
	}
	detail, err := writeFederation(ctx, tx, id, w, nil)
	if err != nil {
		return 0, nextID(err)
	}
	if err := Audit(ctx, tx, &actor, "federation_create", ip,
		"federation "+strconv.FormatUint(id, 10)+" "+*w.Name+detail); err != nil {
		return 0, nextID(err)
	}
	return id, nextID(tx.Commit())
}

// nextID maps a duplicate id or a deadlock to errNextID.
func nextID(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && (me.Number == 1213 || me.Number == 1062 && strings.Contains(me.Message, "PRIMARY")) {
		return errNextID
	}
	return err
}

// UpdateFederation changes a federation and audits what changed, in
// one transaction; ErrNoFederation when there is none. It reports
// whether anything changed. Default's groups cannot be set.
func UpdateFederation(ctx context.Context, db *sql.DB, actor, id uint64, w FederationWrite,
	ip string) (bool, error) {
	if id == DefaultFederation && w.Groups != nil {
		return false, ErrDefaultFixed
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRowContext(ctx, "SELECT name FROM web_federations WHERE id = ? FOR UPDATE", id).
		Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNoFederation
	}
	if err != nil {
		return false, err
	}
	var detail string
	if w.Name != nil && *w.Name != name {
		if id == DefaultFederation {
			return false, ErrDefaultFixed
		}
		if _, err := tx.ExecContext(ctx, "UPDATE web_federations SET name = ? WHERE id = ?",
			*w.Name, id); err != nil {
			return false, fedName(err)
		}
		detail = "; name " + name + "->" + *w.Name
	}
	more, err := writeFederation(ctx, tx, id, w, &name)
	if err != nil {
		return false, err
	}
	detail += more
	if detail == "" {
		return false, nil
	}
	if err := Audit(ctx, tx, &actor, "federation_update", ip,
		"federation "+strconv.FormatUint(id, 10)+detail); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// writeFederation sets the groups and admins given and returns the
// audit detail of what changed ("; groups ...; admins ..."). old is
// nil for a new federation.
func writeFederation(ctx context.Context, tx *sql.Tx, id uint64, w FederationWrite,
	old *string) (string, error) {
	var detail string
	if w.Groups != nil {
		cur, err := int64s(ctx, tx, "SELECT group_id FROM web_federation_groups "+
			"WHERE federation_id = ? ORDER BY group_id FOR UPDATE", id)
		if err != nil {
			return "", err
		}
		next := uniq(*w.Groups)
		var added, removed []int64
		for _, g := range next {
			if !slices.Contains(cur, g) {
				added = append(added, g)
			}
		}
		for _, g := range cur {
			if !slices.Contains(next, g) {
				removed = append(removed, g)
			}
		}
		if len(removed) > 0 {
			ph, args := placeholders(removed)
			if _, err := tx.ExecContext(ctx, "DELETE FROM web_federation_groups "+
				"WHERE federation_id = ? AND group_id IN ("+ph+")",
				append([]any{id}, args...)...); err != nil {
				return "", err
			}
		}
		for _, g := range added {
			if _, err := tx.ExecContext(ctx, "INSERT INTO web_federation_groups "+
				"(federation_id, group_id) VALUES (?, ?)", id, g); err != nil {
				return "", err
			}
		}
		if len(added)+len(removed) > 0 || old == nil {
			detail += fmt.Sprintf("; groups +%d -%d", len(added), len(removed))
		}
	}
	if w.Admins != nil {
		next := uniq(*w.Admins)
		if len(next) > 0 {
			ph, args := placeholders(next)
			var n int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT id FROM web_users "+
				"WHERE id IN ("+ph+") AND role = 'admin' FOR SHARE) u", args...).Scan(&n); err != nil {
				return "", err
			}
			if n != len(next) {
				return "", ErrNotAdmin
			}
		}
		cur, err := uint64s(ctx, tx, "SELECT web_user_id FROM web_federation_admins "+
			"WHERE federation_id = ? ORDER BY web_user_id FOR UPDATE", id)
		if err != nil {
			return "", err
		}
		changed := false
		for _, u := range cur {
			if !slices.Contains(next, u) {
				changed = true
				if _, err := tx.ExecContext(ctx, "DELETE FROM web_federation_admins "+
					"WHERE federation_id = ? AND web_user_id = ?", id, u); err != nil {
					return "", err
				}
			}
		}
		for _, u := range next {
			if !slices.Contains(cur, u) {
				changed = true
				if _, err := tx.ExecContext(ctx, "INSERT INTO web_federation_admins "+
					"(federation_id, web_user_id) VALUES (?, ?)", id, u); err != nil {
					return "", err
				}
			}
		}
		if changed || old == nil {
			detail += "; admins " + idsText(cur) + "->" + idsText(next)
		}
	}
	return detail, nil
}

// DeleteFederation removes a federation, its groups (which fall back
// to Default, unless another federation holds them) and its admins.
// Its members stay in their other federations; a member it was the
// only one of is filed in Default, so it stays managed by someone
// below the superadmins. Default cannot be deleted.
func DeleteFederation(ctx context.Context, db *sql.DB, actor, id uint64, ip string) error {
	if id == DefaultFederation {
		return ErrDefaultFixed
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRowContext(ctx, "SELECT name FROM web_federations WHERE id = ? FOR UPDATE", id).
		Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoFederation
	}
	if err != nil {
		return err
	}
	moved, err := uint64s(ctx, tx, "SELECT m.web_user_id FROM web_federation_members m "+
		"WHERE m.federation_id = ? AND NOT EXISTS (SELECT 1 FROM web_federation_members o "+
		"WHERE o.web_user_id = m.web_user_id AND o.federation_id <> m.federation_id) "+
		"ORDER BY m.web_user_id FOR UPDATE", id)
	if err != nil {
		return err
	}
	for _, u := range moved {
		if _, err := tx.ExecContext(ctx, "INSERT INTO web_federation_members "+
			"(federation_id, web_user_id) VALUES (?, ?)", DefaultFederation, u); err != nil {
			return err
		}
	}
	// The groups, admins and members go with it (ON DELETE CASCADE).
	if _, err := tx.ExecContext(ctx, "DELETE FROM web_federations WHERE id = ?", id); err != nil {
		return err
	}
	detail := "federation " + strconv.FormatUint(id, 10) + " " + name
	if len(moved) > 0 {
		detail += "; members to Default " + idsText(moved)
	}
	if err := Audit(ctx, tx, &actor, "federation_delete", ip, detail); err != nil {
		return err
	}
	return tx.Commit()
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Federations (docs/design/web-accounts.md section 3, web migration
// 000006): named sets of groups with admins and members assigned. The
// Default federation is the row with id DefaultFederation; its groups
// are derived, the groups in no other federation, and never stored.
// Who may change what is the caller's, from the request scope; these
// keep the tables consistent and audit every change.

// DefaultFederation is the Default federation's id (scope's
// DefaultFederation).
const DefaultFederation uint64 = 0

// ErrNoFederation: a federation named does not exist.
var ErrNoFederation = errors.New("no such federation")

// Querier is a pool or a transaction.
type Querier interface {
	Execer
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// The federations of an account, as JSON arrays (JSON_ARRAYAGG, which
// unlike GROUP_CONCAT is not cut at group_concat_max_len), NULL for
// none. They follow a column list and end with a space.
const (
	adminOfColumn = "(SELECT JSON_ARRAYAGG(fa.federation_id) FROM web_federation_admins fa " +
		"WHERE fa.web_user_id = u.id) "
	memberOfColumn = "(SELECT JSON_ARRAYAGG(fm.federation_id) FROM web_federation_members fm " +
		"WHERE fm.web_user_id = u.id) "
	// hiddenGroupsColumn is an admin's hidden groups (groupList).
	hiddenGroupsColumn = "(SELECT JSON_ARRAYAGG(hg.group_id) FROM web_admin_hidden_groups hg " +
		"WHERE hg.web_user_id = u.id) "
)

// groupList reads hiddenGroupsColumn or grantedGroupsColumn: the
// (negative) group ids, ascending and each once, nil for NULL.
func groupList(s sql.NullString) ([]int64, error) {
	if !s.Valid {
		return nil, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(s.String), &ids); err != nil {
		return nil, fmt.Errorf("group ids %q: %w", s.String, err)
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	return slices.Compact(ids), nil
}

// JoinGroupIDs writes group ids for a Session ("" for none), as
// JoinIDs does federations.
func JoinGroupIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, v := range ids {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(parts, ",")
}

// SplitGroupIDs reads a JoinGroupIDs list back, nil for "".
func SplitGroupIDs(s string) []int64 {
	var out []int64
	for _, p := range strings.Split(s, ",") {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

// idList reads an id column of adminOfColumn's kind: ascending, nil
// for NULL.
func idList(s sql.NullString) ([]uint64, error) {
	if !s.Valid {
		return nil, nil
	}
	var ids []uint64
	if err := json.Unmarshal([]byte(s.String), &ids); err != nil {
		return nil, fmt.Errorf("federation ids %q: %w", s.String, err)
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	return slices.Compact(ids), nil
}

// FederatedGroups is every group stored in a federation (each once,
// ascending): the groups that are not in Default.
func FederatedGroups(ctx context.Context, db Querier) ([]int64, error) {
	return int64s(ctx, db, "SELECT DISTINCT group_id FROM web_federation_groups ORDER BY group_id")
}

// FederationGroups is the stored groups of one federation, ascending,
// or ErrNoFederation. Default has none stored (see FederatedGroups).
func FederationGroups(ctx context.Context, db Querier, id uint64) ([]int64, error) {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM web_federations WHERE id = ?", id).
		Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNoFederation
	}
	return int64s(ctx, db, "SELECT group_id FROM web_federation_groups "+
		"WHERE federation_id = ? ORDER BY group_id", id)
}

func int64s(ctx context.Context, db Querier, q string, args ...any) ([]int64, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// JoinIDs writes ids as a comma-separated list, "" for none.
func JoinIDs(ids []uint64) string {
	parts := make([]string, len(ids))
	for i, v := range ids {
		parts[i] = strconv.FormatUint(v, 10)
	}
	return strings.Join(parts, ",")
}

// SplitIDs reads a JoinIDs list back, nil for "", skipping what is
// not an id.
func SplitIDs(s string) []uint64 {
	var out []uint64
	for _, p := range strings.Split(s, ",") {
		if v, err := strconv.ParseUint(p, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

// placeholders is "?, ?, ..." for n values, with the values as args.
func placeholders[T any](vs []T) (string, []any) {
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?, ", len(vs)), ", "), args
}

// checkFederations returns ErrNoFederation unless every id exists. It
// locks the rows it reads (FOR SHARE), so a federation cannot be
// deleted before the caller's transaction commits.
func checkFederations(ctx context.Context, tx *sql.Tx, ids []uint64) error {
	ids = uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	ph, args := placeholders(ids)
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT id FROM web_federations "+
		"WHERE id IN ("+ph+") FOR SHARE) f", args...).Scan(&n); err != nil {
		return err
	}
	if n != len(ids) {
		return ErrNoFederation
	}
	return nil
}

func uniq[T int64 | uint64](vs []T) []T {
	out := slices.Clone(vs)
	slices.Sort(out)
	return slices.Compact(out)
}

func idsText[T int64 | uint64](vs []T) string {
	if len(vs) == 0 {
		return "none"
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.FormatInt(int64(v), 10)
	}
	return strings.Join(parts, ",")
}

// FederationChange sets an account's federations (as an admin or as a
// member): the account's current ones among Within, or all of them
// when WithinAll, are replaced by Set; the others stay. An admin
// changes a member's federations within its own this way, leaving
// those of the other federations' admins alone.
type FederationChange struct {
	Set       []uint64
	Within    []uint64
	WithinAll bool
}

// setAccountFederations applies c to the account's rows in table and
// returns the federations before and after (ascending), locking the
// account's rows first.
func setAccountFederations(ctx context.Context, tx *sql.Tx, table string, id uint64,
	c FederationChange) (before, after []uint64, err error) {
	if err := checkFederations(ctx, tx, c.Set); err != nil {
		return nil, nil, err
	}
	cur, err := uint64s(ctx, tx, "SELECT federation_id FROM "+table+
		" WHERE web_user_id = ? ORDER BY federation_id FOR UPDATE", id)
	if err != nil {
		return nil, nil, err
	}
	within := func(f uint64) bool { return c.WithinAll || slices.Contains(c.Within, f) }
	var next []uint64
	for _, f := range cur {
		if !within(f) {
			next = append(next, f)
		}
	}
	for _, f := range c.Set {
		if within(f) {
			next = append(next, f)
		}
	}
	next = uniq(next)
	for _, f := range cur {
		if !slices.Contains(next, f) {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+
				" WHERE federation_id = ? AND web_user_id = ?", f, id); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, f := range next {
		if !slices.Contains(cur, f) {
			if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+
				" (federation_id, web_user_id) VALUES (?, ?)", f, id); err != nil {
				return nil, nil, err
			}
		}
	}
	return cur, next, nil
}

func uint64s(ctx context.Context, db Querier, q string, args ...any) ([]uint64, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint64
	for rows.Next() {
		var v uint64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

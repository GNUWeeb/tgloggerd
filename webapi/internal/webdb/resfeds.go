// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"strings"
)

// The federation of a forwarding configuration (web migration 000018,
// docs/design/web-accounts.md section 6): chosen at creation among the
// federations holding its chat, it decides which admins may give the
// configuration to another account. web_resource_federations holds one
// row per configuration outside Default; a configuration without a row
// is in Default, as every one made before the table was, and deleting a
// federation deletes its rows. Like the owner, the row has no foreign
// key to its resource: the operation creating one files it, and
// DropResource drops the row with the resource's relations.

// ResourceFederations is the federation of each of ids of type t;
// an id in Default is not in the map. One query, for a whole list.
func ResourceFederations(ctx context.Context, db *sql.DB, t ResourceType, ids []int64) (map[int64]uint64, error) {
	if !t.valid() {
		return nil, ErrUnknownType
	}
	out := map[int64]uint64{}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{string(t)}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.QueryContext(ctx, "SELECT resource_id, federation_id FROM web_resource_federations "+
		"WHERE resource_type = ? AND resource_id IN (?"+strings.Repeat(", ?", len(ids)-1)+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id  int64
			fed uint64
		)
		if err := rows.Scan(&id, &fed); err != nil {
			return nil, err
		}
		out[id] = fed
	}
	return out, rows.Err()
}

// ResourceFederation is one resource's federation, DefaultFederation
// when it has no row.
func ResourceFederation(ctx context.Context, db *sql.DB, r Resource) (uint64, error) {
	m, err := ResourceFederations(ctx, db, r.Type, []int64{r.ID})
	if err != nil {
		return 0, err
	}
	return m[r.ID], nil
}

// FileResource puts a resource, just created, into federation fed:
// Default stores nothing. ErrNoFederation for a federation that does
// not exist.
func FileResource(ctx context.Context, db *sql.DB, r Resource, fed uint64) error {
	if !r.Type.valid() {
		return ErrUnknownType
	}
	if fed == DefaultFederation {
		_, err := db.ExecContext(ctx, "DELETE FROM web_resource_federations "+
			"WHERE resource_type = ? AND resource_id = ?", string(r.Type), r.ID)
		return err
	}
	_, err := db.ExecContext(ctx, "INSERT INTO web_resource_federations (resource_type, resource_id, "+
		"federation_id) VALUES (?, ?, ?) AS n ON DUPLICATE KEY UPDATE federation_id = n.federation_id",
		string(r.Type), r.ID, fed)
	if isForeignKey(err) {
		return ErrNoFederation
	}
	return err
}

// GroupFederations is the federations holding a group, ascending: those
// whose stored groups it is in, or Default alone when it is in none. A
// private chat (a positive id) is held by Default.
func GroupFederations(ctx context.Context, db Querier, chatID int64) ([]uint64, error) {
	if chatID > 0 {
		return []uint64{DefaultFederation}, nil
	}
	ids, err := uint64s(ctx, db, "SELECT federation_id FROM web_federation_groups "+
		"WHERE group_id = ? ORDER BY federation_id", chatID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []uint64{DefaultFederation}, nil
	}
	return ids, nil
}

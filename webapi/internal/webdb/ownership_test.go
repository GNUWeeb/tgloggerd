// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/testdb"
)

// ownAudits are the ownership rows of web_audit, oldest first, as
// "<actor> <target> <action> <ip> <detail>" (0 for NULL).
func ownAudits(t *testing.T, db *testdb.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT COALESCE(web_user_id, 0), COALESCE(target_user_id, 0), " +
		"action, COALESCE(ip, ''), COALESCE(detail, '') FROM web_audit " +
		"WHERE action IN ('owner_set', 'owner_transfer', 'resource_drop') ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a, tg, action, ip, d string
		if err := rows.Scan(&a, &tg, &action, &ip, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, strings.Join([]string{a, tg, action, ip, d}, " "))
	}
	return out
}

func TestOwnershipSetReadTransfer(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts", "admin", "ownership")
	ctx := context.Background()

	// The fixture's owners, read in one query; ids without one are
	// left out.
	m, err := Owners(ctx, db.DB, ResT2DWebhook, []int64{1, 2, 3})
	if err != nil || len(m) != 1 || m[1].ID != 3 || m[1].Name() != "GW Admin" ||
		m[1].Username != "gw_admin" || m[1].Role != "admin" || !m[1].Active {
		t.Fatalf("webhook owners %+v %v", m, err)
	}
	o, ok, err := OwnerOf(ctx, db.DB, Resource{ResD2TRoute, 1})
	if err != nil || !ok || o.ID != 7 || o.Active || o.Name() != "intern2025" {
		t.Fatalf("route 1 owner %+v %v %v", o, ok, err)
	}
	if _, ok, err := OwnerOf(ctx, db.DB, Resource{ResD2TRoute, 2}); ok || err != nil {
		t.Fatalf("route 2 has an owner: %v %v", ok, err)
	}
	if m, err := Owners(ctx, db.DB, ResMCPGroup, nil); err != nil || len(m) != 0 {
		t.Fatalf("no ids: %v %v", m, err)
	}

	// A first owner, then a second one refused: one owner each.
	hook := Resource{ResT2DWebhook, 2}
	if err := SetOwner(ctx, db.DB, hook, 5, By(4, "192.0.2.4")); err != nil {
		t.Fatal(err)
	}
	if err := SetOwner(ctx, db.DB, hook, 6, By(4, "192.0.2.4")); !errors.Is(err, ErrOwned) {
		t.Fatalf("second owner: %v", err)
	}
	if err := SetOwner(ctx, db.DB, hook, 5, By(4, "192.0.2.4")); !errors.Is(err, ErrOwned) {
		t.Fatalf("same owner again: %v", err)
	}
	// The same id of another type is another resource.
	if err := SetOwner(ctx, db.DB, Resource{ResD2TRoute, 2}, 6, By(1, "")); err != nil {
		t.Fatal(err)
	}
	var by string
	if err := db.QueryRow("SELECT created_by FROM web_resource_relations WHERE resource_type = " +
		"'t2d_webhook' AND resource_id = 2").Scan(&by); err != nil || by != "4" {
		t.Fatalf("created_by %q %v", by, err)
	}

	// Transfer: from the owner it had, to the same owner a no-op.
	from, err := TransferOwner(ctx, db.DB, hook, 6, By(1, "203.0.113.1"))
	if err != nil || from != 5 {
		t.Fatalf("transfer: %d %v", from, err)
	}
	if o, _, _ := OwnerOf(ctx, db.DB, hook); o.ID != 6 {
		t.Fatalf("after the transfer %+v", o)
	}
	if from, err := TransferOwner(ctx, db.DB, hook, 6, By(1, "203.0.113.1")); err != nil || from != 6 {
		t.Fatalf("transfer to the owner: %d %v", from, err)
	}
	if _, err := TransferOwner(ctx, db.DB, Resource{ResT2DWebhook, 3}, 6, By(1, "")); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("transfer of an unowned one: %v", err)
	}
	// An account that does not exist cannot own anything.
	if _, err := TransferOwner(ctx, db.DB, hook, 999, By(1, "")); err == nil {
		t.Fatal("transfer to no account")
	}
	if err := SetOwner(ctx, db.DB, Resource{ResT2DWebhook, 3}, 999, By(1, "")); err == nil {
		t.Fatal("owner that is no account")
	}

	// What one account owns, by type.
	if ids, err := OwnedBy(ctx, db.DB, ResT2DWebhook, 6); err != nil || !slices.Equal(ids, []int64{2}) {
		t.Fatalf("owned by 6: %v %v", ids, err)
	}
	if ids, err := OwnedBy(ctx, db.DB, ResD2TRoute, 6); err != nil || !slices.Equal(ids, []int64{2}) {
		t.Fatalf("routes owned by 6: %v %v", ids, err)
	}
	if ids, err := OwnedBy(ctx, db.DB, ResMCPToken, 6); err != nil || len(ids) != 0 {
		t.Fatalf("tokens owned by 6: %v %v", ids, err)
	}

	// Deleting the resource drops its relations; dropping it again, or
	// one with none, changes nothing.
	if err := DropResource(ctx, db.DB, hook, By(6, "192.0.2.6")); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := OwnerOf(ctx, db.DB, hook); ok {
		t.Fatal("dropped resource has an owner")
	}
	if err := DropResource(ctx, db.DB, hook, By(6, "192.0.2.6")); err != nil {
		t.Fatal(err)
	}

	for _, err := range []error{
		SetOwner(ctx, db.DB, Resource{"webhook", 1}, 1, By(1, "")),
		DropResource(ctx, db.DB, Resource{"", 1}, By(1, "")),
		func() error { _, err := TransferOwner(ctx, db.DB, Resource{"x", 1}, 1, By(1, "")); return err }(),
		func() error { _, err := Owners(ctx, db.DB, "x", []int64{1}); return err }(),
		func() error { _, err := OwnedBy(ctx, db.DB, "x", 1); return err }(),
	} {
		if !errors.Is(err, ErrUnknownType) {
			t.Errorf("unknown type: %v", err)
		}
	}

	want := []string{
		"4 5 owner_set 192.0.2.4 t2d_webhook 2",
		"1 6 owner_set  d2t_route 2",
		"1 6 owner_transfer 203.0.113.1 t2d_webhook 2 from 5 to 6",
		"6 0 resource_drop 192.0.2.6 t2d_webhook 2",
	}
	if got := ownAudits(t, db); !slices.Equal(got, want) {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestOwnershipAssignUnowned(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts", "admin", "ownership")
	ctx := context.Background()

	// Webhook 1 is gw_admin's; 2, 3 and a repeated 3 are not anyone's.
	got, err := AssignUnowned(ctx, db.DB, ResT2DWebhook, []int64{3, 1, 2, 3}, 2, Deployment)
	if err != nil || !slices.Equal(got, []int64{2, 3}) {
		t.Fatalf("assigned %v %v", got, err)
	}
	m, _ := Owners(ctx, db.DB, ResT2DWebhook, []int64{1, 2, 3})
	if m[1].ID != 3 || m[2].ID != 2 || m[3].ID != 2 {
		t.Fatalf("owners after %+v", m)
	}
	// Twice is a no-op.
	if got, err := AssignUnowned(ctx, db.DB, ResT2DWebhook, []int64{1, 2, 3}, 2, Deployment); err != nil ||
		len(got) != 0 {
		t.Fatalf("second run %v %v", got, err)
	}
	if got, err := AssignUnowned(ctx, db.DB, ResMCPGroup, nil, 2, Deployment); err != nil || len(got) != 0 {
		t.Fatalf("none %v %v", got, err)
	}
	var by any
	if err := db.QueryRow("SELECT created_by FROM web_resource_relations WHERE resource_type = " +
		"'t2d_webhook' AND resource_id = 3").Scan(&by); err != nil || by != nil {
		t.Fatalf("created_by %v %v", by, err)
	}
	want := []string{
		"0 2 owner_set  t2d_webhook 2 assign-owner",
		"0 2 owner_set  t2d_webhook 3 assign-owner",
	}
	if got := ownAudits(t, db); !slices.Equal(got, want) {
		t.Errorf("audit\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Other relations than owner are held by any number of accounts; only
// owner is one per resource (the table is built to take them).
func TestOwnershipOtherRelations(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "accounts", "ownership")
	for _, uid := range []int{5, 6} {
		if _, err := db.Exec("INSERT INTO web_resource_relations (resource_type, resource_id, "+
			"relation, user_id) VALUES ('t2d_webhook', 1, 'viewer', ?)", uid); err != nil {
			t.Fatalf("viewer %d: %v", uid, err)
		}
	}
	_, err := db.Exec("INSERT INTO web_resource_relations (resource_type, resource_id, " +
		"relation, user_id) VALUES ('t2d_webhook', 1, 'owner', 5)")
	if !isDuplicate(err) {
		t.Fatalf("second owner row: %v", err)
	}
	if m, err := Owners(context.Background(), db.DB, ResT2DWebhook, []int64{1}); err != nil ||
		len(m) != 1 || m[1].ID != 3 {
		t.Fatalf("owner beside viewers %+v %v", m, err)
	}
}

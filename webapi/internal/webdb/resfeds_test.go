// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"errors"
	"slices"
	"testing"

	"tgloggerd/webapi/internal/testdb"
)

// A configuration is in Default until it is filed elsewhere; deleting
// it or its federation puts its id back in Default.
func TestResourceFederations(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "federations")
	ctx := context.Background()

	hook, route := Resource{ResT2DWebhook, 1}, Resource{ResD2TRoute, 1}
	if f, err := ResourceFederation(ctx, db.DB, hook); err != nil || f != DefaultFederation {
		t.Fatalf("unfiled %d %v", f, err)
	}
	if err := FileResource(ctx, db.DB, hook, 2); err != nil {
		t.Fatal(err)
	}
	// The same id of another type is another resource.
	if err := FileResource(ctx, db.DB, route, 1); err != nil {
		t.Fatal(err)
	}
	if err := FileResource(ctx, db.DB, Resource{ResT2DWebhook, 2}, 9); !errors.Is(err, ErrNoFederation) {
		t.Fatalf("unknown federation: %v", err)
	}
	m, err := ResourceFederations(ctx, db.DB, ResT2DWebhook, []int64{1, 2, 3})
	if err != nil || len(m) != 1 || m[1] != 2 {
		t.Fatalf("webhooks %v %v", m, err)
	}
	// Default stores nothing.
	if err := FileResource(ctx, db.DB, route, DefaultFederation); err != nil {
		t.Fatal(err)
	}
	if f, _ := ResourceFederation(ctx, db.DB, route); f != DefaultFederation {
		t.Errorf("route back in Default: %d", f)
	}

	// A dropped resource leaves no row, even one without an owner.
	if err := DropResource(ctx, db.DB, hook, By(101, "")); err != nil {
		t.Fatal(err)
	}
	if f, _ := ResourceFederation(ctx, db.DB, hook); f != DefaultFederation {
		t.Errorf("dropped webhook in %d", f)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM web_audit WHERE action = 'resource_drop'").Scan(&n); err != nil || n != 0 {
		t.Errorf("an unowned drop audited: %d %v", n, err)
	}
	// Deleting a federation puts its configurations in Default.
	if err := FileResource(ctx, db.DB, route, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM web_federations WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if f, _ := ResourceFederation(ctx, db.DB, route); f != DefaultFederation {
		t.Errorf("route of a deleted federation in %d", f)
	}
	if _, err := ResourceFederations(ctx, db.DB, "nope", []int64{1}); !errors.Is(err, ErrUnknownType) {
		t.Errorf("unknown type: %v", err)
	}
}

func TestGroupFederations(t *testing.T) {
	t.Parallel()
	db := testdb.New(t, testdb.Web, "federations")
	ctx := context.Background()
	for chat, want := range map[int64][]uint64{
		-1001000000001: {1},
		-1001000000002: {1, 2},
		-1001000000004: {0},
		9299000001:     {0},
	} {
		if got, err := GroupFederations(ctx, db.DB, chat); err != nil || !slices.Equal(got, want) {
			t.Errorf("%d: %v %v, want %v", chat, got, err, want)
		}
	}
}

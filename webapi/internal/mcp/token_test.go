// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import "testing"

// A minted token is one the endpoint recognises, and its digest is the
// one the endpoint looks it up by.
func TestNewToken(t *testing.T) {
	t.Parallel()
	a, da, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewToken()
	if !LooksLikeToken(a) || a == b || da != TokenDigest(a) {
		t.Fatalf("token %q digest %x", a, da)
	}
}

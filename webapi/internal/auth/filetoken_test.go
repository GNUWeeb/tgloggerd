// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"strings"
	"testing"
)

func TestFileTokens(t *testing.T) {
	f := NewFileTokens(testKey)
	seen := map[string]bool{}
	for _, id := range []uint64{0, 1, 2, 3, 1 << 32, 1<<64 - 1} {
		tok := f.Encrypt(id)
		if len(tok) != 64 || strings.ToLower(tok) != tok {
			t.Errorf("Encrypt(%d) = %q", id, tok)
		}
		if f.Encrypt(id) != tok {
			t.Errorf("Encrypt(%d) is not stable", id)
		}
		if seen[tok] {
			t.Errorf("Encrypt(%d) repeats a token", id)
		}
		seen[tok] = true
		if got, ok := f.Decrypt(tok); !ok || got != id {
			t.Errorf("Decrypt(Encrypt(%d)) = %d, %v", id, got, ok)
		}
		if got, ok := f.Decrypt(strings.ToUpper(tok)); !ok || got != id {
			t.Errorf("Decrypt(upper) = %d, %v", got, ok)
		}
		for i := 0; i < 64; i += 13 {
			b := []byte(tok)
			b[i] ^= 1
			if _, ok := f.Decrypt(string(b)); ok {
				t.Errorf("a tampered token for %d decrypted", id)
			}
		}
	}
	for _, bad := range []string{"", "00", strings.Repeat("0", 64),
		strings.Repeat("g", 64), f.Encrypt(1) + "00"} {
		if _, ok := f.Decrypt(bad); ok {
			t.Errorf("Decrypt(%q) succeeded", bad)
		}
	}
	if _, ok := NewFileTokens([]byte("another key of 16+ bytes")).Decrypt(f.Encrypt(5)); ok {
		t.Error("a token decrypted under another key")
	}
}

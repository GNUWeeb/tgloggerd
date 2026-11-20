// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if !re.MatchString(h) {
		t.Fatalf("HashPassword = %q", h)
	}
	if !VerifyPassword(h, "hunter2") {
		t.Error("the password does not verify")
	}
	if VerifyPassword(h, "hunter3") || VerifyPassword(h, "") {
		t.Error("a wrong password verifies")
	}
	h2, _ := HashPassword("hunter2")
	if h2 == h {
		t.Error("two hashes share a salt")
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	good, _ := HashPassword("pw")
	f := strings.Split(good, "$")
	for _, bad := range []string{
		"", "pw", "$argon2id$", good + "$",
		strings.Replace(good, "v=19", "v=16", 1),
		strings.Replace(good, "argon2id", "argon2d", 1),
		strings.Replace(good, "m=65536,t=2,p=1", "m=65536,t=0,p=1", 1),
		strings.Replace(good, "m=65536,t=2,p=1", "m=65536,t=+2,p=1", 1),
		strings.Replace(good, "m=65536,t=2,p=1", "t=2,m=65536,p=1", 1),
		strings.Join([]string{f[0], f[1], f[2], f[3], f[4] + "=", f[5]}, "$"),
		strings.Join([]string{f[0], f[1], f[2], f[3], "AAAA", f[5]}, "$"),
		strings.Join([]string{f[0], f[1], f[2], f[3], f[4], f[5][:20]}, "$"),
	} {
		if VerifyPassword(bad, "pw") {
			t.Errorf("VerifyPassword(%q) verified", bad)
		}
	}
}

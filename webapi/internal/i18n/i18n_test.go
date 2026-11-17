// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Parallel()
	c := set{
		"en": {"a": "Hello, {name}.", "b": "Only English"},
		"id": {"a": "Halo, {name}."},
	}
	for _, tc := range []struct{ lang, key, want string }{
		{"en", "a", "Hello, Dewi."},
		{"id", "a", "Halo, Dewi."},
		{"id", "b", "Only English"}, // missing in id: English
		{"fr", "a", "Hello, Dewi."}, // no catalogue: English
		{"en", "nope", "nope"},      // missing everywhere: the key
	} {
		if got := c.text(tc.lang, tc.key, "name", "Dewi"); got != tc.want {
			t.Errorf("text(%s, %s) = %q, want %q", tc.lang, tc.key, got, tc.want)
		}
	}
}

func TestCatalogues(t *testing.T) {
	t.Parallel()
	if Pick(Fallback) != Fallback || Pick("xx") != Fallback || Has("xx") {
		t.Error("Pick or Has")
	}
	// Every language's keys are English keys, and no text is empty, so
	// a language can only leave texts out, never invent them.
	for lang, m := range catalogs {
		for k, v := range m {
			if v == "" {
				t.Errorf("%s: %s is empty", lang, k)
			}
			if _, ok := catalogs[Fallback][k]; !ok {
				t.Errorf("%s: %s is not an English key", lang, k)
			}
		}
	}
}

// keyUse finds the keys the service looks up by a literal: i18n.T(lang,
// "key" ...), and through the server's badField(w, lang, "key", ...).
var keyUse = regexp.MustCompile(`(?:i18n\.T\(|badField\([^,()]+,)[^,()]+, "([^"]+)"`)

// Every key the code looks up has an English text.
func TestKeysUsedExist(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if filepath.Dir(f) == "../i18n" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range keyUse.FindAllSubmatch(b, -1) {
			n++
			if _, ok := catalogs[Fallback][string(m[1])]; !ok {
				t.Errorf("%s: %q has no English text", f, m[1])
			}
		}
	}
	t.Logf("%d lookups checked", n)
}

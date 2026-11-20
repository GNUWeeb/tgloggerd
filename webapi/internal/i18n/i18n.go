// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package i18n looks up the user-facing texts of the service by
// language (decisions/2026-11-18-web-rollout.md: English first, every
// text through a lookup from the start). The texts live in one JSON
// catalogue per language, catalog/<lang>.json, a flat object of key to
// text, embedded at build time: adding a language is adding its file,
// with no change to the code.
//
// A text may name values to fill in as {name}; T replaces them. A key
// missing from a language falls back to English, and a key missing
// from English (a bug the tests catch) is answered as the key itself,
// so a page never goes blank for want of a text.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
)

// Fallback is the language every key has a text in.
const Fallback = "en"

//go:embed catalog/*.json
var files embed.FS

// set maps a language to its texts.
type set map[string]map[string]string

// catalogs are the embedded catalogues.
var catalogs = mustLoad()

func mustLoad() set {
	c, err := load()
	if err != nil {
		panic("i18n: " + err.Error())
	}
	return c
}

func load() (set, error) {
	names, err := files.ReadDir("catalog")
	if err != nil {
		return nil, err
	}
	out := set{}
	for _, n := range names {
		lang, ok := strings.CutSuffix(n.Name(), ".json")
		if !ok {
			continue
		}
		b, err := files.ReadFile(path.Join("catalog", n.Name()))
		if err != nil {
			return nil, err
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", n.Name(), err)
		}
		out[lang] = m
	}
	if _, ok := out[Fallback]; !ok {
		return nil, fmt.Errorf("no %s catalogue", Fallback)
	}
	return out, nil
}

// Languages lists the languages that have a catalogue, sorted.
func Languages() []string {
	out := make([]string, 0, len(catalogs))
	for l := range catalogs {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// Has is whether lang has a catalogue of its own.
func Has(lang string) bool {
	_, ok := catalogs[lang]
	return ok
}

// Pick is lang when it has a catalogue, else the fallback.
func Pick(lang string) string {
	if Has(lang) {
		return lang
	}
	return Fallback
}

// T is the text of key in lang, with each {name} replaced by the value
// after it in kv ("name", "value", ...).
func T(lang, key string, kv ...string) string {
	return catalogs.text(lang, key, kv...)
}

func (c set) text(lang, key string, kv ...string) string {
	s, ok := c[lang][key]
	if !ok {
		if s, ok = c[Fallback][key]; !ok {
			return key
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		s = strings.ReplaceAll(s, "{"+kv[i]+"}", kv[i+1])
	}
	return s
}

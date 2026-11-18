// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"strings"
	"unicode"
)

// mysqlText is a full-text value as MySQL's boolean mode reads it: the
// search engine's own syntax taken out, so a value written for the
// engine (web/docs/search-api.md, "The matches value") means the
// nearest thing on MySQL instead of failing there. That syntax is a
// word's fuzziness, word~ and word~N, which MySQL rejects or reads as
// another word: the word is kept and the rest dropped. Everything else
// is MySQL's syntax already ("a b" @N included) and passes as it is;
// the text of a phrase is never changed.
func mysqlText(v string) string {
	if !strings.Contains(v, "~") {
		return v
	}
	var b strings.Builder
	rs := []rune(v)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			j = min(j+1, len(rs))
			b.WriteString(string(rs[i:j]))
			i = j
			continue
		case unicode.IsSpace(r) || r == '(' || r == ')':
			b.WriteRune(r)
			i++
			continue
		}
		// A word with its operators: the operators before it are
		// MySQL's (~word included), a ~ after its first letter starts
		// the engine's fuzziness, which runs to the word's end.
		j := i
		for j < len(rs) && strings.ContainsRune("+-~<>", rs[j]) {
			j++
		}
		k := j
		for k < len(rs) && !unicode.IsSpace(rs[k]) && rs[k] != '"' && rs[k] != '(' &&
			rs[k] != ')' {
			k++
		}
		w := string(rs[j:k])
		if t, _, ok := strings.Cut(w, "~"); ok && t != "" {
			w = t
		}
		b.WriteString(string(rs[i:j]) + w)
		i = k
	}
	return b.String()
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import "strings"

// Word counting for telegram_popular_words, as the Drogon web counts
// (web/src/mcp/telegram/Tools.cpp, countWords).
//
// A word character is an ASCII letter or digit, or any byte >= 0x80, so
// a UTF-8 word stays one token; only ASCII case is folded. URLs and
// e-mail addresses are dropped whole: pasted From: lines alone would
// otherwise put com, org and gmail at the top. @mentions are kept, who
// gets talked about being a real answer. Then the filters: too short,
// all digits, an excluded word, a stopword (stopwords.go), corpus noise
// (noisewords.go) or laughter and filler by shape (isRepetitive).

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// isAddrByte: what may sit inside an address without ending the run.
func isAddrByte(c byte) bool {
	return isWordByte(c) || c == '.' || c == '-' || c == '_' || c == '+' || c == '@'
}

// isRepetitive: a unit of one or two characters repeated to fill a
// token of four or more ("wkwkwkwk", "hahaha").
func isRepetitive(w string) bool {
	if len(w) < 4 {
		return false
	}
	for u := 1; u <= 2; u++ {
		if len(w)%u != 0 || len(w)/u < 2 {
			continue
		}
		same := true
		for i := u; i < len(w) && same; i++ {
			same = w[i] == w[i-u]
		}
		if same {
			return true
		}
	}
	return false
}

// collapseElongation shortens a run of three or more identical
// characters to one ("hmmmm" -> "hm", "yesss" -> "yes"; "coffee", with
// runs of two, survives), as the tool's description says. The Drogon
// code kept two characters; this service answers as documented. By
// codepoint, not by byte, as one character may repeat a byte three
// times.
func collapseElongation(w string) string {
	var cps []string
	for i := 0; i < len(w); {
		c := w[i]
		n := 1
		switch {
		case c < 0x80:
		case c>>5 == 0x06:
			n = 2
		case c>>4 == 0x0E:
			n = 3
		case c>>3 == 0x1E:
			n = 4
		}
		if i+n > len(w) {
			n = 1
		}
		cps = append(cps, w[i:i+n])
		i += n
	}
	var b strings.Builder
	b.Grow(len(w))
	for i := 0; i < len(cps); {
		j := i + 1
		for j < len(cps) && cps[j] == cps[i] {
			j++
		}
		if j-i >= 3 {
			b.WriteString(cps[i])
		} else {
			for k := i; k < j; k++ {
				b.WriteString(cps[k])
			}
		}
		i = j
	}
	return b.String()
}

type wordFilter struct {
	minLen       int
	useStopwords bool
	extra        map[string]struct{}
}

func (f *wordFilter) drop(w string) bool {
	if len(w) < f.minLen {
		return true
	}
	if strings.Trim(w, "0123456789") == "" {
		return true
	}
	if _, ok := f.extra[w]; ok {
		return true
	}
	if !f.useStopwords {
		return false
	}
	return isStopword(w) || isNoiseWord(w) || isRepetitive(w)
}

func isSpace(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' }

func countWords(text string, f *wordFilter, freq map[string]int64, total *int64) {
	n := len(text)
	for i := 0; i < n; {
		c := text[i]
		if !isWordByte(c) {
			i++
			continue
		}
		// A scheme means a URL: skip to the next blank.
		if c == 'h' && (strings.HasPrefix(text[i:], "http://") ||
			strings.HasPrefix(text[i:], "https://")) {
			for i < n && !isSpace(text[i]) {
				i++
			}
			continue
		}
		// The whole run first, punctuation included, so an address is
		// seen as one thing before it is split.
		begin := i
		addr := false
		for i < n && isAddrByte(text[i]) {
			if text[i] == '@' {
				addr = true
			}
			i++
		}
		if addr {
			continue
		}
		for j := begin; j < i; {
			if !isWordByte(text[j]) {
				j++
				continue
			}
			var b strings.Builder
			for j < i && isWordByte(text[j]) {
				ch := text[j]
				j++
				if ch >= 'A' && ch <= 'Z' {
					ch += 'a' - 'A'
				}
				b.WriteByte(ch)
			}
			w := collapseElongation(b.String())
			if f.drop(w) {
				continue
			}
			freq[w]++
			*total++
		}
	}
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// A reader for the subset of YAML webapi/openapi.yaml is written in,
// enough for the OpenAPI test: the standard library has no YAML
// parser, and the test is not worth a third-party module. It keeps the
// tree of block mapping keys with their one-line scalar values;
// sequence items become children named "-0", "-1", ... and the text of
// block scalars (| and >) is skipped.
//
// It refuses what it does not understand instead of guessing, so the
// file cannot drift into a layout it would misread: tabs in the
// indentation, a sequence at the indentation of its key, a plain
// scalar continued on the next line, a flow collection spread over
// several lines, a duplicate key, anchors, aliases, tags and further
// documents.

type yamlNode struct {
	value string // the scalar on the key's line, unquoted
	keys  []string
	child map[string]*yamlNode
	line  int
}

func (n *yamlNode) add(key string, line int) (*yamlNode, error) {
	if n.child == nil {
		n.child = map[string]*yamlNode{}
	}
	if _, dup := n.child[key]; dup {
		return nil, fmt.Errorf("line %d: duplicate key %q", line, key)
	}
	c := &yamlNode{line: line}
	n.child[key] = c
	n.keys = append(n.keys, key)
	return c, nil
}

// isSeq says whether n holds sequence items rather than mapping keys.
func (n *yamlNode) isSeq() bool {
	return len(n.keys) > 0 && strings.HasPrefix(n.keys[0], "-")
}

// get follows a path of keys from n; nil when one is missing.
func (n *yamlNode) get(keys ...string) *yamlNode {
	for _, k := range keys {
		if n == nil {
			return nil
		}
		n = n.child[k]
	}
	return n
}

func parseYAMLKeys(src string) (*yamlNode, error) {
	type frame struct {
		indent int
		node   *yamlNode
	}
	root := &yamlNode{}
	stack := []frame{{-1, root}}
	block := -1 // indentation of the key owning a block scalar, or -1

	for i, line := range strings.Split(src, "\n") {
		ln := i + 1
		text := strings.TrimLeft(line, " ")
		indent := len(line) - len(text)
		if block >= 0 {
			if strings.TrimSpace(text) == "" || indent > block {
				continue
			}
			block = -1
		}
		if text == "" || text[0] == '#' {
			continue
		}
		if text[0] == '\t' {
			return nil, fmt.Errorf("line %d: tab in the indentation", ln)
		}
		if text == "---" || text == "..." {
			return nil, fmt.Errorf("line %d: one document only", ln)
		}

		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].node

		// Sequence items, "- " possibly repeated ("- - x").
		item := false
		for text == "-" || strings.HasPrefix(text, "- ") {
			if parent.value != "" || (len(parent.keys) > 0 && !parent.isSeq()) {
				return nil, fmt.Errorf("line %d: a sequence item must be "+
					"indented under its own key", ln)
			}
			n, err := parent.add("-"+strconv.Itoa(len(parent.keys)), ln)
			if err != nil {
				return nil, err
			}
			stack = append(stack, frame{indent, n})
			rest := strings.TrimLeft(text[1:], " ")
			indent += len(text) - len(rest)
			text, parent, item = rest, n, true
		}
		if text == "" {
			continue
		}

		key, value, ok, err := splitKey(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", ln, err)
		}
		if !ok {
			if !item {
				return nil, fmt.Errorf("line %d: neither a key nor a "+
					"sequence item (a plain scalar continued from the "+
					"line before?)", ln)
			}
			v, err := scalar(text)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", ln, err)
			}
			parent.value = v
			continue
		}
		if parent.isSeq() || parent.value != "" {
			return nil, fmt.Errorf("line %d: key %q mixed into a sequence "+
				"or under a scalar", ln, key)
		}
		n, err := parent.add(key, ln)
		if err != nil {
			return nil, err
		}
		stack = append(stack, frame{indent, n})
		if value != "" && (value[0] == '|' || value[0] == '>') {
			block = indent
			continue
		}
		if n.value, err = scalar(value); err != nil {
			return nil, fmt.Errorf("line %d: %v", ln, err)
		}
	}
	return root, nil
}

// splitKey splits "key: value" (or "key:"). ok is false for a line
// that is a scalar, not a mapping entry.
func splitKey(text string) (key, value string, ok bool, err error) {
	var rest string
	if text[0] == '\'' || text[0] == '"' {
		end := closingQuote(text)
		if end < 0 {
			return "", "", false, fmt.Errorf("unterminated quote")
		}
		rest = text[end+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", "", false, nil
		}
		if key, err = scalar(text[:end+1]); err != nil {
			return "", "", false, err
		}
	} else {
		i := strings.Index(text, ": ")
		if i < 0 {
			if !strings.HasSuffix(text, ":") {
				return "", "", false, nil
			}
			i = len(text) - 1
		}
		key, rest = text[:i], text[i:]
		if strings.ContainsAny(key[:1], "[{&*!|>%@`") {
			return "", "", false, nil
		}
	}
	rest = rest[1:]
	if rest != "" && rest[0] != ' ' {
		return "", "", false, nil
	}
	return key, strings.TrimSpace(rest), true, nil
}

// closingQuote is the index of the quote that closes the quoted
// scalar text starts with, or -1.
func closingQuote(text string) int {
	q := text[0]
	for i := 1; i < len(text); i++ {
		switch {
		case q == '"' && text[i] == '\\':
			i++
		case text[i] == q && q == '\'' && i+1 < len(text) && text[i+1] == '\'':
			i++
		case text[i] == q:
			return i
		}
	}
	return -1
}

// scalar is the value of a one-line scalar: quotes removed, a trailing
// comment dropped. Flow collections are kept as their text.
func scalar(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	switch v[0] {
	case '\'', '"':
		end := closingQuote(v)
		if end < 0 {
			return "", fmt.Errorf("unterminated quote")
		}
		if tail := strings.TrimSpace(v[end+1:]); tail != "" && tail[0] != '#' {
			return "", fmt.Errorf("text after a quoted scalar")
		}
		if v[0] == '"' {
			return strconv.Unquote(v[:end+1])
		}
		return strings.ReplaceAll(v[1:end], "''", "'"), nil
	case '&', '*', '!':
		return "", fmt.Errorf("anchors, aliases and tags are not supported")
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	switch {
	case v[0] == '[' && !strings.HasSuffix(v, "]"),
		v[0] == '{' && !strings.HasSuffix(v, "}"):
		return "", fmt.Errorf("a flow collection must close on its line")
	case v[0] != '[' && v[0] != '{' &&
		(strings.Contains(v, ": ") || strings.HasSuffix(v, ":")):
		return "", fmt.Errorf("a plain scalar cannot hold \": \"; quote it")
	}
	return v, nil
}

func TestParseYAMLKeys(t *testing.T) {
	src := `# comment
a:
  b: 'x: y' # trailing
  c: "q\"r"
  d: >-
    folded text: with a colon

    and a blank line
  e:
    - one
    - k: v
      l: w
    - {}
  f: [1, 2]
  m: {type: string}
'g h':
  i: 'it''s'
`
	root, err := parseYAMLKeys(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path []string
		want string
	}{
		{[]string{"a", "b"}, "x: y"},
		{[]string{"a", "c"}, `q"r`},
		{[]string{"a", "d"}, ""},
		{[]string{"a", "e", "-0"}, "one"},
		{[]string{"a", "e", "-1", "k"}, "v"},
		{[]string{"a", "e", "-1", "l"}, "w"},
		{[]string{"a", "e", "-2"}, "{}"},
		{[]string{"a", "f"}, "[1, 2]"},
		{[]string{"a", "m"}, "{type: string}"},
		{[]string{"g h", "i"}, "it's"},
	} {
		n := root.get(c.path...)
		if n == nil {
			t.Errorf("%v: missing", c.path)
			continue
		}
		if n.value != c.want {
			t.Errorf("%v = %q, want %q", c.path, n.value, c.want)
		}
	}
	if got := strings.Join(root.get("a").keys, ","); got != "b,c,d,e,f,m" {
		t.Errorf("keys of a: %s", got)
	}

	for name, bad := range map[string]string{
		"duplicate key":     "a: 1\na: 2\n",
		"compact sequence":  "a:\n  b: 1\n  c:\n  - x\n  d: 2\n",
		"continued scalar":  "a: some\n  text\n",
		"tab":               "a:\n\tb: 1\n",
		"open flow":         "a: [1,\n  2]\n",
		"open quote":        "a: 'x\n",
		"key under scalar":  "a: 1\n  b: 2\n",
		"alias":             "a: *x\n",
		"colon in plain":    "a: b: c\n",
		"second document":   "a: 1\n---\nb: 2\n",
		"key into sequence": "a:\n  - x\n  b: 1\n",
	} {
		if _, err := parseYAMLKeys(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

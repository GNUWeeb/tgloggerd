// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// webapi/openapi.yaml is the API contract: the React application's
// client is generated from it. This test keeps it in line with the
// route table: every route served is described, every operation
// described without `x-tgd-status: planned` is served, and none that
// is served is still marked planned.

const openAPIFile = "../../openapi.yaml"

// plannedStatus is the only value x-tgd-status takes.
const plannedStatus = "planned"

var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// operation is "METHOD /path" with the path written as in OpenAPI.
type operation string

// wildcard is a ServeMux wildcard: {name} or {name...}.
var wildcard = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(\.\.\.)?\}`)

// servedOperation turns a ServeMux pattern into the operation it
// serves. {name...} is written {name} in OpenAPI, and {$} (an exact
// match on a trailing slash) is dropped.
func servedOperation(pattern string) (operation, error) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || method == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("route %q: want \"METHOD /path\"", pattern)
	}
	path = strings.TrimSuffix(path, "{$}")
	path = wildcard.ReplaceAllString(path, "{$1}")
	return operation(method + " " + path), nil
}

// describedOperations lists the operations of an OpenAPI document,
// with their x-tgd-status ("" when served).
func describedOperations(root *yamlNode) (map[operation]string, error) {
	paths := root.get("paths")
	if paths == nil || len(paths.keys) == 0 {
		return nil, fmt.Errorf("no paths")
	}
	ops := map[operation]string{}
	ids := map[string]int{}
	for _, p := range paths.keys {
		item := paths.child[p]
		for _, m := range item.keys {
			if !httpMethods[m] {
				continue
			}
			op := item.child[m]
			st := op.get("x-tgd-status")
			status := ""
			if st != nil {
				if st.value != plannedStatus {
					return nil, fmt.Errorf("line %d: x-tgd-status %q, want %q",
						st.line, st.value, plannedStatus)
				}
				status = st.value
			}
			id := op.get("operationId")
			if id == nil || id.value == "" {
				return nil, fmt.Errorf("line %d: %s %s has no operationId",
					op.line, m, p)
			}
			if l, dup := ids[id.value]; dup {
				return nil, fmt.Errorf("line %d: operationId %q also on line %d",
					id.line, id.value, l)
			}
			ids[id.value] = id.line
			ops[operation(strings.ToUpper(m)+" "+p)] = status
		}
	}
	return ops, nil
}

// checkRefs reports every local $ref that points nowhere.
func checkRefs(root *yamlNode) []string {
	var bad []string
	var walk func(n *yamlNode)
	walk = func(n *yamlNode) {
		for _, k := range n.keys {
			c := n.child[k]
			if k == "$ref" && strings.HasPrefix(c.value, "#/") {
				t := root
				for _, part := range strings.Split(c.value[2:], "/") {
					part = strings.ReplaceAll(part, "~1", "/")
					t = t.get(strings.ReplaceAll(part, "~0", "~"))
				}
				if t == nil {
					bad = append(bad, fmt.Sprintf("line %d: $ref %s points nowhere",
						c.line, c.value))
				}
			}
			walk(c)
		}
	}
	walk(root)
	return bad
}

// compareRoutes reports where the served routes and the described
// operations disagree. ServeMux answers HEAD for a GET pattern, so a
// described HEAD counts as served when its GET is.
func compareRoutes(patterns []string, described map[operation]string) []string {
	var bad []string
	served := map[operation]bool{}
	for _, p := range patterns {
		op, err := servedOperation(p)
		if err != nil {
			bad = append(bad, err.Error())
			continue
		}
		served[op] = true
		status, ok := described[op]
		switch {
		case !ok:
			bad = append(bad, fmt.Sprintf("%s is served but not described", op))
		case status == plannedStatus:
			bad = append(bad, fmt.Sprintf("%s is served but still marked "+
				"x-tgd-status: planned", op))
		}
	}
	for op, status := range described {
		if status == plannedStatus || served[op] {
			continue
		}
		if p, ok := strings.CutPrefix(string(op), "HEAD "); ok &&
			served[operation("GET "+p)] {
			continue
		}
		bad = append(bad, fmt.Sprintf("%s is described but not served "+
			"(mark it x-tgd-status: planned until it is)", op))
	}
	sort.Strings(bad)
	return bad
}

func routePatterns() []string {
	var ps []string
	for _, rt := range (&Server{}).routes() {
		ps = append(ps, rt.pattern)
	}
	return ps
}

func TestOpenAPIMatchesRoutes(t *testing.T) {
	src, err := os.ReadFile(openAPIFile)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseYAMLKeys(string(src))
	if err != nil {
		t.Fatalf("%s: %v", openAPIFile, err)
	}
	if v := root.get("openapi"); v == nil || !strings.HasPrefix(v.value, "3.1.") {
		t.Fatalf("%s: want openapi: 3.1.x", openAPIFile)
	}
	described, err := describedOperations(root)
	if err != nil {
		t.Fatalf("%s: %v", openAPIFile, err)
	}
	for _, msg := range checkRefs(root) {
		t.Errorf("%s: %s", openAPIFile, msg)
	}
	for _, msg := range compareRoutes(routePatterns(), described) {
		t.Errorf("%s: %s", openAPIFile, msg)
	}
}

// TestCompareRoutes checks that the comparison catches each kind of
// disagreement.
func TestCompareRoutes(t *testing.T) {
	doc := `openapi: 3.1.0
paths:
  /api/a:
    get:
      operationId: getA
    post:
      operationId: postA
      x-tgd-status: planned
  /api/b/{rest}:
    get:
      operationId: getB
    head:
      operationId: headB
  /api/c:
    delete:
      operationId: deleteC
`
	root, err := parseYAMLKeys(doc)
	if err != nil {
		t.Fatal(err)
	}
	described, err := describedOperations(root)
	if err != nil {
		t.Fatal(err)
	}

	got := compareRoutes([]string{
		"GET /api/a",
		"POST /api/a",
		"GET /api/b/{rest...}",
		"PUT /api/d",
	}, described)
	want := []string{
		"DELETE /api/c is described but not served (mark it x-tgd-status: planned until it is)",
		"POST /api/a is served but still marked x-tgd-status: planned",
		"PUT /api/d is served but not described",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if got := compareRoutes([]string{"GET /api/a", "GET /api/b/{rest...}",
		"DELETE /api/c"}, described); len(got) != 0 {
		t.Errorf("agreeing routes: %v", got)
	}

	for name, bad := range map[string]string{
		"unknown status": "paths:\n  /a:\n    get:\n      operationId: a\n" +
			"      x-tgd-status: soon\n",
		"no operationId": "paths:\n  /a:\n    get:\n      summary: x\n",
		"duplicate operationId": "paths:\n  /a:\n    get:\n      operationId: a\n" +
			"  /b:\n    get:\n      operationId: a\n",
	} {
		root, err := parseYAMLKeys(bad)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := describedOperations(root); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	refs, err := parseYAMLKeys("a:\n  $ref: '#/components/schemas/X'\n" +
		"components:\n  schemas:\n    Y:\n      type: string\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := checkRefs(refs); len(got) != 1 {
		t.Errorf("dangling $ref: %v", got)
	}
}

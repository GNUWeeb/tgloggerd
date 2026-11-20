// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The cases of web/app/standin/document.test.ts, the reference
// assembly, plus the ones a hostile string could try on the attributes.

const hostile = "</script><script>alert(1)</script> & \u2028 \u2029 <!-- -->"

func hostilePage() Page {
	return Page{
		V: 1, Route: "users", URL: "/users?q=%3C/script%3E", Status: 200,
		Props:  map[string]any{"name": hostile},
		Viewer: Viewer{SignedIn: true, Now: "2026-11-18T00:00:00Z", Theme: "dark"},
	}
}

var noAssets = Assets{Entry: "/app/e.js"}

func TestScriptJSONEscapes(t *testing.T) {
	t.Parallel()
	p := hostilePage()
	s, err := ScriptJSON(p.Props)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"<", ">", "&", "\u2028", "\u2029"} {
		if strings.Contains(string(s), c) {
			t.Errorf("%q left in %s", c, s)
		}
	}
	for _, c := range []string{`\u003c/script\u003e`, `\u0026`, `\u2028`, `\u2029`} {
		if !strings.Contains(string(s), c) {
			t.Errorf("%s missing from %s", c, s)
		}
	}
	var back map[string]any
	if err := json.Unmarshal(s, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, p.Props) {
		t.Errorf("round trip: %v, want %v", back, p.Props)
	}
}

// pageData returns the embedded page data and checks it is the only
// script content and closes where it should.
func pageData(t *testing.T, doc string) Page {
	t.Helper()
	re := regexp.MustCompile(`(?s)<script type="application/json" id="__tgd">(.*?)</script>`)
	m := re.FindAllStringSubmatch(doc, -1)
	if len(m) != 1 {
		t.Fatalf("want one page data script, got %d in\n%s", len(m), doc)
	}
	if n := strings.Count(doc, "<script"); n != 2 {
		t.Errorf("%d script elements, want the data and the entry", n)
	}
	var p Page
	if err := json.Unmarshal([]byte(m[0][1]), &p); err != nil {
		t.Fatalf("page data: %v", err)
	}
	return p
}

func TestDocumentFallback(t *testing.T) {
	t.Parallel()
	p := hostilePage()
	b, err := Document(p, nil, "", noAssets)
	if err != nil {
		t.Fatal(err)
	}
	d := string(b)
	for _, want := range []string{
		`<!doctype html>`,
		`<html lang="en" data-theme="dark">`,
		`<div id="root" data-ssr="0"></div>`,
		`<title>tgloggerd</title>`,
		`<script type="module" src="/app/e.js"></script>`,
	} {
		if !strings.Contains(d, want) {
			t.Errorf("missing %s in\n%s", want, d)
		}
	}
	if strings.Contains(d, "csrf-token") {
		t.Error("a signed-out document carries a CSRF meta tag")
	}
	got := pageData(t, d)
	if got.Props.(map[string]any)["name"] != hostile || got.URL != p.URL {
		t.Errorf("page data changed: %+v", got)
	}
}

func TestDocumentRendered(t *testing.T) {
	t.Parallel()
	p := hostilePage()
	p.Viewer.Theme = "system"
	r := &Result{Head: "<title>x</title>", HTML: "<p>y</p>"}
	a := Assets{CSS: []string{"/app/a.css"}, Preload: []string{"/app/c.js"}, Entry: "/app/e.js"}
	b, err := Document(p, r, `t"k<&`, a)
	if err != nil {
		t.Fatal(err)
	}
	d := string(b)
	for _, want := range []string{
		`<html lang="en">`,
		`<div id="root" data-ssr="1"><p>y</p></div>`,
		`<meta name="csrf-token" content="t&quot;k&lt;&amp;">`,
		`<title>x</title>`,
		`<link rel="stylesheet" href="/app/a.css">`,
		`<link rel="modulepreload" href="/app/c.js">`,
	} {
		if !strings.Contains(d, want) {
			t.Errorf("missing %s in\n%s", want, d)
		}
	}
	if strings.Contains(d, "data-theme") {
		t.Error("theme system sets data-theme")
	}
	pageData(t, d)
}

func TestDocumentHostileAttributes(t *testing.T) {
	t.Parallel()
	p := hostilePage()
	p.Viewer.Lang = `en"><script>x</script>`
	p.Viewer.Theme = `d" onload="x`
	b, err := Document(p, nil, "", noAssets)
	if err != nil {
		t.Fatal(err)
	}
	d := string(b)
	if !strings.Contains(d, `<html lang="en&quot;>&lt;script>x&lt;/script>" data-theme="d&quot; onload=&quot;x">`) {
		t.Errorf("attributes not escaped:\n%s", d)
	}
	pageData(t, d)
}

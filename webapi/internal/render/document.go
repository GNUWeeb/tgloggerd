// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"encoding/json"
	"strings"
)

// fallbackHead is the head of a page the render service did not
// render; the browser sets the real title once it has rendered.
const fallbackHead = "<title>tgloggerd</title>"

// attr escapes s for a double-quoted attribute value.
var attr = strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;").Replace

// ScriptJSON encodes v for an inline <script type="application/json">.
// encoding/json escapes <, > and & as <, > and &, and
// U+2028 and U+2029, so no string can close the element or break
// JavaScript parsing; the browser reads it with JSON.parse.
func ScriptJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Document assembles the HTML document of a page (web-ssr.md section
// 5). r is the render service's answer, or nil for the fallback, which
// the browser renders from the same embedded data (data-ssr="0").
// csrf is the session's CSRF token, "" for a signed-out visitor; only
// this service adds it, the render service never sees it.
func Document(p Page, r *Result, csrf string, a Assets) ([]byte, error) {
	data, err := ScriptJSON(p)
	if err != nil {
		return nil, err
	}
	lang := p.Viewer.Lang
	if lang == "" {
		lang = "en"
	}

	var b strings.Builder
	b.Grow(len(data) + 2048)
	b.WriteString("<!doctype html>\n<html lang=\"" + attr(lang) + "\"")
	if t := p.Viewer.Theme; t != "" && t != "system" {
		b.WriteString(" data-theme=\"" + attr(t) + "\"")
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	if csrf != "" {
		b.WriteString("<meta name=\"csrf-token\" content=\"" + attr(csrf) + "\">\n")
	}
	if r != nil {
		b.WriteString(r.Head + "\n")
	} else {
		b.WriteString(fallbackHead + "\n")
	}
	for _, h := range a.CSS {
		b.WriteString("<link rel=\"stylesheet\" href=\"" + attr(h) + "\">\n")
	}
	for _, h := range a.Preload {
		b.WriteString("<link rel=\"modulepreload\" href=\"" + attr(h) + "\">\n")
	}
	b.WriteString("</head>\n<body>\n")
	if r != nil {
		b.WriteString("<div id=\"root\" data-ssr=\"1\">" + r.HTML + "</div>\n")
	} else {
		b.WriteString("<div id=\"root\" data-ssr=\"0\"></div>\n")
	}
	b.WriteString("<script type=\"application/json\" id=\"__tgd\">")
	b.Write(data)
	b.WriteString("</script>\n")
	if a.Entry != "" {
		b.WriteString("<script type=\"module\" src=\"" + attr(a.Entry) + "\"></script>\n")
	}
	b.WriteString("</body>\n</html>\n")
	return []byte(b.String()), nil
}

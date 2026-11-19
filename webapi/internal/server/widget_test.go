// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// A widget's detail (WidgetDetail) over the "widgets" fixture: its size,
// caption flag and shared id from the curated keys, or from TDLib's JSON
// for a row stored before them; its HTML on the message page only, null
// over the daemon's cap; and the HTML never reaching a browser as markup
// (decision 2026-11-20-msg-widgets-html).

const widgetLab = int64(-1001000000044)

const widgetHTML = "<script>alert(1)</script><b>Scores</b>"

// widgetWindow is Widget Lab's chat window as the superadmin sees it,
// by row id.
func widgetWindow(t *testing.T, e *adminEnv) map[int]map[string]any {
	t.Helper()
	b := e.want(200, "superadmin", "GET",
		fmt.Sprintf("/api/chats/group/%d/messages?limit=100", widgetLab), nil)
	out := map[int]map[string]any{}
	for _, m := range asList(b["messages"]) {
		m := m.(map[string]any)
		out[int(m["id"].(float64))] = m
	}
	return out
}

func TestWidgetDetail(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "widgets")
	ms := widgetWindow(t, e)
	for row, want := range map[int]string{
		4501: `{"has_caption":true,"height":240,"html":null,"kind":"widget","widget_id":"9007199254740993","width":320}`,
		4502: `{"has_caption":false,"height":0,"html":null,"kind":"widget","widget_id":null,"width":0}`,
		4503: `{"has_caption":false,"height":80,"html":null,"kind":"widget","widget_id":"42","width":120}`,
		4504: `{"has_caption":false,"height":0,"html":null,"kind":"widget","widget_id":null,"width":0}`,
	} {
		if got := js(detailOf(t, ms, row)); got != want {
			t.Errorf("chat row %d: %s, want %s", row, got, want)
		}
	}
	// The message page sends the HTML, as a string.
	for row, want := range map[int]any{4501: widgetHTML, 4502: nil, 4503: "<p>old</p>", 4504: nil} {
		d, _ := pageMessage(t, e, "superadmin", "group", row)["detail"].(map[string]any)
		if d["kind"] != "widget" || d["html"] != want {
			t.Errorf("page row %d: %v, want html %v", row, d, want)
		}
	}
	d := pageMessage(t, e, "superadmin", "group", 4501)["detail"].(map[string]any)
	if d["width"] != float64(320) || d["height"] != float64(240) || d["has_caption"] != true ||
		d["widget_id"] != "9007199254740993" {
		t.Errorf("page row 4501: %v", d)
	}
}

// The widget's HTML reaches the browser only as data: the message
// page's document embeds it in its JSON script with < and > escaped, so
// it cannot close the element, and the JSON answers are served as
// JSON; nothing serves the HTML as a page of its own.
func TestWidgetHTMLNotServed(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, "widgets")
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: e.cookies["superadmin"]})
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		return rec
	}
	doc := get("/messages/group/4501")
	if doc.Code != 200 || doc.Header().Get("Content-Type") != "text/html; charset=utf-8" ||
		doc.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("document: %d %v", doc.Code, doc.Header())
	}
	body := doc.Body.String()
	if strings.Contains(body, "alert(1)</script>") || strings.Contains(body, "<b>Scores") {
		t.Errorf("the widget's HTML is markup in the document:\n%s", body)
	}
	esc := strings.NewReplacer("<", "\\u003c", ">", "\\u003e").Replace(widgetHTML)
	if !strings.Contains(body, `"html":"`+esc+`"`) {
		t.Errorf("the widget's HTML is not in the page data:\n%s", body)
	}
	for _, path := range []string{"/api/page/messages/group/4501",
		fmt.Sprintf("/api/chats/group/%d/messages?limit=100", widgetLab)} {
		r := get(path)
		if r.Code != 200 || r.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s: %d %v", path, r.Code, r.Header())
		}
		if strings.Contains(r.Body.String(), "<script") {
			t.Errorf("%s: markup in the answer", path)
		}
	}
}

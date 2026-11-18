// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package render turns a page's data into the HTML document the
// service answers with: the client of the render service (protocol
// version 1 over a Unix socket), the client assets from the build's
// manifest, and the document template. The protocol and the template
// are specified in docs/design/web-ssr.md sections 4 to 6.
package render

import "tgloggerd/webapi/internal/settings"

// Protocol is the render protocol version this package speaks.
const Protocol = 1

// MaxBody bounds a render request and a render answer, in bytes.
const MaxBody = 8 << 20

// Account is the signed-in account as pages show it.
type Account struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

// Viewer is what rendering needs to match the browser (the Viewer
// schema of webapi/openapi.yaml). The service fills in every setting,
// the account's own or the site's default; one left empty falls back
// to the client's built-in default.
type Viewer struct {
	SignedIn bool     `json:"signed_in"`
	Account  *Account `json:"account,omitempty"`
	// Now is the server's clock (RFC 3339, UTC), for relative times.
	Now string `json:"now"`
	// Timezone (IANA) and DateFormat (24h, 12h, iso) are how dates,
	// stored and sent in UTC, are shown.
	Timezone   string `json:"timezone,omitempty"`
	DateFormat string `json:"date_format,omitempty"`
	Lang       string `json:"lang,omitempty"`
	Theme      string `json:"theme,omitempty"`
	// Sidebar is whether the menu starts open or collapsed.
	Sidebar     string `json:"sidebar,omitempty"`
	RowsPerPage int    `json:"rows_per_page,omitempty"`
	// Nav lists the route ids of the pages the menu shows.
	Nav []string `json:"nav,omitempty"`
	// Pinned are the account's pinned groups that it may see, in the
	// order pinned.
	Pinned []Pin `json:"pinned,omitempty"`
	// Chat is the chat view's settings.
	Chat *settings.Chat `json:"chat,omitempty"`
}

// Pin is a pinned group in the menu.
type Pin struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// Page is the page data: the object embedded in the document, sent to
// the render service and answered by GET /api/page/{path}.
type Page struct {
	V      int    `json:"v"`
	Route  string `json:"route"`
	URL    string `json:"url"`
	Status int    `json:"status"`
	Props  any    `json:"props"`
	Viewer Viewer `json:"viewer"`
}

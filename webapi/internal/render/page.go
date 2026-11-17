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
// schema of webapi/openapi.yaml). Settings left empty fall back to the
// site defaults on both sides.
type Viewer struct {
	SignedIn bool     `json:"signed_in"`
	Account  *Account `json:"account,omitempty"`
	// Now is the server's clock (RFC 3339, UTC), for relative times.
	Now   string `json:"now"`
	Lang  string `json:"lang,omitempty"`
	Theme string `json:"theme,omitempty"`
	// Nav lists the route ids of the pages the menu shows.
	Nav []string `json:"nav,omitempty"`
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

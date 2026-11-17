// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package scope is what one request may see: the request scope of
// docs/design/web-accounts.md section 9. It is built once per request
// and passed to every query over the logger's schema, so visibility is
// decided in the queries and a query without a scope stands out in
// review. Something outside the scope answers 404, never 403, so its
// existence does not leak.
//
// Until accounts get roles (web-roles in docs/TODO.md), every request
// sees everything, so the scope carries nothing yet.
package scope

// Scope is the visibility of one request.
type Scope struct{}

// Everything is the scope of a request that may see every row.
func Everything() Scope {
	return Scope{}
}

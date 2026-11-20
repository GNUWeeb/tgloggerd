// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/http"

	"tgloggerd/webapi/internal/webdb"
)

// Ownership of the resources the administration creates (webdb
// ownership.go, web-accounts.md section 6): the account creating one
// becomes its owner, and deleting one drops its relations.
//
// The resources other than MCP tokens are in the logger's schema and
// their relations in the web's, so the two writes are not one
// transaction. When the relation cannot be written, the resource stays
// without an owner, which counts as the superadmins' and never as
// anyone else's; the error is logged and the operation, already done,
// answers as it would have. Running assign-owner again gives such a
// resource an owner.

// recordOwner makes account uid the owner of res, just created.
func (s *Server) recordOwner(r *http.Request, res webdb.Resource, uid uint64) {
	if err := webdb.SetOwner(r.Context(), s.app, res, uid, webdb.By(uid, clientIP(r))); err != nil {
		s.log.Error("record owner", "resource", res.String(), "owner", uid, "err", err)
	}
}

// dropOwner removes the relations of res, just deleted by account uid.
func (s *Server) dropOwner(r *http.Request, res webdb.Resource, uid uint64) {
	if err := webdb.DropResource(r.Context(), s.app, res, webdb.By(uid, clientIP(r))); err != nil {
		s.log.Error("drop owner", "resource", res.String(), "err", err)
	}
}

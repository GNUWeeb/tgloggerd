// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/auth"
)

// drogonSessionQuery is the statement the Drogon web's AuthFilter
// checks a session with (kSessionQuery in web/src/auth/AuthFilter.cpp),
// read from the source, so the test runs exactly what Drogon runs.
func drogonSessionQuery(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../../../web/src/auth/AuthFilter.cpp")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)kSessionQuery\[\] =\s*((?:"[^"]*"\s*)+);`).FindSubmatch(src)
	if m == nil {
		t.Fatal("kSessionQuery not found in AuthFilter.cpp")
	}
	var q strings.Builder
	for _, part := range regexp.MustCompile(`"([^"]*)"`).FindAllSubmatch(m[1], -1) {
		q.Write(part[1])
	}
	return q.String()
}

// A session ended on this service is refused by the Drogon web's
// check as well; one without a row, or a live one, passes there on
// its epoch alone.
func TestDrogonRefusesEndedSessions(t *testing.T) {
	t.Parallel()
	db := freshAppDB(t)
	q := drogonSessionQuery(t)
	hash, _ := auth.HashPassword("s3cret")
	uid := addUser(t, db, "wulan", hash, "admin", true)
	h := newHandler(db, 5)
	signer := auth.NewSigner(testKey)

	drogon := func(raw string) (epoch uint32, ended int) {
		t.Helper()
		k := auth.SessionKey(raw)
		if err := db.QueryRow(q, hex.EncodeToString(k[:]), uid).Scan(&epoch, &ended); err != nil {
			t.Fatal(err)
		}
		return
	}

	// Signed in on the Drogon web, never seen here: no row.
	unseen := signer.EncodeSession((&auth.Cookies{Signer: signer, Timeout: 3600}).
		NewSession(uid, "wulan", "admin", 0))
	if ep, ended := drogon(unseen); ep != 0 || ended != 0 {
		t.Fatalf("unseen session: epoch %d, ended %d", ep, ended)
	}

	c := &client{t: t, h: h, cookies: map[string]string{}}
	other := &client{t: t, h: h, cookies: map[string]string{}}
	signIn(t, c, "wulan", "s3cret")
	signIn(t, other, "wulan", "s3cret")
	raw := c.cookies[auth.SessionCookie]
	if _, ended := drogon(raw); ended != 0 {
		t.Fatal("a live session reads as ended")
	}
	if a := c.do("POST", "/api/logout", signer.SessionCSRF(raw), nil); a.status != 204 {
		t.Fatalf("logout: %d", a.status)
	}
	if _, ended := drogon(raw); ended != 1 {
		t.Fatal("a signed-out session passes the Drogon check")
	}
	if _, ended := drogon(other.cookies[auth.SessionCookie]); ended != 0 {
		t.Fatal("the other session reads as ended")
	}
}

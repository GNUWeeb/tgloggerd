// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/testdb"
)

// The user page's photo carousel: GET /api/users/{id}/photos and the
// photo_id of the history's photo changes, on the logger fixtures
// "sample" and "photos".

const (
	fotoBanyak = 9299400001 // five photo changes, one a removal
	fotoLain   = 9299400002
)

func newPhotosEnv(t *testing.T) *chatEnv {
	t.Helper()
	ro := testdb.New(t, testdb.Logger, "sample", "photos").DB
	app := testdb.New(t, testdb.Web, "accounts").DB
	signer := auth.NewSigner(testKey)
	e := &chatEnv{t: t, ro: ro, cookies: &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true},
		tokens: auth.NewFileTokens(testKey)}
	e.h = New(Deps{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:      ro,
		App:     app,
		Signer:  signer,
		Cookies: e.cookies,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Tokens:  e.tokens,
		clock:   func() time.Time { return chatNow },
	})
	return e
}

type tPhoto struct {
	ID     uint64
	At     string
	URL    *string
	FileID *uint64 `json:"file_id"`
}

type tPhotos struct {
	Items     []tPhoto
	CurrentID *uint64 `json:"current_id"`
}

func photosPath(id int64) string {
	return fmt.Sprintf("/api/users/%d/photos", id)
}

func TestUserPhotos(t *testing.T) {
	t.Parallel()
	e := newPhotosEnv(t)

	// Every photo newest first (a tie by row id), the removal left
	// out, another user's absent; the file that cannot be served has
	// no URL; the current photo is the newest item with its file.
	var p tPhotos
	e.decode(photosPath(fotoBanyak), e.as("admin"), 200, &p)
	type want struct {
		id   uint64
		at   string
		file uint64
		url  bool
	}
	ws := []want{
		{1005, "2026-07-01T10:00:00Z", 850003, true},
		{1004, "2026-07-01T10:00:00Z", 850001, true},
		{1003, "2026-05-01T10:00:00Z", 850002, false},
		{1001, "2026-03-01T10:00:00Z", 850001, true},
	}
	if len(p.Items) != len(ws) {
		t.Fatalf("items %+v", p.Items)
	}
	for i, w := range ws {
		it := p.Items[i]
		if it.ID != w.id || it.At != w.at || it.FileID == nil || *it.FileID != w.file ||
			(it.URL != nil) != w.url {
			t.Errorf("item %d: %+v, want %+v", i, it, w)
		}
		if w.url && *it.URL != "/files/"+e.tokens.Encrypt(w.file) {
			t.Errorf("item %d: url %s", i, *it.URL)
		}
	}
	if p.CurrentID == nil || *p.CurrentID != 1004 {
		t.Errorf("current_id %v, want 1004", p.CurrentID)
	}
	// The url and file_id keys are there even when null.
	_, _, body := e.get(photosPath(fotoBanyak), e.as("admin"))
	var raw struct{ Items []map[string]json.RawMessage }
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if u, ok := raw.Items[2]["url"]; !ok || string(u) != "null" {
		t.Errorf("unservable url %s", u)
	}

	// Another user's photo is its own; a user whose photo changes
	// were never recorded has none and no current one, although it
	// has a profile photo.
	e.decode(photosPath(fotoLain), e.as("admin"), 200, &p)
	if len(p.Items) != 1 || p.Items[0].ID != 1006 || p.CurrentID == nil || *p.CurrentID != 1006 {
		t.Errorf("foto lain %+v %v", p.Items, p.CurrentID)
	}
	e.decode(photosPath(9299000001), e.as("admin"), 200, &p)
	if len(p.Items) != 0 || p.Items == nil || p.CurrentID != nil {
		t.Errorf("no photos %+v %v", p.Items, p.CurrentID)
	}

	// The history's access: a member sees no one's photos, an unknown
	// user and a bad id are 404, a signed-out request 401.
	e.decode(photosPath(fotoBanyak), e.as("researcher"), 404, nil)
	e.decode(photosPath(9299999999), e.as("admin"), 404, nil)
	e.decode("/api/users/x/photos", e.as("admin"), 404, nil)
	if code, _, _ := e.get(photosPath(fotoBanyak), ""); code != http.StatusUnauthorized {
		t.Errorf("signed out: %d", code)
	}
}

// histPhotoIDs are the photo_id of every photo change, 0 for one
// without, in timeline order.
func histPhotoIDs(t *testing.T, es []map[string]json.RawMessage) []uint64 {
	t.Helper()
	var out []uint64
	for _, h := range es {
		if string(h["kind"]) != `"photo"` {
			if _, ok := h["photo_id"]; ok {
				t.Errorf("photo_id on %s", h["kind"])
			}
			continue
		}
		var id uint64
		if v, ok := h["photo_id"]; ok {
			if err := json.Unmarshal(v, &id); err != nil {
				t.Fatal(err)
			}
		}
		out = append(out, id)
	}
	return out
}

func TestUserHistoryPhotoID(t *testing.T) {
	t.Parallel()
	e := newPhotosEnv(t)
	want := fmt.Sprint([]uint64{1005, 1004, 1003, 0, 1001})

	// The API's page and the user page and history page carry the
	// photos' ids, the same as the photos answer's; a removal has
	// none.
	var a struct{ Entries []map[string]json.RawMessage }
	e.decode(histPath("users", fotoBanyak, "", 0), e.as("admin"), 200, &a)
	if got := fmt.Sprint(histPhotoIDs(t, a.Entries)); got != want {
		t.Errorf("api history photo ids %s, want %s", got, want)
	}
	type props struct{ History []map[string]json.RawMessage }
	for _, pg := range []struct{ path, route string }{
		{fmt.Sprintf("/users/%d", fotoBanyak), "user"},
		{fmt.Sprintf("/users/%d/history", fotoBanyak), "user_history"},
	} {
		up := page[props](e, pg.path, "admin", 200, pg.route)
		if got := fmt.Sprint(histPhotoIDs(t, up.History)); got != want {
			t.Errorf("%s photo ids %s, want %s", pg.path, got, want)
		}
	}
	var p tPhotos
	e.decode(photosPath(fotoBanyak), e.as("admin"), 200, &p)
	var ids []uint64
	for _, it := range p.Items {
		ids = append(ids, it.ID)
	}
	if fmt.Sprint(ids) != fmt.Sprint([]uint64{1005, 1004, 1003, 1001}) {
		t.Errorf("photos ids %v", ids)
	}

	// A group's photo changes have no photo_id.
	var g struct{ Entries []map[string]json.RawMessage }
	e.decode(histPath("groups", gnuWeeb, "", 0), e.as("admin"), 200, &g)
	n := 0
	for _, h := range g.Entries {
		if string(h["kind"]) == `"photo"` {
			n++
			if _, ok := h["photo_id"]; ok || h["photo"] == nil {
				t.Errorf("group photo change %v", h)
			}
		}
	}
	if n != 1 {
		t.Errorf("group photo changes %d, want 1", n)
	}
}

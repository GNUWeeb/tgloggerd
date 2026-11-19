// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package searchengine

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The engine behind logdb.Search for users' bios and groups'
// descriptions, from the profile indices (profiles_test.go) of the
// fixtures "sample" and "search_engine".

// profileEnv is a searchEnv whose engine also has the profile indices,
// after a pass of each.
func profileEnv(t *testing.T) *searchEnv {
	t.Helper()
	e := newSearchEnv(t)
	ctx := context.Background()
	o := IndexerOptions{Log: quiet, Pause: time.Millisecond}
	ux := testProfileIndex(t, ProfileUsers)
	gx := testProfileIndex(t, ProfileGroups)
	for _, x := range []*Index{ux, gx} {
		ix, err := NewProfileIndexer(e.db.DB, x, nil, o)
		if err != nil {
			t.Fatal(err)
		}
		if err := ix.Pass(ctx); err != nil {
			t.Fatal(err)
		}
		if err := x.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
	}
	e.eng = NewEngine(e.x, EngineOptions{Log: quiet, Users: ux, Groups: gx})
	return e
}

func profileMatches(field, v string) string {
	return `[{"c":"` + field + `","o":"matches","v":"` + v + `"}]`
}

// Bios and descriptions are found with the engine's syntax and
// options, within the request scope, and the answers say the engine
// answered.
func TestProfileSearch(t *testing.T) {
	t.Parallel()
	e := profileEnv(t)
	users, groups := logdb.Users, logdb.Groups

	for _, c := range []struct {
		sc   scope.Scope
		ent  *logdb.Entity
		raw  string
		text logdb.TextOptions
		want []int64
	}{
		{superadmin, users, profileMatches("bio", "kernal~"), logdb.TextOptions{}, []int64{9299000001}},
		{superadmin, users, profileMatches("bio", "gamr"), logdb.TextOptions{Fuzzy: true},
			[]int64{9299000003}},
		{superadmin, users, profileMatches("bio", `\"by night\"`), logdb.TextOptions{},
			[]int64{9299000003}},
		{superadmin, users, profileMatches("bio", "assem*"), logdb.TextOptions{}, []int64{9299000007}},
		{hidingAdmin, users, profileMatches("bio", "+kernel -tea"), logdb.TextOptions{},
			[]int64{9299000001}},
		{superadmin, groups, profileMatches("description", "kernel"), logdb.TextOptions{},
			[]int64{kernel}},
		{superadmin, groups, profileMatches("description", "programing~"), logdb.TextOptions{},
			[]int64{gnuWeeb}},
		// The kernel group is hidden from the admin; the member sees
		// only GNU/Weeb.
		{hidingAdmin, groups, profileMatches("description", "kernel"), logdb.TextOptions{}, nil},
		{member, groups, profileMatches("description", "kernel"), logdb.TextOptions{}, nil},
		{member, groups, profileMatches("description", "rust linux"), logdb.TextOptions{},
			[]int64{gnuWeeb}},
	} {
		res := e.search(t, c.sc, c.ent, c.raw, c.text)
		wantEngine(t, c.raw, res)
		wantRows(t, c.ent.Name+" "+c.raw, res, c.want...)
	}

	// Highlights of the bio; facets are no profile's, so ignored.
	res := e.search(t, superadmin, users, profileMatches("bio", "coffee"),
		logdb.TextOptions{Highlight: true, Facets: true})
	if len(res.Highlights) != 1 || !strings.Contains(strings.Join(res.Highlights[0], " "),
		"<mark>Coffee</mark>") {
		t.Errorf("highlights: %v", res.Highlights)
	}
	if res.Facets != nil || !slices.Contains(res.Backend.Ignored, "facets") {
		t.Errorf("facets %v, backend %+v", res.Facets, res.Backend)
	}

	// Beside a condition the documents do not carry, MySQL pages the
	// engine's ids; sorted by id, the engine pages itself.
	res = e.search(t, superadmin, users, `[{"c":"bio","o":"matches","v":"the","n":"AND"},`+
		`{"c":"type","o":"=","v":"regular"}]`, logdb.TextOptions{})
	wantEngine(t, "with a type", res)
	wantRows(t, "with a type", res, 9299000007)

	// A member sees no bio: the field is no member's.
	if _, err := logdb.Search(context.Background(), e.db.DB, member, users, logdb.SearchRequest{
		Conds: conds(t, profileMatches("bio", "kernel")), Limit: 10, Engine: e.eng,
	}); err == nil {
		t.Error("a member searched a bio")
	}
}

// A profile index whose first pass has not finished is behind: its
// searches go to MySQL, and say why.
func TestProfileBehind(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	ux := testProfileIndex(t, ProfileUsers)
	e.eng = NewEngine(e.x, EngineOptions{Log: quiet, Users: ux})
	res := e.search(t, superadmin, logdb.Users, profileMatches("bio", "kernel"), logdb.TextOptions{})
	if res.Backend == nil || res.Backend.Used != "mysql" || res.Backend.Fallback != logdb.FallbackBehind {
		t.Errorf("backend %+v", res.Backend)
	}
	wantRows(t, "on MySQL", res, 9299000001)
	if !e.eng.Answers(logdb.Users.Name) || e.eng.Answers(logdb.Groups.Name) ||
		!e.eng.Answers(logdb.GroupMessages.Name) {
		t.Error("Answers")
	}
	// Without its index, groups stay on MySQL: no engine part at all.
	res = e.search(t, superadmin, logdb.Groups, profileMatches("description", "kernel"),
		logdb.TextOptions{})
	if res.Backend == nil || res.Backend.Used != "mysql" {
		t.Errorf("groups without an index: %+v", res.Backend)
	}
}

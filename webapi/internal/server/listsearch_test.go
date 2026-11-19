// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
	"tgloggerd/webapi/internal/settings"
)

// The list pages' search over the whole field registry, as the Drogon
// lists offered it: every field and operator, OR, the registry's
// sorts, the page size, the estimate and the timeout state, the
// refusals and the old URLs. Fixtures "sample", "search" and "people":
// 11 users, 7 groups.

func newListSearchEnv(t *testing.T, o searchOpts) *searchEnv {
	t.Helper()
	o.fixtures = append(o.fixtures, "people")
	return newSearchEnvOpts(t, o)
}

func fq(conds string, more ...string) []string {
	return append([]string{"f", conds}, more...)
}

func TestListRegistryConditions(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})

	for _, c := range []struct {
		path  string
		query []string
		total int64
		ids   string
	}{
		// OR binds looser than AND, as in SQL and in Drogon's chain.
		{"users", fq(`[{"field":"type","op":"=","value":"bot","n":"OR"},`+
			`{"field":"type","op":"=","value":"deleted"}]`, "sort", "id", "dir", "asc"),
			2, "9299000005,9299000006"},
		{"users", fq(`[{"field":"msg_count","op":">=","value":"3"},{"field":"is_premium","op":"=",`+
			`"value":"1","n":"OR"},{"field":"type","op":"=","value":"bot"}]`, "sort", "id", "dir", "asc"),
			3, "9299000001,9299000003,9299000005"},
		{"users", fq(`[{"field":"first_name","op":"NOT %LIKE%","value":"a"}]`, "sort", "id", "dir", "asc"),
			5, "9299000001,9299000005,9299000006,9299000007,9299100001"},
		{"users", fq(`[{"field":"username","op":"NOT LIKE","value":"%"}]`, "sort", "id", "dir", "asc"),
			4, "9299000004,9299000006,9299000008,9299200002"},
		{"users", fq(`[{"field":"personal_chat_id","op":"IS NOT NULL"}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"emoji_status","op":"IS NOT NULL","value":""}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"has_photo","op":"IS NULL"}]`), 9, "*"},
		{"users", fq(`[{"field":"paid_star_count","op":">=","value":"250"}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"msg_count","op":"<=","value":"1"}]`), 6, "*"},
		{"users", fq(`[{"field":"hist_first_name","op":"=","value":"Tie"}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"hist_username","op":"%LIKE%","value":"nft"}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"hist_username","op":"!=","value":"persona_old"}]`), 10, "*"},
		{"users", fq(`[{"field":"is_contact","op":"=","value":"1"}]`), 5, "*"},
		{"users", fq(`[{"field":"any_name","op":"%LIKE%","value":"(fixture)"}]`), 1, "9299200001"},
		{"users", fq(`[{"field":"contact_last_name","op":"=","value":"kampus"}]`), 1, "9299000002"},
		{"users", fq(`[{"field":"restriction_reason","op":"%LIKE%","value":"fixture"},` +
			`{"field":"has_sensitive","op":"=","value":"1"},{"field":"restricts_new_chats","op":"=","value":"1"}]`),
			1, "9299200001"},
		{"users", fq(`[{"field":"created_at","op":">=","value":"2026-11-10 11:11:00"}]`), 1, "9299000008"},
		// The page's own date filter takes >= and <= as well.
		{"users", fq(`[{"field":"created_at","op":">=","value":"2026-01-01"}]`, "sort", "id"),
			2, "9299100001,9299000008"},
		{"groups", fq(`[{"field":"hist_title","op":"=","value":"Title 7"}]`), 1, "-1001000200001"},
		{"groups", fq(`[{"field":"has_photo","op":"IS NOT NULL"}]`, "sort", "id"), 2,
			"-1001000000001,-1001000200001"},
		{"groups", fq(`[{"field":"exposed","op":"is","value":"yes","n":"OR"},`+
			`{"field":"type","op":"=","value":"channel"}]`, "sort", "id"), 4, "*"},
		{"private_messages", fq(`[{"field":"is_outgoing","op":"=","value":"1","n":"OR"},` +
			`{"field":"content_type","op":"=","value":"voice"}]`), 3, "2,6,5"},
		{"private_messages", fq(`[{"field":"created_at","op":">=","value":"2000-01-01"}]`), 6, "*"},
		{"group_messages", fq(`[{"field":"sender_user_id","op":"=","value":"9299000007","n":"OR"},` +
			`{"field":"chat_id","op":"=","value":"-1001000000010"}]`), 3, "11,10,16"},
		{"group_messages", fq(`[{"field":"date","op":">=","value":"2026-11-17"},` +
			`{"field":"sender_id","op":"<=","value":"9299000002"}]`), 4, "4,3,2,9"},
	} {
		p := e.props("superadmin", c.path, c.query...)
		wantList(t, c.path+" "+strings.Join(c.query, " "), p, c.total, c.ids)
		if p["error"] != nil {
			t.Errorf("%s %v: error %v", c.path, c.query, p["error"])
		}
	}

	// The connectors are echoed back, AND as the default (no n).
	p := e.props("superadmin", "users", fq(`[{"field":"type","op":"=","value":"bot","n":"OR"},`+
		`{"field":"type","op":"=","value":"deleted","n":"AND"}]`)...)
	if js(p["conds"]) != `[{"field":"type","n":"OR","op":"=","value":"bot"},`+
		`{"field":"type","op":"=","value":"deleted"}]` || p["total_exact"] != true {
		t.Errorf("echo %v %v", p["conds"], p["total_exact"])
	}

	// The registry's sorts.
	for _, c := range []struct {
		path, sort, dir, first string
	}{
		{"users", "paid_star_count", "desc", "9299200001"},
		{"users", "updated_at", "asc", "9299000006"},
		{"users", "first_name", "asc", "9299000006"},
		{"groups", "updated_at", "asc", "-1001000200001"},
		{"private_messages", "created_at", "asc", "*"},
	} {
		p := e.props("superadmin", c.path, "sort", c.sort, "dir", c.dir, "per", "1")
		if p["sort"] != c.sort || p["dir"] != c.dir || (c.first != "*" && listIDs(p) != c.first) {
			t.Errorf("%s sort %s: %v %v %s", c.path, c.sort, p["sort"], p["dir"], listIDs(p))
		}
	}
}

// TestListEveryFieldAndOperator runs one condition of every field the
// list offers with each of its operators: each is taken, and its SQL
// runs. A field that needs a chat gets a chat_id condition beside it.
func TestListEveryFieldAndOperator(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})
	want := map[string]int{"users": 32, "groups": 13, "group_messages": 8, "private_messages": 12}
	for path, nfields := range want {
		p := e.props("superadmin", path)
		fields, _ := p["fields"].([]any)
		if len(fields) != nfields {
			t.Errorf("%s: %d fields, want %d", path, len(fields), nfields)
		}
		n := 0
		for _, fa := range fields {
			f := fa.(map[string]any)
			v := map[string]string{"int": "1", "bool": "1", "datetime": "2026-01-01 00:00:00",
				"text": "a"}[f["type"].(string)]
			if f["type"] == "enum" {
				v, _, _ = strings.Cut(f["enum"].(string), ",")
			}
			for _, op := range f["operators"].([]any) {
				c := fmt.Sprintf(`[{"field":%q,"op":%q,"value":%q}]`, f["key"], op, v)
				if op == "in" {
					c = fmt.Sprintf(`[{"field":%q,"op":"in","value":[%q]}]`, f["key"], v)
				}
				conds := 1
				if f["needs_chat"] == true {
					c = `[{"field":"chat_id","op":"=","value":"1"},` + c[1:]
					conds = 2
				}
				p := e.props("superadmin", path, "f", c)
				if p["error"] != nil || len(p["conds"].([]any)) != conds {
					t.Errorf("%s %s: %v %v", path, c, p["error"], p["conds"])
				}
				n++
			}
		}
		if n < nfields {
			t.Errorf("%s: %d conditions", path, n)
		}
	}
}

// TestListInOperator: "in", one of a list of values, on the message
// lists (through the registry) and the files list (its own filter),
// echoed back as a list; a list where one value belongs, and the other
// way round, is refused.
func TestListInOperator(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})

	gnu := `{"field":"chat_id","op":"=","value":"-1001000000001"},`
	p := e.props("superadmin", "group_messages", fq(`[`+gnu+
		`{"field":"content_type","op":"in","value":["photo","document","sticker"]}]`)...)
	wantList(t, "media in one group", p, 3, "7,5,4")
	if p["error"] != nil || js(p["conds"]) != `[{"field":"chat_id","op":"=","value":"-1001000000001"},`+
		`{"field":"content_type","op":"in","value":["photo","document","sticker"]}]` {
		t.Errorf("in echo: %v %s", p["error"], js(p["conds"]))
	}
	// OR keeps its meaning beside an in.
	p = e.props("superadmin", "private_messages", fq(`[{"field":"content_type","op":"in",`+
		`"value":["voice","service"],"n":"OR"},{"field":"is_outgoing","op":"=","value":"1"}]`)...)
	wantList(t, "private in or", p, 3, "2,6,5")

	p = e.props("superadmin", "files", fq(`[{"field":"file_type","op":"in",`+
		`"value":["voice","sticker"]}]`, "sort", "id", "dir", "asc")...)
	wantList(t, "files in", p, 2, "880004,880005")
	if js(p["conds"]) != `[{"field":"file_type","op":"in","value":["voice","sticker"]}]` {
		t.Errorf("files in echo: %s", js(p["conds"]))
	}
	// The files list drops what it does not take, as it does any
	// other filter.
	p = e.props("superadmin", "files", fq(`[{"field":"file_type","op":"in","value":["voice","x"]}]`)...)
	if js(p["conds"]) != "[]" || p["error"] != nil {
		t.Errorf("files bad in: %s %v", js(p["conds"]), p["error"])
	}

	many := make([]string, logdb.MaxInValues+1)
	for i := range many {
		many[i] = `"text"`
	}
	for _, c := range []struct{ conds, msg string }{
		{`[{"field":"content_type","op":"in","value":"text"}]`,
			"operator 'in' expects a list of values for field 'content_type'"},
		{`[{"field":"content_type","op":"=","value":["text"]}]`,
			"operator '=' takes one value, not a list"},
		{`[{"field":"content_type","op":"in","value":[]}]`, "operator 'in' takes 1 to 128 values"},
		{`[{"field":"content_type","op":"in","value":[` + strings.Join(many, ",") + `]}]`,
			"operator 'in' takes 1 to 128 values"},
		{`[{"field":"content_type","op":"in","value":["text","nope"]}]`,
			"field 'content_type' expects one of: " + logdb.MessageTypes},
		{`[{"field":"chat_id","op":"in","value":["1"]}]`,
			"operator 'in' not allowed for field 'chat_id'"},
		{`[{"field":"content_type","op":"in","value":["` + strings.Repeat("x", 600) + `"]}]`,
			"value too long (max 512)"},
		{`[{"field":"content_type","op":"in","value":[1]}]`, "f must be a JSON array of conditions"},
	} {
		p := e.props("superadmin", "group_messages", fq(c.conds)...)
		if js(p["error"]) != js(map[string]any{"code": "bad_search", "message": c.msg}) {
			t.Errorf("%s: %v, want %q", c.conds, p["error"], c.msg)
		}
	}
}

func TestListRefusals(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})
	five := `[` + strings.Repeat(`{"field":"hist_bio","op":"=","value":"x"},`, 4) +
		`{"field":"hist_phone","op":"=","value":"x"}]`
	for _, c := range []struct {
		query []string
		msg   string
	}{
		{fq(`[{"field":"id","op":"LIKE","value":"1"}]`), "operator 'LIKE' not allowed for field 'id'"},
		{fq(`[{"field":"id","op":"=","value":"1","n":"XOR"}]`), "invalid connector: 'XOR'"},
		{fq(`[{"field":"id","op":"=","value":"x"}]`), "field 'id' expects a numeric value"},
		{fq(`[{"field":"id","op":"nearly","value":"1"}]`), "operator 'nearly' not allowed for field 'id'"},
		{fq(`[{"field":"type","op":"=","value":"alien"}]`),
			"field 'type' expects one of: regular,deleted,bot,unknown"},
		{fq(five), "too many history conditions (max 4)"},
		{fq(`{"field":"id"}`), "f must be a JSON array of conditions"},
		{fq(`[{"field":"id","op":"=","value":"` + strings.Repeat("1", 600) + `"}]`),
			"value too long (max 512)"},
		{[]string{"q", strings.Repeat("a", 600)}, "value too long (max 512)"},
	} {
		p := e.props("superadmin", "users", c.query...)
		wantList(t, c.msg, p, 11, "*")
		if js(p["error"]) != js(map[string]any{"code": "bad_search", "message": c.msg}) ||
			js(p["conds"]) != "[]" || p["q"] != "" {
			t.Errorf("%s: %v %v %v", c.msg, p["error"], p["conds"], p["q"])
		}
	}
	// A refused search keeps the sort and the page size.
	p := e.props("superadmin", "users", fq(`[{"field":"nope","op":"=","value":"1"}]`, "sort", "id",
		"dir", "asc", "per", "2")...)
	if listIDs(p) != "9299000001,9299000002" || p["sort"] != "id" || num(p["per"]) != 2 {
		t.Errorf("kept state: %s %v %v", listIDs(p), p["sort"], p["per"])
	}
}

func TestListScopeFields(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})
	keys := func(p map[string]any) string {
		var out []string
		for _, f := range p["fields"].([]any) {
			out = append(out, f.(map[string]any)["key"].(string))
		}
		return strings.Join(out, ",")
	}
	sa, ad := e.props("superadmin", "users"), e.props("admin", "users")
	if keys(sa) != keys(ad) || len(sa["fields"].([]any)) != 32 {
		t.Errorf("superadmin and admin fields differ: %s / %s", keys(sa), keys(ad))
	}
	// A member searches users by id, name, username, type and photo
	// only, and sorts by those.
	m := e.props("member", "users")
	if keys(m) != "id,first_name,last_name,username,type,has_photo" {
		t.Errorf("member fields %s", keys(m))
	}
	p := e.props("member", "users", fq(`[{"field":"bio","op":"%LIKE%","value":"a"}]`)...)
	if js(p["error"]) != `{"code":"bad_search","message":"unknown field: 'bio'"}` {
		t.Errorf("member bio: %v", p["error"])
	}
	p = e.props("member", "users", fq(`[{"field":"first_name","op":"%LIKE%","value":"a"}]`,
		"sort", "paid_star_count")...)
	if p["error"] != nil || len(p["conds"].([]any)) != 1 || p["sort"] != "name" || num(p["total"]) != 0 {
		t.Errorf("member first_name: %v %v %v", p["error"], p["conds"], p["sort"])
	}
	// The member's other lists: groups (none granted yet) with the
	// groups' fields; private messages are not its page.
	if p := e.props("member", "groups"); len(p["fields"].([]any)) != 13 || num(p["total"]) != 0 {
		t.Errorf("member groups: %v", p["total"])
	}
	if st, _ := e.page("member", "private_messages"); st != 404 {
		t.Errorf("member private messages: %d", st)
	}
	// Files keep their own filters and list no registry fields.
	if p := e.props("superadmin", "files"); p["fields"] != nil {
		t.Errorf("files fields: %v", p["fields"])
	}
}

func TestListPageSizeAndEstimate(t *testing.T) {
	t.Parallel()
	fifty := 50
	e := newListSearchEnv(t, searchOpts{prefs: func(_ context.Context, uid uint64) (settings.Prefs, error) {
		if uid == 1 {
			return settings.Prefs{RowsPerPage: &fifty}, nil
		}
		return settings.Prefs{}, nil
	}})
	for _, c := range []struct {
		role, path string
		query      []string
		per        int64
	}{
		{"superadmin", "users", nil, 50}, // the account's rows_per_page
		{"admin", "users", nil, 25},      // the site's
		{"superadmin", "users", []string{"per", "10"}, 10},
		{"superadmin", "group_messages", nil, 50},
		{"admin", "group_messages", nil, 25},
		{"admin", "users", []string{"per", "1000"}, 1000},
		{"admin", "users", []string{"per", "5000"}, 1000},
	} {
		if p := e.props(c.role, c.path, c.query...); num(p["per"]) != c.per {
			t.Errorf("%s %s %v: per %v, want %d", c.role, c.path, c.query, p["per"], c.per)
		}
	}
	// group_messages without conditions reports the table's row
	// estimate; with one it counts.
	if p := e.props("admin", "group_messages"); p["total_exact"] != false {
		t.Errorf("browse estimate: %v", p["total_exact"])
	}
	if p := e.props("admin", "group_messages", fq(`[{"field":"chat_id","op":"=","value":"-1001000000001"}]`)...); p["total_exact"] != true || num(p["total"]) != 8 {
		t.Errorf("counted: %v %v", p["total_exact"], p["total"])
	}
}

func TestListTimeout(t *testing.T) {
	t.Parallel()
	// The search runs out of time for a condition or for the sort by
	// paid stars; with always, for everything.
	always := false
	slow := func(ctx context.Context, db *sql.DB, sc scope.Scope, en *logdb.Entity,
		req logdb.SearchRequest) (*logdb.SearchResult, error) {
		if always || len(req.Conds) > 0 || len(req.And) > 0 || req.Sort == "paid_star_count" {
			return nil, logdb.ErrSearchTimeout
		}
		return logdb.Search(ctx, db, sc, en, req)
	}
	e := newListSearchEnv(t, searchOpts{search: slow})
	timeout := js(map[string]any{"code": "search_timeout", "message": logdb.ErrSearchTimeout.Error()})

	p := e.props("superadmin", "users", fq(`[{"field":"bio","op":"%LIKE%","value":"a"}]`, "sort", "id")...)
	wantList(t, "timed out filter", p, 11, "*")
	if js(p["error"]) != timeout || js(p["conds"]) != "[]" || p["sort"] != "id" {
		t.Errorf("filter: %v %v %v", p["error"], p["conds"], p["sort"])
	}
	p = e.props("superadmin", "group_messages", "q", "build")
	if js(p["error"]) != timeout || p["q"] != "" || p["sort"] != "date" {
		t.Errorf("text: %v %v %v", p["error"], p["q"], p["sort"])
	}
	p = e.props("superadmin", "users", "sort", "paid_star_count")
	if js(p["error"]) != timeout || p["sort"] != "msg_count" || num(p["total"]) != 11 {
		t.Errorf("sort: %v %v %v", p["error"], p["sort"], p["total"])
	}
	always = true
	p = e.props("superadmin", "users")
	if js(p["error"]) != timeout || num(p["total"]) != 0 || p["total_exact"] != false ||
		len(p["rows"].([]any)) != 0 || num(p["pages"]) != 1 {
		t.Errorf("plain list: %v", p)
	}
}

func TestListOldURLs(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})
	pageOf := func(path string, q url.Values) (string, map[string]any) {
		t.Helper()
		st, b := e.get("superadmin", "/api/page/"+path+"?"+q.Encode())
		if st != 200 {
			t.Fatalf("%s %v: %d", path, q, st)
		}
		return b["url"].(string), b["props"].(map[string]any)
	}
	f := `[{"field":"type","op":"=","value":"bot","n":"OR"},{"field":"type","op":"=","value":"deleted"}]`
	u, p := pageOf("users", url.Values{
		"search": {`[{"c":"type","o":"=","v":"bot","n":"OR"},{"c":"type","o":"=","v":"deleted","n":"AND"}]`},
		"offset": {"1"}, "limit": {"1"}, "sort": {"id"}, "order": {"asc"}})
	want := "/users?" + url.Values{"f": {f}, "page": {"2"}, "per": {"1"}, "sort": {"id"},
		"dir": {"asc"}}.Encode()
	if u != want || listIDs(p) != "9299000006" || num(p["page"]) != 2 ||
		js(p["conds"]) != `[{"field":"type","n":"OR","op":"=","value":"bot"},{"field":"type","op":"=","value":"deleted"}]` {
		t.Errorf("translated:\n got %s %s %v\nwant %s", u, listIDs(p), p["conds"], want)
	}
	// Drogon's IS NULL took no value; its default page size was 50.
	u, p = pageOf("groups", url.Values{"search": {`[{"c":"has_photo","o":"IS NOT NULL"}]`},
		"offset": {"50"}})
	if !strings.Contains(u, "per=50") || !strings.Contains(u, "page=2") || num(p["total"]) != 2 ||
		p["error"] != nil {
		t.Errorf("groups: %s %v %v", u, p["total"], p["error"])
	}
	u, p = pageOf("users", url.Values{"limit": {"5"}})
	if u != "/users?per=5" || num(p["per"]) != 5 {
		t.Errorf("limit only: %s %v", u, p["per"])
	}
	u, p = pageOf("users", url.Values{"search": {"not json"}})
	if u != "/users?search=not+json" ||
		js(p["error"]) != `{"code":"bad_search","message":"search must be valid JSON"}` {
		t.Errorf("bad search: %s %v", u, p["error"])
	}
	// A URL of this list's own is left alone, an old parameter or not.
	u, p = pageOf("users", url.Values{"per": {"3"}, "limit": {"5"}})
	if u != "/users?limit=5&per=3" || num(p["per"]) != 3 {
		t.Errorf("own URL: %s %v", u, p["per"])
	}
}

func TestListColumns(t *testing.T) {
	t.Parallel()
	e := newListSearchEnv(t, searchOpts{})
	p := e.props("superadmin", "users", fq(`[{"field":"id","op":"=","value":"9299200001"}]`)...)
	r := p["rows"].([]any)[0].(map[string]any)
	for k, v := range map[string]any{"phone": "+62 800 0000 0001", "bio": "Every field, for tests.",
		"restriction_reason":    "Restricted on some platforms (fixture)",
		"has_sensitive_content": true, "restricts_new_chats": true,
		"paid_message_star_count": 250, "personal_chat_id": -1001000000001,
		"emoji_status": "5373141891321699086", "contact_name": "Persona (fixture)"} {
		if js(r[k]) != js(v) {
			t.Errorf("persona %s: %s, want %s", k, js(r[k]), js(v))
		}
	}
	p = e.props("admin", "groups", fq(`[{"field":"id","op":"=","value":"-1001000200001"}]`)...)
	if g := p["rows"].([]any)[0].(map[string]any); g["updated_at"] != "2026-11-10T10:00:00Z" ||
		g["created_at"] != "2025-12-01T08:00:00Z" {
		t.Errorf("group row %v", g)
	}
}

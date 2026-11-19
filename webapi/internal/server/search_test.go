// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/auth"
	"tgloggerd/webapi/internal/settings"
	"tgloggerd/webapi/internal/testdb"
	"tgloggerd/webapi/internal/webdb"
)

// The advanced search API, the chat search and the list pages, on the
// logger fixtures "sample" and "search" (internal/testdb), each test
// in a database of its own. The expected answers follow the Drogon
// web's dao::search (web/src/dao/Search.cpp): the same rows, cells and
// totals, escaped the same way. Sessions are checked against a fake
// account table: 1 is a superadmin, 2 an admin, 3 a member.

type searchEnv struct {
	t       *testing.T
	h       http.Handler
	tokens  *auth.FileTokens
	cookies map[string]string // by role
}

func newSearchEnv(t *testing.T) *searchEnv {
	t.Helper()
	return newSearchEnvOpts(t, searchOpts{})
}

// searchOpts are a searchEnv's extras: more logger fixtures, the
// accounts' preferences and a replacement of the lists' search.
type searchOpts struct {
	fixtures []string
	prefs    func(ctx context.Context, uid uint64) (settings.Prefs, error)
	search   searchFunc
}

func newSearchEnvOpts(t *testing.T, o searchOpts) *searchEnv {
	t.Helper()
	db := interpolating(t, testdb.New(t, testdb.Logger, append([]string{"sample", "search"},
		o.fixtures...)...))
	signer := auth.NewSigner(testKey)
	ck := &auth.Cookies{Signer: signer, Timeout: 3600, Secure: true}
	e := &searchEnv{t: t, tokens: auth.NewFileTokens(testKey), cookies: map[string]string{}}
	roles := map[uint64]string{1: "superadmin", 2: "admin", 3: "member"}
	e.h = New(Deps{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		RO:  db,
		// Signed-in requests record their session's use in the web's
		// own database.
		App:     testdb.New(t, testdb.Web, "accounts").DB,
		Signer:  signer,
		Cookies: ck,
		Limiter: auth.NewRateLimiter(5, time.Hour),
		Tokens:  e.tokens,
		state: func(_ context.Context, uid uint64, _ [32]byte) (webdb.SessionState, error) {
			r, ok := roles[uid]
			if !ok {
				return webdb.SessionState{}, webdb.ErrNoUser
			}
			return webdb.SessionState{Role: r, Active: true}, nil
		},
		prefs:  o.prefs,
		search: o.search,
	})
	for uid, r := range roles {
		e.cookies[r] = signer.EncodeSession(ck.NewSession(uid, r, r, 0))
	}
	return e
}

// interpolating is a pool on d that puts the values into the
// statements, as the service's pools do (cmd/webapi), so the tests see
// the plans the service gets.
func interpolating(t *testing.T, d *testdb.DB) *sql.DB {
	t.Helper()
	cfg, err := mysql.ParseDSN(d.DSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg.InterpolateParams = true
	c, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	t.Cleanup(func() { db.Close() })
	return db
}

// get asks for path as role ("" for no session) and decodes the JSON
// answer.
func (e *searchEnv) get(role, path string) (int, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if role != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: e.cookies[role]})
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		e.t.Fatalf("GET %s: %d, not JSON: %v", path, rec.Code, err)
	}
	return rec.Code, body
}

// ok asks for path as role and wants a 200.
func (e *searchEnv) ok(role, path string) map[string]any {
	e.t.Helper()
	st, b := e.get(role, path)
	if st != http.StatusOK {
		e.t.Fatalf("%s GET %s: %d %v", role, path, st, b)
	}
	return b
}

func (e *searchEnv) file(id uint64) string { return "/files/" + e.tokens.Encrypt(id) }

// search is the URL of an API search with these conditions.
func search(entity string, conds string, extra ...string) string {
	q := url.Values{}
	if conds != "" {
		q.Set("search", conds)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	return "/api/search/" + entity + "?" + q.Encode()
}

func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func colKeys(b map[string]any) string {
	var ks []string
	for _, c := range b["cols"].([]any) {
		ks = append(ks, c.(map[string]any)["key"].(string))
	}
	return strings.Join(ks, ",")
}

func fieldKeys(b map[string]any) string {
	var ks []string
	for _, f := range b["fields"].([]any) {
		ks = append(ks, f.(map[string]any)["key"].(string))
	}
	return strings.Join(ks, ",")
}

func rows(b map[string]any) []any { return b["rows"].([]any) }

func wantTotal(t *testing.T, what string, b map[string]any, total int64, exact bool) {
	t.Helper()
	if num(b["total"]) != total || b["total_exact"] != exact {
		t.Errorf("%s: total %v exact %v, want %d %v", what, b["total"], b["total_exact"], total, exact)
	}
}

const userCols = "photo,id,name,contact_name,username,type,msg_count,is_verified,is_premium," +
	"is_scam,is_fake,is_support,phone,language,restriction_reason,has_sensitive," +
	"restricts_new_chats,paid_star_count,personal_chat_id,emoji_status,bio,created_at,updated_at"

func TestSearchUsers(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	b := e.ok("superadmin", "/api/search/users")
	if colKeys(b) != userCols {
		t.Errorf("cols %s", colKeys(b))
	}
	// The registry as Drogon lists it; the page loaders' own fields
	// are not part of it.
	if got := fieldKeys(b); !strings.HasPrefix(got, "id,first_name,last_name,any_name,") ||
		!strings.HasSuffix(got, ",hist_bio,hist_phone") {
		t.Errorf("fields %s", got)
	}
	for _, f := range b["fields"].([]any) {
		if f := f.(map[string]any); f["key"] == "type" {
			if js(f) != `{"enum":"regular,deleted,bot,unknown","key":"type","label":"Type",`+
				`"operators":["=","!=","in"],"sortable":true,"type":"enum"}` {
				t.Errorf("type field %s", js(f))
			}
		}
	}
	wantTotal(t, "users", b, 9, true)
	if num(b["limit"]) != 10 || num(b["offset"]) != 0 || num(b["max_offset"]) != 500000 ||
		b["sort"] != "" || b["order"] != "desc" {
		t.Errorf("paging %v %v %v %v %v", b["limit"], b["offset"], b["max_offset"], b["sort"], b["order"])
	}
	// id descending, the escaped name first.
	first := rows(b)[0].([]any)
	if num(first[1]) != 9299100001 ||
		first[2] != "&lt;i&gt;Tom &amp; &quot;Jerry&quot;&lt;/i&gt; O&#39;Neil" ||
		first[4] != "tom_jerry" || first[0] != "" {
		t.Errorf("first row %v", first)
	}

	b = e.ok("superadmin", search("users", `[{"c":"id","o":"=","v":"9299000001"}]`))
	want := []any{e.file(880007), 9299000001, "Dewi Kusuma", "Dewi (kernel)", "dewikusuma",
		"regular", "6", false, true, false, false, false, "+62 812 5550 1234", "id", "",
		false, false, "0", "", "", "Kernel hacker. Coffee first. She/her.",
		"2023-02-11 07:30:00", "2026-11-17 15:31:00"}
	if len(rows(b)) != 1 || js(rows(b)[0]) != js(want) {
		t.Errorf("Dewi:\n got %s\nwant %s", js(rows(b)), js(want))
	}

	for _, c := range []struct {
		conds string
		total int64
	}{
		{`[{"c":"is_premium","o":"=","v":"1"}]`, 3},
		{`[{"c":"type","o":"=","v":"bot","n":"OR"},{"c":"is_premium","o":"=","v":"1"}]`, 4},
		// The contact name.
		{`[{"c":"any_name","o":"%LIKE%","v":"(kernel)"}]`, 1},
		// LIKE is verbatim, %LIKE% escapes the value's own wildcards.
		{`[{"c":"first_name","o":"LIKE","v":"D%"}]`, 1},
		{`[{"c":"first_name","o":"%LIKE%","v":"%"}]`, 0},
		{`[{"c":"username","o":"%LIKE%","v":"sari"}]`, 1},
		// != on an EXISTS field is "never had it".
		{`[{"c":"username","o":"!=","v":"sariw"}]`, 8},
		{`[{"c":"username","o":"=","v":"sari_gw"}]`, 0},
		// A missing extra-info row matches the defaults.
		{`[{"c":"bio","o":"=","v":""}]`, 6},
		{`[{"c":"personal_chat_id","o":"IS NULL"}]`, 9},
		{`[{"c":"created_at","o":">=","v":"2026-01-01"}]`, 2},
		{`[{"c":"has_photo","o":"IS NOT NULL"}]`, 1},
	} {
		b := e.ok("superadmin", search("users", c.conds))
		wantTotal(t, c.conds, b, c.total, true)
		if int64(len(rows(b))) != min(c.total, 10) {
			t.Errorf("%s: %d rows", c.conds, len(rows(b)))
		}
	}

	// Sorting, and the parameters read as Drogon reads them.
	b = e.ok("superadmin", search("users", "", "sort", "msg_count", "order", "ASC", "limit", "2"))
	if b["sort"] != "msg_count" || b["order"] != "asc" || len(rows(b)) != 2 ||
		num(rows(b)[0].([]any)[1]) != 9299100001 {
		t.Errorf("sorted: %v %v %v", b["sort"], b["order"], rows(b))
	}
	b = e.ok("superadmin", search("users", "", "sort", "bio", "limit", "abc", "offset", "-5"))
	if b["sort"] != "" || num(b["limit"]) != 1 || num(b["offset"]) != 0 {
		t.Errorf("unsortable field and atoi: %v %v %v", b["sort"], b["limit"], b["offset"])
	}
	b = e.ok("superadmin", search("users", "", "limit", "5000", "offset", "9999999"))
	if num(b["limit"]) != 1000 || num(b["offset"]) != 500000 || len(rows(b)) != 0 {
		t.Errorf("clamped: %v %v", b["limit"], b["offset"])
	}
	// A page past the end still counts exactly.
	wantTotal(t, "past the end", b, 9, true)
}

func TestSearchRefused(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	var many, histories []string
	for i := 0; i < 17; i++ {
		many = append(many, `{"c":"id","o":">","v":"1","n":"AND"}`)
	}
	for i := 0; i < 5; i++ {
		histories = append(histories, `{"c":"hist_bio","o":"LIKE","v":"x","n":"AND"}`)
	}
	for _, c := range []struct{ conds, msg string }{
		{`notjson`, "search must be valid JSON"},
		{`{}`, "search must be a JSON array"},
		{`[1]`, "each condition must be a JSON object"},
		{`[{"o":"="}]`, "missing 'c' in a condition"},
		{`[{"c":"id","o":1}]`, "'o' must be a string"},
		{`[{"c":"id","o":"=","v":1}]`, "'v' must be a string or a list of strings"},
		{`[{"c":"type","o":"in","v":["bot",1]}]`, "'v' must be a string or a list of strings"},
		{`[{"c":"type","o":"in","v":"bot"}]`, "operator 'in' expects a list of values for field 'type'"},
		{`[{"c":"type","o":"=","v":["bot"]}]`, "operator '=' takes one value, not a list"},
		{`[{"c":"type","o":"in","v":["bot","robot"]}]`,
			"field 'type' expects one of: regular,deleted,bot,unknown"},
		{`[{"c":"id","o":"in","v":["1"]}]`, "operator 'in' not allowed for field 'id'"},
		{`[{"c":"nope","o":"="}]`, "unknown field: 'nope'"},
		{`[{"c":"full_name","o":"=","v":"x"}]`, "unknown field: 'full_name'"},
		{`[{"c":"id","o":"LIKE","v":"1"}]`, "operator 'LIKE' not allowed for field 'id'"},
		{`[{"c":"id","o":"=","v":"x"}]`, "field 'id' expects a numeric value"},
		{`[{"c":"id","o":"="}]`, "field 'id' requires a value"},
		{`[{"c":"created_at","o":"=","v":"today"}]`, "field 'created_at' expects a date/time value"},
		{`[{"c":"type","o":"=","v":"robot"}]`, "field 'type' expects one of: regular,deleted,bot,unknown"},
		{`[{"c":"id","o":"=","v":"1","n":"XOR"},{"c":"id","o":"=","v":"2"}]`, "invalid connector: 'XOR'"},
		{`[{"c":"bio","o":"=","v":"` + strings.Repeat("x", 513) + `"}]`, "value too long (max 512)"},
		{"[" + strings.Join(many, ",") + "]", "too many conditions (max 16)"},
		{"[" + strings.Join(histories, ",") + "]", "too many history conditions (max 4)"},
		{"[" + strings.Repeat(" ", 8192) + "]", "search parameter too large"},
	} {
		st, b := e.get("superadmin", search("users", c.conds))
		if st != 400 || b["error"] != "bad_search" || b["message"] != c.msg {
			t.Errorf("%.40s: %d %v, want %q", c.conds, st, b, c.msg)
		}
	}
	if st, b := e.get("superadmin", "/api/search/accounts"); st != 404 || b["error"] != "not_found" {
		t.Errorf("unknown entity: %d %v", st, b)
	}
	if st, _ := e.get("", "/api/search/users"); st != 401 {
		t.Errorf("signed out: %d", st)
	}
}

func TestSearchGroupsAndFiles(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	b := e.ok("admin", search("groups", "", "sort", "title", "order", "asc", "limit", "1"))
	wantTotal(t, "groups", b, 6, true)
	want := []any{"", -1001000000005, "Assembly Lounge", "asm_lounge", "channel", "2",
		"x86-64, ARM64 and RISC-V assembly.", "1", "2023-04-04 09:00:00", "2026-11-17 14:00:00"}
	if colKeys(b) != "photo,id,title,username,type,msg_count,description,admin_count,created_at,updated_at" ||
		js(rows(b)[0]) != js(want) {
		t.Errorf("groups: %s\n%s", colKeys(b), js(rows(b)))
	}
	b = e.ok("admin", search("groups", `[{"c":"id","o":"=","v":"-1001000000001"}]`))
	if r := rows(b)[0].([]any); r[0] != e.file(880008) || r[7] != "2" {
		t.Errorf("GNU/Weeb: %v", r)
	}
	b = e.ok("admin", search("groups", `[{"c":"hist_title","o":"LIKE","v":"x"}]`))
	wantTotal(t, "history", b, 0, true)

	sum := sha256.Sum256([]byte("fixture photo 880001"))
	b = e.ok("admin", search("files", `[{"c":"sha256","o":"=","v":"`+hex.EncodeToString(sum[:])+`"}]`))
	want = []any{e.file(880001), 880001, "photo", "", "jpg", "179.1 KB", "12", true,
		"2026-11-17 14:05:00", "2026-11-17 15:30:00", "AgACAgUAAxkBAAIfixture01",
		strings.ToUpper(hex.EncodeToString(sum[:]))}
	if colKeys(b) != "thumb,id,file_type,name,ext,size,hits,stored,created_at,accessed_at,tg_file_id,sha256" ||
		len(rows(b)) != 1 || js(rows(b)[0]) != js(want) {
		t.Errorf("file by digest: %s\n got %s\nwant %s", colKeys(b), js(rows(b)), js(want))
	}
	b = e.ok("admin", search("files", `[{"c":"sha256","o":"=","v":"zz"}]`))
	wantTotal(t, "bad hex", b, 0, true)
	b = e.ok("admin", search("files", `[{"c":"stored","o":"=","v":"0"}]`, "sort", "size"))
	wantTotal(t, "not stored", b, 3, true)
	if r := rows(b)[0].([]any); r[1] != float64(880003) || r[5] != "50.0 MB" || r[11] != "" {
		t.Errorf("biggest unstored: %v", r)
	}
}

func TestSearchMessages(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)

	// Browsing the group messages is never counted: the table's row
	// estimate, at least what the page shows.
	b := e.ok("superadmin", "/api/search/group_messages")
	if b["total_exact"] != false || num(b["total"]) < 11 {
		t.Errorf("browse: %v %v", b["total"], b["total_exact"])
	}
	b = e.ok("superadmin", search("group_messages", `[{"c":"chat_id","o":"=","v":"-1001000000001"}]`))
	wantTotal(t, "one group", b, 8, true)
	if colKeys(b) != "group,sender,content_type,text,forwarded,message_id,sent,deleted" {
		t.Errorf("cols %s", colKeys(b))
	}
	gnu := map[string]any{"id": -1001000000001, "kind": "group", "name": "GNU/Weeb",
		"photo": e.file(880008), "username": ""}
	want := []any{gnu, map[string]any{"id": 9299000008, "kind": "user", "name": "Fajar Halim",
		"photo": "", "username": ""}, "service", "", false, fmt.Sprint(int64(410008) << 20),
		"2026-11-17 15:40:00", false}
	if js(rows(b)[0]) != js(want) {
		t.Errorf("newest:\n got %s\nwant %s", js(rows(b)[0]), js(want))
	}

	// A fulltext match orders by relevance unless a sort is asked for.
	b = e.ok("admin", search("group_messages", `[{"c":"text","o":"matches","v":"+build +arm64"}]`,
		"debug", "1"))
	wantTotal(t, "fulltext", b, 1, true)
	dbg := b["debug"].(map[string]any)
	if b["sort"] != "" || !strings.Contains(dbg["sql"].(string),
		"ORDER BY MATCH(m.text) AGAINST(? IN BOOLEAN MODE) DESC LIMIT 10 OFFSET 0") ||
		js(dbg["bind"]) != `["+build +arm64","+build +arm64"]` ||
		len(dbg["explain"].(map[string]any)["rows"].([]any)) == 0 {
		t.Errorf("relevance: %v %v", b["sort"], dbg)
	}
	// The fulltext index yields the rows in relevance order, so no
	// sort runs over the hits.
	if !strings.Contains(js(dbg["explain"]), "Ft_hints: sorted") {
		t.Errorf("relevance sort not from the index: %s", js(dbg["explain"]))
	}
	// The plan reads as MySQL writes it, filtered with its decimals.
	if !strings.Contains(js(dbg["explain"]), `"100.00"`) {
		t.Errorf("plan values: %s", js(dbg["explain"]))
	}
	if r := rows(b)[0].([]any); r[3] != "the build is broken on arm64 again, see the log" {
		t.Errorf("match: %v", r)
	}
	b = e.ok("admin", search("group_messages", `[{"c":"text","o":"matches","v":"generics"}]`))
	wantTotal(t, "one word", b, 1, true)
	b = e.ok("admin", search("group_messages", `[{"c":"chat_id","o":"=","v":"-1001000000004"}]`))
	if r := rows(b)[0].([]any); r[3] != "templates &amp; &lt;generics&gt; in C++" || r[4] != true {
		t.Errorf("escaped text: %v", r)
	}
	// A channel post names the channel as its sender.
	if st, _ := e.get("admin", search("group_messages", `[{"c":"message_id","o":"=","v":"1"}]`)); st != 400 {
		t.Errorf("message_id is not a group message field: %d", st)
	}
	b = e.ok("admin", search("group_messages", `[{"c":"chat_id","o":"=","v":"-1001000000005"}]`,
		"sort", "date", "order", "asc"))
	if s := rows(b)[0].([]any)[1].(map[string]any); s["kind"] != "group" || s["name"] != "Assembly Lounge" {
		t.Errorf("channel post: %v", s)
	}

	// "in": one placeholder per value.
	b = e.ok("admin", search("group_messages", `[{"c":"chat_id","o":"=","v":"-1001000000001","n":"AND"},`+
		`{"c":"content_type","o":"in","v":["photo","document"]}]`, "debug", "1"))
	wantTotal(t, "in", b, 2, true)
	dbg = b["debug"].(map[string]any)
	if !strings.Contains(dbg["sql"].(string), "(m.content_type IN (?, ?))") ||
		!strings.HasSuffix(js(dbg["bind"]), `"photo","document"]`) {
		t.Errorf("in: %v %v", dbg["sql"], dbg["bind"])
	}
	b = e.ok("admin", search("users", `[{"c":"type","o":"in","v":["bot","deleted"]}]`))
	wantTotal(t, "users in", b, 2, true)

	b = e.ok("superadmin", search("private_messages", `[{"c":"deleted","o":"IS NOT NULL"}]`))
	wantTotal(t, "deleted", b, 1, true)
	want = []any{map[string]any{"id": 9299000001, "kind": "user", "name": "Dewi Kusuma",
		"photo": e.file(880007), "username": "dewikusuma"}, false, "text", "never mind", false,
		fmt.Sprint(int64(610004) << 20), "2026-11-17 15:31:40", true}
	if js(rows(b)[0]) != js(want) {
		t.Errorf("private:\n got %s\nwant %s", js(rows(b)[0]), js(want))
	}
}

// Every entity through each role: a member sees no group yet, so no
// row, user, file or message, and its totals say so; private messages
// do not exist for it. Admins see what superadmins see.
func TestSearchScope(t *testing.T) {
	t.Parallel()
	e := newSearchEnv(t)
	all := map[string]int64{"users": 9, "groups": 6, "files": 8, "private_messages": 6}
	for entity, n := range all {
		for _, role := range []string{"superadmin", "admin"} {
			b := e.ok(role, "/api/search/"+entity)
			wantTotal(t, role+" "+entity, b, n, true)
		}
	}
	for _, entity := range []string{"users", "groups", "files", "group_messages"} {
		b := e.ok("member", search(entity, "", "debug", "1"))
		// Counted, never the table's estimate, which would count
		// rows the member does not see.
		wantTotal(t, "member "+entity, b, 0, true)
		if len(rows(b)) != 0 || b["debug"] != nil {
			t.Errorf("member %s: %v %v", entity, rows(b), b["debug"])
		}
	}
	b := e.ok("member", search("group_messages", `[{"c":"chat_id","o":"=","v":"-1001000000001"}]`))
	wantTotal(t, "member, one group", b, 0, true)

	// A member sees users by name and photo only.
	b = e.ok("member", "/api/search/users")
	if colKeys(b) != "photo,id,name,username,type" ||
		fieldKeys(b) != "id,first_name,last_name,username,type,has_photo" {
		t.Errorf("member users: %s / %s", colKeys(b), fieldKeys(b))
	}
	for _, c := range []string{`[{"c":"bio","o":"=","v":""}]`, `[{"c":"hist_bio","o":"LIKE","v":"x"}]`} {
		if st, b := e.get("member", search("users", c)); st != 400 || b["message"] == nil {
			t.Errorf("member %s: %d %v", c, st, b)
		}
	}
	if b := e.ok("member", search("users", "", "sort", "msg_count")); b["sort"] != "" {
		t.Errorf("member sorted by a hidden field: %v", b["sort"])
	}
	if st, b := e.get("member", "/api/search/private_messages"); st != 404 || b["error"] != "not_found" {
		t.Errorf("member private messages: %d %v", st, b)
	}
	if b := e.ok("superadmin", search("groups", "", "debug", "1")); b["debug"] == nil {
		t.Errorf("superadmin debug missing")
	}
}

func TestAtoi(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"": 0, "12": 12, " 7x": 7, "-3": -3, "+4": 4, "x1": 0,
		"99999999999999": 2147483647} {
		if got := atoi(in); got != want {
			t.Errorf("atoi(%q) = %d, want %d", in, got, want)
		}
	}
}

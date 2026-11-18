// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The list pages (users, groups, files, group_messages,
// private_messages): their loaders read the page's list state from
// the query string (q, f, sort, dir, page, per, as web/app's
// useListNav writes it), run it through the advanced search
// (logdb.Search) within the request scope, and answer the rows in the
// shapes of the React pages (UsersProps, GroupsProps, FilesProps,
// MessagesProps in webapi/openapi.yaml).

// listCond is a filter of a list page (Cond).
type listCond struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// listState is the list state the rows were read with (ListState).
type listState struct {
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Pages int        `json:"pages"`
	Per   int        `json:"per"`
	Sort  string     `json:"sort"`
	Dir   string     `json:"dir"`
	Q     string     `json:"q"`
	Conds []listCond `json:"conds"`
	// ArchiveTotal is everything in the archive, for viewers who see
	// all of it only.
	ArchiveTotal *int64 `json:"archive_total,omitempty"`
}

// maxListPer is the most rows a list page shows.
const maxListPer = 100

// relevanceSort is the sort a message list reports when its search
// text orders it by relevance.
const relevanceSort = "relevance"

// pageFilter turns one filter of a page into search conditions; ok is
// false for one the field does not take, which is dropped.
type pageFilter func(sc scope.Scope, op, value string) ([]logdb.Cond, bool)

// listSpec is one list page.
type listSpec struct {
	entity     *logdb.Entity
	defSort    func(sc scope.Scope) string
	defPer     int
	sorts      map[string]string // the page's sort keys: field keys
	filters    map[string]pageFilter
	search     func(sc scope.Scope, q string) []logdb.Cond
	limitedUse map[string]bool // filters and sorts of a viewer who sees in part
	// relevance: the search text is a fulltext match, which orders
	// the rows by relevance unless a sort is asked for.
	relevance bool
}

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func cond(c, o, v string) logdb.Cond {
	return logdb.Cond{C: c, O: o, V: v, N: "AND", HasV: true}
}

func isInt(v string) bool {
	_, err := strconv.ParseInt(v, 10, 64)
	return err == nil
}

func numberFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		switch op {
		case "=", "!=", ">", "<":
			if isInt(v) {
				return []logdb.Cond{cond(key, op, v)}, true
			}
		}
		return nil, false
	}
}

// textFilter is a text field: contains, equals, or empty.
func textFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		switch op {
		case "contains":
			return []logdb.Cond{cond(key, "%LIKE%", v)}, true
		case "=":
			return []logdb.Cond{cond(key, "=", v)}, true
		case "empty":
			return []logdb.Cond{cond(key, "=", "")}, true
		}
		return nil, false
	}
}

// existsTextFilter is a text field held in another table (a current
// username): empty is none at all.
func existsTextFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		switch op {
		case "contains":
			return []logdb.Cond{cond(key, "%LIKE%", v)}, true
		case "=":
			return []logdb.Cond{cond(key, "=", v)}, true
		case "empty":
			return []logdb.Cond{cond(key, "NOT LIKE", "%")}, true
		}
		return nil, false
	}
}

func enumFilter(key, values string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		if (op == "=" || op == "!=") && strings.Contains(","+values+",", ","+v+",") {
			return []logdb.Cond{cond(key, op, v)}, true
		}
		return nil, false
	}
}

func yesNo(v string) (bool, bool) {
	switch v {
	case "yes", "true", "1":
		return true, true
	case "no", "false", "0":
		return false, true
	}
	return false, false
}

func boolFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		b, ok := yesNo(v)
		if op != "is" || !ok {
			return nil, false
		}
		if b {
			return []logdb.Cond{cond(key, "=", "1")}, true
		}
		return []logdb.Cond{cond(key, "=", "0")}, true
	}
}

// nullFilter is a flag held as a nullable column (deleted_at).
func nullFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		b, ok := yesNo(v)
		if op != "is" || !ok {
			return nil, false
		}
		o := "IS NULL"
		if b {
			o = "IS NOT NULL"
		}
		return []logdb.Cond{{C: key, O: o, N: "AND"}}, true
	}
}

// dateFilter compares a DATETIME column with a day, YYYY-MM-DD.
func dateFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		if (op == ">" || op == "<") && dateRE.MatchString(v) {
			if _, err := time.Parse(time.DateOnly, v); err == nil {
				return []logdb.Cond{cond(key, op, v)}, true
			}
		}
		return nil, false
	}
}

// unixDateFilter compares a unix time column with the start of a day
// (UTC), YYYY-MM-DD, as dateFilter does a DATETIME one.
func unixDateFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		if (op == ">" || op == "<") && dateRE.MatchString(v) {
			if t, err := time.Parse(time.DateOnly, v); err == nil {
				return []logdb.Cond{cond(key, op, strconv.FormatInt(t.Unix(), 10))}, true
			}
		}
		return nil, false
	}
}

// fulltextFilter is message text: a word search over the fulltext
// index for contains and equals.
func fulltextFilter(key string) pageFilter {
	return func(_ scope.Scope, op, v string) ([]logdb.Cond, bool) {
		if (op == "contains" || op == "=") && strings.TrimSpace(v) != "" {
			return []logdb.Cond{cond(key, "matches", v)}, true
		}
		return nil, false
	}
}

// or joins conditions into one chain of alternatives.
func or(cs ...logdb.Cond) []logdb.Cond {
	for i := range cs {
		cs[i].N = "OR"
	}
	return cs
}

// nameSearch matches a user's name: own or contact for a viewer who
// sees users in full, only the own one otherwise.
func nameKey(sc scope.Scope) string {
	if logdb.Users.Full(sc) {
		return "any_name"
	}
	return "full_name"
}

var usersList = &listSpec{
	entity: logdb.Users,
	defSort: func(sc scope.Scope) string {
		if logdb.Users.Full(sc) {
			return "msg_count"
		}
		return "name"
	},
	defPer: 25,
	sorts: map[string]string{"name": "first_name", "id": "id", "type": "type",
		"msg_count": "msg_count", "language": "lang", "created_at": "created_at"},
	filters: map[string]pageFilter{
		"id": numberFilter("id"),
		"name": func(sc scope.Scope, op, v string) ([]logdb.Cond, bool) {
			if op == "contains" {
				return []logdb.Cond{cond(nameKey(sc), "%LIKE%", v)}, true
			}
			return textFilter("full_name")(sc, op, v)
		},
		"username":    existsTextFilter("username"),
		"type":        enumFilter("type", "regular,deleted,bot,unknown"),
		"msg_count":   numberFilter("msg_count"),
		"is_premium":  boolFilter("is_premium"),
		"is_verified": boolFilter("is_verified"),
		"is_scam":     boolFilter("is_scam"),
		"is_fake":     boolFilter("is_fake"),
		"language":    textFilter("lang"),
		"bio":         textFilter("bio"),
		"created_at":  dateFilter("created_at"),
	},
	search: func(sc scope.Scope, q string) []logdb.Cond {
		q = strings.TrimPrefix(q, "@")
		cs := []logdb.Cond{cond(nameKey(sc), "%LIKE%", q), cond("username", "%LIKE%", q)}
		if isInt(q) {
			cs = append(cs, cond("id", "=", q))
		}
		return or(cs...)
	},
	limitedUse: map[string]bool{"id": true, "name": true, "username": true, "type": true},
}

var groupsList = &listSpec{
	entity:  logdb.Groups,
	defSort: func(scope.Scope) string { return "msg_count" },
	defPer:  25,
	sorts: map[string]string{"title": "title", "id": "id", "type": "type",
		"msg_count": "msg_count", "admin_count": "admin_count", "created_at": "created_at"},
	filters: map[string]pageFilter{
		"id":          numberFilter("id"),
		"title":       textFilter("title"),
		"username":    existsTextFilter("username"),
		"type":        enumFilter("type", "basic_group,supergroup,channel"),
		"msg_count":   numberFilter("msg_count"),
		"description": textFilter("description"),
		"exposed":     boolFilter("exposed"),
		"created_at":  dateFilter("created_at"),
	},
	search: func(_ scope.Scope, q string) []logdb.Cond {
		q = strings.TrimPrefix(q, "@")
		cs := []logdb.Cond{cond("title", "%LIKE%", q), cond("username", "%LIKE%", q)}
		if isInt(q) {
			cs = append(cs, cond("id", "=", q))
		}
		return or(cs...)
	},
}

var filesList = &listSpec{
	entity:  logdb.Files,
	defSort: func(scope.Scope) string { return "created_at" },
	defPer:  50,
	sorts: map[string]string{"name": "name", "file_type": "file_type", "size": "size",
		"hits": "hits", "created_at": "created_at"},
	filters: map[string]pageFilter{
		"id":         numberFilter("id"),
		"file_type":  enumFilter("file_type", "photo,video,document,audio,voice,sticker,animation,unknown"),
		"name":       textFilter("name"),
		"ext":        textFilter("ext"),
		"size":       numberFilter("size"),
		"hits":       numberFilter("hits"),
		"stored":     boolFilter("stored"),
		"created_at": dateFilter("created_at"),
	},
	search: func(_ scope.Scope, q string) []logdb.Cond {
		cs := []logdb.Cond{cond("name", "%LIKE%", q)}
		if isInt(q) {
			cs = append(cs, cond("id", "=", q))
		}
		return or(cs...)
	},
}

func messageSearch(_ scope.Scope, q string) []logdb.Cond {
	return []logdb.Cond{cond("text", "matches", q)}
}

var groupMessagesList = &listSpec{
	entity:    logdb.GroupMessages,
	defSort:   func(scope.Scope) string { return "date" },
	defPer:    50,
	sorts:     map[string]string{"date": "date"},
	relevance: true,
	filters: map[string]pageFilter{
		"text":         fulltextFilter("text"),
		"content_type": enumFilter("content_type", logdb.MessageTypes),
		"chat_id":      numberFilter("chat_id"),
		"sender_id":    numberFilter("sender_user_id"),
		"is_forwarded": boolFilter("is_forwarded"),
		"is_edited":    boolFilter("is_edited"),
		"is_deleted":   nullFilter("deleted"),
		"date":         unixDateFilter("date"),
	},
	search: messageSearch,
}

var privateMessagesList = &listSpec{
	entity:    logdb.PrivateMessages,
	defSort:   func(scope.Scope) string { return "date" },
	defPer:    50,
	sorts:     map[string]string{"date": "date"},
	relevance: true,
	filters: map[string]pageFilter{
		"text":         fulltextFilter("text"),
		"content_type": enumFilter("content_type", logdb.MessageTypes),
		"chat_id":      numberFilter("chat_id"),
		"is_outgoing":  boolFilter("is_outgoing"),
		"is_forwarded": boolFilter("is_forwarded"),
		"is_edited":    boolFilter("is_edited"),
		"is_deleted":   nullFilter("deleted"),
		"date":         unixDateFilter("date"),
	},
	search: messageSearch,
}

// allowed is whether a viewer may filter or sort the list by key: a
// viewer who sees the rows in part only by the keys it shows.
func (l *listSpec) allowed(sc scope.Scope, key string) bool {
	return l.limitedUse == nil || l.entity.Full(sc) || l.limitedUse[key]
}

// listRun is a list page's search, as the loaders get it back.
type listRun struct {
	state listState
	res   *logdb.SearchResult
}

// run reads the list state from the query, searches, and fills the
// state in. Filters that do not parse are dropped (they are not
// echoed back either); a search the engine refuses falls back to the
// whole list, as the Drogon pages do.
func (l *listSpec) run(ctx context.Context, pr *pageRequest) (*listRun, error) {
	s, sc, q := pr.s, pr.scope, pr.query
	st := listState{Q: strings.TrimSpace(q.Get("q")), Conds: []listCond{}, Page: 1, Per: l.defPer}
	if n, err := strconv.Atoi(q.Get("per")); err == nil && n > 0 {
		st.Per = min(n, maxListPer)
	}
	if n, err := strconv.Atoi(q.Get("page")); err == nil && n > 1 {
		// No deeper than the search's offsets go.
		st.Page = min(n, logdb.MaxOffset/st.Per+1)
	}
	if len(st.Q) > logdb.MaxValueLen {
		st.Q = ""
	}

	var conds []logdb.Cond
	var raw []listCond
	if f := q.Get("f"); f != "" && len(f) <= logdb.MaxSearchBytes {
		_ = json.Unmarshal([]byte(f), &raw)
	}
	for _, c := range raw {
		pf := l.filters[c.Field]
		if pf == nil || !l.allowed(sc, c.Field) || len(c.Value) > logdb.MaxValueLen {
			continue
		}
		if cs, ok := pf(sc, c.Op, c.Value); ok && len(conds)+len(cs) <= logdb.MaxConds/2 {
			conds = append(conds, cs...)
			st.Conds = append(st.Conds, c)
		}
	}

	st.Sort, st.Dir = q.Get("sort"), q.Get("dir")
	if _, ok := l.sorts[st.Sort]; !ok || !l.allowed(sc, st.Sort) {
		st.Sort = ""
	}
	if st.Dir != "asc" {
		st.Dir = "desc"
	}
	explicit := st.Sort != ""
	if !explicit {
		st.Sort = l.defSort(sc)
	}

	req := logdb.SearchRequest{Conds: conds, Order: st.Dir, Internal: true, Limit: st.Per}
	if st.Q != "" {
		req.And = [][]logdb.Cond{l.search(sc, st.Q)}
	}
	// A search text over messages orders by relevance unless a sort
	// was asked for.
	if explicit || st.Q == "" || !l.relevance {
		req.Sort = l.sorts[st.Sort]
	} else {
		st.Sort = relevanceSort
	}

	for attempt := 0; ; attempt++ {
		req.Offset = (st.Page - 1) * st.Per
		res, err := logdb.Search(ctx, s.ro, sc, l.entity, req)
		var se *logdb.SearchError
		if errors.As(err, &se) && (len(req.Conds) > 0 || len(req.And) > 0) {
			req.Conds, req.And, st.Conds, st.Q = nil, nil, []listCond{}, ""
			continue
		}
		if errors.Is(err, logdb.ErrNotFound) {
			return nil, errPageNotFound
		}
		if errors.Is(err, logdb.ErrSearchTimeout) {
			// The page shows what it can: no rows, the search
			// itself left for the viewer to narrow.
			st.Pages = 1
			return &listRun{state: st, res: &logdb.SearchResult{Entity: l.entity}}, nil
		}
		if err != nil {
			return nil, err
		}
		st.Total = res.Total
		st.Pages = max(1, int((res.Total+int64(st.Per)-1)/int64(st.Per)))
		maxPage := logdb.MaxOffset/st.Per + 1
		st.Pages = min(st.Pages, maxPage)
		if st.Page > st.Pages && attempt == 0 && res.TotalExact {
			st.Page = st.Pages
			continue
		}
		run := &listRun{state: st, res: res}
		if n, ok, err := logdb.ArchiveTotal(ctx, s.ro, sc, l.entity); err != nil {
			return nil, err
		} else if ok {
			run.state.ArchiveTotal = &n
		}
		return run, nil
	}
}

// dbTime is a DATETIME as the pages take it, RFC 3339 in UTC (the
// logger's database keeps UTC).
func dbTime(s string) string {
	t, err := time.Parse(time.DateTime, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}

func dbTimePtr(r logdb.Row, c string) *string {
	if r.Null(c) {
		return nil
	}
	t := dbTime(r.Str(c))
	return &t
}

func unixTime(n int64) string {
	return time.Unix(n, 0).UTC().Format(time.RFC3339)
}

func (s *Server) photoPtr(id uint64) *string {
	if id == 0 {
		return nil
	}
	u := s.photoURL(id)
	return &u
}

func ptrTo[T any](v T) *T { return &v }

// listUser is a user as lists show one (UserSummary). Only id,
// name, username, type and photo are there for a viewer who sees users
// in part.
type listUser struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Username    string  `json:"username"`
	Type        string  `json:"type"`
	Photo       *string `json:"photo"`
	ContactName *string `json:"contact_name,omitempty"`
	MsgCount    *int64  `json:"msg_count,omitempty"`
	IsVerified  *bool   `json:"is_verified,omitempty"`
	IsPremium   *bool   `json:"is_premium,omitempty"`
	IsScam      *bool   `json:"is_scam,omitempty"`
	IsFake      *bool   `json:"is_fake,omitempty"`
	IsSupport   *bool   `json:"is_support,omitempty"`
	Language    *string `json:"language,omitempty"`
	CreatedAt   *string `json:"created_at,omitempty"`
	UpdatedAt   *string `json:"updated_at,omitempty"`
}

// usersProps are the users page's (UsersProps).
type usersProps struct {
	listState
	Rows []listUser `json:"rows"`
}

func loadUsers(ctx context.Context, pr *pageRequest) (any, error) {
	s := pr.s
	run, err := usersList.run(ctx, pr)
	if err != nil {
		return nil, err
	}
	full := logdb.Users.Full(pr.scope)
	p := usersProps{listState: run.state, Rows: []listUser{}}
	for _, r := range run.res.Rows {
		u := listUser{ID: r.Int("id"),
			Name:     logdb.DisplayName(r.Str("first_name"), r.Str("last_name")),
			Username: r.Str("username"), Type: r.Str("type"),
			Photo: s.photoPtr(r.Uint("profile_photo_file_id"))}
		if full {
			u.ContactName = ptrTo(logdb.ContactName(r))
			u.MsgCount = ptrTo(r.Int("msg_count"))
			u.IsVerified, u.IsPremium = ptrTo(r.Bool("is_verified")), ptrTo(r.Bool("is_premium"))
			u.IsScam, u.IsFake = ptrTo(r.Bool("is_scam")), ptrTo(r.Bool("is_fake"))
			u.IsSupport = ptrTo(r.Bool("is_support"))
			u.Language = ptrTo(r.Str("language"))
			u.CreatedAt, u.UpdatedAt = dbTimePtr(r, "created_at"), dbTimePtr(r, "updated_at")
		}
		p.Rows = append(p.Rows, u)
	}
	return p, nil
}

// listGroup is a row of the groups page.
type listGroup struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Username    string  `json:"username"`
	Type        string  `json:"type"`
	Photo       *string `json:"photo"`
	MsgCount    int64   `json:"msg_count"`
	Exposed     bool    `json:"exposed"`
	AdminCount  int64   `json:"admin_count"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
}

// groupsProps are the groups page's (GroupsProps).
type groupsProps struct {
	listState
	Rows []listGroup `json:"rows"`
}

func loadGroups(ctx context.Context, pr *pageRequest) (any, error) {
	s := pr.s
	run, err := groupsList.run(ctx, pr)
	if err != nil {
		return nil, err
	}
	p := groupsProps{listState: run.state, Rows: []listGroup{}}
	for _, r := range run.res.Rows {
		p.Rows = append(p.Rows, listGroup{ID: r.Int("id"),
			Title: logdb.GroupTitle(r.Str("title")), Username: r.Str("username"),
			Type: r.Str("type"), Photo: s.photoPtr(r.Uint("photo_file_id")),
			MsgCount: r.Int("msg_count"), Exposed: r.Bool("exposed"),
			AdminCount: r.Int("admin_count"), Description: r.Str("description"),
			CreatedAt: dbTime(r.Str("created_at"))})
	}
	return p, nil
}

type listChatName struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

type listIDName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type listMsgRef struct {
	Scope string `json:"scope"`
	ID    uint64 `json:"id"`
}

// listFile is a row of the files page (FileRow).
type listFile struct {
	ID        uint64        `json:"id"`
	FileType  string        `json:"file_type"`
	Name      string        `json:"name"`
	Ext       string        `json:"ext"`
	Size      uint64        `json:"size"`
	Hits      int64         `json:"hits"`
	Stored    bool          `json:"stored"`
	CreatedAt string        `json:"created_at"`
	TgFileID  string        `json:"tg_file_id"`
	URL       *string       `json:"url"`
	Thumb     *string       `json:"thumb"`
	Chat      *listChatName `json:"chat"`
	Sender    *listIDName   `json:"sender"`
	Message   *listMsgRef   `json:"message"`
}

// filesProps are the files page's (FilesProps).
type filesProps struct {
	listState
	Rows        []listFile `json:"rows"`
	Preview     *listFile  `json:"preview"`
	CanFetch    bool       `json:"can_fetch"`
	StoredBytes *int64     `json:"stored_bytes,omitempty"`
}

// thumbTypes are the file types whose bytes are an image the pages
// can show as a thumbnail.
var thumbTypes = map[string]bool{"photo": true, "sticker": true}

func (s *Server) listFileRows(ctx context.Context, sc scope.Scope, rows []logdb.Row) ([]listFile, error) {
	ids := make([]uint64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Uint("id"))
	}
	origins, err := logdb.FileOrigins(ctx, s.ro, sc, ids)
	if err != nil {
		return nil, err
	}
	out := make([]listFile, 0, len(rows))
	for _, r := range rows {
		f := listFile{ID: r.Uint("id"), FileType: r.Str("file_type"),
			Name: r.Str("orig_file_name"), Ext: r.Str("file_ext"),
			Size: r.Uint("file_size"), Hits: r.Int("hit_count"), Stored: r.Bool("on_disk"),
			CreatedAt: dbTime(r.Str("created_at")), TgFileID: r.Str("tg_file_id"),
			// A file sent in no message the viewer sees (a profile
			// or group photo) has no chat and no message: null.
		}
		if f.Name == "" {
			f.Name = strconv.FormatUint(f.ID, 10)
			if f.Ext != "" {
				f.Name += "." + f.Ext
			}
		}
		if f.Stored {
			f.URL = s.photoPtr(f.ID)
			if thumbTypes[f.FileType] {
				f.Thumb = f.URL
			}
		}
		if o, ok := origins[f.ID]; ok {
			f.Chat = &listChatName{ID: o.ChatID, Title: o.ChatTitle, Kind: o.ChatKind}
			f.Message = &listMsgRef{Scope: o.Scope, ID: o.MessageID}
			if o.SenderID != 0 {
				f.Sender = &listIDName{ID: o.SenderID, Name: o.Sender}
			}
		}
		out = append(out, f)
	}
	return out, nil
}

func loadFiles(ctx context.Context, pr *pageRequest) (any, error) {
	s := pr.s
	run, err := filesList.run(ctx, pr)
	if err != nil {
		return nil, err
	}
	sc := pr.scope
	p := filesProps{listState: run.state, CanFetch: sc.Can(scope.FetchFiles)}
	if p.Rows, err = s.listFileRows(ctx, sc, run.res.Rows); err != nil {
		return nil, err
	}
	if want := pr.query.Get("file"); isInt(want) {
		res, err := logdb.Search(ctx, s.ro, sc, logdb.Files, logdb.SearchRequest{
			Conds: []logdb.Cond{cond("id", "=", want)}, Internal: true, Limit: 1})
		if err != nil {
			return nil, err
		}
		if rows, err := s.listFileRows(ctx, sc, res.Rows); err != nil {
			return nil, err
		} else if len(rows) == 1 {
			p.Preview = &rows[0]
		}
	}
	if n, ok := logdb.StoredBytes(ctx, s.ro, sc); ok {
		p.StoredBytes = &n
	}
	return p, nil
}

// listChat is a message's chat (ChatRef).
type listChat struct {
	ID    int64   `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Photo *string `json:"photo"`
}

// listMessage is a row of the message lists (MessageRow).
type listMessage struct {
	ID          uint64    `json:"id"`
	Scope       string    `json:"scope"`
	MessageID   int64     `json:"message_id"`
	Date        string    `json:"date"`
	Chat        listChat  `json:"chat"`
	Sender      *listUser `json:"sender"`
	ContentType string    `json:"content_type"`
	Snippet     string    `json:"snippet"`
	Forwarded   *string   `json:"forwarded"`
	Edited      bool      `json:"edited"`
	DeletedAt   *string   `json:"deleted_at"`
}

// messagesProps are the message lists' (MessagesProps).
type messagesProps struct {
	listState
	Rows  []listMessage `json:"rows"`
	Scope string        `json:"scope"`
}

// snippetRunes is how much of a message's text a list shows.
const snippetRunes = 300

func listSnippet(text, contentType string) string {
	if text == "" {
		return "[" + contentType + "]"
	}
	if utf8.RuneCountInString(text) <= snippetRunes {
		return text
	}
	n := 0
	for i := range text {
		if n == snippetRunes {
			return text[:i] + "..."
		}
		n++
	}
	return text
}

func loadMessages(ctx context.Context, pr *pageRequest, l *listSpec, msgScope string) (any, error) {
	s := pr.s
	run, err := l.run(ctx, pr)
	if err != nil {
		return nil, err
	}
	sc := pr.scope
	p := messagesProps{listState: run.state, Rows: []listMessage{}, Scope: msgScope}
	var fwd []uint64
	for _, r := range run.res.Rows {
		m := listMessage{ID: r.Uint("id"), Scope: msgScope, MessageID: r.Int("message_id"),
			Date: unixTime(r.Int("date_unix")), ContentType: r.Str("content_type"),
			Snippet: listSnippet(r.Str("text"), r.Str("content_type")),
			Edited:  r.Int("edit_date") != 0, DeletedAt: dbTimePtr(r, "deleted_at")}
		if msgScope == "group" {
			m.Chat = listChat{ID: r.Int("chat_id"), Title: logdb.GroupTitle(r.Str("group_title")),
				Kind: "group", Photo: s.photoPtr(r.Uint("group_photo"))}
			switch {
			case !r.Null("sender_user_id"):
				m.Sender = &listUser{ID: r.Int("sender_user_id"),
					Name:     logdb.DisplayName(r.Str("su_first"), r.Str("su_last")),
					Username: r.Str("su_username"), Type: r.Str("su_type"),
					Photo: s.photoPtr(r.Uint("su_photo"))}
				if m.Sender.Type == "" {
					m.Sender.Type = "unknown"
				}
			case !r.Null("sender_chat_id"):
				// Posted as a chat: named only when the viewer
				// may see that chat.
				m.Sender = &listUser{ID: r.Int("sender_chat_id"), Name: "(hidden)",
					Type: "unknown"}
				if sc.SeesGroup(r.Int("sender_chat_id")) {
					m.Sender.Name = logdb.GroupTitle(r.Str("sg_title"))
					m.Sender.Username = r.Str("sg_username")
					m.Sender.Photo = s.photoPtr(r.Uint("sg_photo"))
				}
			}
		} else {
			peer := logdb.DisplayName(r.Str("peer_first"), r.Str("peer_last"))
			photo := s.photoPtr(r.Uint("peer_photo"))
			m.Chat = listChat{ID: r.Int("chat_id"), Title: peer, Kind: "user", Photo: photo}
			if !r.Bool("is_outgoing") {
				t := r.Str("peer_type")
				if t == "" {
					t = "unknown"
				}
				m.Sender = &listUser{ID: r.Int("chat_id"), Name: peer,
					Username: r.Str("peer_username"), Type: t, Photo: photo}
			}
		}
		if r.Bool("is_forwarded") {
			fwd = append(fwd, m.ID)
		}
		p.Rows = append(p.Rows, m)
	}
	origins, err := logdb.ForwardOrigins(ctx, s.ro, sc, msgScope, fwd)
	if err != nil {
		return nil, err
	}
	for i := range p.Rows {
		if name, ok := origins[p.Rows[i].ID]; ok {
			p.Rows[i].Forwarded = &name
		} else if r := run.res.Rows[i]; r.Bool("is_forwarded") {
			p.Rows[i].Forwarded = ptrTo("unknown")
		}
	}
	return p, nil
}

func loadGroupMessages(ctx context.Context, pr *pageRequest) (any, error) {
	return loadMessages(ctx, pr, groupMessagesList, "group")
}

func loadPrivateMessages(ctx context.Context, pr *pageRequest) (any, error) {
	return loadMessages(ctx, pr, privateMessagesList, "private")
}

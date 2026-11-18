// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	"tgloggerd/webapi/internal/scope"
)

// The advanced search (web/docs/search-api.md), ported from the Drogon
// web's dao::search (web/src/dao/Search.cpp): a flat chain of
// conditions [{c,o,v,n}] over a per-entity field registry. Only the
// bound values come from the request; every other SQL token (column
// expressions, operators, connectors, table names, the sort column and
// its direction) comes from the registries (search_entities.go), so
// the SQL is safe by construction.
//
// Every search takes the request scope. An entity the scope does not
// show (private messages without SeesPrivate) is ErrNotFound, and the
// rows of the others are limited in the SQL itself, by the entity's
// scope condition (GroupFilter and the like) ANDed with the request's
// conditions, so the rows, the totals and the estimates all count only
// visible rows. A scope that limits an entity's rows never gets the
// storage engine's row estimate of the whole table as its total.

// Op is a search operator; a field lists the ones it takes as a mask.
type Op uint32

// The operators.
const (
	OpEQ Op = 1 << iota
	OpNE
	OpLT
	OpGT
	OpLE
	OpGE
	OpLike      // SQL LIKE, the value bound verbatim (% is opt-in)
	OpNLike     // NOT LIKE
	OpIsNull    // IS NULL, no value
	OpIsNotNull // IS NOT NULL, no value
	OpCLike     // "%LIKE%": contains, the value wrapped in %..%
	OpNCLike    // "NOT %LIKE%"
	OpMatch     // "matches": MATCH..AGAINST in boolean mode
)

// opTable is the operators as clients send them, in the order the
// fields list them, with the SQL token of a column comparison.
var opTable = []struct {
	op       Op
	tok, sql string
}{
	{OpEQ, "=", "="},
	{OpNE, "!=", "!="},
	{OpLT, "<", "<"},
	{OpGT, ">", ">"},
	{OpLE, "<=", "<="},
	{OpGE, ">=", ">="},
	{OpLike, "LIKE", "LIKE"},
	{OpNLike, "NOT LIKE", "NOT LIKE"},
	{OpCLike, "%LIKE%", "LIKE"},
	{OpNCLike, "NOT %LIKE%", "NOT LIKE"},
	{OpMatch, "matches", ""},
	{OpIsNull, "IS NULL", ""},
	{OpIsNotNull, "IS NOT NULL", ""},
}

// FieldType is the type of a field's values.
type FieldType int

// The field types. FullText fields take only OpMatch and need a
// FULLTEXT index.
const (
	Text FieldType = iota
	Int
	Bool
	Datetime
	Enum
	FullText
)

func (t FieldType) String() string {
	switch t {
	case Int:
		return "int"
	case Bool:
		return "bool"
	case Datetime:
		return "datetime"
	case Enum:
		return "enum"
	}
	return "text" // FullText: the UI shows a plain text input
}

// Field is one searchable field of an entity.
//
// A column field's condition is `<Expr> <op> ?` (or IS [NOT] NULL). An
// EXISTS field's is `[NOT] EXISTS (SELECT 1 FROM <ExTable> WHERE
// x.<exFK> = <idCol> <ExExtra> AND <ExCol> {=|LIKE} ?)`: =, LIKE and
// %LIKE% mean "ever matched", !=, NOT LIKE and NOT %LIKE% "never
// matched", and IS [NOT] NULL is never allowed.
type Field struct {
	Key, Label string
	Type       FieldType
	Exists     bool
	Expr       string // column: the left-hand side
	ExTable    string // EXISTS: "table x" (the alias must be x)
	ExCol      string // EXISTS: the x.<col> compared
	ExExtra    string // EXISTS: an extra predicate, or ""
	Ops        Op
	Sortable   bool
	Enum       string // Enum: the allowed values, comma-separated
	// Bind is the right-hand side of a column comparison, with
	// exactly one ?; "" is a plain "?". sha256 binds UNHEX(?) so the
	// unique index is used.
	Bind string
	// internal fields are for the page loaders only: the API neither
	// lists nor accepts them.
	internal bool
}

// Col is a displayed column: its key, label, render type and the field
// key a click on its header sorts by ("" = not sortable).
type Col struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Sort  string `json:"sort"`
}

// FieldInfo is a field as the API lists it.
type FieldInfo struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Sortable  bool     `json:"sortable"`
	Operators []string `json:"operators"`
	Enum      *string  `json:"enum,omitempty"`
}

// The limits of web/docs/search-api.md.
const (
	MaxConds       = 16
	MaxExists      = 4
	MaxValueLen    = 512
	MaxLimit       = 1000
	MaxOffset      = 500000
	MaxSearchBytes = 8192
	// statementMS is every search statement's time limit
	// (MAX_EXECUTION_TIME), as in Drogon.
	statementMS = 3000
)

// Cond is one condition of a search: field key, operator, value and
// the connector to the next condition.
type Cond struct {
	C, O, V, N string
	HasV       bool // whether v was given (IS NULL takes none)
}

// SearchRequest is one search.
type SearchRequest struct {
	// Conds is the request's chain of conditions.
	Conds []Cond
	// And are more chains, each put in parentheses and ANDed to
	// Conds: the page loaders' free-text search beside their filters.
	And [][]Cond
	// Sort is a sortable field key; anything else is the default.
	Sort  string
	Order string // "asc" or "desc"; anything else is the default
	// Limit and Offset are clamped to 1..MaxLimit and 0..MaxOffset.
	Limit, Offset int
	// Debug adds the SQL, its values and its plan; the caller decides
	// who may ask.
	Debug bool
	// Internal allows the fields only the page loaders use, and adds
	// the entity's page columns to the query.
	Internal bool
}

// SearchError is a request the search refuses (HTTP 400).
type SearchError struct{ Msg string }

func (e *SearchError) Error() string { return e.Msg }

func refuse(format string, a ...any) error {
	return &SearchError{Msg: fmt.Sprintf(format, a...)}
}

// ErrSearchTimeout is a page query that ran out of its time limit.
var ErrSearchTimeout = errors.New("The search took too long and was stopped. " +
	"Narrow it down and try again.")

// Row is one result row: each column's value as the database sent it,
// NULL as not Valid.
type Row map[string]sql.NullString

// Str is the column's value, "" for NULL.
func (r Row) Str(c string) string { return r[c].String }

// Null is whether the column is NULL.
func (r Row) Null(c string) bool { return !r[c].Valid }

// Int is the column's value as an integer, 0 for NULL.
func (r Row) Int(c string) int64 {
	n, _ := strconv.ParseInt(r[c].String, 10, 64)
	return n
}

// Uint is the column's value as an unsigned integer, 0 for NULL.
func (r Row) Uint(c string) uint64 {
	n, _ := strconv.ParseUint(r[c].String, 10, 64)
	return n
}

// Bool is whether the column holds a non-zero number.
func (r Row) Bool(c string) bool { return r.Int(c) != 0 }

// SearchDebug is the debug part of an answer: the SQL, its values and
// its plan, every string HTML-escaped as in Drogon.
type SearchDebug struct {
	SQL      string   `json:"sql"`
	CountSQL string   `json:"count_sql"`
	Bind     []string `json:"bind"`
	Explain  struct {
		Columns []string   `json:"columns"`
		Rows    [][]string `json:"rows"`
	} `json:"explain"`
}

// SearchResult is the answer of a search.
type SearchResult struct {
	Entity     *Entity
	Scope      scope.Scope
	Fields     []FieldInfo
	Cols       []Col
	Rows       []Row
	Total      int64
	TotalExact bool
	Limit      int
	Offset     int
	Sort       string // the field key sorted by, "" for the default
	Order      string // "asc" or "desc"
	Debug      *SearchDebug
}

// esc is the Drogon web's Render::esc: the API's string cells are
// HTML-escaped, so its pages insert them verbatim.
func esc(s string) string { return escaper.Replace(s) }

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;",
	`"`, "&quot;", "'", "&#39;")

// likeContains wraps q in %..% for a "contains" match, escaping the
// LIKE metacharacters of q itself so they match literally.
func likeContains(q string) string {
	var b strings.Builder
	b.WriteByte('%')
	for i := 0; i < len(q); i++ {
		if c := q[i]; c == '\\' || c == '%' || c == '_' {
			b.WriteByte('\\')
		}
		b.WriteByte(q[i])
	}
	b.WriteByte('%')
	return b.String()
}

func isIntStr(v string) bool {
	i := 0
	if strings.HasPrefix(v, "-") {
		i = 1
	}
	if i >= len(v) {
		return false
	}
	for ; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
	}
	return true
}

func isDatetimeStr(v string) bool {
	if v == "" {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !(c >= '0' && c <= '9' || strings.IndexByte("-: .T/", c) >= 0) {
			return false
		}
	}
	return true
}

func csvHas(csv, v string) bool {
	for _, t := range strings.Split(csv, ",") {
		if t == v {
			return true
		}
	}
	return false
}

func validValue(f *Field, v string) error {
	if len(v) > MaxValueLen {
		return refuse("value too long (max %d)", MaxValueLen)
	}
	switch f.Type {
	case Int, Bool:
		if !isIntStr(v) {
			return refuse("field '%s' expects a numeric value", f.Key)
		}
	case Datetime:
		if !isDatetimeStr(v) {
			return refuse("field '%s' expects a date/time value", f.Key)
		}
	case Enum:
		if !csvHas(f.Enum, v) {
			return refuse("field '%s' expects one of: %s", f.Key, f.Enum)
		}
	}
	return nil
}

// ParseConditions reads the search parameter, a JSON array of
// conditions, as Drogon's parseConditions does; "" is no condition.
// Field and operator names are checked by Search, against the entity.
func ParseConditions(raw string) ([]Cond, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > MaxSearchBytes {
		return nil, refuse("search parameter too large")
	}
	items, err := parseJSONArray(raw)
	if err != nil {
		return nil, err
	}
	out := make([]Cond, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			return nil, refuse("each condition must be a JSON object")
		}
		var c Cond
		for _, k := range []struct {
			name     string
			dst      *string
			required bool
		}{{"c", &c.C, true}, {"o", &c.O, true}, {"n", &c.N, false}} {
			v, present := obj[k.name]
			if !present || v == nil {
				if k.required {
					return nil, refuse("missing '%s' in a condition", k.name)
				}
				continue
			}
			s, ok := v.(string)
			if !ok {
				return nil, refuse("'%s' must be a string", k.name)
			}
			*k.dst = s
		}
		if v, present := obj["v"]; present && v != nil {
			s, ok := v.(string)
			if !ok {
				return nil, refuse("'v' must be a string")
			}
			c.V, c.HasV = s, true
		}
		out = append(out, c)
	}
	return out, nil
}

func parseJSONArray(raw string) ([]any, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, refuse("search must be valid JSON")
	}
	a, ok := v.([]any)
	if !ok {
		return nil, refuse("search must be a JSON array")
	}
	return a, nil
}

// built is a search's SQL and values.
type built struct {
	countSQL, pageSQL string
	where             []any // the WHERE's values, the count's values
	orderBind         any   // the relevance sort's value, or nil
	sort, order       string
	limit, offset     int
	filtered          bool // a WHERE at all, scope included
}

func (e *Entity) field(key string, internal bool) *Field {
	for i := range e.fields {
		f := &e.fields[i]
		if f.Key == key && (internal || !f.internal) {
			return f
		}
	}
	return nil
}

func opLookup(tok string) (Op, string, bool) {
	for _, o := range opTable {
		if o.tok == tok {
			return o.op, o.sql, true
		}
	}
	return 0, "", false
}

// chain builds one chain of conditions: the SQL and its values, and
// the first fulltext match, for the relevance sort.
func (e *Entity) chain(sc scope.Scope, conds []Cond, internal bool, exists *int,
	ftExpr, ftVal *string) (string, []any, error) {
	var (
		where strings.Builder
		binds []any
	)
	for i, c := range conds {
		f := e.field(c.C, internal)
		if f == nil || !e.fieldVisible(sc, f) {
			return "", nil, refuse("unknown field: '%s'", c.C)
		}
		op, sqlTok, ok := opLookup(c.O)
		if !ok || f.Ops&op == 0 {
			return "", nil, refuse("operator '%s' not allowed for field '%s'", c.O, c.C)
		}
		if i > 0 {
			conn := conds[i-1].N
			if conn != "AND" && conn != "OR" {
				return "", nil, refuse("invalid connector: '%s'", conn)
			}
			where.WriteString(" " + conn + " ")
		}
		where.WriteByte('(')
		if op == OpIsNull || op == OpIsNotNull {
			where.WriteString(f.Expr)
			if op == OpIsNull {
				where.WriteString(" IS NULL)")
			} else {
				where.WriteString(" IS NOT NULL)")
			}
			continue
		}
		if !c.HasV {
			return "", nil, refuse("field '%s' requires a value", c.C)
		}
		if err := validValue(f, c.V); err != nil {
			return "", nil, err
		}
		likeish := op == OpLike || op == OpNLike || op == OpCLike || op == OpNCLike
		val := c.V
		if op == OpCLike || op == OpNCLike {
			val = likeContains(c.V)
		}
		switch {
		case !f.Exists && op == OpMatch:
			where.WriteString("MATCH(" + f.Expr + ") AGAINST(? IN BOOLEAN MODE))")
			if *ftExpr == "" {
				*ftExpr, *ftVal = f.Expr, c.V
			}
		case !f.Exists:
			rhs := f.Bind
			if rhs == "" {
				rhs = "?"
			}
			where.WriteString(f.Expr + " " + sqlTok + " " + rhs + ")")
		default:
			if *exists++; *exists > MaxExists {
				return "", nil, refuse("too many history conditions (max %d)", MaxExists)
			}
			cmp := "="
			if likeish {
				cmp = "LIKE"
			}
			if op == OpNE || op == OpNLike || op == OpNCLike {
				where.WriteString("NOT ")
			}
			where.WriteString("EXISTS (SELECT 1 FROM " + f.ExTable + " WHERE x." +
				e.exFK + " = " + e.idCol + " " + f.ExExtra + " AND " + f.ExCol +
				" " + cmp + " ?))")
		}
		binds = append(binds, val)
	}
	return where.String(), binds, nil
}

func (e *Entity) build(sc scope.Scope, req SearchRequest) (*built, error) {
	n := len(req.Conds)
	for _, ch := range req.And {
		n += len(ch)
	}
	if n > MaxConds {
		return nil, refuse("too many conditions (max %d)", MaxConds)
	}

	var (
		parts          []string
		binds          []any
		exists         int
		ftExpr, ftVal  string
		chains         = append([][]Cond{req.Conds}, req.And...)
		scopeCond, sca = e.filter(sc)
	)
	if scopeCond != "" {
		parts = append(parts, scopeCond)
		binds = append(binds, sca...)
	}
	for _, ch := range chains {
		if len(ch) == 0 {
			continue
		}
		w, b, err := e.chain(sc, ch, req.Internal, &exists, &ftExpr, &ftVal)
		if err != nil {
			return nil, err
		}
		parts = append(parts, w)
		binds = append(binds, b...)
	}
	where := ""
	if len(parts) == 1 {
		// One part is emitted as Drogon emits it, unparenthesised.
		where = parts[0]
	} else if len(parts) > 1 {
		where = "(" + strings.Join(parts, ") AND (") + ")"
	}

	b := &built{where: binds, filtered: where != ""}
	sortExpr := e.defSort
	if req.Sort != "" {
		if f := e.field(req.Sort, req.Internal); f != nil && f.Sortable &&
			e.fieldVisible(sc, f) {
			sortExpr, b.sort = f.Expr, f.Key
		}
	}
	switch strings.ToLower(req.Order) {
	case "asc":
		b.order = "ASC"
	case "desc":
		b.order = "DESC"
	default:
		b.order = e.defOrder
	}
	// No explicit sort and a fulltext condition: order by relevance,
	// which the fulltext index yields without sorting every hit.
	if req.Sort == "" && ftExpr != "" {
		sortExpr, b.order, b.orderBind = "MATCH("+ftExpr+") AGAINST(? IN BOOLEAN MODE)", "DESC", ftVal
	}
	b.limit = min(max(req.Limit, 1), MaxLimit)
	b.offset = min(max(req.Offset, 0), MaxOffset)

	// Stable paging: ties broken on the unique id, except for a
	// relevance sort, whose index scan a second key would defeat.
	tiebreak := ""
	if sortExpr != e.idCol && b.orderBind == nil {
		tiebreak = ", " + e.idCol + " DESC"
	}
	whereClause := ""
	if where != "" {
		whereClause = " WHERE " + where
	}
	countFrom := e.countFromFor(where)
	hint := "SELECT /*+ MAX_EXECUTION_TIME(" + strconv.Itoa(statementMS) + ") */ "
	b.countSQL = hint + "COUNT(*) AS n FROM " + countFrom + whereClause
	sel := e.sel
	if req.Internal && e.pageSel != "" {
		sel += ", " + e.pageSel
	}
	b.pageSQL = hint + sel + " FROM " + e.from + whereClause + " ORDER BY " + sortExpr +
		" " + b.order + tiebreak + " LIMIT " + strconv.Itoa(b.limit) +
		" OFFSET " + strconv.Itoa(b.offset)
	if e == Users {
		if t, _ := userTotal(sc); t != "u.msg_count" && b.orderBind == nil {
			// The scope's totals have no index to sort by, so the rows
			// are sorted by them in full: find the page's ids with a
			// narrow query first, and only then read their columns
			// (whose subqueries would otherwise run for every user).
			b.pageSQL = hint + sel + " FROM " + e.from + " JOIN (SELECT u.id AS page_id FROM " +
				e.from + whereClause + " ORDER BY " + sortExpr + " " + b.order + tiebreak +
				" LIMIT " + strconv.Itoa(b.limit) + " OFFSET " + strconv.Itoa(b.offset) +
				") page ON page.page_id = u.id ORDER BY " + sortExpr + " " + b.order + tiebreak
		}
		// A user's total, its sort and its filters as the scope may see
		// them (userTotal).
		b.pageSQL, b.countSQL = scopedTotals(sc, b.pageSQL), scopedTotals(sc, b.countSQL)
	}

	want := len(binds)
	if b.orderBind != nil {
		want++
	}
	if strings.Count(b.pageSQL, "?") != want {
		return nil, errors.New("search: placeholders and values differ")
	}
	return b, nil
}

func (b *built) pageArgs() []any {
	if b.orderBind == nil {
		return b.where
	}
	return append(append([]any{}, b.where...), b.orderBind)
}

// scanRows reads every row of rs into Rows.
func scanRows(rs *sql.Rows) ([]Row, error) {
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	var out []Row
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := make(Row, len(cols))
		for i, c := range cols {
			r[c] = vals[i]
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// isTimeout is a statement stopped by its MAX_EXECUTION_TIME.
func isTimeout(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 3024
}

// Search runs a search of entity within the request scope.
func Search(ctx context.Context, db *sql.DB, sc scope.Scope, e *Entity, req SearchRequest) (*SearchResult, error) {
	if !e.visible(sc) {
		return nil, ErrNotFound
	}
	b, err := e.build(sc, req)
	if err != nil {
		return nil, err
	}
	res := &SearchResult{
		Entity: e, Scope: sc, Fields: e.fieldInfos(sc), Cols: e.columns(sc),
		Limit: b.limit, Offset: b.offset, Sort: b.sort, Order: strings.ToLower(b.order),
	}

	// The count only feeds the pager, so it never fails the request:
	// when it fails or runs out of time, the total is estimated below.
	if b.filtered || !e.estimateBrowse {
		if n, err := countInt(ctx, db, b.countSQL, b.where...); err == nil {
			res.Total, res.TotalExact = n, true
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	rs, err := db.QueryContext(ctx, b.pageSQL, b.pageArgs()...)
	if err == nil {
		res.Rows, err = scanRows(rs)
	}
	if err != nil {
		if isTimeout(err) {
			return nil, ErrSearchTimeout
		}
		return nil, err
	}

	if !res.TotalExact {
		seen := int64(b.offset + len(res.Rows))
		res.Total = 0
		if !b.filtered {
			// Every row matches: the engine's row estimate of the
			// table is the answer, give or take. Only a scope that
			// sees the whole table gets here (b.filtered).
			var n sql.NullInt64
			if err := db.QueryRowContext(ctx, "SELECT table_rows AS n "+
				"FROM information_schema.tables WHERE table_schema = DATABASE() "+
				"AND table_name = ?", e.countTable()).Scan(&n); err == nil && n.Valid {
				res.Total = n.Int64
			}
		}
		// What the page proves exists: its rows, plus one when full.
		floor := seen
		if len(res.Rows) == b.limit {
			floor++
		}
		res.Total = max(res.Total, floor)
	}

	if req.Debug {
		if res.Debug, err = explain(ctx, db, b); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func explain(ctx context.Context, db *sql.DB, b *built) (*SearchDebug, error) {
	d := &SearchDebug{SQL: esc(b.pageSQL), CountSQL: esc(b.countSQL), Bind: []string{}}
	for _, v := range b.pageArgs() {
		d.Bind = append(d.Bind, esc(fmt.Sprint(v)))
	}
	rs, err := db.QueryContext(ctx, "EXPLAIN FORMAT=TRADITIONAL "+b.pageSQL, b.pageArgs()...)
	if err != nil {
		return nil, err
	}
	cols, err := rs.Columns()
	if err != nil {
		rs.Close()
		return nil, err
	}
	// The driver reads FLOAT columns (filtered) as numbers; they are
	// shown with the column's decimals, as MySQL sends them as text.
	scales := make([]int, len(cols))
	for i := range scales {
		scales[i] = -1
	}
	if types, err := rs.ColumnTypes(); err == nil {
		for i, ct := range types {
			_, scale, ok := ct.DecimalSize()
			switch ct.DatabaseTypeName() {
			case "FLOAT", "DOUBLE":
				if ok && scale < 31 {
					scales[i] = int(scale)
				}
			}
		}
	}
	rows, err := scanRows(rs)
	if err != nil {
		return nil, err
	}
	d.Explain.Columns = []string{}
	for _, c := range cols {
		d.Explain.Columns = append(d.Explain.Columns, esc(c))
	}
	d.Explain.Rows = [][]string{}
	for _, r := range rows {
		row := make([]string, len(cols))
		for i, c := range cols {
			v := r.Str(c)
			if f, err := strconv.ParseFloat(v, 64); err == nil && scales[i] >= 0 {
				v = strconv.FormatFloat(f, 'f', scales[i], 64)
			}
			row[i] = esc(v)
		}
		d.Explain.Rows = append(d.Explain.Rows, row)
	}
	return d, nil
}

// countFromFor is the FROM of the entity's count under where: the
// join-free one when where does not name the joined table.
func (e *Entity) countFromFor(where string) string {
	if e.countBare != "" && !e.joinAlias.MatchString(where) {
		return e.countBare
	}
	if e.countFrom != "" {
		return e.countFrom
	}
	return e.from
}

// countTable is the table an entity counts: the first word of its
// count FROM, without backticks.
func (e *Entity) countTable() string {
	from := e.countFrom
	if from == "" {
		from = e.from
	}
	name, _, _ := strings.Cut(from, " ")
	return strings.Trim(name, "`")
}

func (e *Entity) fieldInfos(sc scope.Scope) []FieldInfo {
	out := []FieldInfo{}
	for i := range e.fields {
		f := &e.fields[i]
		if f.internal || !e.fieldVisible(sc, f) {
			continue
		}
		fi := FieldInfo{Key: f.Key, Label: f.Label, Type: f.Type.String(),
			Sortable: f.Sortable, Operators: []string{}}
		for _, o := range opTable {
			if f.Ops&o.op != 0 {
				fi.Operators = append(fi.Operators, o.tok)
			}
		}
		if f.Type == Enum {
			v := f.Enum
			fi.Enum = &v
		}
		out = append(out, fi)
	}
	return out
}

func (e *Entity) columns(sc scope.Scope) []Col {
	out := []Col{}
	for _, c := range e.cols {
		if e.colVisible(sc, c.Key) {
			out = append(out, c)
		}
	}
	return out
}

// PhotoURL turns a file id into the /files/<token> URL of a photo cell.
type PhotoURL func(id uint64) string

// photoID is a photo cell before Positional turns it into its URL.
type photoID uint64

// Positional is the rows as the API sends them: an array per row,
// aligned to Cols (api2.php style), its strings HTML-escaped and its
// photo cells /files/<token> URLs ("" for none).
func (r *SearchResult) Positional(photo PhotoURL) [][]any {
	url := func(v any) any {
		if id, ok := v.(photoID); ok {
			if id == 0 {
				return ""
			}
			return photo(uint64(id))
		}
		return v
	}
	out := make([][]any, 0, len(r.Rows))
	for _, row := range r.Rows {
		cells := r.Entity.cells(r.Scope, row)
		a := make([]any, len(r.Cols))
		for i, c := range r.Cols {
			v := cells[c.Key]
			if p, ok := v.(party); ok {
				p.Photo = url(p.Photo)
				v = p
			}
			a[i] = url(v)
		}
		out = append(out, a)
	}
	return out
}

// party is a message row's linked avatar and name: kind user, group,
// self or none; photo a photo cell.
type party struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Photo    any    `json:"photo"`
	Username string `json:"username"`
}

// FieldInfos are the fields the API lists for the scope: what a list
// page offers in its condition builder.
func (e *Entity) FieldInfos(sc scope.Scope) []FieldInfo { return e.fieldInfos(sc) }

// Sortable is whether the scope may sort the entity by the field key
// through the API (not the page loaders' own fields).
func (e *Entity) Sortable(sc scope.Scope, key string) bool {
	f := e.field(key, false)
	return f != nil && f.Sortable && e.fieldVisible(sc, f)
}

// IsOperator is whether op is one of the search's operators.
func IsOperator(op string) bool {
	_, _, ok := opLookup(op)
	return ok
}

// TakesValue is whether the operator compares with a value (all but
// IS NULL and IS NOT NULL).
func TakesValue(op string) bool { return op != "IS NULL" && op != "IS NOT NULL" }

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"strconv"
	"strings"
	"time"
)

// The filter tree of telegram_search_messages and telegram_get_users,
// compiled to parameterised SQL; the port of web/src/mcp/Filter.cpp,
// with its messages. A node is a condition {"field","op","value"} or a
// group {"and":[...]}, {"or":[...]}, {"not":{...}}, so (A OR B) AND NOT
// C is expressible, which the flat list of the advanced search is not.
//
// Field and operator are looked up and emitted as constants; only a
// value reaches the SQL, always as a bound parameter.

type ftype int

const (
	fInt      ftype = iota // an integer
	fText                  // LIKE-able text
	fFullText              // MATCH ... AGAINST; only "match" applies
	fEnum                  // one of enumVals
	fDateTs                // ISO-8601 or unix seconds in, unix seconds out
	fBool                  // true or false
	// fPresence is a column that stores 0 for "never happened"
	// (edit_date): is_null / is_not_null and = true / false compile
	// to = 0 / <> 0.
	fPresence
	// fPresenceNull is its NULL twin (deleted_at): = true / false and
	// is_not_null / is_null compile to IS NOT NULL / IS NULL, and no
	// other operator is taken.
	fPresenceNull
)

type field struct {
	key      string // what the caller writes
	expr     string // the SQL left-hand side
	typ      ftype
	enumVals string // CSV, for fEnum
	desc     string // shown in the tool's description
	// valExpr is how a bound value is spelled when "?" alone does not
	// compare with expr: deleted_at is a DATETIME and binds
	// FROM_UNIXTIME(?), so the column stays bare and indexable.
	valExpr string
}

type schema []field

func (s schema) find(key string) *field {
	for i := range s {
		if s[i].key == key {
			return &s[i]
		}
	}
	return nil
}

// compiled is a filter's SQL, "" for no filter, and its binds.
type compiled struct {
	sql   string
	binds []any
	// The first positive full-text condition: ordering a match by
	// relevance needs its expression and term, the term bound again.
	ftExpr, ftValue string
}

// The guards: a filter comes from a model and may be any shape.
const (
	maxDepth    = 8
	maxNodes    = 64
	maxValueLen = 512
	maxInItems  = 100
)

func csvHas(csv, v string) bool {
	for _, e := range strings.Split(csv, ",") {
		if e == v {
			return true
		}
	}
	return false
}

// likeEscape escapes the caller's own %, _ and \.
func likeEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' || c == '_' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// toUnixSeconds reads a date value: an integer unix timestamp (as a
// number or a string of digits), or YYYY-MM-DD with an optional
// THH:MM:SS (or a space instead of the T), optional fractional
// seconds and Z or a numeric offset, which is applied. Anything else is
// an error rather than a silent 0, a legitimate stored value.
func toUnixSeconds(v any) (string, error) {
	if i, ok := intValue(v); ok {
		return strconv.FormatInt(i, 10), nil
	}
	s, ok := v.(string)
	if !ok {
		return "", toolErr(`a date value must be a string like "2026-01-31" or ` +
			`"2026-01-31T12:00:00Z", or an integer unix timestamp`)
	}
	if len(s) > 32 {
		return "", toolErr("date value is too long")
	}
	if allDigits(s) {
		return s, nil
	}

	t, rest, ok := parseDateTime(s)
	if !ok {
		return "", toolErr(`could not parse date "` + s +
			`"; use YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ`)
	}
	p := rest
	if strings.HasPrefix(p, ".") {
		p = p[1:]
		if p == "" || p[0] < '0' || p[0] > '9' {
			return "", toolErr(`malformed fractional seconds in "` + s + `"`)
		}
		for p != "" && p[0] >= '0' && p[0] <= '9' {
			p = p[1:]
		}
	}
	digit := func(i int) bool { return i < len(p) && p[i] >= '0' && p[i] <= '9' }
	switch {
	case strings.HasPrefix(p, "Z") || strings.HasPrefix(p, "z"):
		p = p[1:]
	case strings.HasPrefix(p, "+") || strings.HasPrefix(p, "-"):
		west := p[0] == '-'
		p = p[1:]
		if !digit(0) || !digit(1) {
			return "", toolErr(`malformed UTC offset in "` + s + `"`)
		}
		off := int64((p[0]-'0')*10+(p[1]-'0')) * 3600
		p = p[2:]
		if strings.HasPrefix(p, ":") {
			p = p[1:]
		}
		if digit(0) {
			if !digit(1) {
				return "", toolErr(`malformed UTC offset in "` + s + `"`)
			}
			off += int64((p[0]-'0')*10+(p[1]-'0')) * 60
			p = p[2:]
		}
		// +07:00 is a wall clock ahead of UTC: subtract.
		if west {
			t += off
		} else {
			t -= off
		}
	}
	if p != "" {
		return "", toolErr(`unrecognised trailing "` + p + `" in date "` + s + `"`)
	}
	return strconv.FormatInt(t, 10), nil
}

// parseDateTime reads what the Drogon code's strptime formats
// "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S" and "%Y-%m-%d" read, tried in
// that order: glibc's numbers (blanks before them skipped, up to 4
// digits for the year and 2 for the rest, in range), a blank in the
// format matching any run of blanks, out-of-range days rolled over as
// timegm rolls them. It returns the UTC seconds and what is left.
func parseDateTime(s string) (int64, string, bool) {
	type pfield struct {
		max, lo, hi int
	}
	num := func(p string, f pfield) (int, string, bool) {
		for p != "" && isBlank(p[0]) {
			p = p[1:]
		}
		n, i := 0, 0
		for i < len(p) && i < f.max && p[i] >= '0' && p[i] <= '9' {
			n = n*10 + int(p[i]-'0')
			i++
		}
		if i == 0 || n < f.lo || n > f.hi {
			return 0, p, false
		}
		return n, p[i:], true
	}
	lit := func(p string, c byte) (string, bool) {
		if c == ' ' {
			for p != "" && isBlank(p[0]) {
				p = p[1:]
			}
			return p, true
		}
		if p == "" || p[0] != c {
			return p, false
		}
		return p[1:], true
	}
	date := func(p string) (y, m, d int, rest string, ok bool) {
		if y, p, ok = num(p, pfield{4, 0, 9999}); !ok {
			return
		}
		if p, ok = lit(p, '-'); !ok {
			return
		}
		if m, p, ok = num(p, pfield{2, 1, 12}); !ok {
			return
		}
		if p, ok = lit(p, '-'); !ok {
			return
		}
		d, p, ok = num(p, pfield{2, 1, 31})
		return y, m, d, p, ok
	}
	for _, sep := range []byte{'T', ' '} {
		y, mo, d, p, ok := date(s)
		if !ok {
			break
		}
		if p, ok = lit(p, sep); !ok {
			continue
		}
		var h, mi, sec int
		if h, p, ok = num(p, pfield{2, 0, 23}); !ok {
			continue
		}
		if p, ok = lit(p, ':'); !ok {
			continue
		}
		if mi, p, ok = num(p, pfield{2, 0, 59}); !ok {
			continue
		}
		if p, ok = lit(p, ':'); !ok {
			continue
		}
		if sec, p, ok = num(p, pfield{2, 0, 61}); !ok {
			continue
		}
		return time.Date(y, time.Month(mo), d, h, mi, sec, 0, time.UTC).Unix(), p, true
	}
	y, mo, d, p, ok := date(s)
	if !ok {
		return 0, "", false
	}
	return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC).Unix(), p, true
}

func isBlank(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' }

// parseDate is toUnixSeconds as a number, for the tools' date ranges.
func parseDate(v any) (int64, error) {
	s, err := toUnixSeconds(v)
	if err != nil {
		return 0, err
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n, nil
}

// bindValue turns one JSON value into the string that is bound.
func bindValue(f *field, v any) (string, error) {
	switch v.(type) {
	case nil:
		return "", toolErr(`field "` + f.key + `" needs a value`)
	case map[string]any, []any:
		return "", toolErr(`field "` + f.key + `" needs a scalar value`)
	}
	switch f.typ {
	case fDateTs:
		return toUnixSeconds(v)
	case fInt:
		if i, ok := intValue(v); ok {
			return strconv.FormatInt(i, 10), nil
		}
		s, ok := v.(string)
		if !ok || s == "" || len(s) > 24 {
			return "", toolErr(`field "` + f.key + `" needs an integer`)
		}
		for i := 0; i < len(s); i++ {
			if i == 0 && (s[i] == '-' || s[i] == '+') {
				continue
			}
			if s[i] < '0' || s[i] > '9' {
				return "", toolErr(`field "` + f.key + `" needs an integer, got "` + s + `"`)
			}
		}
		return s, nil
	case fBool, fPresence, fPresenceNull:
		if b, ok := v.(bool); ok {
			if b {
				return "1", nil
			}
			return "0", nil
		}
		if i, ok := intValue(v); ok {
			if i != 0 {
				return "1", nil
			}
			return "0", nil
		}
		return "", toolErr(`field "` + f.key + `" needs true or false`)
	case fEnum:
		s, ok := v.(string)
		if !ok {
			return "", toolErr(`field "` + f.key + `" needs one of: ` + f.enumVals)
		}
		if !csvHas(f.enumVals, s) {
			return "", toolErr(`"` + s + `" is not valid for field "` + f.key +
				`"; expected one of: ` + f.enumVals)
		}
		return s, nil
	default:
		s, ok := v.(string)
		if !ok {
			return "", toolErr(`field "` + f.key + `" needs a string`)
		}
		if len(s) > maxValueLen {
			return "", toolErr(`value for "` + f.key + `" is too long (max ` +
				strconv.Itoa(maxValueLen) + ")")
		}
		return s, nil
	}
}

type compiler struct {
	schema          schema
	binds           []any
	nodes           int
	ftExpr, ftValue string
}

func (c *compiler) group(arr any, depth int, joiner string) (string, error) {
	a, ok := arr.([]any)
	if !ok {
		return "", toolErr(`"` + joiner + `" needs an array of filters`)
	}
	if len(a) == 0 {
		return "", toolErr(`"` + joiner + `" needs at least one filter`)
	}
	parts := make([]string, 0, len(a))
	for _, child := range a {
		s, err := c.node(child, depth+1)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	return "(" + strings.Join(parts, " "+joiner+" ") + ")", nil
}

func (c *compiler) leaf(node map[string]any) (string, error) {
	key, ok := node["field"].(string)
	if !ok {
		return "", toolErr(`a filter needs a string "field"`)
	}
	f := c.schema.find(key)
	if f == nil {
		return "", toolErr(`unknown field "` + key + `"`)
	}
	op := "="
	if s, ok := node["op"].(string); ok {
		op = s
	}
	expr := f.expr
	v, hasValue := node["value"]
	hasValue = hasValue && v != nil

	if f.typ == fFullText {
		if op != "match" {
			return "", toolErr(`field "` + key +
				`" is a full-text field; the only operator is "match"`)
		}
		if !hasValue {
			return "", toolErr(`"match" needs a value`)
		}
		s, err := bindValue(f, v)
		if err != nil {
			return "", err
		}
		c.binds = append(c.binds, s)
		if c.ftExpr == "" {
			c.ftExpr, c.ftValue = expr, s
		}
		return "(MATCH(" + expr + ") AGAINST(? IN BOOLEAN MODE))", nil
	}
	if op == "match" {
		return "", toolErr(`field "` + key + `" does not support "match"`)
	}
	ph := "?"
	if f.valExpr != "" {
		ph = f.valExpr
	}

	if op == "is_null" || op == "is_not_null" {
		if f.typ == fPresence {
			if op == "is_null" {
				return "(" + expr + " = 0)", nil
			}
			return "(" + expr + " <> 0)", nil
		}
		if op == "is_null" {
			return "(" + expr + " IS NULL)", nil
		}
		return "(" + expr + " IS NOT NULL)", nil
	}
	if !hasValue {
		return "", toolErr(`operator "` + op + `" on field "` + key + `" needs a value`)
	}

	if f.typ == fPresence && (op == "=" || op == "!=") {
		b, err := bindValue(f, v)
		if err != nil {
			return "", err
		}
		if (op == "=") == (b == "1") {
			return "(" + expr + " <> 0)", nil
		}
		return "(" + expr + " = 0)", nil
	}
	if f.typ == fPresenceNull {
		if op != "=" && op != "!=" {
			return "", toolErr(`field "` + key + `" is a flag; use = true, ` +
				`= false, is_null or is_not_null`)
		}
		b, err := bindValue(f, v)
		if err != nil {
			return "", err
		}
		if (op == "=") == (b == "1") {
			return "(" + expr + " IS NOT NULL)", nil
		}
		return "(" + expr + " IS NULL)", nil
	}

	switch op {
	case "in":
		a, ok := v.([]any)
		if !ok || len(a) == 0 {
			return "", toolErr(`"in" needs a non-empty array of values`)
		}
		if len(a) > maxInItems {
			return "", toolErr(`"in" accepts at most ` + strconv.Itoa(maxInItems) + " values")
		}
		phs := make([]string, len(a))
		for i, e := range a {
			s, err := bindValue(f, e)
			if err != nil {
				return "", err
			}
			phs[i] = ph
			c.binds = append(c.binds, s)
		}
		return "(" + expr + " IN (" + strings.Join(phs, ", ") + "))", nil
	case "between":
		a, ok := v.([]any)
		if !ok || len(a) != 2 {
			return "", toolErr(`"between" needs an array of exactly two values`)
		}
		for _, e := range a {
			s, err := bindValue(f, e)
			if err != nil {
				return "", err
			}
			c.binds = append(c.binds, s)
		}
		return "(" + expr + " BETWEEN " + ph + " AND " + ph + ")", nil
	case "=", "!=", "<", ">", "<=", ">=":
		s, err := bindValue(f, v)
		if err != nil {
			return "", err
		}
		c.binds = append(c.binds, s)
		return "(" + expr + " " + op + " " + ph + ")", nil
	case "contains", "not_contains", "starts_with":
		if f.typ != fText {
			return "", toolErr(`operator "` + op + `" needs a text field; "` +
				key + `" is not one`)
		}
		raw, err := bindValue(f, v)
		if err != nil {
			return "", err
		}
		if op == "starts_with" {
			c.binds = append(c.binds, likeEscape(raw)+"%")
		} else {
			c.binds = append(c.binds, "%"+likeEscape(raw)+"%")
		}
		sqlOp := "LIKE"
		if op == "not_contains" {
			sqlOp = "NOT LIKE"
		}
		return "(" + expr + " " + sqlOp + ` ? ESCAPE '\\')`, nil
	}
	return "", toolErr(`unknown operator "` + op + `" on field "` + key + `"`)
}

func (c *compiler) node(n any, depth int) (string, error) {
	if depth > maxDepth {
		return "", toolErr("filter is nested too deeply (max " + strconv.Itoa(maxDepth) + ")")
	}
	c.nodes++
	if c.nodes > maxNodes {
		return "", toolErr("filter has too many conditions (max " + strconv.Itoa(maxNodes) + ")")
	}
	o, ok := n.(map[string]any)
	if !ok {
		return "", toolErr("each filter must be a JSON object")
	}
	forms := 0
	for _, k := range []string{"and", "or", "not", "field"} {
		if has(o, k) {
			forms++
		}
	}
	if forms == 0 {
		return "", toolErr(`a filter must be a condition ("field") or a group ` +
			`("and", "or", "not")`)
	}
	if forms > 1 {
		return "", toolErr(`a filter must be exactly one of "field", "and", ` +
			`"or" or "not" -- not several at once`)
	}
	switch {
	case has(o, "and"):
		return c.group(o["and"], depth, "AND")
	case has(o, "or"):
		return c.group(o["or"], depth, "OR")
	case has(o, "not"):
		// A negated match excludes rather than ranks: every surviving
		// row scores 0, so only a positive match may drive the order.
		e, v := c.ftExpr, c.ftValue
		s, err := c.node(o["not"], depth+1)
		if err != nil {
			return "", err
		}
		c.ftExpr, c.ftValue = e, v
		return "(NOT " + s + ")", nil
	}
	return c.leaf(o)
}

// compile compiles a filter; null or {} is no filter.
func compile(s schema, n any) (compiled, error) {
	if n == nil {
		return compiled{}, nil
	}
	if o, ok := n.(map[string]any); ok && len(o) == 0 {
		return compiled{}, nil
	}
	c := &compiler{schema: s}
	sqlText, err := c.node(n, 0)
	if err != nil {
		return compiled{}, err
	}
	return compiled{sql: sqlText, binds: c.binds, ftExpr: c.ftExpr, ftValue: c.ftValue}, nil
}

// describeFields lists a schema's fields for a tool's description.
func describeFields(s schema) string {
	var b strings.Builder
	for _, f := range s {
		b.WriteString("  " + f.key + " (")
		switch f.typ {
		case fInt:
			b.WriteString("integer")
		case fText:
			b.WriteString("text")
		case fFullText:
			b.WriteString(`full-text, use op "match"`)
		case fEnum:
			b.WriteString("one of: " + f.enumVals)
		case fDateTs:
			b.WriteString("date")
		case fBool:
			b.WriteString("boolean")
		case fPresence:
			b.WriteString("presence, use op is_null / is_not_null")
		case fPresenceNull:
			b.WriteString("flag, use = true / = false")
		}
		b.WriteString(")")
		if f.desc != "" {
			b.WriteString(" -- " + f.desc)
		}
		b.WriteString("\n")
	}
	return b.String()
}

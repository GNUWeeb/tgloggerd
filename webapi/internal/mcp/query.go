// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// row is one result row by column name, a nil value for NULL, read as
// text the way the Drogon code reads its rows.
type row map[string]*string

// str is the column as text, "" for NULL (colStr).
func (r row) str(c string) string {
	if v := r[c]; v != nil {
		return *v
	}
	return ""
}

// i64 is the column as an integer, 0 for NULL (colI64).
func (r row) i64(c string) int64 {
	v := r[c]
	if v == nil {
		return 0
	}
	if n, err := strconv.ParseInt(*v, 10, 64); err == nil {
		return n
	}
	if u, err := strconv.ParseUint(*v, 10, 64); err == nil {
		return int64(u)
	}
	// A DECIMAL or a double (an aggregate): its integer part.
	f, _ := strconv.ParseFloat(*v, 64)
	return int64(f)
}

func (r row) null(c string) bool { return r[c] == nil }

// queryRows runs a statement and reads every row.
func queryRows(ctx context.Context, db *sql.DB, q string, binds []any) ([]row, error) {
	rs, err := db.QueryContext(ctx, q, binds...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	raw := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	var out []row
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := make(row, len(cols))
		for i, c := range cols {
			if _, dup := r[c]; dup {
				continue // the first column of a name, as Drogon's row[]
			}
			if raw[i] == nil {
				r[c] = nil
				continue
			}
			s := string(raw[i])
			r[c] = &s
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// dbMessage is the server's own wording of a database error, without
// the driver's "Error NNNN (SQLSTATE):" prefix, as the Drogon client
// reports it.
func dbMessage(err error) string {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Message
	}
	return err.Error()
}

// execTool runs a tool's query; a failure is a ToolError, the channel a
// caller can act on. A statement timeout says which knob to turn.
func execTool(ctx context.Context, db *sql.DB, q string, binds []any, what string) ([]row, error) {
	rows, err := queryRows(ctx, db, q, binds)
	if err == nil {
		return rows, nil
	}
	msg := dbMessage(err)
	if strings.Contains(msg, "maximum statement execution time") {
		return nil, toolErr(what + ": the query exceeded the 5-second limit. " +
			"Narrow it -- add a group_id, a date/edit_date " +
			"range, or a smaller limit. Note that combining " +
			"a broad text match with an explicit order_by " +
			"is the expensive case: without order_by the " +
			"same search is ordered by relevance, which " +
			"the full-text index can serve directly.")
	}
	if strings.Contains(msg, ftSyntaxPrefix) {
		return nil, toolErr(what + ": " + ftSyntaxHint + " (MySQL: " + msg + ")")
	}
	return nil, toolErr(what + ": " + msg)
}

// MySQL's full-text parser reports a value it cannot read with a
// message of its own, "syntax error, unexpected ..." (error 1064, as an
// SQL syntax error, whose message reads "You have an error in your SQL
// syntax"). Such a value is the caller's to fix, so say how; the
// Drogon web's execTool answers the same words.
const (
	ftSyntaxPrefix = "syntax error,"
	ftSyntaxHint   = "the match value is not valid full-text search syntax " +
		"(MySQL boolean mode). Write plain words, a phrase in double quotes, " +
		"and the operators + - ~ < > * ( ) only next to a word they apply to; " +
		"a value of operators alone, or ending in one, is refused"
)

const queryFailed = "query failed"

// placeholders is n "?" joined by sep.
func placeholders(n int, sep string) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?"+sep, n-1) + "?"
}

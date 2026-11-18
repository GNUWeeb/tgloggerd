// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package logdb

import "testing"

func TestMySQLText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"kernal~", "kernal"},
		{"kernal~1 +oops~2 -panic~", "kernal +oops -panic"},
		{"kern*~ ~kernel <a~ >b~0", "kern* ~kernel <a >b"},
		{"(kernal~)", "(kernal)"},
		{"a~b kernel~~", "a kernel"},
		{`"kernal~ oops" @3 x~`, `"kernal~ oops" @3 x`},
		{`"unclosed~`, `"unclosed~`},
		{"+kernel -panic \"a b\" @2 c*", "+kernel -panic \"a b\" @2 c*"},
		{"~ ~~", "~ ~~"},
		{"kérnel~ über", "kérnel über"},
	} {
		if got := mysqlText(c.in); got != c.want {
			t.Errorf("mysqlText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

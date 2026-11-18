// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"strconv"
	"strings"
)

// diffMaxLines bounds the LCS of unifiedDiff, which is O(n*m) over
// caller-controlled text: past it the diff is a summary line.
const diffMaxLines = 400

// unifiedDiff is a one-hunk, line-based unified diff between two
// versions of a message, as the Drogon web writes it: the common prefix
// and suffix trimmed, an LCS over the middle, a real "@@ -a,b +c,d @@"
// header. Equal texts give "".
func unifiedDiff(oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a := strings.Split(oldText, "\n")
	b := strings.Split(newText, "\n")

	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	an := len(a) - pre - suf
	bn := len(b) - pre - suf
	if an > diffMaxLines || bn > diffMaxLines {
		return "@@ too large to diff @@\n-" + strconv.Itoa(an) + " line(s) replaced by " +
			strconv.Itoa(bn) + " line(s); request format \"full\" for the text\n"
	}

	dp := make([][]uint16, an+1)
	for i := range dp {
		dp[i] = make([]uint16, bn+1)
	}
	for i := an - 1; i >= 0; i-- {
		for j := bn - 1; j >= 0; j-- {
			if a[pre+i] == b[pre+j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}

	start := func(n int) int {
		if n > 0 {
			return pre + 1
		}
		return pre
	}
	var out strings.Builder
	out.WriteString("@@ -" + strconv.Itoa(start(an)) + "," + strconv.Itoa(an) +
		" +" + strconv.Itoa(start(bn)) + "," + strconv.Itoa(bn) + " @@\n")
	i, j := 0, 0
	for i < an && j < bn {
		switch {
		case a[pre+i] == b[pre+j]:
			out.WriteString(" " + a[pre+i] + "\n")
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			out.WriteString("-" + a[pre+i] + "\n")
			i++
		default:
			out.WriteString("+" + b[pre+j] + "\n")
			j++
		}
	}
	for ; i < an; i++ {
		out.WriteString("-" + a[pre+i] + "\n")
	}
	for ; j < bn; j++ {
		out.WriteString("+" + b[pre+j] + "\n")
	}
	return out.String()
}

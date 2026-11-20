// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import "math"

// The session payload's numbers are read with std::stoull, std::stoll
// and std::stoul in the C++. These read them the same way, so a payload
// parses to the same session in both services: leading white space and
// a sign are allowed, anything after the digits is ignored, no digits
// or an out-of-range value is an error, and a minus sign on an unsigned
// number wraps it as strtoull does.

func isCSpace(c byte) bool {
	return c == ' ' || (c >= '\t' && c <= '\r')
}

// cDigits skips white space and a sign, then reads decimal digits. It
// returns the magnitude, whether it was negative, whether it fitted in
// 64 bits, and whether there was at least one digit.
func cDigits(s string) (mag uint64, neg, fits, ok bool) {
	i := 0
	for i < len(s) && isCSpace(s[i]) {
		i++
	}
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	fits = true
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		d := uint64(s[i] - '0')
		if mag > (math.MaxUint64-d)/10 {
			fits = false
		} else if fits {
			mag = mag*10 + d
		}
		i++
	}
	return mag, neg, fits, i > start
}

// stoull is std::stoull(s) (base 10).
func stoull(s string) (uint64, bool) {
	mag, neg, fits, ok := cDigits(s)
	if !ok || !fits {
		return 0, false
	}
	if neg {
		return -mag, true
	}
	return mag, true
}

// stoul is std::stoul(s) on LP64, where unsigned long is 64 bits.
func stoul(s string) (uint64, bool) {
	return stoull(s)
}

// stoll is std::stoll(s) (base 10).
func stoll(s string) (int64, bool) {
	mag, neg, fits, ok := cDigits(s)
	if !ok || !fits {
		return 0, false
	}
	if neg {
		if mag > 1<<63 {
			return 0, false
		}
		return int64(-mag), true
	}
	if mag > math.MaxInt64 {
		return 0, false
	}
	return int64(mag), true
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// JSON values are the decoded forms of encoding/json with UseNumber:
// nil, bool, string, json.Number, []any and map[string]any. Results add
// int64, int, float64 and []string.
//
// The answers are encoded by encode below rather than by
// encoding/json, so they come out byte for byte as the Drogon web's
// nlohmann::json dump() writes them: keys sorted, no HTML escaping, no
//   escapes, and floats in nlohmann's spelling ("0.0", "12.5",
// "1e-05").

// decode parses one JSON document, keeping numbers as json.Number so an
// integer is told apart from a float the way nlohmann::json tells them
// apart. Anything after the document is an error, as there.
func decode(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return v, nil
}

// intValue is v as an integer when nlohmann::json would hold it as one
// (is_number_integer): a number written without a fraction or an
// exponent that fits 64 bits. An unsigned value above the int64 range
// wraps, as get<long long>() wraps it.
func intValue(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok || strings.ContainsAny(string(n), ".eE") {
		return 0, false
	}
	if i, err := strconv.ParseInt(string(n), 10, 64); err == nil {
		return i, true
	}
	if u, err := strconv.ParseUint(string(n), 10, 64); err == nil {
		return int64(u), true
	}
	return 0, false
}

// has reports whether the object has the key (contains()).
func has(o map[string]any, k string) bool {
	_, ok := o[k]
	return ok
}

// isInt reports whether o[k] is an integer.
func isInt(o map[string]any, k string) bool {
	v, ok := o[k]
	if !ok {
		return false
	}
	_, ok = intValue(v)
	return ok
}

// isNull reports whether o[k] is present and null.
func isNull(o map[string]any, k string) bool {
	v, ok := o[k]
	return ok && v == nil
}

// boolArg is o[k] when it is a boolean.
func boolArg(o map[string]any, k string) (bool, bool) {
	b, ok := o[k].(bool)
	return b, ok
}

// stringArg is o[k] when it is a string.
func stringArg(o map[string]any, k string) (string, bool) {
	s, ok := o[k].(string)
	return s, ok
}

// encode writes v as nlohmann::json's dump() does.
func encode(v any) []byte {
	var b bytes.Buffer
	writeValue(&b, v)
	return b.Bytes()
}

func writeValue(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		writeString(b, x)
	case int:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case uint64:
		b.WriteString(strconv.FormatUint(x, 10))
	case float64:
		b.WriteString(formatFloat(x))
	case json.Number:
		// An echoed number (a request id) is written as nlohmann
		// stores it: an integer as such, anything else as a double.
		if !strings.ContainsAny(string(x), ".eE") {
			if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
				b.WriteString(strconv.FormatInt(i, 10))
				return
			}
			if u, err := strconv.ParseUint(string(x), 10, 64); err == nil {
				b.WriteString(strconv.FormatUint(u, 10))
				return
			}
		}
		f, err := x.Float64()
		if err != nil {
			f = math.Inf(1)
		}
		b.WriteString(formatFloat(f))
	case []string:
		b.WriteByte('[')
		for i, s := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, s)
		}
		b.WriteByte(']')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeValue(b, e)
		}
		b.WriteByte(']')
	case []map[string]any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeValue(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			writeValue(b, x[k])
		}
		b.WriteByte('}')
	default:
		// Not produced by this package; encoding/json is the fallback.
		j, err := json.Marshal(x)
		if err != nil {
			b.WriteString("null")
			return
		}
		b.Write(j)
	}
}

// writeString escapes as nlohmann does with ensure_ascii off: the
// quote, the backslash and the control characters, nothing else. Bytes
// that are not UTF-8 (which MySQL's utf8mb4 columns do not hold) become
// U+FFFD; nlohmann would refuse to write the answer at all.
func writeString(b *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if c < 0x20 {
					b.WriteString(`\u00`)
					b.WriteByte(hex[c>>4])
					b.WriteByte(hex[c&0xf])
				} else {
					b.WriteByte(c)
				}
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			b.WriteString("�")
		} else {
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	b.WriteByte('"')
}

// formatFloat spells a double as nlohmann::json does: the shortest
// digits that read back the same, a decimal point always present in
// fixed notation ("50.0"), fixed notation for decimal exponents from
// -4 to 15 and exponent notation with at least two exponent digits
// outside them. NaN and the infinities are null.
func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	// Shortest digits and the exponent of the first one.
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k := len(digits)
	n := x + 1 // position of the decimal point after the digits' start

	const minExp, maxExp = -4, 15
	var out string
	switch {
	case k <= n && n <= maxExp:
		out = digits + strings.Repeat("0", n-k) + ".0"
	case 0 < n && n <= maxExp:
		out = digits[:n] + "." + digits[n:]
	case minExp < n && n <= 0:
		out = "0." + strings.Repeat("0", -n) + digits
	default:
		m := digits[:1]
		if k > 1 {
			m += "." + digits[1:]
		}
		ex := n - 1
		es := "+"
		if ex < 0 {
			es = "-"
			ex = -ex
		}
		d := strconv.Itoa(ex)
		if ex < 10 {
			d = "0" + d
		}
		out = m + "e" + es + d
	}
	return sign + out
}

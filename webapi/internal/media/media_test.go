// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestStorePath(t *testing.T) {
	for _, c := range []struct{ dir, hex, ext, want string }{
		{"data/storage/files", hex, "jpg",
			"data/storage/files/01/23/45/67/89/" + hex + ".jpg"},
		{"/s", hex, "", "/s/01/23/45/67/89/" + hex},
		// The extension is reduced to [a-z0-9], at most 10.
		{"/s", hex, "../M P4x", "/s/01/23/45/67/89/" + hex + ".mp4x"},
		{"/s", hex, "abcdefghijklmnop", "/s/01/23/45/67/89/" + hex + ".abcdefghij"},
		// Not a SHA-256: no path, never one built from it.
		{"/s", "", "jpg", ""},
		{"/s", hex[:63], "jpg", ""},
		{"/s", "../../../../../../etc/passwd" + strings.Repeat("0", 36), "", ""},
	} {
		if got := StorePath(c.dir, c.hex, c.ext); got != c.want {
			t.Errorf("StorePath(%q, %q, %q) = %q, want %q", c.dir, c.hex, c.ext, got, c.want)
		}
	}
}

func TestSafeFilename(t *testing.T) {
	if got := SafeFilename("a\"b\\c/d\r\ne\x01f.pdf"); got != "abcdef.pdf" {
		t.Errorf("got %q", got)
	}
	if got := SafeFilename("Résumé 2026.pdf"); got != "Résumé 2026.pdf" {
		t.Errorf("got %q", got)
	}
}

func TestViewInline(t *testing.T) {
	for _, ty := range []string{"photo", "video", "audio", "voice", "animation", "sticker"} {
		if !ViewInline(ty) {
			t.Errorf("%s not inline", ty)
		}
	}
	for _, ty := range []string{"document", "unknown", ""} {
		if ViewInline(ty) {
			t.Errorf("%s inline", ty)
		}
	}
}

func TestContentType(t *testing.T) {
	for name, want := range map[string]string{
		"x.jpg":       "image/jpeg",
		"x.JPG":       "image/jpeg",
		"a/b/x.mp4":   "video/mp4",
		"x.webm":      "video/webm",
		"x.oga":       "audio/ogg",
		"x.txt":       "text/plain; charset=utf-8",
		"x.tgs":       "application/octet-stream",
		"noext":       "application/octet-stream",
		"report.pdf":  "application/pdf",
		"dir.v1/blob": "application/octet-stream",
	} {
		if got := ContentType(name); got != want {
			t.Errorf("ContentType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseFetchRange(t *testing.T) {
	for _, c := range []struct {
		h    string
		size uint64
		want FetchRange
	}{
		{"", 100, FetchRange{}},
		{"items=0-5", 100, FetchRange{}},
		{"bytes=10-19", 100, FetchRange{10, 10}},
		{"bytes=10-19", 0, FetchRange{10, 10}},
		{"bytes=10-", 0, FetchRange{10, 0}},
		{"bytes=10-19,30-39", 100, FetchRange{10, 10}},
		{"bytes=19-10", 100, FetchRange{}},
		{"bytes=-30", 100, FetchRange{70, 30}},
		{"bytes=-300", 100, FetchRange{0, 100}},
		// A suffix range needs the size.
		{"bytes=-30", 0, FetchRange{}},
		{"bytes=-0", 100, FetchRange{}},
		{"bytes=", 100, FetchRange{}},
		// Past the end is the daemon's to refuse (416).
		{"bytes=500-", 100, FetchRange{500, 0}},
	} {
		if got := ParseFetchRange(c.h, c.size); got != c.want {
			t.Errorf("ParseFetchRange(%q, %d) = %+v, want %+v", c.h, c.size, got, c.want)
		}
	}
}

func TestPolicy(t *testing.T) {
	p := Policy{Enabled: true, StoreCap: 1000}
	for _, c := range []struct {
		size     uint64
		signedIn bool
		want     Decision
	}{
		{999, false, Allowed},
		{0, false, Allowed}, // unknown size counts as small
		{1000, false, NeedsSession},
		{1000, true, Allowed},
	} {
		if got := p.MayFetch(c.size, c.signedIn); got != c.want {
			t.Errorf("MayFetch(%d, %v) = %v, want %v", c.size, c.signedIn, got, c.want)
		}
	}
	if (Policy{Enabled: true}).MayFetch(1<<40, false) != Allowed {
		t.Error("no cap: every file is small")
	}
	if (Policy{StoreCap: 1000}).MayFetch(1, true) != Off {
		t.Error("off without the daemon API")
	}
}

func TestInsideTDLibDir(t *testing.T) {
	base := t.TempDir()
	data := filepath.Join(base, "data")
	td := filepath.Join(data, "tdlib")
	other := filepath.Join(data, "tdlib-other")
	for _, d := range []string{filepath.Join(td, "videos"), other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string) {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	good := filepath.Join(td, "videos", "file_1.mp4")
	write(good)
	write(filepath.Join(other, "f"))
	write(filepath.Join(base, "secret"))
	if err := os.Symlink(filepath.Join(base, "secret"), filepath.Join(td, "videos", "link")); err != nil {
		t.Fatal(err)
	}

	root := TDLibDir(data)
	if root != td {
		t.Fatalf("TDLibDir = %q, want %q", root, td)
	}
	if p, ok := InsideTDLibDir(root, good); !ok || p != good {
		t.Errorf("good path refused: %q %v", p, ok)
	}
	for _, p := range []string{
		filepath.Join(td, "videos", "..", "..", "..", "secret"),
		filepath.Join(td, "videos", "link"), // a symlink out
		filepath.Join(other, "f"),           // a sibling with the prefix
		td,                                  // the directory itself
		filepath.Join(td, "videos"),         // not a regular file
		filepath.Join(td, "missing"),
		"videos/file_1.mp4", // relative
		"",
	} {
		if q, ok := InsideTDLibDir(root, p); ok {
			t.Errorf("%q accepted as %q", p, q)
		}
	}
	if _, ok := InsideTDLibDir("", good); ok {
		t.Error("accepted without TG_DATA_DIR")
	}
	if TDLibDir("") != "" {
		t.Error("TDLibDir of an empty TG_DATA_DIR")
	}
}

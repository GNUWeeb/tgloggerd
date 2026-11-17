// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package media holds the rules /files/<token> serves files by, as
// web/src/controllers/MediaController.cpp and web/src/media/Fetch.cpp
// apply them in the Drogon web (web/docs/media.md): where a stored
// file's bytes are, what it is called and sent as, which byte range to
// ask the daemon for, which TDLib paths may be opened, and who may make
// the daemon fetch a file at all.
package media

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}

// SafeExt keeps only [a-z0-9] of a file extension, lower-cased, at most
// 10 characters.
func SafeExt(e string) string {
	var b strings.Builder
	for i := 0; i < len(e) && b.Len() < 10; i++ {
		c := e[i]
		switch {
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
			b.WriteByte(c)
		case 'A' <= c && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		}
	}
	return b.String()
}

// SafeFilename drops the bytes that are unsafe in a Content-Disposition
// filename: control characters, '"', '\\' and '/'.
func SafeFilename(n string) string {
	var b strings.Builder
	for i := 0; i < len(n); i++ {
		c := n[i]
		if c < 0x20 || c == '"' || c == '\\' || c == '/' {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// ViewInline reports whether a file type is shown in the page; the
// others (documents, unknown) are offered as a download.
func ViewInline(fileType string) bool {
	switch fileType {
	case "photo", "video", "audio", "voice", "animation", "sticker":
		return true
	}
	return false
}

// StorePath is where the store keeps a blob:
// <dir>/aa/bb/cc/dd/ee/<hex>[.ext], from the row's SHA-256 and
// extension only, the layout of the daemon's blobPath()
// (src/tgloggerd/StoragePath.hpp). It is "" when hex is not a SHA-256.
func StorePath(dir, hex, ext string) string {
	if !isHex64(hex) {
		return ""
	}
	parts := []string{dir}
	for i := 0; i < 5; i++ {
		parts = append(parts, hex[i*2:i*2+2])
	}
	name := hex
	if e := SafeExt(ext); e != "" {
		name += "." + e
	}
	return filepath.Join(append(parts, name)...)
}

// IsRegular reports whether path names a regular file (after
// symlinks).
func IsRegular(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// FetchRange is the part of a file to ask the daemon for. Length 0
// means "from Offset on", which the daemon answers with one chunk.
type FetchRange struct {
	Offset uint64
	Length uint64
}

// strtoull reads the leading decimal digits of s as the C library
// does for the Drogon web: no digits read as 0, and too many saturate.
func strtoull(s string) uint64 {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := i
	for j < len(s) && '0' <= s[j] && s[j] <= '9' {
		j++
	}
	if j == i {
		return 0
	}
	n, err := strconv.ParseUint(s[i:j], 10, 64)
	if err != nil {
		return ^uint64(0)
	}
	return n
}

// ParseFetchRange turns a Range header into the range to fetch, for a
// file not in the store whose size may be unknown (0). Only the first
// range of "bytes=..." counts; anything that is not a usable byte range
// asks from the start, and "bytes=-N" needs a known size.
func ParseFetchRange(header string, knownSize uint64) FetchRange {
	var r FetchRange
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return r
	}
	if i := strings.IndexByte(spec, ','); i >= 0 {
		spec = spec[:i]
	}
	s, e, ok := strings.Cut(spec, "-")
	if !ok {
		return r
	}
	if s == "" {
		var n uint64
		if e != "" {
			n = strtoull(e)
		}
		if n == 0 || knownSize == 0 {
			return r
		}
		n = min(n, knownSize)
		return FetchRange{Offset: knownSize - n, Length: n}
	}
	start := strtoull(s)
	if e == "" {
		return FetchRange{Offset: start}
	}
	end := strtoull(e)
	if end < start {
		return r
	}
	return FetchRange{Offset: start, Length: end - start + 1}
}

// Policy decides who may make the daemon fetch a file the store does
// not hold (docs/design/storage-cache.md section 11). It is the one
// place that decides it: /files/<token> asks it before calling the
// daemon, and the chat data marks media fetchable from the same answer.
type Policy struct {
	// Enabled: the daemon API is configured (WEB_TGLOGGERD_API).
	Enabled bool
	// StoreCap is the daemon's TG_MAX_STORE_FILE_SIZE: files under it
	// are fetched whole into the store. 0 = no cap.
	StoreCap uint64
}

// Decision is the Policy's answer.
type Decision int

const (
	// Allowed: ask the daemon.
	Allowed Decision = iota
	// Off: the daemon API is not configured; nothing can fetch.
	Off
	// NeedsSession: a big file, and the request is not signed in.
	NeedsSession
)

// IsSmall reports whether a file of this size is stored whole by a
// fetch.
func (p Policy) IsSmall(size uint64) bool {
	return p.StoreCap == 0 || size < p.StoreCap
}

// MayFetch decides for a file of this size (telegram_files.file_size,
// 0 when unknown). Streaming a big file from TDLib's cache is for
// signed-in requests only, so a leaked token costs one small file at
// most. An unknown size counts as small here; the caller refuses a
// "partial" answer to an anonymous request anyway.
func (p Policy) MayFetch(size uint64, signedIn bool) Decision {
	if !p.Enabled {
		return Off
	}
	if !signedIn && !p.IsSmall(size) {
		return NeedsSession
	}
	return Allowed
}

// weaklyCanonical makes p absolute and resolves "..", "." and the
// symlinks of its longest existing prefix, as std::filesystem's
// weakly_canonical does for the Drogon web.
func weaklyCanonical(p string) (string, error) {
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	var rest []string
	for {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(append([]string{r}, rest...)...), nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(append([]string{p}, rest...)...), nil
		}
		rest = append([]string{filepath.Base(p)}, rest...)
		p = parent
	}
}

// TDLibDir is the directory the daemon serves partial files from: the
// absolute, canonical form of <dataDir>/tdlib, resolved against the
// working directory, which is the repository root (/workspace) in every
// container. It is "" when dataDir (TG_DATA_DIR) is empty.
func TDLibDir(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	d, err := weaklyCanonical(filepath.Join(dataDir, "tdlib"))
	if err != nil {
		return ""
	}
	return d
}

// InsideTDLibDir resolves a path the daemon answered with and accepts
// it only when it names a regular file inside root (TDLibDir) after
// resolving "..", symlinks and the like. It returns the resolved path.
func InsideTDLibDir(root, path string) (string, bool) {
	if root == "" || !strings.HasPrefix(path, "/") {
		return "", false
	}
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	p = filepath.Clean(p)
	// Component-wise prefix: /a/tdlib-other is not inside /a/tdlib, and
	// the directory itself is not a file inside it.
	prefix := root
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(p, prefix) || len(p) == len(prefix) {
		return "", false
	}
	if !IsRegular(p) {
		return "", false
	}
	return p, true
}

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"fmt"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
)

// A document previews by its extension (MediaRender): the chat's
// media, the message page, the files list and a group's Files tab
// render it as an image, a video, audio, a pdf or text.

// documentRenders are the extensions of each render a document takes.
var documentRenders = map[string]string{
	"image": "jpg jpeg png gif webp avif bmp ico apng svg",
	"video": "mp4 m4v webm mov ogv",
	"audio": "mp3 m4a aac ogg oga opus wav flac",
	"pdf":   "pdf",
	"text": "txt log md csv tsv json xml yaml yml toml ini conf cfg sh py c h " +
		"cpp hpp go rs js ts java kt sql diff patch html htm",
}

func TestRenderDocumentByExtension(t *testing.T) {
	t.Parallel()
	for want, exts := range documentRenders {
		for _, ext := range strings.Fields(exts) {
			for _, e := range []string{ext, strings.ToUpper(ext)} {
				f := logdb.FileRef{Type: "document", Ext: e}
				if got := renderOf("document", "", f); got != want {
					t.Errorf("document .%s: %q, want %q", e, got, want)
				}
				if got := fileRender("document", e); got != want {
					t.Errorf("file row .%s: %q, want %q", e, got, want)
				}
			}
		}
	}
	// Anything else stays a download.
	for _, e := range []string{"", "zip", "exe", "docx", "tgs", "xhtml", "svgz", "php", "mkv"} {
		if got := renderOf("document", "", logdb.FileRef{Type: "document", Ext: e}); got != "file" {
			t.Errorf("document .%s: %q, want file", e, got)
		}
	}
	// A document whose row says photo stays an image, as before.
	if got := renderOf("document", "", logdb.FileRef{Type: "photo"}); got != "image" {
		t.Errorf("photo row: %q", got)
	}
}

// The extension previews a document only: other media keep their
// render, and a file that is not a message's document (an unsupported
// message's, an unknown type's) stays a file.
func TestRenderOtherMediaUnchanged(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ctype, ext, want string
	}{
		{"photo", "jpg", "image"},
		{"video", "mp4", "video"},
		{"video_note", "mp4", "video"},
		{"animation", "mp4", "animation"},
		{"audio", "pdf", "audio"},
		{"voice", "oga", "audio"},
		{"unsupported", "png", "file"},
		{"unknown", "txt", "file"},
	} {
		f := logdb.FileRef{Type: c.ctype, Ext: c.ext}
		if got := renderOf(c.ctype, "", f); got != c.want {
			t.Errorf("%s .%s: %q, want %q", c.ctype, c.ext, got, c.want)
		}
	}
	for _, c := range []struct {
		fileType, ext, want string
	}{
		{"photo", "jpg", "image"},
		{"sticker", "webp", "image"},
		{"sticker", "webm", "animation"},
		{"sticker", "tgs", "lottie"},
		{"video", "mp4", "video"},
		{"voice", "ogg", "audio"},
		{"unknown", "png", "file"},
	} {
		if got := fileRender(c.fileType, c.ext); got != c.want {
			t.Errorf("file row %s .%s: %q, want %q", c.fileType, c.ext, got, c.want)
		}
	}
}

// The group's Files tab and the message page carry the render, on the
// Tabs Lab fixture: message 57 is a pdf document, 60 a txt one.
func TestDocumentRenderInPages(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, tabsFixture)
	want := map[float64]string{
		tabsMsgID(61): "image", tabsMsgID(53): "video", tabsMsgID(54): "animation",
		tabsMsgID(55): "audio", tabsMsgID(56): "audio", tabsMsgID(57): "pdf",
		tabsMsgID(58): "image", tabsMsgID(59): "video", tabsMsgID(60): "text",
	}
	seen := 0
	for _, r := range tabRows(tabProps(t, e, "admin", tabsPath("files", ""))) {
		w, ok := want[r["message_id"].(float64)]
		if !ok {
			continue
		}
		seen++
		if r["render"] != w {
			t.Errorf("files tab message %v (%v .%v): render %v, want %s",
				r["message_id"], r["file_type"], r["ext"], r["render"], w)
		}
	}
	if seen != len(want) {
		t.Errorf("files tab: %d of %d rows seen", seen, len(want))
	}

	for n, w := range map[int]string{57: "pdf", 60: "text", 53: "video"} {
		_, m := e.page("admin", fmt.Sprintf("/messages/group/%d", 6000+n))
		mm, _ := m["props"].(map[string]any)["message"].(map[string]any)
		md, _ := mm["media"].(map[string]any)
		if md == nil || md["render"] != w {
			t.Errorf("message page %d: media %v, want render %s", n, md, w)
		}
	}
}

// The files list's rows carry the render (files fixture: 860004 is a
// pdf document).
func TestFilesListRender(t *testing.T) {
	t.Parallel()
	e := newFilesEnv(t, mediaOpts{})
	ck, _ := e.signIn("render")
	_, rows := e.filesPage(ck)
	f := byFileID(rows)
	for id, w := range map[string]string{"860001": "image", "860003": "video",
		"860004": "pdf", "860005": "audio"} {
		if f[id]["render"] != w {
			t.Errorf("%s: render %v, want %s", id, f[id]["render"], w)
		}
	}
}

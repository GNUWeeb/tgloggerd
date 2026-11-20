// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/media"
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

const sandbox = "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'"

// /files/<token> sends a previewable document inline, typed by its
// extension. Every answer has nosniff, and every inline one but video,
// audio and pdf the sandbox, so html and svg run nothing when opened
// by their URL. Other documents stay downloads, media shown in the
// page stay inline without a disposition, and ?download=1 always
// gives the attachment.
func TestDocumentPreviewHeaders(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{})
	for i, c := range []struct {
		typ, ext, name string
		disp, ctype    string
		sandboxed      bool
	}{
		{"document", "png", "shot.png", "inline; filename=shot.png", "image/png", true},
		{"document", "JPEG", "a.JPEG", "inline; filename=a.JPEG", "image/jpeg", true},
		{"document", "svg", "x.svg", "inline; filename=x.svg", "image/svg+xml", true},
		{"document", "mp4", "clip.mp4", "inline; filename=clip.mp4", "video/mp4", false},
		{"document", "opus", "v.opus", "inline; filename=v.opus", "audio/ogg", false},
		{"document", "pdf", "p.pdf", "inline; filename=p.pdf", "application/pdf", false},
		{"document", "log", "run.log", "inline; filename=run.log", "text/plain; charset=utf-8", true},
		{"document", "html", "page.html", "inline; filename=page.html", "text/plain; charset=utf-8", true},
		{"document", "js", "app.js", "inline; filename=app.js", "text/plain; charset=utf-8", true},
		// The extension types it, not the name: an html name on a
		// png is still an image.
		{"document", "png", "evil.html", "inline; filename=evil.html", "image/png", true},
		// Not previewed: a download typed by its name, as before.
		{"document", "zip", "a.zip", "attachment; filename=a.zip", "application/zip", false},
		{"document", "xhtml", "a.xhtml", "attachment; filename=a.xhtml",
			"application/xhtml+xml; charset=utf-8", false},
		{"unknown", "png", "u.png", "attachment; filename=u.png", "image/png", false},
		// Media shown in the page: inline, no disposition, sandboxed
		// but for video and audio.
		{"photo", "jpg", "", "", "image/jpeg", true},
		{"video", "mp4", "c.mp4", "", "video/mp4", false},
		{"voice", "ogg", "", "", "audio/ogg", false},
		// A media row whose blob has an active extension: an svg is
		// sandboxed, an html (xhtml) page goes out as text.
		{"photo", "svg", "", "", "image/svg+xml", true},
		{"sticker", "svg", "", "", "image/svg+xml", true},
		{"photo", "html", "", "", "text/plain; charset=utf-8", true},
		{"sticker", "xhtml", "", "", "text/plain; charset=utf-8", true},
		{"photo", "xml", "", "", "application/xml; charset=utf-8", true},
	} {
		id := uint64(700 + i)
		data := []byte(fmt.Sprintf("<svg onload=alert(%d)>", i))
		tok := e.addFile(fileRow{id: id, data: data, ext: c.ext, typ: c.typ, name: c.name, onDisk: true})
		g := e.get("/files/"+tok, "")
		expect(t, g, 200, string(data))
		what := c.typ + " ." + c.ext
		check := func(k, want string) {
			t.Helper()
			if g.h.Get(k) != want {
				t.Errorf("%s: %s = %q, want %q", what, k, g.h.Get(k), want)
			}
		}
		check("Content-Disposition", c.disp)
		check("Content-Type", c.ctype)
		check("Content-Security-Policy", map[bool]string{true: sandbox}[c.sandboxed])
		check("X-Content-Type-Options", "nosniff")

		// ?download=1: always the attachment, typed by its name, with
		// nosniff and no sandbox (a download runs nothing).
		g = e.get("/files/"+tok+"?download=1", "")
		expect(t, g, 200, string(data))
		name := c.name
		if name == "" {
			sum := sha256.Sum256(data)
			name = hex.EncodeToString(sum[:]) + "." + c.ext
		}
		check("Content-Disposition", "attachment; filename="+name)
		check("Content-Type", media.ContentType(name))
		check("Content-Security-Policy", "")
		check("X-Content-Type-Options", "nosniff")
		// Another value leaves the choice to the file.
		g = e.get("/files/"+tok+"?download=0", "")
		check("Content-Disposition", c.disp)
	}
}

// A previewed document streamed from TDLib's cache carries the same
// headers, and ?download=1 the attachment.
func TestDocumentPreviewPartial(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 10})
	cookie, _ := e.signIn("partial")
	part := filepath.Join(e.tdlib, "videos", "file_9.html")
	content := []byte("<script>alert(1)</script>")
	if err := os.WriteFile(part, content, 0o644); err != nil {
		t.Fatal(err)
	}
	tok := e.addFile(fileRow{id: 60, typ: "document", ext: "html", name: "big.html", size: 1000})
	e.fake.answer(60, 200, "", fmt.Sprintf(
		`{"state":"partial","path":%q,"offset":0,"available":%d,"size":1000}`, part, len(content)))
	g := e.get("/files/"+tok, cookie)
	expect(t, g, 206, string(content))
	expectHeader(t, g, "Content-Type", "text/plain; charset=utf-8")
	expectHeader(t, g, "Content-Disposition", "inline; filename=big.html")
	expectHeader(t, g, "Content-Security-Policy", sandbox)
	expectHeader(t, g, "X-Content-Type-Options", "nosniff")

	g = e.get("/files/"+tok+"?download=1", cookie)
	expect(t, g, 206, string(content))
	expectHeader(t, g, "Content-Type", "text/html; charset=utf-8")
	expectHeader(t, g, "Content-Disposition", "attachment; filename=big.html")
	expectHeader(t, g, "Content-Security-Policy", "")
	expectHeader(t, g, "X-Content-Type-Options", "nosniff")
}

// Media rows fetched and stored, or streamed from TDLib's cache, carry
// nosniff and the sandbox as stored ones do: a stored answer's blob of
// a photo row typed svg, a partial html file of a sticker row, a
// partial video without the sandbox.
func TestMediaSandboxFetched(t *testing.T) {
	t.Parallel()
	e := newMediaEnv(t, mediaOpts{cap: 10})
	cookie, _ := e.signIn("sandbox")
	data := []byte("<svg onload=alert(1)/>")
	e.addFile(fileRow{id: 70, data: data, ext: "svg", typ: "photo", onDisk: true})
	tok := e.addFile(fileRow{id: 71, typ: "photo", ext: "svg", size: 5})
	e.fake.answer(71, 200, "", `{"state":"stored","id":70}`)
	g := e.get("/files/"+tok, cookie)
	expect(t, g, 200, string(data))
	expectHeader(t, g, "Content-Type", "image/svg+xml")
	expectHeader(t, g, "Content-Security-Policy", sandbox)
	expectHeader(t, g, "X-Content-Type-Options", "nosniff")

	for _, c := range []struct {
		id        uint64
		typ, file string
		ctype     string
		csp       string
	}{
		{72, "sticker", "file_72.html", "text/plain; charset=utf-8", sandbox},
		{73, "video", "file_73.mp4", "video/mp4", ""},
	} {
		part := filepath.Join(e.tdlib, "videos", c.file)
		content := []byte("<script>alert(1)</script>")
		if err := os.WriteFile(part, content, 0o644); err != nil {
			t.Fatal(err)
		}
		tok := e.addFile(fileRow{id: c.id, typ: c.typ, ext: filepath.Ext(c.file)[1:], size: 1000})
		e.fake.answer(c.id, 200, "", fmt.Sprintf(
			`{"state":"partial","path":%q,"offset":0,"available":%d,"size":1000}`, part, len(content)))
		g := e.get("/files/"+tok, cookie)
		expect(t, g, 206, string(content))
		expectHeader(t, g, "Content-Type", c.ctype)
		expectHeader(t, g, "Content-Disposition", "")
		expectHeader(t, g, "Content-Security-Policy", c.csp)
		expectHeader(t, g, "X-Content-Type-Options", "nosniff")
	}
}

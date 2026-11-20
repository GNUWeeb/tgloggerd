// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package media

import "strings"

// A document (a file sent "as a file") is previewed by its extension:
// a screenshot sent as a file shows like a photo, a video file plays, a
// text file shows its text. The pages draw it by its render (the API's
// MediaRender) and /files/<token> serves it by the same table, so what
// the page draws and what the browser gets agree.
const (
	RenderImage = "image"
	RenderVideo = "video"
	RenderAudio = "audio"
	RenderPDF   = "pdf"
	RenderText  = "text"
	RenderFile  = "file"
)

// documentRenders maps a lower-case extension to the render of a
// document that has it; every other extension is a file (a download).
// html and htm are text: their source is shown, never rendered. svg is
// an image for an img element only, which runs no script in it.
var documentRenders = map[string]string{}

func init() {
	for render, exts := range map[string]string{
		RenderImage: "jpg jpeg png gif webp avif bmp ico apng svg",
		RenderVideo: "mp4 m4v webm mov ogv",
		RenderAudio: "mp3 m4a aac ogg oga opus wav flac",
		RenderPDF:   "pdf",
		RenderText: "txt log md csv tsv json xml yaml yml toml ini conf cfg " +
			"sh py c h cpp hpp go rs js ts java kt sql diff patch html htm",
	} {
		for _, e := range strings.Fields(exts) {
			documentRenders[e] = render
		}
	}
}

// DocumentRender is how a document with the extension renders: image,
// video, audio, pdf, text, or file.
func DocumentRender(ext string) string {
	if r, ok := documentRenders[strings.ToLower(ext)]; ok {
		return r
	}
	return RenderFile
}

// textType is what a text document is sent as, whatever its own type
// in contentTypes (text/html for .html, text/javascript for .js,
// video/mp2t for .ts): the browser shows the bytes and interprets none
// of them.
const textType = "text/plain; charset=utf-8"

// previewTypes are the types of previewed extensions that are not in
// contentTypes, which follows the Drogon web's table.
var previewTypes = map[string]string{
	"opus": "audio/ogg",
}

// PreviewType is the Content-Type a previewed document with the
// extension is sent inline with: text/plain for text, else its type
// from the extension.
func PreviewType(ext string) string {
	ext = strings.ToLower(ext)
	if DocumentRender(ext) == RenderText {
		return textType
	}
	if t, ok := contentTypes[ext]; ok {
		return t
	}
	if t, ok := previewTypes[ext]; ok {
		return t
	}
	return octetStream
}

// PreviewSandbox is the Content-Security-Policy of a document sent
// inline as an image or as text: an svg, or an html file served as
// text but opened by a browser that would sniff it, runs nothing and
// loads nothing but its own images. It is not sent for pdf, where it
// would stop the browser's own viewer, nor for video and audio, which
// run no script.
const PreviewSandbox = "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'"

// Sandboxed is whether a previewed document of the render is sent with
// PreviewSandbox.
func Sandboxed(render string) bool {
	return render == RenderImage || render == RenderText
}

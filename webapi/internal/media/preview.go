// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package media

import "strings"

// A document (a file sent "as a file") is previewed by its extension:
// a screenshot sent as a file shows like a photo, a video file plays, a
// text file shows its text. The pages draw it by its render (the API's
// MediaRender).
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

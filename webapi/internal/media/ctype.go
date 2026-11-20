// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package media

import (
	"path/filepath"
	"strings"
)

// octetStream is sent for a name whose extension is not in the table.
const octetStream = "application/octet-stream"

// contentTypes maps a lower-case extension to the Content-Type the
// Drogon web sends for it (its file type and MIME tables, HttpUtils.cc),
// so a file is sent with the same type by either service. The standard
// library's table differs (and depends on the host's mime.types), which
// would change how browsers handle the same file.
var contentTypes = map[string]string{
	"aac":   "audio/aac",
	"ac3":   "audio/ac3",
	"aif":   "audio/aiff",
	"aifc":  "audio/aiff",
	"aiff":  "audio/aiff",
	"apg":   "video/apg",
	"ape":   "audio/x-ape",
	"apng":  "image/apng",
	"av1":   "video/av01",
	"avi":   "video/x-msvideo",
	"avif":  "image/avif",
	"bmp":   "image/bmp",
	"bz":    "application/x-bzip",
	"bz2":   "application/x-bzip2",
	"css":   "text/css; charset=utf-8",
	"csv":   "text/csv; charset=utf-8",
	"doc":   "application/msword",
	"docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"eot":   "application/vnd.ms-fontobject",
	"flac":  "audio/flac",
	"gif":   "image/gif",
	"gz":    "application/gzip",
	"htm":   "text/html; charset=utf-8",
	"html":  "text/html; charset=utf-8",
	"icns":  "image/icns",
	"ico":   "image/vnd.microsoft.icon",
	"j2k":   "image/jp2",
	"jar":   "application/java-archive",
	"j2c":   "image/jp2",
	"jp2":   "image/jp2",
	"jpeg":  "image/jpeg",
	"jpc":   "image/jp2",
	"jpf":   "image/jp2",
	"jpg":   "image/jpeg",
	"jpg2":  "image/jp2",
	"jpm":   "image/jp2",
	"jpx":   "image/jp2",
	"js":    "text/javascript; charset=utf-8",
	"json":  "application/json; charset=utf-8",
	"lzma":  "application/x-xz",
	"m1a":   "audio/mpeg",
	"m1v":   "video/mpeg",
	"m2a":   "audio/mpeg",
	"m2ts":  "video/mp2t",
	"m2v":   "video/mpeg",
	"m4a":   "audio/mp4",
	"m4v":   "video/x-m4v",
	"mjs":   "text/javascript; charset=utf-8",
	"mka":   "audio/matroska",
	"mkv":   "video/matroska",
	"mng":   "image/x-mng",
	"mov":   "video/quicktime",
	"mp1":   "audio/mpeg",
	"mp2":   "audio/mpeg",
	"mp3":   "audio/mpeg",
	"mp4":   "video/mp4",
	"mpa":   "audio/mpeg",
	"mpe":   "video/mpeg",
	"mpeg":  "video/mpeg",
	"mpg":   "video/mpeg",
	"mpv":   "video/mpeg",
	"oga":   "audio/ogg",
	"ogg":   "audio/ogg",
	"ogv":   "video/ogg",
	"otf":   "application/x-font-opentype",
	"pdf":   "application/pdf",
	"php":   "application/x-httpd-php",
	"png":   "image/png",
	"rar":   "application/vnd.rar",
	"svg":   "image/svg+xml",
	"tar":   "application/x-tar",
	"targa": "image/x-tga",
	"tif":   "image/tiff",
	"tiff":  "image/tiff",
	"tga":   "image/x-tga",
	"tgz":   "application/x-tgz",
	"ts":    "video/mp2t",
	"tta":   "audio/x-tta",
	"ttf":   "application/x-font-truetype",
	"txt":   "text/plain; charset=utf-8",
	"w64":   "audio/wav",
	"wav":   "audio/wav",
	"wave":  "audio/wav",
	"wasm":  "application/wasm",
	"weba":  "audio/webm",
	"webm":  "video/webm",
	"webp":  "image/webp",
	"wma":   "audio/x-ms-wma",
	"woff":  "application/font-woff",
	"woff2": "application/font-woff2",
	"wv":    "audio/x-wavpack",
	"xht":   "application/xhtml+xml; charset=utf-8",
	"xhtml": "application/xhtml+xml; charset=utf-8",
	"xml":   "application/xml; charset=utf-8",
	"xsl":   "text/xsl; charset=utf-8",
	"xz":    "application/x-xz",
	"zip":   "application/zip",
	"7z":    "application/x-7z-compressed",
}

// ContentType is the Content-Type for a file name, from its extension:
// the download name when the file is offered as one, else the name on
// disk (a blob's or TDLib's, which carry the extension).
func ContentType(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if t, ok := contentTypes[ext]; ok {
		return t
	}
	return octetStream
}

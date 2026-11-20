# web-doc-preview

Preview a document by its file extension, as the chat already shows
photos, videos and voice: a screenshot sent as a file opens like a
photo, a video file plays, a text file shows its text.

## Why

A file sent "as a document" (for example a PNG screenshot, as on
/messages/group/10202853) is drawn as a plain file row and served as
a download, although the browser could show it.

## Decisions (main session, at the owner's request, 2026-11-20)

- By extension (lower case), when the file is a document:
  - image: jpg, jpeg, png, gif, webp, avif, bmp, ico, apng, and svg
    (svg only through an img element, see below);
  - video: mp4, m4v, webm, mov, ogv;
  - audio: mp3, m4a, aac, ogg, oga, opus, wav, flac;
  - pdf: opened in a new tab, not embedded;
  - text: txt, log, md, csv, tsv, json, xml, yaml, yml, toml, ini,
    conf, cfg, sh, py, c, h, cpp, hpp, go, rs, js, ts, java, kt, sql,
    diff, patch, and html, htm as source text (never rendered).
- Anything else stays a download, as today.
- Serving: a previewable document is sent inline (Content-Disposition
  inline with its name) with X-Content-Type-Options: nosniff; images,
  svg and text also with a Content-Security-Policy `sandbox;
  default-src 'none'; img-src 'self'; style-src 'unsafe-inline'`, so
  an svg or html file opened directly runs nothing. Text and source
  files are served as text/plain; charset=utf-8, whatever their own
  type. PDF is sent as application/pdf without the sandbox, which
  would stop the browser's own viewer. `?download=1` on a file URL
  always gives the attachment.
- The text view shows at most the first 512 KiB, escaped, with a note
  and the download when cut.
- A download stays offered beside every preview.
- Fetching follows web-fetch-permission's rules unchanged: a big
  unstored document is fetched only through its explicit action.

## Scope

- Go (webapi): the media render of a document by extension (image,
  video, audio, plus new `text` and `pdf`), in ChatMedia, FileRow,
  GroupFileRow and the message page; the serving headers above and
  `?download=1`; web/docs/media.md and OpenAPI follow.
- React (frontend): such documents drawn as their render in the chat,
  the message page, /files and a group's Files tab: images in the
  photo viewer, video and audio with the chat's players, text in a
  code view loaded from the file URL, pdf as an "Open" link to a new
  tab; never innerHTML or an iframe for text, html or svg.

## Depends on

Nothing.

## Done when

- The PNG screenshot document on /messages/group/10202853 opens in
  the photo viewer; an mp4 document plays; a .log document shows its
  text; a .html document shows its source; a pdf opens in a new tab.
- An svg or html document opened by its URL runs no script (CSP
  sandbox), checked by a Go test of the headers and an end-to-end
  test.
- Go tests cover the render per extension and the headers; React
  tests cover each drawing.

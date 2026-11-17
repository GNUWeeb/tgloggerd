# web-fetch-unstored-media

Serve files that are not in the store by fetching them through the
daemon, with byte ranges for big files, and show unstored media as
loadable instead of "not stored".

## Why

After eviction and with non-member media recorded without bytes, many
files the pages show are no longer in the store. Without this item
they would all be 404s.

## Scope

- `MediaController::serve` as storage-cache.md section 11 describes:
  `alias_of`, the fetch call, `stored` and `partial` answers, 206 with
  the full size in Content-Range, 503 with `Retry-After`, 410, a
  missing blob treated as unstored, and small-files-only for requests
  without a session.
- The chat templates, `chat-render.js`, `_edit_media.html` and the
  other pages listed in the design: unstored photos and stickers load
  normally, video, audio and documents get "Load from Telegram";
  the chat JSON gains `fetchable`.
- `web/docs/media.md` (started by web-access-reporting) and
  `web/README.web` (the stale "/media" and Range notes).
- Not in scope: the React rework, which takes the contract from
  `web/docs/media.md`.

## Design notes

- The daemon's side is done: its contract is the
  `POST /v1/files/{id}/fetch` section of docs/internal-api.md. The
  reply field is `state` (not `status`), a stored reply carries the
  `id` of the row holding the bytes, and an offset past the end of a
  big file is a 416.
- Validate the daemon's `path` against the TDLib directory before
  serving it; never serve a path from outside it.
- `newFileResponse(path, offset, length, true, ...)` gives the 206;
  set Content-Range's total from the file size the daemon reports, not
  from the partial file's size on disk.
- The audit row per authenticated request stays as it is.

## Depends on

Nothing.

## Done when

- tgloggerd_web builds.
- At cutover: an over-cap video plays and seeks at tgd.gnuweeb.org; an
  evicted photo shows and becomes stored; an anonymous request for a
  big file is refused; the daemon stopped gives 503, not a crash.

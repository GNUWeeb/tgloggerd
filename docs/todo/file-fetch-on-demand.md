# file-fetch-on-demand

Fetch a file from Telegram when it is asked for: whole and into the
store for small files, a byte range at a time for big ones.

## Why

Evicted files, files recorded without their bytes and files over the
per-file cap can only be shown if the daemon can get them back from
Telegram quickly, and big videos must play without being downloaded
whole first.

## Scope

- The fetch service (storage-cache.md section 8): resolve a row to a
  TDLib file (remote id first, then a fresh source), full fetch and
  store for small files with `alias_of` on a sha256 collision, ranged
  fetch for big files, per-file serialization, `TG_FETCH_MAX_ACTIVE`,
  `TG_FETCH_TIMEOUT_SECS`, `TG_FETCH_CHUNK`, the one-hour memory of
  files gone from Telegram, idle deletion after `TG_FETCH_IDLE_SECS`,
  and waiting for an ingest download already in progress.
- `POST /v1/files/{id}/fetch` on the internal API, and its part of
  `docs/internal-api.md`.
- `.env.example` entries.
- Not in scope: the web side (web-fetch-unstored-media).

## Design notes

- Uses `TDLib::request` from the API threads, never from the loop.
- Reuse `message_content_file` and `link_preview_image` to take the
  file out of a re-fetched message; reuse `storeDownloadedFile` to
  store, so the store layout stays in one place.
- The reply's `path` must be a path the web can open: TDLib reports
  absolute paths inside the daemon container, which the web container
  sees at the same place (`/workspace/...`).
- Answer 410 for an inaccessible chat or a deleted message, 503 when
  the limit is reached or the client is not authorized, 504 on timeout.

## Depends on

- `internal-api-server`

## Done when

- tgloggerd builds.
- At cutover: a metadata-only photo becomes stored through the
  endpoint; a range of an over-cap video comes back as `partial` with
  the right path and length, and its TDLib file is gone 15 minutes
  after the last request.

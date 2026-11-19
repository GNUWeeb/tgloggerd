# web-fetch-permission

Make fetching an unstored file from Telegram an explicit action:
superadmins and admins may, members only with `fetch_files`.

## Why

web-fetch-unstored-media lets the web ask the daemon to fetch files
the store does not hold. Each fetch costs Telegram traffic and disk,
and the project owner wants it to be a deliberate action ("Fetch from
Telegram", approved in the mockup's file preview) of accounts allowed
to do it, not something every page view triggers.

## Scope

The Go service's half is done (web/docs/media.md, "Fetch from
Telegram"): page views link small unstored files only, a big one has
`url` null and `fetch`, the path of `POST /api/files/{token}/fetch`
(fetchFile in webapi/openapi.yaml). What is left is the React
application's half:

- Show a "Fetch from Telegram" action for media and file rows whose
  `url` is null and `fetch` is set (chat media, the files list and
  preview, a group's Files tab), instead of "not stored". Clicking it
  POSTs `fetch` with the CSRF token and, on 200, uses the answer's
  `url` to show, play or download the file; 403
  `fetch_not_allowed`, 410 `gone`, 503 `unavailable` (with
  `Retry-After`) and 504 `timeout` get their messages.
- The places that now read `url` as "stored or fetchable" (a file
  with `fetchable` true and `url` null is fetchable through the
  action): web/app/src/routes/chat-media.tsx (the header comment, and
  the photo, video, animation, sticker, audio and document
  renderers), web/app/src/ui/filegrid.tsx (the preview's action,
  missing note and "Stored" detail), web/app/src/routes/files.tsx and
  web/app/src/routes/group-files.tsx (the "Stored" pill and detail).

## Depends on

Nothing.

## Done when

- A member with `fetch_files` fetches a big unstored file from its
  preview and then views it; a member without it sees no action (and
  the API answers it 403); a small unstored file still loads by
  itself.

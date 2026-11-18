# parity-files

Let the new stack get an unstored file from Telegram as the Drogon web
does, and close the files list's gaps.

## Why

The parity audit (2026-11-18) found that the new files list gives no
link for an unstored file and its "Fetch from Telegram" has no
operation behind it; the Drogon web fetches on view for signed-in
viewers.

## Scope

- Unstored files answered with a /files/<token> URL whenever the
  Drogon rules would let the viewer fetch it (the anonymous small-file
  rule, signed-in otherwise), so the chat's "Load from Telegram" and
  the files list work; web-fetch-permission later narrows who may.
- Files list: tg_file_id, sha256 and accessed_at filters; id, ext and
  accessed_at sorts; the SHA-256, last access and extension columns.
- webapi/openapi.yaml exact for everything added.

## Depends on

Nothing.

## Done when

- An unstored file opens from the chat and the files list through the
  daemon fetch, with tests against a fake daemon, and the files rows
  of the audit are served.

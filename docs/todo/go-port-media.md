# go-port-media

Serve `/files/<token>` from the Go service, including fetching
unstored files through the daemon.

## Why

Media serving is the busiest endpoint, and Go's standard file serving
handles Range and conditional requests natively (docs/design/web-go.md).

## Scope

- Everything `web/src/controllers/MediaController.cpp` and
  `web/src/media/Fetch.cpp` do: tokens, alias_of, the store layout,
  Range, the daemon fetch with stored and partial answers, the TDLib
  path check, the anonymous small-files rule, access reporting,
  caching headers, the audit row; the request scope of
  docs/design/web-accounts.md section 8 as far as it exists.
- nginx routes `/files/` to the Go service; `web/docs/media.md`
  follows.

## Depends on

Nothing.

## Done when

- Every case in web/docs/media.md behaves the same from the Go
  service, checked with the same requests against both before
  switching nginx over.

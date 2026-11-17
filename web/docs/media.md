# Media: `/files/<token>`

How the web serves stored files. The design behind it is
[../../docs/design/storage-cache.md](../../docs/design/storage-cache.md)
sections 10 and 11; the daemon side is
[../../docs/internal-api.md](../../docs/internal-api.md).

## The URL

`/files/<token>` is public: the token is the file id
(`telegram_files.id`) encrypted under `WEB_APP_KEY`
(`src/auth/FileToken.cpp`), so it cannot be guessed or enumerated, and
no session is needed. A token that does not decrypt answers 404, like
an unknown id.

## Which bytes

- A row with `alias_of` set holds no bytes of its own; the bytes of the
  row it names are served (one hop). The download name and the
  inline-or-download choice still come from the token's row.
- The file is read from `WEB_STORAGE_DIR` at a path derived from the
  row's SHA-256 and extension only, never from the request.
- A row whose bytes are not stored answers 404 "File not stored"; a
  stored row whose blob is missing answers 404 "File not found".
- A single `Range: bytes=...` is honoured with a 206; an unsatisfiable
  one gets 416.

## Caching

Files are sent with `Cache-Control: public, max-age=604800`: a week,
without `immutable`. A token's bytes never change, so they are safe to
cache, but a browser that keeps showing a file asks for it again at
least weekly, and that request counts as an access below. A year and
`immutable`, as before, hid repeat views from the eviction record
entirely.

## Access reporting

The daemon evicts the least recently accessed files, and only it
writes `telegram_files`, so the web tells it which files it serves:

- Every file served (200 or 206) adds the id of the row whose bytes
  were sent, after `alias_of`, to a process-wide set. Nothing on the
  request path waits for the daemon.
- Every 60 seconds a timer on the main loop sends the set to
  `POST /v1/files/access` in batches of at most 1000 ids and empties
  it. A file served many times in a minute is reported once.
- Best effort: when the daemon is away, slow (5 second timeout) or
  refuses, that minute's ids are dropped, not retried, and one line is
  logged until a report succeeds again. A file viewed again is simply
  reported again.
- With `WEB_TGLOGGERD_API` empty nothing is recorded or sent.

# Media: `/files/<token>`

How the web serves files, from the store or fetched from Telegram
through the daemon. The design behind it is
[../../docs/design/storage-cache.md](../../docs/design/storage-cache.md)
sections 10 and 11; the daemon side is
[../../docs/internal-api.md](../../docs/internal-api.md).

## Which service serves it

Two implementations exist while the web moves to Go
([../../docs/design/web-go.md](../../docs/design/web-go.md)): the Drogon
web's (`src/controllers/MediaController.cpp`, `src/media/Fetch.cpp`)
and the Go service's (`webapi/internal/server/media.go`,
`webapi/internal/media/`). nginx decides which one answers: `/files/`
goes to Drogon until a `location /files/` sends it to the Go service
([../../webapi/README](../../webapi/README), "Routing /files/ through
nginx"). Both read the same store, TDLib directory, settings and
tokens, so a link works on either. What follows holds for both, except
where "Differences" says otherwise.

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
- A stored row (`on_disk = 1`) whose blob is present is read from
  `WEB_STORAGE_DIR` at a path derived from the row's SHA-256 and
  extension only, never from the request.
- A single `Range: bytes=...` is honoured with a 206; an unsatisfiable
  one gets 416.
- Anything else is not in the store: evicted, never fetched (media from
  chats the account is not a member of), over the daemon's per-file
  cap, or a stored row whose blob is missing on disk. Such a file is
  fetched from Telegram, below.

## Files not in the store

The web asks the daemon to fetch the file, `POST /v1/files/{id}/fetch`
([internal-api.md](../../docs/internal-api.md)), for the token's row,
and waits for the answer (the daemon's `TG_FETCH_TIMEOUT_SECS` plus 10
seconds at most). The `Range` header becomes the fetch's `offset` and
`length`; without one, or with an open-ended one, it asks from the
start or from the range's first byte for one chunk (`TG_FETCH_CHUNK`).

- `stored`: the file was small enough (under `TG_MAX_STORE_FILE_SIZE`)
  and is now in the store, possibly as another row's bytes; the `id`
  in the answer names the row, and its blob is served exactly as a
  stored file, Range included. It is served from the store from then
  on.
- `partial`: a big file. The web serves the `available` bytes from
  `offset` (fewer if the Range asked for fewer) straight from TDLib's
  cache at `path`, as a 206 whose `Content-Range` total is the file's
  full `size` from the answer, not the size of the partial file on
  disk. A media player asks for the next range as it plays or seeks.
  When the answer covers the whole file from byte 0, it is a 200.
  `path` must name a regular file inside the TDLib directory,
  `<TG_DATA_DIR>/tdlib/` resolved against the working directory
  (`/workspace` in the compose deployment, as for the daemon), after
  resolving `..` and symlinks; anything else is refused with 502 and
  logged. The web reads `TG_DATA_DIR` from the environment it shares
  with the daemon.

Who may fetch is decided in one place (`src/media/Fetch.cpp`,
`mayFetch()`), shared by this route and the chat pages:

- With `WEB_TGLOGGERD_API` empty nothing is fetched: 404 "File not
  stored".
- A request without a session may fetch small files only (under
  `TG_MAX_STORE_FILE_SIZE`, read from the shared environment; any file
  when it is 0). A big file, or a `partial` answer for a file whose
  size was unknown, is refused with 403 "Sign in to load this file".
  That bounds what a leaked token can cost: one small file stored, not
  a stream of a big one.

The daemon's answers map to:

| Daemon | Browser |
|---|---|
| 503 (busy, fetch limit, TDLib not authorized) | 503 with the daemon's `Retry-After` (5 if it sent none) |
| no answer (daemon stopped, unreachable, timed out) | 503 with `Retry-After: 10` |
| 410 (gone from Telegram, not accessible) | 410 "File no longer available" |
| 504 (Telegram slow) | 504; the download goes on, so asking again soon is cheap |
| 404 | 404 |
| 416 (offset past the end of a big file) | 416, `Content-Range: bytes */<file_size>` |
| anything else (400, 401, 500, a malformed answer) | 502, logged |

Served bytes, from the store or from TDLib, count as an access (below)
and, for a signed-in request, write the same `media` audit row as a
stored file.

## Chat pages

The chat JSON (`/v1/chats/{scope}/{id}/messages`, and the data behind
the server-rendered chat pages) gives every media object (message
media, album items, link-preview images, edit snapshots) a `stored`
flag and a `fetchable` flag: `fetchable` is true when the bytes are not
stored and the viewer may fetch them by the rule above (a public view,
without a session, gets it only for small files). The pages render:

- a stored file, or an unstored photo or sticker that is fetchable, as
  usual: images load lazily and are fetched when they come into view;
- an unstored, fetchable video, animation (not an animated sticker),
  audio or document as "Load from Telegram": a video or animation is a
  `<video preload="none">` box, so nothing is fetched before it is
  clicked, that opens in the viewer and plays from there; audio is a
  player with `preload="none"`; a document is a download link with its
  name and size;
- an unstored file that cannot be fetched as "not stored, only
  metadata kept".

The Go service's chat API (`GET /api/chats/{scope}/{id}/messages`, and
the React chat pages' data; webapi/openapi.yaml, ChatMedia) applies the
same rule to message media, album items and link-preview images. Its
media objects carry `url` only when the file is stored or fetchable
(null otherwise), and `thumb` with the same URL for pictures. Edit
snapshots there carry no media.

## Files list

Every row of the Drogon files list (`/files`) links to the file's
`/files/<token>`, stored or not, so opening an unstored one fetches it
by the rule above. The Go service's files page data (FilesProps,
FileRow in webapi/openapi.yaml) gives each row `url` when the file is
stored or fetchable by the same rule and a `fetchable` flag as in the
chat API; the list's thumbnails are of stored pictures only, so
showing the list fetches nothing.

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
  were sent, after `alias_of` (for a `stored` fetch, the answer's
  `id`), to a process-wide set. Nothing on the request path waits for
  the daemon.
- Every 60 seconds a timer on the main loop sends the set to
  `POST /v1/files/access` in batches of at most 1000 ids and empties
  it. A file served many times in a minute is reported once.
- Best effort: when the daemon is away, slow (5 second timeout) or
  refuses, that minute's ids are dropped, not retried, and one line is
  logged until a report succeeds again. A file viewed again is simply
  reported again.
- With `WEB_TGLOGGERD_API` empty nothing is recorded or sent.

## Differences between the two

The Go service answers the same URLs with the same status codes and
headers. Where it does more, it is the standard file serving of Go's
`http.ServeContent`, which browsers and media players expect:

- A stored file also carries `Last-Modified` (the blob's), and
  `If-Modified-Since` and `If-Range` are honoured: a browser
  revalidating its copy gets a 304. Drogon sends no validators.
- Several ranges in one `Range` header get a `multipart/byteranges`
  206; Drogon serves the first range only. `bytes=0-` is a 206 of the
  whole file; Drogon makes it a 200.
- A session counts only when it is valid against the account's
  `session_epoch`, as for every other request; Drogon checks only the
  cookie's signature and expiry on this route. A session ended by a
  password change is anonymous on the Go service: it fetches small
  files only and writes no audit row.
- A signed-in account fetches only with the `fetch_files` ability
  (web-accounts.md section 5): superadmins and admins hold it, a
  member only when given it. A member without it gets no link to an
  unstored file (`fetchable` false, `url` null), and a link it holds
  answers 404 "File not stored" without asking the daemon.
- Error answers are a bare HTML page with the same heading and text,
  not the Drogon layout.


# tgloggerd internal API

A small HTTP/1.1 + JSON API inside the tgloggerd daemon, for the web.
Only the daemon may hold the TDLib session, so whatever the web needs
from Telegram or from the file store's bookkeeping goes through here.
It is a control plane: small JSON requests and replies. File bytes
never travel over it; they move through the repository mount that
every container shares at `/workspace`. The design behind it is
[design/storage-cache.md](design/storage-cache.md) section 7.

It is a curated set of operations, never a TDLib passthrough: nothing
the web sends can post a message or join a chat.

## Running it

| Variable | Default | Meaning |
|---|---|---|
| `TG_API_LISTEN` | empty | `host:port` (or `[v6]:port`) to listen on. Empty: the API is off and nothing listens. |
| `TG_API_THREADS` | 4 | Handler threads. Each may hold a database connection, so the daemon's default DB pool grows by this many when the API is on. |
| `WEB_APP_KEY` | none | The secret the web and the daemon share; the bearer key is derived from it (below). |

In the compose deployment `TG_API_LISTEN=0.0.0.0:8091`. Inside the
container that means loopback and the tgnet address 10.0.88.5; the
port is not published on the host, and must not be.

The daemon starts the API after the database and TDLib are set up, and
stops it before anything its handlers use goes away. When the API
cannot start (no or a bad `WEB_APP_KEY`, a bad `TG_API_LISTEN`, a bind
failure) the daemon logs an error and keeps logging Telegram without
it; clients see a refused connection and treat the daemon as away.

## Transport

- HTTP/1.1 with keep-alive; an HTTP/1.0 request is answered and closed
  unless it asks for keep-alive.
- Request bodies are JSON, at most 64 KiB (more: 413). The
  `Content-Type` is not checked. Replies are always
  `application/json` with `Cache-Control: no-store`.
- A connection idle for 30 seconds, or a peer that stalls a read or a
  write for that long, is closed. At most 256 connections are open at
  once; more are closed on accept.
- One I/O thread does the network work; the operations run on the
  `TG_API_THREADS` handler threads and may take a while (a fetch waits
  for Telegram). At most 4 requests per handler thread are queued or
  running; beyond that a request is answered 503 with
  `Retry-After: 1` at once instead of waiting behind slow ones.
- At shutdown the daemon stops accepting, answers the requests already
  running (closing their connections), closes idle connections and
  then exits. A request that arrives on a kept-alive connection during
  shutdown gets 503.
- The query string is accepted and ignored by the endpoints below.

## Authentication

Every request carries

    Authorization: Bearer <key>

where `<key>` is 64 lowercase hex digits: the 32-byte keyed BLAKE2b
(libsodium `crypto_generichash`, output length 32) of the ASCII label

    tgloggerd:internal-api:v1

keyed with the base64-decoded `WEB_APP_KEY`. Any of the standard and
URL-safe base64 alphabets, padded or not, is accepted, and the decoded
key must be at least 16 bytes. This is the construction the
`/files/<token>` subkeys use (`src/tgloggerd/FileToken.cpp`,
`web/src/auth/FileToken.cpp`), with its own label, so there is no new
secret to configure and the key changes when `WEB_APP_KEY` does.

The daemon compares the key in constant time (`sodium_memcmp`) before
it looks at the path, so an unauthenticated client learns nothing about
which endpoints exist. The scheme name is case-insensitive.

To compute the key by hand, for curl:

    python3 -c 'import base64,hashlib,sys; print(hashlib.blake2b(b"tgloggerd:internal-api:v1", key=base64.b64decode(sys.argv[1]), digest_size=32).hexdigest())' "$WEB_APP_KEY"

## Errors

An error is answered with its status and a JSON body

    {"error": "<code>", "message": "<human-readable detail>"}

Clients act on the status and `error`; `message` is for logs and may
change.

| Status | `error` | When |
|---|---|---|
| 400 | `bad_request` | Malformed: a body that is not JSON or not of the endpoint's shape, a bad id, too many ids. |
| 401 | `unauthorized` | Missing or wrong bearer key. Comes with `WWW-Authenticate: Bearer`. |
| 404 | `not_found` | Unknown file id, or no such endpoint. |
| 405 | `method_not_allowed` | The path exists under another method; `Allow` lists them. |
| 410 | `gone` | The file is gone from Telegram or not accessible to the account. |
| 413 | `too_large` | Request body over 64 KiB. The connection is closed. |
| 500 | `internal` | A bug or a database failure; the daemon logs the detail. |
| 503 | `unavailable` | Busy (too many requests in flight, a fetch limit reached), the TDLib client not authorized, or the daemon shutting down. Comes with `Retry-After` (seconds). |
| 504 | `timeout` | The operation did not finish in time (a fetch waiting on Telegram). |

## Endpoints

File ids are `telegram_files.id`, decimal, 1 to 2^64-1; anything else
in their place is a 400. Times are Unix seconds.

### GET /v1/files/{id}

A file's cache state.

    200 {
      "id": 123,
      "on_disk": true,
      "size": 48213,
      "type": "photo",
      "ext": "jpg",
      "alias_of": null,
      "has_unique_id": true,
      "fetchable": true,
      "accessed_at": 1763400000
    }

- `on_disk`: the bytes are in the store. False means evicted, never
  fetched, or over the per-file cap; such a file can be fetched.
- `size`: `file_size` in bytes (0 if unknown).
- `type`: `telegram_files.file_type` (`photo`, `video`, `document`,
  `audio`, `voice`, `sticker`, `animation`, `unknown`).
- `ext`: the lowercase extension without the dot, or null.
- `alias_of`: the id of the row that holds the same bytes, or null.
  Readers follow it one hop (design section 3).
- `has_unique_id`: TDLib's `remote.unique_id` is recorded for the row.
- `fetchable`: whether asking for a fetch can succeed. Every row keeps
  its `tg_file_id`, so this is true for now; file-fetch-on-demand makes
  it false for files it found gone from Telegram in the last hour.
- `accessed_at`: the file's last recorded use.

Errors: 400 bad id, 404 unknown file.

### POST /v1/files/access

Records that the web used these files (design section 10).

    {"ids": [123, 456, 789]}

    200 {"ids": 3, "updated": 1}

- `ids`: at most 1000 positive integers; duplicates count once and
  unknown ids are ignored.
- For each row whose `accessed_at` is more than an hour old, sets
  `accessed_at = NOW()` and keeps `updated_at`, which records changes
  to the row and an access is not one. Rows accessed within the hour
  are left alone, so a file viewed over and over costs one write an
  hour at most.
- In the reply, `ids` is the number of distinct ids received and
  `updated` the number of rows whose `accessed_at` moved.

The web sends this best effort, batched every 60 seconds; a failed
batch is dropped, never retried into a request path.

Errors: 400 if the body is not `{"ids": [...]}` with positive integer
ids, or carries more than 1000.

### POST /v1/files/{id}/fetch

Added by `file-fetch-on-demand`, which fills in this section. Planned
shape (design section 8):

    {"offset": 0, "length": 1048576}

- A small file (under `TG_MAX_STORE_FILE_SIZE`) is downloaded whole
  into the store: `{"status": "stored"}`, and the web serves it from
  the store as usual (following `alias_of` if the bytes turned out to
  belong to another row).
- A big file is fetched a range at a time:
  `{"status": "partial", "path": "<TDLib local path>", "offset": ...,
  "available": ..., "size": ...}`, the bytes from `offset` that are
  present at `path` on the shared mount.

Errors: 404 unknown file, 410 gone or not accessible, 503 busy or not
authorized, 504 timed out.

### GET /v1/storage

Added by `storage-janitor`, which fills in this section. Planned: the
stored bytes, the cap, the retention, the dry-run flag and the result
of the last sweep (design section 9).

### POST /v1/storage/sweep

Added by `storage-janitor`, which fills in this section. Planned: runs
a sweep now, optionally as a dry run, and answers what it evicted or
would evict.

## Adding an endpoint

The server core (`src/tgloggerd/InternalApi.cpp`) has no operations of
its own. The part of the daemon that owns an operation registers it in
`TgLoggerd::start()` before the API starts, as
`registerFileApi()` (`src/tgloggerd/FileApi.cpp`) does:

    api.route("GET", "/v1/files/{id}", [](const InternalApi::Request &req) {
            uint64_t id = parseFileId(req.param("id"));
            ...
            InternalApi::Response r;
            r.body = { ... };
            return r;
    });

- A pattern's segments are literal or `{name}`; literal segments win
  over captures. The capture is `req.param("name")`, the parsed body
  `req.body` (null when the request has none).
- Handlers run on the handler threads, never on the TDLib loop thread,
  so they may block on the database or on `TDLib::request`.
- To answer an error, throw `InternalApi::Error` (`badRequest`,
  `notFound`, `gone`, `unavailable`, `timeout`, or any status and code
  from the table above). A `nlohmann::json` type error while reading
  the body becomes a 400; any other exception a 500, logged.
- Document the endpoint in this file in the same change.

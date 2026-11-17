# tgloggerd internal API

A small HTTP/1.1 + JSON API inside the tgloggerd daemon, for the web.
Only the daemon may hold the TDLib session, so whatever the web needs
from Telegram or from the file store's bookkeeping goes through here.
It is a control plane: small JSON requests and replies. File bytes
never travel over it; they move through the repository mount that
every container shares at `/workspace`. The design behind it is
[design/storage-cache.md](design/storage-cache.md) section 7.

It is a curated set of operations, never a TDLib passthrough: nothing
the web sends can post a message, and the one operation that changes
the account, joining a chat, takes only an invite link or a public
username that the daemon parses itself.

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
| 403 | `forbidden` | The deployment does not allow it: a real sweep asked for while `TG_STORAGE_DRY_RUN=1`. |
| 404 | `not_found` | Unknown file id, or no such endpoint. |
| 405 | `method_not_allowed` | The path exists under another method; `Allow` lists them. |
| 410 | `gone` | The file is gone from Telegram or not accessible to the account. |
| 413 | `too_large` | Request body over 64 KiB. The connection is closed. |
| 416 | `bad_range` | A fetch's `offset` is at or past the end of the file. |
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
- `fetchable`: whether asking for a fetch can succeed: false for a
  file found gone from Telegram within the last hour (a fetch would be
  answered 410 at once), true otherwise. Every row keeps its
  `tg_file_id`, so any other file may be fetched.
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

Gets a file's bytes from Telegram while someone waits for them (design
section 8). The body is optional:

    {"offset": 0, "length": 1048576}

- `offset`: the first byte wanted, default 0.
- `length`: how many bytes from `offset`, default 0, meaning one chunk
  (`TG_FETCH_CHUNK`). The daemon never fetches more than one chunk per
  request; an open-ended or longer request gets a chunk, and the
  client asks again for the rest.

A small file, under `TG_MAX_STORE_FILE_SIZE` (every file when that is
0), is downloaded whole and stored exactly as ingest stores it;
`offset` and `length` are ignored:

    200 {"state": "stored", "id": 123}

- `id`: the row that now holds the bytes. It is the requested id, or,
  when the bytes turned out to be another row's (same sha256), that
  row: the requested row then has `alias_of` set to it. Serve the file
  from the store by that row's sha256 and extension, as for any stored
  file.
- A file already in the store (its blob present, directly or through
  `alias_of`) is answered the same way without asking Telegram.

A big file is never copied into the store. The range is downloaded
into TDLib's cache and the answer says where it is:

    200 {
      "state": "partial",
      "path": "/workspace/data/users/main/tdlib/videos/file_12.mp4",
      "offset": 0,
      "available": 16777216,
      "size": 2147483648
    }

- `path`: TDLib's file, absolute, always under
  `/workspace/<TG_DATA_DIR>/tdlib/` (the daemon checks; a client should
  check again before opening it). Every container mounts the
  repository at `/workspace`, so the web opens the same path.
- `available`: how many bytes from `offset` are present at `path`. At
  least 1, at most `size - offset`; it may be more than was asked for
  when an earlier request already fetched further, and it may be less
  when Telegram delivered only part of the range within the time
  limit. Serve at most `available` bytes from `offset`, and only those:
  the rest of the file may be holes.
- `size`: the file's full size in bytes, for `Content-Range`.

The partial file stays in TDLib's cache while it is in use and is
deleted once no fetch has asked for it in `TG_FETCH_IDLE_SECS`.

How it works, and its limits:

- The file is found by its remote id; when that no longer downloads
  (an expired file reference), something referencing it is fetched
  from Telegram again: the newest live message carrying it, or the
  group or user whose current photo it is.
- Downloads of one file run one at a time; a request waits for the one
  in flight (within its own time limit). A file ingest is downloading
  is waited for, not requested again with another range.
- At most `TG_FETCH_MAX_ACTIVE` fetches run at once; more are answered
  503 with `Retry-After: 1` at once.
- Each fetch, from the request to the answer, is bounded by
  `TG_FETCH_TIMEOUT_SECS`; past it the answer is 504 and the download
  goes on in the background, so asking again soon is cheap.
- A file found gone from Telegram (the messages carrying it deleted,
  the chat no longer accessible) is answered 410 for an hour without
  asking Telegram again, and `GET /v1/files/{id}` says
  `"fetchable": false` meanwhile.

| Variable | Default | Meaning |
|---|---|---|
| `TG_FETCH_MAX_ACTIVE` | 4 | Fetches running at once. Keep it at most `TG_API_THREADS`. |
| `TG_FETCH_CHUNK` | 16777216 | Most bytes one request fetches (16 MiB). |
| `TG_FETCH_TIMEOUT_SECS` | 30 | Time limit of one fetch. |
| `TG_FETCH_IDLE_SECS` | 900 | A partial file not asked for this long is deleted from TDLib's cache. |

Errors: 400 bad id or body, 404 unknown file, 410 gone from Telegram
or not accessible, 416 `offset` past the end of a big file, 503 too
many fetches, TDLib not authorized or Telegram asking to wait (with
`Retry-After`), 504 timed out, 500 the downloaded file could not be
stored.

### GET /v1/storage

The store's state and the storage janitor's last sweep (design
section 9).

    200 {
      "files": 812345,
      "bytes": 561903112704,
      "max_bytes": 536870912000,
      "low_watermark": 510027366400,
      "retention_days": 90,
      "sweep_secs": 600,
      "dry_run": true,
      "sweeping": false,
      "pending": false,
      "last_sweep": {
        "started_at": 1763400000,
        "finished_at": 1763400042,
        "dry_run": true,
        "completed": true,
        "error": null,
        "retention": {"files": 120345, "bytes": 40265318400},
        "cap": {"files": 30211, "bytes": 11610427904},
        "bytes_before": 561903112704,
        "bytes_after": 510027366400,
        "kept": 0,
        "missing_blobs": 0,
        "unlink_errors": 0
      }
    }

- `files`, `bytes`: the rows with `on_disk = 1` and the sum of their
  `file_size`, read when asked. This is the logical size the cap is
  measured in; btrfs compression makes the space used lower, and blobs
  no row describes are not counted.
- `max_bytes`: `TG_STORAGE_MAX_BYTES`, 0 for no cap. `low_watermark`:
  what a sweep over the cap evicts down to, 95% of it (0 for no cap).
- `retention_days`: `TG_STORAGE_RETENTION_DAYS`, 0 when off.
  `sweep_secs`: `TG_STORAGE_SWEEP_SECS`, 0 when sweeps only run on
  request.
- `dry_run`: `TG_STORAGE_DRY_RUN`; true means the janitor only reports
  what it would evict. It cannot be changed over the API.
- `sweeping`: a sweep is running now. `pending`: one was asked for and
  has not started yet.
- `last_sweep`: the last sweep that ended, null before the first one.
  - `dry_run`: whether it only reported.
  - `completed`: it went through both phases. False with `error` set
    when it failed (a database error) or was stopped by a shutdown;
    the counts are then what it did before that.
  - `retention`, `cap`: files and bytes evicted by each phase, or, in a
    dry run, that each would evict. Retention goes first; the cap
    phase counts only what is left after it.
  - `bytes_before`, `bytes_after`: the stored bytes when the sweep
    started and when it ended (in a dry run, what would be left).
  - `kept`: files the guard kept because they were accessed or stored
    again while the sweep ran (always 0 in a dry run).
  - `missing_blobs`: evicted files whose blob was already gone, which
    is not an error. `unlink_errors`: evicted files whose blob could
    not be removed; the daemon logs each, and the blob is left as an
    orphan.

### POST /v1/storage/sweep

Asks the janitor for a sweep now.

    {"dry_run": true}

    202 {"queued": true, "dry_run": true}

- The body is optional; without one, or without `dry_run`, the sweep
  runs in the deployment's mode (`TG_STORAGE_DRY_RUN`).
- `"dry_run": false` is refused with 403 while `TG_STORAGE_DRY_RUN=1`,
  so nothing sent over the API can make a report-only deployment evict.
  `"dry_run": true` is always allowed.
- It answers at once; the sweep runs on the janitor's thread, after the
  one running now if there is one. Read its result from
  `GET /v1/storage` once `sweeping` and `pending` are both false.
- Requests made while a sweep is pending merge into it; if any of them
  asked for a real sweep, it is a real one. The reply's `dry_run` is
  the mode the pending sweep will run in.

Errors: 400 if the body is not an object or `dry_run` is not a
boolean, 403 as above.

### POST /v1/chats/resolve

Previews the chat behind an invite link or a public username, for the
web's join page. Read-only on the account.

    {"ref": "https://t.me/+AbCdEfGhIjKlMnOp"}

    200 {
      "outcome": "ok",
      "message": "",
      "ref": "https://t.me/+AbCdEfGhIjKlMnOp",
      "kind": "invite",
      "chat": {
        "chat_id": -1001234567890,
        "title": "GNU/Weeb",
        "type": "supergroup",
        "member_count": 1234,
        "is_member": false,
        "banned": false,
        "needs_approval": false,
        "paid": false,
        "username": null
      }
    }

`ref` is what the admin pasted. The daemon accepts only, after
trimming whitespace:

- invite links `https://t.me/+<hash>` and `https://t.me/joinchat/<hash>`
  (`<hash>`: 4 to 64 of `[A-Za-z0-9_-]`, not all digits: `t.me/+<digits>`
  is a phone number);
- usernames `@<name>`, `<name>` and `https://t.me/<name>` (`<name>`: 4 to
  32 of `[A-Za-z0-9_]`, starting with a letter, not ending with `_`).

The scheme is optional (`http` or `https`), the host may be `t.me`,
`telegram.me` or `telegram.dog` with an optional `www.`, and a trailing
`/`, a query string or a fragment is ignored. Anything else (other
hosts, message, sticker or folder links, `tg://` links, extra path
segments) is a 400, and nothing reaches Telegram.

In the reply, `ref` is the normalised form (`https://t.me/+<hash>` or
`@<name>`) and `kind` is `invite` or `username`. `chat`:

- `chat_id`: TDLib's chat id, or null when the account cannot see the
  chat before joining (a private group behind an invite link).
- `type`: `basic_group`, `supergroup`, `channel`, or `private` for a
  username that belongs to a user or a bot.
- `member_count`: as Telegram reports it; 0 if unknown.
- `is_member`, `banned`: the account's status in the chat, when it can
  see the chat; false otherwise.
- `needs_approval`: joining sends a join request that an administrator
  must approve (the invite link's setting, or the public chat's).
- `paid`: the invite link is a paid subscription; joining it from here
  fails.
- `username`: the chat's public username, if any.

`outcome` is `ok`, or one of the failures below, with `chat` null when
nothing was resolved. A `private` chat answers `not_a_group`.

### POST /v1/chats/join

Makes the account join the chat. Same body as `/v1/chats/resolve`.

    {"ref": "@gnuweeb"}

    200 {"outcome": "joined", "message": "joined", "ref": "@gnuweeb",
         "kind": "username", "chat": {...}}

The daemon resolves the chat again first (the preview may be stale),
answers `already_member`, `banned` or `not_a_group` without asking
Telegram to join, and otherwise calls `joinChatByInviteLink` (invite)
or `joinChat` (username). Joins are serialised: one at a time across
the whole API. `chat` is the preview, with `chat_id` and `is_member`
updated after a successful join; null when the chat could not be
resolved.

Each Telegram-level result is a 200 with an `outcome`; the web shows
`message` (Telegram's own text for `error`):

| `outcome` | Meaning |
|---|---|
| `ok` | (resolve only) the chat was found. |
| `joined` | The account is now a member. |
| `already_member` | It was a member already; nothing was sent. |
| `request_sent` | A join request was sent; an administrator must approve it. |
| `approval_required` | The chat's guard bot must approve the join through a Web App; finish it in a Telegram client. |
| `declined` | The chat's guard bot declined the join. |
| `not_a_group` | The username is a user or a bot. |
| `not_found` | No such username. |
| `link_invalid` | The invite link is invalid, expired or revoked. |
| `flood_wait` | Telegram rate-limited the account; `retry_after` is the wait in seconds (0 if Telegram gave none). |
| `banned` | The account is banned from the chat, or it is private and not accessible to it. |
| `too_many_chats` | The account is in too many chats to join another. |
| `error` | Anything else; `message` is Telegram's error. |

Errors: 400 if the body is not `{"ref": "<string>"}` or the string is
not an invite link or a username; 503 when TDLib is not authorized or
the daemon is shutting down; 504 when Telegram did not answer a step
within 15 seconds. A preview is up to three TDLib requests and a join
one more, so a client should wait about a minute.

Joining has consequences on the account (Telegram's rate limits, a
visible join in the chat): the web asks for a confirmation and audits
every attempt. Once joined, chat-membership tracking picks the chat
up, and its media start being downloaded.

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

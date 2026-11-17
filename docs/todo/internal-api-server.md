# internal-api-server

An HTTP API inside tgloggerd for the web: the server, its
authentication, file status and access recording.

## Why

The web cannot reach TDLib: only tgloggerd may hold the session. The
fetch, eviction and access items all need a channel from the web to
the daemon, and there is none.

## Scope

- `src/tgloggerd/InternalApi.{hpp,cpp}`: Boost.Beast server on
  `TG_API_LISTEN` with `TG_API_THREADS` threads, body cap 64 KiB,
  bearer authentication, JSON errors (storage-cache.md section 7).
- Endpoints `GET /v1/files/{id}` and `POST /v1/files/access`
  (section 10: `accessed_at = NOW()` keeping `updated_at`, only for
  rows more than an hour old).
- A way for later items to register handlers (fetch, storage) without
  editing the server core.
- `docs/internal-api.md`: the contract, for every endpoint including
  the ones later items add (they fill in their parts).
- `.env.example` and CMake changes; the daemon's DB pool sizing.
- Not in scope: fetch (file-fetch-on-demand), storage endpoints
  (storage-janitor), the web client (web-access-reporting).

## Design notes

- Beast and Asio are header-only; gwdiscord already compiles against
  them. Keep them inside InternalApi.cpp so the rest of the daemon does
  not include Boost.
- The bearer key is derived from `WEB_APP_KEY` with keyed BLAKE2b
  (libsodium, already linked) over `tgloggerd:internal-api:v1`; compare
  with `sodium_memcmp`. Refuse to start the API when `WEB_APP_KEY` is
  missing.
- Empty `TG_API_LISTEN` means the API is off and nothing listens.
- Shut down cleanly with the daemon (no thread left running).

## Depends on

- `files-cache-schema`

## Done when

- tgloggerd builds; with `TG_API_LISTEN` set, curl from a container on
  tgnet gets 401 without the bearer, file status with it, and an access
  batch moves `accessed_at` but not `updated_at` (checked at cutover,
  or against a throwaway database in the worktree).

# files-cache-schema

Migration 000032: the columns of `telegram_files` that turn the store
into a cache, and the web brought in line with them.

## Why

Every other storage item needs somewhere to keep what it knows: when a
file was last used, its stable Telegram identity, that its bytes were
never fetched, and that it has the same bytes as another row.

## Scope

- `migrations/000032_*.up.sql` and `.down.sql`, as storage-cache.md
  section 3 describes: `tg_unique_id` (UNIQUE), `sha256` NULL-able,
  `accessed_at` set to `created_at` for existing rows, the key
  `idx_files_evict (on_disk, accessed_at)`, `alias_of` with its foreign
  key, and the new `on_disk` comment.
- The daemon's existing writers keep working unchanged: `upsertFile`
  still writes `sha256`, and its ON DUPLICATE KEY branch also sets
  `accessed_at = NOW()` (storage-cache.md section 9).
- The web, by the rule that a schema change updates the web:
  `dao::browse::getFile` selects `alias_of` and `accessed_at`, and the
  files search (`/files`, `/v1/search/files`) gains a sortable "Last
  access" column; `web/docs/search-api.md` lists it.
- Not in scope: anything that reads `tg_unique_id` or `alias_of` for
  real (media-record-without-download, file-fetch-on-demand,
  web-fetch-unstored-media).

## Design notes

- Additive: the table holds 980k rows of real data. No row is deleted
  or rewritten beyond the `accessed_at` backfill, which must keep
  `updated_at` (`SET accessed_at = created_at, updated_at = updated_at`).
- Changing `sha256` to NULL-able rebuilds the table; on 980k rows that
  takes minutes, during which the daemon is stopped (storage-cutover).
- The down migration must refuse, or fail clearly, when rows with
  `sha256` NULL exist, since `NOT NULL` cannot be restored over them.
- Test on a throwaway `mysql:9.7.0` container loaded with the schema
  from migrations 000001-000031 and some rows, never on the production
  server.

## Depends on

Nothing.

## Done when

- `migrate up` then `migrate down 1` then `migrate up` succeed on a
  throwaway database with existing rows, keeping their data.
- tgloggerd, discordd and tgloggerd_web build.
- The files search shows and sorts by "Last access".

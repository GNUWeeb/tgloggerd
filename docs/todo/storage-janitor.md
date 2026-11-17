# storage-janitor

Evict files by retention and by the total cap, with a dry run, and
report the store's state over the internal API.

## Why

These are the first two things asked for: a cap on the store's size
with least-recently-accessed eviction, and a retention period, both
configurable.

## Scope

- The StorageJanitor thread (storage-cache.md section 9):
  `TG_STORAGE_MAX_BYTES`, `TG_STORAGE_RETENTION_DAYS`,
  `TG_STORAGE_SWEEP_SECS`, `TG_STORAGE_DRY_RUN`, the guarded per-row
  eviction, paced batches, logging.
- `GET /v1/storage` and `POST /v1/storage/sweep` on the internal API,
  and their part of `docs/internal-api.md`.
- `.env.example` entries.
- Not in scope: seeding `accessed_at` (seed-file-access) and turning
  it on in production (storage-cutover).

## Design notes

- The blob path is built exactly as `storeDownloadedFile` builds it;
  share that function rather than copying the layout.
- Unlink only after the guarded UPDATE changed one row; a missing blob
  is not an error.
- Cap accounting uses `SUM(file_size)`, the logical size; btrfs
  compression makes the real usage lower. Orphan blobs are not counted
  (orphan-blob-audit).
- Stop cleanly with the daemon, mid-batch.
- `TG_STORAGE_DRY_RUN` defaults to 1: nothing is evicted until it is
  set to 0 (decisions/2026-11-18-janitor-dry-run-default.md).

## Depends on

- `internal-api-server`

## Done when

- tgloggerd builds.
- On a throwaway database and a scratch store directory: a dry run
  reports and changes nothing; a real run evicts the expected rows and
  only their blobs, and leaves the store at most 95% of a small cap.

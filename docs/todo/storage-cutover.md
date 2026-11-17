# storage-cutover

Deploy the storage cache: migrate, seed, run the janitor as a dry run,
and turn real eviction on only after the project owner agrees.

## Why

The migration rebuilds a 980k-row table, the seeding needs root, and
the first real sweep deletes hundreds of GiB of archived bytes. Each
step needs care, and the last one needs the owner's go.

## Scope

- Build tgloggerd, discordd and tgloggerd_web.
- Stop tgloggerd, tgloggerd-discordd and tgloggerd-web; start
  tgloggerd so it applies 000032; run `scripts/seed_file_access.sql`
  as root, first as a dry run, then with `@seed_apply = 1`.
- Set the `.env` values of storage-cache.md section 12 with
  `TG_STORAGE_DRY_RUN=1`; recreate the containers
  (`--force-recreate`, since env changes need it).
- Run every "at cutover" check of the other items, and check that
  `TDLib::request()` (getOption "version") from a thread other than
  the loop gets its answer after login; its own item is closed.
- Report the dry run's numbers (files and GiB for retention and for
  the cap) to the owner and wait for a go; then set
  `TG_STORAGE_DRY_RUN=0` and recreate.
- Watch the next days' growth and disk use.

## Design notes

- The coordinator does this, not a sub agent: it deploys and changes
  the production configuration.
- Follow the data-preserving cutover procedure: stop, migrate, verify,
  start.
- Checks of items already done, whose files are gone:
  - `chat-membership-tracking`: with `TG_LOG_LEVEL=debug`, the log
    has `membership: chat <id>: ... -> member` and `-> not a member`
    lines; a spot check of a few chats against the Telegram client
    agrees.
  - `seed-file-access`: `SELECT @@global.time_zone,
    @@system_time_zone` is UTC (web_audit times are written in the
    server's zone); the dry run's "files raised" and "older after"
    figures are plausible; after applying, a second run within a
    minute raises 0 files (a later one re-raises only the current
    avatars, by design) and `scripts/seed_file_access_report.sql`
    agrees with the applied run.
  - `media-record-without-download`: new `telegram_files` rows for
    media from non-member channels have `on_disk = 0`, `sha256` NULL
    and `tg_unique_id` set, and the log has "Recorded file without
    its bytes" lines; media from member chats is still stored, with
    `tg_unique_id` set; no file at or over `TG_MAX_STORE_FILE_SIZE`
    with a known size is downloaded (TDLib's temp dir stays small);
    a metadata-only file later posted in a member chat is downloaded
    and fills in its row (same id, `sha256` set, `on_disk = 1`).
  - `internal-api-server`: with `TG_API_LISTEN=0.0.0.0:8091`, the log
    has `api: internal API listening on 0.0.0.0:8091`; curl from a
    container on tgnet to `http://10.0.88.5:8091/v1/files/<id>` gets
    401 without the bearer (docs/internal-api.md) and the file's
    status with it; `POST /v1/files/access` with an id last accessed
    over an hour ago moves its `accessed_at` but not its `updated_at`.
  - `storage-janitor`: the log has `storage: janitor started:` with
    the configured cap, retention and "(dry run: reporting only)",
    then a `storage: dry-run sweep done` line a minute later;
    `GET /v1/storage` shows the same numbers in `last_sweep`, and
    `POST /v1/storage/sweep {"dry_run": false}` is refused with 403
    while `TG_STORAGE_DRY_RUN=1`. Shutting the daemon down during a
    sweep logs `stopped at shutdown` and exits promptly. After
    `TG_STORAGE_DRY_RUN=0`, once the first real sweep is done,
    `bytes` and `last_sweep.bytes_after` are at most
    `low_watermark`, and a few evicted files (`on_disk = 0`) have no
    blob left while their `updated_at` is unchanged.
  - `web-access-reporting`: with
    `WEB_TGLOGGERD_API=http://10.0.88.5:8091`, the web's log has
    `tgloggerd-api: using http://10.0.88.5:8091`; viewing a file
    whose `accessed_at` is over an hour old moves it within about a minute, `updated_at`
    unchanged; `/files/<token>` answers `Cache-Control: public,
    max-age=604800`; with tgloggerd stopped, stored files are still
    served and the web logs one "access report failed" line.

## Depends on

- `file-fetch-on-demand`
- `web-fetch-unstored-media`

## Done when

- All the checks pass in production, the owner has agreed to the
  eviction, the store is at most 95% of 500 GiB, and the daily growth
  has dropped.

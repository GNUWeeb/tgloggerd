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

## Depends on

- `media-record-without-download`
- `internal-api-server`
- `file-fetch-on-demand`
- `storage-janitor`
- `web-access-reporting`
- `web-fetch-unstored-media`

## Done when

- All the checks pass in production, the owner has agreed to the
  eviction, the store is at most 95% of 500 GiB, and the daily growth
  has dropped.

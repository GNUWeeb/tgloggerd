# media-record-without-download

Record media from non-member chats, and media over the per-file cap,
as metadata only instead of downloading it; store `tg_unique_id` for
every file.

## Why

Most of the daily growth is media from channels the account is not in,
reached by the backfiller, reply resolution and edits. And every file
over `TG_MAX_STORE_FILE_SIZE` is still downloaded in full only to hash
it (TDLib's temp dir reached 13 GiB during one sweep).

## Scope

- The decision in `maybe_download_message_file`,
  `maybe_store_link_preview` and `maybe_download_group_photo`
  (storage-cache.md section 6), behind `TG_SKIP_NONMEMBER_MEDIA`
  (default on), documented in `.env.example`.
- `DB::upsertFileMeta`, upserting on `tg_unique_id` with `on_disk = 0`
  and `sha256` NULL, and the link to the message through the same path
  the index hit uses today; the in-memory index learns the row.
- `tg_unique_id` written by the normal store path (`upsertFile`) too.
- Not in scope: fetching such files later (file-fetch-on-demand) and
  showing them (web-fetch-unstored-media). User profile photos are
  unchanged.

## Design notes

- `file.size_` may be 0 when unknown; then the cap check cannot skip
  the download, and the existing post-download check still applies.
- A file whose `tg_unique_id` is already in the table, stored or not,
  must link to that row and never create a second one.
- Messages keep being logged; only the bytes are skipped (the
  2026-11-18 decision).

## Depends on

Nothing.

## Done when

- tgloggerd builds.
- At cutover: new media rows from non-member channels have
  `on_disk = 0`, `sha256` NULL and `tg_unique_id` set; member chats
  still store their media; no over-cap file is downloaded.

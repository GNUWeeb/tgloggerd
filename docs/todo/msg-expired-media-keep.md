# msg-expired-media-keep

Keep the stored content of a self-destructing photo, video, video note
or voice note when a later copy of the message arrives expired.

## Why

The message upserts take the content columns from any copy whose
`edit_date` is not older than the stored one. Once self-destructing
media is opened or times out, TDLib reports it as `messageExpired*`;
ingesting that copy again (reply resolution, backfill, relink re-fetch)
rewrites the stored `photo` row to `unknown` with no caption. See
message-types.md section 2.1.

## Scope

- Group and private message upserts, and the edit re-fetch path: an
  expired copy never replaces stored content, never creates an edit
  snapshot, and leaves `file_id` alone.
- A message first seen already expired keeps being stored as today
  (`unknown`); the `expired` kind comes with `msg-classify`.
- Not in scope: repairing rows already overwritten (their original
  kind is gone; `msg-repair-old-rows` re-reads what TDLib still has).

## Design notes

Decision
[2026-11-18-msg-expired-media-keep](../design/decisions/2026-11-18-msg-expired-media-keep.md).
`is_server_churn_content` already keeps `updateMessageContent` from
recording a version; the gap is the full-message upsert.

## Depends on

Nothing.

## Done when

- A test stores a photo message, ingests the same message as
  `messageExpiredPhoto` with the same `edit_date`, and the row still
  says `photo` with its caption and file; it fails before the change.

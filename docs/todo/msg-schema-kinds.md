# msg-schema-kinds

Add the new content kinds, the `tdlib_type` column and the
`content_extra` column to the message and edit tables, and the new
file kinds to `telegram_files`.

## Why

Every other item in the "Message types" section writes into these
columns; without them the daemon has nowhere to put what it reads.

## Scope

- One additive migration (number agreed with the main session before
  use): append `video_note, location, venue, contact, dice, game, poll,
  checklist, story, paid_media, invoice, giveaway, call, rich, widget,
  expired, unsupported` to `content_type` of
  `telegram_group_messages`, `telegram_private_messages` and both
  `*_message_edits` tables; add `tdlib_type VARCHAR(64) NULL` and
  `content_extra JSON NULL` to all four; append `video_note, thumbnail`
  to `telegram_files.file_type`.
- The down migration drops the columns and restores the ENUMs after
  mapping new values back to `unknown`.
- `models::MessageContentType` and its string mappings, the
  `MessageContent` model, and the DAO reads that parse the ENUM.
- Not in scope: writing the new values (`msg-classify`).

## Design notes

message-types.md sections 4.1 and 4.2; decisions
[2026-11-18-msg-content-kinds](../design/decisions/2026-11-18-msg-content-kinds.md)
and
[2026-11-18-msg-content-extra](../design/decisions/2026-11-18-msg-content-extra.md).
Check with `ALGORITHM=INSTANT` on a copy of the largest table that
every statement is instant; a statement that is not must be split or
reported before it reaches production.

## Depends on

Nothing.

## Done when

- The migration applies and rolls back on a copy of the schema, each
  ALTER runs with `ALGORITHM=INSTANT`, and the daemon and the web build
  and run against the migrated schema with no behaviour change.

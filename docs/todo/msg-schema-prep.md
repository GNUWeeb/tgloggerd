# msg-schema-prep

Prepare the schema the lossy message kinds need before any of them is
worked on: new file roles, a column for link preview extras, and a
table of custom emoji.

## Why

Several kinds are stored without what the web needs to draw them
(message-types.md section 3): the media inside a rich message, a
gift's sticker, a chat background's wallpaper, a link preview's video,
and the stickers behind custom emoji ids. Each needs a place in the
schema. Doing the migrations first, together, keeps them additive and
in one reviewed step, and lets the store items and the web items that
follow work in parallel against a known schema.

## Scope

Daemon migrations (next free numbers; renumber at merge time if
another migration lands first):

- 000045: `telegram_message_files.role` ENUM append `rich`, `gift`,
  `background`, `preview_media` (`ALGORITHM=INSTANT`); and
  `telegram_link_previews` ADD COLUMN `extra JSON NULL` (instant: that
  table has no FULLTEXT index).
- 000046: CREATE TABLE `telegram_custom_emoji` (`custom_emoji_id`
  BIGINT primary key, `emoji` VARCHAR(16), `set_id` BIGINT, `format`
  ENUM('webp','tgs','webm'), `needs_repainting` TINYINT(1), `file_id`
  BIGINT UNSIGNED NULL referencing `telegram_files` ON DELETE SET NULL,
  `created_at`, `updated_at`).
- Each with its down file. The down of 000045 first deletes the
  `telegram_message_files` rows that use a new role, since the ENUM
  cannot shrink while rows hold the new values.
- Go (webapi): the test database schema applies both (as it does for
  pending migrations); nothing reads the new roles or table yet.
- Design: message-types.md section 4.3 lists the new roles, section 4.8
  the `extra` column, and a new section describes the custom emoji
  table and how it is filled.
- Left to other items: writing the new roles (`msg-rich-store`,
  `msg-gifts`, `msg-chat-backgrounds`, `msg-link-preview-media`) and
  filling the table (`msg-custom-emoji-store`). No web migration is
  needed (web's next free number stays 000014).

## Design notes

The ENUM append is instant because `role` is only part of the unique
key, which an append does not change; 000034 showed that an indexed
ENUM is not always instant, so this is confirmed on a copy first (an
`ALGORITHM=INSTANT` that cannot be instant fails instead of copying the
table). Additive only: the production data is kept and migrates on
restart. The table's shape follows the TDLib session's proposal; a
message effect's sticker (`meta.effect_id`) may share it or need a
sibling table, which `msg-custom-emoji-store` decides.

## Depends on

Nothing.

## Done when

- On a throwaway MySQL with migrations through 000044 and sample rows
  in `telegram_message_files` and `telegram_link_previews`, 000045 and
  000046 go up, down and up again, and 000045's ALTERs succeed with
  `ALGORITHM=INSTANT`.
- The daemon and webapi build, and the webapi tests pass on the new
  schema.
- message-types.md describes the new roles, the column and the table.

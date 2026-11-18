# msg-message-files

Record every file of a message, not just the main one, and download the
ones the logger skips today (video notes, paid media, covers, live
photo videos, poll, game and invoice media, chat photo changes).

## Why

`file_id` holds one file per message; video notes are not linked at
all, and the other files listed above are lost.

## Scope

- Additive migration: `telegram_message_files` (message-types.md
  section 4.3).
- `message_content_file` becomes a list of (role, position, file); the
  main one still fills `file_id`.
- The file index, store holds, relink and janitor reference checks
  learn the new table.
- Downloads follow the storage cache rules (non-member chats keep the
  file id only).

## Design notes

Decision
[2026-11-18-msg-message-files](../design/decisions/2026-11-18-msg-message-files.md).
`storage-cache.md` for the download and eviction rules.

## Depends on

Nothing.

## Done when

- A video note, a paid media message with several items and a photo
  with a live video each get one row per file; evicting a file leaves
  the rows in place.

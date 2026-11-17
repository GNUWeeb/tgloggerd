# Decision log: 2026-11-18, A table for every file of a message

Decided by the TDLib session on 2026-11-18 and approved by the
project owner the same day, for
[../message-types.md](../message-types.md) section 4.3:

- A new table `telegram_message_files` lists every file of a message
  with its position and role (main, cover, live video, paid item,
  preview, poll, game, invoice, chat photo, thumbnail), with one
  nullable foreign key per message table, as `telegram_link_previews`
  does.
- The existing `file_id` column stays and keeps meaning the main file.
- Reason: paid media, live photos, video covers and polls with media
  carry several files, and one column cannot hold them. Keeping
  `file_id` means the web, the forwarders and the storage janitor's
  reference checks need no change until they want the extra files.

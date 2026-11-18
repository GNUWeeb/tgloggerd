# Decision log: 2026-11-18, The message files table holds only the other files

Decided by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.3. It changes
[2026-11-18-msg-message-files](2026-11-18-msg-message-files.md), which
had the table list every file of a message, the main one included:

- `telegram_message_files` holds the files of a message beyond its main
  one. The main file stays in the message row's `file_id` only, and
  the table has no `main` role.
- Reason: listing the main files too would mean copying about a
  million existing links into the table in a migration that runs before
  the daemon starts, and keeping two copies of every main link in step
  afterwards. Readers that want all files read `file_id` and the table,
  which is what they would do for the link preview image anyway.

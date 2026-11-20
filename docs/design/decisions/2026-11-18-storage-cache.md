# Decision log: 2026-11-18, Storage cache

Decided with the project owner on 2026-11-18, for
[../storage-cache.md](../storage-cache.md):

- The file store becomes a cache in front of Telegram: a total cap
  with least-recently-accessed eviction, a retention period, fetch on
  demand (with byte ranges for big files), and no downloads from chats
  the account is not a member of.
- A total cap, set per deployment, evicting down to 95% of it.
- Retention: 90 days by default. Existing files start from their
  created time, raised to the latest recorded sign of use (web audit
  views, newest referencing message, Discord forwards).
- Messages from chats the account is not a member of are still logged
  as text; only their media is recorded without its bytes.
- Eviction keeps the row and clears `on_disk`; rows are never deleted.
- The daemon gets an internal HTTP API, a curated set of operations
  rather than a TDLib passthrough. Bytes are not proxied through it;
  the web serves them from the shared mount.
- Access is recorded in the database by the daemon, batched by the
  web. File access times (relatime) were considered and rejected: the
  store is mounted noatime, the filesystem is shared with other
  projects, and on btrfs atime updates turn reads into metadata
  writes.
- The first eviction runs as a dry run; real eviction starts only
  after the project owner has seen its numbers and agreed.

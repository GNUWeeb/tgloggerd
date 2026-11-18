# Decision log: 2026-11-18, Rebuild the message tables once, without FULLTEXT

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.1:

- Migration 000034 drops the FULLTEXT index of each message table,
  appends the ENUM members and adds the new columns in place, and
  builds the FULLTEXT index again. Full-text search fails while the
  index is missing, and writes to the table wait while it is rebuilt.
- The same migration adds every column the design puts on the two
  message tables, including those of later items (topic, pinned state,
  metadata, interaction totals), unused until those items write them.
  Later items do not ALTER the message tables again.
- Reason: with a FULLTEXT index, InnoDB changes these tables only by
  copying them, maintaining the full-text index row by row. On a
  synthetic table of seven million rows, about the archive's size,
  that took 10.4 hours, all of it daemon downtime, since migrations
  run before the daemon starts. Without the index, the ENUM append took
  a second and adding the columns 2 minutes online; the whole
  migration, building the index again included, took under 10 minutes
  on the same table. Doing it once avoids repeating that window
  for every later item. Keeping the new data in side tables would have
  avoided the window entirely, but new kinds would then be 'unknown' in
  content_type for good, and every filter would need a join.

# Decision log: 2026-11-18, TDLib JSON only for unmapped classes

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 6, question 1:

- The raw TDLib JSON of a message's content is stored only for classes
  without a curated `content_extra` mapping, not for every message.
- Reason: the curated detail covers what a reader needs for the
  classes that matter, and raw JSON for every message would add about
  one to two kilobytes to each row of tables that already hold
  millions of rows.

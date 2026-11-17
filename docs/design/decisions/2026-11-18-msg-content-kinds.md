# Decision log: 2026-11-18, Content kinds and the exact TDLib class

Proposed by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.1; takes effect
when the plan is approved:

- The `content_type` ENUM of the message and edit tables keeps its
  coarse kinds and gains new ones appended at the end, one per kind a
  reader sees differently (video note, location, poll, ...). Search and
  the web filter on it as they do today.
- A new column `tdlib_type` records the exact TDLib class of every new
  row. `unknown` then means only "a class newer than the code", and
  even that row says which class it was.
- Reason: one column cannot serve both. A per-class ENUM would need a
  migration for every TDLib release and make the coarse filters long
  lists; a coarse ENUM alone is what lost the identity of tens of
  thousands of rows. Appending ENUM members and adding a nullable
  column are instant on MySQL 8.0.29 and later, so neither touches the
  large tables' data.

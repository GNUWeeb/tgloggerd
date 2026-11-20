# Decision log: 2026-11-18, The repair pass is opt-in and only re-reads

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.9:

- The repair of old rows runs only when `TG_REPAIR_MESSAGES_ON_START`
  is set, with a dry-run value (`dry`) that counts the rows first, like
  the existing media relink flag. It re-reads both the `unknown` rows
  and the service rows without detail.
- The planned local re-map of rows that hold only TDLib's JSON for a
  class with a curated mapping is dropped.
- Reason: the pass is long (on the order of half a million old rows at
  ten re-reads a second, about half a day), and rows TDLib can no
  longer return are tried again on every start while it is on, so the
  owner chooses when it runs. The re-map would only find rows written
  between classification and the curated mappings, and every message
  types change goes live at the same restart, so it would find none.

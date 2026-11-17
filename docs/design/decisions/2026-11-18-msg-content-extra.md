# Decision log: 2026-11-18, Structured content detail in one JSON column

Proposed by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.2; takes effect
when the plan is approved:

- The detail of each content class (coordinates, contact, sticker
  emoji, media size and duration, a service action's users and ids)
  goes into one nullable `content_extra` JSON column with a curated,
  stable set of keys per class, not into a column or table per class.
- Classes without a curated mapping store TDLib's own JSON of the
  content under the key `tdlib`, without local file paths.
- `content_extra` is part of the edit snapshot.
- Files are never referenced from the JSON; they go through
  `telegram_message_files`, so foreign keys keep working.
- Reason: about seventy classes carry detail, most of them rare. A
  table per class would be dozens of tables written a handful of times;
  columns would widen the hottest tables for values that are almost
  always NULL. What is queried often (kind, topic, pinned, polls) gets
  real columns or tables instead.

# Decision log: 2026-11-18, Expired media never replaces stored content

Proposed by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 2.1; takes effect
when the plan is approved:

- When an ingested copy of a message is `messageExpired*` and a row
  already exists, the stored content columns stay as they are; only
  `tdlib_type` (once it exists) and the expiry are recorded. A message
  first seen already expired is stored with the `expired` kind.
- Reason: the archive's purpose is to keep what was shown. Expiry is
  Telegram removing the content from clients, the same as deletion,
  and deleted messages already keep the content they were deleted
  with.

# Decision log: 2026-11-18, Expired media never replaces stored content

Decided by the TDLib session on 2026-11-18 and approved by the
project owner the same day, for
[../message-types.md](../message-types.md) section 2.1:

- When an ingested copy of a message is `messageExpired*` and a row
  already exists, the stored row stays as it is, `tdlib_type` included,
  so that it keeps describing the content it holds. A message first
  seen already expired is stored with the `expired` kind.
- Reason: the archive's purpose is to keep what was shown. Expiry is
  Telegram removing the content from clients, the same as deletion,
  and deleted messages already keep the content they were deleted
  with.

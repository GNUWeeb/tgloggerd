# msg-interaction

Store each message's latest views, forwards, reply count and reaction
totals.

## Why

They show how a post was received, which the archive does not keep.

## Scope

- Additive migration: `view_count`, `forward_count`, `reply_count`,
  `reactions JSON` on both message tables.
- `updateMessageInteractionInfo` and `updateMessageReactions`,
  coalesced per message.

## Design notes

message-types.md section 4.7; decision
[2026-11-18-msg-interaction-latest-totals](../design/decisions/2026-11-18-msg-interaction-latest-totals.md).
Measure the update rate on the live account for a day (log only)
before choosing the coalescing interval.

## Depends on

Nothing.

## Done when

- The measured rate is recorded in the design; counts update at most
  once per message per interval.

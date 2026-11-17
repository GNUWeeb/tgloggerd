# msg-interaction

Store each message's latest views, forwards, reply count and reaction
totals (optional, waiting for a decision).

## Why

They show how a post was received, which the archive does not keep.

## Scope

- Additive migration: `view_count`, `forward_count`, `reply_count`,
  `reactions JSON` on both message tables.
- `updateMessageInteractionInfo` and `updateMessageReactions`,
  coalesced per message.

## Design notes

message-types.md section 4.7 and open question 2. Measure the update
rate on the live account for a day (log only) before choosing the
coalescing interval.

## Depends on

Nothing.

## Done when

- The measured rate is recorded in the design; counts update at most
  once per message per interval.

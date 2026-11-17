# Decision log: 2026-11-18, Views and reactions as latest totals

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.7 and section 6,
question 2:

- Views, forwards, reply count and reaction totals are stored as the
  latest values on the message row, updated in place and coalesced per
  message. Their history is not kept, nor who reacted with what.
- Reason: totals show how a post was received; a history of counters
  on busy channels would cost a write per change for little reading
  value.

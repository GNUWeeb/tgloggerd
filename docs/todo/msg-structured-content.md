# msg-structured-content

Store locations, live locations, venues, contacts, dice, games, shared
stories, invoices, checklists, giveaways and calls as structured
detail.

## Why

These classes are stored as `unknown` today with nothing that says
what they showed (message-types.md section 3.2).

## Scope

- Curated `content_extra` for each class of section 3.2 except polls
  (`msg-polls`), rich messages and widgets (kept as TDLib JSON).
- A live location's later positions arrive by `updateMessageContent`;
  record them as versions (remove `messageLiveLocation` and
  `messageLocation` from `is_server_churn_content`), coalesced to at
  most one version per message per minute.
- A shared story keeps its reference only (decision
  [2026-11-18-msg-stories-reference-only](../design/decisions/2026-11-18-msg-stories-reference-only.md)).

## Design notes

message-types.md sections 3.2 and 4.2.

## Depends on

Nothing.

## Done when

- Tests cover each class; a moving live location produces versions at
  the coalescing rate and no faster.

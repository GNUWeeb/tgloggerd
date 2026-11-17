# msg-meta

Store the message-level fields that are lost today: pinned state, via
bot, reply quote, story replies, forward source, reply markup, effect,
self-destruct timer, paid stars, fact check, import and restriction
info.

## Why

message-types.md section 3.6.

## Scope

- Additive migration: `is_pinned` and `meta JSON` on both message
  tables.
- `updateMessageIsPinned` and `updateMessageFactCheck` handled.
- Not in scope: views and reactions (`msg-interaction`).

## Design notes

message-types.md section 4.6.

## Depends on

Nothing.

## Done when

- Tests cover each `meta` key; pinning and unpinning a message updates
  `is_pinned`.

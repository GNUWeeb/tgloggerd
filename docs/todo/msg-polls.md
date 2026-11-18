# msg-polls

Store polls and quizzes with their options and keep their latest
results current.

## Why

Polls are stored as `unknown` today, and `updatePoll` is ignored.

## Scope

- Additive migration: `telegram_polls`, `telegram_poll_options`
  (message-types.md section 4.4).
- Ingest on new messages and `updatePoll`; poll media through
  `telegram_message_files`.
- `messagePollOptionAdded` and `messagePollOptionDeleted` keep the
  options table current.

## Design notes

Decision [2026-11-18-msg-polls](../design/decisions/2026-11-18-msg-polls.md).

## Depends on

`msg-message-files`

## Done when

- Tests: a poll message creates both rows; an `updatePoll` updates the
  counts without an edit snapshot; a quiz keeps its correct option and
  explanation.

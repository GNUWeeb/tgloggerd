# msg-topics

Record which forum topic or thread each group message belongs to, and
the topics themselves.

## Why

Forum groups mix many conversations in one chat; without the topic a
reader cannot tell them apart.

## Scope

- Additive migration: `telegram_forum_topics`. The `topic_kind` and
  `topic_id` columns and their index on `telegram_group_messages`
  exist since migration 000034.
- Fill from `message.topic_id`, `updateForumTopicInfo`, one
  `getForumTopics` per forum, and the topic service messages.

## Design notes

message-types.md section 4.5. Do not ALTER the message tables again
(decision 2026-11-18-msg-message-table-ddl).

## Depends on

Nothing.

## Done when

- Messages of a forum group carry their topic; the topic table lists
  each topic with its current name and state.

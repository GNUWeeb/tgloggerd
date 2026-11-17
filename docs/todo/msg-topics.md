# msg-topics

Record which forum topic or thread each group message belongs to, and
the topics themselves.

## Why

Forum groups mix many conversations in one chat; without the topic a
reader cannot tell them apart.

## Scope

- Additive migration: `topic_kind`, `topic_id` on
  `telegram_group_messages` with an index on (`chat_id`, `topic_kind`,
  `topic_id`); `telegram_forum_topics`.
- Fill from `message.topic_id`, `updateForumTopicInfo`, one
  `getForumTopics` per forum, and the topic service messages.

## Design notes

message-types.md section 4.5. The index must be built online; measure
on a copy of the table first.

## Depends on

Nothing.

## Done when

- Messages of a forum group carry their topic; the topic table lists
  each topic with its current name and state.

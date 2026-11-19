# text-entities

Show Telegram's text formatting in the web: links, mentions, hashtags,
bold, italic, underline, strikethrough, spoilers, code, quotes and the
rest, in the chat, the message page, edits, polls and checklists.

## Why

The API now sends each text's entities (TextEntity in
webapi/openapi.yaml, web/docs/message-kinds.md "Text formatting"), but
the React application still shows every message as plain text and
links cannot be clicked.

## Scope

- React: a safe renderer (no innerHTML) of the `entities` of
  ChatMessage, MessageFull and each edit, PollDetail's
  `question_entities` and `explanation_entities`, and each checklist
  task's `entities`; offsets are UTF-16 code units (JavaScript string
  indices). Links with rel=noopener, a text_url's scheme checked before
  it is linked; mentions to user pages when `user_id` is set, plain
  text otherwise; hashtags to a search in that chat; click-to-reveal
  spoilers; monospace code and pre blocks; quotes; unknown types as
  plain text. In the chat, the message page, edits and polls.
- The app's tests and stand-in gain the new required fields.

## Depends on

Nothing.

## Done when

- A message with a link, a mention, a hashtag and bold text shows each
  formatted and clickable in the chat and on its message page.

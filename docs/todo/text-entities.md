# text-entities

Show Telegram's text formatting in the web: links, mentions, hashtags,
bold, italic, underline, strikethrough, spoilers, code, quotes and the
rest, in the chat, the message page, edits, polls and checklists.

## Why

The daemon stores each message's text entities (the entities column,
per edit, and for polls and checklist tasks), but the API does not
send them, so every message shows as plain text and links cannot be
clicked.

## Scope

- Go: a TextEntity schema {offset, length, type, url?, user_id?,
  language?, custom_emoji_id? (a string)}, offsets in UTF-16 code
  units as stored; `entities` ([] for plain text) on ChatMessage,
  MessageFull and each edit; PollDetail question_entities and
  explanation_entities; checklist tasks' entities. A mention_name's
  user_id only when the viewer may see that user (else null); a
  mention by @username resolved to the user's id when the archive has
  that username and the viewer may see the user. Service messages
  whose text is rebuilt for the viewer get [].
- React: a safe renderer (no innerHTML): links with rel=noopener,
  mentions to user pages when known, hashtags to a search in that
  chat, click-to-reveal spoilers, monospace code and pre blocks,
  quotes; in the chat, the message page, edits and polls.

## Depends on

Nothing.

## Done when

- A message with a link, a mention, a hashtag and bold text shows each
  formatted and clickable in the chat and on its message page.

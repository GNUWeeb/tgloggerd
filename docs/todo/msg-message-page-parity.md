# msg-message-page-parity

Give the message page what the chat already shows: the link preview,
and the forum topic's name and colour instead of its number.

## Why

The message page now draws media and albums as the chat does, but
`MessageFull` has no `link_preview`, so a message with a preview shows
only its text there; and the Details card shows the forum topic as
"topic <id>", since the page has no topic list.

## Scope

- Go (webapi): `MessageFull.link_preview` (as ChatMessage has it) and
  `MessageFull.topic` (the ChatTopic, or null); OpenAPI and web/docs
  follow.
- React (frontend): the message page draws the link preview with the
  chat's component, and the topic as a link with its name and colour
  dot.

## Design notes

The link preview component lives in routes/chat.tsx; move it to a
shared module rather than duplicating it.

## Depends on

Nothing.

## Done when

- The page of a message with a link preview shows the preview, and the
  page of a forum message shows its topic's name linked to the topic.
- Go and React tests cover both.

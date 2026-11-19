# msg-message-page-parity

Give the message page what the chat already shows: the link preview,
and the forum topic's name and colour instead of its number.

## Why

The message page now draws media and albums as the chat does, but a
message with a preview shows only its text there, and the Details card
shows the forum topic as "topic <id>". The API already sends both:
`MessageFull.link_preview` (the LinkPreview schema ChatMessage uses)
and `MessageFull.topic` (the ChatTopic `topic_id` names, or null); the
app does not draw them yet.

## Scope

- React (frontend): the message page draws the link preview with the
  chat's component, and the topic as a link with its name and colour
  dot.
- Once it does, make `link_preview` and `topic` required in
  MessageFull (webapi/openapi.yaml, regenerated schema.d.ts): the Go
  service always sends them, and they are optional only so the app's
  contract test kept passing until this part landed.

## Design notes

The link preview component lives in routes/chat.tsx; move it to a
shared module rather than duplicating it.

## Depends on

Nothing.

## Done when

- The page of a message with a link preview shows the preview, and the
  page of a forum message shows its topic's name linked to the topic.
- React tests cover both, and both fields are required in the spec.

# msg-custom-emoji-web

Draw custom emoji as their stickers wherever the web shows one: in
message text, reactions, forum topic icons and message effects.

## Why

A custom emoji reaction shows as a diamond, a custom emoji in a
message's text as its fallback emoji, and a forum topic's custom icon
not at all, because the API sends only ids. The daemon resolves ids
to stickers on demand (`POST /v1/custom-emoji` and, for effects,
`POST /v1/message-effects`, docs/internal-api.md; the Go client has
`CustomEmojiStickers` and `MessageEffects`).

## Scope

- React (frontend): draw them inline at text size (WebP as an image,
  WebM as a muted looping video, TGS with the chat's Lottie player),
  with the fallback emoji as alt text; in reactions, topic tags and
  the topic strip; a message's effect as a small mark on the bubble.
  The API sends each id with what is known of it (webapi/openapi.yaml:
  CustomEmoji, MessageEffect; web/docs/message-kinds.md, "Custom
  emoji"): `custom_emoji` on text entities and reactions,
  `icon_custom_emoji_id` and `icon_custom_emoji` on ChatTopic and
  ServiceTopic, `effect_id` and `effect` on every message. Each is
  null while the id is not known, and its `url` is null when there is
  nothing to draw: the fallback emoji is shown then.
- Done in Go (webapi): the page's ids are resolved in one batch from
  the tables, and the daemon is asked for the ones they lack, never
  waited on beyond a short budget. Tests cover it.
- Left out: animating effects over the chat.

## Design notes

Custom emoji that need repainting (`needs_repainting`) take the text
colour. Lottie custom emoji can be many on one page; draw them still
until hovered or visible, and measure the chat's frame rate with a
page full of them. The text renderer is the text-entities item's (web
interface section, treated as landed). An id the daemon has not
resolved yet is null on that page and filled in on a later one, so a
chat opened for the first time may show fallbacks that a reload
replaces.

## Depends on

Nothing.

## Done when

- A message with a custom emoji in its text, a custom emoji reaction
  and a forum topic with a custom icon show the stickers in the chat
  and on the message page.
- React tests cover the three places and the fallback.

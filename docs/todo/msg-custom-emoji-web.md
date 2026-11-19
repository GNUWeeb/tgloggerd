# msg-custom-emoji-web

Draw custom emoji as their stickers wherever the web shows one: in
message text, reactions, forum topic icons and message effects.

## Why

A custom emoji reaction shows as a diamond, a custom emoji in a
message's text as its fallback emoji, and a forum topic's custom icon
not at all, because the API sends only ids. `msg-custom-emoji-store`
lets the daemon resolve ids to stickers.

## Scope

- Go (webapi): resolve the custom emoji ids of a page (text entities,
  reactions, topic icons, `meta.effect_id`) to their
  `telegram_custom_emoji` rows, asking the daemon for the ids not yet
  known, in one batch per page; send each as `{emoji, format, url}`
  beside the id (OpenAPI and web/docs follow). A page does not wait
  for the daemon: ids it cannot resolve in time keep their fallback.
- React (frontend): draw them inline at text size (WebP as an image,
  WebM as a muted looping video, TGS with the chat's Lottie player),
  with the fallback emoji as alt text; in reactions, topic tags and
  the topic strip; a message's effect as a small mark on the bubble.
- Left out: animating effects over the chat.

## Design notes

Custom emoji that need repainting (`needs_repainting`) take the text
colour. Lottie custom emoji can be many on one page; draw them still
until hovered or visible, and measure the chat's frame rate with a
page full of them. The text renderer is the text-entities item's (web
interface section, treated as landed).

## Depends on

`msg-custom-emoji-store`.

## Done when

- A message with a custom emoji in its text, a custom emoji reaction
  and a forum topic with a custom icon show the stickers in the chat
  and on the message page.
- A page whose ids the daemon cannot resolve still loads in its usual
  time, with the fallbacks.
- Go and React tests cover the three places and the fallback.

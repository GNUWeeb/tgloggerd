# msg-link-preview-types

Store and show the link previews still kept as `unsupported`, and the
media of the mapped ones that are not stored yet.

## Why

`telegram_link_previews.type` names 23 of TDLib's preview types. The
others are stored as `unsupported`, with no `extra` keys and no
media, so the web shows only their title and URL: an album, a video
note, a voice note, a live story, a story album, a chat background, a
sticker set, a theme, an upgraded gift, a gift collection or auction,
a premium gift code, a shareable chat folder, a direct messages chat,
a managed bot request and a text composition style. Some mapped types
also lose their media: a video preview's cover photo (only
`cover: true` is kept), a document preview's file and a sticker
preview's sticker.

## Scope

- Daemon (TDLib session): a `type` name for each of these TDLib
  classes, curated `extra` keys for what each carries (message-types.md
  section 4.8 lists them), and their media in `telegram_message_files`
  under role `preview_media` (a video note, a voice note, an album's
  items by position, a sticker set's first stickers, a background's
  document, a video preview's cover), through the normal file path and
  fetched again by the existing MessageExtra source.
- Go (webapi): the new type names and their `extra` keys and media on
  `link_preview` (ChatMessage and MessageFull); every type name the
  web reads today keeps its meaning. OpenAPI and web/docs follow.
- React (frontend): a card for each new type, drawn with the chat's
  media components; a type the app does not know keeps today's
  title-and-URL card.
- Left out: running a theme or a text composition style, and buying
  or joining anything a preview offers (its link opens in Telegram).

## Design notes

message-types.md section 4.8 and decision
[2026-11-18-msg-link-preview-options-in-meta](../design/decisions/2026-11-18-msg-link-preview-options-in-meta.md).
The `type` column is a VARCHAR, so new names need no migration; rows
stored as `unsupported` keep that type (only new and edited messages
get the new names), so readers must keep handling it. The media use
the `preview_media` role of migration 000045 with positions past the
preview's own video, animation or audio (position 0).

## Depends on

Nothing.

## Done when

- A new message with a link of each of these types stores its type
  name, its `extra` keys and its media, covered by daemon tests over
  hand-built TDLib objects.
- The web shows each new type with its media, and an `unsupported` row
  still shows as before; Go and React tests cover both.

# msg-link-preview-media

Keep and show a link preview's own media and target: the video,
animation or audio of a video preview, an embedded player's size, and
the message or story a Telegram link preview points at.

## Why

`telegram_link_previews` keeps one image per preview. A video or
animation preview shows only a still, an embed only its picture, and a
preview of a Telegram message or story only its URL and title.

## Scope

- Daemon (TDLib session): the preview's type-specific keys in the new
  `extra` column (duration, embed URL and size, author, the target
  chat and message or story of a `message` or `story` preview), and its
  video, animation or audio in `telegram_message_files` under role
  `preview_media`, stored or recorded under the storage cache rules,
  with a fetcher source for it. Done; its `extra` keys are listed in
  message-types.md section 4.8.
- Go (webapi): the preview's `extra` keys and its media as ChatMedia
  on `link_preview` (ChatMessage and MessageFull); a `message` preview
  whose target is in the archive and visible links to it. Done: the
  LinkPreview schema's `type`, `author`, `media`, `duration`, `embed`
  and `target`, described in web/docs/message-kinds.md.
- React (frontend): the preview plays its video or animation with the
  chat's components (`media`, with the photo as its poster), shows its
  duration, links a Telegram message preview to the archived message
  (`target`, to `/groups/{chat_id}/chat?at=<message_id>`), and opens
  an embed's URL (`embed.url`) in a new tab. The app's fixtures that
  build a `link_preview` gain its required `type`.
- Left out: running an embed's player (its URL opens in a new tab).

## Design notes

message-types.md section 4.8. The preview options stay in
`meta.link_preview_options` (decision
2026-11-18-msg-link-preview-options-in-meta). Old previews keep what
they have; only new and edited messages get the extras.

## Depends on

Nothing.

## Done when

- A new message with a YouTube or video link stores the preview's
  extras and media, and the chat plays the preview's video.
- A preview of a Telegram message in the archive links to it.
- React tests cover the extras, the media and the link (Go's are
  done).

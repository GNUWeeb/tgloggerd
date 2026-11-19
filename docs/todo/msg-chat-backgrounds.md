# msg-chat-backgrounds

Keep and show what a chat background change set, and an animated chat
photo's video: the wallpaper image or pattern, a fill's colours, and
the animation of an animated group photo.

## Why

A `chat_set_background` row keeps the background's id, name and type
but not the wallpaper image nor a fill's colours, so the web can only
say "changed the chat background". A `chat_change_photo` (or
`suggest_profile_photo`) with an animated photo keeps only
`animated: true`, not the video.

## Scope

- Daemon (TDLib session): the background's document in
  `telegram_message_files` under role `background`, and curated keys
  for the fill (`fill {type, colors, rotation_angle}`) and the pattern
  (`intensity`, `is_inverted`, `is_moving`); an animated chat photo's
  video under role `chat_photo` at position 1. Both stored or recorded
  under the storage cache rules, with fetcher sources.
  Done: the keys are in message-types.md sections 4.2.3 and the files
  in section 4.3.
- Go (webapi): `ServiceInfo.detail` gains the background (its image as
  ChatMedia and its fill) and the photo's animation; OpenAPI and
  web/docs follow.
- React (frontend): a small swatch of the background (the image over
  its fill) on the service line and the message page; the animated
  photo plays in the photo viewer.

## Design notes

message-types.md section 3.3 and 4.2.3. Old rows keep what they have.

## Depends on

Nothing.

## Done when

- A new background change shows its wallpaper, or its fill's colours,
  on its service line.
- A new animated group photo plays in the viewer from its service line.
- Daemon unit tests of the curated keys; Go and React tests of the
  detail.

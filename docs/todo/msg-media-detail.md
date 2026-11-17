# msg-media-detail

Store the detail of media messages that today keep only their file and
caption: size, duration, MIME type, spoiler, caption position, sticker
emoji and set, audio title and performer, voice waveform.

## Why

message-types.md section 3.1: the file alone does not show a reader
whether the photo was behind a spoiler, what a sticker meant, or what
song an audio file was.

## Scope

- Curated `content_extra` for photo, video, animation, audio,
  document, voice note, video note, sticker and paid media.
- Not in scope: extra files (`msg-message-files`).

## Design notes

message-types.md section 4.2. Keys are stable and documented in the
design file as they are added.

## Depends on

`msg-classify`

## Done when

- Tests cover each class's object; a live message of each kind (where
  one can be found in the archive's chats) shows the expected keys.

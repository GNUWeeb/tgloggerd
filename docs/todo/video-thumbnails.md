# video-thumbnails

Show a thumbnail for every video before it is played, wherever the web
shows one: a still frame instead of a generic film icon.

## Why

Videos, round video notes, GIF-like animations and video stickers
(.webm) show a type icon in the files grid and the file preview, and
only sometimes a cover in the chat, so a reader cannot tell them apart
or see what they are without playing each.

## Scope

- Every place a video appears before it plays: the files grid and the
  file preview, the chat's media and albums, the message page, search
  and list results that show media, and the users' and groups' photo
  carousels (video avatars).
- Kinds: videos, video notes, animations, video stickers (.webm), and
  video avatars.
- Not in scope: animated (.tgs) stickers, which the chat already plays
  with Lottie; a thumbnail for them is a separate question.

## Design notes

- Sources, in order of preference:
  1. what Telegram sends: a video's cover (telegram_message_files,
     role cover; MediaDetail.poster already uses it when stored) and
     the thumbnail TDLib offers with most videos, which the daemon can
     download like any small file;
  2. TDLib's minithumbnail (a tiny blurred JPEG, already stored and
     sent as MediaDetail.minithumb) as a placeholder while nothing
     better is stored;
  3. a frame taken from the stored bytes (ffmpeg, in a container of
     its own, never in the web service's request path), for files
     with neither.
- The API gives one field for it, a /files/ URL of a stored image (or
  the minithumb data URL), on FileRow (today's thumb is null for
  videos) and on the chat's media, so every page draws it the same
  way; FileRow.thumb stays an image, never the video itself.
- Thumbnails are small files of their own in the store, counted
  against its cap and evicted like others.
- Decide with the project owner whether frames are generated (3) or
  only Telegram's own thumbnails are used.

## Depends on

Nothing.

## Done when

- The files grid, the file preview and the chat show a still image for
  a stored video, video note, animation and .webm sticker that has a
  Telegram thumbnail or cover, and the minithumb (or the type icon)
  for one that has none.

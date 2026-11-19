# msg-files-roles

Show on /files which role a file plays in its message (cover, game,
invoice, paid item, link preview, thumbnail) and let the list filter
by it (optional).

## Why

Files linked through `telegram_message_files` (message-types.md
section 4.3) appear on /files, if at all, with no hint that they are a
video's cover or a link preview's picture rather than the message's
own file. Whether and how they are listed was not confirmed.

## Scope

- First check how /files lists those files today, and write down what
  it does.
- Go (webapi): `FileRow.role` (the role and the message's kind) and a
  role filter in the search registry; web/docs/search-api.md follows.
- React (frontend): the role as a small tag in the row, the card and
  the preview, and the filter.

## Design notes

What /files does today with the files a message carries beyond its
main one (`telegram_message_files`: cover, thumbnail, poll, game,
invoice, paid, live_video, chat_photo, and later rich, gift,
background, preview_media) and with a link preview's picture
(`telegram_link_previews.photo_file_id`, not a role row), checked in
webapi (logdb `Files`, `filesFilter`, `FileOrigins`, `FileStills`;
server `listFileRows`):

- A scope that sees every group and private chat (an administrator)
  lists every `telegram_files` row, so these files are listed, mixed
  in with the rest by id. The page count counts them too.
- Their row has no chat, sender or message: `FileOrigins` looks only
  at the messages' own `file_id`, so a file that is only a cover, a
  thumbnail or a link preview's picture looks like a profile or group
  photo. Nothing says what it is; a still thumbnail (file_type
  `thumbnail`) is answered as `photo`, and the `file_type = thumbnail`
  filter is the only way to pick out one role.
- A file that is also some message's main file shows that message,
  never the one it is a cover or a preview picture of.
- A scope that sees only some chats (`filesFilter`) lists only the
  files that are the main file of a message it sees, so these files
  are not listed at all, and not counted, even when the message that
  carries them is visible.
- The search API (`/api/search/files`) has the same rows and no field
  for the role.
- Covers and thumbnails are also used, by `FileStills`, as the
  thumbnail of the video they belong to; that does not list them.

## Depends on

Nothing.

## Done when

- A link preview's picture on /files says it is one and links to its
  message; filtering by the role lists only those.
- Go and React tests cover the field and the filter.

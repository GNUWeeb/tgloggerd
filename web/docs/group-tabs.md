# The group page's Files tab

Beside its Overview, Admins and History, a group's page has a Files tab
(`/groups/{id}/files`, route `group_files`). Its props are the group page's
own (`GroupProps`), as its other tabs carry them, with the first page of
its list beside them; `GET /api/groups/{id}/files` answers the same list
without the group page's props, for the following pages and for other
clients. The shapes are in
`webapi/openapi.yaml` (`GroupFilesProps`, `GroupFilesPage`,
`GroupFileRow`, `TabMessage`, `TabSender`).

## Who sees it

A signed-in viewer whose request scope sees the group: a superadmin, an
admin the group is not hidden from, a member granted it. Anyone else gets
404, as for a group that does not exist. Unlike an exposed group's
overview and chat, the tab is not public: a signed-out visitor is asked
to sign in, and a member not granted an exposed group gets 404.

## What it lists

The group's messages whose kind is one of photo, video, animation, audio,
voice, document, sticker and video_note, each with its main file
(`file_id`), newest first by Telegram message id, 50 a page. A message of
another kind with a file (a game's photo, a poll's media, a link
preview's picture) is not listed. `?before=<message_id>` gives the page
below that message; each page answers `next_before`, the last row's
message id, or null on the last page. `?kind=` (one of those kinds, or
`all`, the default) lists one kind. The API answers 400 for a `before`
that is not a positive integer and for any other `kind`; the page ignores
such a value.

`kinds` names the kinds the group has, with how many messages of each,
for the sub tabs. Counting them reads every message of the group, so the
count stops after 1.5 s: past that every kind is listed with a null
count, as which are present is not known then. The counts are kept for
two minutes per group, so paging and switching sub tabs count once.

Each row carries its file as the files list shows it (`url`, `thumb`,
`minithumb`, `stored`, `fetchable`, by the same rules; without the files
list's chat and message of the first message that carried the file
anywhere), and its own message: `message_id` for the chat's `?at=`,
`message_row_id` for `/messages/group/{row}`, `chat_link`
(`/groups/{id}/chat?at=<message_id>`), `date` and `sender`. The sender is
named as in the chat: a user, or a chat posting in its own name (a
channel, the group itself for an anonymous admin) with that chat's id and
title; a chat the viewer does not see has a null id and the name `(hidden
chat)`.

## Cost

A page is one query on `telegram_group_messages`, walking the
`(chat_id, message_id)` index down from `before` (joined to
`telegram_files`), then one batch each for the senders, the chats posting
and the videos' pictures (`FileStills`): never a query per row. A kind
the group rarely has makes its page read far down the group's messages to
fill 50 rows.

# The group page's Files and Polls tabs

Beside its Overview, Admins and History, a group's page has a Files tab
(`/groups/{id}/files`, route `group_files`) and a Polls tab
(`/groups/{id}/polls`, route `group_polls`). Each page's props are the
group page's own (`GroupProps`), as its other tabs carry them, or for a
public viewer the public group page's (see below), with the first page
of its list beside them; `GET /api/groups/{id}/files` and
`GET /api/groups/{id}/polls` answer the same lists without the group
page's props, for the following pages and for other clients. The shapes
are in `webapi/openapi.yaml` (`GroupFilesProps`, `GroupFilesPage`,
`GroupFileRow`, `GroupPollsProps`, `GroupPollsPage`, `GroupPollRow`,
`TabMessage`, `TabSender`).

## Who sees them

A viewer whose request scope sees the group: a superadmin, an admin the
group is not hidden from, a member granted it. Its props are then the
group page's own (`GroupProps`).

Like an exposed group's overview and chat, its tabs are public: a viewer
whose scope does not see an exposed group (`telegram_public_groups`) -
signed out, a member not granted it, an admin it is hidden from - gets
both tabs and their API twins as the public chat shows the group. The
senders are named by the same rules (a chat the viewer does not see is
`(hidden chat)`, the group itself included), the file links follow the
anonymous fetch rule of `/files` tokens, and a video's pictures come from
the group's own messages. The page's props are then the public group
page's (`GroupPublicProps`, `public: true`: no admins, history or
posters) with the tab's list beside them (`GroupFilesPublicProps`,
`GroupPollsPublicProps`).

A group that is neither seen nor exposed asks a signed-out visitor to
sign in (401 from the API) and is not found (404) for anyone else,
whether or not it exists.

## What they list

Both list the group's messages newest first, by Telegram message id, 50 a
page. `?before=<message_id>` gives the page below that message; each page
answers `next_before`, the last row's message id, or null on the last
page. The API answers 400 for a `before` that is not a positive integer;
the page ignores one and starts from the newest.

- Files: the messages whose kind is one of photo, video, animation, audio,
  voice, document, sticker and video_note, each with its main file
  (`file_id`). A message of another kind with a file (a game's photo, a
  poll's media, a link preview's picture) is not listed. `?kind=` (one of
  those, or `all`, the default) lists one kind; the API answers 400 for
  any other value. `kinds` names the kinds the group has, with how many
  messages of each, for the sub tabs. Counting them reads every message of
  the group, so the count stops after 1.5 s: past that every kind is
  listed with a null count, as which are present is not known then. The
  counts are kept for two minutes per group, so paging and switching sub
  tabs count once.
- Polls: the messages of kind poll, each with its poll as the chat shows
  it (`PollDetail`, docs in [message-kinds.md](message-kinds.md)): closed
  once past its close date even when Telegram never sent the update that
  closed it, the first 20 options with `options_total`, a quiz's correct
  option marked. A poll message whose poll was never stored has `poll:
  null`.

A file's row carries the file as the files list shows it (`id`, `url`,
`thumb`, `minithumb`, `stored`, `fetchable`, by the same rules; without
the files list's chat and message of the first message that carried the
file anywhere). Every row of both tabs carries its own message:
`message_id` for the chat's `?at=`, `message_row_id` for
`/messages/group/{row}`, `chat_link` (`/groups/{id}/chat?at=<message_id>`),
`date` and `sender`. The sender is
named as in the chat: a user, or a chat posting in its own name (a
channel, the group itself for an anonymous admin) with that chat's id and
title; a chat the viewer does not see has a null id and the name `(hidden
chat)`.

## Cost

A page is one query on `telegram_group_messages`, walking the
`(chat_id, message_id)` index down from `before` (joined to
`telegram_files` for the files), then one batch each for the senders, the
chats posting, the videos' pictures (`FileStills`) and the polls
(`telegram_polls`, `telegram_poll_options`): never a query per row. A
kind the group rarely has makes its page read far down the group's
messages to fill 50 rows.

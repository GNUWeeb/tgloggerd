# Message kinds in the chat

How the Go service (webapi/) shows the kinds of messages the logger
stores since daemon migration 000034 (docs/design/message-types.md,
sections 4.1 to 4.7): what the chat window
(`GET /api/chats/{scope}/{id}/messages`), the chat pages and the
message page (`/api/page/messages/{scope}/{id}`) send beyond the text
and the media, and what each viewer may see of it. The shapes are in
webapi/openapi.yaml: ContentType, MessageKinds, MessageDetail,
ServiceInfo, ChatTopic; the code is webapi/internal/server/kinds.go
and service.go.

## Content types

`content_type` is the stored kind: the ten the Drogon web knows
(`text`, `photo`, `video`, `document`, `audio`, `voice`, `sticker`,
`animation`, `service`, `unknown`) and the seventeen the daemon
appended (`video_note`, `location`, `venue`, `contact`, `dice`, `game`,
`poll`, `checklist`, `story`, `paid_media`, `invoice`, `giveaway`,
`call`, `rich`, `widget`, `expired`, `unsupported`). `unknown` is a
TDLib class newer than the daemon's code; `tdlib_type` names the exact
class. A video note is media like a video, with `round: true` on its
media object.

## Detail

`detail` is the kind's own data, an object chosen by `kind`; null for
text, the media kinds, service messages and unknown. It is built from
the row's `content_extra`, the poll tables (`telegram_polls`,
`telegram_poll_options`) and `telegram_message_files` (a game's or an
invoice's photo, paid media's items), all read in one batch per page
of messages.

- `location`: latitude, longitude, accuracy; a live location also its
  period, heading, proximity radius and `updates`, how many earlier
  positions the logger kept (one a minute at most). Only the latest
  position is shown, no track.
- `venue`, `dice` (a stake dice with `stake: true`), `game`, `story`
  (a reference: stories are not archived), `invoice`, `giveaway`,
  `call` (`outcome`: missed, declined, disconnected, hung_up, upgraded,
  ended, or for a group call active), `paid_media`.
- `poll`: the question, the type (regular or quiz), the flags, the
  latest results and the options still in it, a quiz's correct option
  marked. A poll message whose poll was never stored has no detail.
- `checklist`: its tasks, which are done, by whom and when; the title
  is the message text.
- `contact`: the names, and the phone number and vCard only for
  viewers that see users in full (superadmins and admins), as users'
  phones; a member gets null for both.
- `rich`, `widget`, `expired`, `unsupported`: the kind alone; the chat
  shows a one-line placeholder.

Long lists stop at the first 20 items: poll options, checklist tasks,
giveaway chats and group call participants. The matching `*_total`
counts them all, including people and chats left out as hidden, which
tells how many there were but not who.

Rows logged before migration 000034 have no `content_extra`. They keep
the kinds they had and get no detail, so they render as before.

## People and chats named

A detail or a service message names users and chats by id: a
contact's user, who completed a task, a call's participants, a story's
poster, a giveaway's chats, the members a service message added. Each
is a ChatRef (`kind` user or group) resolved through the request
scope, as the pages' users and groups are (accounts.md):

- users through logdb.VisibleUsers: every user for superadmins and
  admins; for a member only users that appear in its granted groups
  (senders and admins there);
- chats by the scope's SeesChat, and the chat being read.

One the viewer may not see is left out: null in a single field,
absent from a list. A user or chat the archive has no row for is named
by its id (`#9299000101`) for a viewer that would see it. Signed-out
visitors of an exposed group's chat get the anonymous scope's rule:
only the chat itself.

## Service messages

`service` is a service message's action: `type` (its service_type)
and `refs`, the users and chats it names that the viewer may see, and
`message_id`, the message of the chat it is about (the pinned
message, a changed poll or checklist, a giveaway), for `?at=` links.

The logger's own description of an action (the text column) names
nobody: "added 2 members to the group". Where the action names people,
the service rebuilds the text for the viewer with the names it may see
and a count of the others, never their names:

| Action | Text |
|---|---|
| chat_add_members | added Dewi Kusuma and 1 other member to the group |
| chat_delete_member | removed Bima Pratama from the group |
| chat_owner_left, chat_owner_changed | ... handed ownership to / transferred ownership of the group to ... |
| invite_video_chat_participants | invited ... to the video chat |
| giveaway_winners | announced the winners of the giveaway in <chat>: ... |
| chat_shared | shared <chat> with the bot |

When nothing visible can be named, and for rows logged before
migration 000034, the stored description is sent as it is. A member
granted one group thus never reads the name of someone who appears
only in another group, and an admin never reads the title of a group
hidden from it.

## Topics, pins and interaction

Each message carries `topic_id` (its forum topic, null outside forums),
`pinned`, and the latest `views`, `forwards`, `replies` (null while
never observed) and `reactions` (`[{emoji | custom_emoji_id | paid,
count}]`, empty when none were seen). A forum group's chat page lists
its `topics` (pinned first, then the newest); `?topic=<id>` on the
chat page and `topic` on the chat window keep the window to one topic,
with the cursors and has_older/has_newer counted within it. The
topic's rows are read through the (chat_id, topic_kind, topic_id)
index. A private chat takes no topic (400 on the API).

## Search

The advanced search takes the new content types, and `tdlib_type` and
`service_type` within one chat (search-api.md).

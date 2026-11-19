# Message kinds in the chat

How the Go service (webapi/) shows the kinds of messages the logger
stores since daemon migration 000034 (docs/design/message-types.md,
sections 4.1 to 4.7): what the chat window
(`GET /api/chats/{scope}/{id}/messages`), the chat pages and the
message page (`/api/page/messages/{scope}/{id}`) send beyond the text
and the media, and what each viewer may see of it. The shapes are in
webapi/openapi.yaml: ContentType, MessageKinds, MessageDetail,
ServiceInfo, ChatTopic, LinkPreview, TextEntity; the code is
webapi/internal/server/kinds.go, service.go and textentities.go.

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
- `story`: its `poster` and `story_id`, and `username`, the poster's
  active username for a `t.me/<username>/s/<id>` link: read with the
  poster in the same batch, and null without one or when the poster
  is hidden from the viewer (`poster` null).
- `poll`: the question, the type (regular or quiz), the flags, the
  latest results and the options still in it, a quiz's correct option
  marked. A poll message whose poll was never stored has no detail.
- `checklist`: its tasks, which are done, by whom and when; the title
  is the message text.
- `contact`: the names, and the phone number and vCard only for
  viewers that see users in full (superadmins and admins), as users'
  phones; a member gets null for both. The chat reads the vCard for
  its other numbers, e-mail addresses and organisation, and offers it
  as a `.vcf` download.
- `rich`, `widget`, `expired`, `unsupported`: the kind alone; the chat
  shows a one-line placeholder.

Long lists stop at the first 20 items: poll options, checklist tasks,
giveaway chats and group call participants. The matching `*_total`
counts them all, including people and chats left out as hidden, which
tells how many there were but not who.

Rows logged before migration 000034 have no `content_extra`. They keep
the kinds they had and get no detail, so they render as before.

## Summaries

Where one line stands for a message (a reply's quote in the chat and
on the message page, the message page's `snippet`, the message lists'
`snippet` on /group_messages and /private_messages, the dashboard's
deleted messages), a message without text is summarised by its kind
(webapi/internal/server/snippets.go):

- `poll`: the question (a quiz's too);
- `venue`: its title; `location`: "Location" or "Live location";
- `contact`: the contact's name as the message carries it;
- `call`: the call and its outcome, with the duration when it lasted,
  e.g. "Video call, missed", "Voice call, ended (1:23)", "Voice chat,
  ongoing" for a group call;
- `dice`: the emoji and the value rolled, separated by a space;
- `game`, `invoice`: the title;
- `story`: "Story by <poster>", the poster named through the request
  scope like any name, so a poster the viewer may not see leaves
  "Story";
- `giveaway`: "Giveaway: " and the prize (Premium months or Stars) and
  any extra prize;
- any other kind with a file: the file's name.

A kind without a summary, a row without its detail (logged before
migration 000034) and a poll whose poll was never stored keep the
placeholder `[<content_type>]`. A message with text shows its text, as
before. The summaries read the same detail as the chat; the reply
query reads the target's `content_extra` and file name only for a
target without text, and the lists read them only for such rows.

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

The message lists (the group_messages and private_messages pages,
MessageRow) carry the same action as `service_type` on a service
message's row, read from the stored column in the list's own query, so
a list can label the row; it is null on every other row.

The logger's own description of an action (the text column) names
nobody: "added 2 members to the group". Where the action names people,
the service rebuilds the text for the viewer with the names it may see
and a count of the others, never their names. Where its detail says
more than the description (a period, a distance, a checklist's tasks,
a giveaway's prize), the rebuilt text says it too, with or without
names:

| Action | Text |
|---|---|
| chat_add_members | added Dewi Kusuma and 1 other member to the group |
| chat_delete_member | removed Bima Pratama from the group |
| chat_owner_left, chat_owner_changed | ... handed ownership to / transferred ownership of the group to ... |
| invite_video_chat_participants | invited ... to the video chat |
| giveaway_winners | announced the winners of the giveaway in <chat>: ... |
| chat_shared | shared <chat> with the bot |
| chat_set_message_auto_delete_time | set messages to auto-delete after 1 day[, the default setting of <user>] (the period in the largest whole unit: seconds to years) |
| proximity_alert_triggered | came within 120 metres of <user or chat>; "reported that <traveler> came within ..." when the traveler is not the sender |
| users_shared | shared Bima Pratama and 1 other user with the bot |
| gifted_premium, gifted_stars | gifted a Telegram Premium subscription for 3 months / gifted 500 Telegram Stars [from <gifter>] [to <receiver>] |
| gift | sent a gift [from <sender>] [to <receiver>] |
| managed_bot_created | created the bot <bot> |
| checklist_tasks_done | marked "Task 2" and "Task 3" as done and "Task 1" as not done (the texts from the checklist message when the archive has it, else "2 tasks") |
| checklist_tasks_added | added "Write notes" to the checklist |
| suggest_birthdate | suggested a birthdate: 14 March 1995 |
| giveaway_created | created a giveaway of 500 Telegram Stars |
| giveaway_completed | the giveaway of Telegram Premium for 6 months and Stickers was completed with 3 winners and 1 unclaimed prize (a sentence, without an actor; the prize from the giveaway message when the archive has it) |

A gift's giver is named only when it is not the message's sender,
whom the line follows. A line quotes at most three tasks and counts the
others.

When nothing visible can be named, and for rows logged before
migration 000034, the stored description is sent as it is. A member
granted one group thus never reads the name of someone who appears
only in another group, and an admin never reads the title of a group
hidden from it.

`parts` is the rebuilt text piece by piece, so a client can link the
names without matching them in the text (two people can share a name,
and a name can contain the words around it). Each part has `text`;
a name also has `user` or `chat`, its index in `refs.users` or
`refs.chats`. Joined, the parts' texts are the message's text:

    [{"text": "added "}, {"text": "Dewi Kusuma", "user": 0},
     {"text": " and 1 other member to the group"}]

`parts` is absent where the stored description is sent; draw the text
then. Someone the viewer may not see is never a part, only counted.

## Text formatting

`entities` is Telegram's formatting of a text, as the daemon stores it
(format_entities_json in src/tgloggerd/MessageExtract.cpp): on every
ChatMessage and MessageFull (the text or caption; an album's is its
caption's), on each edit's earlier version, on a poll's question
(`question_entities`) and explanation (`explanation_entities`), and on
each checklist task. The shape is TextEntity in webapi/openapi.yaml:
`{offset, length, type}`, with `url` for a text_url, `language` for a
pre_code block, `custom_emoji_id` (a string, as the id does not fit a
double) for a custom emoji, and `user_id` for mentions. A plain text
has `[]`.

- `offset` and `length` count UTF-16 code units, as Telegram does
  (what a JavaScript string indexes), not bytes or code points.
- `type` is the TDLib entity type in snake_case: url, text_url,
  mention, mention_name, hashtag, cashtag, bot_command, email,
  phone_number, bank_card_number, bold, italic, underline,
  strikethrough, spoiler, code, pre, pre_code, block_quote,
  expandable_block_quote, custom_emoji, media_timestamp. A type newer
  than the daemon's code is stored as `unknown`; any type is passed
  through as stored, and a client shows the text of one it does not
  know as plain text.
- A text_url's `url` is sent as stored: a client checks its scheme
  before it links it.
- A media_timestamp carries no value: the daemon does not store it, so
  a client reads the time from the entity's text.

Mentions name people, so they follow the scope as the detail's refs
do (one batch per page of messages, never a query per message):

- a `mention_name` (a mention of someone without a username) has
  `user_id` only when the viewer may see that user
  (`logdb.VisibleUserIDs`), else `user_id: null`, its text still shown;
- a `mention` (`@username`) gets `user_id` when a user holds that
  username as an active one now (telegram_user_usernames, compared
  without case) and the viewer may see that user; otherwise it has no
  `user_id`, as for released, disabled and unknown usernames.

A service message gets `[]`: its text is the logger's description,
rebuilt for the viewer (above). A stored value that is not an array of
entities with an offset, a length and a type gives `[]` as a whole,
logged once at debug level.

## Topics, pins and interaction

Each message carries `topic_id` (its forum topic, null outside forums),
`pinned`, and the latest `views`, `forwards`, `replies` (null while
never observed) and `reactions` (`[{emoji | custom_emoji_id | paid,
count}]`, empty when none were seen). A forum group's chat page lists
its `topics` (pinned first, then the newest); `?topic=<id>` on the
chat page and `topic` on the chat window keep the window to one topic,
with the cursors and has_older/has_newer counted within it. The
topic's rows are read through the (chat_id, topic_kind, topic_id)
index. A private chat takes no topic (400 on the API). The message page
has no topic list, so its message also carries `topic`: the ChatTopic
`topic_id` names (name, icon colour, closed, hidden), read by its
primary key, or null when `topic_id` is null or the topic was never
seen.

## Link previews

A message with a link preview card (telegram_link_previews) carries it
as `link_preview` ({site, title, desc, url, media}, the LinkPreview
schema; media is its photo as ChatMedia), both in the chat window and
on the message page; null when it has none.

## Search

The advanced search takes the new content types, and `tdlib_type` and
`service_type` within one chat (search-api.md).

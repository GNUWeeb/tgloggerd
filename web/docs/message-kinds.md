# Message kinds in the chat

How the Go service (webapi/) shows the kinds of messages the logger
stores since daemon migration 000034 (docs/design/message-types.md,
sections 4.1 to 4.7): what the chat window
(`GET /api/chats/{scope}/{id}/messages`), the chat pages and the
message page (`/api/page/messages/{scope}/{id}`) send beyond the text
and the media, and what each viewer may see of it. The shapes are in
webapi/openapi.yaml: ContentType, MessageKinds, MessageDetail,
ServiceInfo, ChatTopic, LinkPreview, TextEntity, RichDetail; the code
is webapi/internal/server/kinds.go, service.go, service_detail.go,
textentities.go and rich.go.

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
invoice's photo, paid media's items, a giveaway's prize sticker), all
read in one batch per page of messages.

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
- `giveaway`: the prize, the winner count and date, the chats taking
  part, and `sticker`, the prize's sticker (the message's file of role
  gift) as ChatMedia; null when it is not recorded.
- `rich`: the page blocks; see "Rich messages" below.
- `widget`, `expired`, `unsupported`: the kind alone; the chat shows
  a one-line placeholder.

Long lists stop at the first 20 items: poll options, checklist tasks,
giveaway chats and group call participants. The matching `*_total`
counts them all, including people and chats left out as hidden, which
tells how many there were but not who.

Rows logged before migration 000034 have no `content_extra`. They keep
the kinds they had and get no detail, so they render as before.

Each earlier version of an edited message (`edits[]`, the Edit
schema) carries its own `detail`, built the same way from the edit
row's `content_extra` (the edit tables have it since migration 000034)
and with the people it names resolved through the same scope, in the
same batch as the page's messages. The logger keeps a live location's
positions as edit versions, one a minute at most, so its edit list is
the track of earlier positions: each version's `detail` holds that
position, and its `live.updates` counts the versions before it. A
checklist's version holds its tasks as they were then. A poll's
versions have no detail (null): the poll tables keep only the latest
results, so they keep only their text. A version shows none of the
message's other files (a game's or an invoice's photo, paid media's
items), which are the message's as it is now. A version without
`content_extra`, or of a kind without a detail, gets null.

## Rich messages

A rich message (`messageRichMessage`) is a page of blocks. The daemon
keeps its plain text in `text`, a summary and TDLib's own JSON of the
content in `content_extra`, and the media inside its blocks as message
files of role `rich`, numbered in order of appearance
(docs/design/message-types.md sections 4.2 and 4.3). Its `detail`
(RichDetail) is built from that JSON:

- `blocks`: the page's blocks (RichBlock), each with `type`, TDLib's
  class in snake_case without its `pageBlock` prefix (`paragraph`,
  `section_heading`, `list`, `table`, `photo`, ...), and the fields of
  that type; nested blocks (quotes, lists, collages, slideshows,
  collapsible blocks, covers, embedded posts) hold their own
  `blocks`. Every type TDLib has is converted; a block of a class this
  service does not know yet, and one TDLib itself could not read, is
  `unknown` with its class in `name`, so the page says it holds a
  block it does not show.
- Text is a RichText, `{text, entities}` in the shape of a message's
  text (offsets in UTF-16 code units), so it is drawn by the same
  text formatting renderer. Bold, italic, links, mentions, custom
  emoji and the rest map onto the entity types of "Text formatting";
  subscript, superscript and marked (highlighted) text are three types
  only a rich text has. A mention by user id is resolved like a
  message's `mention_name`: its `user_id` is null for a viewer that
  may not see that user.
- Links (a text link, a photo's link, an embed's or a post's address,
  a button's URL) are sent only when they are http, https, tg, mailto
  or tel links; a client opens them with `rel=noopener`. An embed's
  HTML is never sent: it needs a third party's script, so the page
  shows it as a link with its poster.
- A block's medium (`media`, a related article's `photo`) is a
  ChatMedia from the message's file at the position the daemon gave
  it, absent when no file is known. The position is counted the
  daemon's way: depth first, and only the media that have a file
  (a photo's largest size, a video, an animation, an audio file, a
  document, a voice note, an embed's poster, an embedded post's author
  photo, a related article's photo).
- `blocks` is null when the page was not kept: its JSON was over the
  daemon's cap (256 KiB), or the row was stored before the mapping and
  not read again yet. The chat then shows the message `text`, which
  holds the plain text of every block either way.
- `is_full` is false when TDLib had only the start of the message.

An earlier version (`edits[]`) of a rich message carries its own page
from the edit row, without media, as other kinds' versions do.

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

`detail` (ServiceDetail) is what the action's stored detail says
beyond its line, tagged by `kind`, for the types that have one; it is
absent where the type has none, nothing of it is stored, or what it
names the viewer may not see:

| Action | detail |
|---|---|
| chat_change_photo, suggest_profile_photo | `{kind: "photo", photo, animation}`: the new (or suggested) photo as ChatMedia, the message's file of role chat_photo at position 0; an animated photo's video at position 1 as ChatMedia (render animation, the still photo its poster when stored), else null |
| video_chat_ended, video_chat_scheduled | `{kind: "video_chat", duration, start_date}`: seconds, or when it is to start; the other is null |
| pin_message | `{kind: "pin", snippet}`: the pinned message quoted as a reply quotes it |
| poll_option_added, poll_option_deleted | `{kind: "poll_option", text}`: the option's text |
| gift, refunded_upgraded_gift, upgraded_gift, upgraded_gift_purchase_offer, upgraded_gift_purchase_offer_rejected, gifted_premium, gifted_stars, gifted_grams, giveaway_prize_stars | `{kind: "gift", sticker, symbol_sticker, star_count, price, sender, receiver, text, entities, upgraded}`: the gift (below) |
| chat_set_theme | `{kind: "theme", theme_name, gift_id, gift_name}`: an emoji theme's name, or the upgraded gift a gift theme is made from |
| forum_topic_* | `{kind: "topic", topic: {id, name, icon_color}}` |
| chat_added_to_community, chat_join_from_community | `{kind: "community", community: {id, name, photo}}`: the community (ServiceCommunity) |
| chat_create, chat_upgrade_from | `{kind: "title", title, basic_group}`: basic_group a ChatRef or null |
| chat_set_background | `{kind: "background", only_for_self, type, dark, image, fill, intensity, is_inverted, is_moving, is_blurred, theme_name}`: the background (below) |

    {"type": "pin_message", "refs": {...},
     "detail": {"kind": "pin", "snippet": "Release notes for 1.0 are out"}}

A gift's `sticker` is the message's file of role gift at position 0
as ChatMedia (render lottie for a `.tgs`, animation for a `.webm`,
image for a `.webp`): a regular gift's sticker (with the gift's emoji),
an upgraded gift's model sticker, or the animation sticker of the
Premium, Stars or grams given; `symbol_sticker` is an upgraded gift's
symbol sticker at position 1, drawn as a pattern over the backdrop.
`star_count` is a regular gift's price in Stars, or the Stars of
gifted_stars and giveaway_prize_stars; `price` a purchase offer's
(`{stars}` or `{gram_cents}`). `sender` and `receiver` are ChatRefs
(the giver of Premium, Stars or grams is `sender`), null for a private
gift or one the viewer may not see; `text` and `entities` are the
message the giver attached. `upgraded` (UpgradedGift) is the upgraded
gift of upgraded_gift and the purchase offers: `id`,
`regular_gift_id`, `title`, `name` (its unique name, for
`https://t.me/nft/<name>`), `number`, `owner` (a ChatRef or null),
`model` `{name, rarity_per_mille, rarity, crafted}`, `symbol` `{name,
rarity_per_mille, rarity}`, `backdrop` `{name, center_color,
edge_color, symbol_color, text_color, rarity_per_mille, rarity}`
(colours RGB integers, `0xRRGGBB`; the backdrop is a radial gradient
from the centre to the edge colour) and `original_details` `{sender,
receiver, text, entities, date}`, the regular gift it was upgraded
from. A rarity is `rarity_per_mille` (how many of 1000 upgraded gifts
get it; 0 means under 0.1%) or, when Telegram names it instead,
`rarity` (`uncommon`, `rare`, `epic`, `legendary`); the other is null.
The people and chats a gift names go through the request scope as the
refs do, so the owner or the original sender may be null for one
viewer and not for another. Rows logged before the stickers and the
look were kept have null for them; a gift with nothing stored has no
detail.

    {"kind": "gift", "sticker": {"render": "lottie", "ext": "tgs", ...},
     "symbol_sticker": null, "star_count": 50, "price": null,
     "sender": {"id": 9299000002, "kind": "user", ...},
     "receiver": {"id": 9299000001, "kind": "user", ...},
     "text": "Happy birthday", "entities": [], "upgraded": null}

A pin has a detail only when the archive has the pinned message (pins
logged before their message id was recorded have none); a message
without text is summarised as a reply's target is, so a story's poster
the viewer may not see is not named ("Story"). A topic's `id` is the
message's own forum topic, for a `?topic=` link; its name and colour
are the action's, else the topic's as last seen. The basic group is
given only when the archive has the group and the viewer may see it.
A community is not a chat: its `id` is TDLib's community id, it has no
page to link to, and `photo` is its photo's `/files/<token>` URL or
null. It comes from the communities the logger records
(telegram_communities and telegram_community_chats,
docs/design/message-types.md section 4.13) and is given once the
archive has its name, to a viewer who may see at least one of its
chats or every chat; chat_removed_from_community has none, since its
action names no community.

A background's `type` is `wallpaper`, `pattern`, `fill` or
`chat_theme` (null on rows logged before the background was kept,
which have only `only_for_self`). `image` is the message's file of
role background as ChatMedia: a wallpaper's JPEG or a pattern's PNG
(render image), or a pattern in Telegram's gzipped SVG format (`tgv`,
render file, which a browser cannot draw as it is); null for a fill
and a chat theme. `fill` is a fill's, or the fill a pattern is drawn
over: `{type, colors, rotation_angle}`, `type` `solid`, `gradient` or
`freeform_gradient`, `colors` RGB integers (`0xRRGGBB`: one, the top
then the bottom colour, or three or four), `rotation_angle` a
gradient's clockwise rotation in degrees (0 otherwise). A pattern has
its `intensity` (0-100, the pattern's opacity over the fill),
`is_inverted` and `is_moving`; a wallpaper `is_blurred` and
`is_moving`; a chat theme its `theme_name` (an emoji; its colours are
not stored). The rest is null or false. `dark` says the background is
meant for a dark theme.

    {"kind": "background", "only_for_self": false, "type": "pattern",
     "dark": false, "image": {"render": "file", "ext": "tgv", ...},
     "fill": {"type": "gradient", "colors": [16744448, 255],
              "rotation_angle": 45},
     "intensity": 50, "is_inverted": false, "is_moving": true,
     "is_blurred": false, "theme_name": null}

## Text formatting

`entities` is Telegram's formatting of a text, as the daemon stores it
(format_entities_json in src/tgloggerd/MessageExtract.cpp): on every
ChatMessage and MessageFull (the text or caption; an album's is its
caption's), on each edit's earlier version, on a poll's question
(`question_entities`) and explanation (`explanation_entities`), and on
each checklist task. The shape is TextEntity in webapi/openapi.yaml:
`{offset, length, type}`, with `url` for a text_url, `language` for a
pre_code block, `custom_emoji_id` (a string, as the id does not fit a
double) and `custom_emoji` (below, "Custom emoji") for a custom emoji,
and `user_id` for mentions. A plain text
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

Each message carries `topic_id` (its forum topic, null outside
forums), `pinned`, and the latest `views`, `forwards`, `replies` (null
while never observed) and `reactions` (`[{emoji | custom_emoji_id and
custom_emoji | paid, count}]`, empty when none were seen). A forum
group's chat page lists its `topics` (pinned first, then the newest);
`?topic=<id>` on the chat page and `topic` on the chat window keep the
window to one topic, with the cursors and has_older/has_newer counted
within it. The topic's rows are read through the (chat_id, topic_kind,
topic_id) index. A private chat takes no topic (400 on the API). The
message page has no topic list, so its message also carries `topic`:
the ChatTopic `topic_id` names (name, icon colour and custom emoji,
closed, hidden), read by its primary key, or null when `topic_id` is
null or the topic was never seen.

## Custom emoji

Telegram names a custom emoji by its id only, and a message effect
(`meta.effect_id`) likewise. Each id goes out with what is known of it
beside it, null while nothing is:

- a text entity of type custom_emoji and a reaction with a
  `custom_emoji_id` carry `custom_emoji`, a CustomEmoji
  `{emoji, format, url, needs_repainting}` or null;
- a ChatTopic (the chat page's `topics`, the message page's `topic`)
  and a topic line's ServiceTopic carry `icon_custom_emoji_id` and
  `icon_custom_emoji` (null for an icon drawn with its colour alone);
  a topic line takes the icon its action set, or removed, else the
  topic's as last seen;
- each ChatMessage and MessageFull carries `effect_id` and `effect`, a
  MessageEffect `{emoji, format, url}` or null.

`emoji` is the fallback emoji. `url` is the sticker's /files/ URL in
`format` (webp, tgs or webm), given as any file's is (stored, or
fetchable by the viewer through the daemon), and null when there is no
such file or its format is not known: the client shows the emoji then.
A custom emoji with `needs_repainting` is drawn in the text colour. An
effect's picture is its static icon (webp), else a premium sticker
effect's sticker, else an emoji reaction effect's select animation
(tgs); its full-screen animation is not sent.

The ids of a page (its messages' entities, edits, polls, checklist
tasks, reactions, effects, topic lines and the topics the page lists)
are resolved in one batch: read from telegram_custom_emoji and
telegram_message_effects, which the daemon fills on demand
(docs/design/message-types.md sections 4.11 and 4.12), and those the
tables lack asked of the daemon in one `POST /v1/custom-emoji` and one
`POST /v1/message-effects` (docs/internal-api.md). The page waits for
those answers at most 400 ms; an ask that takes longer goes on in the
background, the page sends its ids as null, and a later page reads the
rows the ask stored. The service remembers, in memory:

- the ids being asked for, which another page does not ask for again
  nor wait on;
- the ids Telegram does not know, not asked for again for an hour;
- a pause after a flood wait (its retry_after) and, for 30 s, after an
  ask with no answer (the daemon off, away or slow) or one Telegram
  failed; no page asks in a pause, so it costs no page any time.

## Link previews

A message with a link preview card (telegram_link_previews) carries it
as `link_preview` (the LinkPreview schema), both in the chat window and
on the message page; null when it has none. The card has:

- `type`, the preview's TDLib type (`article`, `video`,
  `embedded_video_player`, `message`, ...), `site`, `title`, `desc`,
  `url`, and `author` when the page names one.
- `media`, as ChatMedia: the preview's own medium
  (telegram_message_files, role `preview_media`, position 0) when the
  daemon kept one: the video, animation, audio, document, sticker,
  video note (round) or voice note of such a preview, the cached
  medium of an embedded player, or a story album's video icon. Its
  detail comes from the preview's `extra` (an embedded player's from
  `extra.media`, a story album's from `extra.video`; an audio's title
  from `audio_title`), with the preview's photo as its `poster` (video,
  animation) and `thumb`; a video preview's kept cover (position 1) is
  its poster before the photo. Without such a medium it is the photo,
  rendering as an image, sized for article, photo, app, web_app and
  story_album previews. A card never carries a `minithumb`.
- `duration`, the length of a video, animation or audio preview or of
  an embedded player; `embed` ({url, w, h}) for an embedded player,
  whose URL the web opens in a new tab rather than running it.
- `target` ({chat_id, message_id}) for a `message` preview whose link
  names a logged group message the viewer may open: a group the scope
  sees, or an exposed one, as the chat page itself decides. A
  `t.me/c/` link names the chat by id; a public link by a username,
  resolved to the group that holds it as an active username now
  (telegram_group_usernames, case-insensitive). The client links it to
  `/groups/{chat_id}/chat?at=<message_id>`. The lookups take two
  queries per window at most, by the usernames' unique key and the
  messages' (chat_id, message_id) key; nothing is asked of Telegram.

- The types whose media are not one medium carry them in fields of
  their own, each item the `preview_media` file at its position
  (docs/design/message-types.md section 4.3), null when it is not
  recorded: `album` ({caption, items}) for an album; `background`
  (LinkPreviewBackground: the type, fill, intensity and flags as a
  chat background service message has them, and its image) for a
  background; `stickers` for a sticker set's first stickers and a gift
  collection's icons; `gift` (LinkPreviewGift: the sticker, star count
  and auction dates of a gift auction, or an upgraded gift with its
  model and symbol stickers) for gift_auction and upgraded_gift. Their
  `media` is the card's photo alone, when it has one.
- `story` ({poster, story_id, username}) for a story or live_story
  preview, the poster as a ref the viewer may see (null otherwise), as
  StoryDetail has it; `theme` (LinkPreviewTheme: its base theme,
  colours, background and outgoing message fill; the theme's files are
  not kept) for a theme; `custom_emoji_id` and `custom_emoji` (resolved
  for the page as text entities' are, null when not) for a text
  composition style. The people an upgraded gift or a story names, and
  that custom emoji, load with the page's other refs, in the same
  batch.

The type is sent as stored. Rows logged as `unsupported` before the
daemon named video_note, voice_note, album, background, sticker_set,
gift_collection, gift_auction, upgraded_gift, live_story, story_album,
text_composition_style, theme, premium_gift_code,
request_managed_bot, shareable_chat_folder and direct_messages_chat
keep that type and show their title, URL and photo as before. The
extras (`extra`, docs/design/message-types.md section 4.8) and the
media exist only for previews logged since the daemon kept them; older
cards have the photo alone.

## Search

The advanced search takes the new content types, and `tdlib_type` and
`service_type` within one chat (search-api.md).

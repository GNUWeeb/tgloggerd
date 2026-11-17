# Message types: storing every kind of Telegram message faithfully

Status: planned 2026-11-18, not implemented; awaiting approval of the
plan. The items that implement it are in the "Message types" section
of [../TODO.md](../TODO.md); the decisions behind it are under
[decisions/](decisions/) (files named `2026-11-18-msg-*.md`).

## 1. Problem and goals

TDLib 1.8.70 (the tree's `submodules/td`) reports the content of a
message as one of 109 `MessageContent` classes. The daemon recognises
eight of them as media (`extract_message_content` and
`message_content_file` in `src/tgloggerd/TDLib.cpp`), describes about
thirty service messages in English (`describe_service_message`), and
stores everything else as `content_type = 'unknown'` with no text, no
file and no record of what it was. In the live archive about half a
percent of group messages are such rows; what they were cannot be
told from the database.

Even the recognised kinds lose detail: a pinned-message notice does not
say which message was pinned, a member removal does not say who was
removed, a sticker keeps its file but not its emoji, and a photo with a
spoiler keeps the photo but not the spoiler.

Goals:

1. Every message gets a row that says exactly which TDLib class it
   was, even for classes the daemon does not interpret.
2. Every class a person can see in a chat is stored with the fields a
   reader needs to reconstruct it: what it said, what it showed, who
   it referred to, and its files.
3. Structured detail is stored as data, not only as English text; the
   existing `text` description of service messages stays for
   compatibility and search.
4. Schema changes are additive (new columns, new tables, new ENUM
   values appended at the end), so the deployed web keeps working
   before it learns the new kinds.
5. Rows stored before the change are repaired where TDLib can still
   return the message.

Non-goals: secret chats (the daemon runs with `use_secret_chats_ =
false`), scheduled messages, and content only bots receive
(`updatePollAnswer`, passport data in full).

## 2. What is stored today

Per message, both `telegram_group_messages` and
`telegram_private_messages` keep: chat and message id (the server id,
see `to_server_msg_id`), sender, date, edit date, `content_type` (ENUM
`text, photo, video, document, audio, voice, sticker, animation,
service, unknown`), `text` (text or caption), `entities` (formatting
as JSON, UTF-16 offsets), `service_type` (an action id for service
messages), one `file_id`, `deleted_at`, `is_forwarded`,
`media_album_id`, and the reply target (`reply_to_chat_id`,
`reply_to_msg_id`, `reply_to_id`). Group rows also keep
`is_channel_post` and `author_signature`; private rows `is_outgoing`.

Side tables: `*_message_edits` (snapshot of content_type, text,
entities and file before each edit), `*_message_fwd_info` (forward
origin: user, hidden user, chat or channel, origin date and message),
`telegram_link_previews` (one card per text message: url, title,
description as plain text, type, flags, image file), and
`telegram_files` (one row per Telegram file, `file_type` ENUM without
a video note kind).

Ingest paths: `updateNewMessage` and `updateMessageSendSucceeded`
(`handle_new_message`), `updateMessageContent` (late link previews and
inline edit versions; polls, locations and expired media are skipped
as churn), `updateMessageEdited` (re-fetch with `getMessage`),
`updateDeleteMessages`, history backfill (`getChatHistory`), and the
reply resolver and relink re-fetch (`getMessage`). No other message
update is handled: not `updateMessageIsPinned`,
`updateMessageInteractionInfo`, `updateMessageReactions`, `updatePoll`,
`updateMessageFactCheck` or `updateForumTopic*`.

### 2.1 Known defect

The upsert in `models/GroupMessage.cpp` (and its private twin) takes the
content columns from any copy whose `edit_date` is not older than the
stored one. A self-destructing photo, video, video note or voice note
becomes `messageExpired*` once opened or timed out; when that message
is ingested again (reply resolution, backfill, relink) the stored
`photo` row is rewritten to `unknown` with no text. The file link
survives only because the upsert does not touch `file_id`. Item
`msg-expired-media-keep` fixes this before anything else (decision
[2026-11-18-msg-expired-media-keep](decisions/2026-11-18-msg-expired-media-keep.md)).

## 3. Content types and what is lost

"Stored" is one of: **fully** (nothing a reader needs is lost),
**partly**, **text only** (a service message whose detail survives
only in the English description), or **not** (stored as `unknown`).
"Planned kind" is the `content_type` after the change (section 4.1);
service messages keep `content_type = 'service'` and get the
`service_type` shown. An animated emoji is stored as text (decision
[2026-11-18-msg-animated-emoji-text](decisions/2026-11-18-msg-animated-emoji-text.md)).

### 3.1 Text and media

| TDLib class | Stored | What is lost today | Planned kind |
|---|---|---|---|
| messageText | partly | link preview options (disabled, forced url, size, position); entities of the preview description | text |
| messageRichMessage | not | the whole message (page blocks, its media) | rich |
| messageAnimatedEmoji | not | the emoji itself: this is a text message of one emoji that TDLib reports specially | text |
| messagePhoto | partly | spoiler, caption above media, secret flag, size, live-photo video (`video`) | photo |
| messageVideo | partly | cover, start timestamp, other qualities, size, duration, MIME, spoiler, caption above | video |
| messageAnimation | partly | size, duration, MIME, spoiler, caption above, secret flag | animation |
| messageAudio | partly | title, performer, duration, MIME, album cover | audio |
| messageDocument | partly | MIME type, thumbnail | document |
| messageVoiceNote | partly | duration, waveform, MIME, listened flag | voice |
| messageVideoNote | not | the round video file, duration, length, secret flag | video_note |
| messageSticker | partly | emoji, set id, format, kind (regular, mask, custom emoji), premium flag | sticker |
| messagePaidMedia | not | price in stars, every photo or video (or its preview), caption | paid_media |
| messageExpiredPhoto, messageExpiredVideo, messageExpiredVideoNote, messageExpiredVoiceNote | not | which kind expired; worse, see section 2.1 | expired |

### 3.2 Places, people, games and interactive content

| TDLib class | Stored | What is lost today | Planned kind |
|---|---|---|---|
| messageLocation | not | latitude, longitude, accuracy | location |
| messageLiveLocation | not | the position, period, heading, proximity radius, expiry; later positions | location |
| messageVenue | not | title, address, provider and venue id, type, position | venue |
| messageContact | not | phone, names, vCard, user id | contact |
| messageDice | not | emoji and value | dice |
| messageStakeDice | not | value, stake and prize | dice |
| messageGame | not | game title, description, short name, photo or animation, text | game |
| messagePoll | not | question, options, type (regular, quiz), flags, results, close date, description, media | poll |
| messageChecklist | not | title, tasks, who did which task | checklist |
| messageWidget | not | the widget's HTML, size, caption | widget |
| messageStory | not | which story was shared (poster chat, story id) | story |
| messageInvoice | not | product title, description, photo, currency, amount, paid media | invoice |
| messageGiveaway | not | parameters, prize, winner count | giveaway |
| messageCall | not | video or voice, duration, how it ended | call |
| messageGroupCall | not | active or missed, video, duration, participants | call |
| messageUnsupported | not | (TDLib itself cannot read it) | unsupported |

### 3.3 Service messages: chat membership and settings

| TDLib class | Stored | What is lost today | Planned service_type |
|---|---|---|---|
| messageBasicGroupChatCreate | text only | title, initial members | chat_create |
| messageSupergroupChatCreate | text only | title | chat_create |
| messageChatChangeTitle | text only | (title is inside the description) | chat_change_title |
| messageChatChangePhoto | partly | the new photo (only `updateChatPhoto` history keeps one) | chat_change_photo |
| messageChatDeletePhoto | fully | - | chat_delete_photo |
| messageChatAddMembers | partly | which users were added | chat_add_members |
| messageChatJoinByLink | fully | - | chat_join_by_link |
| messageChatJoinByRequest | fully | - | chat_join_by_request |
| messageChatJoinFromCommunity | not | community id | chat_join_from_community |
| messageChatDeleteMember | partly | which user was removed | chat_delete_member |
| messageChatAddedToCommunity | not | community id | chat_added_to_community |
| messageChatRemovedFromCommunity | not | the event | chat_removed_from_community |
| messageChatUpgradeTo | partly | supergroup id | chat_upgrade_to |
| messageChatUpgradeFrom | partly | old title, basic group id | chat_upgrade_from |
| messageChatOwnerLeft | not | new owner | chat_owner_left |
| messageChatOwnerChanged | not | new owner | chat_owner_changed |
| messageChatHasProtectedContentToggled | not | old and new value, request message | chat_has_protected_content_toggled |
| messageChatHasProtectedContentDisableRequested | not | expired flag | chat_has_protected_content_disable_requested |
| messagePinMessage | partly | which message was pinned | pin_message |
| messageScreenshotTaken | fully | - | screenshot_taken |
| messageChatSetBackground | partly | the background, own-only flag, previous message | chat_set_background |
| messageChatSetTheme | partly | the theme | chat_set_theme |
| messageChatSetMessageAutoDeleteTime | text only | who set it (`from_user_id`) | chat_set_message_auto_delete_time |
| messageChatBoost | partly | number of boosts | chat_boost |
| messageForumTopicCreated | text only | icon, implicit-name flag | forum_topic_created |
| messageForumTopicEdited | partly | new name, new icon | forum_topic_edited |
| messageForumTopicIsClosedToggled | partly | closed or reopened | forum_topic_is_closed_toggled |
| messageForumTopicIsHiddenToggled | partly | hidden or shown | forum_topic_is_hidden_toggled |
| messageCustomServiceAction | fully | - | custom_service_action |
| messageContactRegistered | fully | - | contact_registered |
| messageSuggestProfilePhoto | not | the photo | suggest_profile_photo |
| messageSuggestBirthdate | not | the date | suggest_birthdate |
| messageProximityAlertTriggered | not | who, whom, distance | proximity_alert_triggered |
| messageManagedBotCreated | not | bot and manager bot | managed_bot_created |
| messageBotWriteAccessAllowed | not | why (web app, attachment menu, request) | bot_write_access_allowed |
| messageUsersShared | not | shared users, button id | users_shared |
| messageChatShared | not | shared chat, button id | chat_shared |
| messageWebAppDataSent | not | button text | web_app_data_sent |
| messageWebAppDataReceived | not | button text, data (bots only) | web_app_data_received |
| messagePassportDataSent | not | element types | passport_data_sent |
| messagePassportDataReceived | not | (bots only; encrypted, kept out on purpose) | passport_data_received |
| messageGameScore | not | game message, score | game_score |

### 3.4 Service messages: calls, polls, checklists, posts

| TDLib class | Stored | What is lost today | Planned service_type |
|---|---|---|---|
| messageVideoChatScheduled | partly | call id, start date | video_chat_scheduled |
| messageVideoChatStarted | partly | call id | video_chat_started |
| messageVideoChatEnded | partly | duration | video_chat_ended |
| messageInviteVideoChatParticipants | partly | call id, invited users | invite_video_chat_participants |
| messagePollOptionAdded | not | poll message, option, text | poll_option_added |
| messagePollOptionDeleted | not | poll message, option, text | poll_option_deleted |
| messageChecklistTasksDone | not | checklist message, tasks marked and unmarked | checklist_tasks_done |
| messageChecklistTasksAdded | not | checklist message, new tasks | checklist_tasks_added |
| messageSuggestedPostApprovalFailed | not | post, price | suggested_post_approval_failed |
| messageSuggestedPostApproved | not | post, price, send date | suggested_post_approved |
| messageSuggestedPostDeclined | not | post, comment | suggested_post_declined |
| messageSuggestedPostPaid | not | post, amounts | suggested_post_paid |
| messageSuggestedPostRefunded | not | post, reason | suggested_post_refunded |

### 3.5 Service messages: payments, gifts, stars

| TDLib class | Stored | What is lost today | Planned service_type |
|---|---|---|---|
| messagePaymentSuccessful | partly | invoice message, currency, amount, subscription | payment_successful |
| messagePaymentSuccessfulBot | partly | currency, amount, charge ids (payload and order info are bot-side) | payment_successful_bot |
| messagePaymentRefunded | not | owner, currency, amount, charge ids | payment_refunded |
| messageGiftedPremium | partly | gifter, receiver, text, price, months | gifted_premium |
| messagePremiumGiftCode | not | creator, text, price, months, code | premium_gift_code |
| messageGiftedStars | not | gifter, receiver, price, stars | gifted_stars |
| messageGiftedGrams | not | gifter, receiver, amount | gifted_grams |
| messageGiveawayCreated | not | stars | giveaway_created |
| messageGiveawayCompleted | not | giveaway message, winners, unclaimed | giveaway_completed |
| messageGiveawayWinners | not | chat, winners, prize | giveaway_winners |
| messageGiveawayPrizeStars | not | stars, chat, giveaway | giveaway_prize_stars |
| messageGift | not | gift, sender, receiver, text, flags, prices | gift |
| messageUpgradedGift | not | unique gift, sender, receiver, origin, flags | upgraded_gift |
| messageRefundedUpgradedGift | not | gift, sender, receiver, origin | refunded_upgraded_gift |
| messageUpgradedGiftPurchaseOffer | not | gift, state, price, expiry | upgraded_gift_purchase_offer |
| messageUpgradedGiftPurchaseOfferRejected | not | gift, price, offer message | upgraded_gift_purchase_offer_rejected |
| messagePaidMessagesRefunded | not | message count, stars | paid_messages_refunded |
| messagePaidMessagePriceChanged | not | price | paid_message_price_changed |
| messageDirectMessagePriceChanged | not | enabled flag, price | direct_message_price_changed |
| messageUnpaidMessagesAllowedByLink | not | the event | unpaid_messages_allowed_by_link |
| messageTonWalletTransfer | not | sender, transaction, peer address, amount, comment | ton_wallet_transfer |
| messageTonConnectRequest | not | session, state, dApp name, topic | ton_connect_request |

Every service_type is the TDLib class name without its `message`
prefix in snake case, which is what the existing ones already are
except `chat_create` (two classes share it) and `payment_successful`
(today used for both payment classes; the bot one gets its own name
for new rows). See decision
[2026-11-18-msg-service-type-names](decisions/2026-11-18-msg-service-type-names.md).

### 3.6 Message-level fields

| `td_api::message` field | Stored | What is lost today |
|---|---|---|
| forward_info | partly | `source` (the chat and message a forward was saved from), public service announcement type |
| reply_to (messageReplyToMessage) | partly | quote (text, entities, position), checklist task or poll option replied to, origin and content of a reply into a chat not logged |
| reply_to (messageReplyToStory) | not | the story replied to |
| topic_id | not | forum topic, thread, direct-messages or saved-messages topic |
| is_pinned | not | pinned state (only the pin service message, without its target) |
| interaction_info | not | views, forwards, reply count, reactions |
| unread_reactions | not | (account-local state, not archived on purpose) |
| via_bot_user_id | not | inline bot used |
| reply_markup | not | inline and reply keyboards (buttons, URLs) |
| effect_id | not | message effect |
| self_destruct_type, auto_delete_in | not | self-destruct timer |
| paid_message_star_count | not | stars paid to send it |
| sender_boost_count, sender_tag | not | sender's boosts and tag |
| fact_check | not | fact check text |
| import_info | not | imported-from name and date |
| restriction_info | not | why the message is restricted |
| suggested_post_info | not | suggested post state and price |
| media_album_id, author_signature, is_channel_post, date, edit_date | fully | - |

## 4. Design

### 4.1 The content kind (`content_type`) and the exact class (`tdlib_type`)

The `content_type` ENUM of both message tables and both edit tables
gains, appended at the end so existing values keep their numbers:
`video_note, location, venue, contact, dice, game, poll, checklist,
story, paid_media, invoice, giveaway, call, rich, widget, expired,
unsupported`. `unknown` stays and from now on means only "a class
newer than the daemon's code".

A new column `tdlib_type VARCHAR(64)` holds the exact TDLib class name
(for example `messageVideoNote`) for every row written after the
change, so no row is ever ambiguous again, and a later TDLib with new
classes is still recorded by name.

On MySQL 8.0.29 and later, both changes are instant metadata changes
even on the largest table (appending ENUM members that fit the same
storage size; adding a nullable column). See decision
[2026-11-18-msg-content-kinds](decisions/2026-11-18-msg-content-kinds.md).

`telegram_files.file_type` gains `video_note`, and `thumbnail` for
covers and previews (section 4.3).

### 4.2 Structured detail (`content_extra`)

A new nullable `content_extra JSON` column on both message tables and
both edit tables holds the fields of section 3 that a reader needs and
that have no column, as a curated object per class with stable keys,
for example:

- location: `{"lat":..,"lon":..,"accuracy":..}` plus, for a live
  location, `{"live_period":..,"heading":..,"proximity_radius":..}`;
- contact: `{"phone":..,"first_name":..,"last_name":..,"vcard":..,
  "user_id":..}`;
- sticker: `{"emoji":..,"set_id":..,"format":"webp|tgs|webm",
  "sticker_type":..,"custom_emoji_id":..}`;
- media: `{"width":..,"height":..,"duration":..,"mime":..,
  "spoiler":true,"caption_above":true}` (only the keys that apply);
- service messages: the action's own fields, e.g. `{"user_ids":[..]}`
  for chat_add_members, `{"message_id":..}` (server id) for
  pin_message, `{"currency":..,"amount":..}` for payments.

User ids, chat ids and message ids in it use the same conventions as
the columns (server message ids). File references are not in the JSON
but in the message-files table (section 4.3), so the foreign keys keep
working.

For a class with no curated mapping yet (rare or bot-only ones, rich
messages, widgets), `content_extra` holds `{"tdlib": <the content as
TDLib's own JSON>}` with local file paths removed, produced by the
JSON serializer TDLib already builds (`tdjson_private`). A later
version can turn it into a curated object from the row alone.

`content_extra` is part of the edit snapshot: an edit that changes only
it (a live location moving, a poll description edited) is a version.
See decision
[2026-11-18-msg-content-extra](decisions/2026-11-18-msg-content-extra.md).

### 4.3 Several files per message (`telegram_message_files`)

A message can carry more than one file: paid media (several photos or
videos), a photo with a live-photo video, a video with a cover, a poll
with media, a game with a photo, an invoice with a photo, a chat photo
change. A new table follows the pattern of `telegram_link_previews`:

    telegram_message_files
      id, group_message_id NULL, private_message_id NULL,
      position SMALLINT, role ENUM('main','cover','live_video',
      'paid','preview','poll','game','invoice','chat_photo',
      'thumbnail'), file_id (FK telegram_files, ON DELETE SET NULL)

The existing `file_id` column stays the main file and keeps its
meaning, so the web and the forwarders do not change; the new table
lists every file including the main one. Downloads follow the storage
cache rules (`docs/design/storage-cache.md`): non-member chats keep
the file id only. See decision
[2026-11-18-msg-message-files](decisions/2026-11-18-msg-message-files.md).

### 4.4 Polls

A poll changes after it is sent (votes, closing), and TDLib announces
it with `updatePoll` keyed by the poll id, not the message. Two tables:

    telegram_polls
      id (TDLib poll id), question, question_entities JSON,
      description, description_entities JSON, type ENUM('regular',
      'quiz'), is_anonymous, allows_multiple_answers,
      allows_revoting, correct_option_id, explanation, explanation_
      entities, open_period, close_date, is_closed,
      total_voter_count, created_at, updated_at
    telegram_poll_options
      poll_id, position, option_id, text, entities JSON,
      voter_count, author_user_id NULL, addition_date

The message row's `content_extra` is `{"poll_id":..}`. `updatePoll`
updates the latest results in place; the results are not versioned
(the edit history stays about what the message said). Votes of
individual users are not available to a user account for anonymous
polls and come only to bots otherwise, so they are not stored. See
decision [2026-11-18-msg-polls](decisions/2026-11-18-msg-polls.md).

### 4.5 Forum topics

Group rows gain `topic_kind ENUM('forum','thread','direct','saved')
NULL` and `topic_id BIGINT NULL` from `message.topic_id`, indexed with
`chat_id`. A table `telegram_forum_topics (chat_id, topic_id, name,
icon_color, icon_custom_emoji_id, is_closed, is_hidden, creation_date,
creator_user_id NULL, ...)` is filled from `updateForumTopicInfo`,
`getForumTopics` once per forum, and the topic service messages.

### 4.6 Message-level metadata (`meta`)

A nullable `meta JSON` column on both message tables (not snapshotted
on edits) for the fields of section 3.6 that are rare and not queried:
via bot, reply quote, reply to a story or a checklist task or poll
option, cross-chat reply origin, forward source, reply markup, effect,
self-destruct timer, paid star count, sender boosts and tag, fact
check, import info, restriction info. `is_pinned TINYINT` becomes a
column, kept current by `updateMessageIsPinned`.

### 4.7 Views and reactions (needs a decision)

`interaction_info` (views, forwards, reply count, reaction totals)
changes constantly on busy channels. Proposed: columns `view_count`,
`forward_count`, `reply_count` and `reactions JSON` (latest totals per
reaction) updated in place from `updateMessageInteractionInfo` and
`updateMessageReactions`, coalesced per message so a busy channel
causes at most one write per message per minute. Who reacted with what
is available only through `getMessageAddedReactions` and only where
the chat allows it; it is left out. Whether to store these at all, and
how often to write them, is open (section 6).

### 4.8 Link previews

`telegram_link_previews` gains `description_entities JSON`,
`show_media_above_description`, and `options JSON` (the message's
`link_preview_options`: disabled, url, force small or large media,
above text).

### 4.9 Repairing old rows

Rows stored as `unknown` before the change, and service rows without
`content_extra`, are re-read with `getMessage` through the existing
rate-limited relink re-fetch path and stored again with the new
extraction. A row whose message TDLib cannot return any more keeps
what it has, with `tdlib_type` left NULL. Re-reading the media rows
too (for the detail of section 4.2) is optional and much larger
(section 6).

### 4.10 Forwarding to Discord

The new kinds are forwarded under the existing `fwd_other` flag
(migration 000031), as a short text rendering (a location as a map
link, a contact as name and phone, a poll as its question and options,
a dice as its value) plus the main file where there is one.

## 5. Web impact (Go port; the main session's work)

- The search API's `content_type` field gains the new values
  (`web/docs/search-api.md`), and `tdlib_type` and `service_type` can
  be offered as filters.
- The chat view renders each new kind from `content_extra`, the
  message-files table and the poll and topic tables; until then the
  Drogon web shows new kinds like `unknown` (it already falls back to
  that), so nothing breaks in between.
- Topics: a topic filter in the chat view of forum groups.
- The MCP exposure allowlist decides separately whether the new
  columns are exposed.

## 6. Open questions

1. Raw TDLib JSON for every message, not only for classes without a
   curated mapping: the most faithful archive and lets any later
   mapping be rebuilt from the database alone, at the cost of roughly
   one to two kilobytes per message on top of the current row size.
   Proposed: no, curated plus raw only for unmapped classes.
2. Views and reactions (section 4.7): store the latest totals, or not
   at all; and whether to keep their history.
3. Stories shared in messages (`messageStory`) and story replies: keep
   only the reference (proposed), or also fetch the story with
   `getStory` while it is still live and archive its media, which is
   a new table and more storage.
4. Repair scope (section 4.9): only `unknown` and service rows
   (proposed), or also every media row for its detail.

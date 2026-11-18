# Message types: storing every kind of Telegram message faithfully

Status: planned and approved 2026-11-18, not implemented. The items that implement it are in the "Message types" section
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

### 2.1 Expired media (fixed)

The upsert in `models/GroupMessage.cpp` (and its private twin) took the
content columns from any copy whose `edit_date` was not older than the
stored one. A self-destructing photo, video, video note or voice note
becomes `messageExpired*` once opened or timed out; when that message
was ingested again (reply resolution, backfill, relink) the stored
`photo` row was rewritten to `unknown` with no text. Since 2026-11-18
an expired copy of a stored message leaves the row alone (decision
[2026-11-18-msg-expired-media-keep](decisions/2026-11-18-msg-expired-media-keep.md));
rows overwritten before that cannot be told apart from other `unknown`
rows.

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

See decision
[2026-11-18-msg-content-kinds](decisions/2026-11-18-msg-content-kinds.md).

Both message tables have a FULLTEXT index on `text`, and with it
InnoDB can neither append ENUM members nor add a column in place: only
a copy of the table works, which took about ten hours on a synthetic
table of the archive's size. Migration 000034 therefore drops the
FULLTEXT index, makes the changes (the ENUM append is then a metadata
change and the new columns an online rebuild), and builds the index
again. It also adds every other column this design puts on the two
message tables (sections 4.5 to 4.7), so they are rebuilt only once.
See decision
[2026-11-18-msg-message-table-ddl](decisions/2026-11-18-msg-message-table-ddl.md).
The edit tables and `telegram_files` change instantly.

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

For a class with no curated mapping yet, `content_extra` holds
`{"tdlib": <the content as TDLib's own JSON>}`, produced by the JSON
serializer TDLib already builds (`tdjson_private`), with local state
removed (file paths and download state, the account's viewed and
listened marks). Until the curated items land that is every class but
`messageText` (fully kept by the text, entities and link preview),
`messageAnimatedEmoji` (stored as its text) and the expired classes
(nothing left); for good it is the rare and bot-only ones, rich
messages and widgets. Encrypted passport data is never stored, and a
content whose JSON exceeds 256 KiB keeps only its class and size
(`tdlib_omitted_bytes`). A later version can turn the JSON into a
curated object from the row alone.

`content_extra` is part of the edit snapshot: an edit that changes a
curated key (a live location moving, say) is a version. The `tdlib`
part does not count when deciding whether an edit changed anything,
since it also changes with nobody editing (refreshed file references),
and a row with no curated keys yet matches any.
See decision
[2026-11-18-msg-content-extra](decisions/2026-11-18-msg-content-extra.md).

### 4.2.1 Media keys

The curated `content_extra` of the media classes replaces their TDLib
JSON. Keys appear only when they apply: a flag only when true, a number
only when non-zero, a string only when non-empty. Ids that are 64-bit in
TDLib are JSON strings, as in TDLib's own JSON, so that JavaScript
readers do not lose precision. Files are not referenced here (section
4.3); file names stay in `telegram_files.orig_file_name`.

Shared keys:

| Key | Meaning |
|---|---|
| `width`, `height` | pixels; for a photo, its largest size |
| `duration` | seconds |
| `mime` | MIME type as sent |
| `spoiler` | true: hidden behind a spoiler |
| `caption_above` | true: caption shown above the media |
| `secret` | true: self-destructing once opened |
| `has_stickers` | true: stickers were added to the media |
| `minithumbnail` | `{"width", "height", "data"}`: TDLib's tiny inline JPEG, base64; shows a blurred preview even when the file is not stored |

Per class:

| Class | Keys |
|---|---|
| messagePhoto | width, height, minithumbnail, spoiler, caption_above, secret, has_stickers; `live_photo`: `{width, height, duration, mime}` when the photo carries a video |
| messageVideo | width, height, duration, mime, minithumbnail, spoiler, caption_above, secret, has_stickers, `streaming` (supports streaming), `start_timestamp`, `cover` (true when a cover photo was set), `qualities`: `[{width, height, codec}]` of the alternative videos |
| messageAnimation | width, height, duration, mime, minithumbnail, spoiler, caption_above, secret, has_stickers |
| messageAudio | duration, mime, `title`, `performer`, minithumbnail (album cover), `album_cover` (true when one exists) |
| messageDocument | mime, minithumbnail |
| messageVoiceNote | duration, mime, `waveform` (base64, TDLib's 5-bit samples), `transcript` (Telegram's speech recognition, when done) |
| messageVideoNote | duration, `length` (diameter in pixels), minithumbnail, waveform, transcript, secret |
| messageSticker | `emoji`, `sticker_id`, `set_id`, `format` (`webp`, `tgs`, `webm`), `sticker_type` (`regular`, `mask`, `custom_emoji`), `custom_emoji_id`, width, height, `premium` (sent as a premium sticker) |
| messagePaidMedia | `star_count`, caption_above, `items`: `[{kind, width, height, duration, minithumbnail}]` with kind `photo`, `video`, `preview` (not paid for: only its size and blur are known) or `unsupported` |

### 4.2.2 Places, people, games and calls

The same rules as section 4.2.1. Coordinates are degrees; chat and user
ids are numbers (they fit in 53 bits); call, game and other 64-bit ids
are strings; message ids are server ids.

| Class | Keys |
|---|---|
| messageLocation | `lat`, `lon`, `accuracy` (metres) |
| messageLiveLocation | lat, lon, accuracy, `live: true`, `live_period`, `heading`, `proximity_radius`; the remaining time is left out, since it changes with nobody moving |
| messageVenue | lat, lon, accuracy, `title`, `address`, `provider`, `venue_id`, `venue_type` |
| messageContact | `phone`, `first_name`, `last_name`, `vcard`, `user_id` |
| messageDice | `emoji`, `value` |
| messageStakeDice | `stake: true`, `value`, `stake_gram_amount`, `prize_gram_amount` |
| messageGame | `game_id`, `short_name`, `title`, `description`, `has_photo`, `has_animation`; the game's text is the message text |
| messageStory | `poster_chat_id`, `story_id`, `via_mention` (a reference only; decision [2026-11-18-msg-stories-reference-only](decisions/2026-11-18-msg-stories-reference-only.md)) |
| messageInvoice | `title`, `description` (`{text, entities}`), `has_photo`, `currency`, `total_amount` (in the currency's smallest unit), `start_parameter`, `is_test`, `need_shipping_address`, `receipt_message_id`, `has_paid_media` |
| messageChecklist | `tasks`: `[{id, text, entities, completed_by: {user_id} or {chat_id}, completion_date}]`, `others_can_add_tasks`, `others_can_mark_tasks_as_done`; the title is the message text |
| messageGiveaway | `boosted_chat_id`, `additional_chat_ids`, `winners_selection_date`, `only_new_members`, `has_public_winners`, `country_codes`, `prize_description`, `winner_count`, `prize`: `{premium_months}` or `{stars}` |
| messageCall | `call_id`, `video`, `duration`, `discard_reason` (`missed`, `declined`, `disconnected`, `hung_up`, `upgrade_to_group_call`) |
| messageGroupCall | `call_id`, `group: true`, `active`, `missed`, `video`, `duration`, `participants`: `[{user_id} or {chat_id}]` |

A live location's new positions arrive as content updates and edits.
Each one updates the live row, but a location keeps at most one version
a minute in the edits table, so a shared position is kept as a track
with one point a minute rather than one a second.

### 4.2.3 Service messages

A service message keeps its English description in `text` and its
action's own fields in `content_extra`, by the same rules as section
4.2.1. Message ids are server ids; user and chat ids are numbers;
64-bit ids are strings; a sender is `{user_id}` or `{chat_id}`; a price
is `{stars}` or `{gram_cents}`; a union type is named by its variant in
snake case (`"origin": "resale"`). A toggle keeps its value even when
false.

| service_type | Keys |
|---|---|
| chat_create | `title`, `user_ids` (basic groups) |
| chat_change_title | `title` |
| chat_change_photo, suggest_profile_photo | `photo`: `{photo_id, width, height, animated, minithumbnail}` |
| chat_add_members | `user_ids` |
| chat_delete_member | `user_id` |
| chat_join_from_community, chat_added_to_community | `community_id` |
| chat_upgrade_to | `supergroup_id` |
| chat_upgrade_from | `title`, `basic_group_id` |
| chat_owner_left, chat_owner_changed | `new_owner_user_id` |
| chat_has_protected_content_toggled | `old_value`, `new_value`, `request_message_id` |
| chat_has_protected_content_disable_requested | `expired` |
| pin_message | `message_id` |
| chat_set_background | `background`: `{id, name, dark, type, dark_theme_dimming}`, `old_background_message_id`, `only_for_self` |
| chat_set_theme | `theme_name`, or `theme_kind` for a gift theme |
| chat_set_message_auto_delete_time | `seconds`, `from_user_id` |
| chat_boost | `boost_count` |
| forum_topic_created | `name`, `name_implicit`, `icon_color`, `icon_custom_emoji_id` |
| forum_topic_edited | `name`, `icon_edited`, `icon_custom_emoji_id` |
| forum_topic_is_closed_toggled, forum_topic_is_hidden_toggled | `closed`, `hidden` |
| suggest_birthdate | `day`, `month`, `year` |
| game_score | `game_message_id`, `game_id`, `score` |
| managed_bot_created | `bot_user_id`, `manager_bot_user_id` |
| video_chat_scheduled, video_chat_started, video_chat_ended, invite_video_chat_participants | `group_call_id`, `start_date`, `duration`, `user_ids` |
| poll_option_added, poll_option_deleted | `poll_message_id`, `option_id`, `option` (`{text, entities}`) |
| checklist_tasks_done | `checklist_message_id`, `done_task_ids`, `not_done_task_ids` |
| checklist_tasks_added | `checklist_message_id`, `tasks` |
| payment_successful | `invoice_chat_id`, `invoice_message_id`, `currency`, `total_amount`, `subscription_until_date`, `recurring`, `first_recurring`, `invoice_name` |
| payment_successful_bot | currency, total_amount, subscription_until_date, recurring, first_recurring, `telegram_payment_charge_id`, `provider_payment_charge_id` |
| payment_refunded | `owner`, currency, total_amount, the two charge ids |
| gifted_premium, premium_gift_code | `gifter_user_id`, `receiver_user_id` or `creator`, `text`, `currency`, `amount`, `cryptocurrency`, `cryptocurrency_amount`, `month_count`, `day_count`, `from_giveaway`, `unclaimed`, `has_code` |
| gifted_stars, gifted_grams, giveaway_prize_stars | gifter and receiver, amounts, `star_count` or `gram_amount`, `transaction_id`, giveaway chat and message |
| giveaway_created, giveaway_completed, giveaway_winners | `star_count`; `giveaway_message_id`, `winner_count`, `unclaimed_prize_count`; the winners, prize and selection date |
| gift, upgraded_gift, refunded_upgraded_gift | `gift` (`{id, star_count, emoji, ...}` or, upgraded, `{id, title, name, number, owner}`), `sender`, `receiver`, `origin`, `received_gift_id`, `text`, its flags and dates |
| upgraded_gift_purchase_offer(_rejected) | `gift`, `state`, `price`, `expiration_date` or `offer_message_id`, `expired` |
| paid_messages_refunded | `message_count`, `star_count` |
| paid_message_price_changed, direct_message_price_changed | `paid_message_star_count`, `enabled` |
| suggested_post_* | `suggested_post_message_id`, `price`, `send_date`, `comment`, `star_count`, `nanostar_count`, `gram_amount`, `reason` |
| ton_wallet_transfer | `sender_user_id`, `transaction_id`, `peer_address`, `amount`, `comment` |
| ton_connect_request | `session_id`, `state`, `dapp_name`, `topic` |
| users_shared, chat_shared | `users` (`{user_id, first_name, last_name, username}`), `chat` (`{chat_id, title, username}`), `button_id` |
| bot_write_access_allowed | `reason`, `domain_name` |
| web_app_data_sent, web_app_data_received | `button_text`, `data` |
| passport_data_sent, passport_data_received | `types` |
| proximity_alert_triggered | `traveler`, `watcher`, `distance` |

Kept out on purpose: a Premium gift code (anyone who can read the
archive could redeem it; `has_code` records that there was one), an
encrypted TON comment (`comment_encrypted` instead), a bot payment's
payload and order info, encrypted passport data (only the element types
are kept), and flags that only say what this account may do. The
classes not listed carry nothing beyond their description.

### 4.3 Several files per message (`telegram_message_files`)

A message can carry more than one file: paid media (several photos or
videos), a photo with a live-photo video, a video with a cover, a poll
with media, a game with a photo, an invoice with a photo, a chat photo
change. A new table follows the pattern of `telegram_link_previews`:

    telegram_message_files (migration 000036)
      id, group_message_id NULL, private_message_id NULL,
      role ENUM('cover','live_video','paid','preview','poll','game',
      'invoice','chat_photo','thumbnail'), position SMALLINT,
      file_id (FK telegram_files, ON DELETE SET NULL),
      UNIQUE (message, role, position)

The existing `file_id` column stays the main file and keeps its
meaning, so the web and the forwarders do not change; the new table
holds the other files only (decision
[2026-11-18-msg-extra-files-only](decisions/2026-11-18-msg-extra-files-only.md)).
A video note's round video is now the main file, like any other media.

| Role | Files |
|---|---|
| `cover` | a video's cover photo (largest size) |
| `live_video` | the video of a live photo |
| `paid` | each photo or video of paid media, `position` its index among the items (previews of unpaid items have no file); an invoice's paid media at position 0 |
| `game` | a game's photo (position 0) and animation (position 1) |
| `invoice` | an invoice's product photo |
| `chat_photo` | the new photo of a chat photo change or a suggested profile photo (largest size) |
| `poll`, `preview`, `thumbnail` | reserved: poll media (msg-polls); thumbnails and a video's other qualities are not stored, since they repeat the media |

Each file goes through the same path as a main file: linked when it is
already known, recorded without its bytes where the storage cache rules
say so (`docs/design/storage-cache.md`), else downloaded. The fetcher
can fetch one again from the message, which it finds through the
table (a `MessageExtra` source with the role and position). An edit
that replaces a file overwrites its (role, position); one that drops a
file leaves the old row in place. See decision
[2026-11-18-msg-message-files](decisions/2026-11-18-msg-message-files.md).

### 4.4 Polls

A poll changes after it is sent (votes, added options, closing), and
TDLib announces it with `updatePoll` keyed by the poll id, not the
message. Two tables (migration 000037):

    telegram_polls
      id (TDLib poll id), question, question_entities JSON,
      type ENUM('regular','quiz'), is_anonymous,
      allows_multiple_answers, allows_revoting, members_only,
      country_codes JSON, correct_option_ids JSON (quiz: 0-based
      positions), explanation, explanation_entities JSON,
      media JSON, explanation_media JSON, open_period, close_date,
      is_closed, total_voter_count, recent_voters JSON
    telegram_poll_options
      poll_id, option_id (TDLib's, a string), position, text,
      entities JSON, media JSON, voter_count, vote_percentage,
      author_user_id or author_chat_id, addition_date, deleted_at

The message row's `content_extra` is `{"poll_id": "<id>", "has_media":
true}`, and its `text` is the poll's description. Each ingested poll
message and every `updatePoll` store the poll as it stands; the results
are not versioned (the edit history stays about what the message said).
An option that disappears from the poll is kept with `deleted_at` set.
`updatePoll` does not carry the poll's own media, so a stored one is
kept.

Media (`media` columns) use the curated shape of section 4.2.1 plus a
`kind` (`photo`, `video`, `animation`, `audio`, `document`, `sticker`,
`location`, `venue`, `link`). Their files are in
`telegram_message_files` with role `poll`: the poll's own media at
position 0, a quiz explanation's at 1, and option i's at 2 + i.

Left out, as they describe only this account: its own vote, whether it
may vote or see the voters, and the order its client shows the options
in. Votes of individual users are not available to a user account for
anonymous polls and come only to bots otherwise, so they are not
stored. See decision
[2026-11-18-msg-polls](decisions/2026-11-18-msg-polls.md).

### 4.5 Forum topics

Group rows have `topic_kind ENUM('forum','thread','direct','saved')
NULL` and `topic_id BIGINT NULL` for `message.topic_id`, indexed with
`chat_id` (added by migration 000034). A forum topic's id is TDLib's
`forum_topic_id`; a thread's is the server id of the thread's first
message (comments of a channel post in its discussion group); direct
and saved-messages topics keep TDLib's id. A copy of a message without
a topic leaves a stored one in place.

The topics are in `telegram_forum_topics` (migration 000039), keyed by
`(chat_id, topic_id)`: `name`, `is_name_implicit`, `icon_color`,
`icon_custom_emoji_id`, `creation_date`, `creator_user_id` or
`creator_chat_id`, `is_general`, `is_closed`, `is_hidden`, `is_pinned`.
There is no foreign key to `telegram_groups`, since topic updates can
arrive before the group row is written. Unread counts, notification
settings and drafts describe this account and are left out.

The table is filled from three sources:

- `updateForumTopicInfo`, which replaces the stored state;
- one `getForumTopics` sweep per forum per run, started when
  `updateSupergroup` says the group is a forum (at most 20 pages of
  100), which also sets `is_pinned`; `updateForumTopic` keeps it
  current;
- a "topic created" service message, which only fills in a topic not
  stored yet, since history backfill reads such messages long after
  the topic may have been renamed.

### 4.6 Message-level metadata (`meta`)

A nullable `meta JSON` column on both message tables (added by
migration 000034; not snapshotted on edits) holds the fields of section
3.6 that are rare and not queried. Every copy of a message sets it, as
it sets the identity columns: it is not content, so it follows the copy
being written rather than the edit date. `is_pinned` (also 000034) is
set the same way and kept current by `updateMessageIsPinned`;
`updateMessageFactCheck` sets or removes `meta.fact_check` alone.

| Key | Value |
|---|---|
| `via_bot_user_id` | the inline bot used |
| `business_bot_user_id` | the business bot that sent it |
| `guest_bot_caller` | `{user_id}` or `{chat_id}` |
| `reply` | `quote` (`{text, entities, position, manual}`), `checklist_task_id`, `poll_option_id`; for a reply into a chat not logged also `origin` (`{kind, user_id or chat_id, message_id, sender_name, author_signature}`), `origin_send_date` and `content` (`{tdlib_type, text}`) |
| `reply_to_story` | `{poster_chat_id, story_id}` |
| `forward_source` | where a forward was saved from: `{chat_id, message_id, sender, sender_name, date}` |
| `psa_type` | public service announcement type of a forward |
| `import` | `{sender_name, date}` of an imported message |
| `link_preview_options` | `{disabled, url, force_small_media, force_large_media, show_above_text}`, when not the defaults |
| `reply_markup` | `{kind}`: `inline_keyboard` or `keyboard` with `rows` of `{text, type, url, query, user_id, copy_text}`, `force_reply`, `remove_keyboard` |
| `effect_id` | the message effect |
| `self_destruct_time`, `self_destruct_immediately` | the self-destruct timer |
| `paid_message_star_count` | stars paid to send it |
| `sender_boost_count`, `sender_tag` | the sender's boosts and tag |
| `fact_check` | `{text, entities, country_code}` |
| `restriction` | `{reason, sensitive}` |
| `suggested_post` | `{price, send_date, state}` |

Left out: bot callback data (the bot's own payload), the time left
before a self-destruct or auto-delete (it changes by the second), and
what describes this account (`can_*`, unread marks, `is_outgoing` of a
forward source). A plain reply in the same chat needs no `reply` key:
the reply columns hold it.

### 4.7 Views and reactions

`interaction_info` (views, forwards, reply count, reaction totals)
changes constantly on busy channels. The columns `view_count`,
`forward_count`, `reply_count` and `reactions JSON` (latest totals per
reaction; added by migration 000034) are updated in place from `updateMessageInteractionInfo` and
`updateMessageReactions`, coalesced per message so a busy channel
causes at most one write per message per minute. Who reacted with what
is available only through `getMessageAddedReactions` and only where
the chat allows it; it is left out, and so is the history of the
counters (decision
[2026-11-18-msg-interaction-latest-totals](decisions/2026-11-18-msg-interaction-latest-totals.md)).
The coalescing interval is set after measuring the update rate.

### 4.8 Link previews

`telegram_link_previews` gains `description_entities JSON`,
`show_media_above_description` and `skip_confirmation` (migration
000041; instant, as the table has no FULLTEXT index). The message's
`link_preview_options` (turned off, another URL, small or large media,
above the text) go into its `meta` as `link_preview_options`, not onto
the card: a message with previews turned off has no card (decision
[2026-11-18-msg-link-preview-options-in-meta](decisions/2026-11-18-msg-link-preview-options-in-meta.md)).

### 4.9 Repairing old rows

Rows stored before classification as `unknown`, and service rows
without their detail, are re-read with `getMessage` and stored again
through the normal path, which classifies them. The pass is opt-in: it
runs at startup when `TG_REPAIR_MESSAGES_ON_START` is set, and `dry`
only logs how many rows it would re-read, per table and kind. It shares
the relink re-fetch's pace (10 `getMessage` a second for both).

A row qualifies while it is live, has no `tdlib_type`, and is
`unknown` or `service`. A re-read row gets its `tdlib_type`, so a pass
cut short by a restart resumes where it was without any saved state.
A row whose message TDLib cannot return any more keeps what it has and
is tried again on every start while the flag is on. Media rows are not
re-read for the detail of section 4.2 (decision
[2026-11-18-msg-repair-scope](decisions/2026-11-18-msg-repair-scope.md)).
The local re-map of rows holding only TDLib's JSON, once planned here,
is dropped (decision
[2026-11-18-msg-repair-opt-in](decisions/2026-11-18-msg-repair-opt-in.md)).

### 4.10 Forwarding to Discord

The new kinds are forwarded under the existing `fwd_other` flag
(migration 000031). A kind with nothing to read but its structure is
forwarded as a short plain-text line after its text or caption, which
keeps the caption's formatting offsets valid:

| Kind | Line |
|---|---|
| location, live location | `Location: <map link>`, `Live location: <map link>` (OpenStreetMap) |
| venue | `Venue: <title>, <address>` and a map link |
| contact | `Contact: <name>, <phone>` |
| dice | `Dice <emoji>: <value>` |
| poll | `Poll: <question>` (or `Quiz:`) and one `- <option>` line per option |
| game | `Game: <title>` |
| story | `Shared a story` |
| invoice | `Invoice: <title> (<currency>)` |
| checklist | `Checklist:` and one `[x]` or `[ ]` line per task, under the title (the message text) |
| call | `Call, 1:23`, `Video call`, `Missed call`, `Declined video call`; `Group call, 1:02:05` |
| giveaway | `Giveaway: <n> winners` |
| paid media | `Paid media: <n> items, <stars> stars` |

A video note forwards its round video as its file, like any other
media; paid media has no main file and forwards its line only.

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

## 6. Settled questions

The questions left open by the first draft were settled on 2026-11-18:

1. Raw TDLib JSON is stored only for classes without a curated
   mapping, not for every message
   ([2026-11-18-msg-raw-json-unmapped-only](decisions/2026-11-18-msg-raw-json-unmapped-only.md)).
2. Views, forwards, replies and reactions are stored as latest totals,
   without history (section 4.7).
3. Shared and replied stories are kept as references only; stories are
   not fetched or archived
   ([2026-11-18-msg-stories-reference-only](decisions/2026-11-18-msg-stories-reference-only.md)).
4. The repair pass covers `unknown` and service rows only (section
   4.9).

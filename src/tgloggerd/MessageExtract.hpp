// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MESSAGE_EXTRACT_HPP
#define TGLOGGERD__MESSAGE_EXTRACT_HPP

#include <tgloggerd/models/Message.hpp>
#include <tgloggerd/models/Poll.hpp>
#include <tgloggerd/models/ForumTopic.hpp>

#include <td/telegram/td_api.h>

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

namespace tgloggerd {

namespace td_api = td::td_api;

/*
 * Every message content class of the TDLib in submodules/td, with the
 * content kind it is stored as (docs/design/message-types.md section 3):
 * X(<td_api class>, <MessageContentType>). A class TDLib adds later is not
 * in the list; it is stored as Unknown, still with its class name.
 */
#define TGLOGGERD_MESSAGE_CONTENT_CLASSES(X) \
	X(messageText, Text) \
	X(messageRichMessage, Rich) \
	X(messageAnimation, Animation) \
	X(messageAudio, Audio) \
	X(messageDocument, Document) \
	X(messagePaidMedia, PaidMedia) \
	X(messagePhoto, Photo) \
	X(messageSticker, Sticker) \
	X(messageVideo, Video) \
	X(messageVideoNote, VideoNote) \
	X(messageVoiceNote, Voice) \
	X(messageExpiredPhoto, Expired) \
	X(messageExpiredVideo, Expired) \
	X(messageExpiredVideoNote, Expired) \
	X(messageExpiredVoiceNote, Expired) \
	X(messageLiveLocation, Location) \
	X(messageLocation, Location) \
	X(messageVenue, Venue) \
	X(messageContact, Contact) \
	X(messageAnimatedEmoji, Text) \
	X(messageDice, Dice) \
	X(messageGame, Game) \
	X(messagePoll, Poll) \
	X(messageStakeDice, Dice) \
	X(messageStory, Story) \
	X(messageWidget, Widget) \
	X(messageChecklist, Checklist) \
	X(messageInvoice, Invoice) \
	X(messageCall, Call) \
	X(messageGroupCall, Call) \
	X(messageVideoChatScheduled, Service) \
	X(messageVideoChatStarted, Service) \
	X(messageVideoChatEnded, Service) \
	X(messageInviteVideoChatParticipants, Service) \
	X(messagePollOptionAdded, Service) \
	X(messagePollOptionDeleted, Service) \
	X(messageBasicGroupChatCreate, Service) \
	X(messageSupergroupChatCreate, Service) \
	X(messageChatChangeTitle, Service) \
	X(messageChatChangePhoto, Service) \
	X(messageChatDeletePhoto, Service) \
	X(messageChatOwnerLeft, Service) \
	X(messageChatOwnerChanged, Service) \
	X(messageChatHasProtectedContentToggled, Service) \
	X(messageChatHasProtectedContentDisableRequested, Service) \
	X(messageChatAddMembers, Service) \
	X(messageChatJoinByLink, Service) \
	X(messageChatJoinByRequest, Service) \
	X(messageChatJoinFromCommunity, Service) \
	X(messageChatDeleteMember, Service) \
	X(messageChatAddedToCommunity, Service) \
	X(messageChatRemovedFromCommunity, Service) \
	X(messageChatUpgradeTo, Service) \
	X(messageChatUpgradeFrom, Service) \
	X(messagePinMessage, Service) \
	X(messageScreenshotTaken, Service) \
	X(messageChatSetBackground, Service) \
	X(messageChatSetTheme, Service) \
	X(messageChatSetMessageAutoDeleteTime, Service) \
	X(messageChatBoost, Service) \
	X(messageForumTopicCreated, Service) \
	X(messageForumTopicEdited, Service) \
	X(messageForumTopicIsClosedToggled, Service) \
	X(messageForumTopicIsHiddenToggled, Service) \
	X(messageSuggestProfilePhoto, Service) \
	X(messageSuggestBirthdate, Service) \
	X(messageCustomServiceAction, Service) \
	X(messageGameScore, Service) \
	X(messageManagedBotCreated, Service) \
	X(messagePaymentSuccessful, Service) \
	X(messagePaymentSuccessfulBot, Service) \
	X(messagePaymentRefunded, Service) \
	X(messageGiftedPremium, Service) \
	X(messagePremiumGiftCode, Service) \
	X(messageGiveawayCreated, Service) \
	X(messageGiveaway, Giveaway) \
	X(messageGiveawayCompleted, Service) \
	X(messageGiveawayWinners, Service) \
	X(messageGiftedStars, Service) \
	X(messageGiftedGrams, Service) \
	X(messageGiveawayPrizeStars, Service) \
	X(messageTonWalletTransfer, Service) \
	X(messageTonConnectRequest, Service) \
	X(messageGift, Service) \
	X(messageUpgradedGift, Service) \
	X(messageRefundedUpgradedGift, Service) \
	X(messageUpgradedGiftPurchaseOffer, Service) \
	X(messageUpgradedGiftPurchaseOfferRejected, Service) \
	X(messagePaidMessagesRefunded, Service) \
	X(messagePaidMessagePriceChanged, Service) \
	X(messageDirectMessagePriceChanged, Service) \
	X(messageUnpaidMessagesAllowedByLink, Service) \
	X(messageChecklistTasksDone, Service) \
	X(messageChecklistTasksAdded, Service) \
	X(messageSuggestedPostApprovalFailed, Service) \
	X(messageSuggestedPostApproved, Service) \
	X(messageSuggestedPostDeclined, Service) \
	X(messageSuggestedPostPaid, Service) \
	X(messageSuggestedPostRefunded, Service) \
	X(messageContactRegistered, Service) \
	X(messageUsersShared, Service) \
	X(messageChatShared, Service) \
	X(messageBotWriteAccessAllowed, Service) \
	X(messageWebAppDataSent, Service) \
	X(messageWebAppDataReceived, Service) \
	X(messagePassportDataSent, Service) \
	X(messagePassportDataReceived, Service) \
	X(messageProximityAlertTriggered, Service) \
	X(messageUnsupported, Unsupported)

/* The td_api class name of a MessageContent constructor id, e.g.
 * "messageVideoNote", or nullptr for a class not in the list above. */
const char *message_content_class_name(std::int32_t id);

/*
 * Map a td_api message content to the MessageContent stored for both
 * private and group messages: the content type, the text or caption with
 * its formatting entities, the exact class (tdlib_type), for system
 * messages the service type and a human-readable description, and
 * content_extra: for a class without a curated mapping, TDLib's own JSON of
 * the content under the key "tdlib", without local file paths. A rich
 * message gets its blocks' plain text and keeps TDLib's JSON beside its
 * curated keys. Media files are not linked here.
 *
 * @msg is the message around @content, when there is one; describing a
 * service message needs it (it reports who acted). updateMessageContent
 * has no message to give, but service messages are not edited.
 */
void extract_message_content(const td_api::MessageContent &content,
			     models::MessageContent &out,
			     const td_api::message *msg = nullptr);

/* The same, for callers that have the message. */
void extract_message_content(const td_api::message &message,
			     models::MessageContent &out);

/*
 * The plain text of a rich message's page blocks, one line per heading,
 * paragraph, caption, list item, table row and so on, in reading order;
 * empty when the blocks show no text. Stored as the message's text.
 */
std::string rich_message_text(const td_api::richMessage &m);

/*
 * Whether @content is a rich message TDLib delivered without all of its
 * blocks (richMessage.is_full false), so getFullRichMessage has the rest.
 */
bool rich_message_is_partial(const td_api::MessageContent &content);

/* The message's text or caption with its entities, or nullptr for content
 * that has none. Mirrors extract_message_content's text sources. */
const td_api::formattedText *
message_formatted_text(const td_api::message &message);

/*
 * One of a message's files beyond its main one (telegram_message_files):
 * the file, its telegram_files.file_type, its name if it has one, and its
 * role and position in the table.
 */
struct ExtraFile {
	const td_api::file	*file;
	const char		*category;
	std::string		file_name;
	const char		*role;
	int			position;
};

/*
 * The files of @content beyond its main one, in a stable order: a video's
 * cover, a live photo's video, each paid media item, a game's photo and
 * animation, an invoice's photo and paid media, the photo of a chat photo
 * change or suggestion and its video, a chat background's wallpaper or
 * pattern, the still thumbnail of a video, an animation, a video note or
 * a video sticker, the media in a rich message's blocks, a link
 * preview's own medium (link_preview_media_files), the stickers of a
 * gift (an upgraded gift's model and symbol), of a gifted Premium, Stars
 * or grams, of a giveaway prize and of a giveaway (role "gift").
 */
std::vector<ExtraFile> message_extra_files(const td_api::MessageContent &content);

/*
 * The image of a link preview, its telegram_link_previews.photo_file_id
 * (nullptr when it has none), and in *@type_name the preview's type, the
 * telegram_link_previews.type value ("unsupported" for a type not known).
 */
const td_api::file *link_preview_image(const td_api::LinkPreviewType *t,
				       const char **type_name);

/*
 * A link preview's own media, role "preview_media" (message-types.md
 * section 4.3): at position 0 the medium of a video, animation, audio,
 * document, sticker, video note or voice note preview, the cached medium
 * of an embedded player, a background's document, a story album's video
 * icon or a gift auction's sticker; at position 1 a video preview's
 * cover; an album's items, and the first stickers of a sticker set or
 * gift collection, at their index; an upgraded gift's model sticker at
 * 0 and symbol sticker at 1. Its image is not listed; it goes to
 * telegram_link_previews.photo_file_id.
 */
std::vector<ExtraFile> link_preview_media_files(const td_api::linkPreview &lp);

/*
 * The curated telegram_link_previews.extra of @lp: the fields of its type
 * that have no column (message-types.md section 4.8), as a JSON object,
 * or nullopt when the type carries none.
 */
std::optional<std::string> link_preview_extra_json(const td_api::linkPreview &lp);

/* The message a Telegram message link names: a chat id (t.me/c/ links)
 * or a public username, and the message's server id. */
struct MessageLinkTarget {
	int64_t		chat_id = 0;
	std::string	username;
	int64_t		message_id = 0;
};

/* Parse @url as a t.me (telegram.me, telegram.dog) or tg:// message
 * link into @out; false, leaving @out alone, when it is not one. */
bool telegram_message_link_target(const std::string &url,
				  MessageLinkTarget &out);

/*
 * The media in a rich message's page blocks, role "rich", position their
 * order of appearance (depth first): each photo (largest size), video,
 * animation, audio file, document and voice note, an embedded page's
 * poster, an embedded post's author photo and the photos of related
 * articles, including those inside collages, slideshows, quotes, lists,
 * collapsible blocks, covers and embedded posts.
 */
std::vector<ExtraFile> rich_message_files(const td_api::richMessage &m);

/* The one of @files whose Telegram unique id is @unique_id, or nullptr.
 * An edit can move a rich message's media, so a file is found again by
 * its unique id rather than its position. */
const ExtraFile *rich_file_with_unique_id(const std::vector<ExtraFile> &files,
					  const std::string &unique_id);

/*
 * A TDLib poll as telegram_polls and telegram_poll_options keep it: what
 * every reader of the chat sees, without this account's own vote and
 * permissions.
 */
models::Poll extract_poll(const td_api::poll &p);

/* The same for the poll of a message, with the poll's own media, which
 * the message carries and updatePoll does not. */
models::Poll extract_poll(const td_api::messagePoll &m);

/*
 * The topic of a message (td_api::message.topic_id) as stored on group
 * messages: @kind "forum", "thread", "direct" or "saved" and @id within the
 * chat, a thread's as the server id of its first message. Returns false,
 * leaving both alone, when the message has no topic.
 */
bool message_topic(const td_api::MessageTopic *t, std::string &kind,
		   int64_t &id);

/* A forum topic as TDLib describes it now (updateForumTopicInfo,
 * getForumTopics). */
models::ForumTopic extract_forum_topic(const td_api::forumTopicInfo &info);

/*
 * The forum topic a "topic created" service message describes, as it was
 * created: its name and icon, the message's date and sender, and the topic
 * id the message itself carries. nullopt for any other message, or one
 * without a forum topic id.
 */
std::optional<models::ForumTopic>
forum_topic_from_created(const td_api::message &message);

/*
 * The message-level detail stored in a message row's `meta` column
 * (docs/design/message-types.md section 4.6), or nullopt when the message
 * has none: the inline bot used, a reply's quote and target details, the
 * story replied to, where a forward was saved from, import info, the
 * keyboard, the message effect, the self-destruct timer, paid stars, the
 * sender's boosts and tag, fact check, restriction and suggested post
 * state. Bot callback data and what counts down with time are left out.
 */
std::optional<std::string> message_meta_json(const td_api::message &m);

/* The entities of a formattedText as stored JSON, or nullopt for none. */
std::optional<std::string>
formatted_text_entities_json(const td_api::formattedText &ft);

/* A fact check as the "fact_check" value of meta, or nullopt for none. */
std::optional<std::string> fact_check_json(const td_api::factCheck *fc);

/*
 * A short plain-text rendering of content that has no text of its own to
 * forward, for the Discord forwarder: a location as a map link, a contact
 * as its name and phone, a poll as its question and options, and so on
 * for venues, dice, games, shared stories, invoices, checklists, calls,
 * giveaways and paid media. Empty for every other class, whose text (or
 * caption) and file say it all.
 */
std::string forward_summary(const td_api::MessageContent &c);

/*
 * A message's interaction totals (views, forwards, replies, reaction
 * totals), or nullopt when TDLib reports none. Reactions that are Saved
 * Messages tags are this account's own labels and are left out.
 */
std::optional<models::Interaction>
extract_interaction(const td_api::messageInteractionInfo *info);

/* Reaction totals as the stored JSON array, or nullopt for none. */
std::optional<std::string>
reactions_json(const std::vector<td_api::object_ptr<td_api::messageReaction>> &r);

/* The largest size of a photo, or nullptr. */
const td_api::file *photo_best_size(const td_api::photo *p);

/* The largest of a list of photo sizes, or nullptr. */
const td_api::file *chat_photo_best_size_of(
	const std::vector<td_api::object_ptr<td_api::photoSize>> &sizes);

/* The largest size of a chat photo, or nullptr. */
const td_api::file *chat_photo_best_size(const td_api::chatPhoto *p);

/* The video of an animated chat photo (the big animation, else the
 * small one), or nullptr for a still photo. */
const td_api::file *chat_photo_animation(const td_api::chatPhoto *p);

/* The user id of a message's sender, or 0 if the sender is a chat or
 * channel, or absent. */
int64_t sender_user_id(const td_api::message &message);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MESSAGE_EXTRACT_HPP */

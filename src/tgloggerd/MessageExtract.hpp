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
 * the content under the key "tdlib", without local file paths. Media files
 * are not linked here.
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
 * change or suggestion.
 */
std::vector<ExtraFile> message_extra_files(const td_api::MessageContent &content);

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

/* The largest size of a photo, or nullptr. */
const td_api::file *photo_best_size(const td_api::photo *p);

/* The largest size of a chat photo, or nullptr. */
const td_api::file *chat_photo_best_size(const td_api::chatPhoto *p);

/* The user id of a message's sender, or 0 if the sender is a chat or
 * channel, or absent. */
int64_t sender_user_id(const td_api::message &message);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MESSAGE_EXTRACT_HPP */

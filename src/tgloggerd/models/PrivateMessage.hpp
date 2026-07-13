// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP
#define TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP

#include <string>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * Coarse content type of a private message.
 */
enum class PrivateMessageContentType {
	Text,
	Photo,
	Video,
	Document,
	Audio,
	Voice,
	Sticker,
	Animation,
	Unknown,
};

/*
 * Kind of message origin for forwarded messages, mirroring
 * td_api::MessageOrigin subtypes.
 */
enum class ForwardOriginType {
	User,         /* messageOriginUser */
	HiddenUser,   /* messageOriginHiddenUser */
	Chat,         /* messageOriginChat */
	Channel,      /* messageOriginChannel */
};

/*
 * Forward information extracted from td_api::messageForwardInfo.
 */
struct ForwardInfo {
	ForwardOriginType	origin_type = ForwardOriginType::User;

	/* Original sender user id (messageOriginUser). */
	std::optional<int64_t>	origin_sender_user_id;

	/* Sender name (messageOriginHiddenUser) or author_signature. */
	std::optional<std::string>	origin_sender_name;

	/* Original chat/channel id (messageOriginChat/Channel). */
	std::optional<int64_t>	origin_chat_id;

	/* Original message id (messageOriginChannel). */
	std::optional<int64_t>	origin_message_id;

	/* Unix timestamp from messageForwardInfo.date_. */
	int32_t			origin_date = 0;
};

/*
 * A private-chat message, as stored in the private_messages,
 * private_message_edits, and private_message_fwd_info tables.
 */
struct PrivateMessage {
	int64_t		chat_id = 0;
	int64_t		message_id = 0;
	int64_t		sender_id = 0;
	bool		is_outgoing = false;
	int32_t		date = 0;
	int32_t		edit_date = 0;
	PrivateMessageContentType	content_type = PrivateMessageContentType::Unknown;

	/* Message text; nullopt for non-text messages. */
	std::optional<std::string>	text;

	/* files.id for media attachments; nullopt if none. */
	std::optional<uint64_t>		file_id;

	bool		is_deleted = false;

	/* Forward information; nullopt if not forwarded. */
	std::optional<ForwardInfo>	forward_info;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP */

// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__MESSAGE_HPP
#define TGLOGGERD__MODELS__MESSAGE_HPP

#include <string>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * Coarse content type of a message. Shared by private and group
 * messages, which store it in the identical content_type ENUM column.
 */
enum class MessageContentType {
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
 * Forward information extracted from td_api::messageForwardInfo. Stored
 * in the *_message_fwd_info tables, whose layout is identical for
 * private and group messages.
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
 * The mutable content of a message: everything an edit can change. It is
 * snapshotted verbatim into the *_message_edits tables before each edit,
 * so private and group messages share this layout exactly.
 */
struct MessageContent {
	MessageContentType	content_type = MessageContentType::Unknown;

	/* Message text; nullopt for non-text messages. */
	std::optional<std::string>	text;

	/* files.id for media attachments; nullopt if none. */
	std::optional<uint64_t>		file_id;
};

/* Textual encodings used by the *_messages tables' ENUM columns. */
const char *to_string(MessageContentType t);
const char *to_string(ForwardOriginType t);

/* Inverse of to_string(MessageContentType); unknown text maps to Unknown. */
MessageContentType message_content_type_from_string(const std::string &s);

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__MESSAGE_HPP */

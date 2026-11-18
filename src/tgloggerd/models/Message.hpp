// SPDX-License-Identifier: GPL-2.0-or-later
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
 * messages, which store it in the identical content_type ENUM column;
 * the order follows the ENUM (migration 000034 appended the kinds after
 * Unknown). docs/design/message-types.md section 3 says which TDLib
 * classes map to which kind.
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
	Service,	/* a system message (member joined, title changed, ...) */
	Unknown,	/* a TDLib class newer than this code */
	VideoNote,
	Location,	/* static or live */
	Venue,
	Contact,
	Dice,
	Game,
	Poll,
	Checklist,
	Story,		/* a shared story */
	PaidMedia,
	Invoice,
	Giveaway,
	Call,		/* a private call or a group call */
	Rich,		/* messageRichMessage */
	Widget,
	Expired,	/* self-destructed media, first seen already expired */
	Unsupported,	/* messageUnsupported: TDLib cannot read it either */
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
	int64_t			origin_date = 0;
};

/*
 * The mutable content of a message: everything an edit can change. It is
 * snapshotted into the *_message_edits tables before each edit, so private
 * and group messages share this layout exactly.
 *
 * Exception: service_type is not part of the edit snapshot (service
 * messages are not edited); the edits tables have no such column.
 */
struct MessageContent {
	MessageContentType	content_type = MessageContentType::Unknown;

	/* Message text or media caption; nullopt when there is none. */
	std::optional<std::string>	text;

	/*
	 * Rich-text formatting of `text`, serialized as a JSON array of TDLib
	 * text entities (see extract_message_content). nullopt when the text
	 * carries no formatting. Snapshotted alongside `text` on edits.
	 */
	std::optional<std::string>	entities;

	/*
	 * For a Service message, the precise action id (e.g.
	 * "chat_add_members"); nullopt otherwise. Not snapshotted on edits.
	 */
	std::optional<std::string>	service_type;

	/* telegram_files.id for media attachments; nullopt if none. */
	std::optional<uint64_t>		file_id;

	/* The exact TDLib content class, e.g. "messageVideoNote". */
	std::optional<std::string>	tdlib_type;

	/*
	 * Per-class detail as a JSON object (docs/design/message-types.md
	 * section 4.2): curated keys, and TDLib's own JSON under "tdlib" for
	 * what has no curated mapping yet. Snapshotted on edits; only the
	 * curated keys count when deciding whether an edit changed anything,
	 * since the TDLib JSON also carries state that changes with nobody
	 * editing (refreshed file references, for one).
	 */
	std::optional<std::string>	content_extra;

	/*
	 * The copy is self-destructed media (messageExpired*): Telegram has
	 * removed what it showed, so it says nothing about the content. A
	 * stored message keeps the content it had. Not snapshotted on edits.
	 */
	bool				is_expired = false;
};

/* Textual encodings used by the *_messages tables' ENUM columns. */
const char *to_string(MessageContentType t);
const char *to_string(ForwardOriginType t);

/* Inverse of to_string(MessageContentType); unknown text maps to Unknown. */
MessageContentType message_content_type_from_string(const std::string &s);

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__MESSAGE_HPP */

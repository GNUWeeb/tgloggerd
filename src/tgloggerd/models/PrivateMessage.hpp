// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP
#define TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP

#include <string>
#include <cstdint>
#include <optional>

#include <tgloggerd/models/Message.hpp>

namespace tgloggerd {
namespace models {

/*
 * A private-chat message, as stored in the telegram_private_messages,
 * telegram_private_message_edits, and telegram_private_message_fwd_info tables. Its
 * chat_id is the peer user's id (a valid telegram_users.id).
 */
struct PrivateMessage {
	int64_t		chat_id = 0;
	int64_t		message_id = 0;

	/* Sender user id; nullopt for messages sent by the own account. */
	std::optional<int64_t>	sender_id;

	bool		is_outgoing = false;
	int64_t		date = 0;
	int64_t		edit_date = 0;

	/* Album (media group) id shared by messages sent together; nullopt
	 * when the message is not part of an album. */
	std::optional<int64_t>	media_album_id;

	/* Content that an edit can change (type, text, file). */
	MessageContent	content;

	bool		is_deleted = false;

	/* Forward information; nullopt if not forwarded. */
	std::optional<ForwardInfo>	forward_info;

	/* Message-level detail without a column of its own, as a JSON object
	 * (docs/design/message-types.md section 4.6); nullopt if none. Not
	 * part of the content, so not snapshotted on edits. */
	std::optional<std::string>	meta;

	/* Pinned in the chat, as last seen. */
	bool				is_pinned = false;

	/* Interaction totals carried by this copy; nullopt if none. */
	std::optional<Interaction>	interaction;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__PRIVATE_MESSAGE_HPP */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MESSAGE_EXTRACT_HPP
#define TGLOGGERD__MESSAGE_EXTRACT_HPP

#include <tgloggerd/models/Message.hpp>

#include <td/telegram/td_api.h>

#include <cstdint>

namespace tgloggerd {

namespace td_api = td::td_api;

/*
 * Map a td_api message content to the MessageContent stored for both
 * private and group messages: the content type, the text or caption with
 * its formatting entities, and, for system messages, the service type and
 * a human-readable description. Media files are not linked here.
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

/* The user id of a message's sender, or 0 if the sender is a chat or
 * channel, or absent. */
int64_t sender_user_id(const td_api::message &message);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MESSAGE_EXTRACT_HPP */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__FORUM_TOPIC_HPP
#define TGLOGGERD__MODELS__FORUM_TOPIC_HPP

#include <string>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * A topic of a forum group (telegram_forum_topics), keyed by the chat and
 * TDLib's forum topic id. Only what every member sees; unread counts and
 * this account's settings are not part of it.
 */
struct ForumTopic {
	int64_t			chat_id = 0;
	int32_t			topic_id = 0;
	std::string		name;
	bool			is_name_implicit = false;
	int32_t			icon_color = 0;
	std::optional<int64_t>	icon_custom_emoji_id;
	int64_t			creation_date = 0;
	std::optional<int64_t>	creator_user_id;
	std::optional<int64_t>	creator_chat_id;
	bool			is_general = false;
	bool			is_closed = false;
	bool			is_hidden = false;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__FORUM_TOPIC_HPP */

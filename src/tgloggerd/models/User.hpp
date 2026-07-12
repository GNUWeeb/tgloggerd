// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__USER_HPP
#define TGLOGGERD__MODELS__USER_HPP

#include <string>
#include <vector>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * Mirrors td_api::UserType.
 */
enum class UserType {
	Regular,
	Deleted,
	Bot,
	Unknown,
};

/*
 * Bot-specific attributes (td_api::userTypeBot). Present only when a
 * user's type is Bot. Maps to the user_bot_info table.
 */
struct BotInfo {
	bool		can_be_edited = false;
	bool		can_join_groups = false;
	bool		can_read_all_group_messages = false;
	bool		has_main_web_app = false;
	bool		has_topics = false;
	bool		allows_users_to_create_topics = false;
	bool		can_manage_bots = false;
	bool		is_inline = false;
	std::string	inline_query_placeholder;
	bool		supports_guest_queries = false;
	bool		is_guard = false;
	bool		need_location = false;
	bool		can_connect_to_business = false;
	bool		can_be_added_to_attachment_menu = false;
	int32_t		active_user_count = 0;
};

/*
 * A snapshot of a Telegram user (td_api::user), as stored in the users,
 * user_usernames and user_bot_info tables. Uses only plain types so it
 * can be shared between the TDLib and database layers.
 */
struct User {
	int64_t		id = 0;
	std::string	first_name;
	std::string	last_name;
	std::string	phone_number;
	UserType	type = UserType::Unknown;
	std::string	language_code;

	/* files.id of the current profile photo; resolved after download. */
	std::optional<uint64_t>	profile_photo_file_id;

	int32_t		accent_color_id = 0;
	int64_t		background_custom_emoji_id = 0;
	int32_t		profile_accent_color_id = -1;
	int64_t		profile_background_custom_emoji_id = 0;

	std::optional<int64_t>	emoji_status_custom_emoji_id;
	std::optional<int32_t>	emoji_status_expiration_date;

	bool		is_contact = false;
	bool		is_mutual_contact = false;
	bool		is_close_friend = false;

	bool		is_verified = false;
	bool		is_scam = false;
	bool		is_fake = false;

	bool		is_premium = false;
	bool		is_support = false;

	std::string	restriction_reason;
	bool		has_sensitive_content = false;

	bool		restricts_new_chats = false;
	int64_t		paid_message_star_count = 0;
	bool		have_access = true;
	bool		added_to_attachment_menu = false;

	/* td_api::usernames */
	std::vector<std::string>	active_usernames;
	std::vector<std::string>	disabled_usernames;
	std::string			editable_username;
	std::vector<std::string>	collectible_usernames;

	/* Set when type == Bot. */
	std::optional<BotInfo>		bot;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__USER_HPP */

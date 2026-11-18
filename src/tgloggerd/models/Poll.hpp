// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__POLL_HPP
#define TGLOGGERD__MODELS__POLL_HPP

#include <string>
#include <vector>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/* One option of a poll, as telegram_poll_options keeps it. */
struct PollOption {
	std::string			option_id;	/* TDLib's id, unique in the poll */
	int				position = 0;	/* among the current options */
	std::string			text;
	std::optional<std::string>	entities;	/* JSON array */
	std::optional<std::string>	media;		/* curated JSON object */
	int				voter_count = 0;
	int				vote_percentage = 0;
	std::optional<int64_t>		author_user_id;
	std::optional<int64_t>		author_chat_id;
	int64_t				addition_date = 0;
};

/*
 * A poll or quiz with its latest results (telegram_polls), keyed by TDLib's
 * poll id. Only what every reader of the chat sees: this account's own vote
 * and permissions are not part of it. The JSON members are serialized
 * already, ready for their JSON columns.
 */
struct Poll {
	int64_t				id = 0;
	std::string			question;
	std::optional<std::string>	question_entities;
	bool				is_quiz = false;
	bool				is_anonymous = true;
	bool				allows_multiple_answers = false;
	bool				allows_revoting = false;
	bool				members_only = false;
	std::optional<std::string>	country_codes;
	std::optional<std::string>	correct_option_ids;
	std::optional<std::string>	explanation;
	std::optional<std::string>	explanation_entities;
	std::optional<std::string>	media;
	std::optional<std::string>	explanation_media;
	int				open_period = 0;
	int64_t				close_date = 0;
	bool				is_closed = false;
	int				total_voter_count = 0;
	std::optional<std::string>	recent_voters;
	std::vector<PollOption>		options;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__POLL_HPP */

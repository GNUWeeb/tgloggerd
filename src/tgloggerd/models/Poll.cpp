// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <vector>
#include <optional>

namespace tgloggerd {

namespace {

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

mysql::Param opt(const std::optional<std::string> &v)
{
	if (v.has_value())
		return *v;
	return std::monostate{};
}

mysql::Param opt(const std::optional<int64_t> &v)
{
	if (v.has_value())
		return *v;
	return std::monostate{};
}

} /* namespace */

/*
 * Store a poll as it stands now: the poll row and every current option,
 * inserted or updated in place, and the options it no longer has marked
 * deleted (an option removed from a poll is kept, like a deleted message).
 * Called for the poll of each ingested message and for every updatePoll,
 * so the results are always the latest seen. updatePoll does not carry the
 * poll's own media (the message does), so a poll without media keeps the
 * one already stored.
 */
void DB::upsertPoll(const models::Poll &p)
{
	db_.transaction([&](mysql::Transaction &tx) {
		tx.execute(
			"INSERT INTO telegram_polls ("
			" id, question, question_entities, type, is_anonymous,"
			" allows_multiple_answers, allows_revoting, members_only,"
			" country_codes, correct_option_ids, explanation,"
			" explanation_entities, media, explanation_media,"
			" open_period, close_date, is_closed, total_voter_count,"
			" recent_voters"
			") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,"
			" ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
			" question = new.question,"
			" question_entities = new.question_entities,"
			" type = new.type,"
			" is_anonymous = new.is_anonymous,"
			" allows_multiple_answers = new.allows_multiple_answers,"
			" allows_revoting = new.allows_revoting,"
			" members_only = new.members_only,"
			" country_codes = new.country_codes,"
			" correct_option_ids = new.correct_option_ids,"
			" explanation = new.explanation,"
			" explanation_entities = new.explanation_entities,"
			" media = COALESCE(new.media, telegram_polls.media),"
			" explanation_media = new.explanation_media,"
			" open_period = new.open_period,"
			" close_date = new.close_date,"
			" is_closed = new.is_closed,"
			" total_voter_count = new.total_voter_count,"
			" recent_voters = new.recent_voters",
			{ p.id, p.question, opt(p.question_entities),
			  std::string(p.is_quiz ? "quiz" : "regular"),
			  b(p.is_anonymous), b(p.allows_multiple_answers),
			  b(p.allows_revoting), b(p.members_only),
			  opt(p.country_codes), opt(p.correct_option_ids),
			  opt(p.explanation), opt(p.explanation_entities),
			  opt(p.media), opt(p.explanation_media),
			  (int64_t)p.open_period, p.close_date, b(p.is_closed),
			  (int64_t)p.total_voter_count, opt(p.recent_voters) });

		std::string keep;
		std::vector<mysql::Param> keep_params{ p.id };
		for (const auto &o : p.options) {
			tx.execute(
				"INSERT INTO telegram_poll_options ("
				" poll_id, option_id, position, text, entities,"
				" media, voter_count, vote_percentage,"
				" author_user_id, author_chat_id, addition_date"
				") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
				" AS new ON DUPLICATE KEY UPDATE"
				" position = new.position,"
				" text = new.text,"
				" entities = new.entities,"
				" media = new.media,"
				" voter_count = new.voter_count,"
				" vote_percentage = new.vote_percentage,"
				" author_user_id = new.author_user_id,"
				" author_chat_id = new.author_chat_id,"
				" addition_date = new.addition_date,"
				" deleted_at = NULL",
				{ p.id, o.option_id, (int64_t)o.position, o.text,
				  opt(o.entities), opt(o.media),
				  (int64_t)o.voter_count,
				  (int64_t)o.vote_percentage,
				  opt(o.author_user_id), opt(o.author_chat_id),
				  o.addition_date });
			keep += keep.empty() ? "?" : ", ?";
			keep_params.push_back(o.option_id);
		}

		/* An empty option list is a poll TDLib describes without its
		 * options, not one that lost them all: leave them. */
		if (keep.empty())
			return;
		tx.execute("UPDATE telegram_poll_options SET deleted_at = NOW()"
			   " WHERE poll_id = ? AND deleted_at IS NULL"
			   " AND option_id NOT IN (" + keep + ")",
			   keep_params);
	});
}

} /* namespace tgloggerd */

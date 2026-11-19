// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <cstdint>
#include <string>
#include <vector>

namespace tgloggerd {

namespace {

mysql::Param fileParam(uint64_t id)
{
	return id ? mysql::Param(id) : mysql::Param(std::monostate{});
}

int64_t b(bool v)
{
	return v ? 1 : 0;
}

} /* namespace */

void DB::upsertCommunity(const models::Community &c, uint64_t photo_file_id)
{
	mysql::Param date = std::monostate{};
	if (c.date > 0)
		date = c.date;
	db_.execute(
		"INSERT INTO telegram_communities (id, name, have_access,"
		" photo_file_id, date) VALUES (?, ?, ?, ?, ?)"
		" AS new ON DUPLICATE KEY UPDATE"
		" name = new.name, have_access = new.have_access,"
		" photo_file_id = new.photo_file_id, date = new.date",
		{ c.id, c.name, b(c.have_access), fileParam(photo_file_id),
		  date });
}

void DB::putCommunityFullInfo(const models::CommunityFullInfo &fi,
			      uint64_t photo_file_id)
{
	mysql::Param admins = std::monostate{};
	if (fi.administrator_count)
		admins = (int64_t)*fi.administrator_count;
	db_.transaction([&](mysql::Transaction &tx) {
		/* The name comes with updateCommunity, which TDLib sends
		 * before the id is used; a row made here waits for it. */
		tx.execute(
			"INSERT INTO telegram_communities (id, photo_file_id,"
			" administrator_count) VALUES (?, ?, ?)"
			" AS new ON DUPLICATE KEY UPDATE"
			" photo_file_id = new.photo_file_id,"
			" administrator_count = new.administrator_count",
			{ fi.community_id, fileParam(photo_file_id), admins });

		std::string keep;
		std::vector<int64_t> keep_ids;
		for (const auto &ch : fi.chats) {
			tx.execute(
				"INSERT INTO telegram_community_chats (community_id,"
				" chat_id, is_hidden, can_view_history)"
				" VALUES (?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
				" is_hidden = new.is_hidden,"
				" can_view_history = new.can_view_history",
				{ fi.community_id, ch.chat_id, b(ch.is_hidden),
				  b(ch.can_view_history) });
			keep += keep.empty() ? "?" : ", ?";
			keep_ids.push_back(ch.chat_id);
		}
		/*
		 * The full info lists every chat the account may see: one it
		 * no longer lists has left the community (or the view). A
		 * community always has a chat, so an empty list says nothing
		 * is visible right now, not that every chat left: the chats
		 * stored keep their rows, as an empty poll answer keeps its
		 * options.
		 */
		if (keep.empty())
			return;
		std::string sql = "DELETE FROM telegram_community_chats"
				  " WHERE community_id = ? AND chat_id NOT IN (" +
				  keep + ")";
		std::vector<mysql::Param> params{ fi.community_id };
		params.insert(params.end(), keep_ids.begin(), keep_ids.end());
		tx.execute(sql, params);
	});
}

bool DB::communityNeedsFullInfo(int64_t community_id)
{
	auto rows = db_.query(
		"SELECT EXISTS(SELECT 1 FROM telegram_communities WHERE id = ?),"
		" EXISTS(SELECT 1 FROM telegram_community_chats"
		"  WHERE community_id = ?)",
		{ community_id, community_id });
	if (rows.empty() || rows[0].size() < 2)
		return true;
	return rows[0][0].value_or("0") != "1" ||
	       rows[0][1].value_or("0") != "1";
}

} /* namespace tgloggerd */

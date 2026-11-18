// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <optional>

namespace tgloggerd {

namespace {

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

mysql::Param opt(const std::optional<int64_t> &v)
{
	if (v.has_value())
		return *v;
	return std::monostate{};
}

} /* namespace */

void DB::upsertForumTopic(const models::ForumTopic &t, bool only_if_absent)
{
	/*
	 * A topic described by TDLib (updateForumTopicInfo, getForumTopics)
	 * is the current state and replaces what is stored. One read from a
	 * "topic created" service message describes the topic as it was
	 * created, possibly long ago (history backfill), so it only fills in
	 * a topic not stored yet.
	 */
	std::string sql =
		"INSERT INTO telegram_forum_topics ("
		" chat_id, topic_id, name, is_name_implicit, icon_color,"
		" icon_custom_emoji_id, creation_date, creator_user_id,"
		" creator_chat_id, is_general, is_closed, is_hidden"
		") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) AS new"
		" ON DUPLICATE KEY UPDATE ";
	if (only_if_absent)
		sql += "chat_id = telegram_forum_topics.chat_id";
	else
		sql += "name = new.name,"
		       " is_name_implicit = new.is_name_implicit,"
		       " icon_color = new.icon_color,"
		       " icon_custom_emoji_id = new.icon_custom_emoji_id,"
		       " creation_date = IF(new.creation_date <> 0,"
		       "   new.creation_date, telegram_forum_topics.creation_date),"
		       " creator_user_id = COALESCE(new.creator_user_id,"
		       "   telegram_forum_topics.creator_user_id),"
		       " creator_chat_id = COALESCE(new.creator_chat_id,"
		       "   telegram_forum_topics.creator_chat_id),"
		       " is_general = new.is_general,"
		       " is_closed = new.is_closed,"
		       " is_hidden = new.is_hidden";
	db_.execute(sql, { t.chat_id, (int64_t)t.topic_id, t.name,
			   b(t.is_name_implicit), (int64_t)t.icon_color,
			   opt(t.icon_custom_emoji_id), t.creation_date,
			   opt(t.creator_user_id), opt(t.creator_chat_id),
			   b(t.is_general), b(t.is_closed), b(t.is_hidden) });
}

void DB::setForumTopicPinned(int64_t chat_id, int32_t topic_id, bool pinned)
{
	db_.execute("UPDATE telegram_forum_topics SET is_pinned = ?"
		    " WHERE chat_id = ? AND topic_id = ?",
		    { b(pinned), chat_id, (int64_t)topic_id });
}

} /* namespace tgloggerd */

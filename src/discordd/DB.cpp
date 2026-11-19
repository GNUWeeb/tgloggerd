// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "DB.hpp"

#include <cstdlib>

namespace discordd {

/* Snowflakes exceed INT64_MAX only in theory, but bind them as unsigned. */
static inline mysql::Param u64(uint64_t v)
{
	return mysql::Param{v};
}

static inline mysql::Param i64(int64_t v)
{
	return mysql::Param{v};
}

static int64_t row_i64(const mysql::Row &r, size_t i)
{
	if (i >= r.size() || !r[i].has_value())
		return 0;
	return strtoll(r[i]->c_str(), nullptr, 10);
}

static uint64_t row_u64(const mysql::Row &r, size_t i)
{
	if (i >= r.size() || !r[i].has_value())
		return 0;
	return strtoull(r[i]->c_str(), nullptr, 10);
}

static std::string row_str(const mysql::Row &r, size_t i)
{
	if (i >= r.size() || !r[i].has_value())
		return std::string();
	return *r[i];
}

DB::DB(const mysql::Config &cfg) : db_(new mysql::Database(cfg))
{
}

DB::~DB(void) = default;

void DB::ping(void)
{
	db_->query("SELECT 1");
}

std::vector<Route> DB::loadRoutes(bool bots_table)
{
	static const char *SQL =
		"SELECT r.id, r.discord_channel_id, r.telegram_chat_id, "
		"       r.telegram_bot_id, b.token, b.bot_user_id, b.username, "
		"       r.fwd_sticker, r.fwd_text, r.fwd_photo, r.fwd_video, "
		"       r.fwd_document, r.discord_bot_id, r.fwd_audio, "
		"       r.fwd_animation, r.fwd_voice, r.fwd_embed, r.fwd_poll, "
		"       r.fwd_system "
		"FROM discord_telegram_routes r "
		"JOIN telegram_bots b ON b.id = r.telegram_bot_id "
		"LEFT JOIN discord_bots d ON d.id = r.discord_bot_id "
		"WHERE r.enabled = 1 AND b.enabled = 1 "
		"  AND (r.discord_bot_id IS NULL OR d.enabled = 1)";
	/* Before migration 000040: no Discord bot column, no table, and no
	 * audio or animation flag either (000048), nor voice, embed, poll or
	 * system (000050), so those follow the flags they were split from. */
	static const char *SQL_BEFORE =
		"SELECT r.id, r.discord_channel_id, r.telegram_chat_id, "
		"       r.telegram_bot_id, b.token, b.bot_user_id, b.username, "
		"       r.fwd_sticker, r.fwd_text, r.fwd_photo, r.fwd_video, "
		"       r.fwd_document, NULL, r.fwd_document, r.fwd_photo, "
		"       r.fwd_document, r.fwd_text, r.fwd_text, r.fwd_text "
		"FROM discord_telegram_routes r "
		"JOIN telegram_bots b ON b.id = r.telegram_bot_id "
		"WHERE r.enabled = 1 AND b.enabled = 1";

	/* Column order of the fwd_* flags above, from column 7 on. */
	static const unsigned bits[] = { FWD_STICKER, FWD_TEXT, FWD_PHOTO,
					 FWD_VIDEO, FWD_DOCUMENT };

	std::vector<Route> out;
	for (const auto &r : db_->query(bots_table ? SQL : SQL_BEFORE)) {
		Route rt;
		rt.id = row_u64(r, 0);
		rt.discord_channel_id = row_u64(r, 1);
		rt.telegram_chat_id = row_i64(r, 2);
		rt.telegram_bot_id = row_u64(r, 3);
		rt.bot_token = row_str(r, 4);
		rt.bot_user_id = row_i64(r, 5);
		rt.bot_username = row_str(r, 6);
		for (size_t i = 0; i < sizeof(bits) / sizeof(bits[0]); i++) {
			if (row_u64(r, 7 + i))
				rt.types |= bits[i];
		}
		rt.discord_bot_id = row_u64(r, 12);
		if (row_u64(r, 13))
			rt.types |= FWD_AUDIO;
		if (row_u64(r, 14))
			rt.types |= FWD_ANIMATION;
		/* From column 15 on, migration 000050's flags. */
		static const unsigned more[] = { FWD_VOICE, FWD_EMBED,
						 FWD_POLL, FWD_SYSTEM };
		for (size_t i = 0; i < sizeof(more) / sizeof(more[0]); i++) {
			if (row_u64(r, 15 + i))
				rt.types |= more[i];
		}
		out.push_back(std::move(rt));
	}
	return out;
}

bool DB::hasBotsTable(void)
{
	auto rows = db_->query("SELECT COUNT(*) FROM information_schema.tables "
			       "WHERE table_schema = DATABASE() "
			       "AND table_name = 'discord_bots'");
	return !rows.empty() && row_u64(rows[0], 0) > 0;
}

uint64_t DB::countRoutesWithoutDiscordBot(void)
{
	auto rows = db_->query("SELECT COUNT(*) FROM discord_telegram_routes "
			       "WHERE enabled = 1 AND discord_bot_id IS NULL");
	return rows.empty() ? 0 : row_u64(rows[0], 0);
}

std::vector<DiscordBotRow> DB::loadDiscordBots(void)
{
	std::vector<DiscordBotRow> out;
	for (const auto &r : db_->query("SELECT id, token FROM discord_bots "
					"WHERE enabled = 1 ORDER BY id")) {
		DiscordBotRow b;
		b.id = row_u64(r, 0);
		b.token = row_str(r, 1);
		out.push_back(std::move(b));
	}
	return out;
}

std::vector<TelegramBotRow> DB::loadTelegramBots(void)
{
	std::vector<TelegramBotRow> out;
	for (const auto &r : db_->query("SELECT id, token, bot_user_id, username "
					"FROM telegram_bots WHERE enabled = 1 "
					"ORDER BY id")) {
		TelegramBotRow b;
		b.id = row_u64(r, 0);
		b.token = row_str(r, 1);
		b.bot_user_id = row_i64(r, 2);
		b.username = row_str(r, 3);
		out.push_back(std::move(b));
	}
	return out;
}

/* The UPDATE of a bot report, for the table named (a constant of ours). */
static void report_bot(mysql::Database &db, const char *table,
		       const BotReport &r)
{
	std::string sql = std::string("UPDATE ") + table +
			  " SET status = ?, status_detail = ?, "
			  "status_at = NOW(), enabled = IF(?, 0, enabled)";
	std::vector<mysql::Param> p = {
		std::string(bot_status_name(r.status)),
		r.detail.empty() ? mysql::Param{} : mysql::Param{r.detail},
		(int64_t)(r.disable ? 1 : 0)};
	if (r.has_identity) {
		sql += ", bot_user_id = ?, username = ?";
		p.push_back(u64(r.user_id));
		p.push_back(r.username);
	}
	sql += " WHERE id = ? AND token = ? AND enabled = 1";
	p.push_back(u64(r.id));
	p.push_back(r.token);
	db.execute(sql, p);
}

void DB::reportDiscordBot(const BotReport &r)
{
	report_bot(*db_, "discord_bots", r);
}

void DB::reportTelegramBot(const BotReport &r)
{
	report_bot(*db_, "telegram_bots", r);
}

void DB::setBotUserId(uint64_t bot_row_id, int64_t user_id,
		      const std::string &username)
{
	db_->execute("UPDATE telegram_bots SET bot_user_id = ?, username = ? "
		     "WHERE id = ?",
		     {i64(user_id), username, u64(bot_row_id)});
}

void DB::upsertGuild(uint64_t id, const std::string &name)
{
	if (!id)
		return;
	db_->execute("INSERT INTO discord_guilds (id, name) VALUES (?, ?) "
		     "ON DUPLICATE KEY UPDATE name = VALUES(name)",
		     {u64(id), name});
}

void DB::upsertChannel(uint64_t id, uint64_t guild_id,
		       const std::string &name, int type)
{
	if (!id)
		return;
	db_->execute("INSERT INTO discord_channels (id, guild_id, name, type) "
		     "VALUES (?, ?, ?, ?) "
		     "ON DUPLICATE KEY UPDATE guild_id = VALUES(guild_id), "
		     "  name = VALUES(name), type = VALUES(type)",
		     {u64(id), u64(guild_id), name, (int64_t)type});
}

void DB::upsertUser(uint64_t id, const std::string &username,
		    const std::string &global_name,
		    const std::string &discriminator, const std::string &avatar,
		    bool is_bot)
{
	if (!id)
		return;
	db_->execute(
		"INSERT INTO discord_users "
		"  (id, username, global_name, discriminator, avatar, is_bot) "
		"VALUES (?, ?, ?, ?, ?, ?) "
		"ON DUPLICATE KEY UPDATE username = VALUES(username), "
		"  global_name = VALUES(global_name), "
		"  discriminator = VALUES(discriminator), "
		"  avatar = VALUES(avatar), is_bot = VALUES(is_bot)",
		{u64(id), username, global_name, discriminator, avatar,
		 (int64_t)(is_bot ? 1 : 0)});
}

void DB::upsertMessage(uint64_t id, uint64_t channel_id, uint64_t guild_id,
		       uint64_t author_id, uint64_t webhook_id,
		       const std::string &content, uint64_t reply_to_message_id,
		       uint64_t sent_at_ms, bool edited)
{
	/*
	 * FROM_UNIXTIME with a fractional argument gives DATETIME(3); the
	 * snowflake carries milliseconds, so divide rather than truncate.
	 */
	const double secs = (double)sent_at_ms / 1000.0;

	if (edited) {
		db_->execute(
			"INSERT INTO discord_messages "
			"  (id, channel_id, guild_id, author_id, webhook_id, "
			"   content, reply_to_message_id, sent_at, edited_at) "
			"VALUES (?, ?, ?, ?, ?, ?, ?, FROM_UNIXTIME(?), NOW(3)) "
			"ON DUPLICATE KEY UPDATE content = VALUES(content), "
			"  edited_at = NOW(3)",
			{u64(id), u64(channel_id), u64(guild_id), u64(author_id),
			 u64(webhook_id), content, u64(reply_to_message_id),
			 secs});
		return;
	}

	db_->execute(
		"INSERT INTO discord_messages "
		"  (id, channel_id, guild_id, author_id, webhook_id, content, "
		"   reply_to_message_id, sent_at) "
		"VALUES (?, ?, ?, ?, ?, ?, ?, FROM_UNIXTIME(?)) "
		"ON DUPLICATE KEY UPDATE content = VALUES(content)",
		{u64(id), u64(channel_id), u64(guild_id), u64(author_id),
		 u64(webhook_id), content, u64(reply_to_message_id), secs});
}

void DB::upsertAttachment(uint64_t id, uint64_t message_id,
			  const std::string &filename,
			  const std::string &content_type, uint64_t size,
			  const std::string &url, int width, int height)
{
	if (!id)
		return;
	db_->execute("INSERT INTO discord_attachments "
		     "  (id, message_id, filename, content_type, size, url, "
		     "   width, height) "
		     "VALUES (?, ?, ?, ?, ?, ?, ?, ?) "
		     "ON DUPLICATE KEY UPDATE url = VALUES(url), "
		     "  filename = VALUES(filename), size = VALUES(size)",
		     {u64(id), u64(message_id), filename, content_type,
		      u64(size), url, (int64_t)width, (int64_t)height});
}

void DB::markMessageDeleted(uint64_t message_id)
{
	db_->execute("UPDATE discord_messages SET deleted_at = NOW() "
		     "WHERE id = ? AND deleted_at IS NULL",
		     {u64(message_id)});
}

bool DB::alreadyForwarded(uint64_t discord_message_id, uint64_t route_id)
{
	auto rows = db_->query("SELECT 1 FROM discord_telegram_sent_messages "
			       "WHERE discord_message_id = ? AND route_id = ? "
			       "LIMIT 1",
			       {u64(discord_message_id), u64(route_id)});
	return !rows.empty();
}

void DB::recordForwarded(uint64_t discord_message_id, uint64_t route_id,
			 int64_t telegram_chat_id, int64_t telegram_message_id)
{
	db_->execute("INSERT IGNORE INTO discord_telegram_sent_messages "
		     "  (discord_message_id, route_id, telegram_chat_id, "
		     "   telegram_message_id) "
		     "VALUES (?, ?, ?, ?)",
		     {u64(discord_message_id), u64(route_id),
		      i64(telegram_chat_id), i64(telegram_message_id)});
}

std::vector<ForwardedMessage> DB::getForwarded(uint64_t discord_message_id)
{
	static const char *SQL =
		"SELECT f.route_id, f.telegram_chat_id, f.telegram_message_id, "
		"       r.telegram_bot_id, b.token "
		"FROM discord_telegram_sent_messages f "
		"JOIN discord_telegram_routes r ON r.id = f.route_id "
		"JOIN telegram_bots b ON b.id = r.telegram_bot_id "
		"WHERE f.discord_message_id = ?";

	std::vector<ForwardedMessage> out;
	for (const auto &r : db_->query(SQL, {u64(discord_message_id)})) {
		ForwardedMessage fm;
		fm.route_id = row_u64(r, 0);
		fm.telegram_chat_id = row_i64(r, 1);
		fm.telegram_message_id = row_i64(r, 2);
		fm.telegram_bot_id = row_u64(r, 3);
		fm.bot_token = row_str(r, 4);
		out.push_back(std::move(fm));
	}
	return out;
}

std::optional<ReplyTarget> DB::resolveReply(uint64_t discord_reply_to_id,
					    int64_t telegram_chat_id)
{
	if (!discord_reply_to_id)
		return std::nullopt;

	/*
	 * 1. discordd forwarded the replied-to message itself. Constrain to
	 *    the destination chat: replying across chats is meaningless.
	 */
	auto rows = db_->query(
		"SELECT telegram_chat_id, telegram_message_id "
		"FROM discord_telegram_sent_messages "
		"WHERE discord_message_id = ? AND telegram_chat_id = ? LIMIT 1",
		{u64(discord_reply_to_id), i64(telegram_chat_id)});
	if (!rows.empty()) {
		ReplyTarget t;
		t.telegram_chat_id = row_i64(rows[0], 0);
		t.telegram_message_id = row_i64(rows[0], 1);
		if (t.telegram_message_id)
			return t;
	}

	/*
	 * 2. The replied-to message is itself a mirror of a Telegram message,
	 *    posted into Discord by the Telegram -> Discord webhook forwarder.
	 *    That table keys the Discord id as a string, so bind it as text.
	 */
	rows = db_->query("SELECT telegram_chat_id, telegram_message_id "
			  "FROM telegram_discord_sent_messages "
			  "WHERE message_id = ? AND telegram_chat_id = ? "
			  "LIMIT 1",
			  {std::to_string(discord_reply_to_id),
			   i64(telegram_chat_id)});
	if (!rows.empty()) {
		ReplyTarget t;
		t.telegram_chat_id = row_i64(rows[0], 0);
		t.telegram_message_id = row_i64(rows[0], 1);
		if (t.telegram_message_id)
			return t;
	}

	return std::nullopt;
}

} /* namespace discordd */

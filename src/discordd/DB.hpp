// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__DB_HPP
#define DISCORDD__DB_HPP

#include "DiscordBots.hpp"

#include <mysql/Database.hpp>

#include <cstdint>
#include <memory>
#include <optional>
#include <string>
#include <vector>

namespace discordd {

/*
 * The content types a route can be told to forward, one bit each. A message is
 * filtered part by part: its text follows FWD_TEXT, each attachment the bit of
 * its own type, each sticker FWD_STICKER.
 */
enum : unsigned {
	FWD_STICKER  = 1u << 0,
	FWD_TEXT     = 1u << 1,
	FWD_PHOTO    = 1u << 2,	/* an attachment of an image type */
	FWD_VIDEO    = 1u << 3,	/* an attachment of a video type */
	FWD_DOCUMENT = 1u << 4,	/* every other attachment */
};

/* One enabled Discord channel -> Telegram chat route, with its bots. */
struct Route {
	uint64_t	id = 0;
	uint64_t	discord_bot_id = 0;	/* the bot that reads the channel */
	uint64_t	discord_channel_id = 0;
	int64_t		telegram_chat_id = 0;
	uint64_t	telegram_bot_id = 0;
	std::string	bot_token;
	int64_t		bot_user_id = 0;
	std::string	bot_username;	/* as last recorded; may be empty */
	unsigned	types = 0;	/* FWD_* bits this route forwards */
};

/* An enabled Telegram bot, as loaded from telegram_bots. */
struct TelegramBotRow {
	uint64_t	id = 0;
	std::string	token;
	int64_t		bot_user_id = 0;
	std::string	username;
};

/* A Telegram message a Discord reply should be threaded onto. */
struct ReplyTarget {
	int64_t	telegram_chat_id = 0;
	int64_t	telegram_message_id = 0; /* server id (tdlib id >> 20) */
};

/* A Telegram message discordd previously sent for a Discord message. */
struct ForwardedMessage {
	uint64_t	route_id = 0;
	int64_t		telegram_chat_id = 0;
	int64_t		telegram_message_id = 0;
	uint64_t	telegram_bot_id = 0;
	std::string	bot_token;
};

/*
 * discordd's view of the database. Deliberately separate from
 * tgloggerd::DB: the two daemons share the connection pool implementation
 * (src/mysql) but not their schemas or their queries.
 */
class DB {
public:
	explicit DB(const mysql::Config &cfg);
	~DB(void);

	DB(const DB &) = delete;
	DB &operator=(const DB &) = delete;

	void ping(void);

	/*
	 * Every route that is enabled, as are both its bots, joined to the
	 * Telegram bot's credentials. A route that names no Discord bot comes
	 * with discord_bot_id 0 (LEGACY_BOT_ID), for the environment's bot of
	 * the transition; so does every route before migration 000040, when
	 * bots_table is false.
	 */
	std::vector<Route> loadRoutes(bool bots_table);

	/*
	 * Whether discord_bots exists, i.e. whether tgloggerd has applied
	 * migration 000040. Before it, routes name no Discord bot and the
	 * bots have no status columns.
	 */
	bool hasBotsTable(void);

	/* Enabled routes that name no Discord bot yet (made before 000040). */
	uint64_t countRoutesWithoutDiscordBot(void);

	/* The enabled bots of each kind. */
	std::vector<DiscordBotRow> loadDiscordBots(void);
	std::vector<TelegramBotRow> loadTelegramBots(void);

	/*
	 * Write what discordd saw of a bot to its row: status, detail, time,
	 * the identity if known, and enabled = 0 when it was refused. Only an
	 * enabled row that still holds the reported token is changed, so a
	 * late report neither overrides the owner switching the bot off nor
	 * lands on a replaced token. Throws on a database error.
	 */
	void reportDiscordBot(const BotReport &r);
	void reportTelegramBot(const BotReport &r);

	/* Record the bot's own Telegram user id, learned at login. */
	void setBotUserId(uint64_t bot_row_id, int64_t user_id,
			  const std::string &username);

	/* Entity upserts. Cheap and idempotent; called on every message. */
	void upsertGuild(uint64_t id, const std::string &name);
	void upsertChannel(uint64_t id, uint64_t guild_id,
			   const std::string &name, int type);
	void upsertUser(uint64_t id, const std::string &username,
			const std::string &global_name,
			const std::string &discriminator,
			const std::string &avatar, bool is_bot);

	/*
	 * Insert (or update, on a redelivered event) one logged message.
	 * `sent_at_ms` is derived from the snowflake by the caller.
	 */
	void upsertMessage(uint64_t id, uint64_t channel_id, uint64_t guild_id,
			   uint64_t author_id, uint64_t webhook_id,
			   const std::string &content,
			   uint64_t reply_to_message_id, uint64_t sent_at_ms,
			   bool edited);

	void upsertAttachment(uint64_t id, uint64_t message_id,
			      const std::string &filename,
			      const std::string &content_type, uint64_t size,
			      const std::string &url, int width, int height);

	void markMessageDeleted(uint64_t message_id);

	/* Has this message already been forwarded on this route? */
	bool alreadyForwarded(uint64_t discord_message_id, uint64_t route_id);

	void recordForwarded(uint64_t discord_message_id, uint64_t route_id,
			     int64_t telegram_chat_id,
			     int64_t telegram_message_id);

	/* Everything discordd sent for a Discord message (for edit/delete). */
	std::vector<ForwardedMessage> getForwarded(uint64_t discord_message_id);

	/*
	 * Resolve the Telegram message a Discord reply should point at, for a
	 * given destination chat. Two sources, because a Discord message can
	 * correspond to a Telegram message in either direction:
	 *
	 *   1. discordd forwarded it  -> discord_telegram_sent_messages
	 *   2. it mirrors a Telegram message posted by the Telegram -> Discord
	 *      webhook forwarder -> telegram_discord_sent_messages
	 *
	 * Returns nothing when the replied-to message has no Telegram
	 * counterpart in `telegram_chat_id`, in which case the forward is sent
	 * without a reply rather than threaded onto the wrong message.
	 */
	std::optional<ReplyTarget> resolveReply(uint64_t discord_reply_to_id,
						int64_t telegram_chat_id);

private:
	std::unique_ptr<mysql::Database> db_;
};

} /* namespace discordd */

#endif /* #ifndef DISCORDD__DB_HPP */

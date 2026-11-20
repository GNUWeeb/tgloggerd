// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__DISCORDD_HPP
#define DISCORDD__DISCORDD_HPP

#include <mysql/ConnectionPool.hpp>

#include <cstdint>
#include <memory>
#include <string>

namespace discordd {

struct Config {
	/*
	 * The Discord bots, and the Telegram bots, are rows of the database
	 * (discord_bots, telegram_bots), not settings. DEPRECATED, for the
	 * transition only: while the table is missing or holds no enabled
	 * bot, the token of DISCORD_BOT_TOKEN runs as before and serves the
	 * routes that name no Discord bot.
	 */
	std::string	legacy_discord_bot_token;

	/* Telegram side: TDLib needs these even for a bot session. */
	int32_t		api_id = 0;
	std::string	api_hash;
	std::string	data_dir;	/* parent of the per-bot session dirs */

	mysql::Config	db;

	/* How often the bots and routes are re-read, in seconds. */
	int		route_refresh_secs = 30;

	int		log_level = 2;	/* 0=error 1=warn 2=info 3=debug */
};

/*
 * The Discord -> Telegram bridge.
 *
 * Runs one gwdiscord gateway per enabled Discord bot, logs every message
 * that belongs to a route read by that bot, and forwards it to the route's
 * Telegram chat through the route's Telegram bot. Edits and deletes follow
 * the message. Each bot's state, as discordd sees it, is written back to
 * its row.
 */
class DiscordD {
public:
	explicit DiscordD(Config cfg);
	~DiscordD(void);

	DiscordD(const DiscordD &) = delete;
	DiscordD &operator=(const DiscordD &) = delete;

	/*
	 * Blocks until stop(). 0 on a clean exit, 1 when the database is
	 * unavailable at start. A bot Discord refuses is disabled, not a
	 * reason to exit.
	 */
	int run(void);

	/* Safe from any thread, including a signal handler. */
	void stop(void);

private:
	struct Impl;
	std::unique_ptr<Impl> impl_;
};

} /* namespace discordd */

#endif /* #ifndef DISCORDD__DISCORDD_HPP */

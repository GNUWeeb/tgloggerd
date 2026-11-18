// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__DISCORD_BOTS_HPP
#define DISCORDD__DISCORD_BOTS_HPP

#include <gwdiscord/Gateway.hpp>

#include <chrono>
#include <cstdint>
#include <functional>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <vector>

namespace discordd {

/*
 * A bot's status as the bots tables store it (migration 000040) and the web
 * shows it. Disabled is the web's to write; discordd writes the others.
 */
enum class BotStatus {
	Connecting,
	Connected,
	IntentMissing,	/* Discord only: Message Content is off for the bot */
	TokenRefused,
	Disabled,
};

const char *bot_status_name(BotStatus st);

/* An enabled Discord bot, as loaded from discord_bots. */
struct DiscordBotRow {
	uint64_t	id = 0;
	std::string	token;
};

/*
 * What discordd learned about a bot, to be written back to its row. The row
 * is matched by id AND token, so a report about a token that has since been
 * replaced changes nothing.
 */
struct BotReport {
	uint64_t	id = 0;
	std::string	token;
	BotStatus	status = BotStatus::Connecting;
	std::string	detail;		/* "" = none */
	bool		disable = false; /* refused: switch the bot off */

	/* Set once the bot has logged in. */
	bool		has_identity = false;
	uint64_t	user_id = 0;
	std::string	username;
};

/* The report for a gateway that stopped by itself, from why it stopped. */
BotReport report_for_stop(const DiscordBotRow &row, gwdiscord::StopReason why,
			  const gwdiscord::CloseInfo &close);

/*
 * Runs one gateway connection per enabled Discord bot.
 *
 * sync() is given the enabled bots each time discordd reloads its tables: it
 * starts the new ones, stops the ones that were disabled, removed or given a
 * new token, and leaves the rest running. Each bot runs on its own thread,
 * and its events reach the handlers on that thread, tagged with the bot's id.
 *
 * A bot Discord refuses (a bad token, a missing privileged intent) stops and
 * is reported with disable set, so its row is switched off; it is not started
 * again until the row is enabled anew. Each IDENTIFY spends one of a daily
 * budget whose exhaustion makes Discord reset the token, so a refused bot is
 * never retried in a loop: should the report fail to reach the database, it
 * is retried on the next sync and the bot stays stopped meanwhile.
 */
class DiscordBots {
public:
	using Transports =
		std::function<gwdiscord::Transport(const DiscordBotRow &)>;
	/* Write a report; false when it could not be stored. */
	using Reporter = std::function<bool(const BotReport &)>;
	using Log = std::function<void(int /*level*/, const std::string &)>;

	struct Handlers {
		std::function<void(uint64_t bot, const gwdiscord::Message &,
				   bool edited)>
			message;
		std::function<void(uint64_t bot,
				   const gwdiscord::MessageDelete &)>
			deleted;
	};

	using Clock = std::chrono::steady_clock;

	DiscordBots(gwdiscord::GatewayConfig base, Transports tp,
		    Reporter report, Handlers h, Log log);
	~DiscordBots(void);

	DiscordBots(const DiscordBots &) = delete;
	DiscordBots &operator=(const DiscordBots &) = delete;

	/*
	 * Bring the running bots in line with `enabled`, which was read from
	 * the database starting at `loaded_at`. A bot that stopped by itself
	 * after that moment is left alone until a later load, which then
	 * reflects its report.
	 */
	void sync(const std::vector<DiscordBotRow> &enabled,
		  Clock::time_point loaded_at);

	/* Stop every bot and wait for its thread. */
	void stop_all(void);

	/* The Discord user ids of the bots that have logged in. */
	std::vector<uint64_t> user_ids(void) const;

	/* The ids of the bots whose gateway is running, ascending. */
	std::vector<uint64_t> running(void) const;

private:
	struct Bot;

	void start(const DiscordBotRow &row);
	void run_bot(const std::shared_ptr<Bot> &b);
	void stop_bot(const std::shared_ptr<Bot> &b);

	gwdiscord::GatewayConfig	base_;
	Transports			tp_;
	Reporter			report_;
	Handlers			h_;
	Log				log_;

	mutable std::mutex			mtx_;
	std::map<uint64_t, std::shared_ptr<Bot>> bots_;
};

} /* namespace discordd */

#endif /* #ifndef DISCORDD__DISCORD_BOTS_HPP */

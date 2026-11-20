// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__BOT_WATCH_HPP
#define TGLOGGERD__BOT_WATCH_HPP

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <functional>
#include <map>
#include <mutex>
#include <string>
#include <thread>

#include "helpers/log.h"

namespace tgloggerd {

class DB;
class EventBus;

/*
 * Publishes bot.updated (docs/internal-api.md) when a bot row changes.
 *
 * The bots are written by discordd (their status) and by the web (name,
 * enabled, token), neither of which publishes events, so this reads the
 * bots of both kinds every interval and publishes one event per bot that
 * was added, changed or removed since the last read. The first read only
 * learns what is there. A read that fails publishes nothing and is tried
 * again on the next interval.
 */
class BotWatch {
public:
	using Load = std::function<std::map<uint64_t, std::string>(void)>;
	using Publish = std::function<void(uint64_t bot_id)>;

	BotWatch(Load load, Publish publish, std::chrono::milliseconds every,
		 log_hd_t *l);
	BotWatch(DB *db, EventBus *events, log_hd_t *l);
	~BotWatch(void);

	BotWatch(const BotWatch &) = delete;
	BotWatch &operator=(const BotWatch &) = delete;

	void start(void);
	void stop(void);

	/* One read and its events; the thread's step, exposed for tests. */
	void poll(void);

private:
	void run(void);

	Load				load_;
	Publish				publish_;
	std::chrono::milliseconds	every_;
	log_hd_t			*l_;

	bool				primed_ = false;
	std::map<uint64_t, std::string>	last_;

	std::mutex			mtx_;
	std::condition_variable		cv_;
	bool				stopping_ = false;
	std::thread			thr_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__BOT_WATCH_HPP */

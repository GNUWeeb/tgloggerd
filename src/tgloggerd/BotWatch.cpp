// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "BotWatch.hpp"

#include "DB.hpp"
#include "EventBus.hpp"

#include <set>

namespace tgloggerd {

/* Often enough for a status to show soon, rarely enough to cost nothing. */
static constexpr std::chrono::seconds BOT_WATCH_INTERVAL{5};

BotWatch::BotWatch(Load load, Publish publish, std::chrono::milliseconds every,
		   log_hd_t *l)
	: load_(std::move(load)), publish_(std::move(publish)), every_(every),
	  l_(l)
{
}

BotWatch::BotWatch(DB *db, EventBus *events, log_hd_t *l)
	: BotWatch([db] { return db->loadBotSignatures(); },
		   [events](uint64_t id) {
			   events->publish("bot.updated", { { "bot_id", id } });
		   },
		   BOT_WATCH_INTERVAL, l)
{
}

BotWatch::~BotWatch(void)
{
	stop();
}

void BotWatch::poll(void)
{
	std::map<uint64_t, std::string> now;
	try {
		now = load_();
	} catch (const std::exception &e) {
		pr_debug(l_, "bot watch: read failed: %s", e.what());
		return;
	}
	if (primed_) {
		std::set<uint64_t> changed;
		for (const auto &kv : now) {
			auto it = last_.find(kv.first);
			if (it == last_.end() || it->second != kv.second)
				changed.insert(kv.first);
		}
		for (const auto &kv : last_) {
			if (!now.count(kv.first))
				changed.insert(kv.first);
		}
		for (uint64_t id : changed)
			publish_(id);
	}
	last_.swap(now);
	primed_ = true;
}

void BotWatch::run(void)
{
	for (;;) {
		poll();
		std::unique_lock<std::mutex> lk(mtx_);
		if (cv_.wait_for(lk, every_, [this] { return stopping_; }))
			return;
	}
}

void BotWatch::start(void)
{
	if (thr_.joinable())
		return;
	thr_ = std::thread([this] { run(); });
}

void BotWatch::stop(void)
{
	{
		std::lock_guard<std::mutex> lk(mtx_);
		stopping_ = true;
	}
	cv_.notify_all();
	if (thr_.joinable())
		thr_.join();
}

} /* namespace tgloggerd */

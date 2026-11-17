// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "EventBus.hpp"

namespace tgloggerd {

namespace {

uint64_t now_us(void)
{
	using namespace std::chrono;
	return (uint64_t)duration_cast<microseconds>(
		system_clock::now().time_since_epoch()).count();
}

} /* namespace */

EventBus::EventBus(void) : EventBus(Config(), now_us()) {}

EventBus::EventBus(Config cfg) : EventBus(cfg, now_us()) {}

EventBus::EventBus(Config cfg, uint64_t seq_base)
	: cfg_(cfg), last_(seq_base)
{
	if (cfg_.ring < 1)
		cfg_.ring = 1;
}

EventBus::~EventBus(void)
{
	stop();
}

void EventBus::start(void)
{
	std::lock_guard<std::mutex> lk(run_mtx_);
	if (thread_.joinable())
		return;
	stopping_ = false;
	thread_ = std::thread([this] { run(); });
}

void EventBus::stop(void)
{
	{
		std::lock_guard<std::mutex> lk(run_mtx_);
		if (!thread_.joinable())
			return;
		stopping_ = true;
	}
	run_cv_.notify_all();
	thread_.join();
	flushStats();
}

void EventBus::run(void)
{
	std::unique_lock<std::mutex> lk(run_mtx_);
	while (!stopping_) {
		/*
		 * A fixed cadence rather than "interval after the first
		 * change": a stats.changed goes out at most once per
		 * interval, and a counter that moved waits at most one.
		 */
		run_cv_.wait_for(lk, cfg_.stats_interval,
				 [this] { return stopping_; });
		if (stopping_)
			break;
		lk.unlock();
		flushStats();
		lk.lock();
	}
}

uint64_t EventBus::publish(const char *kind, const nlohmann::json &fields)
{
	/*
	 * Serialise the fields outside the lock. The result is spelled by
	 * hand so that seq and kind come first, which nlohmann's sorted
	 * objects would not keep.
	 */
	std::string rest;
	if (fields.is_object() && !fields.empty()) {
		rest = fields.dump(-1, ' ', false,
				   nlohmann::json::error_handler_t::replace);
		rest[0] = ',';
	} else {
		rest = "}";
	}
	std::string kind_json = nlohmann::json(kind).dump();

	std::lock_guard<std::mutex> lk(mtx_);
	uint64_t seq = ++last_;
	std::string json;
	json.reserve(32 + kind_json.size() + rest.size());
	json += "{\"seq\":";
	json += std::to_string(seq);
	json += ",\"kind\":";
	json += kind_json;
	json += rest;
	ring_.push_back(Event{ seq, std::move(json) });
	while (ring_.size() > cfg_.ring)
		ring_.pop_front();
	for (auto &s : subs_)
		s.second();
	return seq;
}

uint64_t EventBus::messageNew(int64_t chat_id, int64_t message_id)
{
	return publish("message.new", { { "chat_id", chat_id },
					{ "message_id", message_id } });
}

uint64_t EventBus::messageEdited(int64_t chat_id, int64_t message_id)
{
	return publish("message.edited", { { "chat_id", chat_id },
					   { "message_id", message_id } });
}

uint64_t EventBus::messageDeleted(int64_t chat_id, int64_t message_id)
{
	return publish("message.deleted", { { "chat_id", chat_id },
					    { "message_id", message_id } });
}

uint64_t EventBus::userUpdated(int64_t user_id,
			       std::initializer_list<const char *> fields)
{
	nlohmann::json f = { { "user_id", user_id } };
	if (fields.size())
		f["fields"] = fields;
	return publish("user.updated", f);
}

uint64_t EventBus::groupUpdated(int64_t chat_id,
				std::initializer_list<const char *> fields)
{
	nlohmann::json f = { { "chat_id", chat_id } };
	if (fields.size())
		f["fields"] = fields;
	return publish("group.updated", f);
}

uint64_t EventBus::fileStored(uint64_t file_id)
{
	return publish("file.stored", { { "file_id", file_id } });
}

uint64_t EventBus::fileStored(uint64_t file_id, int64_t chat_id,
			      int64_t message_id)
{
	return publish("file.stored", { { "file_id", file_id },
					{ "chat_id", chat_id },
					{ "message_id", message_id } });
}

void EventBus::countersMoved(int64_t chat_id, int64_t user_id)
{
	if (!chat_id && !user_id)
		return;
	std::lock_guard<std::mutex> lk(stats_mtx_);
	if (chat_id)
		stats_chats_.insert(chat_id);
	if (user_id)
		stats_users_.insert(user_id);
}

uint64_t EventBus::flushStats(void)
{
	std::set<int64_t> chats, users;
	{
		std::lock_guard<std::mutex> lk(stats_mtx_);
		if (stats_chats_.empty() && stats_users_.empty())
			return 0;
		chats.swap(stats_chats_);
		users.swap(stats_users_);
	}

	/*
	 * A burst (a backfill page, a flood in a big group) could name
	 * thousands; past the cap the list is cut and "more" tells the
	 * reader to treat every counter as changed.
	 */
	const size_t cap = cfg_.stats_max_ids;
	bool more = chats.size() > cap || users.size() > cap;
	auto list = [cap](const std::set<int64_t> &s) {
		nlohmann::json a = nlohmann::json::array();
		for (int64_t id : s) {
			if (a.size() >= cap)
				break;
			a.push_back(id);
		}
		return a;
	};
	nlohmann::json f = { { "chat_ids", list(chats) },
			     { "user_ids", list(users) } };
	if (more)
		f["more"] = true;
	return publish("stats.changed", f);
}

EventBus::ReadResult EventBus::readSince(uint64_t since,
					 std::vector<Event> &out, size_t max,
					 uint64_t *last) const
{
	std::lock_guard<std::mutex> lk(mtx_);
	if (last)
		*last = last_;
	if (since > last_)
		return ReadResult::Gap;
	if (since == last_)
		return ReadResult::Ok;
	/* Empty only before the first event: since is from before it. */
	if (ring_.empty())
		return ReadResult::Gap;
	const uint64_t oldest = ring_.front().seq;
	if (since + 1 < oldest)
		return ReadResult::Gap;
	/* Seqs in the ring are consecutive. */
	size_t i = (size_t)(since + 1 - oldest);
	for (; i < ring_.size() && max > 0; i++, max--)
		out.push_back(ring_[i]);
	return ReadResult::Ok;
}

uint64_t EventBus::lastSeq(void) const
{
	std::lock_guard<std::mutex> lk(mtx_);
	return last_;
}

uint64_t EventBus::subscribe(std::function<void()> wake)
{
	std::lock_guard<std::mutex> lk(mtx_);
	if (subs_.size() >= cfg_.max_subscribers)
		return 0;
	uint64_t id = next_sub_++;
	subs_.emplace(id, std::move(wake));
	return id;
}

void EventBus::unsubscribe(uint64_t id)
{
	std::lock_guard<std::mutex> lk(mtx_);
	subs_.erase(id);
}

} /* namespace tgloggerd */

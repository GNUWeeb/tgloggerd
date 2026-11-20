// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__EVENT_BUS_HPP
#define TGLOGGERD__EVENT_BUS_HPP

#include <chrono>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <deque>
#include <functional>
#include <initializer_list>
#include <map>
#include <mutex>
#include <set>
#include <string>
#include <thread>
#include <vector>

#include <nlohmann/json.hpp>

namespace tgloggerd {

/*
 * What changed in the archive, as invalidation events for the web
 * (docs/design/web-live.md section 3, served by GET /v1/events,
 * docs/internal-api.md).
 *
 * The workers that write the rows publish an event after the write is
 * committed. Each event gets the next seq and goes into a bounded ring of
 * recent events, from which subscribers read at their own pace: a reader
 * that asks for events after a seq the ring no longer holds (too old, or
 * from an earlier run of the daemon) is told to resynchronise instead.
 * Nothing is queued per subscriber, so a slow one costs no memory.
 *
 * Counters move with nearly every message, so they are not published one
 * by one: countersMoved() collects the chats and users, and a timer
 * thread publishes them as one stats.changed per interval at most.
 *
 * Thread-safe. Publishing never blocks on a subscriber: a wakeup only
 * signals that there is something to read.
 */
class EventBus {
public:
	struct Config {
		/* Events kept for resuming. */
		size_t				ring = 16384;
		/* Least time between two stats.changed events. */
		std::chrono::milliseconds	stats_interval{1000};
		/* Most ids per list in one stats.changed; more set "more". */
		size_t				stats_max_ids = 1000;
		/* Subscribers at once; subscribe() refuses more. */
		size_t				max_subscribers = 16;
	};

	struct Event {
		uint64_t	seq;
		/* The event as compact JSON, "seq" and "kind" first. */
		std::string	json;
	};

	/*
	 * seq_base: the seq before the first event. The default is the
	 * current time in microseconds, so seqs keep increasing across
	 * restarts and a seq from an earlier run is always older than the
	 * ring (a resync), never mistaken for one of this run.
	 */
	EventBus(void);
	explicit EventBus(Config cfg);
	EventBus(Config cfg, uint64_t seq_base);
	~EventBus(void);

	EventBus(const EventBus &) = delete;
	EventBus &operator=(const EventBus &) = delete;

	/* Start/stop the stats timer thread. stop() flushes; idempotent. */
	void start(void);
	void stop(void);

	/*
	 * Publish one event of kind with fields (an object; "seq" and
	 * "kind" are set here). Returns its seq.
	 */
	uint64_t publish(const char *kind, const nlohmann::json &fields);

	uint64_t messageNew(int64_t chat_id, int64_t message_id);
	uint64_t messageEdited(int64_t chat_id, int64_t message_id);
	uint64_t messageDeleted(int64_t chat_id, int64_t message_id);

	/*
	 * fields: the parts of the user or group the write may have
	 * changed ("name", "photo", ...), when the writer knows; empty
	 * leaves them out, meaning anything may have changed.
	 */
	uint64_t userUpdated(int64_t user_id,
			     std::initializer_list<const char *> fields = {});
	uint64_t groupUpdated(int64_t chat_id,
			      std::initializer_list<const char *> fields = {});

	/* A file that is not a message's media (a user's or group's photo). */
	uint64_t fileStored(uint64_t file_id);
	/* A message's media or link-preview image. */
	uint64_t fileStored(uint64_t file_id, int64_t chat_id,
			    int64_t message_id);

	/*
	 * A chat's and/or a user's counters moved (0: none). Collected for
	 * the next stats.changed.
	 */
	void countersMoved(int64_t chat_id, int64_t user_id);

	/*
	 * Publish the collected counters as one stats.changed, if any.
	 * Returns its seq, or 0 when nothing was collected. The timer thread
	 * calls it once per stats_interval; exposed for tests.
	 */
	uint64_t flushStats(void);

	enum class ReadResult {
		/* out holds the events after since (maybe none). */
		Ok,
		/* since is not in the ring: resynchronise from *last. */
		Gap,
	};

	/*
	 * Append to out up to max events with seq > since, oldest first.
	 * since is valid when it is the last seq or an event still in the
	 * ring, or the one just before the oldest (nothing was missed).
	 * Anything else, older or newer, is a Gap. last is set to the last
	 * seq published either way.
	 */
	ReadResult readSince(uint64_t since, std::vector<Event> &out,
			     size_t max, uint64_t *last = nullptr) const;

	uint64_t lastSeq(void) const;

	/*
	 * Call wake (from the publishing thread, under the bus's lock; it
	 * must only signal, never call back into the bus) after each event.
	 * Returns an id for unsubscribe(), or 0 when max_subscribers are
	 * subscribed already. After unsubscribe() returns, wake is never
	 * called again.
	 */
	uint64_t subscribe(std::function<void()> wake);
	void unsubscribe(uint64_t id);

	const Config &config(void) const noexcept { return cfg_; }

private:
	void run(void);

	Config					cfg_;

	mutable std::mutex			mtx_;
	std::deque<Event>			ring_;
	uint64_t				last_;
	std::map<uint64_t, std::function<void()>> subs_;
	uint64_t				next_sub_ = 1;

	std::mutex				stats_mtx_;
	std::set<int64_t>			stats_chats_;
	std::set<int64_t>			stats_users_;

	std::mutex				run_mtx_;
	std::condition_variable			run_cv_;
	bool					stopping_ = false;
	std::thread				thread_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__EVENT_BUS_HPP */

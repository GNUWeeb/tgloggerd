// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__FORWARD_QUEUE_HPP
#define DISCORDD__FORWARD_QUEUE_HPP

#include <chrono>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <deque>
#include <functional>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <thread>

namespace discordd {

/*
 * Work queues keyed by Discord bot, so the gateway callbacks only enqueue
 * and return: gwdiscord reads heartbeat acknowledgements on the thread
 * that runs them, and a callback that blocks past a heartbeat interval
 * makes the gateway resume.
 *
 * Each key has its own FIFO and its own worker thread, started on its
 * first job: one bot's events run in arrival order (a route's create,
 * edit and delete stay in order), and bots never wait on each other. A
 * key's queue holds at most `cap` jobs; a job pushed onto a full queue is
 * dropped and counted.
 */
class ForwardQueue {
public:
	using Job = std::function<void(void)>;
	using Log = std::function<void(const std::string &)>;

	ForwardQueue(size_t cap, Log warn);
	~ForwardQueue(void);

	ForwardQueue(const ForwardQueue &) = delete;
	ForwardQueue &operator=(const ForwardQueue &) = delete;

	/* Queue @job on @key's worker; false if the queue was full or the
	 * queue is stopping (the job is dropped). Never blocks on a job. */
	bool push(uint64_t key, Job job);

	/* Let every worker finish its current job, drop what is queued
	 * (logged with its count), and join them. Idempotent. */
	void stop(void);

	/* Jobs dropped so far because a queue was full. */
	uint64_t dropped(void) const;

private:
	struct Worker {
		std::deque<Job>		jobs;
		std::thread		thr;
	};

	void run(uint64_t key, Worker *w);

	const size_t			cap_;
	Log				warn_;
	mutable std::mutex		mtx_;
	std::condition_variable		cv_;
	std::map<uint64_t, std::unique_ptr<Worker>> workers_;
	bool				stopping_ = false;
	uint64_t			dropped_ = 0;
};

/*
 * Spaces the sends to each Telegram chat: a send waits until @gap has
 * passed since the previous one to the same chat, whichever worker made
 * it. Telegram lets a bot send about 20 messages a minute to one group
 * and about one a second to one private chat; going faster ends in flood
 * waits, which TDLib sits out while the send times out.
 */
class ChatPacer {
public:
	using Clock = std::chrono::steady_clock;
	using Sleep = std::function<void(Clock::duration)>;
	using Now = std::function<Clock::time_point(void)>;

	/* @group_gap for chats with negative ids (groups and channels),
	 * @private_gap for the others. @now and @sleep are for tests. */
	ChatPacer(Clock::duration group_gap, Clock::duration private_gap,
		  Now now = nullptr, Sleep sleep = nullptr);

	/* Wait for @chat_id's turn, then take it. */
	void wait_turn(int64_t chat_id);

private:
	const Clock::duration			group_gap_;
	const Clock::duration			private_gap_;
	Now					now_;
	Sleep					sleep_;
	std::mutex				mtx_;
	std::map<int64_t, Clock::time_point>	next_;
};

} /* namespace discordd */

#endif /* #ifndef DISCORDD__FORWARD_QUEUE_HPP */

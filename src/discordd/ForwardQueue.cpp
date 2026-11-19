// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ForwardQueue.hpp"

#include <exception>
#include <utility>
#include <vector>

namespace discordd {

ForwardQueue::ForwardQueue(size_t cap, Log warn)
	: cap_(cap ? cap : 1), warn_(std::move(warn))
{
}

ForwardQueue::~ForwardQueue(void)
{
	stop();
}

bool ForwardQueue::push(uint64_t key, Job job)
{
	uint64_t n = 0;
	{
		std::lock_guard<std::mutex> lk(mtx_);
		if (stopping_)
			return false;
		auto &w = workers_[key];
		if (!w) {
			w = std::make_unique<Worker>();
			Worker *wp = w.get();
			wp->thr = std::thread([this, key, wp] { run(key, wp); });
		}
		if (w->jobs.size() < cap_) {
			w->jobs.push_back(std::move(job));
			cv_.notify_all();
			return true;
		}
		n = ++dropped_;
	}
	if (warn_)
		warn_("forward queue of bot " + std::to_string(key) +
		      " is full: event dropped (" + std::to_string(n) +
		      " dropped so far)");
	return false;
}

void ForwardQueue::run(uint64_t key, Worker *w)
{
	for (;;) {
		Job job;
		{
			std::unique_lock<std::mutex> lk(mtx_);
			cv_.wait(lk, [&] { return stopping_ || !w->jobs.empty(); });
			if (stopping_)
				return;
			job = std::move(w->jobs.front());
			w->jobs.pop_front();
		}
		try {
			job();
		} catch (const std::exception &e) {
			if (warn_)
				warn_("forward job of bot " + std::to_string(key) +
				      " failed: " + e.what());
		}
	}
}

void ForwardQueue::stop(void)
{
	std::vector<std::thread> thrs;
	size_t left = 0;
	{
		std::lock_guard<std::mutex> lk(mtx_);
		if (stopping_ && workers_.empty())
			return;
		stopping_ = true;
		for (auto &kv : workers_) {
			left += kv.second->jobs.size();
			kv.second->jobs.clear();
			thrs.push_back(std::move(kv.second->thr));
		}
		cv_.notify_all();
	}
	for (auto &t : thrs)
		if (t.joinable())
			t.join();
	{
		std::lock_guard<std::mutex> lk(mtx_);
		workers_.clear();
	}
	if (left && warn_)
		warn_("stopping: " + std::to_string(left) +
		      " queued event(s) not forwarded");
}

uint64_t ForwardQueue::dropped(void) const
{
	std::lock_guard<std::mutex> lk(mtx_);
	return dropped_;
}

ChatPacer::ChatPacer(Clock::duration group_gap, Clock::duration private_gap,
		     Now now, Sleep sleep)
	: group_gap_(group_gap), private_gap_(private_gap),
	  now_(now ? std::move(now) : Now([] { return Clock::now(); })),
	  sleep_(sleep ? std::move(sleep)
		       : Sleep([](Clock::duration d) {
			       std::this_thread::sleep_for(d);
		       }))
{
}

void ChatPacer::wait_turn(int64_t chat_id)
{
	const Clock::duration gap = chat_id < 0 ? group_gap_ : private_gap_;
	Clock::time_point at;
	{
		/* Reserve the slot under the lock, wait outside it, so two
		 * workers sending to one chat queue up instead of racing. */
		std::lock_guard<std::mutex> lk(mtx_);
		const Clock::time_point now = now_();
		auto it = next_.find(chat_id);
		at = (it == next_.end() || it->second < now) ? now : it->second;
		next_[chat_id] = at + gap;
	}
	const Clock::time_point now = now_();
	if (at > now)
		sleep_(at - now);
}

} /* namespace discordd */

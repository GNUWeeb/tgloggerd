// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "FetchState.hpp"

#include <algorithm>
#include <cstdlib>
#include <vector>

namespace tgloggerd {
namespace fetch {

uint64_t rangeLength(uint64_t offset, uint64_t length, uint64_t size,
		     uint64_t chunk)
{
	if (size != 0 && offset >= size)
		return 0;
	uint64_t n = length == 0 ? chunk : std::min(length, chunk);
	if (size != 0)
		n = std::min(n, size - offset);
	return n;
}

uint64_t availableBytes(uint64_t offset, uint64_t prefix, uint64_t size)
{
	if (size == 0)
		return prefix;
	if (offset >= size)
		return 0;
	return std::min(prefix, size - offset);
}

ErrorClass classifyError(int32_t code, const std::string &message)
{
	/* TDLib::kRequestNotReady / kRequestTimeout. */
	if (code == 503)
		return { ErrorKind::NotReady, 5 };
	if (code == 504)
		return { ErrorKind::Timeout, 0 };
	if (code == 429) {
		/* "Too Many Requests: retry after 17" */
		int secs = 5;
		auto pos = message.find("retry after ");
		if (pos != std::string::npos) {
			long v = strtol(message.c_str() + pos + 12, nullptr, 10);
			if (v > 0)
				secs = (int)std::min(v, 3600L);
		}
		return { ErrorKind::FloodWait, secs };
	}
	if (code == 400 || code == 403 || code == 404 || code == 406 ||
	    message.find("FILE_REFERENCE") != std::string::npos)
		return { ErrorKind::Access, 0 };
	/* Unauthorized (401) means the session is going away. */
	if (code == 401)
		return { ErrorKind::NotReady, 5 };
	return { ErrorKind::Other, 5 };
}

void GoneCache::add(uint64_t id, Clock::time_point now)
{
	std::lock_guard<std::mutex> lk(mtx_);
	if (until_.size() >= cap_) {
		for (auto it = until_.begin(); it != until_.end();)
			it = it->second <= now ? until_.erase(it) : std::next(it);
		/* Still full of live entries: forgetting them only costs a
		 * question to Telegram each. */
		if (until_.size() >= cap_)
			until_.clear();
	}
	until_[id] = now + ttl_;
}

bool GoneCache::contains(uint64_t id, Clock::time_point now)
{
	std::lock_guard<std::mutex> lk(mtx_);
	auto it = until_.find(id);
	if (it == until_.end())
		return false;
	if (it->second <= now) {
		until_.erase(it);
		return false;
	}
	return true;
}

void GoneCache::purge(Clock::time_point now)
{
	std::lock_guard<std::mutex> lk(mtx_);
	for (auto it = until_.begin(); it != until_.end();)
		it = it->second <= now ? until_.erase(it) : std::next(it);
}

size_t GoneCache::size(void)
{
	std::lock_guard<std::mutex> lk(mtx_);
	return until_.size();
}

bool ActiveLimit::tryAcquire(void)
{
	std::lock_guard<std::mutex> lk(mtx_);
	if (active_ >= max_)
		return false;
	active_++;
	return true;
}

void ActiveLimit::release(void)
{
	std::lock_guard<std::mutex> lk(mtx_);
	if (active_ > 0)
		active_--;
}

size_t ActiveLimit::active(void)
{
	std::lock_guard<std::mutex> lk(mtx_);
	return active_;
}

std::shared_ptr<LeaseTable::Entry> LeaseTable::acquire(int32_t file_id)
{
	std::lock_guard<std::mutex> lk(mtx_);
	auto &e = files_[file_id];
	if (!e)
		e = std::make_shared<Entry>();
	e->leases++;
	return e;
}

void LeaseTable::release(int32_t file_id, Clock::time_point now, bool partial,
			 std::optional<Clock::time_point> prune_at)
{
	std::lock_guard<std::mutex> lk(mtx_);
	auto it = files_.find(file_id);
	if (it == files_.end())
		return;
	Entry &e = *it->second;
	if (e.leases > 0)
		e.leases--;
	e.last_used = now;
	if (partial) {
		/* A ranged download supersedes a scheduled prune: the file
		 * is in use as a partial now and goes by the idle time. */
		e.partial = true;
		e.prune_at.reset();
	} else if (prune_at && !e.partial) {
		e.prune_at = prune_at;
	}
	if (e.leases == 0 && !e.partial && !e.prune_at)
		files_.erase(it);
}

size_t LeaseTable::reap(Clock::time_point now, Clock::duration idle,
			const std::function<void(int32_t)> &del)
{
	std::lock_guard<std::mutex> lk(mtx_);
	size_t n = 0;
	for (auto it = files_.begin(); it != files_.end();) {
		const Entry &e = *it->second;
		bool due = e.leases == 0 &&
			   ((e.partial && e.last_used + idle <= now) ||
			    (e.prune_at && *e.prune_at <= now));
		if (!due) {
			++it;
			continue;
		}
		del(it->first);
		n++;
		it = files_.erase(it);
	}
	return n;
}

size_t LeaseTable::size(void)
{
	std::lock_guard<std::mutex> lk(mtx_);
	return files_.size();
}

} /* namespace fetch */
} /* namespace tgloggerd */

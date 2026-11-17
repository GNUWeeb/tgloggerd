// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__STORE_HOLDS_HPP
#define TGLOGGERD__STORE_HOLDS_HPP

#include <cstdint>
#include <functional>
#include <mutex>
#include <unordered_map>

namespace tgloggerd {

/*
 * Counts the store jobs that still have to read TDLib's local copy of a
 * file, so the copy is pruned once, after the last of them.
 *
 * TDLib keeps one file per remote file: several messages carrying the same
 * media each get a store job on the same local path. If every job pruned
 * the copy when it was done with it, the first to finish would delete it
 * under the others, which then fail to hash it and leave their message
 * unlinked. Instead each job takes a hold when it is queued (hold()) and
 * gives it back when it ends (release()); the copy is pruned when the last
 * hold goes and at least one of the jobs stored the bytes.
 *
 * Thread-safe: hold() runs on the TDLib loop thread, release() on the file
 * workers. @prune runs on the releasing thread, outside the lock.
 */
class StoreHolds {
public:
	using PruneFn = std::function<void(int32_t)>;

	explicit StoreHolds(PruneFn prune) : prune_(std::move(prune)) {}

	StoreHolds(const StoreHolds &) = delete;
	StoreHolds &operator=(const StoreHolds &) = delete;

	/* A store job reading local file @fid was queued. */
	void hold(int32_t fid)
	{
		std::lock_guard<std::mutex> lk(mtx_);
		holds_[fid].jobs++;
	}

	/*
	 * A store job on @fid ended; @stored says whether it secured the
	 * bytes. Prunes the copy if this was the last job and any of them
	 * stored it.
	 */
	void release(int32_t fid, bool stored)
	{
		bool prune = false;
		{
			std::lock_guard<std::mutex> lk(mtx_);
			auto it = holds_.find(fid);
			if (it == holds_.end())
				return;
			it->second.stored |= stored;
			if (--it->second.jobs > 0)
				return;
			prune = it->second.stored;
			holds_.erase(it);
		}
		if (prune && prune_)
			prune_(fid);
	}

	/* Whether a store job still needs local file @fid. */
	bool held(int32_t fid) const
	{
		std::lock_guard<std::mutex> lk(mtx_);
		return holds_.count(fid) != 0;
	}

	/*
	 * Releases the hold of one job when it goes out of scope, also when
	 * the job throws. Set stored once the bytes are durable.
	 */
	class Job {
	public:
		Job(StoreHolds &h, int32_t fid) : h_(h), fid_(fid) {}
		~Job(void) { h_.release(fid_, stored); }
		Job(const Job &) = delete;
		Job &operator=(const Job &) = delete;

		bool stored = false;

	private:
		StoreHolds	&h_;
		int32_t		fid_;
	};

private:
	struct Entry {
		unsigned	jobs = 0;
		bool		stored = false;
	};

	PruneFn					prune_;
	mutable std::mutex			mtx_;
	std::unordered_map<int32_t, Entry>	holds_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__STORE_HOLDS_HPP */

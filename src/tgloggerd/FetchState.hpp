// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__FETCH_STATE_HPP
#define TGLOGGERD__FETCH_STATE_HPP

#include <chrono>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <optional>
#include <string>
#include <unordered_map>

/*
 * The bookkeeping of the on-demand fetch service (FileFetcher,
 * storage-cache.md section 8), kept apart from TDLib and the database so
 * it can be reasoned about and tested on its own: range arithmetic, how a
 * TDLib error is to be answered, the memory of files gone from Telegram,
 * the limit on concurrent fetches, and the per-file leases that serialize
 * downloads and decide when TDLib's copy may be deleted.
 */
namespace tgloggerd {
namespace fetch {

using Clock = std::chrono::steady_clock;

/*
 * How many bytes from @offset to ask TDLib for: @length, or one @chunk
 * when @length is 0 (open-ended), at most @chunk, and never past @size
 * when it is known (not 0). 0 when @offset is at or past a known @size.
 */
uint64_t rangeLength(uint64_t offset, uint64_t length, uint64_t size,
		     uint64_t chunk);

/*
 * Bytes to report as available from @offset, given TDLib's downloaded
 * prefix there: the prefix, but never past a known @size.
 */
uint64_t availableBytes(uint64_t offset, uint64_t prefix, uint64_t size);

/* What a TDLib error means for a fetch. */
enum class ErrorKind {
	NotReady,	/* client not authorized or closing: 503 */
	Timeout,	/* no answer in time: 504 */
	FloodWait,	/* Telegram asks to wait: 503, retry_after */
	Access,		/* reference expired, no access, not found */
	Other,		/* anything else, transient as far as we know: 503 */
};

struct ErrorClass {
	ErrorKind	kind;
	int		retry_after;	/* seconds, for the 503s */
};

/*
 * Classify a td_api::error by @code and @message. TDLib::request's own
 * codes (503 not ready, 504 timeout) come first; 400, 403, 404 and 406,
 * and any message naming a file reference, are access errors, which a
 * fresh source may cure and which otherwise mean the file is gone; 429
 * is a flood wait whose "retry after N" is honoured.
 */
ErrorClass classifyError(int32_t code, const std::string &message);

/*
 * Files found gone from Telegram, remembered for a while so repeated
 * requests are answered at once instead of asking Telegram again.
 * Thread-safe.
 */
class GoneCache {
public:
	explicit GoneCache(Clock::duration ttl, size_t cap = 100000)
		: ttl_(ttl), cap_(cap) {}

	void add(uint64_t id, Clock::time_point now);
	bool contains(uint64_t id, Clock::time_point now);
	/* Drop expired entries. */
	void purge(Clock::time_point now);
	size_t size(void);

private:
	std::mutex					mtx_;
	std::unordered_map<uint64_t, Clock::time_point>	until_;
	Clock::duration					ttl_;
	size_t						cap_;
};

/* At most N fetches at once. Thread-safe. */
class ActiveLimit {
public:
	explicit ActiveLimit(size_t max) : max_(max) {}

	bool tryAcquire(void);
	void release(void);
	size_t active(void);

	/* A slot held for a scope; held() is false when the limit was hit. */
	class Slot {
	public:
		explicit Slot(ActiveLimit &l) : l_(l), held_(l.tryAcquire()) {}
		~Slot(void) { if (held_) l_.release(); }
		Slot(const Slot &) = delete;
		Slot &operator=(const Slot &) = delete;
		bool held(void) const { return held_; }
	private:
		ActiveLimit	&l_;
		bool		held_;
	};

private:
	std::mutex	mtx_;
	size_t		max_;
	size_t		active_ = 0;
};

/*
 * Per TDLib file: a lock that serializes its downloads (a downloadFile
 * with another range ends the wait of one in flight), how many fetches
 * hold it (a leased file is never deleted), and what to do with TDLib's
 * copy once nobody does. Thread-safe.
 */
class LeaseTable {
public:
	struct Entry {
		std::timed_mutex		mtx;
		int				leases = 0;
		Clock::time_point		last_used{};
		/* A ranged (big file) download: delete after the idle time. */
		bool				partial = false;
		/* A stored small file: delete TDLib's copy at this time. */
		std::optional<Clock::time_point>	prune_at;
	};

	/* Take a lease on @file_id, creating its entry. */
	std::shared_ptr<Entry> acquire(int32_t file_id);

	/*
	 * Give the lease back. @partial marks the file as holding a ranged
	 * download; @prune_at schedules the deletion of a stored copy. An
	 * entry with no lease and nothing to delete is forgotten.
	 */
	void release(int32_t file_id, Clock::time_point now, bool partial,
		     std::optional<Clock::time_point> prune_at);

	/*
	 * Delete what is due: every unleased entry that is partial and idle
	 * for @idle, or whose prune_at has passed. @del is called for each
	 * under the table's lock (so no fetch can lease the file in between)
	 * and must not block; the entry is forgotten either way. Returns how
	 * many were handed to @del.
	 */
	size_t reap(Clock::time_point now, Clock::duration idle,
		    const std::function<void(int32_t)> &del);

	size_t size(void);

private:
	std::mutex						mtx_;
	std::unordered_map<int32_t, std::shared_ptr<Entry>>	files_;
};

} /* namespace fetch */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__FETCH_STATE_HPP */

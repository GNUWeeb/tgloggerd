// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "daemon/AccessReport.hpp"

#include "daemon/Api.hpp"

#include <drogon/HttpAppFramework.h>
#include <drogon/utils/coroutine.h>

#include <algorithm>
#include <atomic>
#include <cstdio>
#include <mutex>
#include <unordered_set>
#include <vector>

namespace tgweb::daemon::access {

namespace {

constexpr double kFlushSecs = 60.0;
/* The daemon takes at most this many ids per request. */
constexpr size_t kBatch = 1000;
/* Distinct ids kept between flushes. A minute of serving never comes near
 * it; it only bounds memory should a flush stall. */
constexpr size_t kMaxPending = 200000;

std::atomic<bool> g_on{false};
std::mutex g_mu;
std::unordered_set<uint64_t> g_ids;	/* guarded by g_mu */

/* One flush at a time: a slow daemon must not stack them up. */
std::atomic<bool> g_flushing{false};
/* The last flush failed; log the next failure only after a success, so a
 * daemon that is away for a day costs one line, not 1440. */
std::atomic<bool> g_failing{false};

drogon::Task<> flush(void)
{
	std::vector<uint64_t> ids;
	{
		std::lock_guard<std::mutex> lk(g_mu);
		if (g_ids.empty())
			co_return;
		ids.assign(g_ids.begin(), g_ids.end());
		g_ids.clear();
	}

	for (size_t i = 0; i < ids.size(); i += kBatch) {
		size_t end = std::min(ids.size(), i + kBatch);
		std::vector<uint64_t> batch(ids.begin() + (long)i,
					    ids.begin() + (long)end);
		api::Reply r = co_await api::recordAccess(std::move(batch));
		if (!r.ok()) {
			/* Drop this flush's ids, including the batches not
			 * sent yet: a file served again is reported again,
			 * and accessed_at only needs to be roughly right. */
			if (!g_failing.exchange(true))
				fprintf(stderr, "tgloggerd-api: access report "
					"failed (%s); %zu ids dropped, and no more "
					"of this until a report succeeds\n",
					r.error.c_str(), ids.size() - i);
			co_return;
		}
	}
	if (g_failing.exchange(false))
		fprintf(stderr, "tgloggerd-api: access reports work again\n");
}

void tick(void)
{
	if (g_flushing.exchange(true))
		return;
	drogon::async_run([]() -> drogon::Task<> {
		try {
			co_await flush();
		} catch (const std::exception &e) {
			fprintf(stderr, "tgloggerd-api: access report: %s\n",
				e.what());
		}
		g_flushing = false;
	});
}

} /* namespace */

void start(void)
{
	if (!api::enabled())
		return;
	g_on = true;
	/* The main loop exists only once the app runs. */
	drogon::app().registerBeginningAdvice([]() {
		drogon::app().getLoop()->runEvery(kFlushSecs, tick);
	});
}

void record(uint64_t id)
{
	if (!g_on.load(std::memory_order_relaxed))
		return;
	std::lock_guard<std::mutex> lk(g_mu);
	if (g_ids.size() < kMaxPending)
		g_ids.insert(id);
}

} /* namespace tgweb::daemon::access */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * discordd_forward_queue_test -- the per-bot worker queues that keep
 * forwarding off the gateway thread, and the per-chat send pacing.
 */
#include "../ForwardQueue.hpp"

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstdio>
#include <mutex>
#include <stdexcept>
#include <string>
#include <thread>
#include <vector>

using namespace discordd;
using namespace std::chrono_literals;

static int g_failures;

#define CHECK(cond, what)                                                     \
	do {                                                                  \
		if (!(cond)) {                                                \
			printf("  FAIL: %s\n", (what));                       \
			g_failures++;                                         \
		} else {                                                      \
			printf("  ok:   %s\n", (what));                       \
		}                                                             \
	} while (0)

/* A gate a job blocks on until the test opens it. */
struct Gate {
	std::mutex		mtx;
	std::condition_variable	cv;
	bool			open = false;

	void wait(void)
	{
		std::unique_lock<std::mutex> lk(mtx);
		cv.wait(lk, [this] { return open; });
	}
	void release(void)
	{
		{
			std::lock_guard<std::mutex> lk(mtx);
			open = true;
		}
		cv.notify_all();
	}
};

/* Poll @cond for up to 5 s. */
template <typename F> static bool eventually(F cond)
{
	for (int i = 0; i < 500; i++) {
		if (cond())
			return true;
		std::this_thread::sleep_for(10ms);
	}
	return cond();
}

static void test_queue(void)
{
	printf("per-bot queues\n");
	std::vector<std::string> warns;
	std::mutex wmtx;
	ForwardQueue q(3, [&](const std::string &w) {
		std::lock_guard<std::mutex> lk(wmtx);
		warns.push_back(w);
	});

	/* Bot 1's first job blocks, like a send to a slow Telegram. */
	Gate gate;
	std::atomic<bool> started{false};
	std::mutex omtx;
	std::vector<int> order;
	const auto t0 = std::chrono::steady_clock::now();
	q.push(1, [&] {
		started = true;
		gate.wait();
		std::lock_guard<std::mutex> lk(omtx);
		order.push_back(0);
	});
	CHECK(eventually([&] { return started.load(); }),
	      "the first job runs on a worker");
	for (int i = 1; i <= 3; i++) {
		bool ok = q.push(1, [&, i] {
			std::lock_guard<std::mutex> lk(omtx);
			order.push_back(i);
		});
		CHECK(ok, ("job " + std::to_string(i) + " queued").c_str());
	}
	const auto pushed = std::chrono::steady_clock::now() - t0;
	CHECK(pushed < 1s, "pushing never waits for the blocked job");

	CHECK(!q.push(1, [] {}), "a full queue refuses the job");
	CHECK(q.dropped() == 1, "the drop is counted");
	{
		std::lock_guard<std::mutex> lk(wmtx);
		CHECK(warns.size() == 1 &&
			      warns[0].find("full") != std::string::npos,
		      "the drop is logged");
	}

	/* Another bot runs while bot 1 is blocked. */
	std::atomic<bool> other{false};
	q.push(2, [&] { other = true; });
	CHECK(eventually([&] { return other.load(); }),
	      "bot 2 does not wait for bot 1");

	gate.release();
	CHECK(eventually([&] {
		      std::lock_guard<std::mutex> lk(omtx);
		      return order.size() == 4;
	      }),
	      "bot 1's jobs all run once unblocked");
	{
		std::lock_guard<std::mutex> lk(omtx);
		CHECK(order == std::vector<int>({ 0, 1, 2, 3 }),
		      "in the order they were pushed");
	}

	/* A throwing job is logged and the worker carries on. */
	std::atomic<bool> after{false};
	q.push(1, [] { throw std::runtime_error("boom"); });
	q.push(1, [&] { after = true; });
	CHECK(eventually([&] { return after.load(); }),
	      "a job that throws does not stop the worker");
	q.stop();
}

static void test_stop(void)
{
	printf("stopping\n");
	std::vector<std::string> warns;
	std::mutex wmtx;
	ForwardQueue q(10, [&](const std::string &w) {
		std::lock_guard<std::mutex> lk(wmtx);
		warns.push_back(w);
	});
	Gate gate;
	std::atomic<bool> started{false}, ran_queued{false};
	q.push(7, [&] {
		started = true;
		gate.wait();
	});
	eventually([&] { return started.load(); });
	q.push(7, [&] { ran_queued = true; });
	q.push(7, [&] { ran_queued = true; });

	std::thread stopper([&] { q.stop(); });
	std::this_thread::sleep_for(50ms);
	gate.release(); /* the current job finishes */
	stopper.join();
	CHECK(!ran_queued, "queued jobs are dropped on stop");
	{
		std::lock_guard<std::mutex> lk(wmtx);
		bool found = false;
		for (const auto &w : warns)
			found |= w.find("2 queued event(s)") != std::string::npos;
		CHECK(found, "and their number is logged");
	}
	CHECK(!q.push(7, [] {}), "no job is taken after stop");
	q.stop(); /* idempotent */
}

static void test_pacer(void)
{
	printf("per-chat pacing\n");
	using Clock = ChatPacer::Clock;
	Clock::time_point now{};
	std::vector<Clock::duration> sleeps;
	ChatPacer p(3s, 1s, [&] { return now; },
		    [&](Clock::duration d) {
			    sleeps.push_back(d);
			    now += d;
		    });

	p.wait_turn(-100);
	CHECK(sleeps.empty(), "the first send to a group goes at once");
	p.wait_turn(-100);
	CHECK(sleeps.size() == 1 && sleeps[0] == 3s,
	      "the next one waits for the group gap");
	p.wait_turn(-200);
	CHECK(sleeps.size() == 1, "another group does not wait");
	p.wait_turn(55);
	p.wait_turn(55);
	CHECK(sleeps.size() == 2 && sleeps[1] == 1s,
	      "a private chat waits for the private gap");
	now += 10s;
	p.wait_turn(-100);
	CHECK(sleeps.size() == 2, "after a pause there is no wait");
}

int main(void)
{
	test_queue();
	test_stop();
	test_pacer();
	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

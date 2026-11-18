// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * bot_watch_test -- BotWatch over a fake read of the bots: no database.
 * The DB and EventBus members BotWatch.cpp names for the daemon's own
 * constructor are stubbed below; the test uses the other one.
 */
#include <tgloggerd/BotWatch.hpp>
#include <tgloggerd/DB.hpp>
#include <tgloggerd/EventBus.hpp>

#include <cstdio>
#include <stdexcept>
#include <vector>

using namespace tgloggerd;

std::map<uint64_t, std::string> tgloggerd::DB::loadBotSignatures(void)
{
	return {};
}

uint64_t tgloggerd::EventBus::publish(const char *, const nlohmann::json &)
{
	return 0;
}

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

int main(void)
{
	std::mutex mtx;
	std::map<uint64_t, std::string> rows = {{1, "a"}, {1000000000, "x"}};
	bool fail = false;
	std::vector<uint64_t> pub;

	BotWatch w(
		[&] {
			std::lock_guard<std::mutex> lk(mtx);
			if (fail)
				throw std::runtime_error("database down");
			return rows;
		},
		[&](uint64_t id) {
			std::lock_guard<std::mutex> lk(mtx);
			pub.push_back(id);
		},
		std::chrono::milliseconds(10), nullptr);

	printf("test: bot.updated from reads of the bots\n");
	w.poll();
	CHECK(pub.empty(), "the first read publishes nothing");
	w.poll();
	CHECK(pub.empty(), "an unchanged read publishes nothing");

	rows[1] = "b";
	rows[2] = "new";
	rows.erase(1000000000);
	w.poll();
	CHECK((pub == std::vector<uint64_t>{1, 2, 1000000000}),
	      "a changed, an added and a removed bot, once each");

	pub.clear();
	fail = true;
	w.poll();
	CHECK(pub.empty(), "a failed read publishes nothing");
	fail = false;
	rows[2] = "newer";
	w.poll();
	CHECK((pub == std::vector<uint64_t>{2}),
	      "the next read compares with the last good one");

	pub.clear();
	w.start();
	{
		std::lock_guard<std::mutex> lk(mtx);
		rows[1] = "c";
	}
	bool seen = false;
	for (int i = 0; i < 500 && !seen; i++) {
		std::this_thread::sleep_for(std::chrono::milliseconds(10));
		std::lock_guard<std::mutex> lk(mtx);
		seen = !pub.empty() && pub[0] == 1;
	}
	w.stop();
	CHECK(seen, "the thread reads every interval");

	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}

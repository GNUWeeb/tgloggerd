// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * chat_recheck_test -- the pure parts of a chat recheck (ChatRecheck.hpp):
 * the queue's limits, the choice of the messages marked deleted and the
 * restart catch-up's chats, over plain data.
 */
#include <tgloggerd/ChatRecheck.hpp>

#include <cstdio>
#include <string>
#include <vector>

using namespace tgloggerd;
using namespace tgloggerd::recheck;
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

static void testNewestLimit(void)
{
	printf("newest stretch: once a minute\n");
	const Clock::time_point t0 = Clock::now();
	Queue q;

	CHECK(q.push({ -100, 0, false }, t0) == Admit::Queued, "queued");
	CHECK(q.push({ -100, 0, false }, t0 + 1s) == Admit::AlreadyQueued,
	      "again while waiting: queued");
	auto it = q.pop(t0 + 30s);
	CHECK(it && it->chat_id == -100 && it->around == 0, "popped");
	CHECK(q.push({ -100, 0, false }, t0 + 61s) == Admit::Recent,
	      "61 s after queueing, 31 s after it ran: recent");
	CHECK(q.push({ -100, 0, false }, t0 + 90s) == Admit::Queued,
	      "60 s after it ran: queued");
	CHECK(q.push({ -200, 0, false }, t0 + 90s) == Admit::Queued,
	      "another chat is limited on its own");

	Queue q2;
	CHECK(q2.push({ -1, 0, false }, t0) == Admit::Queued, "queued (q2)");
	q2.pop(t0);
	CHECK(q2.push({ -1, 0, false }, t0 + 59s) == Admit::Recent, "59 s: recent");
	CHECK(q2.push({ -1, 0, false }, t0 + 60s) == Admit::Queued, "60 s: queued");
}

static void testOlderLimit(void)
{
	printf("older stretch: once a day per (chat, around)\n");
	const Clock::time_point t0 = Clock::now();
	Queue q;

	CHECK(q.push({ -100, 5000, false }, t0) == Admit::Queued, "queued");
	q.pop(t0);
	CHECK(q.push({ -100, 5000, false }, t0 + 2h) == Admit::Recent,
	      "2 h later: recent");
	CHECK(q.push({ -100, 5001, false }, t0 + 2h) == Admit::Queued,
	      "another around in the same chat: queued");
	CHECK(q.push({ -100, 0, false }, t0 + 2h) == Admit::Queued,
	      "the newest of the same chat: queued");
	CHECK(q.push({ -100, 5000, false }, t0 + 23h + 59min) == Admit::Recent,
	      "23:59 later: recent");
	CHECK(q.push({ -100, 5000, false }, t0 + 24h) == Admit::Queued,
	      "24 h later: queued");
}

static void testBound(void)
{
	printf("queue bound\n");
	const Clock::time_point t0 = Clock::now();
	Queue q(3);

	CHECK(q.push({ -1, 0, false }, t0) == Admit::Queued, "1 queued");
	CHECK(q.push({ -2, 0, true }, t0) == Admit::Queued, "2 queued");
	CHECK(q.push({ -3, 7, false }, t0) == Admit::Queued, "3 queued");
	CHECK(q.size() == 3, "size 3");
	CHECK(q.push({ -4, 0, false }, t0) == Admit::Busy, "4th: busy");
	CHECK(q.push({ -1, 0, false }, t0) == Admit::AlreadyQueued,
	      "a waiting one while full: queued, not busy");
	q.pop(t0);
	CHECK(q.push({ -4, 0, false }, t0 + 1s) == Admit::Queued,
	      "busy did not count as recent: queued once there is room");

	Queue d(kQueueMax);
	size_t queued = 0;
	for (int64_t c = 1; c <= (int64_t)kQueueMax + 5; c++)
		queued += d.push({ -c, 0, true }, t0) == Admit::Queued;
	CHECK(queued == kQueueMax && d.size() == kQueueMax,
	      "the default bound holds kQueueMax");
}

static void testOrder(void)
{
	printf("order: asked for before background\n");
	const Clock::time_point t0 = Clock::now();
	Queue q;

	q.push({ -1, 0, true }, t0);
	q.push({ -2, 0, true }, t0);
	q.push({ -3, 0, false }, t0);
	q.push({ -4, 9, false }, t0);
	std::vector<int64_t> got;
	while (auto it = q.pop(t0))
		got.push_back(it->chat_id);
	CHECK((got == std::vector<int64_t>{ -3, -4, -1, -2 }),
	      "asked for first, each kind in order");
	CHECK(q.empty(), "empty after");
	CHECK(q.push({ -1, 0, false }, t0 + 1s) == Admit::Recent,
	      "a background recheck counts as recent too");
}

static void testReasons(void)
{
	printf("reasons\n");
	CHECK(std::string(admitReason(Admit::Recent)) == "recent", "recent");
	CHECK(std::string(admitReason(Admit::Busy)) == "busy", "busy");
	CHECK(std::string(admitReason(Admit::AlreadyQueued)) == "queued", "queued");
	CHECK(std::string(admitReason(Admit::Queued)).empty(), "202 has none");
}

static Page newest(std::vector<int64_t> ids, size_t total = 0)
{
	Page p;
	p.around = 0;
	p.total = total ? total : ids.size();
	p.ids = std::move(ids);
	return p;
}

static Page older(int64_t around, std::vector<int64_t> ids)
{
	Page p;
	p.around = around;
	p.total = ids.size();
	p.ids = std::move(ids);
	return p;
}

static std::vector<int64_t> range(int64_t from, int64_t to)
{
	std::vector<int64_t> v;
	for (int64_t i = from; i <= to; i++)
		v.push_back(i);
	return v;
}

static std::vector<int64_t> without(std::vector<int64_t> v,
				    const std::vector<int64_t> &drop)
{
	std::vector<int64_t> out;
	for (int64_t x : v) {
		bool d = false;
		for (int64_t y : drop)
			d = d || x == y;
		if (!d)
			out.push_back(x);
	}
	return out;
}

static void testSelection(void)
{
	printf("deletion selection\n");

	/* A full newest page, 149..200 without 160 and 170 (50 messages). */
	{
		Page p = newest(without(range(149, 200), { 160, 170 }));
		Stored s;
		s.between = range(150, 199);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded, "full newest page: concluded");
		CHECK((v.deleted == std::vector<int64_t>{ 160, 170 }),
		      "full newest page: the two missing ones");
	}

	/* Never outside the answer: stored ids at or beyond the span edges
	 * are left alone even if the caller passes them. */
	{
		Page p = newest(range(151, 200));
		Stored s;
		s.between = { 100, 150, 151, 175, 200, 201, 250 };
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && v.deleted.empty(),
		      "ids at or outside the span are never marked");
	}

	{
		Verdict v = selectDeleted(newest({}), Stored{ { 5, 6 }, true });
		CHECK(!v.concluded && v.deleted.empty(), "empty answer: nothing");
	}
	{
		Verdict v = selectDeleted(newest({ 10 }), Stored{ { 5, 6 }, true });
		CHECK(!v.concluded && v.deleted.empty(), "one message: nothing");
	}
	{
		Verdict v = selectDeleted(newest({ 10, 10 }), Stored{ { 5 }, false });
		CHECK(!v.concluded, "one distinct id twice: nothing");
	}

	/* Short newest page while older messages are stored: TDLib may be
	 * loading; mark nothing. */
	{
		Page p = newest({ 190, 200 });
		Stored s;
		s.between = range(191, 199);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(!v.concluded && v.deleted.empty(),
		      "short newest page above stored older messages: nothing");
	}
	/* The same short page at the chat's start (nothing stored older). */
	{
		Page p = newest({ 1, 3, 4 });
		Stored s;
		s.between = { 2 };
		s.older_exists = false;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && (v.deleted == std::vector<int64_t>{ 2 }),
		      "short newest page at the chat's start: concluded");
	}
	/* total counts local messages too: 50 returned, 49 with server ids. */
	{
		Page p = newest(without(range(150, 200), { 160, 199 }), 50);
		Stored s;
		s.between = range(151, 199);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && (v.deleted == std::vector<int64_t>{ 160, 199 }),
		      "total, not ids, decides a short newest page");
	}

	/* Around 500: up to 10 newer (501..510), 50 at or older (451..500). */
	{
		Page p = older(500, range(451, 510));
		Stored s;
		s.between = range(452, 509);
		s.between.push_back(490);	/* duplicates come out once */
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && v.deleted.empty(), "full older page, none gone");
	}
	{
		/* 50 at or older (449..500 but 480, 470), 10 newer but 505. */
		Page p = older(500, without(range(449, 510), { 470, 480, 505 }));
		Stored s;
		s.between = range(450, 509);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && (v.deleted == std::vector<int64_t>{ 470, 480, 505 }),
		      "full older page: the missing ones on both sides of N");
	}
	/* Few newer near the newest end is normal, not short. */
	{
		Page p = older(500, without(range(451, 502), { 460 }));
		Stored s;
		s.between = range(452, 501);
		s.older_exists = true;
		/* 451..500 but 460 = 49 older: give one more to make 50. */
		p.ids.push_back(450);
		p.total++;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && (v.deleted == std::vector<int64_t>{ 460 }),
		      "two newer only (newest end): still concluded");
	}
	/* Older side short while older messages are stored. */
	{
		Page p = older(500, range(460, 510));
		Stored s;
		s.between = range(461, 509);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(!v.concluded && v.deleted.empty(),
		      "41 older of 50 with older stored: nothing");
	}
	/* Newer ones do not make up for missing older ones. */
	{
		Page p = older(500, range(452, 515));
		Stored s;
		s.between = range(453, 514);
		s.older_exists = true;
		Verdict v = selectDeleted(p, s);
		CHECK(!v.concluded, "49 older and 15 newer: still short");
	}
	/* Older side short at the chat's start. */
	{
		Page p = older(30, without(range(1, 40), { 4 }));
		Stored s;
		s.between = range(2, 39);
		s.older_exists = false;
		Verdict v = selectDeleted(p, s);
		CHECK(v.concluded && (v.deleted == std::vector<int64_t>{ 4 }),
		      "short older side at the chat's start: concluded");
	}
}

static void testShape(void)
{
	printf("stretch shape\n");
	Shape n = shapeOf(0);
	CHECK(n.from == 0 && n.offset == 0 && n.limit == 50,
	      "newest: from 0, offset 0, limit 50");
	Shape o = shapeOf(1149528);
	CHECK(o.from == 1205367472128LL && o.offset == -10 && o.limit == 60,
	      "around N: from N << 20, offset -10, limit 60");
}

static void testCatchUp(void)
{
	printf("restart catch-up\n");
	const int64_t now = 1800000000;
	std::unordered_map<int64_t, int64_t> last = {
		{ -1, now - 60 },
		{ -2, now - 47 * 3600 },
		{ -3, now - 49 * 3600 },
		{ 4, now - 5 },
		{ -5, 0 },
		{ -6, now - 48 * 3600 },
		{ -7, now - 60 },
	};
	auto got = catchUpChats(last, now);
	CHECK((got == std::vector<int64_t>{ 4, -7, -1, -2, -6 }),
	      "within 48 h, newest first, ties by chat id, no date skipped");
	CHECK(catchUpChats({}, now).empty(), "no chats: none");
}

static void testRepairYields(void)
{
	printf("the repair waits for rechecks\n");
	const auto now = 1s;
	CHECK(repairMayRun(0, 0, now), "nothing waiting or in flight: it runs");
	CHECK(!repairMayRun(1, 0, now), "a recheck waiting: it waits");
	CHECK(!repairMayRun(0, 1, now), "a recheck in flight: it waits");
	CHECK(!repairMayRun(200, 2, 59s), "a catch-up under way: it waits");
	CHECK(repairMayRun(200, 2, 60s),
	      "but after a minute without sending it gets a tick");

	printf("the repair's share of the budget\n");
	CHECK(repairAllowance(10, 0, 0) == 5, "at most 5 a second of 10");
	CHECK(repairAllowance(10, 3, 2) == 3, "5 minus what it sent");
	CHECK(repairAllowance(10, 8, 0) == 2, "no more than the budget leaves");
	CHECK(repairAllowance(10, 10, 0) == 0, "nothing once the tick is full");
	CHECK(repairAllowance(10, 5, 5) == 0, "nothing once it sent 5");
}

static void testQueuedAt(void)
{
	printf("a queued recheck knows when it was queued\n");
	Queue q;
	const auto t0 = Clock::time_point{} + std::chrono::hours(1);
	q.push({ -100, 0, false }, t0);
	auto it = q.pop(t0 + std::chrono::seconds(5));
	CHECK(it && it->queued_at == t0, "queued_at is the push time");
}

static void testGapChats(void)
{
	printf("the catch-up rechecks only chats with a gap\n");
	const std::vector<int64_t> chats{ -1, -2, -3, -4, 5 };
	const std::unordered_map<int64_t, int64_t> tdlib{
		{ -1, 100 }, { -2, 100 }, { -3, 100 }, { 5, 7 } };
	const std::unordered_map<int64_t, int64_t> stored{
		{ -1, 100 }, { -2, 99 }, { 5, 9 } };
	auto g = gapChats(chats, tdlib, stored);
	CHECK(g == std::vector<int64_t>({ -2, -3 }),
	      "an older newest stored, or none stored: a gap");
	CHECK(gapChats({ -1 }, tdlib, stored).empty(),
	      "the newest already stored: no recheck");
	CHECK(gapChats({ -4 }, tdlib, stored).empty(),
	      "TDLib named no last message: nothing to compare");

	printf("a gap below the newest stored message\n");
	const int64_t sg = -1001234567890, sg2 = -1009876543210;
	const std::unordered_map<int64_t, int64_t> tl{ { sg, 500 }, { sg2, 500 },
		{ -5, 90 } };
	const std::unordered_map<int64_t, int64_t> sm{ { sg, 500 }, { sg2, 500 },
		{ -5, 90 } };
	const std::unordered_map<int64_t, int64_t> near{ { sg, 3 }, { sg2, 48 },
		{ -5, 1 } };
	CHECK(gapChats({ sg, sg2, -5 }, tl, sm, near) ==
		      std::vector<int64_t>({ sg }),
	      "a sparse supergroup is rechecked, a dense one and a basic group"
	      " are not");
	CHECK(sparseNearNewest(sg, 500, 39) && !sparseNearNewest(sg, 500, 40),
	      "dense from 40 of the last 50");
	CHECK(sparseNearNewest(sg, 11, 7) && !sparseNearNewest(sg, 11, 9),
	      "a small chat: the share of its own ids");
	CHECK(!sparseNearNewest(55, 500, 0), "a private chat: not applicable");
	CHECK(isSupergroupId(-1001483770714) && !isSupergroupId(-4123456) &&
		      !isSupergroupId(77),
	      "supergroup and channel ids are -100...");
}

static void testSlotForAsked(void)
{
	printf("a slot stays free for rechecks asked for\n");
	Queue q;
	const auto t0 = Clock::time_point{} + std::chrono::hours(1);
	q.push({ -9, 0, true }, t0);
	CHECK(!q.pop(t0, false), "no background recheck while one is in flight");
	auto it = q.pop(t0, true);
	CHECK(it && it->chat_id == -9, "but one when none is");
	q.push({ -8, 0, true }, t0);
	q.push({ -7, 0, false }, t0);
	it = q.pop(t0, false);
	CHECK(it && it->chat_id == -7, "an asked one goes regardless");
}

static void testPaging(void)
{
	printf("a short answer reads another page\n");
	CHECK(askAgain(1, true, 1), "short and something new: page 2");
	CHECK(askAgain(2, true, 5), "and page 3");
	CHECK(askAgain(3, true, 5), "and page 4");
	CHECK(!askAgain(4, true, 5), "never more than four pages");
	CHECK(askAgain(2, false, 50),
	      "a full page of new messages reads on: the hole goes on");
	CHECK(askAgain(1, false, 3), "a full first page with new ones too");
	CHECK(askAgain(1, true, 0),
	      "page 1: 1 returned, 0 new, short: read on (the newest is stored,"
	      " the ones before it are missing)");
	CHECK(!askAgain(2, true, 0), "from page 2, nothing new: stop");
	CHECK(!askAgain(1, false, 0), "a full first page, nothing new: done");
	CHECK(!askAgain(2, false, 0), "a full later page, nothing new: done");

	Page p;
	p.around = 0;
	p.ids = { 100 };
	p.total = 1;
	CHECK(isShort(p, true), "one message above stored older ones is short");
	CHECK(!isShort(p, false), "but not at the chat's start");
	p.ids.clear();
	for (int64_t id = 51; id <= 100; id++)
		p.ids.push_back(id);
	p.total = 50;
	CHECK(!isShort(p, true), "a full newest page is not short");

	Page c;
	c.cont = true;
	c.around = 900;	/* the recheck's, not this page's */
	c.ids = { 10, 11, 12 };
	c.total = 3;
	CHECK(isShort(c, true), "a later page judges all of it as older");
	for (int64_t id = 13; id < 60; id++)
		c.ids.push_back(id);
	c.total = c.ids.size();
	CHECK(!isShort(c, true), "and 50 of them are a full page");
}

int main(void)
{
	testPaging();
	testGapChats();
	testSlotForAsked();
	testRepairYields();
	testQueuedAt();
	testNewestLimit();
	testOlderLimit();
	testBound();
	testOrder();
	testReasons();
	testShape();
	testSelection();
	testCatchUp();
	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

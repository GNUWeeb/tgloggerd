// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * discord_bots_test -- discordd's Discord bot runner over gwdiscord's mock
 * transport: no network, no database, no TDLib.
 *
 * It checks what the bots table relies on: one gateway per enabled bot,
 * started, stopped and restarted as the rows change; each bot's state
 * reported (connecting, connected with its identity, refused with the
 * reason); a bot Discord refuses disabled and never retried in a loop, even
 * when its report cannot be stored at first; enabling it again is one more
 * try. The tokens are made up.
 */
#include "../DiscordBots.hpp"

#include <gwdiscord/MockTransport.hpp>

#include <chrono>
#include <cstdio>
#include <functional>
#include <map>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

using namespace discordd;
using Clock = DiscordBots::Clock;

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

static const char *HELLO = R"({"op":10,"d":{"heartbeat_interval":45000}})";

static std::string ready(uint64_t user_id, const std::string &name)
{
	return R"({"op":0,"s":1,"t":"READY","d":{"session_id":"s",)"
	       R"("resume_gateway_url":"wss://resume.invalid",)"
	       R"("user":{"id":")" + std::to_string(user_id) +
	       R"(","username":")" + name + R"(","bot":true}}})";
}

static const char *MSG = R"({"op":0,"s":2,"t":"MESSAGE_CREATE","d":{
	"id":"1541191219416399922","channel_id":"1541156106343161896",
	"guild_id":"845302963739033611","content":"hi",
	"author":{"id":"42","username":"someone"}}})";

/* Made-up tokens, one per behaviour of the fake Discord. */
static const std::string GOOD_A = "MTAwMDAwMDAwMDAwMDAwMDAx.GaAAAA.good-a";
static const std::string GOOD_B = "MTAwMDAwMDAwMDAwMDAwMDAy.GbBBBB.good-b";
static const std::string BAD = "MTAwMDAwMDAwMDAwMDAwMDAz.GcCCCC.bad";
static const std::string NO_INTENT = "MTAwMDAwMDAwMDAwMDAwMDA0.GdDDDD.nointent";

/* The fake Discord: a script and a record per token. */
struct FakeDiscord {
	std::map<std::string, std::shared_ptr<gwdiscord::MockScript>> scripts;
	std::map<std::string, std::shared_ptr<gwdiscord::MockRecord>> records;

	void good(const std::string &token, uint64_t user_id,
		  const std::string &name)
	{
		auto s = std::make_shared<gwdiscord::MockScript>();
		s->frames = {HELLO, ready(user_id, name), MSG};
		scripts[token] = s;
		records[token] = std::make_shared<gwdiscord::MockRecord>();
	}

	void closes(const std::string &token, uint16_t code,
		    const std::string &reason)
	{
		auto s = std::make_shared<gwdiscord::MockScript>();
		s->frames = {HELLO};
		s->close_when_done = true;
		s->close_code = code;
		s->close_reason = reason;
		scripts[token] = s;
		records[token] = std::make_shared<gwdiscord::MockRecord>();
	}

	int connects(const std::string &token)
	{
		auto &r = records[token];
		std::lock_guard<std::mutex> lk(r->mtx);
		return r->connect_attempts;
	}

	DiscordBots::Transports transports(void)
	{
		return [this](const DiscordBotRow &row) {
			return gwdiscord::mock_transport(scripts.at(row.token),
							 records.at(row.token));
		};
	}
};

/* The bots table as far as the reports go. */
struct FakeStore {
	std::mutex		mtx;
	std::vector<BotReport>	reports;
	bool			fail = false;

	DiscordBots::Reporter reporter(void)
	{
		return [this](const BotReport &r) {
			std::lock_guard<std::mutex> lk(mtx);
			if (fail)
				return false;
			reports.push_back(r);
			return true;
		};
	}

	std::vector<BotReport> of(uint64_t id)
	{
		std::lock_guard<std::mutex> lk(mtx);
		std::vector<BotReport> out;
		for (const auto &r : reports) {
			if (r.id == id)
				out.push_back(r);
		}
		return out;
	}

	bool has(uint64_t id, BotStatus st)
	{
		for (const auto &r : of(id)) {
			if (r.status == st)
				return true;
		}
		return false;
	}

	void set_fail(bool f)
	{
		std::lock_guard<std::mutex> lk(mtx);
		fail = f;
	}
};

static bool wait_for(const std::function<bool(void)> &cond)
{
	const auto end = Clock::now() + std::chrono::seconds(10);
	while (Clock::now() < end) {
		if (cond())
			return true;
		std::this_thread::sleep_for(std::chrono::milliseconds(10));
	}
	return cond();
}

static gwdiscord::GatewayConfig base_cfg(void)
{
	gwdiscord::GatewayConfig c;
	c.backoff_min = 1;
	c.backoff_max = 1;
	return c;
}

static DiscordBots::Log quiet(void)
{
	return [](int, const std::string &) {};
}

static void test_runs_each_enabled_bot(void)
{
	printf("test: one gateway per enabled bot, with its state and events\n");
	FakeDiscord d;
	d.good(GOOD_A, 9001, "bot_a");
	d.good(GOOD_B, 9002, "bot_b");
	FakeStore st;

	std::mutex mm;
	std::map<uint64_t, int> msgs;
	DiscordBots::Handlers h;
	h.message = [&](uint64_t bot, const gwdiscord::Message &m, bool) {
		std::lock_guard<std::mutex> lk(mm);
		if (m.content == "hi")
			msgs[bot]++;
	};

	DiscordBots bots(base_cfg(), d.transports(), st.reporter(), h, quiet());
	bots.sync({{1000000000, GOOD_A}, {1000000001, GOOD_B}}, Clock::now());

	CHECK(wait_for([&] { return st.has(1000000000, BotStatus::Connected) &&
				    st.has(1000000001, BotStatus::Connected); }),
	      "both bots report connected");
	auto a = st.of(1000000000);
	CHECK(!a.empty() && a[0].status == BotStatus::Connecting,
	      "a bot is reported connecting first");
	bool ident = false;
	for (const auto &r : a) {
		if (r.status == BotStatus::Connected)
			ident = r.has_identity && r.user_id == 9001 &&
				r.username == "bot_a" && r.token == GOOD_A;
	}
	CHECK(ident, "connected carries the bot's Discord id and username");
	CHECK(wait_for([&] {
		std::lock_guard<std::mutex> lk(mm);
		return msgs[1000000000] == 1 && msgs[1000000001] == 1;
	}), "each bot's messages arrive tagged with its id");
	CHECK(bots.running().size() == 2, "two gateways run");
	auto u = bots.user_ids();
	CHECK(u.size() == 2, "both bots' Discord user ids are known");

	/* A sync with the same rows changes nothing. */
	bots.sync({{1000000000, GOOD_A}, {1000000001, GOOD_B}}, Clock::now());
	CHECK(d.connects(GOOD_A) == 1 && d.connects(GOOD_B) == 1,
	      "an unchanged bot is not reconnected");
}

static void test_reload_follows_rows(void)
{
	printf("test: disabling, removing and replacing a token\n");
	FakeDiscord d;
	d.good(GOOD_A, 9001, "bot_a");
	d.good(GOOD_B, 9002, "bot_b");
	FakeStore st;
	DiscordBots bots(base_cfg(), d.transports(), st.reporter(), {}, quiet());

	bots.sync({{1000000000, GOOD_A}, {1000000001, GOOD_B}}, Clock::now());
	CHECK(wait_for([&] { return st.has(1000000001, BotStatus::Connected); }),
	      "bot b connected");

	/* b disabled (no longer among the enabled rows). */
	bots.sync({{1000000000, GOOD_A}}, Clock::now());
	auto run = bots.running();
	CHECK(run.size() == 1 && run[0] == 1000000000,
	      "a disabled bot's gateway is stopped");

	/* a's row given b's token: the old connection goes, a new one comes. */
	bots.sync({{1000000000, GOOD_B}}, Clock::now());
	CHECK(wait_for([&] { return d.connects(GOOD_B) == 2; }),
	      "a replaced token connects with the new token");
	CHECK(d.connects(GOOD_A) == 1, "the old token is not used again");
	bool reported_new = false;
	CHECK(wait_for([&] {
		for (const auto &r : st.of(1000000000))
			reported_new = reported_new ||
				       (r.token == GOOD_B &&
					r.status == BotStatus::Connected);
		return reported_new;
	}), "the report names the new token, so only that row matches");

	bots.sync({}, Clock::now());
	CHECK(bots.running().empty(), "every bot stops when none is enabled");
}

static void test_refusals_disable(void)
{
	printf("test: refused bots are disabled with the reason, not retried\n");
	FakeDiscord d;
	d.closes(BAD, 4004, "Authentication failed.");
	d.closes(NO_INTENT, 4014, "Disallowed intent(s).");
	FakeStore st;
	DiscordBots bots(base_cfg(), d.transports(), st.reporter(), {}, quiet());

	const auto t0 = Clock::now();
	bots.sync({{1000000005, BAD}, {1000000006, NO_INTENT}}, t0);
	CHECK(wait_for([&] {
		return st.has(1000000005, BotStatus::TokenRefused) &&
		       st.has(1000000006, BotStatus::IntentMissing);
	}), "4004 reports token_refused, 4014 intent_missing");
	for (const auto &r : st.of(1000000005)) {
		if (r.status == BotStatus::TokenRefused) {
			CHECK(r.disable, "a refused token disables the bot");
			CHECK(r.detail.find("4004") != std::string::npos,
			      "the detail names the close code");
			CHECK(r.detail.find(BAD) == std::string::npos,
			      "the detail does not carry the token");
		}
	}
	for (const auto &r : st.of(1000000006)) {
		if (r.status == BotStatus::IntentMissing)
			CHECK(r.disable &&
				      r.detail.find("Message Content") !=
					      std::string::npos,
			      "a missing intent disables it and says which");
	}
	CHECK(wait_for([&] { return bots.running().empty(); }),
	      "neither keeps a gateway");

	/*
	 * The rows of a load that started before the refusal still say
	 * enabled: they must not start the bots again.
	 */
	for (int i = 0; i < 5; i++)
		bots.sync({{1000000005, BAD}, {1000000006, NO_INTENT}}, t0);
	CHECK(d.connects(BAD) == 1 && d.connects(NO_INTENT) == 1,
	      "stale rows do not retry a refused bot");

	/* A load after the report sees the bots disabled. */
	bots.sync({}, Clock::now());
	CHECK(d.connects(BAD) == 1, "a disabled refused bot stays stopped");

	/* Enabled again by its owner: one more try, and only one. */
	bots.sync({{1000000005, BAD}}, Clock::now());
	CHECK(wait_for([&] { return d.connects(BAD) == 2; }),
	      "enabling it again tries once more");
	CHECK(wait_for([&] {
		int n = 0;
		for (const auto &r : st.of(1000000005))
			n += r.status == BotStatus::TokenRefused;
		return n == 2;
	}), "and it is refused and disabled again");
	std::this_thread::sleep_for(std::chrono::milliseconds(200));
	CHECK(d.connects(BAD) == 2, "without a retry loop");
}

static void test_unstored_report_blocks_restart(void)
{
	printf("test: a refusal that cannot be stored keeps the bot stopped\n");
	FakeDiscord d;
	d.closes(BAD, 4004, "Authentication failed.");
	FakeStore st;
	DiscordBots bots(base_cfg(), d.transports(), st.reporter(), {}, quiet());

	st.set_fail(true);
	bots.sync({{1000000007, BAD}}, Clock::now());
	CHECK(wait_for([&] { return bots.running().empty(); }),
	      "the refused gateway stops");
	std::this_thread::sleep_for(std::chrono::milliseconds(50));

	/* The database is down: the row still says enabled. */
	for (int i = 0; i < 3; i++)
		bots.sync({{1000000007, BAD}}, Clock::now());
	CHECK(d.connects(BAD) == 1, "it is not started while unreported");
	CHECK(st.of(1000000007).empty(), "nothing was stored");

	st.set_fail(false);
	bots.sync({{1000000007, BAD}}, Clock::now());
	CHECK(st.has(1000000007, BotStatus::TokenRefused),
	      "the next sync stores the refusal");
	CHECK(d.connects(BAD) == 1, "and still does not start it");
	bots.sync({}, Clock::now());
	CHECK(d.connects(BAD) == 1, "a load after it sees the bot disabled");
}

static void test_legacy_transition(void)
{
	printf("test: DISCORD_BOT_TOKEN runs until a bot is stored\n");
	const std::string LEGACY = GOOD_A, IMPORTED = GOOD_B;
	FakeDiscord d;
	d.good(LEGACY, 9001, "legacy");
	d.good(IMPORTED, 9002, "imported");
	FakeStore st;
	std::mutex mm;
	std::vector<uint64_t> from;
	DiscordBots::Handlers h;
	h.message = [&](uint64_t bot, const gwdiscord::Message &, bool) {
		std::lock_guard<std::mutex> lk(mm);
		from.push_back(bot);
	};
	DiscordBots bots(base_cfg(), d.transports(), st.reporter(), h, quiet());

	/* The table missing and the table empty load the same: no rows. */
	auto run = bots_to_run({}, LEGACY, false);
	CHECK(run.size() == 1 && run[0].id == LEGACY_BOT_ID &&
		      run[0].token == LEGACY,
	      "no stored bot: the environment's token runs as the legacy bot");
	CHECK(bots_to_run({}, "", false).empty(),
	      "no stored bot and no token: nothing runs");
	bots.sync(run, Clock::now());
	CHECK(wait_for([&] {
		std::lock_guard<std::mutex> lk(mm);
		return from.size() == 1 && from[0] == LEGACY_BOT_ID;
	}), "its messages are tagged with the legacy id (unbound routes)");
	CHECK(wait_for([&] { return st.has(LEGACY_BOT_ID, BotStatus::Connected); }),
	      "the legacy bot reports under its id, which no row has");

	/* After the import: the stored bot runs, the token is ignored. */
	run = bots_to_run({{1000000000, IMPORTED}}, LEGACY, false);
	CHECK(run.size() == 1 && run[0].id == 1000000000,
	      "a stored bot replaces the legacy one");
	bots.sync(run, Clock::now());
	CHECK(wait_for([&] { return st.has(1000000000, BotStatus::Connected); }),
	      "the imported bot connects");
	auto r = bots.running();
	CHECK(r.size() == 1 && r[0] == 1000000000, "the legacy gateway is stopped");
	CHECK(d.connects(LEGACY) == 1, "and not started again");

	/* A refused legacy token is not retried either. */
	FakeDiscord bad;
	bad.closes(BAD, 4004, "Authentication failed.");
	FakeStore st2;
	bool refused = false;
	DiscordBots::Reporter rep = [&](const BotReport &rp) {
		if (rp.id == LEGACY_BOT_ID && rp.disable)
			refused = true;
		return st2.reporter()(rp);
	};
	DiscordBots b2(base_cfg(), bad.transports(), rep, {}, quiet());
	b2.sync(bots_to_run({}, BAD, false), Clock::now());
	CHECK(wait_for([&] { return st2.has(LEGACY_BOT_ID, BotStatus::TokenRefused); }),
	      "a refused legacy token is reported");
	for (int i = 0; i < 3; i++)
		b2.sync(bots_to_run({}, BAD, refused), Clock::now());
	CHECK(refused && bots_to_run({}, BAD, refused).empty() &&
		      bad.connects(BAD) == 1,
	      "and is not started again");
}

int main(void)
{
	test_runs_each_enabled_bot();
	test_reload_follows_rows();
	test_refusals_disable();
	test_unstored_report_blocks_restart();
	test_legacy_transition();

	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}

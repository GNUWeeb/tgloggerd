// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * recheck_api_test -- POST /v1/chats/{chat_id}/recheck over a real
 * InternalApi on loopback, with stand-ins for TDLib, the database and a
 * real recheck::Queue on a clock of the test's own: no TDLib client, no
 * database.
 */
#include <tgloggerd/InternalApi.hpp>
#include <tgloggerd/RecheckApi.hpp>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstdio>
#include <ctime>
#include <mutex>
#include <random>
#include <set>
#include <string>
#include <vector>

using namespace tgloggerd;
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

/* 32 bytes of 0x01, base64. */
static const char kKey[] = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=";

struct Reply {
	int		status = 0;
	nlohmann::json	body;
};

/* One HTTP/1.0 POST, with a body unless body is null; the server closes
 * the connection after it. */
static Reply post(int port, const std::string &path, const std::string &auth,
		  const char *body)
{
	Reply r;
	int fd = socket(AF_INET, SOCK_STREAM, 0);
	if (fd < 0)
		return r;

	sockaddr_in a{};
	a.sin_family = AF_INET;
	a.sin_port = htons(static_cast<uint16_t>(port));
	a.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
	if (connect(fd, reinterpret_cast<sockaddr *>(&a), sizeof(a)) < 0) {
		close(fd);
		return r;
	}

	const std::string b = body ? body : "";
	std::string req = "POST " + path + " HTTP/1.0\r\nHost: 127.0.0.1\r\n"
		"Authorization: Bearer " + auth + "\r\n";
	if (body)
		req += "Content-Type: application/json\r\n";
	req += "Content-Length: " + std::to_string(b.size()) + "\r\n\r\n" + b;
	if (write(fd, req.data(), req.size()) != (ssize_t)req.size()) {
		close(fd);
		return r;
	}

	std::string all;
	char buf[4096];
	ssize_t n;
	while ((n = read(fd, buf, sizeof(buf))) > 0)
		all.append(buf, static_cast<size_t>(n));
	close(fd);

	if (sscanf(all.c_str(), "HTTP/1.%*d %d", &r.status) != 1)
		r.status = 0;
	size_t p = all.find("\r\n\r\n");
	if (p != std::string::npos)
		r.body = nlohmann::json::parse(all.substr(p + 4), nullptr, false);
	return r;
}

/* The stand-ins. */
struct Fake {
	std::mutex			mtx;
	bool				ready = true;
	bool				tdlib_ready = true;
	std::set<int64_t>		tdlib_chats;
	std::set<int64_t>		db_chats;
	recheck::Queue			queue{ 3 };
	recheck::Clock::time_point	now = recheck::Clock::now();
	std::vector<std::pair<int64_t, int64_t>> asked;
};

static bool queued(const Reply &r)
{
	return r.status == 202 && r.body.is_object() &&
	       r.body.value("queued", false) == true && !r.body.contains("reason");
}

static bool refused(const Reply &r, const char *reason)
{
	return r.status == 200 && r.body.is_object() &&
	       r.body.contains("queued") && r.body["queued"] == false &&
	       r.body.value("reason", "") == reason;
}

int main(void)
{
	Fake f;
	RecheckBackend be;
	be.ready = [&] {
		std::lock_guard<std::mutex> lk(f.mtx);
		return f.ready;
	};
	be.tdlib_knows = [&](int64_t chat_id) -> std::optional<bool> {
		std::lock_guard<std::mutex> lk(f.mtx);
		if (!f.tdlib_ready)
			return std::nullopt;
		return f.tdlib_chats.count(chat_id) > 0;
	};
	be.db_knows = [&](int64_t chat_id) {
		std::lock_guard<std::mutex> lk(f.mtx);
		return f.db_chats.count(chat_id) > 0;
	};
	be.enqueue = [&](int64_t chat_id, int64_t around) {
		std::lock_guard<std::mutex> lk(f.mtx);
		f.asked.emplace_back(chat_id, around);
		return f.queue.push({ chat_id, around, false }, f.now);
	};
	f.tdlib_chats = { -100, -200, 77 };
	f.db_chats = { -300 };

	InternalApi api(nullptr);
	registerRecheckApi(api, be);

	std::mt19937 rng(static_cast<unsigned>(std::time(nullptr)) ^
			 static_cast<unsigned>(getpid()));
	int port = 0;
	for (int i = 0; i < 20 && !port; i++) {
		int p = 20000 + static_cast<int>(rng() % 40000);
		InternalApi::Config cfg;
		cfg.listen = "127.0.0.1:" + std::to_string(p);
		cfg.threads = 2;
		cfg.web_app_key = kKey;
		if (api.start(cfg) == 0)
			port = p;
	}
	if (!port) {
		printf("FAIL: could not bind a loopback port\n");
		return 1;
	}
	const std::string bearer = InternalApi::deriveBearer(kKey);
	auto recheck = [&](const std::string &chat, const char *body) {
		return post(port, "/v1/chats/" + chat + "/recheck", bearer, body);
	};

	printf("queued and the reasons\n");
	Reply r = recheck("-100", R"({"around": 0})");
	CHECK(queued(r), "202 {queued: true} for the newest");
	CHECK(!f.asked.empty() && f.asked.back() == std::make_pair((int64_t)-100, (int64_t)0),
	      "asked for (chat, 0)");
	r = recheck("-100", nullptr);
	CHECK(refused(r, "queued"), "no body is the newest: already queued");
	{
		std::lock_guard<std::mutex> lk(f.mtx);
		f.queue.pop(f.now);	/* it runs */
		f.now += 10s;
	}
	r = recheck("-100", "{}");
	CHECK(refused(r, "recent"), "{} 10 s after it ran: recent");
	{
		std::lock_guard<std::mutex> lk(f.mtx);
		f.now += 60s;
	}
	r = recheck("-100", R"({"around": 0})");
	CHECK(queued(r), "a minute later: 202 again");

	r = recheck("-200", R"({"around": 1234})");
	CHECK(queued(r), "an older stretch: 202");
	CHECK(f.asked.back() == std::make_pair((int64_t)-200, (int64_t)1234),
	      "asked for (chat, around)");
	r = recheck("-200", R"({"around": 1234})");
	CHECK(refused(r, "queued"), "the same older stretch again: queued");
	{
		std::lock_guard<std::mutex> lk(f.mtx);
		f.queue.pop(f.now);
		f.queue.pop(f.now);
		f.now += 2h;
	}
	r = recheck("-200", R"({"around": 1234})");
	CHECK(refused(r, "recent"), "2 h after it ran: recent");

	r = recheck("-300", R"({"around": 5})");
	CHECK(queued(r), "a chat only the DB knows: 202");
	r = recheck("77", R"({"around": 1})");
	CHECK(queued(r), "a private chat: 202");
	r = recheck("-100", R"({"around": 2})");
	CHECK(queued(r), "third in the queue of 3: 202");
	r = recheck("-100", R"({"around": 3})");
	CHECK(refused(r, "busy"), "the queue is full: busy");

	printf("400\n");
	const size_t asked = f.asked.size();
	for (const char *bad : { R"({"around": -1})", R"({"around": 1.5})",
				 R"({"around": "5"})", R"({"around": null})",
				 R"({"around": 9223372036854775807})",
				 R"({"around": 18446744073709551615})",
				 "[]", "5" }) {
		r = recheck("-100", bad);
		CHECK(r.status == 400 && r.body.value("error", "") == "bad_request",
		      (std::string("400 for ") + bad).c_str());
	}
	for (const char *chat : { "abc", "0", "12x", "-" }) {
		r = recheck(chat, "{}");
		CHECK(r.status == 400, (std::string("400 for chat ") + chat).c_str());
	}
	CHECK(f.asked.size() == asked, "nothing queued for a bad request");

	printf("404\n");
	r = recheck("-999", "{}");
	CHECK(r.status == 404 && r.body.value("error", "") == "not_found",
	      "404 for a chat neither TDLib nor the DB knows");

	printf("503\n");
	{
		std::lock_guard<std::mutex> lk(f.mtx);
		f.ready = false;
	}
	r = recheck("-100", "{}");
	CHECK(r.status == 503 && r.body.value("error", "") == "unavailable",
	      "503 while TDLib is not authorized");
	{
		std::lock_guard<std::mutex> lk(f.mtx);
		f.ready = true;
		f.tdlib_ready = false;
	}
	r = recheck("-999", "{}");
	CHECK(r.status == 503, "503 when TDLib cannot be asked, before any 404");
	CHECK(f.asked.size() == asked, "nothing queued while not ready");

	api.stop();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

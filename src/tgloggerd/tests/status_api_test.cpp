// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * status_api_test -- GET /v1/status over a real InternalApi on loopback,
 * with a fake TDLib::status(): no TDLib client, no database.
 */
#include <tgloggerd/InternalApi.hpp>
#include <tgloggerd/StatusApi.hpp>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstdio>
#include <cstdlib>
#include <ctime>
#include <mutex>
#include <random>
#include <string>

using namespace tgloggerd;

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
	std::string	body;
};

/* One HTTP/1.0 request; the server closes the connection after it. */
static Reply get(int port, const std::string &path, const std::string &auth)
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

	std::string req = "GET " + path + " HTTP/1.0\r\nHost: 127.0.0.1\r\n";
	if (!auth.empty())
		req += "Authorization: Bearer " + auth + "\r\n";
	req += "\r\n";
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
		r.body = all.substr(p + 4);
	return r;
}

int main(void)
{
	std::mutex mtx;
	TDLib::Status cur{ TDLib::State::Waiting, 1700000000 };

	InternalApi api(nullptr);
	registerStatusApi(api, [&] {
		std::lock_guard<std::mutex> lk(mtx);
		return cur;
	});

	/* A random high port; retry a few on a clash with another test. */
	std::mt19937 rng(static_cast<unsigned>(std::time(nullptr)) ^
			 static_cast<unsigned>(getpid()));
	int port = 0;
	for (int i = 0; i < 20 && !port; i++) {
		int p = 20000 + static_cast<int>(rng() % 40000);
		InternalApi::Config cfg;
		cfg.listen = "127.0.0.1:" + std::to_string(p);
		cfg.threads = 1;
		cfg.web_app_key = kKey;
		if (api.start(cfg) == 0)
			port = p;
	}
	if (!port) {
		printf("FAIL: could not bind a loopback port\n");
		return 1;
	}

	std::string bearer = InternalApi::deriveBearer(kKey);

	printf("GET /v1/status without the bearer\n");
	Reply r = get(port, "/v1/status", "");
	CHECK(r.status == 401, "answers 401");

	printf("GET /v1/status with a wrong bearer\n");
	r = get(port, "/v1/status", std::string(64, '0'));
	CHECK(r.status == 401, "answers 401");

	struct Case {
		TDLib::State	state;
		int64_t		since;
		const char	*name;
	} cases[] = {
		{ TDLib::State::Waiting, 1700000000, "waiting" },
		{ TDLib::State::Ready, 1700000123, "ready" },
		{ TDLib::State::Closing, 1700000456, "closing" },
	};
	for (const Case &c : cases) {
		{
			std::lock_guard<std::mutex> lk(mtx);
			cur = { c.state, c.since };
		}
		printf("GET /v1/status while %s\n", c.name);
		r = get(port, "/v1/status", bearer);
		CHECK(r.status == 200, "answers 200");
		nlohmann::json j = nlohmann::json::parse(r.body, nullptr, false);
		CHECK(j.is_object() && j.size() == 2, "an object of two fields");
		CHECK(j.is_object() && j.value("tdlib", "") == c.name,
		      "tdlib names the state");
		CHECK(j.is_object() && j.contains("since") &&
		      j["since"].is_number_integer() &&
		      j["since"].get<int64_t>() == c.since,
		      "since is the change time");
	}

	api.stop();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

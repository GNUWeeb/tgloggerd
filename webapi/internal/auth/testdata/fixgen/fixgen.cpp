// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
// Makes the Go auth tests' fixtures with the Drogon web's own auth code,
// and checks values the Go code made. Build and run it with run.sh in
// this directory:
//
//   fixgen gen    > ../drogon.json
//   fixgen verify < go-minted.json
#include "auth/Csrf.hpp"
#include "auth/FileToken.hpp"
#include "auth/Password.hpp"
#include "auth/Session.hpp"
#include "auth/Token.hpp"
#include "daemon/Api.hpp"

#include <drogon/HttpRequest.h>
#include <drogon/HttpResponse.h>
#include <drogon/utils/coroutine.h>
#include <nlohmann/json.hpp>
#include <trantor/net/EventLoopThread.h>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sodium.h>
#include <sys/socket.h>
#include <unistd.h>

#include <ctime>
#include <future>
#include <iostream>
#include <thread>

using json = nlohmann::json;
namespace auth = tgweb::auth;

static const char *kKey = "q83vEjRWeJCrze8SNFZ4kKvN7xI0VniQq83vEjRWeJA=";

static std::string hex(const std::string &s)
{
	std::string out(s.size() * 2 + 1, '\0');
	sodium_bin2hex(out.data(), out.size(),
		       reinterpret_cast<const unsigned char *>(s.data()), s.size());
	out.resize(s.size() * 2);
	return out;
}

static std::string unhex(const std::string &h)
{
	std::string out(h.size() / 2, '\0');
	size_t len = 0;
	sodium_hex2bin(reinterpret_cast<unsigned char *>(out.data()), out.size(),
		       h.data(), h.size(), nullptr, &len, nullptr);
	out.resize(len);
	return out;
}

static drogon::HttpRequestPtr reqWithCookie(const char *name,
					    const std::string &v)
{
	auto req = drogon::HttpRequest::newHttpRequest();
	req->addCookie(name, v);
	return req;
}

static json sessionJson(const std::optional<auth::session::Session> &s)
{
	if (!s)
		return nullptr;
	return json{{"uid", std::to_string(s->uid)}, {"username", s->username},
		    {"role", s->role}, {"exp", s->exp}, {"epoch", s->epoch}};
}

static void initKey(const std::string &k)
{
	setenv("WEB_APP_KEY", k.c_str(), 1);
	auth::token::init();
	auth::filetoken::init();
}

/* --- A scripted HTTP server for the daemon client's replies. --- */

struct Canned {
	std::string response; /* full raw HTTP response; "" = never answer */
};

struct Seen {
	std::string head;
	std::string body;
};

static int g_lfd = -1;

static int listenLocal(void)
{
	g_lfd = socket(AF_INET, SOCK_STREAM, 0);
	int one = 1;
	setsockopt(g_lfd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));
	sockaddr_in a{};
	a.sin_family = AF_INET;
	a.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
	a.sin_port = 0;
	bind(g_lfd, (sockaddr *)&a, sizeof(a));
	listen(g_lfd, 8);
	socklen_t l = sizeof(a);
	getsockname(g_lfd, (sockaddr *)&a, &l);
	return ntohs(a.sin_port);
}

static Seen serveOne(const Canned &c)
{
	Seen s;
	int fd = accept(g_lfd, nullptr, nullptr);
	std::string buf;
	char tmp[4096];
	size_t hend;
	while ((hend = buf.find("\r\n\r\n")) == std::string::npos) {
		ssize_t n = read(fd, tmp, sizeof(tmp));
		if (n <= 0)
			break;
		buf.append(tmp, n);
	}
	s.head = buf.substr(0, hend);
	size_t clen = 0;
	{
		std::string lower = s.head;
		for (auto &ch : lower)
			ch = (char)tolower((unsigned char)ch);
		auto p = lower.find("content-length:");
		if (p != std::string::npos)
			clen = strtoul(lower.c_str() + p + 15, nullptr, 10);
	}
	std::string body = buf.substr(hend + 4);
	while (body.size() < clen) {
		ssize_t n = read(fd, tmp, sizeof(tmp));
		if (n <= 0)
			break;
		body.append(tmp, n);
	}
	s.body = body;
	if (c.response.empty()) {
		/* Never answer: hold the connection until the client gives up. */
		std::this_thread::sleep_for(std::chrono::milliseconds(2500));
	} else {
		ssize_t w = write(fd, c.response.data(), c.response.size());
		(void)w;
	}
	close(fd);
	return s;
}

static json headJson(const std::string &head)
{
	json h = json::object();
	size_t eol = head.find("\r\n");
	h["request_line"] = head.substr(0, eol);
	size_t pos = eol + 2;
	while (pos < head.size()) {
		size_t e = head.find("\r\n", pos);
		if (e == std::string::npos)
			e = head.size();
		std::string line = head.substr(pos, e - pos);
		auto colon = line.find(':');
		if (colon != std::string::npos) {
			std::string k = line.substr(0, colon);
			for (auto &ch : k)
				ch = (char)tolower((unsigned char)ch);
			std::string v = line.substr(colon + 1);
			while (!v.empty() && v[0] == ' ')
				v.erase(0, 1);
			if (k == "authorization" || k == "content-type")
				h[k] = v;
		}
		pos = e + 2;
	}
	return h;
}

static json replyJson(const tgweb::daemon::api::Reply &r)
{
	return json{{"sent", r.sent}, {"status", r.status}, {"body", r.body},
		    {"error", r.error}, {"retry_after", r.retryAfter}};
}

static json daemonCases(void)
{
	using namespace tgweb::daemon::api;
	int port = listenLocal();
	setenv("WEB_TGLOGGERD_API",
	       ("http://127.0.0.1:" + std::to_string(port) + "/").c_str(), 1);
	setenv("TG_FETCH_TIMEOUT_SECS", "30", 1);
	if (!init() || !enabled()) {
		std::cerr << "api init failed\n";
		exit(1);
	}

	trantor::EventLoopThread elt;
	elt.run();
	auto *loop = elt.getLoop();

	auto run = [&](std::function<drogon::Task<Reply>()> f,
		       const Canned &c) {
		std::promise<Reply> prom;
		auto fut = prom.get_future();
		loop->queueInLoop([&]() {
			drogon::async_run([&]() -> drogon::Task<> {
				Reply r = co_await f();
				prom.set_value(r);
			});
		});
		Seen s = serveOne(c);
		Reply r = fut.get();
		json j;
		j["request"] = headJson(s.head);
		j["request"]["body"] = s.body;
		j["response"] = c.response;
		j["reply"] = replyJson(r);
		return j;
	};

	auto resp = [](int code, const char *reason, const std::string &extra,
		       const std::string &body) {
		return "HTTP/1.1 " + std::to_string(code) + " " + reason +
		       "\r\nConnection: close\r\n" + extra +
		       "Content-Length: " + std::to_string(body.size()) +
		       "\r\n\r\n" + body;
	};

	json out = json::array();
	json one;

	one = run([] { return call(drogon::Post, "/v1/x", json{{"a", 1}}, 5.0); },
		  {resp(200, "OK", "Content-Type: application/json\r\n",
			"{\"ok\":true}")});
	one["name"] = "ok-json";
	out.push_back(one);

	one = run([] { return call(drogon::Get, "/v1/files/7", nullptr, 5.0); },
		  {resp(404, "Not Found", "Content-Type: application/json\r\n",
			"{\"error\":\"no such file\"}")});
	one["name"] = "404-error";
	out.push_back(one);

	one = run([] { return call(drogon::Post, "/v1/y", json::object(), 5.0); },
		  {resp(503, "Service Unavailable",
			"Retry-After: 7\r\nContent-Type: application/json\r\n",
			"{\"error\":\"not ready\"}")});
	one["name"] = "503-retry-after";
	out.push_back(one);

	one = run([] { return call(drogon::Get, "/v1/z", nullptr, 5.0); },
		  {resp(500, "Internal Server Error", "", "oops")});
	one["name"] = "500-not-json";
	out.push_back(one);

	one = run([] { return call(drogon::Post, "/v1/w", nullptr, 5.0); },
		  {resp(204, "No Content", "", "")});
	one["name"] = "204-empty";
	out.push_back(one);

	one = run([] { return call(drogon::Get, "/v1/v", nullptr, 5.0); },
		  {resp(400, "Bad Request", "", "{\"error\":5}")});
	one["name"] = "400-error-not-string";
	out.push_back(one);

	one = run([] { return recordAccess({1, 2, 18446744073709551615ull}); },
		  {resp(204, "No Content", "", "")});
	one["name"] = "record-access";
	out.push_back(one);

	one = run([] { return recordAccess({}); },
		  {resp(204, "No Content", "", "")});
	one["name"] = "record-access-empty";
	out.push_back(one);

	one = run([] { return fetch(5, 10, 0); },
		  {resp(200, "OK", "", "{\"state\":\"stored\",\"id\":5}")});
	one["name"] = "fetch";
	out.push_back(one);

	one = run([] { return resolveChat("@gnu"); },
		  {resp(200, "OK", "", "{}")});
	one["name"] = "resolve-chat";
	out.push_back(one);

	one = run([] { return joinChat("https://t.me/x"); },
		  {resp(200, "OK", "", "{}")});
	one["name"] = "join-chat";
	out.push_back(one);

	one = run([] { return lookupRealName(-100123); },
		  {resp(200, "OK", "", "{}")});
	one["name"] = "lookup-name";
	out.push_back(one);

	one = run([] { return call(drogon::Get, "/v1/slow", nullptr, 1.0); },
		  {""});
	one["name"] = "timeout";
	out.push_back(one);

	return out;
}

static int gen(void)
{
	json out;
	out["app_key"] = kKey;

	/* Key spellings: the same 32 bytes in four alphabets, plus bad ones. */
	json spell = json::array();
	for (std::string k : {std::string(kKey),
			      std::string("q83vEjRWeJCrze8SNFZ4kKvN7xI0VniQq83vEjRWeJA"),
			      std::string("q83vEjRWeJCrze8SNFZ4kKvN7xI0VniQq83vEjRWeJA="),
			      std::string("_-_-_-_-_-_-_-_-_-_-_-_-"),
			      std::string("_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_A=="),
			      std::string("_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_g="),
			      std::string("_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_-_g"),
			      std::string("AAAAAAAAAAAAAAAAAAAAAA=="),
			      std::string("AAAAAAAAAAAAAAAAAAAAAA"),
			      std::string("AAAAAAAAAAAAAAAAAAAA"),
			      std::string("AAAAAAAAAAAAAAAAAAAAAB=="),
			      std::string("AAAAAAAAAAAAAAAAAAAAAA="),
			      std::string("AAAAAAAAAAAAAAAAAAAAAA==\n"),
			      std::string("AAAAAAAA AAAAAAAAAAAAAA=="),
			      std::string("not base64 at all!"),
			      std::string("")}) {
		setenv("WEB_APP_KEY", k.c_str(), 1);
		bool t = auth::token::init();
		bool f = auth::filetoken::init();
		json e{{"value", k}, {"token_init", t}, {"filetoken_init", f}};
		if (t) {
			e["tag_abc"] = auth::token::tag("abc");
			e["file_token_1"] = auth::filetoken::encrypt(1);
		}
		spell.push_back(e);
	}
	out["key_spellings"] = spell;
	initKey(kKey);

	json toks = json::array();
	for (std::string p : {std::string(""), std::string("a"),
			      std::string("1\n0\n0\nadmin\nroot"),
			      std::string("\x00\xff\xfe binary \n", 13),
			      std::string(200, 'x')}) {
		toks.push_back({{"payload_hex", hex(p)}, {"token", auth::token::make(p)}});
	}
	out["tokens"] = toks;

	json tags = json::array();
	for (std::string m : {std::string(""), std::string("csrf\nabc.def")})
		tags.push_back({{"msg_hex", hex(m)}, {"tag", auth::token::tag(m)}});
	out["tags"] = tags;

	/* open() on malformed and tampered tokens. */
	std::string good = auth::token::make("hello");
	json opens = json::array();
	for (std::string t : {good, good + "x", "x" + good, std::string("nodot"),
			      std::string("."), good.substr(0, good.find('.')),
			      std::string("aGVsbG8.") + good.substr(good.find('.') + 1),
			      good + ".extra"}) {
		auto r = auth::token::open(t);
		opens.push_back({{"token", t},
				 {"payload_hex", r ? json(hex(*r)) : json(nullptr)}});
	}
	out["open"] = opens;

	/* Sessions as issue() makes them. */
	setenv("WEB_SESSION_TIMEOUT", "43200", 1);
	setenv("WEB_SECURE_COOKIE", "1", 1);
	json sess = json::array();
	struct U {
		uint64_t uid;
		std::string name, role;
		uint32_t epoch;
	};
	for (const U &u : {U{1, "admin", "admin", 0},
			   U{18446744073709551615ull, "we\nird \"name\" ;=", "viewer", 4294967295u},
			   U{42, "", "viewer", 7}}) {
		auto resp = drogon::HttpResponse::newHttpResponse();
		time_t before = time(nullptr);
		auth::session::issue(resp, u.uid, u.name, u.role, u.epoch);
		time_t after = time(nullptr);
		const auto &c = resp->cookies().at(auth::session::kCookie);
		auto cur = auth::session::current(
			reqWithCookie(auth::session::kCookie, c.value()));
		auto req = reqWithCookie(auth::session::kCookie, c.value());
		sess.push_back({{"uid", std::to_string(u.uid)},
				{"username", u.name},
				{"role", u.role},
				{"epoch", u.epoch},
				{"issued_between", {before, after}},
				{"cookie", c.value()},
				{"set_cookie", c.cookieString()},
				{"current", sessionJson(cur)},
				{"csrf", auth::csrf::forSession(req)}});
	}
	out["sessions"] = sess;
	{
		auto resp = drogon::HttpResponse::newHttpResponse();
		auth::session::clear(resp);
		out["session_clear"] =
			resp->cookies().at(auth::session::kCookie).cookieString();
	}

	/* current() on hand-made payloads. */
	json cur = json::array();
	for (std::string p : {std::string("5\n0\n3\nadmin\nbob"),
			      std::string("5\n1\n3\nadmin\nbob"),     /* expired */
			      std::string("5\n4102444800\n3\nviewer\nx\ny"),
			      std::string("5\n0\nadmin\nbob"),        /* old format */
			      std::string("abc\n0\n3\nadmin\nbob"),
			      std::string("5\n0\n4294967296\nadmin\nbob"),
			      std::string("5\n0\n3\n\n"),
			      std::string("-1\n0\n3\nadmin\nbob"),
			      std::string(" 5x\n +0\n+3y\nadmin\nbob"),
			      std::string("5\n-1\n3\nadmin\nbob"),
			      std::string("5\n0\n-1\nadmin\nbob"),
			      std::string("18446744073709551616\n0\n3\nadmin\nbob"),
			      std::string("5\n9223372036854775808\n3\nadmin\nbob"),
			      std::string("5\n0\n18446744073709551616\nadmin\nbob"),
			      std::string("\n0\n3\nadmin\nbob"),
			      std::string("5\n0\n3\nadmin")}) {
		std::string c = auth::token::make(p);
		cur.push_back({{"payload_hex", hex(p)}, {"cookie", c},
			       {"current", sessionJson(auth::session::current(
					reqWithCookie(auth::session::kCookie, c)))}});
	}
	out["session_current"] = cur;

	/* Login CSRF tokens and the pre-auth cookie. */
	json logins = json::array();
	for (int i = 0; i < 3; i++) {
		std::string t = auth::csrf::newLoginToken();
		logins.push_back({{"token", t},
				  {"check", auth::csrf::checkLogin(
					reqWithCookie(auth::csrf::kCookie, t), t)}});
	}
	out["login_tokens"] = logins;
	{
		auto resp = drogon::HttpResponse::newHttpResponse();
		auth::csrf::setLoginCookie(resp, "tok");
		out["login_cookie"] =
			resp->cookies().at(auth::csrf::kCookie).cookieString();
	}

	/* File tokens. */
	json ft = json::array();
	for (uint64_t id : {0ull, 1ull, 2ull, 42ull, 4294967296ull,
			    123456789012345ull, 18446744073709551615ull}) {
		std::string t = auth::filetoken::encrypt(id);
		auto back = auth::filetoken::decrypt(t);
		ft.push_back({{"id", std::to_string(id)}, {"token", t},
			      {"roundtrip", back && *back == id}});
	}
	out["file_tokens"] = ft;
	json ftbad = json::array();
	{
		std::string t = auth::filetoken::encrypt(42);
		std::string flipped = t;
		flipped[0] = flipped[0] == '0' ? '1' : '0';
		std::string flippedEnd = t;
		flippedEnd[63] = flippedEnd[63] == '0' ? '1' : '0';
		std::string upper = t;
		for (auto &ch : upper)
			ch = (char)toupper((unsigned char)ch);
		for (std::string b : {flipped, flippedEnd, t.substr(0, 62), t + "00",
				      std::string(64, '0'), std::string(64, 'g'),
				      upper}) {
			auto r = auth::filetoken::decrypt(b);
			ftbad.push_back({{"token", b},
					 {"id", r ? json(std::to_string(*r)) : json(nullptr)}});
		}
	}
	out["file_token_decrypt"] = ftbad;

	/* Passwords. */
	json pw = json::array();
	for (std::string p : {std::string("hunter2"), std::string(""),
			      std::string("p\xc3\xa4ss w\xc3\xb6rd \xe2\x9c\x93"),
			      std::string(300, 'z')}) {
		std::string h = auth::hashPassword(p);
		pw.push_back({{"password_hex", hex(p)}, {"hash", h},
			      {"verify", auth::verifyPassword(h, p)},
			      {"verify_wrong", auth::verifyPassword(h, p + "x")}});
	}
	out["passwords"] = pw;

	out["daemon"] = daemonCases();

	std::cout << out.dump(1, '\t') << "\n";
	return 0;
}

static int verify(void)
{
	json in = json::parse(std::cin);
	int bad = 0;
	auto check = [&](bool ok, const std::string &what) {
		std::cout << (ok ? "ok   " : "FAIL ") << what << "\n";
		if (!ok)
			bad++;
	};

	initKey(in["app_key"].get<std::string>());

	for (auto &t : in["tokens"]) {
		auto r = auth::token::open(t["token"].get<std::string>());
		check(r && hex(*r) == t["payload_hex"].get<std::string>(),
		      "token open " + t["token"].get<std::string>());
	}
	for (auto &s : in["sessions"]) {
		std::string c = s["cookie"].get<std::string>();
		auto cur = auth::session::current(
			reqWithCookie(auth::session::kCookie, c));
		check(cur && std::to_string(cur->uid) == s["uid"].get<std::string>() &&
			      cur->username == s["username"].get<std::string>() &&
			      cur->role == s["role"].get<std::string>() &&
			      cur->epoch == s["epoch"].get<uint32_t>() &&
			      cur->exp == s["exp"].get<int64_t>(),
		      "session current uid=" + s["uid"].get<std::string>());
		auto req = reqWithCookie(auth::session::kCookie, c);
		check(auth::csrf::forSession(req) == s["csrf"].get<std::string>(),
		      "csrf forSession uid=" + s["uid"].get<std::string>());
		check(auth::csrf::checkSession(req, s["csrf"].get<std::string>()),
		      "csrf checkSession uid=" + s["uid"].get<std::string>());
	}
	for (auto &l : in["login_tokens"]) {
		std::string t = l["token"].get<std::string>();
		check(auth::csrf::checkLogin(reqWithCookie(auth::csrf::kCookie, t), t),
		      "csrf checkLogin " + t);
	}
	for (auto &f : in["file_tokens"]) {
		auto r = auth::filetoken::decrypt(f["token"].get<std::string>());
		check(r && std::to_string(*r) == f["id"].get<std::string>() &&
			      auth::filetoken::encrypt(*r) == f["token"].get<std::string>(),
		      "file token id=" + f["id"].get<std::string>());
	}
	for (auto &p : in["passwords"]) {
		std::string pw = unhex(p["password_hex"].get<std::string>());
		std::string h = p["hash"].get<std::string>();
		check(auth::verifyPassword(h, pw), "password verify " + h);
		check(!auth::verifyPassword(h, pw + "x"), "password reject wrong " + h);
	}
	std::cout << (bad ? "FAILED " : "all ok ") << bad << "\n";
	return bad ? 1 : 0;
}

int main(int argc, char **argv)
{
	if (!auth::initCrypto())
		return 1;
	if (argc > 1 && std::string(argv[1]) == "gen")
		return gen();
	if (argc > 1 && std::string(argv[1]) == "verify")
		return verify();
	std::cerr << "usage: fixgen gen|verify\n";
	return 2;
}

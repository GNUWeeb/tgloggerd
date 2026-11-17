// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "daemon/Api.hpp"

#include "Config.hpp"

#include <drogon/HttpAppFramework.h>
#include <drogon/HttpClient.h>
#include <trantor/net/EventLoop.h>

#include <sodium.h>

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <optional>
#include <utility>

namespace tgweb::daemon::api {

namespace {

/* The label the daemon derives its bearer key from (docs/internal-api.md). */
constexpr const char *kBearerLabel = "tgloggerd:internal-api:v1";

std::string g_base;	/* "http://host:port"; "" = off. */
std::string g_auth;	/* "Bearer <64 hex>".            */

/* base64-decode trying the common alphabets/padding, as auth/FileToken.cpp
 * and the daemon do, so every WEB_APP_KEY spelling both accept works. */
std::optional<std::vector<unsigned char>> b64decodeAny(const std::string &s)
{
	for (int v : {sodium_base64_VARIANT_ORIGINAL,
		      sodium_base64_VARIANT_ORIGINAL_NO_PADDING,
		      sodium_base64_VARIANT_URLSAFE,
		      sodium_base64_VARIANT_URLSAFE_NO_PADDING}) {
		std::vector<unsigned char> out(s.size());
		size_t len = 0;
		if (sodium_base642bin(out.data(), out.size(), s.data(), s.size(),
				      nullptr, &len, nullptr, v) == 0) {
			out.resize(len);
			return out;
		}
	}
	return std::nullopt;
}

/* Accept "http(s)://host[:port]" with at most a trailing slash: Drogon's
 * client takes no path prefix, and the API's paths are absolute. */
std::optional<std::string> parseBase(std::string url)
{
	size_t scheme;
	if (url.rfind("http://", 0) == 0)
		scheme = 7;
	else if (url.rfind("https://", 0) == 0)
		scheme = 8;
	else
		return std::nullopt;
	if (!url.empty() && url.back() == '/')
		url.pop_back();
	if (url.size() <= scheme ||
	    url.find_first_of("/?#@ ", scheme) != std::string::npos)
		return std::nullopt;
	return url;
}

/*
 * One HttpClient per event-loop thread: a client is bound to one loop and
 * resumes its awaiters there, so each request handler thread gets its own and
 * a coroutine never hops threads. Deliberately leaked: the web exits with
 * _exit() and the clients must not be torn down after their loops are gone.
 */
drogon::HttpClientPtr client(void)
{
	thread_local drogon::HttpClientPtr *c = nullptr;
	if (!c) {
		trantor::EventLoop *loop =
			trantor::EventLoop::getEventLoopOfCurrentThread();
		if (!loop)
			loop = drogon::app().getLoop();
		c = new drogon::HttpClientPtr(
			drogon::HttpClient::newHttpClient(g_base, loop));
		(*c)->setUserAgent("tgloggerd_web");
	}
	return *c;
}

} /* namespace */

bool init(void)
{
	std::string url = tgweb::env("WEB_TGLOGGERD_API", "");
	if (url.empty())
		return true;

	auto base = parseBase(url);
	if (!base) {
		fprintf(stderr, "tgloggerd-api: WEB_TGLOGGERD_API must be "
			"http://host:port; the daemon API stays off\n");
		return false;
	}

	auto key = b64decodeAny(tgweb::env("WEB_APP_KEY", ""));
	if (!key || key->size() < 16) {
		fprintf(stderr, "tgloggerd-api: WEB_APP_KEY is not usable; "
			"the daemon API stays off\n");
		return false;
	}

	unsigned char raw[32];
	crypto_generichash(raw, sizeof(raw),
			   reinterpret_cast<const unsigned char *>(kBearerLabel),
			   std::strlen(kBearerLabel), key->data(), key->size());
	char hex[sizeof(raw) * 2 + 1];
	sodium_bin2hex(hex, sizeof(hex), raw, sizeof(raw));
	sodium_memzero(raw, sizeof(raw));
	sodium_memzero(key->data(), key->size());

	g_base = *base;
	g_auth = std::string("Bearer ") + hex;
	sodium_memzero(hex, sizeof(hex));
	return true;
}

bool enabled(void)
{
	return !g_base.empty();
}

const std::string &baseUrl(void)
{
	return g_base;
}

drogon::Task<Reply> call(drogon::HttpMethod method, std::string path,
			 nlohmann::json body, double timeoutSecs)
{
	Reply r;
	if (!enabled()) {
		r.error = "WEB_TGLOGGERD_API is not set";
		co_return r;
	}

	auto req = drogon::HttpRequest::newHttpRequest();
	req->setMethod(method);
	req->setPath(path);
	req->addHeader("Authorization", g_auth);
	if (!body.is_null()) {
		req->setContentTypeCode(drogon::CT_APPLICATION_JSON);
		req->setBody(body.dump());
	}

	drogon::HttpResponsePtr resp;
	try {
		resp = co_await client()->sendRequestCoro(req, timeoutSecs);
	} catch (const drogon::HttpException &e) {
		r.error = e.what();
		co_return r;
	} catch (const std::exception &e) {
		r.error = e.what();
		co_return r;
	}

	r.sent = true;
	r.status = (int)resp->getStatusCode();
	r.retryAfter = resp->getHeader("retry-after");
	auto text = resp->body();
	if (!text.empty()) {
		r.body = nlohmann::json::parse(text.begin(), text.end(),
					       nullptr, false);
		if (r.body.is_discarded())
			r.body = nullptr;
	}
	if (!r.ok()) {
		r.error = "HTTP " + std::to_string(r.status);
		if (r.body.is_object() && r.body.contains("error") &&
		    r.body["error"].is_string())
			r.error += " " + r.body["error"].get<std::string>();
	}
	co_return r;
}

drogon::Task<Reply> recordAccess(std::vector<uint64_t> ids)
{
	/* Small body, quick answer: a daemon that takes longer is treated as
	 * away, and the batch is dropped by the caller. */
	nlohmann::json body = nlohmann::json::object();
	body["ids"] = std::move(ids);
	Reply r = co_await call(drogon::Post, "/v1/files/access",
				std::move(body), 5.0);
	co_return r;
}

drogon::Task<Reply> fetch(uint64_t id, uint64_t offset, uint64_t length)
{
	/* The daemon's limit plus a margin for the round trip and its own
	 * bookkeeping; read once, like the daemon reads it at startup. */
	static const double timeout = [] {
		long secs = strtol(tgweb::env("TG_FETCH_TIMEOUT_SECS", "30")
					   .c_str(), nullptr, 10);
		if (secs <= 0)
			secs = 30;
		return (double)secs + 10.0;
	}();

	nlohmann::json body = nlohmann::json::object();
	body["offset"] = offset;
	body["length"] = length;
	Reply r = co_await call(drogon::Post,
				"/v1/files/" + std::to_string(id) + "/fetch",
				std::move(body), timeout);
	co_return r;
}

} /* namespace tgweb::daemon::api */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_DAEMON_API_HPP
#define TGLOGGERD_WEB_DAEMON_API_HPP

#include <drogon/HttpTypes.h>
#include <drogon/utils/coroutine.h>

#include <nlohmann/json.hpp>

#include <cstdint>
#include <string>
#include <vector>

namespace tgweb::daemon::api {

/*
 * Client for the tgloggerd daemon's internal API (docs/internal-api.md).
 *
 * Only the daemon holds the TDLib session and writes telegram_files, so what
 * the web needs from Telegram or from the file store's bookkeeping is asked of
 * it here. The API is optional: with WEB_TGLOGGERD_API empty the client is
 * off, enabled() is false and every call answers "not sent" without touching
 * the network. Callers must treat the daemon as possibly away at any time.
 *
 * Calls are coroutines on the calling thread's event loop, each with its own
 * timeout; a refused connection or a timeout is an answer like any other, never
 * an exception.
 */

/* The outcome of one call. */
struct Reply {
	bool		sent = false;	/* an HTTP answer arrived (any status). */
	int		status = 0;	/* its status; 0 when !sent.            */
	nlohmann::json	body;		/* parsed JSON body; null if none.      */
	std::string	error;		/* for logs: why it failed, if it did.  */
	std::string	retryAfter;	/* the Retry-After header, "" if none.  */

	/* A 2xx answer. */
	bool ok(void) const { return sent && status >= 200 && status < 300; }
};

/*
 * Read WEB_TGLOGGERD_API ("http://host:port", no path) and derive the bearer
 * key from WEB_APP_KEY exactly as the daemon does. Call once at startup, after
 * libsodium is initialized. An empty WEB_TGLOGGERD_API leaves the client off
 * and returns true; a malformed value or an unusable WEB_APP_KEY leaves it off
 * and returns false so the caller can log it. Never fatal: the web works
 * without the daemon.
 */
bool init(void);

/* Whether the client is configured. */
bool enabled(void);

/* The configured base URL, for logs ("" when off). */
const std::string &baseUrl(void);

/*
 * Send one request: `path` is absolute ("/v1/..."), `body` is sent as JSON
 * unless it is null. Gives up after `timeoutSecs`.
 */
drogon::Task<Reply> call(drogon::HttpMethod method, std::string path,
			 nlohmann::json body, double timeoutSecs);

/* POST /v1/files/access: at most 1000 ids (the daemon refuses more). */
drogon::Task<Reply> recordAccess(std::vector<uint64_t> ids);

/*
 * POST /v1/chats/resolve and /v1/chats/join {"ref": ref}, for the admin
 * join page. The daemon parses ref itself; these only send it.
 */
drogon::Task<Reply> resolveChat(std::string ref);
drogon::Task<Reply> joinChat(std::string ref);

/*
 * POST /v1/files/{id}/fetch: get a file's bytes from Telegram. `length` 0
 * asks for one chunk from `offset`. Answers {state:"stored", id} or
 * {state:"partial", path, offset, available, size}; see the contract in
 * docs/internal-api.md. The daemon bounds a fetch by TG_FETCH_TIMEOUT_SECS
 * (read from the shared environment here), and this call waits a little
 * longer than that, so the daemon's own 504 is what a slow fetch gets.
 */
drogon::Task<Reply> fetch(uint64_t id, uint64_t offset, uint64_t length);

} /* namespace tgweb::daemon::api */

#endif /* TGLOGGERD_WEB_DAEMON_API_HPP */

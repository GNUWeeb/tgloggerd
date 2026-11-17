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
	int		retryAfter = 0;	/* Retry-After seconds; 0 if none.      */

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
 * POST /v1/users/{id}/lookup-name, the "Look up real name" admin action:
 * the daemon removes the contact, reads the user's own name and adds the
 * contact back, so it may take a while.
 */
drogon::Task<Reply> lookupRealName(int64_t userId);

} /* namespace tgweb::daemon::api */

#endif /* TGLOGGERD_WEB_DAEMON_API_HPP */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__INTERNAL_API_HPP
#define TGLOGGERD__INTERNAL_API_HPP

#include <cstddef>
#include <memory>
#include <string>
#include <utility>
#include <vector>
#include <stdexcept>
#include <functional>
#include <unordered_map>

#include <nlohmann/json.hpp>

#include "helpers/log.h"

namespace tgloggerd {

/*
 * The daemon's internal HTTP/1.1 + JSON API for the web
 * (docs/internal-api.md, docs/design/storage-cache.md section 7).
 *
 * The server core knows nothing about files or storage: the operations are
 * routes registered with route() before start(), by whichever part of the
 * daemon owns them. Handlers run on the API's own TG_API_THREADS threads,
 * never on the TDLib loop thread nor on the I/O thread, so they may block
 * (on the database, or on TDLib::request).
 *
 * Boost.Beast/Asio stay inside InternalApi.cpp; this header only exposes
 * plain types and nlohmann::json.
 */
class InternalApi {
public:
	struct Request {
		std::string	method;		/* "GET", "POST", ... */
		std::string	path;		/* without the query string */
		std::string	query;		/* raw query string, may be empty */
		/* Captures of the route's {name} segments. */
		std::unordered_map<std::string, std::string> params;
		/* The parsed body; null when the request has none. */
		nlohmann::json	body;

		const std::string &param(const std::string &name) const;
	};

	struct Response {
		int		status = 200;
		nlohmann::json	body = nlohmann::json::object();
		std::vector<std::pair<std::string, std::string>> headers;
	};

	/*
	 * Thrown by a handler to answer with a JSON error. code is one of the
	 * error codes of docs/internal-api.md; retry_after > 0 adds a
	 * Retry-After header (503).
	 */
	class Error : public std::runtime_error {
	public:
		Error(int status, std::string code, const std::string &message,
		      int retry_after = 0)
			: std::runtime_error(message), status_(status),
			  code_(std::move(code)), retry_after_(retry_after) {}

		int status(void) const noexcept { return status_; }
		const std::string &code(void) const noexcept { return code_; }
		int retryAfter(void) const noexcept { return retry_after_; }

		static Error badRequest(const std::string &msg)
		{
			return Error(400, "bad_request", msg);
		}
		static Error notFound(const std::string &msg)
		{
			return Error(404, "not_found", msg);
		}
		static Error gone(const std::string &msg)
		{
			return Error(410, "gone", msg);
		}
		static Error unavailable(const std::string &msg, int retry_after = 5)
		{
			return Error(503, "unavailable", msg, retry_after);
		}
		static Error timeout(const std::string &msg)
		{
			return Error(504, "timeout", msg);
		}

	private:
		int		status_;
		std::string	code_;
		int		retry_after_;
	};

	using Handler = std::function<Response(const Request &)>;

	struct Config {
		/* "host:port" or "[v6]:port"; empty = the API is off. */
		std::string	listen;
		/* Handler threads (each may hold a DB connection). */
		size_t		threads = 4;
		/* WEB_APP_KEY, base64; the bearer key is derived from it. */
		std::string	web_app_key;
	};

	/* Request bodies above this are refused with 413. */
	static constexpr size_t kMaxBody = 64 * 1024;

	explicit InternalApi(log_hd_t *l);
	~InternalApi(void);

	InternalApi(const InternalApi &) = delete;
	InternalApi &operator=(const InternalApi &) = delete;

	/*
	 * Register a handler for method + pattern. A pattern is a path whose
	 * segments are literal or "{name}", e.g. "/v1/files/{id}"; literal
	 * segments win over captures. Call before start().
	 */
	void route(const std::string &method, const std::string &pattern,
		   Handler h);

	/*
	 * Derive the bearer key from WEB_APP_KEY (base64, any alphabet): the
	 * lowercase hex of the 32-byte keyed BLAKE2b of the label
	 * "tgloggerd:internal-api:v1" under the decoded key. Empty when the
	 * key is missing, undecodable or shorter than 16 bytes.
	 */
	static std::string deriveBearer(const std::string &web_app_key_b64);

	/*
	 * Bind and start the I/O and handler threads. Returns 0 on success,
	 * -1 (logged) on a bad listen address, a missing WEB_APP_KEY or a
	 * bind failure; nothing is left running then.
	 */
	int start(const Config &cfg);

	/*
	 * Stop accepting, let in-flight handlers finish and answer, then
	 * join every thread. Idempotent. Must run before anything the
	 * handlers use (DB pools, TDLib) goes away.
	 */
	void stop(void);

private:
	struct Impl;
	std::unique_ptr<Impl> impl_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__INTERNAL_API_HPP */

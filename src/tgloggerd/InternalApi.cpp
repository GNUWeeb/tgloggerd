// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * The internal API server. This is the only translation unit of the daemon
 * that includes Boost: Beast and Asio are header-only, so nothing links a
 * Boost binary, and the rest of the daemon sees only InternalApi.hpp.
 *
 * Threads: one I/O thread runs the io_context (accept, read, write; never
 * blocks), and a ThreadPool of Config::threads runs the handlers, which may
 * block on the database or on TDLib::request. A request whose handler cannot
 * be queued (too many in flight) is answered 503 at once rather than left to
 * wait behind slow fetches.
 */
#include "InternalApi.hpp"
#include "ThreadPool.hpp"

#include <boost/asio.hpp>
#include <boost/beast/core.hpp>
#include <boost/beast/http.hpp>

#include <sodium.h>

#include <atomic>
#include <cctype>
#include <chrono>
#include <cstring>
#include <mutex>
#include <optional>
#include <thread>
#include <unordered_set>

namespace tgloggerd {

namespace asio  = boost::asio;
namespace beast = boost::beast;
namespace http  = beast::http;
using tcp = asio::ip::tcp;

namespace {

constexpr const char *kBearerLabel = "tgloggerd:internal-api:v1";
/* Idle keep-alive connections and slow peers are dropped after this. */
constexpr auto kIoTimeout = std::chrono::seconds(30);
/* Open connections at most; more are closed on accept. */
constexpr size_t kMaxConnections = 256;
/* Queued + running handlers per handler thread before answering 503. */
constexpr size_t kInflightPerThread = 4;

/* base64-decode trying the common alphabets/padding, like FileToken.cpp. */
std::vector<unsigned char> b64decode_any(const std::string &s)
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
	return {};
}

std::vector<std::string> split_path(const std::string &path)
{
	std::vector<std::string> out;
	size_t i = 0;
	while (i < path.size()) {
		if (path[i] == '/') {
			i++;
			continue;
		}
		size_t j = path.find('/', i);
		if (j == std::string::npos)
			j = path.size();
		out.emplace_back(path, i, j - i);
		i = j;
	}
	return out;
}

bool is_capture(const std::string &seg)
{
	return seg.size() > 2 && seg.front() == '{' && seg.back() == '}';
}

InternalApi::Response error_response(int status, const std::string &code,
				     const std::string &message,
				     int retry_after = 0)
{
	InternalApi::Response r;
	r.status = status;
	r.body = { { "error", code }, { "message", message } };
	if (retry_after > 0)
		r.headers.emplace_back("Retry-After", std::to_string(retry_after));
	return r;
}

/* Case-insensitive "Bearer " prefix check; returns the token or empty. */
std::string bearer_token(beast::string_view auth)
{
	static const char kScheme[] = "bearer ";
	const size_t n = sizeof(kScheme) - 1;
	if (auth.size() <= n)
		return std::string();
	for (size_t i = 0; i < n; i++) {
		if (std::tolower((unsigned char)auth[i]) != kScheme[i])
			return std::string();
	}
	size_t i = n;
	while (i < auth.size() && auth[i] == ' ')
		i++;
	return std::string(auth.substr(i));
}

struct Route {
	std::string			method;
	std::vector<std::string>	segs;
	size_t				literals;
	InternalApi::Handler		handler;
};

class Session;

struct ApiServer {
	explicit ApiServer(log_hd_t *l) : l(l) {}

	log_hd_t			*l;
	std::vector<Route>		routes;
	std::string			bearer;

	/*
	 * Live sessions, so stop() can close idle ones. Declared before io:
	 * sessions still referenced by handlers in io deregister on
	 * destruction.
	 */
	std::mutex			sessions_mtx;
	std::unordered_set<Session *>	sessions;

	/* Declared before the objects that use it, so destroyed after. */
	asio::io_context		io{1};
	std::optional<asio::executor_work_guard<asio::io_context::executor_type>> guard;
	std::optional<tcp::acceptor>	acceptor;
	std::optional<asio::steady_timer> accept_retry;
	std::thread			io_thread;
	std::unique_ptr<ThreadPool>	pool;

	std::atomic<size_t>		inflight{0};
	size_t				max_inflight = 0;
	/* Set on the I/O thread when stop() begins. */
	bool				stopping = false;
	bool				running = false;

	void do_accept(void);
	void close_idle(void);
	/*
	 * Find the route for a request. Fills params on a match; on a miss,
	 * allow lists the methods the path has under other routes.
	 */
	const Route *match(const std::string &method, const std::string &path,
			   std::unordered_map<std::string, std::string> &params,
			   std::string &allow) const;
	InternalApi::Response run(const Route &rt,
				  const InternalApi::Request &req);
};

class Session : public std::enable_shared_from_this<Session> {
public:
	Session(ApiServer &api, tcp::socket &&s)
		: api_(api), stream_(std::move(s))
	{
		std::lock_guard<std::mutex> lk(api_.sessions_mtx);
		api_.sessions.insert(this);
	}

	~Session(void)
	{
		std::lock_guard<std::mutex> lk(api_.sessions_mtx);
		api_.sessions.erase(this);
	}

	void start(void) { do_read(); }

	/* I/O thread only, under sessions_mtx. */
	void close_if_idle(void)
	{
		if (!reading_)
			return;
		beast::error_code ec;
		stream_.socket().shutdown(tcp::socket::shutdown_both, ec);
		stream_.socket().close(ec);
	}

private:
	void do_read(void)
	{
		parser_.emplace();
		parser_->body_limit(InternalApi::kMaxBody);
		stream_.expires_after(kIoTimeout);
		reading_ = true;
		http::async_read(stream_, buf_, *parser_,
				 beast::bind_front_handler(&Session::on_read,
							   shared_from_this()));
	}

	void on_read(beast::error_code ec, size_t)
	{
		reading_ = false;
		if (ec == http::error::body_limit) {
			send(error_response(413, "too_large",
					    "request body over 64 KiB"),
			     false, 11);
			return;
		}
		if (ec) {
			close();
			return;
		}

		auto req = parser_->release();
		const bool keep = req.keep_alive();
		const unsigned ver = req.version();

		if (api_.stopping) {
			send(error_response(503, "unavailable",
					    "the daemon is shutting down", 5),
			     false, ver);
			return;
		}

		/* Authenticate before routing, so routes are not probeable. */
		std::string tok = bearer_token(req[http::field::authorization]);
		if (tok.size() != api_.bearer.size() ||
		    sodium_memcmp(tok.data(), api_.bearer.data(),
				  tok.size()) != 0) {
			auto r = error_response(401, "unauthorized",
						"missing or bad bearer key");
			r.headers.emplace_back("WWW-Authenticate", "Bearer");
			send(std::move(r), keep, ver);
			return;
		}

		InternalApi::Request r;
		r.method = std::string(req.method_string());
		std::string target(req.target());
		size_t q = target.find('?');
		r.path = target.substr(0, q);
		if (q != std::string::npos)
			r.query = target.substr(q + 1);

		std::string allow;
		const Route *rt = api_.match(r.method, r.path, r.params, allow);
		if (!rt) {
			if (!allow.empty()) {
				auto e = error_response(405, "method_not_allowed",
							"method not allowed");
				e.headers.emplace_back("Allow", allow);
				send(std::move(e), keep, ver);
			} else {
				send(error_response(404, "not_found",
						    "no such endpoint"),
				     keep, ver);
			}
			return;
		}

		const std::string &body = req.body();
		if (!body.empty()) {
			r.body = nlohmann::json::parse(body, nullptr, false);
			if (r.body.is_discarded()) {
				send(error_response(400, "bad_request",
						    "body is not valid JSON"),
				     keep, ver);
				return;
			}
		}

		if (api_.inflight.fetch_add(1) >= api_.max_inflight) {
			api_.inflight.fetch_sub(1);
			send(error_response(503, "unavailable",
					    "too many requests in flight", 1),
			     keep, ver);
			return;
		}

		auto self = shared_from_this();
		api_.pool->post([self, rt, r = std::move(r), keep, ver]() mutable {
			auto resp = self->api_.run(*rt, r);
			self->api_.inflight.fetch_sub(1);
			auto ex = self->stream_.get_executor();
			asio::post(ex, [self, resp = std::move(resp), keep,
					ver]() mutable {
				self->send(std::move(resp), keep, ver);
			});
		});
	}

	void send(InternalApi::Response resp, bool keep, unsigned ver)
	{
		auto res = std::make_shared<http::response<http::string_body>>(
			static_cast<http::status>(resp.status), ver);
		res->set(http::field::server, "tgloggerd");
		res->set(http::field::content_type, "application/json");
		res->set(http::field::cache_control, "no-store");
		for (const auto &h : resp.headers)
			res->set(h.first, h.second);
		res->body() = resp.body.dump(-1, ' ', false,
				nlohmann::json::error_handler_t::replace);
		res->keep_alive(keep && !api_.stopping);
		res->prepare_payload();

		stream_.expires_after(kIoTimeout);
		auto self = shared_from_this();
		http::async_write(stream_, *res,
			[self, res](beast::error_code ec, size_t) {
				if (ec || !res->keep_alive() ||
				    self->api_.stopping) {
					self->close();
					return;
				}
				self->do_read();
			});
	}

	void close(void)
	{
		beast::error_code ec;
		stream_.socket().shutdown(tcp::socket::shutdown_send, ec);
		stream_.socket().close(ec);
	}

	ApiServer						&api_;
	beast::tcp_stream					stream_;
	beast::flat_buffer					buf_;
	std::optional<http::request_parser<http::string_body>>	parser_;
	bool							reading_ = false;
};

void ApiServer::do_accept(void)
{
	acceptor->async_accept([this](beast::error_code ec, tcp::socket s) {
		if (ec == asio::error::operation_aborted || stopping)
			return;
		if (ec) {
			/* EMFILE and friends: back off instead of spinning. */
			pr_warn(l, "api: accept failed: %s", ec.message().c_str());
			accept_retry->expires_after(std::chrono::milliseconds(200));
			accept_retry->async_wait([this](beast::error_code e) {
				if (!e && !stopping)
					do_accept();
			});
			return;
		}

		size_t n;
		{
			std::lock_guard<std::mutex> lk(sessions_mtx);
			n = sessions.size();
		}
		if (n >= kMaxConnections) {
			beast::error_code e;
			s.close(e);
		} else {
			std::make_shared<Session>(*this, std::move(s))->start();
		}
		do_accept();
	});
}

void ApiServer::close_idle(void)
{
	std::lock_guard<std::mutex> lk(sessions_mtx);
	for (Session *s : sessions)
		s->close_if_idle();
}

const Route *ApiServer::match(const std::string &method,
		const std::string &path,
		std::unordered_map<std::string, std::string> &params,
		std::string &allow) const
{
	auto segs = split_path(path);
	const Route *best = nullptr;

	for (const auto &rt : routes) {
		if (rt.segs.size() != segs.size())
			continue;
		bool ok = true;
		for (size_t i = 0; i < segs.size() && ok; i++)
			ok = is_capture(rt.segs[i]) || rt.segs[i] == segs[i];
		if (!ok)
			continue;
		if (rt.method != method) {
			if (allow.find(rt.method) == std::string::npos)
				allow += (allow.empty() ? "" : ", ") + rt.method;
			continue;
		}
		if (!best || rt.literals > best->literals)
			best = &rt;
	}
	if (!best)
		return nullptr;

	for (size_t i = 0; i < segs.size(); i++) {
		const auto &s = best->segs[i];
		if (is_capture(s))
			params[s.substr(1, s.size() - 2)] = segs[i];
	}
	allow.clear();
	return best;
}

InternalApi::Response ApiServer::run(const Route &rt,
				     const InternalApi::Request &req)
{
	try {
		auto resp = rt.handler(req);
		pr_debug(l, "api: %s %s -> %d", req.method.c_str(),
			 req.path.c_str(), resp.status);
		return resp;
	} catch (const InternalApi::Error &e) {
		pr_debug(l, "api: %s %s -> %d %s: %s", req.method.c_str(),
			 req.path.c_str(), e.status(), e.code().c_str(),
			 e.what());
		return error_response(e.status(), e.code(), e.what(),
				      e.retryAfter());
	} catch (const nlohmann::json::exception &e) {
		/* A body of the wrong shape reached a handler's accessors. */
		return error_response(400, "bad_request", e.what());
	} catch (const std::exception &e) {
		pr_error(l, "api: %s %s failed: %s", req.method.c_str(),
			 req.path.c_str(), e.what());
		return error_response(500, "internal", "internal error");
	}
}

} /* namespace */

struct InternalApi::Impl : ApiServer {
	using ApiServer::ApiServer;
};

const std::string &InternalApi::Request::param(const std::string &name) const
{
	static const std::string empty;
	auto it = params.find(name);
	return it == params.end() ? empty : it->second;
}

InternalApi::InternalApi(log_hd_t *l) : impl_(std::make_unique<Impl>(l)) {}

InternalApi::~InternalApi(void)
{
	stop();
}

void InternalApi::route(const std::string &method, const std::string &pattern,
			Handler h)
{
	Route rt;
	rt.method = method;
	rt.segs = split_path(pattern);
	rt.literals = 0;
	for (const auto &s : rt.segs)
		rt.literals += is_capture(s) ? 0 : 1;
	rt.handler = std::move(h);
	impl_->routes.push_back(std::move(rt));
}

std::string InternalApi::deriveBearer(const std::string &web_app_key_b64)
{
	if (sodium_init() < 0 || web_app_key_b64.empty())
		return std::string();

	auto raw = b64decode_any(web_app_key_b64);
	if (raw.size() < 16)
		return std::string();

	/* Same subkey construction as FileToken: keyed BLAKE2b of a label. */
	unsigned char key[32];
	crypto_generichash(key, sizeof(key),
			   reinterpret_cast<const unsigned char *>(kBearerLabel),
			   std::strlen(kBearerLabel), raw.data(), raw.size());
	sodium_memzero(raw.data(), raw.size());

	char hex[sizeof(key) * 2 + 1];
	sodium_bin2hex(hex, sizeof(hex), key, sizeof(key));
	sodium_memzero(key, sizeof(key));
	return std::string(hex, sizeof(key) * 2);
}

int InternalApi::start(const Config &cfg)
{
	Impl &d = *impl_;
	log_hd_t *l = d.l;

	if (d.running)
		return 0;
	if (cfg.listen.empty()) {
		pr_info(l, "api: TG_API_LISTEN is empty; the internal API is off");
		return 0;
	}

	d.bearer = deriveBearer(cfg.web_app_key);
	if (d.bearer.empty()) {
		pr_error(l, "api: WEB_APP_KEY is missing or invalid; "
			 "not starting the internal API");
		return -1;
	}

	/* "host:port" or "[v6addr]:port". */
	std::string host, port;
	const std::string &lst = cfg.listen;
	if (!lst.empty() && lst[0] == '[') {
		size_t e = lst.find("]:");
		if (e != std::string::npos) {
			host = lst.substr(1, e - 1);
			port = lst.substr(e + 2);
		}
	} else {
		size_t c = lst.rfind(':');
		if (c != std::string::npos) {
			host = lst.substr(0, c);
			port = lst.substr(c + 1);
		}
	}
	char *endp = nullptr;
	unsigned long pnum = port.empty() ? 70000 : strtoul(port.c_str(), &endp, 10);
	beast::error_code ec;
	auto addr = asio::ip::make_address(host, ec);
	if (ec || host.empty() || !endp || *endp || pnum > 65535) {
		pr_error(l, "api: bad TG_API_LISTEN \"%s\" (want host:port); "
			 "not starting the internal API", lst.c_str());
		return -1;
	}

	tcp::endpoint ep(addr, (unsigned short)pnum);
	d.acceptor.emplace(d.io);
	d.acceptor->open(ep.protocol(), ec);
	if (!ec)
		d.acceptor->set_option(asio::socket_base::reuse_address(true), ec);
	if (!ec)
		d.acceptor->bind(ep, ec);
	if (!ec)
		d.acceptor->listen(asio::socket_base::max_listen_connections, ec);
	if (ec) {
		pr_error(l, "api: cannot listen on %s: %s; not starting the "
			 "internal API", lst.c_str(), ec.message().c_str());
		d.acceptor.reset();
		return -1;
	}
	auto bound = d.acceptor->local_endpoint(ec);

	size_t threads = cfg.threads ? cfg.threads : 1;
	d.max_inflight = threads * kInflightPerThread;
	d.pool = std::make_unique<ThreadPool>(threads, 0, l);
	d.accept_retry.emplace(d.io);
	d.guard.emplace(asio::make_work_guard(d.io));
	d.stopping = false;
	d.do_accept();
	d.io_thread = std::thread([&d] {
		for (;;) {
			try {
				d.io.run();
				return;
			} catch (const std::exception &e) {
				pr_error(d.l, "api: I/O thread: %s", e.what());
			}
		}
	});
	d.running = true;

	pr_info(l, "api: internal API listening on %s:%u (%zu threads, "
		"%zu routes)", bound.address().to_string().c_str(),
		(unsigned)bound.port(), threads, d.routes.size());
	return 0;
}

void InternalApi::stop(void)
{
	Impl &d = *impl_;
	if (!d.running)
		return;
	d.running = false;

	/* No new connections or requests; idle keep-alives go now. */
	asio::post(d.io, [&d] {
		d.stopping = true;
		beast::error_code ec;
		d.acceptor->close(ec);
		d.accept_retry->cancel();
		d.close_idle();
	});
	/* Handlers already accepted finish; their answers are posted. */
	d.pool->shutdown();
	/* Once those are written their sessions close (stopping), and with
	 * the guard gone run() returns; writes are bounded by kIoTimeout. */
	asio::post(d.io, [&d] { d.close_idle(); });
	d.guard.reset();
	d.io_thread.join();

	d.pool.reset();
	d.acceptor.reset();
	pr_info(d.l, "api: internal API stopped");
}

} /* namespace tgloggerd */

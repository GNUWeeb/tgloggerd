// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * THE ONLY TRANSLATION UNIT IN gwdiscord THAT INCLUDES BOOST.
 *
 * Everything Boost.Beast-shaped is confined here, behind the gwdiscord::
 * WebSocket / HttpClient interfaces. No Boost type appears in any header, so
 * no other TU -- and no consumer of this library -- compiles against Boost at
 * all. To move to a different network stack, add a sibling .cpp implementing
 * the same two interfaces and swap one line in CMakeLists.txt; see README.md.
 */
#include <gwdiscord/Transport.hpp>

#include <boost/asio/connect.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/ssl/host_name_verification.hpp>
#include <boost/asio/ssl/stream.hpp>
#include <boost/beast/core.hpp>
#include <boost/beast/http.hpp>
#include <boost/beast/version.hpp>
#include <boost/beast/websocket.hpp>
#include <boost/beast/websocket/ssl.hpp>

#include <atomic>
#include <future>
#include <memory>
#include <mutex>
#include <thread>

namespace beast = boost::beast;
namespace http  = boost::beast::http;
namespace ws    = boost::beast::websocket;
namespace net   = boost::asio;
namespace ssl   = boost::asio::ssl;
using tcp       = boost::asio::ip::tcp;

namespace gwdiscord {
namespace {

/* Shared TLS setup: system trust store, peer + hostname verification. */
static void init_ssl_ctx(ssl::context &ctx, const std::string &host)
{
	ctx.set_default_verify_paths();
	ctx.set_verify_mode(ssl::verify_peer);
	ctx.set_verify_callback(ssl::host_name_verification(host));
}

class BeastWebSocket final : public WebSocket {
public:
	BeastWebSocket(void)
		: ctx_(ssl::context::tlsv12_client),
		  work_(net::make_work_guard(ioc_)) {}

	~BeastWebSocket(void) override
	{
		disconnect();
		work_.reset();
		if (runner_.joinable())
			runner_.join();
	}

	/*
	 * Called once, before any read/write/close, from the thread that owns
	 * this socket; the io thread starts only when it succeeds. Everything
	 * in here runs synchronously on the caller, which is safe precisely
	 * because nothing else can be touching the stream yet.
	 */
	bool connect(const std::string &host, const std::string &target,
		     std::string *err) override
	{
		std::lock_guard<std::mutex> lk(state_mtx_);
		if (disconnected_.load()) {
			/*
			 * A disconnect() that raced us -- the owner is being
			 * stopped. Refusing here is what makes stop() during a
			 * connect attempt stick instead of being wiped by the
			 * attempt succeeding afterwards.
			 */
			if (err)
				*err = "socket was disconnected";
			return false;
		}
		try {
			init_ssl_ctx(ctx_, host);
			s_ = std::make_unique<Stream>(ioc_, ctx_);

			tcp::resolver resolver{ioc_};
			auto const results = resolver.resolve(host, "443");
			net::connect(s_->next_layer().next_layer(), results);

			/* SNI: Discord's edge requires it. */
			if (!SSL_set_tlsext_host_name(
				    s_->next_layer().native_handle(),
				    host.c_str())) {
				if (err)
					*err = "failed to set TLS SNI hostname";
				s_.reset();
				return false;
			}
			s_->next_layer().handshake(ssl::stream_base::client);

			/*
			 * Beast's default is a permessage-deflate-less, 16 MiB
			 * message limit; Discord frames stay far below it.
			 */
			s_->set_option(ws::stream_base::decorator(
				[](ws::request_type &req) {
					req.set(http::field::user_agent,
						"gwdiscord (https://git.gnuweeb.net/GNUWeeb/tgloggerd)");
				}));
			s_->handshake(host, target);

			/*
			 * From here on every stream operation is an async one
			 * initiated on this thread and RUN on the io thread;
			 * see the class comment at the bottom.
			 */
			runner_ = std::thread([this] { ioc_.run(); });
			running_ = true;
			return true;
		} catch (const std::exception &e) {
			if (err)
				*err = e.what();
			s_.reset();
			return false;
		}
	}

	ReadStatus read(std::string &out, std::string *err) override
	{
		if (!s_) {
			if (err)
				*err = "read on a closed WebSocket";
			return ReadStatus::Error;
		}

		struct Result {
			beast::error_code ec;
			uint16_t          code = 0;
			std::string       reason;
		};
		beast::flat_buffer buf;
		std::promise<Result> pr;
		auto fut = pr.get_future();

		net::post(ioc_, [&] {
			s_->async_read(buf, [&](beast::error_code ec, size_t) {
				Result r;
				r.ec = ec;
				/*
				 * The close code is stream state; take it here
				 * on the io thread, where nothing else can be
				 * running against the stream, not afterwards
				 * on the caller's thread.
				 */
				if (ec == ws::error::closed) {
					r.code = static_cast<uint16_t>(
						s_->reason().code);
					r.reason = std::string(
						s_->reason().reason.data(),
						s_->reason().reason.size());
				}
				pr.set_value(std::move(r));
			});
		});
		Result r = fut.get();

		if (r.ec == ws::error::closed) {
			/*
			 * Beast surfaces the close code as a typed field. This
			 * is what lets the Gateway distinguish a fatal 4014
			 * from a recoverable drop -- see is_fatal_close_code().
			 */
			peer_close_.code = r.code;
			peer_close_.reason = r.reason;
			return ReadStatus::Closed;
		}
		if (r.ec) {
			/* A disconnect() we asked for is not an error. */
			if (disconnected_.load())
				return ReadStatus::Closed;
			if (err)
				*err = r.ec.message();
			return ReadStatus::Error;
		}

		out = beast::buffers_to_string(buf.data());
		return ReadStatus::Message;
	}

	bool write(const std::string &payload, std::string *err) override
	{
		if (!s_) {
			if (err)
				*err = "write on a closed WebSocket";
			return false;
		}
		/*
		 * One write-class operation in flight at a time (Beast's
		 * rule); the blocking wait under the mutex enforces it.
		 */
		std::lock_guard<std::mutex> lk(write_mtx_);
		std::promise<beast::error_code> pr;
		auto fut = pr.get_future();
		net::post(ioc_, [&] {
			s_->text(true);
			s_->async_write(net::buffer(payload),
					[&](beast::error_code ec, size_t) {
						pr.set_value(ec);
					});
		});
		const beast::error_code ec = fut.get();
		if (ec) {
			if (err)
				*err = ec.message();
			return false;
		}
		return true;
	}

	void close(uint16_t code, const std::string &reason) override
	{
		if (!s_ || !running_)
			return;
		/* async_close is a write-class operation: same serialization. */
		std::lock_guard<std::mutex> lk(write_mtx_);
		std::promise<beast::error_code> pr;
		auto fut = pr.get_future();
		net::post(ioc_, [&] {
			/*
			 * Beast htons-casts whatever code it is handed, so
			 * Discord's 4000-4999 range passes through verbatim.
			 * Closing with 1000 or 1001 would make the session
			 * unresumable.
			 */
			s_->async_close(
				ws::close_reason(
					static_cast<ws::close_code>(code),
					reason),
				[&](beast::error_code ec) { pr.set_value(ec); });
		});
		(void)fut.get(); /* Best-effort: we are tearing down anyway. */
	}

	CloseInfo peer_close(void) const override
	{
		return peer_close_;
	}

	void disconnect(void) override
	{
		std::lock_guard<std::mutex> lk(state_mtx_);
		disconnected_ = true;
		if (!s_)
			return;
		/*
		 * Shut the TCP socket down underneath the TLS and WebSocket
		 * layers: that is what makes a read() blocked on the io thread
		 * complete promptly. Once the io thread runs, the socket may
		 * only be touched from there; before it runs (a connect that
		 * failed, or never happened) there is no concurrency and the
		 * direct call is fine.
		 */
		if (running_) {
			net::post(ioc_, [this] { raw_close(); });
		} else {
			raw_close();
		}
	}

private:
	using Stream = ws::stream<ssl::stream<tcp::socket>>;

	void raw_close(void)
	{
		beast::error_code ec;
		s_->next_layer().next_layer().shutdown(
			tcp::socket::shutdown_both, ec);
		s_->next_layer().next_layer().close(ec);
	}

	/*
	 * THREADING. The synchronous first cut of this class had the gateway
	 * thread blocked in a sync read while the heartbeat thread called sync
	 * write on the same stream. That is undefined: Beast's one-reader/
	 * one-writer allowance covers ASYNC operations only, a sync read
	 * writes its own auto-pong replies to WS pings, and both threads end
	 * up in SSL_read/SSL_write on one SSL*, which OpenSSL does not
	 * support -- TLS 1.3 KeyUpdates make the reader write even without a
	 * ping in flight. So the stream is now driven entirely by ONE io
	 * thread running ioc_: public calls post async operations and block
	 * on their completion, keeping the blocking Transport contract while
	 * no two operations ever touch the stream concurrently.
	 */
	net::io_context		ioc_;
	ssl::context		ctx_;
	net::executor_work_guard<net::io_context::executor_type> work_;
	std::unique_ptr<Stream>	s_;
	std::thread		runner_;
	bool			running_ = false;
	std::mutex		state_mtx_; /* connect vs disconnect */
	std::mutex		write_mtx_;
	CloseInfo		peer_close_;
	std::atomic<bool>	disconnected_{false};
};

class BeastHttpClient final : public HttpClient {
public:
	HttpResponse get(const std::string &host, const std::string &target,
			 const HttpHeaders &headers) override
	{
		HttpResponse out;
		try {
			net::io_context ioc;
			ssl::context ctx{ssl::context::tlsv12_client};
			init_ssl_ctx(ctx, host);

			ssl::stream<tcp::socket> s{ioc, ctx};
			tcp::resolver resolver{ioc};

			/*
			 * Bound every phase of the exchange. This GET runs on
			 * the gateway thread during (re)connect discovery, and
			 * Gateway::stop() can only unblock the WebSocket -- a
			 * stalled GET /gateway/bot would wedge that thread
			 * with nothing able to reach it. Kernel socket
			 * timeouts cover connect, the TLS handshake and both
			 * HTTP halves alike, since they all reduce to reads
			 * and writes on this one socket. DNS resolution is the
			 * remaining unbounded step; bounding it needs an async
			 * resolver and is not worth the machinery for a
			 * lookup of one well-known name.
			 */
			auto const eps = resolver.resolve(host, "443");
			boost::system::error_code cec =
				net::error::host_not_found;
			for (const auto &ep : eps) {
				tcp::socket &raw = s.next_layer();
				raw.close(cec);
				raw.open(ep.endpoint().protocol());
				const struct timeval tv = { 30, 0 };
				setsockopt(raw.native_handle(), SOL_SOCKET,
					   SO_RCVTIMEO, &tv, sizeof(tv));
				setsockopt(raw.native_handle(), SOL_SOCKET,
					   SO_SNDTIMEO, &tv, sizeof(tv));
				raw.connect(ep.endpoint(), cec);
				if (!cec)
					break;
			}
			if (cec)
				throw boost::system::system_error(cec);
			if (!SSL_set_tlsext_host_name(s.native_handle(),
						      host.c_str())) {
				out.error = "failed to set TLS SNI hostname";
				return out;
			}
			s.handshake(ssl::stream_base::client);

			http::request<http::empty_body> req{http::verb::get,
							    target, 11};
			req.set(http::field::host, host);
			for (const auto &h : headers)
				req.set(h.first, h.second);
			http::write(s, req);

			beast::flat_buffer buf;
			http::response<http::string_body> res;
			http::read(s, buf, res);

			out.status = static_cast<long>(res.result_int());
			out.body = res.body();

			beast::error_code ec;
			s.shutdown(ec); /* stream_truncated here is benign. */
			return out;
		} catch (const std::exception &e) {
			out.status = 0;
			out.error = e.what();
			return out;
		}
	}
};

} /* anonymous namespace */

Transport beast_transport(void)
{
	Transport tp;
	tp.ws = [] { return std::unique_ptr<WebSocket>(new BeastWebSocket()); };
	tp.http = [] {
		return std::unique_ptr<HttpClient>(new BeastHttpClient());
	};
	return tp;
}

} /* namespace gwdiscord */

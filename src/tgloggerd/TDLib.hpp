// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__TDLIB_HPP
#define TGLOGGERD__TDLIB_HPP

#include <cstdint>
#include <string>
#include <memory>
#include <functional>

#include <tgloggerd/models/User.hpp>

namespace tgloggerd {

/*
 * A text message delivered to the message handler.
 *
 * It intentionally uses only plain types so that TDLib headers do not
 * leak into other tgloggerd sources.
 */
struct TextMessage {
	int64_t		sender_id;
	int64_t		message_id;
	std::string	sender_name;
	std::string	sender_username;
	std::string	text;
};

/*
 * tgloggerd::TDLib is a wrapper class for TDLib.
 *
 * Since TDLib contains very heavy header files, keep tgloggerd
 * compilation time low by not including TDLib header files in
 * other tgloggerd files. Expose only used functions in
 * tgloggerd::TDLib class.
 */
class TDLib {
public:
	TDLib(uint32_t api_id, const char *api_hash, const char *data_dir);
	~TDLib(void);

	TDLib(const TDLib &) = delete;
	TDLib &operator=(const TDLib &) = delete;

	/*
	 * Set the callback invoked for every incoming text message.
	 */
	void setMessageHandler(std::function<void(const TextMessage &)> cb);

	/*
	 * Set the callback invoked whenever a user's information is received
	 * or updated (td_api::updateUser).
	 */
	void setUserHandler(std::function<void(const models::User &)> cb);

	/*
	 * Process a single batch of TDLib events, waiting up to @timeout
	 * seconds for one to arrive. Drives authentication and message
	 * delivery. Call it repeatedly until isStopped() returns true.
	 */
	void loop(int timeout);

	/*
	 * Whether the TDLib client has been closed and the loop should
	 * terminate.
	 */
	bool isStopped(void) const;

	/*
	 * Request a graceful shutdown of the TDLib client.
	 */
	void close(void);

private:
	struct Impl;
	std::unique_ptr<Impl> impl_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__TDLIB_HPP */

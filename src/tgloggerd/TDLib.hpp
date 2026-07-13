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
#include <tgloggerd/models/Group.hpp>
#include <tgloggerd/models/PrivateMessage.hpp>

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
 * A profile photo whose download has completed, ready to be stored.
 */
struct ProfilePhoto {
	int64_t		user_id;
	std::string	local_path;
	std::string	tg_file_id;
	int64_t		file_size;
};

/*
 * A group photo whose download has completed, ready to be stored.
 */
struct GroupPhoto {
	int64_t		group_id;
	std::string	local_path;
	std::string	tg_file_id;
	int64_t		file_size;
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
	 * Set the callback invoked for every private-chat message
	 * (new, edited, or deleted). Replaces the simpler
	 * setMessageHandler for full private message tracking.
	 */
	void setPrivateMessageHandler(
		std::function<void(const models::PrivateMessage &)> cb);

	/*
	 * Set the callback invoked whenever a user's information is received
	 * or updated (td_api::updateUser).
	 */
	void setUserHandler(std::function<void(const models::User &)> cb);

	/*
	 * Set the callback invoked when a user's (big) profile photo has
	 * finished downloading.
	 */
	void setProfilePhotoHandler(std::function<void(const ProfilePhoto &)> cb);

	/*
	 * Set the callback invoked whenever a group's information is received
	 * or updated (assembled from the chat, supergroup/basicGroup and
	 * full-info objects).
	 */
	void setGroupHandler(std::function<void(const models::Group &)> cb);

	/*
	 * Set the callback invoked when a group's (big) photo has finished
	 * downloading.
	 */
	void setGroupPhotoHandler(std::function<void(const GroupPhoto &)> cb);

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

// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__DB_HPP
#define TGLOGGERD__DB_HPP

#include <mysql/Database.hpp>
#include <tgloggerd/models/User.hpp>
#include <tgloggerd/models/File.hpp>
#include <tgloggerd/models/Group.hpp>
#include <tgloggerd/models/PrivateMessage.hpp>

namespace tgloggerd {

/*
 * tgloggerd::DB is the tgloggerd-specific database facade. It maps the
 * tgloggerd models onto SQL statements executed through the generic
 * mysql::Database, so the rest of tgloggerd never writes SQL directly.
 */
class DB {
public:
	explicit DB(const mysql::Config &cfg);
	~DB(void);

	/* Verify connectivity; throws std::runtime_error on failure. */
	void ping(void);

	/*
	 * Insert or update a user together with its usernames, atomically.
	 * Does not touch users.profile_photo_file_id, which is managed
	 * separately once the profile photo has been downloaded.
	 */
	void upsertUser(const models::User &u);

	/*
	 * Insert a file, or, if a row with the same SHA-256 already exists,
	 * bump its hit_count. Returns the files.id in both cases.
	 */
	uint64_t upsertFile(const models::File &f);

	/* Point a user's profile_photo_file_id at a files row. */
	void setUserProfilePhoto(int64_t user_id, uint64_t file_id);

	/*
	 * Insert or update a group together with its usernames, atomically,
	 * recording title/description/username changes in the history
	 * tables. Does not touch groups.photo_file_id, which is managed by
	 * setGroupPhoto.
	 */
	void upsertGroup(const models::Group &g);

	/* Point a group's photo_file_id at a files row. */
	void setGroupPhoto(int64_t group_id, uint64_t file_id);

	/*
	 * Insert or update a private-chat message. Handles:
	 *  - First-seen messages (insert).
	 *  - Edits (copies old row into private_message_edits, then
	 *    updates private_messages).
	 *  - Deletions (sets is_deleted = 1, keeps the row).
	 *  - Forward info (inserts into private_message_fwd_info if
	 *    present and not already recorded).
	 */
	void upsertPrivateMessage(const models::PrivateMessage &msg);

private:
	void syncUsernames(mysql::Transaction &tx, const models::User &u);
	void trackProfilePhotoChange(mysql::Transaction &tx,
				     int64_t user_id, uint64_t file_id);
	void syncGroupUsernames(mysql::Transaction &tx, const models::Group &g);
	void trackGroupPhotoChange(mysql::Transaction &tx,
				   int64_t group_id, uint64_t file_id);
	void insertForwardInfo(mysql::Transaction &tx,
			       uint64_t private_message_id,
			       const models::ForwardInfo &info);

	mysql::Database db_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__DB_HPP */

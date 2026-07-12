// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>

namespace tgloggerd {

namespace {

const char *user_type_to_string(models::UserType t)
{
	switch (t) {
	case models::UserType::Regular:	return "regular";
	case models::UserType::Deleted:	return "deleted";
	case models::UserType::Bot:	return "bot";
	case models::UserType::Unknown:	return "unknown";
	}
	return "unknown";
}

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

} /* namespace */

void DB::upsertUser(const models::User &u)
{
	/*
	 * Note: profile_photo_file_id and the created_at/updated_at columns
	 * are intentionally omitted; the photo reference is managed after
	 * the photo has been downloaded.
	 */
	static const char *sql =
		"INSERT INTO users ("
		" id, first_name, last_name, phone_number, type,"
		" accent_color_id, background_custom_emoji_id,"
		" profile_accent_color_id, profile_background_custom_emoji_id,"
		" emoji_status_custom_emoji_id, emoji_status_expiration_date,"
		" is_verified, is_scam, is_fake, is_premium, is_support,"
		" restriction_reason, has_sensitive_content, restricts_new_chats,"
		" paid_message_star_count"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		") AS new ON DUPLICATE KEY UPDATE"
		" first_name = new.first_name,"
		" last_name = new.last_name,"
		" phone_number = new.phone_number,"
		" type = new.type,"
		" accent_color_id = new.accent_color_id,"
		" background_custom_emoji_id = new.background_custom_emoji_id,"
		" profile_accent_color_id = new.profile_accent_color_id,"
		" profile_background_custom_emoji_id = new.profile_background_custom_emoji_id,"
		" emoji_status_custom_emoji_id = new.emoji_status_custom_emoji_id,"
		" emoji_status_expiration_date = new.emoji_status_expiration_date,"
		" is_verified = new.is_verified,"
		" is_scam = new.is_scam,"
		" is_fake = new.is_fake,"
		" is_premium = new.is_premium,"
		" is_support = new.is_support,"
		" restriction_reason = new.restriction_reason,"
		" has_sensitive_content = new.has_sensitive_content,"
		" restricts_new_chats = new.restricts_new_chats,"
		" paid_message_star_count = new.paid_message_star_count";

	mysql::Param emoji_id = std::monostate{};
	if (u.emoji_status_custom_emoji_id.has_value())
		emoji_id = (int64_t)*u.emoji_status_custom_emoji_id;

	mysql::Param emoji_exp = std::monostate{};
	if (u.emoji_status_expiration_date.has_value())
		emoji_exp = (int64_t)*u.emoji_status_expiration_date;

	db_.transaction([&](mysql::Transaction &tx) {
		/*
		 * Fetch the current name and phone number in one query
		 * so we can detect changes (existing user) or record
		 * first-seen values (new user).
		 */
		auto old = tx.query(
			"SELECT first_name, last_name, phone_number"
			" FROM users WHERE id = ?",
			{ (int64_t)u.id });

		tx.execute(sql, {
			(int64_t)u.id,
			u.first_name,
			u.last_name,
			u.phone_number,
			std::string(user_type_to_string(u.type)),
			(int64_t)u.accent_color_id,
			(int64_t)u.background_custom_emoji_id,
			(int64_t)u.profile_accent_color_id,
			(int64_t)u.profile_background_custom_emoji_id,
			emoji_id,
			emoji_exp,
			b(u.is_verified),
			b(u.is_scam),
			b(u.is_fake),
			b(u.is_premium),
			b(u.is_support),
			u.restriction_reason,
			b(u.has_sensitive_content),
			b(u.restricts_new_chats),
			(int64_t)u.paid_message_star_count,
		});

		if (old.empty()) {
			/*
			 * First time seeing this user - record initial
			 * values. Runs after the UPSERT so the FK exists.
			 */
			tx.insert("INSERT INTO user_hist_name"
				  " (user_id, first_name, last_name)"
				  " VALUES (?, ?, ?)",
				  { (int64_t)u.id, u.first_name,
				    u.last_name });
			tx.insert("INSERT INTO user_hist_phone_num"
				  " (user_id, phone_number)"
				  " VALUES (?, ?)",
				  { (int64_t)u.id, u.phone_number });
		} else {
			auto &r = old[0];
			std::string of = r[0].value_or("");
			std::string ol = r[1].value_or("");
			if (of != u.first_name || ol != u.last_name) {
				tx.insert("INSERT INTO user_hist_name"
					  " (user_id, first_name,"
					  " last_name)"
					  " VALUES (?, ?, ?)",
					  { (int64_t)u.id, of, ol });
			}

			std::string op = r[2].value_or("");
			if (op != u.phone_number) {
				tx.insert("INSERT INTO user_hist_phone_num"
					  " (user_id, phone_number)"
					  " VALUES (?, ?)",
					  { (int64_t)u.id, op });
			}
		}

		syncUsernames(tx, u);
	});
}

void DB::setUserProfilePhoto(int64_t user_id, uint64_t file_id)
{
	db_.transaction([&](mysql::Transaction &tx) {
		trackProfilePhotoChange(tx, user_id, file_id);
		tx.execute("UPDATE users SET profile_photo_file_id = ?"
			   " WHERE id = ?",
			   { (int64_t)file_id, (int64_t)user_id });
	});
}

void DB::syncUsernames(mysql::Transaction &tx, const models::User &u)
{
	/*
	 * Release old usernames by nullifying user_id rather than
	 * deleting them, so the table preserves a history of username
	 * ownership.
	 */
	tx.execute("UPDATE user_usernames SET user_id = NULL WHERE user_id = ?",
		   { (int64_t)u.id });

	static const char *ins =
		"INSERT INTO user_usernames (user_id, username, kind, position)"
		" VALUES (?, ?, ?, ?)";

	auto insert_list = [&](const std::vector<std::string> &names,
			       const char *kind) {
		for (size_t i = 0; i < names.size(); i++) {
			tx.execute(ins, {
				(int64_t)u.id,
				names[i],
				std::string(kind),
				(int64_t)i,
			});
		}
	};

	insert_list(u.active_usernames, "active");
	insert_list(u.disabled_usernames, "disabled");
	insert_list(u.collectible_usernames, "collectible");
}

void DB::trackProfilePhotoChange(mysql::Transaction &tx,
				 int64_t user_id, uint64_t file_id)
{
	auto rows = tx.query(
		"SELECT profile_photo_file_id FROM users WHERE id = ?",
		{ user_id });
	if (rows.empty())
		return;

	auto &val = rows[0][0];
	if (!val.has_value()) {
		/* First profile photo — record it. */
		tx.insert("INSERT INTO user_hist_profile_photo"
			  " (user_id, file_id) VALUES (?, ?)",
			  { user_id, (int64_t)file_id });
		return;
	}

	uint64_t old_id = std::stoull(*val);
	if (old_id == file_id)
		return;

	tx.insert("INSERT INTO user_hist_profile_photo"
		  " (user_id, file_id) VALUES (?, ?)",
		  { user_id, (int64_t)old_id });
}

} /* namespace tgloggerd */

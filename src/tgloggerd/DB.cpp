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

DB::DB(const mysql::Config &cfg)
	: db_(cfg)
{
}

DB::~DB(void) = default;

void DB::ping(void)
{
	db_.query("SELECT 1");
}

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

		syncUsernames(tx, u);
	});
}

uint64_t DB::upsertFile(const models::File &f)
{
	/*
	 * De-duplicate by content: the SHA-256 is stored as BINARY(32), so
	 * bind the hex digest and let the server decode it with UNHEX().
	 * The lookup and the insert/update run in one transaction.
	 */
	uint64_t id = 0;
	db_.transaction([&](mysql::Transaction &tx) {
		auto rows = tx.query(
			"SELECT id FROM files WHERE sha256 = UNHEX(?)",
			{ f.sha256_hex });
		if (!rows.empty() && rows[0][0].has_value()) {
			id = std::stoull(*rows[0][0]);
			tx.execute("UPDATE files SET hit_count = hit_count + 1"
				   " WHERE id = ?", { (int64_t)id });
			return;
		}

		mysql::Param ext = std::monostate{};
		if (f.file_ext.has_value())
			ext = *f.file_ext;

		id = tx.insert(
			"INSERT INTO files (tg_file_id, file_type, file_size,"
			" sha256, file_ext) VALUES (?, ?, ?, UNHEX(?), ?)",
			{
				f.tg_file_id,
				f.file_type,
				(int64_t)f.file_size,
				f.sha256_hex,
				ext,
			});
	});
	return id;
}

void DB::setUserProfilePhoto(int64_t user_id, uint64_t file_id)
{
	db_.execute("UPDATE users SET profile_photo_file_id = ? WHERE id = ?",
		    { (int64_t)file_id, (int64_t)user_id });
}

void DB::syncUsernames(mysql::Transaction &tx, const models::User &u)
{
	tx.execute("DELETE FROM user_usernames WHERE user_id = ?",
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

} /* namespace tgloggerd */

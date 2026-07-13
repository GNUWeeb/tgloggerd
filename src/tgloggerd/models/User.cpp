// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <vector>
#include <utility>
#include <unordered_map>

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
		" paid_message_star_count, is_contact, is_mutual_contact,"
		" is_close_friend, have_access, language_code"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,"
		" ?, ?, ?, ?, ?"
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
		" paid_message_star_count = new.paid_message_star_count,"
		" is_contact = new.is_contact,"
		" is_mutual_contact = new.is_mutual_contact,"
		" is_close_friend = new.is_close_friend,"
		" have_access = new.have_access,"
		" language_code = new.language_code";

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
			b(u.is_contact),
			b(u.is_mutual_contact),
			b(u.is_close_friend),
			b(u.have_access),
			u.language_code,
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
	struct Entry {
		std::string	kind;
		int		position = 0;
	};

	/*
	 * Load the usernames this user currently owns, so the new set can
	 * be diffed against them to record the individual changes.
	 */
	auto old_rows = tx.query(
		"SELECT username, kind, position FROM user_usernames"
		" WHERE user_id = ?",
		{ (int64_t)u.id });

	std::unordered_map<std::string, Entry> old_map;
	for (auto &r : old_rows) {
		if (!r[0].has_value())
			continue;
		old_map.emplace(*r[0], Entry{
			r[1].value_or(""),
			r[2].has_value() ? std::stoi(*r[2]) : 0,
		});
	}

	/* Build the new set from the model, preserving list order. */
	std::vector<std::pair<std::string, Entry>> new_list;
	std::unordered_map<std::string, Entry> new_map;
	auto collect = [&](const std::vector<std::string> &names,
			   const char *kind) {
		for (size_t i = 0; i < names.size(); i++) {
			Entry e{ kind, (int)i };
			new_list.emplace_back(names[i], e);
			new_map[names[i]] = e;
		}
	};
	collect(u.active_usernames, "active");
	collect(u.disabled_usernames, "disabled");
	collect(u.collectible_usernames, "collectible");

	static const char *ev =
		"INSERT INTO user_hist_usernames_events"
		" (user_id, username, action, kind, position)"
		" VALUES (?, ?, ?, ?, ?)";

	/* Additions, kind changes and reorders. */
	for (auto &n : new_list) {
		const std::string &uname = n.first;
		const Entry &ne = n.second;
		auto it = old_map.find(uname);
		if (it == old_map.end()) {
			tx.execute(ev, { (int64_t)u.id, uname,
					 std::string("added"), ne.kind,
					 (int64_t)ne.position });
		} else if (it->second.kind != ne.kind) {
			tx.execute(ev, { (int64_t)u.id, uname,
					 std::string("kind_changed"), ne.kind,
					 (int64_t)ne.position });
		} else if (it->second.position != ne.position) {
			tx.execute(ev, { (int64_t)u.id, uname,
					 std::string("reordered"), ne.kind,
					 (int64_t)ne.position });
		}
	}

	/* Removals: usernames the user no longer owns are released. */
	for (auto &o : old_map) {
		if (new_map.find(o.first) != new_map.end())
			continue;
		tx.execute(ev, { (int64_t)u.id, o.first,
				 std::string("removed"), std::monostate{},
				 std::monostate{} });
		tx.execute("UPDATE user_usernames SET user_id = NULL"
			   " WHERE user_id = ? AND username = ?",
			   { (int64_t)u.id, o.first });
	}

	/*
	 * Upsert the current usernames. The UNIQUE key on username lets a
	 * single statement claim a new username, transfer ownership of an
	 * existing one, and update its kind and position.
	 */
	static const char *ins =
		"INSERT INTO user_usernames (user_id, username, kind, position)"
		" VALUES (?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
		" user_id = new.user_id, kind = new.kind,"
		" position = new.position";
	for (auto &n : new_list) {
		tx.execute(ins, { (int64_t)u.id, n.first, n.second.kind,
				  (int64_t)n.second.position });
	}
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

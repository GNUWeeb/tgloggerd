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

const char *group_type_to_string(models::GroupType t)
{
	switch (t) {
	case models::GroupType::BasicGroup:	return "basic_group";
	case models::GroupType::Supergroup:	return "supergroup";
	case models::GroupType::Channel:	return "channel";
	}
	return "basic_group";
}

} /* namespace */

void DB::upsertGroup(const models::Group &g)
{
	/*
	 * Note: photo_file_id is intentionally omitted; the photo reference
	 * is managed by setGroupPhoto once the photo has been downloaded.
	 */
	static const char *sql =
		"INSERT INTO `groups` (id, type, title, description)"
		" VALUES (?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
		" type = new.type, title = new.title,"
		" description = new.description";

	db_.transaction([&](mysql::Transaction &tx) {
		auto old = tx.query(
			"SELECT title, description FROM `groups` WHERE id = ?",
			{ (int64_t)g.id });

		tx.execute(sql, {
			(int64_t)g.id,
			std::string(group_type_to_string(g.type)),
			g.title,
			g.description,
		});

		if (old.empty()) {
			/* First time seeing this group; record initial values. */
			tx.insert("INSERT INTO group_hist_title"
				  " (group_id, title) VALUES (?, ?)",
				  { (int64_t)g.id, g.title });
			tx.insert("INSERT INTO group_hist_description"
				  " (group_id, description) VALUES (?, ?)",
				  { (int64_t)g.id, g.description });
		} else {
			std::string ot = old[0][0].value_or("");
			if (ot != g.title) {
				tx.insert("INSERT INTO group_hist_title"
					  " (group_id, title) VALUES (?, ?)",
					  { (int64_t)g.id, ot });
			}

			std::string od = old[0][1].value_or("");
			if (od != g.description) {
				tx.insert("INSERT INTO group_hist_description"
					  " (group_id, description)"
					  " VALUES (?, ?)",
					  { (int64_t)g.id, od });
			}
		}

		syncGroupUsernames(tx, g);
	});
}

void DB::setGroupPhoto(int64_t group_id, uint64_t file_id)
{
	db_.transaction([&](mysql::Transaction &tx) {
		trackGroupPhotoChange(tx, group_id, file_id);
		tx.execute("UPDATE `groups` SET photo_file_id = ?"
			   " WHERE id = ?",
			   { (int64_t)file_id, (int64_t)group_id });
	});
}

void DB::trackGroupPhotoChange(mysql::Transaction &tx,
			       int64_t group_id, uint64_t file_id)
{
	auto rows = tx.query(
		"SELECT photo_file_id FROM `groups` WHERE id = ?",
		{ group_id });
	if (rows.empty())
		return;

	auto &val = rows[0][0];
	if (!val.has_value()) {
		/* First group photo — record it. */
		tx.insert("INSERT INTO group_hist_photo"
			  " (group_id, file_id) VALUES (?, ?)",
			  { group_id, (int64_t)file_id });
		return;
	}

	uint64_t old_id = std::stoull(*val);
	if (old_id == file_id)
		return;

	tx.insert("INSERT INTO group_hist_photo"
		  " (group_id, file_id) VALUES (?, ?)",
		  { group_id, (int64_t)old_id });
}

void DB::syncGroupUsernames(mysql::Transaction &tx, const models::Group &g)
{
	struct Entry {
		std::string	kind;
		int		position = 0;
	};

	/* Usernames the group currently owns, to diff against the new set. */
	auto old_rows = tx.query(
		"SELECT username, kind, position FROM group_usernames"
		" WHERE group_id = ?",
		{ (int64_t)g.id });

	std::unordered_map<std::string, Entry> old_map;
	for (auto &r : old_rows) {
		if (!r[0].has_value())
			continue;
		old_map.emplace(*r[0], Entry{
			r[1].value_or(""),
			r[2].has_value() ? std::stoi(*r[2]) : 0,
		});
	}

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
	collect(g.active_usernames, "active");
	collect(g.disabled_usernames, "disabled");
	collect(g.collectible_usernames, "collectible");

	static const char *ev =
		"INSERT INTO group_hist_usernames_events"
		" (group_id, username, action, kind, position)"
		" VALUES (?, ?, ?, ?, ?)";

	for (auto &n : new_list) {
		const std::string &uname = n.first;
		const Entry &ne = n.second;
		auto it = old_map.find(uname);
		if (it == old_map.end()) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("added"), ne.kind,
					 (int64_t)ne.position });
		} else if (it->second.kind != ne.kind) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("kind_changed"), ne.kind,
					 (int64_t)ne.position });
		} else if (it->second.position != ne.position) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("reordered"), ne.kind,
					 (int64_t)ne.position });
		}
	}

	for (auto &o : old_map) {
		if (new_map.find(o.first) != new_map.end())
			continue;
		tx.execute(ev, { (int64_t)g.id, o.first,
				 std::string("removed"), std::monostate{},
				 std::monostate{} });
		tx.execute("UPDATE group_usernames SET group_id = NULL"
			   " WHERE group_id = ? AND username = ?",
			   { (int64_t)g.id, o.first });
	}

	static const char *ins =
		"INSERT INTO group_usernames (group_id, username, kind, position)"
		" VALUES (?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
		" group_id = new.group_id, kind = new.kind,"
		" position = new.position";
	for (auto &n : new_list) {
		tx.execute(ins, { (int64_t)g.id, n.first, n.second.kind,
				  (int64_t)n.second.position });
	}
}

} /* namespace tgloggerd */

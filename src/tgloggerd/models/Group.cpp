// SPDX-License-Identifier: GPL-2.0-or-later
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
		"INSERT INTO `telegram_groups` (id, type, title, description)"
		" VALUES (?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
		" type = new.type, title = new.title,"
		" description = new.description";

	db_.transaction([&](mysql::Transaction &tx) {
		auto old = tx.query(
			"SELECT title, description FROM `telegram_groups` WHERE id = ?",
			{ (int64_t)g.id });

		tx.execute(sql, {
			(int64_t)g.id,
			std::string(group_type_to_string(g.type)),
			g.title,
			g.description,
		});

		if (old.empty()) {
			/* First time seeing this group; record the initial
			 * title. The description is handled below: it is empty
			 * here (it only arrives later with full info), so it
			 * must not be snapshotted as a blank initial value. */
			tx.insert("INSERT INTO telegram_group_hist_title"
				  " (group_id, title) VALUES (?, ?)",
				  { (int64_t)g.id, g.title });
		} else {
			std::string ot = old[0][0].value_or("");
			if (ot != g.title) {
				tx.insert("INSERT INTO telegram_group_hist_title"
					  " (group_id, title) VALUES (?, ?)",
					  { (int64_t)g.id, ot });
			}
		}

		/*
		 * Snapshot the description as observed (skipping empty and
		 * unchanged values), so the first real description -- which
		 * arrives with full info after the row already exists -- is
		 * captured instead of a run of blank rows.
		 */
		recordTextHistory(tx, "telegram_group_hist_description", "group_id",
				  "description", g.id, g.description);

		syncGroupUsernames(tx, g);
	});
}

void DB::setGroupPhoto(int64_t group_id, uint64_t file_id)
{
	db_.transaction([&](mysql::Transaction &tx) {
		trackGroupPhotoChange(tx, group_id, file_id);
		tx.execute("UPDATE `telegram_groups` SET photo_file_id = ?"
			   " WHERE id = ?",
			   { (int64_t)file_id, (int64_t)group_id });
	});
}

void DB::trackGroupPhotoChange(mysql::Transaction &tx,
			       int64_t group_id, uint64_t file_id)
{
	auto rows = tx.query(
		"SELECT photo_file_id FROM `telegram_groups` WHERE id = ?",
		{ group_id });
	if (rows.empty())
		return;

	/* As for users (DB::trackProfilePhotoChange): every photo that is
	 * or was the group's has a history row. */
	auto &val = rows[0][0];
	if (val.has_value()) {
		uint64_t old_id = std::stoull(*val);
		if (old_id != file_id)
			ensurePhotoHistory(tx, true, group_id, old_id, 0);
	}
	ensurePhotoHistory(tx, true, group_id, file_id, 0);
}

void DB::syncGroupUsernames(mysql::Transaction &tx, const models::Group &g)
{
	struct Entry {
		std::string	kind;         /* "active" or "disabled". */
		int		position = 0;
		bool		collectible = false;
	};

	/* Usernames the group currently owns, to diff against the new set. */
	auto old_rows = tx.query(
		"SELECT username, kind, position, is_collectible FROM telegram_group_usernames"
		" WHERE group_id = ?",
		{ (int64_t)g.id });

	std::unordered_map<std::string, Entry> old_map;
	for (auto &r : old_rows) {
		if (!r[0].has_value())
			continue;
		old_map.emplace(*r[0], Entry{
			r[1].value_or(""),
			r[2].has_value() ? std::stoi(*r[2]) : 0,
			r[3].has_value() && *r[3] != "0",
		});
	}

	/*
	 * One entry per username: kind is active XOR disabled, collectible is an
	 * orthogonal flag (see the user path in User.cpp for why a collectible
	 * "kind" made the diff flap). A collectible-only username is disabled.
	 */
	std::vector<std::pair<std::string, Entry>> new_list;
	std::unordered_map<std::string, size_t> idx;
	auto add = [&](const std::string &name, const char *kind, int pos) {
		idx[name] = new_list.size();
		new_list.push_back({ name, Entry{ kind, pos, false } });
	};
	for (size_t i = 0; i < g.active_usernames.size(); i++)
		add(g.active_usernames[i], "active", (int)i);
	for (size_t i = 0; i < g.disabled_usernames.size(); i++)
		add(g.disabled_usernames[i], "disabled", (int)i);
	for (size_t i = 0; i < g.collectible_usernames.size(); i++) {
		const std::string &name = g.collectible_usernames[i];
		auto it = idx.find(name);
		if (it != idx.end())
			new_list[it->second].second.collectible = true;
		else {
			add(name, "disabled", (int)i);
			new_list.back().second.collectible = true;
		}
	}

	static const char *ev =
		"INSERT INTO telegram_group_hist_usernames_events"
		" (group_id, username, action, kind, position, is_collectible)"
		" VALUES (?, ?, ?, ?, ?, ?)";

	for (auto &n : new_list) {
		const std::string &uname = n.first;
		const Entry &ne = n.second;
		int64_t coll = ne.collectible ? 1 : 0;
		auto it = old_map.find(uname);
		if (it == old_map.end()) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("added"), ne.kind,
					 (int64_t)ne.position, coll });
		} else if (it->second.kind != ne.kind) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("kind_changed"), ne.kind,
					 (int64_t)ne.position, coll });
		} else if (it->second.collectible != ne.collectible) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("collectible_changed"), ne.kind,
					 (int64_t)ne.position, coll });
		} else if (it->second.position != ne.position) {
			tx.execute(ev, { (int64_t)g.id, uname,
					 std::string("reordered"), ne.kind,
					 (int64_t)ne.position, coll });
		}
	}

	for (auto &o : old_map) {
		if (idx.find(o.first) != idx.end())
			continue;
		tx.execute(ev, { (int64_t)g.id, o.first,
				 std::string("removed"), std::monostate{},
				 std::monostate{}, std::monostate{} });
		tx.execute("UPDATE telegram_group_usernames SET group_id = NULL"
			   " WHERE group_id = ? AND username = ?",
			   { (int64_t)g.id, o.first });
	}

	static const char *ins =
		"INSERT INTO telegram_group_usernames"
		" (group_id, username, kind, position, is_collectible)"
		" VALUES (?, ?, ?, ?, ?) AS new ON DUPLICATE KEY UPDATE"
		" group_id = new.group_id, kind = new.kind,"
		" position = new.position, is_collectible = new.is_collectible";
	for (auto &n : new_list) {
		tx.execute(ins, { (int64_t)g.id, n.first, n.second.kind,
				  (int64_t)n.second.position,
				  (int64_t)(n.second.collectible ? 1 : 0) });
	}
}

void DB::setGroupMemberInfo(int64_t group_id,
			    std::optional<int64_t> member_count,
			    std::optional<bool> can_get_members)
{
	if (!member_count && !can_get_members)
		return;
	mysql::Param count = std::monostate{};
	if (member_count)
		count = *member_count;
	mysql::Param can = std::monostate{};
	if (can_get_members)
		can = (int64_t)(*can_get_members ? 1 : 0);
	/*
	 * updated_at is kept as it was: the row's own fields did not change,
	 * and its ON UPDATE would otherwise move with every count.
	 */
	db_.execute("UPDATE telegram_groups SET"
		    " member_count_at = IF(? IS NULL, member_count_at, NOW()),"
		    " member_count = COALESCE(?, member_count),"
		    " can_get_members = COALESCE(?, can_get_members),"
		    " updated_at = updated_at"
		    " WHERE id = ?",
		    { count, count, can, group_id });
}

std::optional<DB::GroupMemberInfo> DB::getGroupMemberInfo(int64_t group_id)
{
	auto rows = db_.query("SELECT type, member_count,"
			      " IFNULL(UNIX_TIMESTAMP(member_count_at), 0),"
			      " can_get_members"
			      " FROM telegram_groups WHERE id = ?",
			      { group_id });
	if (rows.empty())
		return std::nullopt;
	const auto &r = rows[0];
	GroupMemberInfo g;
	g.type = r[0].value_or("");
	if (r[1].has_value())
		g.member_count = std::stoll(*r[1]);
	g.member_count_at = r[2].has_value() ? std::stoll(*r[2]) : 0;
	if (r[3].has_value())
		g.can_get_members = *r[3] == "1";
	return g;
}

} /* namespace tgloggerd */

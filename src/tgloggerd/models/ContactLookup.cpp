// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <vector>

namespace tgloggerd {

/*
 * telegram_contact_lookups, the journal of the "Look up real name" cycle
 * (ContactLookup.hpp), and the result it stores on telegram_users.
 */

void DB::insertContactLookup(const ContactLookupRow &c)
{
	mysql::Param note = std::monostate{};
	if (c.note_json)
		note = *c.note_json;
	/*
	 * A plain INSERT: a row already there is a contact not added back
	 * yet, and must never be overwritten by a later cycle's view of it.
	 */
	db_.execute(
		"INSERT INTO telegram_contact_lookups (user_id, phone_number,"
		" first_name, last_name, note, share_phone_number)"
		" VALUES (?, ?, ?, ?, ?, ?)",
		{ c.user_id, c.phone_number, c.first_name, c.last_name, note,
		  (int64_t)(c.share_phone_number ? 1 : 0) });
}

void DB::markContactLookupRemoved(int64_t user_id)
{
	db_.execute("UPDATE telegram_contact_lookups SET removed_at = NOW()"
		    " WHERE user_id = ?", { user_id });
}

void DB::noteContactLookupFailure(int64_t user_id, const std::string &err)
{
	db_.execute("UPDATE telegram_contact_lookups SET attempts = attempts + 1,"
		    " last_error = LEFT(?, 255) WHERE user_id = ?",
		    { err, user_id });
}

void DB::deleteContactLookup(int64_t user_id)
{
	db_.execute("DELETE FROM telegram_contact_lookups WHERE user_id = ?",
		    { user_id });
}

std::vector<DB::ContactLookupRow> DB::listContactLookups(void)
{
	auto rows = db_.query(
		"SELECT user_id, phone_number, first_name, last_name, note,"
		" share_phone_number FROM telegram_contact_lookups"
		" ORDER BY created_at, user_id", {});
	std::vector<ContactLookupRow> out;
	out.reserve(rows.size());
	for (auto &r : rows) {
		ContactLookupRow c;
		c.user_id = std::stoll(r[0].value_or("0"));
		c.phone_number = r[1].value_or("");
		c.first_name = r[2].value_or("");
		c.last_name = r[3].value_or("");
		c.note_json = r[4];
		c.share_phone_number = r[5].value_or("0") != "0";
		out.push_back(std::move(c));
	}
	return out;
}

void DB::recordOwnNameLookup(int64_t user_id, const models::Name &own)
{
	db_.transaction([&](mysql::Transaction &tx) {
		auto old = tx.query(
			"SELECT first_name, last_name, own_name_known"
			" FROM telegram_users WHERE id = ? FOR UPDATE",
			{ user_id });
		if (old.empty())
			return;

		/*
		 * Normally a no-op on the name: the removal made TDLib report
		 * the own name, and that report was stored first, through the
		 * same serial queue. The history rule is upsertUser's.
		 */
		models::Name stored{ old[0][0].value_or(""),
				     old[0][1].value_or("") };
		bool known = old[0][2].value_or("1") != "0";
		if (!known || stored != own) {
			const models::Name &h = known ? stored : own;
			tx.insert("INSERT INTO telegram_user_hist_name"
				  " (user_id, first_name, last_name)"
				  " VALUES (?, ?, ?)",
				  { user_id, h.first, h.second });
		}
		tx.execute("UPDATE telegram_users SET first_name = ?,"
			   " last_name = ?, own_name_known = 1,"
			   " own_name_checked_at = NOW() WHERE id = ?",
			   { own.first, own.second, user_id });
	});
}

} /* namespace tgloggerd */

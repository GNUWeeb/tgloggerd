// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * chat_recheck_db_test -- what a chat recheck reads and writes in the
 * database (DB::listRecheckStored, DB::markDeletedOnRecheck,
 * DB::chatKnown) and the plans of its reads, against a MySQL with every
 * migration applied. It writes rows under ids no real chat has and
 * deletes them after; run it only against a throwaway database:
 *
 *   TGL_TEST_MYSQL_HOST=127.0.0.1 TGL_TEST_MYSQL_PORT=3306
 *   TGL_TEST_MYSQL_USER=root TGL_TEST_MYSQL_PASSWORD=...
 *   TGL_TEST_MYSQL_DB=tgloggerd build/chat_recheck_db_test
 *
 * Without TGL_TEST_MYSQL_HOST it does nothing and exits 77 (skipped).
 */
#include <tgloggerd/DB.hpp>

#include <cstdio>
#include <cstdlib>
#include <string>
#include <vector>

using namespace tgloggerd;

static int g_failures;

#define CHECK(cond, what)                                                     \
	do {                                                                  \
		if (!(cond)) {                                                \
			printf("  FAIL: %s\n", (what));                       \
			g_failures++;                                         \
		} else {                                                      \
			printf("  ok:   %s\n", (what));                       \
		}                                                             \
	} while (0)

/* Ids no real chat has: a basic group's chat id is minus its id, and no
 * real user id is this small. */
static const int64_t kGroup = -4242003;
static const int64_t kUser = 4242;
static const int64_t kSuper = -1004242000001;

static std::string env(const char *k, const char *def = "")
{
	const char *v = std::getenv(k);
	return v ? v : def;
}

/* The single value of a one-row, one-column query; "NULL" for NULL. */
static std::string one(mysql::Database &db, const std::string &sql,
		       const std::vector<mysql::Param> &args = {})
{
	auto rows = db.query(sql, args);
	if (rows.empty() || rows[0].empty())
		return "<none>";
	return rows[0][0].value_or("NULL");
}

static void cleanup(mysql::Database &raw)
{
	raw.execute("DELETE FROM telegram_group_messages WHERE chat_id = ?", { kGroup });
	raw.execute("DELETE FROM telegram_group_messages WHERE chat_id = ?", { kSuper });
	raw.execute("DELETE FROM `telegram_groups` WHERE id = ?", { kSuper });
	raw.execute("DELETE FROM telegram_private_messages WHERE chat_id = ?", { kUser });
	raw.execute("DELETE FROM `telegram_groups` WHERE id = ?", { kGroup });
	raw.execute("DELETE FROM telegram_users WHERE id = ?", { kUser });
}

static void groupMessage(DB &db, int64_t id, const char *meta = nullptr,
			 bool deleted = false)
{
	models::GroupMessage m;
	m.chat_id = kGroup;
	m.message_id = id;
	m.date = 1700000000 + id;
	m.content.content_type = models::MessageContentType::Text;
	m.content.text = "message " + std::to_string(id);
	if (meta)
		m.meta = meta;
	db.upsertGroupMessage(m);
	if (deleted) {
		models::GroupMessage d;
		d.chat_id = kGroup;
		d.message_id = id;
		d.is_deleted = true;
		db.upsertGroupMessage(d);
	}
}

static void privateMessage(DB &db, int64_t id, const char *meta = nullptr)
{
	models::PrivateMessage m;
	m.chat_id = kUser;
	m.message_id = id;
	m.date = 1700000000 + id;
	m.content.content_type = models::MessageContentType::Text;
	m.content.text = "message " + std::to_string(id);
	if (meta)
		m.meta = meta;
	db.upsertPrivateMessage(m);
}

static std::string ids(const std::vector<int64_t> &v)
{
	std::string s;
	for (int64_t x : v)
		s += (s.empty() ? "" : ",") + std::to_string(x);
	return s;
}

/*
 * The key and access type EXPLAIN reports for a statement, as
 * "key/type". Values are written into the text: EXPLAIN is not taken
 * as a prepared statement with parameters.
 */
static std::string plan(mysql::Database &raw, const std::string &sql)
{
	auto rows = raw.query("EXPLAIN FORMAT=TRADITIONAL " + sql);
	if (rows.empty() || rows[0].size() < 7)
		return "<none>";
	/* id, select_type, table, partitions, type, possible_keys, key */
	return rows[0][6].value_or("NULL") + "/" + rows[0][4].value_or("NULL");
}

int main()
{
	if (env("TGL_TEST_MYSQL_HOST").empty()) {
		printf("TGL_TEST_MYSQL_HOST unset: skipped\n");
		return 77;
	}
	mysql::Config cfg;
	cfg.host = env("TGL_TEST_MYSQL_HOST");
	cfg.port = (uint16_t)std::atoi(env("TGL_TEST_MYSQL_PORT", "3306").c_str());
	cfg.user = env("TGL_TEST_MYSQL_USER", "root");
	cfg.password = env("TGL_TEST_MYSQL_PASSWORD");
	cfg.database = env("TGL_TEST_MYSQL_DB", "tgloggerd");
	cfg.pool_size = 2;

	DB db(cfg);
	mysql::Database raw(cfg);
	cleanup(raw);

	printf("chat known:\n");
	CHECK(!db.chatKnown(kGroup), "a group not stored is not known");
	CHECK(!db.chatKnown(kUser), "a private chat not stored is not known");
	models::Group g;
	g.id = kGroup;
	g.title = "recheck test";
	db.upsertGroup(g);
	models::User u;
	u.id = kUser;
	u.first_name = "recheck test";
	db.upsertUser(u);
	CHECK(db.chatKnown(kGroup), "a stored group is known");
	CHECK(db.chatKnown(kUser), "a stored user's private chat is known");

	printf("stored around a span:\n");
	for (int64_t id = 10; id <= 20; id++)
		groupMessage(db, id, id == 12 ? R"({"fact_check": {"text": "x"}})"
					      : nullptr, id == 15);
	recheck::Stored s = db.listRecheckStored(kGroup, true, { 10, 20 });
	CHECK(ids(s.between) == "11,12,13,14,16,17,18,19",
	      "live ones strictly between, the deleted one left out");
	CHECK(!s.older_exists, "nothing below");
	groupMessage(db, 4, nullptr, true);
	s = db.listRecheckStored(kGroup, true, { 10, 20 });
	CHECK(!s.older_exists, "a deleted older one does not count");
	groupMessage(db, 5);
	groupMessage(db, 25);
	s = db.listRecheckStored(kGroup, true, { 10, 20 });
	CHECK(s.older_exists, "a live older one");
	CHECK(ids(s.between) == "11,12,13,14,16,17,18,19",
	      "ones beyond the span are not between");
	s = db.listRecheckStored(kGroup, false, { 10, 20 });
	CHECK(s.between.empty() && !s.older_exists,
	      "the private table is apart");

	printf("mark deleted on recheck:\n");
	auto marked = db.markDeletedOnRecheck(kGroup, true, { 12, 13, 15, 99 });
	CHECK(ids(marked) == "12,13",
	      "marks the live ones; deleted and unknown ones left alone");
	CHECK(one(raw, "SELECT deleted_at IS NOT NULL FROM telegram_group_messages"
		  " WHERE chat_id = ? AND message_id = 13", { kGroup }) == "1",
	      "deleted_at stamped");
	CHECK(one(raw, "SELECT meta FROM telegram_group_messages"
		  " WHERE chat_id = ? AND message_id = 13", { kGroup }) ==
	      R"({"deleted_on_recheck": true})",
	      "a NULL meta becomes an object with the flag");
	CHECK(one(raw, "SELECT JSON_TYPE(JSON_EXTRACT(meta, '$.deleted_on_recheck'))"
		  " FROM telegram_group_messages WHERE chat_id = ? AND message_id = 13",
		  { kGroup }) == "BOOLEAN",
	      "the flag is JSON true, not 1");
	CHECK(one(raw, "SELECT meta FROM telegram_group_messages"
		  " WHERE chat_id = ? AND message_id = 12", { kGroup }) ==
	      R"({"fact_check": {"text": "x"}, "deleted_on_recheck": true})",
	      "an existing meta keeps its keys and gains the flag");
	CHECK(one(raw, "SELECT CONCAT(deleted_at IS NOT NULL, IFNULL(meta, 'NULL'))"
		  " FROM telegram_group_messages WHERE chat_id = ? AND message_id = 15",
		  { kGroup }) == "1NULL",
	      "a live deletion is not flagged");
	CHECK(db.markDeletedOnRecheck(kGroup, true, { 12, 13 }).empty(),
	      "marking again changes nothing");
	CHECK(db.markDeletedOnRecheck(kGroup, true, {}).empty(), "no ids: nothing");

	printf("count what is stored, for the paging:\n");
	CHECK(db.countStoredMessages(kGroup, true, { 12, 13, 15, 99, 1000 }) == 3,
	      "stored ones counted, deleted ones too; unknown ones not");
	CHECK(db.countStoredMessages(kGroup, true, {}) == 0, "no ids: 0");
	CHECK(db.countStoredMessages(kGroup, false, { 12 }) == 0,
	      "the private table is apart");
	{
		auto one = db.listRecheckStored(kGroup, true, { 14, 14 });
		CHECK(one.between.empty() && one.older_exists,
		      "a one-id span: nothing between, an older one stored");
		auto first = db.listRecheckStored(kGroup, true, { 1, 1 });
		CHECK(!first.older_exists, "nothing older than the first");
	}

	printf("stored near the newest, for the catch-up:\n");
	{
		models::Group sgr;
		sgr.id = kSuper;
		sgr.title = "recheck supergroup";
		db.upsertGroup(sgr);
		/* Ids 101..150 with only every fifth stored, plus 150. */
		for (int64_t id = 105; id <= 150; id += 5) {
			models::GroupMessage m;
			m.chat_id = kSuper;
			m.message_id = id;
			m.date = 1700000000 + id;
			m.content.content_type = models::MessageContentType::Text;
			m.content.text = "m";
			db.upsertGroupMessage(m);
		}
		auto newest = db.newestStoredMessageIds({ kSuper, kGroup });
		CHECK(newest.count(kSuper) && newest[kSuper] == 150,
		      "the newest stored id");
		auto near = db.storedNearNewest(newest);
		CHECK(near.count(kSuper) && near[kSuper] == 10,
		      "10 of the 50 ids up to it are stored");
		CHECK(!near.count(kGroup), "a basic group is not counted");
	}

	printf("restore a recheck deletion:\n");
	auto back = db.restoreRecheckDeleted(kGroup, true, { 12, 15, 16, 99 });
	CHECK(ids(back) == "12",
	      "only the recheck-flagged one comes back");
	CHECK(one(raw, "SELECT CONCAT(deleted_at IS NULL, meta)"
		  " FROM telegram_group_messages WHERE chat_id = ? AND message_id = 12",
		  { kGroup }) == R"(1{"fact_check": {"text": "x"}})",
	      "deleted_at cleared, the flag removed, other keys kept");
	CHECK(one(raw, "SELECT deleted_at IS NOT NULL FROM telegram_group_messages"
		  " WHERE chat_id = ? AND message_id = 15", { kGroup }) == "1",
	      "a live deletion stays deleted");
	CHECK(db.restoreRecheckDeleted(kGroup, true, { 12 }).empty(),
	      "restoring again changes nothing");
	CHECK(ids(db.markDeletedOnRecheck(kGroup, true, { 12 })) == "12",
	      "it can be marked again");
	s = db.listRecheckStored(kGroup, true, { 10, 20 });
	CHECK(ids(s.between) == "11,14,16,17,18,19", "marked ones are no longer live");

	privateMessage(db, 7);
	privateMessage(db, 8, R"({"a": 1})");
	privateMessage(db, 9);
	marked = db.markDeletedOnRecheck(kUser, false, { 8 });
	CHECK(ids(marked) == "8", "private: marked");
	CHECK(one(raw, "SELECT meta FROM telegram_private_messages"
		  " WHERE chat_id = ? AND message_id = 8", { kUser }) ==
	      R"({"a": 1, "deleted_on_recheck": true})",
	      "private: flag beside the existing meta");
	CHECK(one(raw, "SELECT COUNT(*) FROM telegram_group_messages"
		  " WHERE chat_id = ? AND message_id = 8", { kGroup }) == "0",
	      "private: the group table untouched");

	printf("plans:\n");
	/* The reads as DB::listRecheckStored words them. */
	for (const char *t : { "telegram_group_messages", "telegram_private_messages" }) {
		const std::string tt = t;
		const std::string ft = tt + " FORCE INDEX (uq_" +
			tt.substr(sizeof("telegram_") - 1) + "_chat_msg)";
		const std::string c = std::to_string(kGroup);
		std::string p1 = plan(raw, "SELECT message_id FROM " + ft +
			" WHERE chat_id = " + c + " AND message_id > 10"
			" AND message_id < 20 AND deleted_at IS NULL");
		std::string p2 = plan(raw, "SELECT 1 FROM " + ft +
			" WHERE chat_id = " + c + " AND message_id < 10"
			" AND deleted_at IS NULL LIMIT 1");
		std::string p4 = plan(raw, "UPDATE " + tt + " SET deleted_at = NOW()"
			" WHERE chat_id = " + c + " AND message_id = 12"
			" AND deleted_at IS NULL");
		printf("  %s: between %s, older %s, mark %s\n", t,
		       p1.c_str(), p2.c_str(), p4.c_str());
		auto on_key = [](const std::string &p) {
			return p.find("chat_msg/") != std::string::npos &&
			       (p.find("/range") != std::string::npos ||
				p.find("/const") != std::string::npos ||
				p.find("/ref") != std::string::npos);
		};
		CHECK(on_key(p1) && on_key(p2) && on_key(p4),
		      (tt + ": every read and the mark use (chat_id, message_id)").c_str());
	}

	cleanup(raw);
	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

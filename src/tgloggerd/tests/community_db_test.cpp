// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * community_db_test -- the community upserts and the replacement of a
 * community's chat list, against a MySQL with every migration applied.
 * It writes rows under ids no real community has and deletes them
 * after; run it only against a throwaway database:
 *
 *   TGL_TEST_MYSQL_HOST=127.0.0.1 TGL_TEST_MYSQL_PORT=3306
 *   TGL_TEST_MYSQL_USER=root TGL_TEST_MYSQL_PASSWORD=...
 *   TGL_TEST_MYSQL_DB=tgloggerd build/community_db_test
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

static const int64_t kId = -4242001;	/* no real community id is negative */

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

static std::string chats(mysql::Database &db)
{
	auto rows = db.query(
		"SELECT chat_id, is_hidden, can_view_history"
		" FROM telegram_community_chats WHERE community_id = ?"
		" ORDER BY chat_id", { kId });
	std::string s;
	for (const auto &r : rows)
		s += (s.empty() ? "" : " ") + r[0].value_or("?") + ":" +
		     r[1].value_or("?") + r[2].value_or("?");
	return s;
}

static models::File photo(const std::string &tag)
{
	models::File f;
	f.tg_file_id = "community-db-test-" + tag;
	f.tg_unique_id = "community-db-test-u-" + tag;
	f.file_type = "photo";
	f.file_ext = "jpg";
	f.on_disk = false;
	return f;
}

static void cleanup(mysql::Database &raw)
{
	raw.execute("DELETE FROM telegram_communities WHERE id = ?", { kId });
	raw.execute("DELETE FROM telegram_files WHERE tg_file_id LIKE"
		    " 'community-db-test-%'", {});
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

	printf("community row:\n");
	CHECK(db.communityNeedsFullInfo(kId), "a missing row needs full info");

	uint64_t small = db.upsertFileMeta(photo("small")).id;
	models::Community c;
	c.id = kId;
	c.name = "First name";
	c.have_access = true;
	c.date = 1700000000;
	db.upsertCommunity(c, small);
	CHECK(one(raw, "SELECT CONCAT_WS('|', name, have_access, photo_file_id,"
		  " IFNULL(administrator_count, 'NULL'), date)"
		  " FROM telegram_communities WHERE id = ?", { kId }) ==
	      "First name|1|" + std::to_string(small) + "|NULL|1700000000",
	      "inserted with name, access, photo and date");
	CHECK(db.communityNeedsFullInfo(kId),
	      "a row without chats still needs full info");

	printf("full info:\n");
	uint64_t big = db.upsertFileMeta(photo("big")).id;
	models::CommunityFullInfo fi;
	fi.community_id = kId;
	fi.administrator_count = 3;
	fi.chats = { { -1001, false, true }, { -1002, true, false } };
	db.putCommunityFullInfo(fi, big);
	CHECK(one(raw, "SELECT CONCAT_WS('|', name, photo_file_id,"
		  " administrator_count) FROM telegram_communities WHERE id = ?",
		  { kId }) == "First name|" + std::to_string(big) + "|3",
	      "big photo and administrator count, name kept");
	CHECK(chats(raw) == "-1002:10 -1001:01", "both chats stored");
	CHECK(!db.communityNeedsFullInfo(kId),
	      "a row with chats needs no full info");

	fi.administrator_count.reset();
	fi.chats = { { -1002, false, true }, { -1003, false, false } };
	db.putCommunityFullInfo(fi, big);
	CHECK(chats(raw) == "-1003:00 -1002:01",
	      "list replaced: -1001 dropped, -1002 updated, -1003 added");
	CHECK(one(raw, "SELECT IFNULL(administrator_count, 'NULL')"
		  " FROM telegram_communities WHERE id = ?", { kId }) == "NULL",
	      "administrator count cleared when not reported");

	fi.chats.clear();
	db.putCommunityFullInfo(fi, 0);
	CHECK(chats(raw) == "-1003:00 -1002:01",
	      "an empty list keeps the chats stored");
	CHECK(one(raw, "SELECT IFNULL(photo_file_id, 'NULL')"
		  " FROM telegram_communities WHERE id = ?", { kId }) == "NULL",
	      "a full info without a photo clears it");

	printf("update after full info:\n");
	fi.chats = { { -1004, false, true } };
	db.putCommunityFullInfo(fi, big);
	c.name = "Renamed";
	db.upsertCommunity(c, big);
	CHECK(one(raw, "SELECT name FROM telegram_communities WHERE id = ?",
		  { kId }) == "Renamed", "renamed");
	CHECK(chats(raw) == "-1004:01", "an update keeps the chats");

	printf("file sources:\n");
	{
		auto has = [&](void) {
			for (const auto &src : db.listFileSources(big, 20))
				if (src.kind == models::FileSource::Kind::Community &&
				    src.object_id == kId &&
				    src.unique_id == "community-db-test-u-big")
					return true;
			return false;
		};
		CHECK(has(), "the photo's file lists the community as a source");
		c.have_access = false;
		db.upsertCommunity(c, big);
		CHECK(!has(), "an inaccessible community is no source");
		c.have_access = true;
		db.upsertCommunity(c, big);
	}

	printf("cascades:\n");
	raw.execute("DELETE FROM telegram_files WHERE id = ?", { big });
	CHECK(one(raw, "SELECT IFNULL(photo_file_id, 'NULL')"
		  " FROM telegram_communities WHERE id = ?", { kId }) == "NULL",
	      "deleting the photo's file row clears photo_file_id");
	raw.execute("DELETE FROM telegram_communities WHERE id = ?", { kId });
	CHECK(chats(raw).empty(), "deleting the community drops its chats");

	printf("nameless row:\n");
	fi.chats = { { -1005, false, true } };
	db.putCommunityFullInfo(fi, 0);
	CHECK(one(raw, "SELECT CONCAT('[', name, ']') FROM telegram_communities"
		  " WHERE id = ?", { kId }) == "[]",
	      "full info before the community makes a nameless row");
	CHECK(chats(raw) == "-1005:01", "and stores its chats");

	cleanup(raw);
	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}

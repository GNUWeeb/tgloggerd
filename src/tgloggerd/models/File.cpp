// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>

namespace tgloggerd {

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
			/*
			 * Bump the hit count, and fill in the original file
			 * name if we did not have one yet (the same content may
			 * first be seen without a name, then later with one).
			 */
			tx.execute("UPDATE files SET hit_count = hit_count + 1,"
				   " orig_file_name = IF(orig_file_name = '',"
				   " ?, orig_file_name) WHERE id = ?",
				   { f.orig_file_name, (int64_t)id });
			return;
		}

		mysql::Param ext = std::monostate{};
		if (f.file_ext.has_value())
			ext = *f.file_ext;

		id = tx.insert(
			"INSERT INTO files (tg_file_id, file_type, file_size,"
			" sha256, file_ext, orig_file_name)"
			" VALUES (?, ?, ?, UNHEX(?), ?, ?)",
			{
				f.tg_file_id,
				f.file_type,
				(int64_t)f.file_size,
				f.sha256_hex,
				ext,
				f.orig_file_name,
			});
	});
	return id;
}

} /* namespace tgloggerd */

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

} /* namespace tgloggerd */

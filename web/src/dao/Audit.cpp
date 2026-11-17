// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "dao/Audit.hpp"

#include <string>
#include <utility>

namespace tgweb::dao::audit {

namespace {

/* web_audit.detail holds this many characters. */
constexpr size_t kMaxDetail = 255;

/*
 * Cut detail to kMaxDetail characters (UTF-8 code points), so an
 * over-long value, such as a username typed at the login form, is
 * recorded instead of failing the insert.
 */
std::string fitDetail(std::string detail)
{
	size_t chars = 0;
	for (size_t i = 0; i < detail.size(); i++) {
		if ((static_cast<unsigned char>(detail[i]) & 0xC0) == 0x80)
			continue;
		if (chars == kMaxDetail) {
			detail.resize(i);
			break;
		}
		chars++;
	}
	return detail;
}

} /* namespace */

drogon::Task<void> log(drogon::orm::DbClientPtr db,
		       std::optional<uint64_t> uid,
		       std::string action,
		       std::string ip,
		       std::string detail)
{
	co_await db->execSqlCoro(
		"INSERT INTO web_audit (web_user_id, action, ip, detail) "
		"VALUES (?, ?, ?, ?)",
		uid, action, ip, fitDetail(std::move(detail)));
	co_return;
}

} /* namespace tgweb::dao::audit */

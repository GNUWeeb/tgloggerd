// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "media/Fetch.hpp"

#include "Config.hpp"
#include "daemon/Api.hpp"

#include <cstdlib>
#include <system_error>

namespace tgweb::media {

namespace {

/* The daemon's per-file store cap, from the environment both share; the
 * daemon's default when unset. 0 = no cap. */
uint64_t storeFileCap(void)
{
	static const uint64_t cap = strtoull(
		tgweb::env("TG_MAX_STORE_FILE_SIZE", "1073741824").c_str(),
		nullptr, 10);
	return cap;
}

} /* namespace */

bool isSmall(uint64_t size)
{
	const uint64_t cap = storeFileCap();
	return cap == 0 || size < cap;
}

FetchDecision mayFetch(uint64_t size, bool hasSession)
{
	if (!daemon::api::enabled())
		return FetchDecision::Off;
	if (!hasSession && !isSmall(size))
		return FetchDecision::NeedsSession;
	return FetchDecision::Allowed;
}

const std::filesystem::path &tdlibDir(void)
{
	static const std::filesystem::path dir = [] {
		std::string d = tgweb::env("TG_DATA_DIR", "");
		if (d.empty())
			return std::filesystem::path();
		std::error_code ec;
		auto p = std::filesystem::weakly_canonical(
			std::filesystem::absolute(std::filesystem::path(d) /
						  "tdlib", ec), ec);
		if (ec)
			return std::filesystem::path();
		return p;
	}();
	return dir;
}

std::optional<std::filesystem::path> insideTdlibDir(const std::string &path)
{
	const auto &root = tdlibDir();
	if (root.empty() || path.empty() || path[0] != '/')
		return std::nullopt;

	std::error_code ec;
	auto p = std::filesystem::weakly_canonical(path, ec);
	if (ec)
		return std::nullopt;

	/* Component-wise prefix: /a/tdlib-other is not inside /a/tdlib, and
	 * the directory itself is not a file inside it. */
	auto r = root.begin();
	auto q = p.begin();
	for (; r != root.end(); ++r, ++q)
		if (q == p.end() || *q != *r)
			return std::nullopt;
	if (q == p.end())
		return std::nullopt;

	if (!std::filesystem::is_regular_file(p, ec))
		return std::nullopt;
	return p;
}

} /* namespace tgweb::media */

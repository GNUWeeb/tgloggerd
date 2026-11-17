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

void markOne(nlohmann::json &md, bool hasSession)
{
	if (!md.is_object() || !md.contains("file_id"))
		return;
	const bool stored = !md.contains("stored") ||
			    !md["stored"].is_boolean() ||
			    md["stored"].get<bool>();
	uint64_t size = 0;
	if (md.contains("size") && md["size"].is_number_unsigned())
		size = md["size"].get<uint64_t>();
	md["fetchable"] = !stored &&
			  mayFetch(size, hasSession) == FetchDecision::Allowed;
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

void markFetchable(nlohmann::json &msg, bool hasSession)
{
	if (msg.contains("media"))
		markOne(msg["media"], hasSession);
	if (msg.contains("items") && msg["items"].is_array())
		for (auto &it : msg["items"])
			if (it.contains("media"))
				markOne(it["media"], hasSession);
	if (msg.contains("link_preview") && msg["link_preview"].is_object() &&
	    msg["link_preview"].contains("media"))
		markOne(msg["link_preview"]["media"], hasSession);
	if (msg.contains("edits") && msg["edits"].is_array())
		for (auto &e : msg["edits"])
			if (e.contains("media"))
				markOne(e["media"], hasSession);
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

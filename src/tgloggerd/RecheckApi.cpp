// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "RecheckApi.hpp"

#include <memory>
#include <string>
#include <utility>

namespace tgloggerd {

namespace {

/* A chat id from the path: a nonzero integer; a 400 otherwise. */
int64_t requireChatId(const InternalApi::Request &req)
{
	const std::string &s = req.param("chat_id");
	size_t used = 0;
	int64_t id = 0;
	try {
		id = std::stoll(s, &used);
	} catch (const std::exception &) {
		used = 0;
	}
	if (used == 0 || used != s.size() || id == 0)
		throw InternalApi::Error::badRequest("chat_id must be an integer");
	return id;
}

/* The body's "around": a server message id, 0 (the default) for the
 * newest; a 400 for anything but an integer from 0 up. */
int64_t requireAround(const nlohmann::json &b)
{
	if (b.is_null())
		return 0;
	if (!b.is_object())
		throw InternalApi::Error::badRequest(
			"want {\"around\": <message id, 0 = the newest>}");
	auto it = b.find("around");
	if (it == b.end())
		return 0;
	int64_t around = -1;
	if (it->is_number_unsigned()) {
		uint64_t u = it->get<uint64_t>();
		if (u <= (uint64_t)recheck::kMaxServerId)
			around = (int64_t)u;
	} else if (it->is_number_integer()) {
		around = it->get<int64_t>();
	}
	if (around < 0 || around > recheck::kMaxServerId)
		throw InternalApi::Error::badRequest(
			"around must be a message id: an integer from 0 to " +
			std::to_string(recheck::kMaxServerId));
	return around;
}

InternalApi::Response recheckChat(const RecheckBackend &be,
				  const InternalApi::Request &req)
{
	const int64_t chat_id = requireChatId(req);
	const int64_t around = requireAround(req.body);

	if (!be.ready())
		throw InternalApi::Error::unavailable("TDLib is not ready");
	std::optional<bool> known = be.tdlib_knows(chat_id);
	if (!known)
		throw InternalApi::Error::unavailable("TDLib is not ready");
	if (!*known && !be.db_knows(chat_id))
		throw InternalApi::Error::notFound("no such chat");

	const recheck::Admit a = be.enqueue(chat_id, around);
	InternalApi::Response r;
	if (a == recheck::Admit::Queued) {
		r.status = 202;
		r.body = { { "queued", true } };
	} else {
		r.body = { { "queued", false },
			   { "reason", recheck::admitReason(a) } };
	}
	return r;
}

} /* namespace */

void registerRecheckApi(InternalApi &api, RecheckBackend backend)
{
	auto be = std::make_shared<RecheckBackend>(std::move(backend));
	api.route("POST", "/v1/chats/{chat_id}/recheck",
		  [be](const InternalApi::Request &req) {
		return recheckChat(*be, req);
	});
}

} /* namespace tgloggerd */

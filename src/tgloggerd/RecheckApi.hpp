// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__RECHECK_API_HPP
#define TGLOGGERD__RECHECK_API_HPP

#include "ChatRecheck.hpp"
#include "InternalApi.hpp"

#include <cstdint>
#include <functional>
#include <optional>

namespace tgloggerd {

class TDLib;
class DB;

/*
 * What POST /v1/chats/{chat_id}/recheck needs, each called on handler
 * threads and thread-safe: whether the TDLib client is authorized;
 * whether TDLib knows the chat (getChat, from its cache), nullopt when
 * it could not be asked because it is not authorized; whether the chat
 * is stored (DB::chatKnown); and the queue (TDLib::recheckChat). The
 * daemon passes TDLib and the DB, the test stand-ins.
 */
struct RecheckBackend {
	std::function<bool()>					ready;
	std::function<std::optional<bool>(int64_t)>		tdlib_knows;
	std::function<bool(int64_t)>				db_knows;
	std::function<recheck::Admit(int64_t, int64_t)>		enqueue;
};

/* The backend of the daemon: @tdlib and @db, which must outlive the API. */
RecheckBackend recheckBackendOf(TDLib &tdlib, DB &db);

/*
 * Register POST /v1/chats/{chat_id}/recheck (docs/internal-api.md):
 * queue a recheck of the chat around a message and answer at once.
 */
void registerRecheckApi(InternalApi &api, RecheckBackend backend);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__RECHECK_API_HPP */

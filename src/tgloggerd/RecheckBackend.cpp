// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * The daemon's RecheckBackend, apart from RecheckApi.cpp so that the
 * endpoint's test links neither TDLib's client nor the database.
 */
#include "RecheckApi.hpp"

#include "DB.hpp"
#include "TDLib.hpp"

#include <td/telegram/td_api.h>

#include <chrono>

namespace tgloggerd {

namespace {

namespace td_api = td::td_api;
using namespace std::chrono_literals;

/* getChat answers from TDLib's cache for a chat it knows. */
constexpr std::chrono::milliseconds kGetChatTimeout = 5s;

} /* namespace */

RecheckBackend recheckBackendOf(TDLib &tdlib, DB &db)
{
	TDLib *t = &tdlib;
	DB *d = &db;
	RecheckBackend be;
	be.ready = [t] { return t->status().state == TDLib::State::Ready; };
	be.tdlib_knows = [t](int64_t chat_id) -> std::optional<bool> {
		TDLib::TdObject obj = t->request(
			td_api::make_object<td_api::getChat>(chat_id),
			kGetChatTimeout);
		if (obj->get_id() == td_api::chat::ID)
			return true;
		if (obj->get_id() == td_api::error::ID &&
		    static_cast<const td_api::error &>(*obj).code_ ==
			    TDLib::kRequestNotReady)
			return std::nullopt;
		/* Not known, or too slow to tell: the DB decides. */
		return false;
	};
	be.db_knows = [d](int64_t chat_id) { return d->chatKnown(chat_id); };
	be.enqueue = [t](int64_t chat_id, int64_t around) {
		return t->recheckChat(chat_id, around);
	};
	return be;
}

} /* namespace tgloggerd */

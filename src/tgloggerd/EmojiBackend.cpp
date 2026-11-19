// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * The daemon's EmojiBackend, apart from EmojiApi.cpp so that the route
 * test links neither the database nor the TDLib client.
 */
#include "EmojiApi.hpp"

#include "DB.hpp"

#include <td/telegram/td_api.h>

#include <utility>

namespace tgloggerd {

EmojiBackend emojiBackendOf(TDLib &tdlib, DB &db)
{
	DB *d = &db;
	TDLib *t = &tdlib;
	EmojiBackend b;
	b.load_emoji = [d](const std::vector<int64_t> &ids) {
		return d->getCustomEmoji(ids);
	};
	b.save_emoji = [d](const models::CustomEmoji &e) { d->putCustomEmoji(e); };
	b.record_file = [d](const models::File &f) {
		return d->upsertFileMeta(f).id;
	};
	b.request = [t](TDLib::TdFunction f, std::chrono::milliseconds timeout) {
		return t->request(std::move(f), timeout);
	};
	return b;
}

} /* namespace tgloggerd */

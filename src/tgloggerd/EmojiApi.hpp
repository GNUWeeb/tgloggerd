// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__EMOJI_API_HPP
#define TGLOGGERD__EMOJI_API_HPP

#include "InternalApi.hpp"
#include "TDLib.hpp"

#include <tgloggerd/models/Emoji.hpp>
#include <tgloggerd/models/File.hpp>

#include <chrono>
#include <functional>
#include <vector>

namespace tgloggerd {

/*
 * What the custom emoji route needs: the table (DB::getCustomEmoji,
 * DB::putCustomEmoji), the metadata-only file rows
 * (DB::upsertFileMeta, returning the row id) and TDLib
 * (TDLib::request with a timeout). Every member is called on handler
 * threads and must be thread-safe; the daemon passes the DB and the
 * TDLib client, the test stand-ins.
 */
struct EmojiBackend {
	std::function<std::vector<models::CustomEmoji>(const std::vector<int64_t> &)>
		load_emoji;
	std::function<void(const models::CustomEmoji &)> save_emoji;
	std::function<uint64_t(const models::File &)> record_file;
	std::function<TDLib::TdObject(TDLib::TdFunction, std::chrono::milliseconds)>
		request;
};

class DB;

/* The backend of the daemon: @db's table and file rows, and @tdlib. Both
 * must outlive the API. */
EmojiBackend emojiBackendOf(TDLib &tdlib, DB &db);

/*
 * Register POST /v1/custom-emoji (docs/internal-api.md): the rows of
 * the ids asked for, filled from getCustomEmojiStickers for the ids not
 * stored yet.
 */
void registerEmojiApi(InternalApi &api, EmojiBackend backend);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__EMOJI_API_HPP */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__EMOJI_EXTRACT_HPP
#define TGLOGGERD__EMOJI_EXTRACT_HPP

#include <tgloggerd/models/Emoji.hpp>
#include <tgloggerd/models/File.hpp>

#include <td/telegram/td_api.h>

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

/*
 * What the custom emoji and message effect tables (message-types.md
 * sections 4.11 and 4.12) keep of TDLib's stickers and effects. Pure
 * functions over td_api objects, shared by the internal API that fills
 * the tables and by the fetcher, which takes a file out of a fresh
 * answer the same way.
 */

namespace tgloggerd {

namespace td_api = td::td_api;

/* "webp", "tgs" or "webm"; "" for a format TDLib adds later or none. */
std::string sticker_format_ext(const td_api::StickerFormat *f);

/* The custom emoji id of a custom emoji sticker; 0 for any other. */
int64_t sticker_custom_emoji_id(const td_api::sticker &s);

/* @s cut to at most @n code points, never inside one (the tables' emoji
 * columns are VARCHAR(16)). */
std::string utf8_prefix(const std::string &s, size_t n);

/* The telegram_custom_emoji row of a custom emoji sticker; file_id 0. */
models::CustomEmoji custom_emoji_of(const td_api::sticker &s);

/*
 * The metadata-only telegram_files row of a sticker's or an effect's
 * file (file_type sticker, extension @ext, on_disk false), or nullopt
 * when the file carries no remote id to record.
 */
std::optional<models::File> sticker_file_meta(const td_api::file *f,
					      const std::string &ext);

/* A file of a message effect, and the column it goes in. */
struct EffectFile {
	enum class Slot { StaticIcon, SelectAnimation, EffectAnimation, Sticker };
	Slot			slot;
	const td_api::file	*file;	/* never null */
	std::string		ext;
};

/*
 * The files of a message effect: its static icon, then an emoji
 * reaction's select and effect animations, or a premium sticker and its
 * premium animation (as the effect animation). Files absent from the
 * answer are left out.
 */
std::vector<EffectFile> message_effect_files(const td_api::messageEffect &e);

/* The telegram_message_effects row of an effect, every file id 0. */
models::MessageEffect message_effect_of(const td_api::messageEffect &e);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__EMOJI_EXTRACT_HPP */

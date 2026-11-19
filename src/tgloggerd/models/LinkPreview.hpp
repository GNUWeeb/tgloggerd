// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__LINK_PREVIEW_HPP
#define TGLOGGERD__MODELS__LINK_PREVIEW_HPP

#include <cstdint>
#include <string>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * The preview card Telegram renders under a message that contains a URL.
 *
 * Resolved by Telegram's servers, not by us: TDLib never fetches a URL, and the
 * preview's image arrives as a Telegram file with a remote id on their CDN,
 * which only their backend can mint. It is worth recording for the same reason
 * the message text is -- it is what the reader saw, and the linked page can
 * change or disappear afterwards.
 *
 * Identified by the message that carries it, since td_api::messageText holds
 * exactly one link_preview.
 */
struct LinkPreview {
	int64_t		chat_id = 0;
	int64_t		message_id = 0;   /* server id */

	std::string	url;
	std::string	display_url;
	std::string	site_name;
	std::string	title;
	std::string	description;
	std::string	author;

	/* td_api::LinkPreviewType, lower-cased and without the prefix. */
	std::string	type = "unsupported";

	int32_t		instant_view_version = 0;
	bool		has_large_media = false;
	bool		show_large_media = false;
	bool		show_above_text = false;

	/* Formatting of `description` as JSON (UTF-16 offsets); nullopt if
	 * none. */
	std::optional<std::string>	description_entities;
	bool		show_media_above_description = false;
	bool		skip_confirmation = false;

	/* The type's own fields that have no column, a curated JSON
	 * object (message-types.md 4.8); nullopt if the type has none. */
	std::optional<std::string>	extra;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__LINK_PREVIEW_HPP */

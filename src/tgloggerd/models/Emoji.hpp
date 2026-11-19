// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__EMOJI_HPP
#define TGLOGGERD__MODELS__EMOJI_HPP

#include <cstdint>
#include <optional>
#include <string>

namespace tgloggerd {
namespace models {

/*
 * A telegram_custom_emoji row (migration 000046): the sticker behind a
 * custom emoji id (docs/design/message-types.md section 4.11).
 */
struct CustomEmoji {
	int64_t			custom_emoji_id = 0;
	std::string		emoji;		/* the emoji it stands for */
	std::optional<int64_t>	set_id;		/* its sticker set, if known */
	std::string		format;		/* webp, tgs, webm; "" unknown */
	bool			needs_repainting = false;
	uint64_t		file_id = 0;	/* telegram_files.id; 0 none */
};

/*
 * A telegram_message_effects row (migration 000047): a message effect
 * with its files (message-types.md section 4.12). A file id of 0 is
 * none.
 */
struct MessageEffect {
	int64_t		effect_id = 0;
	std::string	emoji;
	bool		is_premium = false;
	std::string	effect_type;	/* emoji_reaction, premium_sticker; "" */
	uint64_t	static_icon_file_id = 0;
	uint64_t	select_animation_file_id = 0;
	uint64_t	effect_animation_file_id = 0;
	uint64_t	sticker_file_id = 0;
	std::string	sticker_format;	/* webp, tgs, webm; "" none */
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__EMOJI_HPP */

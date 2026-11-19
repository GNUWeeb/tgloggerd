// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <cstdint>
#include <string>
#include <vector>

namespace tgloggerd {

namespace {

/* "?, ?, ..." for @n placeholders. */
std::string placeholders(size_t n)
{
	std::string s;
	for (size_t i = 0; i < n; i++)
		s += i ? ", ?" : "?";
	return s;
}

mysql::Param fileParam(uint64_t id)
{
	return id ? mysql::Param(id) : mysql::Param(std::monostate{});
}

mysql::Param textOrNull(const std::string &s)
{
	return s.empty() ? mysql::Param(std::monostate{}) : mysql::Param(s);
}

uint64_t fileOf(const std::optional<std::string> &v)
{
	return v.has_value() ? (uint64_t)std::stoull(*v) : 0;
}

} /* namespace */

std::vector<models::CustomEmoji>
DB::getCustomEmoji(const std::vector<int64_t> &ids)
{
	std::vector<models::CustomEmoji> out;
	if (ids.empty())
		return out;
	std::vector<mysql::Param> args(ids.begin(), ids.end());
	auto rows = db_.query(
		"SELECT custom_emoji_id, emoji, set_id, format, needs_repainting,"
		" file_id FROM telegram_custom_emoji"
		" WHERE custom_emoji_id IN (" + placeholders(ids.size()) + ")",
		args);
	for (const auto &r : rows) {
		if (!r[0].has_value())
			continue;
		models::CustomEmoji e;
		e.custom_emoji_id = std::stoll(*r[0]);
		e.emoji = r[1].value_or("");
		if (r[2].has_value())
			e.set_id = std::stoll(*r[2]);
		e.format = r[3].value_or("");
		e.needs_repainting = r[4].has_value() && *r[4] == "1";
		e.file_id = fileOf(r[5]);
		out.push_back(std::move(e));
	}
	return out;
}

void DB::putCustomEmoji(const models::CustomEmoji &e)
{
	mysql::Param set = std::monostate{};
	if (e.set_id)
		set = *e.set_id;
	db_.execute(
		"INSERT INTO telegram_custom_emoji (custom_emoji_id, emoji, set_id,"
		" format, needs_repainting, file_id) VALUES (?, ?, ?, ?, ?, ?)"
		" AS new ON DUPLICATE KEY UPDATE"
		" emoji = new.emoji, set_id = new.set_id, format = new.format,"
		" needs_repainting = new.needs_repainting,"
		" file_id = COALESCE(new.file_id, telegram_custom_emoji.file_id)",
		{ e.custom_emoji_id, e.emoji, set, textOrNull(e.format),
		  (int64_t)(e.needs_repainting ? 1 : 0), fileParam(e.file_id) });
}

std::vector<models::MessageEffect>
DB::getMessageEffects(const std::vector<int64_t> &ids)
{
	std::vector<models::MessageEffect> out;
	if (ids.empty())
		return out;
	std::vector<mysql::Param> args(ids.begin(), ids.end());
	auto rows = db_.query(
		"SELECT effect_id, emoji, is_premium, effect_type,"
		" static_icon_file_id, select_animation_file_id,"
		" effect_animation_file_id, sticker_file_id, sticker_format"
		" FROM telegram_message_effects"
		" WHERE effect_id IN (" + placeholders(ids.size()) + ")",
		args);
	for (const auto &r : rows) {
		if (!r[0].has_value())
			continue;
		models::MessageEffect e;
		e.effect_id = std::stoll(*r[0]);
		e.emoji = r[1].value_or("");
		e.is_premium = r[2].has_value() && *r[2] == "1";
		e.effect_type = r[3].value_or("");
		e.static_icon_file_id = fileOf(r[4]);
		e.select_animation_file_id = fileOf(r[5]);
		e.effect_animation_file_id = fileOf(r[6]);
		e.sticker_file_id = fileOf(r[7]);
		e.sticker_format = r[8].value_or("");
		out.push_back(std::move(e));
	}
	return out;
}

void DB::putMessageEffect(const models::MessageEffect &e)
{
	db_.execute(
		"INSERT INTO telegram_message_effects (effect_id, emoji, is_premium,"
		" effect_type, static_icon_file_id, select_animation_file_id,"
		" effect_animation_file_id, sticker_file_id, sticker_format)"
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
		" AS new ON DUPLICATE KEY UPDATE"
		" emoji = new.emoji, is_premium = new.is_premium,"
		" effect_type = new.effect_type,"
		" static_icon_file_id = COALESCE(new.static_icon_file_id,"
		"  telegram_message_effects.static_icon_file_id),"
		" select_animation_file_id = COALESCE(new.select_animation_file_id,"
		"  telegram_message_effects.select_animation_file_id),"
		" effect_animation_file_id = COALESCE(new.effect_animation_file_id,"
		"  telegram_message_effects.effect_animation_file_id),"
		" sticker_file_id = COALESCE(new.sticker_file_id,"
		"  telegram_message_effects.sticker_file_id),"
		" sticker_format = new.sticker_format",
		{ e.effect_id, e.emoji, (int64_t)(e.is_premium ? 1 : 0),
		  textOrNull(e.effect_type), fileParam(e.static_icon_file_id),
		  fileParam(e.select_animation_file_id),
		  fileParam(e.effect_animation_file_id),
		  fileParam(e.sticker_file_id), textOrNull(e.sticker_format) });
}

} /* namespace tgloggerd */

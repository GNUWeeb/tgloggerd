// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "EmojiExtract.hpp"

namespace tgloggerd {

std::string sticker_format_ext(const td_api::StickerFormat *f)
{
	if (!f)
		return "";
	switch (f->get_id()) {
	case td_api::stickerFormatWebp::ID: return "webp";
	case td_api::stickerFormatTgs::ID:  return "tgs";
	case td_api::stickerFormatWebm::ID: return "webm";
	default:                            return "";
	}
}

int64_t sticker_custom_emoji_id(const td_api::sticker &s)
{
	if (!s.full_type_ ||
	    s.full_type_->get_id() != td_api::stickerFullTypeCustomEmoji::ID)
		return 0;
	return static_cast<const td_api::stickerFullTypeCustomEmoji &>(
		*s.full_type_).custom_emoji_id_;
}

std::string utf8_prefix(const std::string &s, size_t n)
{
	size_t i = 0, cps = 0;
	while (i < s.size()) {
		/* A code point starts at any byte but a continuation byte. */
		if (((unsigned char)s[i] & 0xC0) != 0x80) {
			if (cps == n)
				break;
			cps++;
		}
		i++;
	}
	return s.substr(0, i);
}

models::CustomEmoji custom_emoji_of(const td_api::sticker &s)
{
	models::CustomEmoji e;
	e.custom_emoji_id = sticker_custom_emoji_id(s);
	e.emoji = utf8_prefix(s.emoji_, 16);
	if (s.set_id_ != 0)
		e.set_id = s.set_id_;
	e.format = sticker_format_ext(s.format_.get());
	if (s.full_type_ &&
	    s.full_type_->get_id() == td_api::stickerFullTypeCustomEmoji::ID)
		e.needs_repainting =
			static_cast<const td_api::stickerFullTypeCustomEmoji &>(
				*s.full_type_).needs_repainting_;
	return e;
}

std::optional<models::File> sticker_file_meta(const td_api::file *f,
					      const std::string &ext)
{
	if (!f || !f->remote_ || f->remote_->id_.empty())
		return std::nullopt;
	models::File mf;
	mf.tg_file_id = f->remote_->id_;
	mf.tg_unique_id = f->remote_->unique_id_;
	mf.file_type = "sticker";
	mf.file_size = f->size_ > 0 ? (uint64_t)f->size_ : 0;
	mf.on_disk = false;
	if (!ext.empty())
		mf.file_ext = ext;
	return mf;
}

namespace {

void addSticker(std::vector<EffectFile> &out, EffectFile::Slot slot,
		const td_api::sticker *s)
{
	if (s && s->sticker_)
		out.push_back({ slot, s->sticker_.get(),
				sticker_format_ext(s->format_.get()) });
}

} /* namespace */

std::vector<EffectFile> message_effect_files(const td_api::messageEffect &e)
{
	using Slot = EffectFile::Slot;
	std::vector<EffectFile> out;
	addSticker(out, Slot::StaticIcon, e.static_icon_.get());
	if (!e.type_)
		return out;
	switch (e.type_->get_id()) {
	case td_api::messageEffectTypeEmojiReaction::ID: {
		auto &t = static_cast<const td_api::messageEffectTypeEmojiReaction &>(
			*e.type_);
		addSticker(out, Slot::SelectAnimation, t.select_animation_.get());
		addSticker(out, Slot::EffectAnimation, t.effect_animation_.get());
		break;
	}
	case td_api::messageEffectTypePremiumSticker::ID: {
		auto &t = static_cast<const td_api::messageEffectTypePremiumSticker &>(
			*e.type_);
		addSticker(out, Slot::Sticker, t.sticker_.get());
		/* The effect itself is the sticker's premium animation, a
		 * Lottie animation like the reactions' effects. */
		if (t.sticker_ && t.sticker_->full_type_ &&
		    t.sticker_->full_type_->get_id() ==
			    td_api::stickerFullTypeRegular::ID) {
			auto &r = static_cast<const td_api::stickerFullTypeRegular &>(
				*t.sticker_->full_type_);
			if (r.premium_animation_)
				out.push_back({ Slot::EffectAnimation,
						r.premium_animation_.get(), "tgs" });
		}
		break;
	}
	default:
		break;
	}
	return out;
}

models::MessageEffect message_effect_of(const td_api::messageEffect &e)
{
	models::MessageEffect m;
	m.effect_id = e.id_;
	m.emoji = utf8_prefix(e.emoji_, 16);
	m.is_premium = e.is_premium_;
	if (!e.type_)
		return m;
	switch (e.type_->get_id()) {
	case td_api::messageEffectTypeEmojiReaction::ID:
		m.effect_type = "emoji_reaction";
		break;
	case td_api::messageEffectTypePremiumSticker::ID: {
		m.effect_type = "premium_sticker";
		auto &t = static_cast<const td_api::messageEffectTypePremiumSticker &>(
			*e.type_);
		if (t.sticker_)
			m.sticker_format = sticker_format_ext(t.sticker_->format_.get());
		break;
	}
	default:
		break;
	}
	return m;
}

} /* namespace tgloggerd */

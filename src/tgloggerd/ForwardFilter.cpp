// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include <tgloggerd/ForwardFilter.hpp>

namespace tgloggerd {

unsigned forward_kind_type(const std::string &kind)
{
	static const struct {
		const char	*kind;
		unsigned	bit;
	} map[] = {
		{ "text", FWD_TEXT },
		{ "photo", FWD_PHOTO },
		{ "video", FWD_VIDEO },
		{ "animation", FWD_ANIMATION },
		{ "video_note", FWD_VIDEO_NOTE },
		{ "document", FWD_DOCUMENT },
		{ "sticker", FWD_STICKER },
		{ "voice", FWD_VOICE },
		{ "audio", FWD_AUDIO },
		{ "poll", FWD_POLL },
		{ "checklist", FWD_CHECKLIST },
		{ "dice", FWD_DICE },
		{ "game", FWD_GAME },
		{ "location", FWD_LOCATION },
		{ "venue", FWD_LOCATION },
		{ "contact", FWD_CONTACT },
		{ "story", FWD_STORY },
		{ "paid_media", FWD_PAID_MEDIA },
		{ "invoice", FWD_INVOICE },
		{ "service", FWD_SERVICE },
	};
	for (const auto &m : map)
		if (kind == m.kind)
			return m.bit;
	return FWD_OTHER;
}

unsigned forward_text_type(const std::string &kind, bool has_file)
{
	if (kind == "text" || kind == "paid_media" || has_file)
		return FWD_TEXT;
	return forward_kind_type(kind);
}

ForwardParts forward_parts(const std::string &kind, bool has_file,
			   unsigned types)
{
	ForwardParts p;
	p.text = (types & forward_text_type(kind, has_file)) != 0;
	p.summary = (types & forward_kind_type(kind)) != 0;
	p.file = has_file && (types & forward_kind_type(kind)) != 0;
	return p;
}

} /* namespace tgloggerd */

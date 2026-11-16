// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_DAO_FORWARDTYPES_HPP
#define TGLOGGERD_WEB_DAO_FORWARDTYPES_HPP

#include <cstddef>
#include <string>

/*
 * The content types a forwarding route may carry: the fwd_* columns of
 * `telegram_discord_webhooks` and `discord_telegram_routes`. The daemons
 * forward a part of a message only when the flag of its type is set.
 *
 * Both directions share this one description so their forms, their request
 * parameters (fwd_<key>) and their labels cannot drift apart. They differ only
 * in how many of the types apply: Discord -> Telegram has no "other".
 */
namespace tgweb::dao {

struct ForwardTypes {
	bool sticker  = true;
	bool text     = true;
	bool photo    = true;
	bool video    = true;
	bool document = true;
	bool other    = true;
};

struct ForwardTypeDef {
	const char *key;	/* column fwd_<key>, request parameter fwd_<key> */
	const char *label;	/* as shown to the operator */
	bool ForwardTypes::*flag;
};

/* In the order the forms list them. "other" is last so a direction without
 * it takes a prefix. */
inline constexpr ForwardTypeDef kForwardTypes[] = {
	{ "sticker",  "Sticker",      &ForwardTypes::sticker  },
	{ "text",     "Text message", &ForwardTypes::text     },
	{ "photo",    "Photo",        &ForwardTypes::photo    },
	{ "video",    "Video",        &ForwardTypes::video    },
	{ "document", "Document",     &ForwardTypes::document },
	{ "other",    "Others",       &ForwardTypes::other    },
};

/* How many of kForwardTypes each direction uses. */
inline constexpr size_t kTelegramDiscordTypes = 6;
inline constexpr size_t kDiscordTelegramTypes = 5;

/* The enabled types' keys, comma-separated: what the edit form restores its
 * checkboxes from. */
inline std::string forwardTypesKeys(const ForwardTypes &t, size_t n)
{
	std::string out;
	for (size_t i = 0; i < n; i++) {
		if (!(t.*kForwardTypes[i].flag))
			continue;
		if (!out.empty())
			out += ",";
		out += kForwardTypes[i].key;
	}
	return out;
}

/* The enabled types as a sentence fragment for the route table. */
inline std::string forwardTypesLabel(const ForwardTypes &t, size_t n)
{
	std::string out;
	size_t on = 0;
	for (size_t i = 0; i < n; i++) {
		if (!(t.*kForwardTypes[i].flag))
			continue;
		if (!out.empty())
			out += ", ";
		out += kForwardTypes[i].label;
		on++;
	}
	if (on == n)
		return "Everything";
	return on ? out : std::string("Nothing");
}

} /* namespace tgweb::dao */

#endif /* TGLOGGERD_WEB_DAO_FORWARDTYPES_HPP */

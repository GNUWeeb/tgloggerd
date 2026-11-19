// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ChatRef.hpp"

#include <algorithm>
#include <cctype>
#include <cstdlib>

namespace tgloggerd {

namespace {

std::string lower(std::string_view s)
{
	std::string out(s);
	for (auto &c : out)
		c = (char)std::tolower((unsigned char)c);
	return out;
}

bool startsWithNoCase(std::string_view s, std::string_view prefix)
{
	return s.size() >= prefix.size() &&
	       lower(s.substr(0, prefix.size())) == prefix;
}

/*
 * A username as Telegram allows it: 4 to 32 of [A-Za-z0-9_], starting with
 * a letter and not ending with '_' (4 only for the auctioned short names).
 */
bool validUsername(std::string_view s)
{
	if (s.size() < 4 || s.size() > 32)
		return false;
	if (!std::isalpha((unsigned char)s.front()) || s.back() == '_')
		return false;
	return std::all_of(s.begin(), s.end(), [](char c) {
		return std::isalnum((unsigned char)c) || c == '_';
	});
}

/* An invite hash: URL-safe base64. Hashes are 16 to 22 characters today;
 * the bounds only keep junk out. */
bool validHash(std::string_view s)
{
	if (s.size() < 4 || s.size() > 64)
		return false;
	return std::all_of(s.begin(), s.end(), [](char c) {
		return std::isalnum((unsigned char)c) || c == '_' || c == '-';
	});
}

/*
 * t.me paths that are not usernames but would pass validUsername(). Their
 * links carry a second segment, which is refused anyway; the bare path is
 * refused here so it never reaches searchPublicChat.
 */
bool reservedPath(std::string_view s)
{
	static const char *const kReserved[] = {
		"joinchat", "addstickers", "addemoji", "addlist", "addtheme",
		"share", "proxy", "socks", "login", "setlanguage",
		"confirmphone", "boost", "invoice", "contact", "giftcode",
	};
	std::string l = lower(s);
	for (const char *r : kReserved)
		if (l == r)
			return true;
	return false;
}

/* The decimal number right after `marker` in `s`, or -1. */
long numberAfter(const std::string &s, const char *marker)
{
	size_t p = s.find(marker);
	if (p == std::string::npos)
		return -1;
	p += std::char_traits<char>::length(marker);
	if (p >= s.size() || !std::isdigit((unsigned char)s[p]))
		return -1;
	return strtol(s.c_str() + p, nullptr, 10);
}

bool has(const std::string &s, const char *needle)
{
	return s.find(needle) != std::string::npos;
}

} /* namespace */

std::string ChatRef::canonical(void) const
{
	return kind == Kind::Invite ? "https://t.me/+" + value : "@" + value;
}

const char *ChatRef::kindName(void) const
{
	return kind == Kind::Invite ? "invite" : "username";
}

std::optional<ChatRef> parseChatRef(std::string_view in)
{
	auto ws = [](char c) { return std::isspace((unsigned char)c) != 0; };
	while (!in.empty() && ws(in.front()))
		in.remove_prefix(1);
	while (!in.empty() && ws(in.back()))
		in.remove_suffix(1);
	if (in.empty() || in.size() > 256)
		return std::nullopt;
	if (std::any_of(in.begin(), in.end(), ws))
		return std::nullopt;

	if (in.front() == '@') {
		in.remove_prefix(1);
		if (!validUsername(in))
			return std::nullopt;
		return ChatRef{ ChatRef::Kind::Username, std::string(in) };
	}

	/* A link: strip the scheme, then require one of Telegram's hosts. */
	std::string_view s = in;
	bool link = false;
	if (startsWithNoCase(s, "https://")) {
		s.remove_prefix(8);
		link = true;
	} else if (startsWithNoCase(s, "http://")) {
		s.remove_prefix(7);
		link = true;
	}
	if (startsWithNoCase(s, "www."))
		s.remove_prefix(4);

	bool host = false;
	for (std::string_view h : { "t.me/", "telegram.me/", "telegram.dog/" }) {
		if (startsWithNoCase(s, h)) {
			s.remove_prefix(h.size());
			host = true;
			break;
		}
	}
	if (!host) {
		/* No host: only a bare username is left to accept. */
		if (link || !validUsername(s))
			return std::nullopt;
		return ChatRef{ ChatRef::Kind::Username, std::string(s) };
	}

	/* Query string and fragment carry nothing we use (?start=...). */
	size_t q = s.find_first_of("?#");
	if (q != std::string_view::npos)
		s = s.substr(0, q);
	if (!s.empty() && s.back() == '/')
		s.remove_suffix(1);

	if (!s.empty() && s.front() == '+') {
		s.remove_prefix(1);
		/* t.me/+<digits> is a phone number, not an invite. */
		if (!validHash(s) ||
		    std::all_of(s.begin(), s.end(), [](char c) {
			    return std::isdigit((unsigned char)c);
		    }))
			return std::nullopt;
		return ChatRef{ ChatRef::Kind::Invite, std::string(s) };
	}
	if (startsWithNoCase(s, "joinchat/")) {
		s.remove_prefix(9);
		if (!validHash(s))
			return std::nullopt;
		return ChatRef{ ChatRef::Kind::Invite, std::string(s) };
	}
	if (s.find('/') != std::string_view::npos || !validUsername(s) ||
	    reservedPath(s))
		return std::nullopt;
	return ChatRef{ ChatRef::Kind::Username, std::string(s) };
}

ChatOutcome classifyChatError(int32_t code, const std::string &message)
{
	ChatOutcome o;
	o.message = message;

	/*
	 * TDLib reports FLOOD_WAIT_<n> as 429 "Too Many Requests: retry after
	 * <n>"; the raw Telegram form is matched too, in case it comes through.
	 */
	long wait = numberAfter(message, "retry after ");
	if (wait < 0)
		wait = numberAfter(message, "FLOOD_WAIT_");
	if (code == 429 || wait >= 0) {
		o.outcome = "flood_wait";
		o.retry_after = wait > 0 ? (int)std::min(wait, 86400L * 7) : 0;
		return o;
	}

	/* The codes TDLib::request answers with itself (TDLib.hpp). */
	if (code == 503) {
		o.outcome = "unavailable";
		return o;
	}
	if (code == 504) {
		o.outcome = "timeout";
		return o;
	}

	if (has(message, "INVITE_HASH_EXPIRED") ||
	    has(message, "INVITE_HASH_INVALID") ||
	    has(message, "INVITE_HASH_EMPTY") ||
	    has(message, "Wrong invite link") ||
	    has(message, "Invalid invite link"))
		o.outcome = "link_invalid";
	else if (has(message, "INVITE_REQUEST_SENT"))
		o.outcome = "request_sent";
	else if (has(message, "USER_ALREADY_PARTICIPANT"))
		o.outcome = "already_member";
	else if (has(message, "USERNAME_NOT_OCCUPIED") ||
		 has(message, "USERNAME_INVALID") ||
		 has(message, "Chat not found"))
		o.outcome = "not_found";
	else if (has(message, "USER_BANNED_IN_CHANNEL") ||
		 has(message, "CHANNEL_PRIVATE") ||
		 has(message, "CHANNEL_BANNED") ||
		 has(message, "CHAT_FORBIDDEN"))
		/* CHANNEL_PRIVATE is also what a ban from the chat looks
		 * like; the message says which Telegram meant. */
		o.outcome = "banned";
	else if (has(message, "CHANNELS_TOO_MUCH"))
		o.outcome = "too_many_chats";
	else
		o.outcome = "error";
	return o;
}

namespace {

int hexValue(char c)
{
	if (c >= '0' && c <= '9')
		return c - '0';
	if (c >= 'a' && c <= 'f')
		return c - 'a' + 10;
	if (c >= 'A' && c <= 'F')
		return c - 'A' + 10;
	return -1;
}

/* Code points of a UTF-8 string, or -1 when it is not valid UTF-8. */
long utf8Length(const std::string &s)
{
	long n = 0;
	for (size_t i = 0; i < s.size(); n++) {
		const unsigned char c = (unsigned char)s[i];
		size_t len = c < 0x80 ? 1 : (c >> 5) == 0x6 ? 2
			   : (c >> 4) == 0xe ? 3 : (c >> 3) == 0x1e ? 4 : 0;
		if (len == 0 || i + len > s.size())
			return -1;
		for (size_t k = 1; k < len; k++)
			if (((unsigned char)s[i + k] >> 6) != 0x2)
				return -1;
		i += len;
	}
	return n;
}

std::string lowerAscii(std::string s)
{
	for (char &ch : s)
		if (ch >= 'A' && ch <= 'Z')
			ch = (char)(ch - 'A' + 'a');
	return s;
}

} /* namespace */

std::optional<std::string> parseMemberQuery(std::string_view raw)
{
	std::string q;
	for (size_t i = 0; i < raw.size(); i++) {
		if (raw[i] == '+') {
			q += ' ';
		} else if (raw[i] == '%') {
			/* Two hex digits must follow. */
			if (i + 2 >= raw.size())
				return std::nullopt;
			int hi = hexValue(raw[i + 1]);
			int lo = hexValue(raw[i + 2]);
			if (hi < 0 || lo < 0)
				return std::nullopt;
			q += (char)(hi * 16 + lo);
			i += 2;
		} else {
			q += raw[i];
		}
	}
	size_t b = q.find_first_not_of(" \t\r\n");
	size_t e = q.find_last_not_of(" \t\r\n");
	q = b == std::string::npos ? std::string() : q.substr(b, e - b + 1);
	if (!q.empty() && q[0] == '@')
		q.erase(0, 1);
	const long len = utf8Length(q);
	if (len < 0 || len > 64)
		return std::nullopt;
	return q;
}

std::optional<int64_t> memberQueryId(const std::string &q)
{
	size_t i = (!q.empty() && q[0] == '-') ? 1 : 0;
	if (i >= q.size() || q.size() - i > 19)
		return std::nullopt;
	for (size_t k = i; k < q.size(); k++)
		if (q[k] < '0' || q[k] > '9')
			return std::nullopt;
	try {
		int64_t id = std::stoll(q);
		if (id == 0)
			return std::nullopt;
		return id;
	} catch (const std::exception &) {
		return std::nullopt;
	}
}

bool memberMatchesQuery(const std::string &q, const std::string &first_name,
			const std::string &last_name,
			const std::vector<std::string> &usernames)
{
	const std::string needle = lowerAscii(q);
	if (needle.empty())
		return true;
	auto has = [&needle](const std::string &hay) {
		return lowerAscii(hay).find(needle) != std::string::npos;
	};
	if (has(first_name) || has(last_name) ||
	    has(first_name + " " + last_name))
		return true;
	for (const auto &u : usernames)
		if (has(u))
			return true;
	return false;
}

} /* namespace tgloggerd */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__CHAT_REF_HPP
#define TGLOGGERD__CHAT_REF_HPP

#include <cstdint>
#include <optional>
#include <string>
#include <string_view>

namespace tgloggerd {

/*
 * What an admin may ask the account to join (docs/internal-api.md,
 * POST /v1/chats/resolve and /v1/chats/join): an invite link or a public
 * username, and nothing else. Kept free of TDLib so it can be tested alone.
 */
struct ChatRef {
	enum class Kind { Invite, Username };

	Kind		kind;
	/* Invite: the link's hash. Username: the name, without the '@'. */
	std::string	value;

	/* The canonical form, as answered to the client and given to TDLib
	 * for an invite: "https://t.me/+<hash>" or "@<name>". */
	std::string canonical(void) const;
	const char *kindName(void) const;
};

/*
 * Parse what an admin pasted. Accepted, after trimming whitespace:
 *
 *   https://t.me/+<hash>, https://t.me/joinchat/<hash>   (invite links)
 *   @<name>, <name>, https://t.me/<name>                 (usernames)
 *
 * The scheme (http or https) is optional, the host may be t.me,
 * telegram.me or telegram.dog with an optional "www.", and a trailing '/',
 * a query string or a fragment is ignored. Everything else is refused:
 * other hosts, message and sticker links, phone-number links (t.me/+<digits>),
 * extra path segments, malformed hashes or usernames.
 */
std::optional<ChatRef> parseChatRef(std::string_view in);

/*
 * How a join attempt or a preview ended, as the client sees it. "ok" is a
 * successful preview; the rest are the join outcomes and the failures a
 * preview shares with a join.
 */
struct ChatOutcome {
	std::string	outcome;
	std::string	message;	/* human-readable detail */
	int		retry_after = 0;	/* flood_wait: seconds */
};

/*
 * Classify a td_api::error (code, message) from a resolve or join request.
 * Outcomes: flood_wait, link_invalid, not_found, banned, already_member,
 * request_sent, too_many_chats, unavailable (code 503: TDLib not ready),
 * timeout (code 504: no answer in time), error (anything else; message is
 * Telegram's).
 */
ChatOutcome classifyChatError(int32_t code, const std::string &message);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__CHAT_REF_HPP */

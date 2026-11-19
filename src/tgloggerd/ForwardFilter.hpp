// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__FORWARD_FILTER_HPP
#define TGLOGGERD__FORWARD_FILTER_HPP

#include <string>

namespace tgloggerd {

/*
 * The content types a Discord webhook integration can be told to forward, one
 * bit each (the fwd_* columns, migrations 000031 and 000048). A message is
 * filtered part by part: its text or caption follows FWD_TEXT, its media the
 * bit of its own type, and a kind's own text and summary line (a poll, a
 * location, ...) the bit of its kind.
 */
enum : unsigned {
	FWD_STICKER    = 1u << 0,
	FWD_TEXT       = 1u << 1,
	FWD_PHOTO      = 1u << 2,
	FWD_VIDEO      = 1u << 3,
	FWD_DOCUMENT   = 1u << 4,
	/* Every kind without a bit of its own: rich messages, calls,
	 * giveaways, and kinds added later. */
	FWD_OTHER      = 1u << 5,
	FWD_ANIMATION  = 1u << 6,
	FWD_VIDEO_NOTE = 1u << 7,
	FWD_VOICE      = 1u << 8,
	FWD_AUDIO      = 1u << 9,
	FWD_POLL       = 1u << 10,	/* quizzes too */
	FWD_CHECKLIST  = 1u << 11,
	FWD_DICE       = 1u << 12,
	FWD_GAME       = 1u << 13,
	FWD_LOCATION   = 1u << 14,	/* venues and live locations too */
	FWD_CONTACT    = 1u << 15,
	FWD_STORY      = 1u << 16,
	FWD_PAID_MEDIA = 1u << 17,
	FWD_INVOICE    = 1u << 18,
	FWD_SERVICE    = 1u << 19,	/* service (system) messages */
};

/*
 * Which parts of a Telegram message a Discord webhook gets, from its FWD_*
 * bits (above; decision 2026-11-20-fwd-more-types). A message is filtered
 * part by part:
 *
 *  - its file follows the bit of its kind (photo, voice, video note, ...);
 *  - its text follows FWD_TEXT when it is a message's text or a caption
 *    (a text message, a message with a file, paid media); otherwise the
 *    text belongs to the kind (a poll's description, a game's text, a
 *    checklist's title, a rich message, a service message's line) and
 *    follows the kind's bit;
 *  - its summary line (message-types.md section 4.10) follows the kind's
 *    bit.
 *
 * A kind without a bit of its own (rich, call, giveaway, and kinds added
 * later) follows FWD_OTHER.
 */

/* The FWD_* bit of a content kind, as forward_kind_str or content_type
 * names it ("photo", "venue", "service", ...). */
unsigned forward_kind_type(const std::string &kind);

/* The FWD_* bit the text of a message of @kind follows. */
unsigned forward_text_type(const std::string &kind, bool has_file);

/* The parts of a message a webhook taking @types gets. */
struct ForwardParts {
	bool	text = false;
	bool	summary = false;
	bool	file = false;
};

ForwardParts forward_parts(const std::string &kind, bool has_file,
			   unsigned types);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__FORWARD_FILTER_HPP */

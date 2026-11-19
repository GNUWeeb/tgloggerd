// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__ROUTE_FILTER_HPP
#define DISCORDD__ROUTE_FILTER_HPP

#include <gwdiscord/Events.hpp>

namespace discordd {

/*
 * The content types a route can be told to forward, one bit each (the fwd_*
 * columns, migrations 000031, 000048 and 000050). A message is filtered part
 * by part: its text follows FWD_TEXT, each attachment the bit of its own
 * type (a voice message's recording FWD_VOICE), each sticker FWD_STICKER,
 * its embeds FWD_EMBED and its poll FWD_POLL. A system message is one line
 * under FWD_SYSTEM, its content included, since that is part of the event
 * (a thread's or a channel's new name).
 */
enum : unsigned {
	FWD_STICKER   = 1u << 0,
	FWD_TEXT      = 1u << 1,
	FWD_PHOTO     = 1u << 2,	/* an image attachment, but a GIF */
	FWD_VIDEO     = 1u << 3,	/* a video attachment */
	FWD_DOCUMENT  = 1u << 4,	/* every other attachment */
	FWD_AUDIO     = 1u << 5,	/* an audio attachment */
	FWD_ANIMATION = 1u << 6,	/* a GIF attachment */
	FWD_VOICE     = 1u << 7,	/* a voice message's recording */
	FWD_EMBED     = 1u << 8,	/* a message's rich embeds */
	FWD_POLL      = 1u << 9,	/* a message's poll */
	FWD_SYSTEM    = 1u << 10,	/* a system message */
};

/* The FWD_* bit an attachment is filtered by, from its MIME type. */
unsigned attachment_type(const gwdiscord::Attachment &a);

/*
 * The FWD_* bit attachment @a of message @m is filtered by: FWD_VOICE for a
 * voice message's recording, else what its MIME type says.
 */
unsigned attachment_type(const gwdiscord::Message &m,
			 const gwdiscord::Attachment &a);

} /* namespace discordd */

#endif /* #ifndef DISCORDD__ROUTE_FILTER_HPP */

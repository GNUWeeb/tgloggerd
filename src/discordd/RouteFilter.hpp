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
 * columns, migrations 000031 and 000049). A message is filtered part by
 * part: its text follows FWD_TEXT, each attachment the bit of its own type,
 * each sticker FWD_STICKER.
 */
enum : unsigned {
	FWD_STICKER   = 1u << 0,
	FWD_TEXT      = 1u << 1,
	FWD_PHOTO     = 1u << 2,	/* an image attachment, but a GIF */
	FWD_VIDEO     = 1u << 3,	/* a video attachment */
	FWD_DOCUMENT  = 1u << 4,	/* every other attachment */
	FWD_AUDIO     = 1u << 5,	/* an audio attachment */
	FWD_ANIMATION = 1u << 6,	/* a GIF attachment */
};

/* The FWD_* bit an attachment is filtered by, from its MIME type. */
unsigned attachment_type(const gwdiscord::Attachment &a);

} /* namespace discordd */

#endif /* #ifndef DISCORDD__ROUTE_FILTER_HPP */

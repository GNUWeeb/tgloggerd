// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "RouteFilter.hpp"

#include <cctype>
#include <string>

namespace discordd {

namespace {

/* Whether MIME type @t (any case, parameters allowed) starts with @prefix,
 * which is lower case. */
bool mime_is(const std::string &t, const char *prefix)
{
	size_t i = 0;
	for (; prefix[i]; i++)
		if (i >= t.size() ||
		    std::tolower((unsigned char)t[i]) != prefix[i])
			return false;
	return true;
}

} /* namespace */

unsigned attachment_type(const gwdiscord::Attachment &a)
{
	const std::string &t = a.content_type;
	/* image/gif exactly, not a type that merely starts so. */
	if (mime_is(t, "image/gif") &&
	    (t.size() == 9 || t[9] == ';' || t[9] == ' '))
		return FWD_ANIMATION;
	if (mime_is(t, "image/"))
		return FWD_PHOTO;
	if (mime_is(t, "video/"))
		return FWD_VIDEO;
	if (mime_is(t, "audio/"))
		return FWD_AUDIO;
	return FWD_DOCUMENT; /* including one Discord sent no type for */
}

unsigned attachment_type(const gwdiscord::Message &m,
			 const gwdiscord::Attachment &a)
{
	const unsigned t = attachment_type(a);
	/*
	 * A voice message carries its recording as its only attachment,
	 * audio/ogg with a duration and a waveform: it follows FWD_VOICE, not
	 * FWD_AUDIO, so a route can take voice messages without audio files
	 * and the other way round.
	 */
	if (m.is_voice_message() &&
	    (t == FWD_AUDIO || a.duration_secs > 0 || !a.waveform.empty()))
		return FWD_VOICE;
	return t;
}

} /* namespace discordd */

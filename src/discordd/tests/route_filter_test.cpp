// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * discordd_route_filter_test -- the forwarding flag each Discord attachment
 * follows (decision 2026-11-20-fwd-more-types), a voice message's recording
 * included.
 */
#include "../RouteFilter.hpp"

#include <cstdio>
#include <string>

using namespace discordd;

static int g_failures;

static void check(const char *type, unsigned want)
{
	gwdiscord::Attachment a;
	a.content_type = type;
	const bool ok = attachment_type(a) == want;
	printf("  %s: '%s'\n", ok ? "ok:  " : "FAIL:", type);
	if (!ok)
		g_failures++;
}

/* Attachment @type in a message with or without IS_VOICE_MESSAGE. */
static void check_voice(const char *type, bool voice, double secs,
			unsigned want)
{
	gwdiscord::Message m;
	if (voice)
		m.flags = gwdiscord::Message::FLAG_IS_VOICE_MESSAGE;
	gwdiscord::Attachment a;
	a.content_type = type;
	a.duration_secs = secs;
	const bool ok = attachment_type(m, a) == want;
	printf("  %s: '%s'%s, %.1f s\n", ok ? "ok:  " : "FAIL:", type,
	       voice ? " in a voice message" : "", secs);
	if (!ok)
		g_failures++;
}

int main(void)
{
	printf("attachments by MIME type\n");
	check("image/png", FWD_PHOTO);
	check("image/jpeg", FWD_PHOTO);
	check("image/webp", FWD_PHOTO);
	check("image/gif", FWD_ANIMATION);
	check("IMAGE/GIF", FWD_ANIMATION);
	check("image/gif; charset=binary", FWD_ANIMATION);
	check("image/gifx", FWD_PHOTO);
	check("video/mp4", FWD_VIDEO);
	check("audio/ogg", FWD_AUDIO);
	check("audio/mpeg", FWD_AUDIO);
	check("application/pdf", FWD_DOCUMENT);
	check("text/plain; charset=utf-8", FWD_DOCUMENT);
	check("", FWD_DOCUMENT);
	check("image", FWD_DOCUMENT);

	printf("voice messages\n");
	check_voice("audio/ogg", true, 0, FWD_VOICE);
	check_voice("audio/ogg", false, 0, FWD_AUDIO);
	check_voice("", true, 2.5, FWD_VOICE);
	check_voice("", false, 2.5, FWD_DOCUMENT);
	check_voice("image/png", true, 0, FWD_PHOTO);

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

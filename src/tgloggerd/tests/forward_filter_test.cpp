// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * forward_filter_test -- which parts of a Telegram message a Discord
 * webhook gets for its forwarding flags (decision 2026-11-20-fwd-more-types).
 */
#include <tgloggerd/ForwardFilter.hpp>

#include <cstdio>
#include <string>

using namespace tgloggerd;

static int g_failures;

#define CHECK(cond, what)                                                     \
	do {                                                                  \
		if (!(cond)) {                                                \
			printf("  FAIL: %s\n", (what));                       \
			g_failures++;                                         \
		} else {                                                      \
			printf("  ok:   %s\n", (what));                       \
		}                                                             \
	} while (0)

/* Every flag but @bit. */
static unsigned all_but(unsigned bit)
{
	return ~bit;
}

int main(void)
{
	printf("each kind has its own flag\n");
	static const struct {
		const char	*kind;
		unsigned	bit;
	} kinds[] = {
		{ "text", FWD_TEXT },	      { "photo", FWD_PHOTO },
		{ "video", FWD_VIDEO },	      { "animation", FWD_ANIMATION },
		{ "video_note", FWD_VIDEO_NOTE },
		{ "document", FWD_DOCUMENT }, { "sticker", FWD_STICKER },
		{ "voice", FWD_VOICE },	      { "audio", FWD_AUDIO },
		{ "poll", FWD_POLL },	      { "checklist", FWD_CHECKLIST },
		{ "dice", FWD_DICE },	      { "game", FWD_GAME },
		{ "location", FWD_LOCATION }, { "venue", FWD_LOCATION },
		{ "contact", FWD_CONTACT },   { "story", FWD_STORY },
		{ "paid_media", FWD_PAID_MEDIA },
		{ "invoice", FWD_INVOICE },   { "service", FWD_SERVICE },
	};
	for (const auto &k : kinds) {
		std::string what = std::string(k.kind) + " follows its flag";
		CHECK(forward_kind_type(k.kind) == k.bit, what.c_str());
	}

	printf("kinds without a flag follow other\n");
	for (const char *k : { "rich", "call", "giveaway", "", "widget",
			       "some_future_kind" }) {
		std::string what = std::string("'") + k + "' follows other";
		CHECK(forward_kind_type(k) == FWD_OTHER, what.c_str());
	}

	printf("captions follow text, a file its kind\n");
	ForwardParts p = forward_parts("voice", true, all_but(FWD_VOICE));
	CHECK(p.text && !p.file, "voice off: the caption still goes");
	p = forward_parts("voice", true, all_but(FWD_TEXT));
	CHECK(!p.text && p.file, "text off: the voice note goes alone");
	p = forward_parts("video_note", true, FWD_VIDEO_NOTE);
	CHECK(p.file && !p.text, "a video note with only its flag");
	p = forward_parts("animation", true, FWD_OTHER);
	CHECK(!p.file, "an animation no longer follows other");
	p = forward_parts("paid_media", false, FWD_TEXT);
	CHECK(p.text && !p.summary, "paid media: the caption follows text");
	p = forward_parts("paid_media", false, FWD_PAID_MEDIA);
	CHECK(!p.text && p.summary, "paid media: its line follows its flag");

	printf("a kind's own text and line follow the kind\n");
	p = forward_parts("poll", false, all_but(FWD_POLL));
	CHECK(!p.text && !p.summary, "poll off: neither its description nor its options");
	p = forward_parts("poll", false, FWD_POLL);
	CHECK(p.text && p.summary, "poll on, text off: the whole poll");
	p = forward_parts("checklist", false, FWD_TEXT);
	CHECK(!p.text && !p.summary, "checklist off: not even its title");
	p = forward_parts("location", false, FWD_LOCATION);
	CHECK(p.summary, "location on: its map line");
	p = forward_parts("venue", false, all_but(FWD_LOCATION));
	CHECK(!p.summary, "location off: no venue line");
	p = forward_parts("service", false, FWD_TEXT | FWD_OTHER);
	CHECK(!p.text, "a service line follows service, not text or other");
	p = forward_parts("service", false, FWD_SERVICE);
	CHECK(p.text, "service on: the service line");

	printf("rich messages follow other\n");
	p = forward_parts("rich", false, FWD_TEXT);
	CHECK(!p.text, "text on, other off: no rich text");
	p = forward_parts("rich", false, FWD_OTHER);
	CHECK(p.text, "other on: the rich text");

	printf("a plain text message\n");
	p = forward_parts("text", false, FWD_TEXT);
	CHECK(p.text && !p.file, "text on: the text");
	p = forward_parts("text", false, all_but(FWD_TEXT));
	CHECK(!p.text && !p.summary, "text off: nothing");

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

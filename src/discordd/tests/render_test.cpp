// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * discordd_render_test -- the Telegram text a Discord message is rendered as
 * for a route's content types: embeds, polls, system messages, voice
 * messages and forwards' parts each under their own flag, and nothing at all
 * when the route takes none of a message's parts.
 */
#include "../Render.hpp"
#include "../RouteFilter.hpp"

#include <cstdio>
#include <string>

using namespace discordd;
using gwdiscord::Message;

static int g_failures;

static const unsigned ALL = FWD_STICKER | FWD_TEXT | FWD_PHOTO | FWD_VIDEO |
			    FWD_DOCUMENT | FWD_AUDIO | FWD_ANIMATION |
			    FWD_VOICE | FWD_EMBED | FWD_POLL | FWD_SYSTEM;

static void check(const char *what, const std::string &got,
		  const std::string &want)
{
	const bool ok = got == want;
	printf("  %s %s\n", ok ? "ok:  " : "FAIL:", what);
	if (!ok) {
		printf("    got:  \"%s\"\n    want: \"%s\"\n", got.c_str(),
		       want.c_str());
		g_failures++;
	}
}

static Message base(void)
{
	Message m;
	m.author.username = "alice";
	m.author.discriminator = "0";
	return m;
}

static void test_text_and_nothing(void)
{
	printf("text, and messages with nothing to send\n");

	Message m = base();
	m.content = "hello";
	check("text", render_message(m, ALL), "alice#0:\nhello");
	check("text filtered out", render_message(m, ALL & ~FWD_TEXT), "");

	/* Used to be "(no text)": now nothing at all. */
	const Message empty = base();
	check("empty message", render_message(empty, ALL), "");
	check("empty message, text only", render_message(empty, FWD_TEXT), "");

	/* An unfurled link embed adds nothing beside its link. */
	Message link = base();
	gwdiscord::Embed e;
	e.type = "link";
	e.url = "https://example.org/";
	e.title = "Example";
	link.embeds.push_back(e);
	check("only a link embed", render_message(link, ALL), "");
}

static void test_embeds(void)
{
	printf("embeds\n");

	Message m = base();
	m.content = "news";
	gwdiscord::Embed e;
	e.title = "Release 1.2";
	e.description = "Fixes";
	e.url = "https://example.org/1.2";
	e.author_name = "Maintainers";
	e.fields.push_back({"Changes", "12"});
	m.embeds.push_back(e);

	check("text and embed", render_message(m, ALL),
	      "alice#0:\nnews\nMaintainers\nRelease 1.2\nFixes\n"
	      "Changes: 12\nhttps://example.org/1.2");
	check("embed only", render_message(m, FWD_EMBED),
	      "alice#0:\nMaintainers\nRelease 1.2\nFixes\nChanges: 12\n"
	      "https://example.org/1.2");
	check("embed filtered out", render_message(m, ALL & ~FWD_EMBED),
	      "alice#0:\nnews");
	m.content.clear();
	check("embed-only message, embeds off",
	      render_message(m, ALL & ~FWD_EMBED), "");
}

static void test_polls(void)
{
	printf("polls\n");

	Message m = base();
	gwdiscord::Poll p;
	p.question = "Lunch?";
	p.answers = {"Pizza", "Soup"};
	m.poll = p;
	check("poll", render_message(m, ALL),
	      "alice#0:\nPoll: Lunch?\n- Pizza\n- Soup");
	check("poll filtered out", render_message(m, ALL & ~FWD_POLL), "");

	m.poll->allow_multiselect = true;
	m.poll->expiry = "2026-11-21T10:00:00.000000+00:00";
	check("poll with multiselect and expiry",
	      render_message(m, FWD_POLL),
	      "alice#0:\nPoll: Lunch? (multiple answers)\n- Pizza\n- Soup\n"
	      "Closes 2026-11-21 10:00 UTC");
}

static void test_system(void)
{
	printf("system messages\n");

	Message m = base();
	m.type = 7;
	check("member join", render_message(m, ALL),
	      "alice#0:\nJoined the server");
	check("member join, system off", render_message(m, ALL & ~FWD_SYSTEM),
	      "");
	check("member join, text only", render_message(m, FWD_TEXT), "");

	m.type = 18;
	m.content = "planning";
	check("thread created: content is the detail, under system",
	      render_message(m, FWD_SYSTEM),
	      "alice#0:\nStarted a thread: planning");
	check("thread created, text only", render_message(m, FWD_TEXT), "");

	m.type = 6;
	m.content.clear();
	check("pin", render_message(m, ALL), "alice#0:\nPinned a message");

	m.type = 8;
	m.content = "3";
	check("boost", render_message(m, ALL),
	      "alice#0:\nBoosted the server 3 times");

	m.type = 46;
	m.content.clear();
	gwdiscord::Embed e;
	e.type = "poll_result";
	e.fields.push_back({"poll_question_text", "Lunch?"});
	e.fields.push_back({"victor_answer_text", "Pizza"});
	m.embeds.push_back(e);
	check("poll result", render_message(m, ALL),
	      "alice#0:\nPoll ended: Lunch? (winner: Pizza)");

	m = base();
	m.type = 99;
	check("unknown type", render_message(m, ALL),
	      "alice#0:\nSystem message (type 99)");

	/* Replies and slash command answers are ordinary messages. */
	m = base();
	m.content = "pong";
	for (int t : {0, 19, 20, 23}) {
		m.type = t;
		check(("type " + std::to_string(t) + " is text").c_str(),
		      render_message(m, FWD_TEXT), "alice#0:\npong");
	}
}

static void test_voice(void)
{
	printf("voice messages\n");

	Message m = base();
	m.flags = Message::FLAG_IS_VOICE_MESSAGE;
	gwdiscord::Attachment a;
	a.filename = "voice-message.ogg";
	a.content_type = "audio/ogg";
	a.url = "https://cdn.example/v.ogg";
	a.duration_secs = 65.4;
	m.attachments.push_back(a);

	check("voice as a link line", render_message(m, FWD_VOICE),
	      "alice#0:\n[voice message, 1:05] https://cdn.example/v.ogg");
	check("voice, audio only", render_message(m, FWD_AUDIO), "");
	check("voice, recording skipped",
	      render_message(m, ALL, &m.attachments[0]), "");

	m.flags = 0;
	check("audio file", render_message(m, FWD_AUDIO),
	      "alice#0:\n[voice-message.ogg] https://cdn.example/v.ogg");
	check("audio file, voice only", render_message(m, FWD_VOICE), "");
}

/* What alice forwards: text, a photo, a sticker and a rich embed. */
static Message forward(void)
{
	Message m = base();
	gwdiscord::MessageReference ref;
	ref.type = gwdiscord::MessageReference::TYPE_FORWARD;
	ref.message_id = 42;
	m.reference = ref;

	gwdiscord::Snapshot s;
	s.content = "look at this";
	gwdiscord::Attachment a;
	a.filename = "dog.jpg";
	a.content_type = "image/jpeg";
	a.url = "https://cdn.example/dog.jpg";
	s.attachments.push_back(a);
	gwdiscord::Sticker st;
	st.id = 7;
	st.name = "Hi";
	st.format_type = gwdiscord::Sticker::FORMAT_PNG;
	s.stickers.push_back(st);
	gwdiscord::Embed e;
	e.title = "Note";
	e.description = "forwarded embed";
	s.embeds.push_back(e);
	m.snapshot = s;
	return m;
}

static void test_forwards(void)
{
	printf("forwarded messages\n");

	const Message m = forward();
	check("forward, everything", render_message(m, ALL),
	      "alice#0:\nForwarded:\nlook at this\n"
	      "[dog.jpg] https://cdn.example/dog.jpg\n"
	      "[sticker: Hi] https://cdn.discordapp.com/stickers/7.png\n"
	      "Note\nforwarded embed");
	check("forward, text off", render_message(m, ALL & ~FWD_TEXT),
	      "alice#0:\nForwarded:\n"
	      "[dog.jpg] https://cdn.example/dog.jpg\n"
	      "[sticker: Hi] https://cdn.discordapp.com/stickers/7.png\n"
	      "Note\nforwarded embed");
	check("forward, text only", render_message(m, FWD_TEXT),
	      "alice#0:\nForwarded:\nlook at this");
	check("forward, photos only", render_message(m, FWD_PHOTO),
	      "alice#0:\nForwarded:\n[dog.jpg] https://cdn.example/dog.jpg");
	check("forward, embeds only", render_message(m, FWD_EMBED),
	      "alice#0:\nForwarded:\nNote\nforwarded embed");
	check("forward, nothing it holds passes",
	      render_message(m, FWD_VIDEO | FWD_POLL | FWD_SYSTEM), "");

	/* A forwarded voice message: a link line, never a voice note. */
	Message v = base();
	v.reference = m.reference;
	gwdiscord::Snapshot s;
	s.flags = Message::FLAG_IS_VOICE_MESSAGE;
	gwdiscord::Attachment a;
	a.filename = "voice-message.ogg";
	a.content_type = "audio/ogg";
	a.url = "https://cdn.example/v.ogg";
	a.duration_secs = 12;
	s.attachments.push_back(a);
	v.snapshot = s;
	check("forwarded voice message", render_message(v, FWD_VOICE),
	      "alice#0:\nForwarded:\n[voice message, 0:12] "
	      "https://cdn.example/v.ogg");
	check("forwarded voice message, audio only",
	      render_message(v, FWD_AUDIO), "");

	/* What is archived as the forward's text. */
	check("archived text of a forward", archived_content(m),
	      "look at this");
	Message own = base();
	own.content = "mine";
	check("archived text of a message", archived_content(own), "mine");
}

int main(void)
{
	test_text_and_nothing();
	test_embeds();
	test_polls();
	test_system();
	test_voice();
	test_forwards();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

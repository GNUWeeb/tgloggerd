// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * discordd_voice_note_test -- the pure parts of sending a Discord voice
 * message as a Telegram voice note: which attachment goes, the base64
 * waveform decode, its packing into Telegram's 5-bit format, and the CDN URL
 * split. The send itself needs Telegram and is not covered here.
 */
#include "../RouteFilter.hpp"
#include "../VoiceNote.hpp"

#include <cstdio>
#include <string>

using namespace discordd;
using gwdiscord::Message;

static int g_failures;

static void check(const char *what, bool ok)
{
	printf("  %s %s\n", ok ? "ok:  " : "FAIL:", what);
	if (!ok)
		g_failures++;
}

static void test_attachment(void)
{
	printf("which attachment is sent\n");

	Message m;
	m.flags = Message::FLAG_IS_VOICE_MESSAGE;
	gwdiscord::Attachment a;
	a.content_type = "audio/ogg";
	a.duration_secs = 2;
	m.attachments.push_back(a);

	check("voice message, voice on",
	      voice_note_attachment(m, FWD_VOICE) == &m.attachments[0]);
	check("voice message, voice off",
	      voice_note_attachment(m, FWD_AUDIO | FWD_TEXT) == nullptr);
	m.flags = 0;
	check("audio file", voice_note_attachment(m, FWD_VOICE) == nullptr);
}

static void test_base64(void)
{
	printf("base64\n");

	std::string out;
	check("padded", base64_decode("QUJD", &out) && out == "ABC");
	check("one pad", base64_decode("QUI=", &out) && out == "AB");
	check("two pads", base64_decode("QQ==", &out) && out == "A");
	check("unpadded", base64_decode("QQ", &out) && out == "A");
	check("empty", base64_decode("", &out) && out.empty());
	check("high bytes", base64_decode("/+8=", &out) && out == "\xff\xef");
	check("bad character", !base64_decode("QU*D", &out));
	check("data after padding", !base64_decode("QQ==QQ==", &out));
	check("dangling character", !base64_decode("QUJDR", &out));
	check("three pads", !base64_decode("Q===", &out));
}

/* Sample @i of a packed 5-bit waveform. */
static unsigned sample(const std::string &w, size_t i)
{
	const size_t bit = i * 5;
	unsigned v = (unsigned char)w[bit / 8];
	if (bit / 8 + 1 < w.size())
		v |= (unsigned)(unsigned char)w[bit / 8 + 1] << 8;
	return (v >> (bit % 8)) & 0x1f;
}

static void test_waveform(void)
{
	printf("waveform\n");

	/* 0, 255, 128, 255: 0, 31, 16, 31. */
	const std::string w = telegram_waveform("AP+A/w==");
	check("4 samples in 3 bytes", w.size() == 3);
	check("samples scaled to 5 bits",
	      w.size() == 3 && sample(w, 0) == 0 && sample(w, 1) == 31 &&
		      sample(w, 2) == 16 && sample(w, 3) == 31);
	check("packed least significant bit first",
	      w.size() == 3 && (unsigned char)w[0] == 0xe0 &&
		      (unsigned char)w[1] == 0xc3 &&
		      (unsigned char)w[2] == 0x0f);

	/* 256 samples, as Discord sends: down to 100, each bucket's peak. */
	std::string amp(256, '\0');
	amp[255] = (char)255;
	std::string b64;
	static const char *tbl =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	for (size_t i = 0; i < amp.size(); i += 3) {
		unsigned v = (unsigned char)amp[i] << 16;
		const size_t n = amp.size() - i < 3 ? amp.size() - i : 3;
		if (n > 1)
			v |= (unsigned char)amp[i + 1] << 8;
		if (n > 2)
			v |= (unsigned char)amp[i + 2];
		b64 += tbl[(v >> 18) & 63];
		b64 += tbl[(v >> 12) & 63];
		b64 += n > 1 ? tbl[(v >> 6) & 63] : '=';
		b64 += n > 2 ? tbl[v & 63] : '=';
	}
	const std::string w2 = telegram_waveform(b64);
	check("256 samples become 100 in 63 bytes", w2.size() == 63);
	check("the last bucket keeps its peak",
	      w2.size() == 63 && sample(w2, 99) == 31 && sample(w2, 0) == 0 &&
		      sample(w2, 98) == 0);

	check("bad base64 gives no waveform",
	      telegram_waveform("not base64!").empty());
	check("empty gives no waveform", telegram_waveform("").empty());
}

static void test_url(void)
{
	printf("CDN URLs\n");

	std::string host, target;
	check("attachment URL",
	      split_https_url("https://cdn.discordapp.com/attachments/1/2/"
			      "voice-message.ogg?ex=1&is=2&hm=3",
			      &host, &target) &&
		      host == "cdn.discordapp.com" &&
		      target == "/attachments/1/2/voice-message.ogg?ex=1&is=2"
				"&hm=3");
	check("bare host", split_https_url("https://example.org", &host,
					   &target) &&
				   host == "example.org" && target == "/");
	check("query without path",
	      split_https_url("https://example.org?a=1", &host, &target) &&
		      target == "/?a=1");
	check("fragment dropped",
	      split_https_url("https://example.org/a#b", &host, &target) &&
		      target == "/a");
	check("http refused", !split_https_url("http://example.org/a", &host,
					       &target));
	check("port refused", !split_https_url("https://example.org:8443/a",
					       &host, &target));
	check("credentials refused",
	      !split_https_url("https://u:p@example.org/a", &host, &target));
	check("empty host refused",
	      !split_https_url("https:///a", &host, &target));
}

int main(void)
{
	test_attachment();
	test_base64();
	test_waveform();
	test_url();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}

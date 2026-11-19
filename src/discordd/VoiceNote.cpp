// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "VoiceNote.hpp"

#include "RouteFilter.hpp"

#include <algorithm>
#include <vector>

namespace discordd {

const gwdiscord::Attachment *voice_note_attachment(const gwdiscord::Message &m,
						   unsigned types)
{
	if (!m.is_voice_message() || !(types & FWD_VOICE))
		return nullptr;
	for (const auto &a : m.attachments) {
		if (attachment_type(m, a) == FWD_VOICE)
			return &a;
	}
	return nullptr;
}

bool base64_decode(const std::string &in, std::string *out)
{
	std::string res;
	unsigned acc = 0;
	int bits = 0;
	size_t pad = 0;

	for (char c : in) {
		int v;
		if (c >= 'A' && c <= 'Z')
			v = c - 'A';
		else if (c >= 'a' && c <= 'z')
			v = c - 'a' + 26;
		else if (c >= '0' && c <= '9')
			v = c - '0' + 52;
		else if (c == '+')
			v = 62;
		else if (c == '/')
			v = 63;
		else if (c == '=') {
			pad++;
			continue;
		} else
			return false;
		if (pad)
			return false; /* data after padding */
		acc = (acc << 6) | (unsigned)v;
		bits += 6;
		if (bits >= 8) {
			bits -= 8;
			res.push_back((char)((acc >> bits) & 0xff));
		}
	}
	/* Leftover bits must be zero padding of less than a byte. */
	if (pad > 2 || bits >= 6 || (acc & ((1u << bits) - 1)))
		return false;
	if (pad && (in.size() % 4) != 0)
		return false;
	*out = std::move(res);
	return true;
}

std::string telegram_waveform(const std::string &b64)
{
	std::string amp;
	if (!base64_decode(b64, &amp) || amp.empty())
		return std::string();

	/* Telegram's clients draw 100 samples; keep each bucket's peak. */
	const size_t n = std::min<size_t>(100, amp.size());
	std::vector<unsigned> samples(n);
	for (size_t j = 0; j < n; j++) {
		const size_t from = j * amp.size() / n;
		const size_t to = (j + 1) * amp.size() / n;
		unsigned peak = 0;
		for (size_t i = from; i < to; i++)
			peak = std::max(peak, (unsigned)(unsigned char)amp[i]);
		samples[j] = (peak * 31 + 127) / 255; /* 0..255 -> 0..31 */
	}

	std::string out((n * 5 + 7) / 8, '\0');
	for (size_t j = 0; j < n; j++) {
		const size_t bit = j * 5;
		const unsigned v = (samples[j] & 0x1f) << (bit % 8);
		out[bit / 8] = (char)((unsigned char)out[bit / 8] | (v & 0xff));
		if ((v >> 8) && bit / 8 + 1 < out.size())
			out[bit / 8 + 1] = (char)((unsigned char)out[bit / 8 + 1] |
						  (v >> 8));
	}
	return out;
}

bool split_https_url(const std::string &url, std::string *host,
		     std::string *target)
{
	static const std::string scheme = "https://";
	if (url.compare(0, scheme.size(), scheme) != 0)
		return false;
	const size_t start = scheme.size();
	const size_t slash = url.find_first_of("/?#", start);
	const std::string h = url.substr(start, slash == std::string::npos
							? std::string::npos
							: slash - start);
	/* A bare host name: no port, no credentials. */
	if (h.empty() || h.find_first_of(":@") != std::string::npos)
		return false;
	*host = h;
	if (slash == std::string::npos)
		*target = "/";
	else if (url[slash] == '/')
		*target = url.substr(slash, url.find('#', slash) - slash);
	else if (url[slash] == '?')
		*target = "/" + url.substr(slash, url.find('#', slash) - slash);
	else
		*target = "/";
	return true;
}

} /* namespace discordd */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__VOICE_NOTE_HPP
#define DISCORDD__VOICE_NOTE_HPP

#include <gwdiscord/Events.hpp>

#include <cstdint>
#include <string>

namespace discordd {

/*
 * The largest recording sent as a voice note; a larger one goes as a link
 * line. Discord's voice messages are small Opus files, and this is also the
 * most gwdiscord's HTTP client reads of one response body.
 */
constexpr uint64_t MAX_VOICE_NOTE_BYTES = 8ULL << 20;

/*
 * The recording of @m to send as a voice note on a route forwarding @types,
 * or nullptr: @m must be a voice message, and the route must take FWD_VOICE.
 */
const gwdiscord::Attachment *voice_note_attachment(const gwdiscord::Message &m,
						   unsigned types);

/* Decode standard base64 (padding optional); false on anything else. */
bool base64_decode(const std::string &in, std::string *out);

/*
 * Telegram's waveform for Discord's @b64 (base64 of one-byte amplitudes):
 * at most 100 samples of 5 bits each, packed least significant bit first.
 * "" when @b64 does not decode, which makes Telegram draw a flat line.
 */
std::string telegram_waveform(const std::string &b64);

/* Split "https://host/path?query" into host and target; false otherwise. */
bool split_https_url(const std::string &url, std::string *host,
		     std::string *target);

} /* namespace discordd */

#endif /* #ifndef DISCORDD__VOICE_NOTE_HPP */

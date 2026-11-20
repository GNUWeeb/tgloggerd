// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef DISCORDD__RENDER_HPP
#define DISCORDD__RENDER_HPP

#include <gwdiscord/Events.hpp>

#include <string>

namespace discordd {

/*
 * Who a forwarded message is from: username#discriminator, the stable handle
 * of the account ("unknown" when Discord sent none).
 */
std::string author_handle(const gwdiscord::Message &m);

/*
 * Whether @m is a system message (a member joining, a pin, a boost, ...),
 * which is forwarded as one line under FWD_SYSTEM.
 */
bool is_system_message(const gwdiscord::Message &m);

/* The one line a system message is forwarded as. */
std::string system_line(const gwdiscord::Message &m);

/*
 * Render the parts of @m a route forwarding @types (FWD_* bits) takes, as
 * the Telegram text to send: the author line, then one part after the
 * other; for a forward, a "Forwarded:" line and the parts of the message
 * it forwarded, each filtered as if that had been posted here. "" when the
 * route takes none of them, so it gets nothing at all.
 * @skip, when given, is an attachment of @m to leave out, because it goes
 * across some other way (a voice message's recording, as a voice note).
 */
std::string render_message(const gwdiscord::Message &m, unsigned types,
			   const gwdiscord::Attachment *skip = nullptr);

/*
 * The text archived for @m: its content, or for a forward, which has none of
 * its own, the content of the message it forwarded.
 */
std::string archived_content(const gwdiscord::Message &m);

} /* namespace discordd */

#endif /* #ifndef DISCORDD__RENDER_HPP */

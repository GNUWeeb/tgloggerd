// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "Render.hpp"

#include "RouteFilter.hpp"

#include <cmath>
#include <cstdio>

namespace discordd {

namespace {

/* Append @line to @body on a line of its own; an empty @line adds nothing. */
void add_line(std::string &body, const std::string &line)
{
	if (line.empty())
		return;
	if (!body.empty() && body.back() != '\n')
		body += "\n";
	body += line;
}

/* "1:05" for 65.3 seconds. */
std::string duration_str(double secs)
{
	const long s = secs > 0 ? std::lround(secs) : 0;
	char buf[32];
	snprintf(buf, sizeof(buf), "%ld:%02ld", s / 60, s % 60);
	return buf;
}

/* "2026-11-21 10:00 UTC" for Discord's "2026-11-21T10:00:00.000000+00:00". */
std::string time_str(const std::string &iso)
{
	if (iso.size() < 16 || iso[10] != 'T')
		return iso;
	std::string out = iso.substr(0, 10) + " " + iso.substr(11, 5);
	if (iso.compare(iso.size() - 6, 6, "+00:00") == 0 || iso.back() == 'Z')
		out += " UTC";
	return out;
}

/* Lines of a bot's or webhook's embed: author, title, text, fields, URL. */
std::string embed_text(const gwdiscord::Embed &e)
{
	std::string out;
	add_line(out, e.author_name);
	add_line(out, e.title);
	add_line(out, e.description);
	for (const auto &f : e.fields) {
		if (f.name.empty())
			add_line(out, f.value);
		else if (f.value.empty())
			add_line(out, f.name);
		else
			add_line(out, f.name + ": " + f.value);
	}
	add_line(out, e.url);
	return out;
}

/* "Poll: <question>", one "- <answer>" line each, and when it closes. */
std::string poll_text(const gwdiscord::Poll &p)
{
	std::string out = "Poll: " + p.question;
	if (p.allow_multiselect)
		out += " (multiple answers)";
	for (const auto &a : p.answers)
		add_line(out, "- " + a);
	if (!p.expiry.empty())
		add_line(out, "Closes " + time_str(p.expiry));
	return out;
}

/* A field of the poll_result embed a POLL_RESULT message carries. */
std::string poll_result_field(const gwdiscord::Message &m, const char *name)
{
	for (const auto &e : m.embeds) {
		if (e.type != "poll_result")
			continue;
		for (const auto &f : e.fields)
			if (f.name == name)
				return f.value;
	}
	return std::string();
}

/*
 * The parts of an ordinary (not a system) message @m that a route forwarding
 * @types takes, one after the other, without the author line; @skip as for
 * render_message().
 */
std::string parts_text(const gwdiscord::Message &m, unsigned types,
		       const gwdiscord::Attachment *skip)
{
	std::string body;

	if (types & FWD_TEXT)
		body = m.content;

	/* Media goes across as a link line each. */
	for (const auto &a : m.attachments) {
		if (&a == skip)
			continue;
		const unsigned t = attachment_type(m, a);
		if (!(types & t))
			continue;
		if (t == FWD_VOICE)
			add_line(body, "[voice message, " +
					       duration_str(a.duration_secs) +
					       "] " + a.url);
		else
			add_line(body, "[" + a.filename + "] " + a.url);
	}
	for (const auto &st : m.stickers) {
		if (types & FWD_STICKER)
			add_line(body, "[sticker: " + st.name + "] " +
					       st.url());
	}

	/*
	 * Only a bot's or webhook's own embeds: the others are Discord
	 * unfurling a link the content carries already, and Telegram shows a
	 * preview of that link itself.
	 */
	for (const auto &e : m.embeds) {
		if ((types & FWD_EMBED) && e.is_rich())
			add_line(body, embed_text(e));
	}
	if (m.poll && (types & FWD_POLL))
		add_line(body, poll_text(*m.poll));
	return body;
}

/*
 * What forward @m forwarded, as a message of its own, so that its parts are
 * filtered and rendered exactly as an ordinary message's: its flags come
 * along, so a forwarded voice message's recording still follows FWD_VOICE.
 */
gwdiscord::Message snapshot_message(const gwdiscord::Snapshot &s)
{
	gwdiscord::Message m;
	m.content = s.content;
	m.timestamp = s.timestamp;
	m.type = s.type;
	m.flags = s.flags;
	m.attachments = s.attachments;
	m.stickers = s.stickers;
	m.embeds = s.embeds;
	return m;
}

/* @what, then ": " and @detail when there is one. */
std::string with_detail(const char *what, const std::string &detail)
{
	return detail.empty() ? std::string(what)
			      : std::string(what) + ": " + detail;
}

} /* namespace */

std::string author_handle(const gwdiscord::Message &m)
{
	/*
	 * Not the display name: that is free text anyone can set to anything
	 * (and is often long), whereas username#discriminator identifies the
	 * account. Discord moved most accounts to a "0" discriminator, which
	 * is kept rather than hidden so the rendering is uniform.
	 */
	std::string who = m.author.username;
	if (who.empty())
		who = "unknown";
	if (!m.author.discriminator.empty())
		who += "#" + m.author.discriminator;
	return who;
}

bool is_system_message(const gwdiscord::Message &m)
{
	/*
	 * DEFAULT and REPLY are ordinary messages, and so are a bot's answers
	 * to a slash command (CHAT_INPUT_COMMAND, 20) and to a context menu
	 * command (CONTEXT_MENU_COMMAND, 23): their content is the bot's own
	 * text, which follows FWD_TEXT like any other.
	 */
	switch (m.type) {
	case gwdiscord::Message::TYPE_DEFAULT:
	case gwdiscord::Message::TYPE_REPLY:
	case 20:
	case 23:
		return false;
	default:
		return true;
	}
}

std::string system_line(const gwdiscord::Message &m)
{
	/*
	 * Discord's message types. The content of a system message, when it
	 * has one, is the event's detail: a channel's or a thread's new name,
	 * a stage's topic, a boost count.
	 */
	const std::string &c = m.content;
	switch (m.type) {
	case 1:  return "Added a member to the group";
	case 2:  return "Removed a member from the group";
	case 3:  return "Started a call";
	case 4:  return with_detail("Changed the channel name", c);
	case 5:  return "Changed the channel icon";
	case 6:  return "Pinned a message";
	case 7:  return "Joined the server";
	case 8:
		return c.empty() || c == "1"
			       ? "Boosted the server"
			       : "Boosted the server " + c + " times";
	case 9:  return "Boosted the server to level 1";
	case 10: return "Boosted the server to level 2";
	case 11: return "Boosted the server to level 3";
	case 12: return with_detail("Followed a channel here", c);
	case 18: return with_detail("Started a thread", c);
	case 21: return "Started a thread from this message";
	case 22: return "Invite reminder";
	case 24: return "AutoMod blocked a message";
	case 25: return "Bought a role subscription";
	case 27: return with_detail("Started a stage", c);
	case 28: return with_detail("Ended the stage", c);
	case 29: return "Became a stage speaker";
	case 31: return with_detail("Changed the stage topic", c);
	case 36: return "Paused the server's invites and DMs";
	case 37: return "Resumed the server's invites and DMs";
	case 38: return "Reported a raid";
	case 39: return "Reported a false alarm";
	case 46: {
		const std::string q = poll_result_field(m, "poll_question_text");
		const std::string a = poll_result_field(m, "victor_answer_text");
		std::string out = with_detail("Poll ended", q);
		if (!a.empty())
			out += " (winner: " + a + ")";
		return out;
	}
	default:
		return "System message (type " + std::to_string(m.type) + ")";
	}
}

std::string render_message(const gwdiscord::Message &m, unsigned types,
			   const gwdiscord::Attachment *skip)
{
	/*
	 * Plain text. Telegram entity formatting is deliberately not
	 * attempted yet -- Discord markdown would have to be translated, and
	 * getting that wrong is worse than sending it verbatim.
	 */
	std::string body;

	if (is_system_message(m)) {
		/* The event and its detail are one part, under FWD_SYSTEM. */
		if (types & FWD_SYSTEM)
			body = system_line(m);
	} else {
		body = parts_text(m, types, skip);
	}

	/*
	 * A forward: a "Forwarded:" line, then the forwarded message's parts,
	 * each under the flag it would follow if it had been posted here.
	 * The line comes only with a part the route takes, never alone. The
	 * forwarded message is rendered as an ordinary one, whatever its type.
	 */
	if (m.snapshot) {
		const std::string fwd =
			parts_text(snapshot_message(*m.snapshot), types, nullptr);
		if (!fwd.empty()) {
			add_line(body, "Forwarded:");
			add_line(body, fwd);
		}
	}

	/*
	 * Nothing the route takes, or nothing at all (a message whose parts
	 * gwdiscord does not parse): send nothing rather than an empty
	 * message with only its author.
	 */
	if (body.empty())
		return std::string();
	return author_handle(m) + ":\n" + body;
}

std::string archived_content(const gwdiscord::Message &m)
{
	if (m.content.empty() && m.snapshot)
		return m.snapshot->content;
	return m.content;
}

} /* namespace discordd */

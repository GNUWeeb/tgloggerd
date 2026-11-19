// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "EventParse.hpp"

#include <cstdlib>
#include <memory>

namespace gwdiscord {

Snowflake parse_snowflake(const Json::Value &v)
{
	/*
	 * Ids arrive as strings because they do not survive a double. Accept a
	 * numeric form too, in case a mock or a future endpoint sends one.
	 */
	if (v.isString()) {
		const std::string s = v.asString();
		if (s.empty())
			return 0;
		return strtoull(s.c_str(), nullptr, 10);
	}
	if (v.isUInt64())
		return v.asUInt64();
	if (v.isInt64() && v.asInt64() >= 0)
		return (Snowflake)v.asInt64();
	return 0;
}

std::string json_str(const Json::Value &v, const char *key)
{
	if (!v.isObject() || !v.isMember(key))
		return std::string();
	const Json::Value &f = v[key];
	return f.isString() ? f.asString() : std::string();
}

User parse_user(const Json::Value &v)
{
	User u;
	if (!v.isObject())
		return u;
	u.id = parse_snowflake(v["id"]);
	u.username = json_str(v, "username");
	u.global_name = json_str(v, "global_name");
	u.discriminator = json_str(v, "discriminator");
	u.avatar = json_str(v, "avatar");
	u.bot = v.isMember("bot") && v["bot"].isBool() && v["bot"].asBool();
	return u;
}

Attachment parse_attachment(const Json::Value &v)
{
	Attachment a;
	if (!v.isObject())
		return a;
	a.id = parse_snowflake(v["id"]);
	a.filename = json_str(v, "filename");
	a.content_type = json_str(v, "content_type");
	if (v.isMember("size") && v["size"].isIntegral())
		a.size = v["size"].asUInt64();
	a.url = json_str(v, "url");
	a.proxy_url = json_str(v, "proxy_url");
	if (v.isMember("width") && v["width"].isIntegral())
		a.width = v["width"].asInt();
	if (v.isMember("height") && v["height"].isIntegral())
		a.height = v["height"].asInt();
	if (v.isMember("duration_secs") && v["duration_secs"].isNumeric())
		a.duration_secs = v["duration_secs"].asDouble();
	a.waveform = json_str(v, "waveform");
	return a;
}

Sticker parse_sticker(const Json::Value &v)
{
	Sticker s;
	if (!v.isObject())
		return s;
	s.id = parse_snowflake(v["id"]);
	s.name = json_str(v, "name");
	if (v.isMember("format_type") && v["format_type"].isIntegral())
		s.format_type = v["format_type"].asInt();
	return s;
}

Embed parse_embed(const Json::Value &v)
{
	Embed e;
	if (!v.isObject())
		return e;
	e.type = json_str(v, "type");
	e.title = json_str(v, "title");
	e.description = json_str(v, "description");
	e.url = json_str(v, "url");
	e.author_name = json_str(v["author"], "name");
	if (v.isMember("fields") && v["fields"].isArray()) {
		for (const auto &f : v["fields"]) {
			Embed::Field ef;
			ef.name = json_str(f, "name");
			ef.value = json_str(f, "value");
			e.fields.push_back(std::move(ef));
		}
	}
	return e;
}

Poll parse_poll(const Json::Value &v)
{
	Poll p;
	if (!v.isObject())
		return p;
	/* The question and each answer are "poll media": text and emoji. */
	p.question = json_str(v["question"], "text");
	if (v.isMember("answers") && v["answers"].isArray()) {
		for (const auto &a : v["answers"])
			p.answers.push_back(json_str(a["poll_media"], "text"));
	}
	p.allow_multiselect = v.isMember("allow_multiselect") &&
			      v["allow_multiselect"].isBool() &&
			      v["allow_multiselect"].asBool();
	p.expiry = json_str(v, "expiry");
	return p;
}

Message parse_message(const Json::Value &v)
{
	Message m;
	if (!v.isObject())
		return m;

	m.id = parse_snowflake(v["id"]);
	m.channel_id = parse_snowflake(v["channel_id"]);
	m.guild_id = parse_snowflake(v["guild_id"]);
	m.author = parse_user(v["author"]);
	m.webhook_id = parse_snowflake(v["webhook_id"]);
	m.content = json_str(v, "content");
	m.timestamp = json_str(v, "timestamp");
	m.edited_timestamp = json_str(v, "edited_timestamp");
	if (v.isMember("type") && v["type"].isInt())
		m.type = v["type"].asInt();
	if (v.isMember("flags") && v["flags"].isUInt64())
		m.flags = v["flags"].asUInt64();

	if (v.isMember("attachments") && v["attachments"].isArray()) {
		for (const auto &a : v["attachments"])
			m.attachments.push_back(parse_attachment(a));
	}

	if (v.isMember("sticker_items") && v["sticker_items"].isArray()) {
		for (const auto &st : v["sticker_items"])
			m.stickers.push_back(parse_sticker(st));
	}

	if (v.isMember("embeds") && v["embeds"].isArray()) {
		for (const auto &e : v["embeds"])
			m.embeds.push_back(parse_embed(e));
	}

	if (v.isMember("poll") && v["poll"].isObject())
		m.poll = parse_poll(v["poll"]);

	if (v.isMember("message_reference") &&
	    v["message_reference"].isObject()) {
		const Json::Value &r = v["message_reference"];
		MessageReference ref;
		ref.message_id = parse_snowflake(r["message_id"]);
		ref.channel_id = parse_snowflake(r["channel_id"]);
		ref.guild_id = parse_snowflake(r["guild_id"]);
		/*
		 * A reference with no message_id is a forward/crosspost
		 * pointer, not a reply; only surface real replies.
		 */
		if (ref.message_id != 0)
			m.reference = ref;
	}
	return m;
}

MessageDelete parse_message_delete(const Json::Value &v)
{
	MessageDelete d;
	if (!v.isObject())
		return d;
	d.id = parse_snowflake(v["id"]);
	d.channel_id = parse_snowflake(v["channel_id"]);
	d.guild_id = parse_snowflake(v["guild_id"]);
	return d;
}

Ready parse_ready(const Json::Value &v)
{
	Ready r;
	if (!v.isObject())
		return r;
	r.user = parse_user(v["user"]);
	r.session_id = json_str(v, "session_id");
	r.resume_gateway_url = json_str(v, "resume_gateway_url");
	return r;
}

bool parse_json(const std::string &raw, Json::Value &out, std::string *err)
{
	Json::CharReaderBuilder rb;
	std::unique_ptr<Json::CharReader> r(rb.newCharReader());
	std::string e;

	if (!r->parse(raw.data(), raw.data() + raw.size(), &out, &e)) {
		if (err)
			*err = e;
		return false;
	}
	return true;
}

std::string dump_json(const Json::Value &v)
{
	Json::StreamWriterBuilder b;
	b["indentation"] = "";
	return Json::writeString(b, v);
}

} /* namespace gwdiscord */

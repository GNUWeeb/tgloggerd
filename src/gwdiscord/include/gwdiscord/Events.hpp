// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef GWDISCORD__EVENTS_HPP
#define GWDISCORD__EVENTS_HPP

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

namespace gwdiscord {

/*
 * Discord serialises every snowflake id as a JSON *string*, because the values
 * exceed what a double can represent exactly. We parse them to uint64_t once,
 * here, so callers never handle stringly-typed ids. 0 means "absent".
 */
using Snowflake = uint64_t;

/* Creation time of any snowflake, in milliseconds since the Unix epoch. */
constexpr uint64_t DISCORD_EPOCH_MS = 1420070400000ULL;
inline uint64_t snowflake_created_ms(Snowflake id)
{
	return (id >> 22) + DISCORD_EPOCH_MS;
}

struct User {
	Snowflake	id = 0;
	std::string	username;
	std::string	global_name;	/* display name; may be empty */
	std::string	discriminator;	/* "0" for migrated accounts */
	std::string	avatar;		/* avatar hash; may be empty */
	bool		bot = false;
};

struct Attachment {
	Snowflake	id = 0;
	std::string	filename;
	std::string	content_type;	/* may be empty */
	uint64_t	size = 0;
	/*
	 * CDN URLs are HMAC-signed and EXPIRE (the ex/is/hm query params).
	 * Fetch the bytes promptly; the only supported refresh is re-reading
	 * the message from the REST API.
	 */
	std::string	url;
	std::string	proxy_url;
	int		width = 0;	/* images only; 0 if not applicable */
	int		height = 0;
	/*
	 * Set on a voice message's recording only: its length, and its
	 * waveform as Discord sends it (base64 of up to 256 one-byte
	 * amplitude samples), kept undecoded.
	 */
	double		duration_secs = 0;
	std::string	waveform;
};

/*
 * A sticker attached to a message (the message's `sticker_items`). Discord
 * sends only this stub with a message, not the full sticker object.
 */
struct Sticker {
	enum Format {
		FORMAT_PNG    = 1,
		FORMAT_APNG   = 2,
		FORMAT_LOTTIE = 3,	/* a JSON animation, not an image */
		FORMAT_GIF    = 4,
	};

	Snowflake	id = 0;
	std::string	name;
	int		format_type = 0;	/* a Format; 0 if not sent */

	/*
	 * Where the sticker's file is served. Unlike an attachment URL this
	 * is not signed and does not expire. GIF stickers are served only by
	 * the media proxy; the rest come from the CDN.
	 */
	std::string url(void) const
	{
		const std::string sid = std::to_string(id);

		switch (format_type) {
		case FORMAT_GIF:
			return "https://media.discordapp.net/stickers/" + sid +
			       ".gif";
		case FORMAT_LOTTIE:
			return "https://cdn.discordapp.com/stickers/" + sid +
			       ".json";
		default:
			return "https://cdn.discordapp.com/stickers/" + sid +
			       ".png";
		}
	}
};

/*
 * An embed, with only the parts a text bridge renders. Bots and webhooks post
 * "rich" embeds; Discord itself adds the other types ("link", "image",
 * "video", "article", ...) when it unfurls a link in the content.
 */
struct Embed {
	struct Field {
		std::string	name;
		std::string	value;
	};

	std::string		type;		/* "rich" when absent */
	std::string		title;
	std::string		description;
	std::string		url;
	std::string		author_name;
	std::vector<Field>	fields;

	bool is_rich(void) const { return type.empty() || type == "rich"; }
};

/* A poll attached to a message, without its results. */
struct Poll {
	std::string		question;
	std::vector<std::string> answers;	/* each answer's text */
	bool			allow_multiselect = false;
	std::string		expiry;	/* ISO-8601, as sent; empty if none */
};

/* Where a reply points. Present only when the message is a reply. */
struct MessageReference {
	Snowflake	message_id = 0;
	Snowflake	channel_id = 0;
	Snowflake	guild_id = 0;
};

struct Message {
	/* The message types a bridge tells apart; the full list is Discord's. */
	enum Type {
		TYPE_DEFAULT = 0,
		TYPE_REPLY   = 19,
	};

	/* Message flags (a bit field); only those a caller reads. */
	enum Flag : uint64_t {
		FLAG_IS_VOICE_MESSAGE = 1ULL << 13,
	};

	Snowflake	id = 0;
	Snowflake	channel_id = 0;
	Snowflake	guild_id = 0;	/* 0 for a DM */
	User		author;
	/*
	 * Non-zero when the message was posted by a webhook rather than by a
	 * user or a bot. This is the discriminator a bridge uses to avoid
	 * re-forwarding its own traffic: note that a BOT's own message has
	 * author.bot == true but webhook_id == 0, so a bridge that also posts
	 * as a bot must additionally skip its own author id.
	 */
	Snowflake	webhook_id = 0;
	std::string	content;	/* empty without the MESSAGE_CONTENT intent */
	std::string	timestamp;	/* ISO-8601, as sent */
	std::string	edited_timestamp; /* empty when never edited */
	/*
	 * The message type: anything but TYPE_DEFAULT and TYPE_REPLY is
	 * mostly a system message (a member joining, a pin, a boost, ...).
	 */
	int		type = TYPE_DEFAULT;
	uint64_t	flags = 0;	/* Flag bits */
	std::vector<Attachment>		attachments;
	std::vector<Sticker>		stickers;
	std::vector<Embed>		embeds;
	std::optional<Poll>		poll;
	std::optional<MessageReference>	reference;

	bool from_webhook(void) const { return webhook_id != 0; }
	bool is_reply(void) const { return reference.has_value(); }
	bool is_voice_message(void) const
	{
		return (flags & FLAG_IS_VOICE_MESSAGE) != 0;
	}
};

/*
 * MESSAGE_DELETE carries ONLY these ids -- never the content. A consumer that
 * wants to render what was deleted must have persisted it at ingest time.
 */
struct MessageDelete {
	Snowflake	id = 0;
	Snowflake	channel_id = 0;
	Snowflake	guild_id = 0;
};

struct Ready {
	User		user;
	std::string	session_id;
	std::string	resume_gateway_url;
};

} /* namespace gwdiscord */

#endif /* #ifndef GWDISCORD__EVENTS_HPP */

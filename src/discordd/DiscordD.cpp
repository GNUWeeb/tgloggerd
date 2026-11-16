// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "DiscordD.hpp"

#include "DB.hpp"
#include "TelegramSender.hpp"

#include <gwdiscord/Gateway.hpp>

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstdio>
#include <map>
#include <mutex>
#include <thread>
#include <vector>

namespace discordd {

enum { LOG_ERROR = 0, LOG_WARN = 1, LOG_INFO = 2, LOG_DEBUG = 3 };

static const char *lvl_name(int lvl)
{
	switch (lvl) {
	case LOG_ERROR: return "error";
	case LOG_WARN:  return "warn";
	case LOG_INFO:  return "info";
	default:        return "debug";
	}
}

struct DiscordD::Impl {
	Config				cfg;
	std::unique_ptr<DB>		db;
	std::unique_ptr<TelegramSender>	tg;
	std::unique_ptr<gwdiscord::Gateway> gw;

	/* Route cache, reloaded periodically so web/DB edits take effect. */
	std::mutex				routes_mtx;
	std::map<uint64_t, std::vector<Route>>	routes_by_channel;
	/* Bot user ids we send as; used to ignore our own echo. */
	std::vector<int64_t>			our_bot_user_ids;

	std::thread			refresh_thr;
	std::atomic<bool>		stopping{false};
	std::mutex			wake_mtx;
	std::condition_variable		wake_cv;

	void log(int lvl, const std::string &msg) const
	{
		if (lvl > cfg.log_level)
			return;
		fprintf(stderr, "[discordd/%s] %s\n", lvl_name(lvl),
			msg.c_str());
		fflush(stderr);
	}

	std::vector<Route> routes_for(uint64_t channel_id)
	{
		std::lock_guard<std::mutex> lk(routes_mtx);
		auto it = routes_by_channel.find(channel_id);
		if (it == routes_by_channel.end())
			return {};
		return it->second;
	}

	void reload_routes(void);
	void refresh_loop(void);

	void on_message(const gwdiscord::Message &m, bool edited);
	void on_delete(const gwdiscord::MessageDelete &d);

	/* Render a Discord message as the Telegram text to send. */
	std::string render(const gwdiscord::Message &m, unsigned types) const;
};

/*
 * Load every enabled route and make sure each distinct bot is logged in.
 * A bot that fails to authorize has its routes dropped for this cycle rather
 * than taking the whole daemon down: the other routes still work.
 */
void DiscordD::Impl::reload_routes(void)
{
	std::vector<Route> rows;
	try {
		rows = db->loadRoutes();
	} catch (const std::exception &e) {
		log(LOG_WARN, std::string("route reload failed: ") + e.what());
		return;
	}

	std::map<uint64_t, std::vector<Route>> next;
	std::vector<int64_t> bot_ids;

	for (auto &r : rows) {
		std::string err;
		const int64_t uid =
			tg->addBot(r.telegram_bot_id, r.bot_token, &err);
		if (!uid) {
			log(LOG_WARN,
			    "bot " + std::to_string(r.telegram_bot_id) +
				    " unavailable, skipping its routes: " + err);
			continue;
		}
		/*
		 * Record the identity on first login, when the token is
		 * repointed at a different bot, or when the username is still
		 * missing -- the last case backfills rows created before the
		 * username was captured, and rows the web UI inserted (it can
		 * only store the token; only a login reveals who the bot is).
		 */
		const std::string uname = tg->botUsername(r.telegram_bot_id);
		if (r.bot_user_id != uid ||
		    (r.bot_username.empty() && !uname.empty())) {
			try {
				/* The username is what /routes labels the bot
				 * with; without it the page can only show a
				 * bare numeric id. */
				db->setBotUserId(r.telegram_bot_id, uid, uname);
			} catch (const std::exception &e) {
				log(LOG_WARN,
				    std::string("could not record bot user id: ") +
					    e.what());
			}
			r.bot_user_id = uid;
		}
		bot_ids.push_back(uid);
		next[r.discord_channel_id].push_back(r);
	}

	size_t n = 0;
	for (const auto &kv : next)
		n += kv.second.size();

	{
		std::lock_guard<std::mutex> lk(routes_mtx);
		routes_by_channel.swap(next);
		our_bot_user_ids.swap(bot_ids);
	}
	log(LOG_INFO, "loaded " + std::to_string(n) + " route(s) across " +
			      std::to_string(routes_by_channel.size()) +
			      " channel(s)");
}

void DiscordD::Impl::refresh_loop(void)
{
	while (!stopping.load()) {
		std::unique_lock<std::mutex> lk(wake_mtx);
		wake_cv.wait_for(lk,
				 std::chrono::seconds(cfg.route_refresh_secs),
				 [this] { return stopping.load(); });
		if (stopping.load())
			return;
		lk.unlock();
		reload_routes();
	}
}

/*
 * Telegram caps a message at 4096 UTF-16 code units -- that is the unit its
 * API counts in, so a string of 4-byte emoji hits the cap at half the
 * codepoint count. Split the rendered text into chunks under the cap,
 * preferring to break on a newline so an attachment URL line is never torn
 * in half. Sent sequentially, everything arrives instead of the whole
 * message being rejected with MESSAGE_TOO_LONG and silently dropped.
 */
static std::vector<std::string> split_for_telegram(const std::string &s)
{
	static constexpr size_t MAX_UNITS = 4096;
	std::vector<std::string> out;
	size_t start = 0, i = 0, units = 0, last_nl = 0;

	while (i < s.size()) {
		const unsigned char b = (unsigned char)s[i];
		size_t len = b < 0x80 ? 1 :
			     (b >> 5) == 0x06 ? 2 :
			     (b >> 4) == 0x0E ? 3 :
			     (b >> 3) == 0x1E ? 4 : 1;
		if (i + len > s.size())
			len = 1;
		/* Astral-plane codepoints are a surrogate pair: two units. */
		const size_t cp_units = len == 4 ? 2 : 1;

		if (units + cp_units > MAX_UNITS) {
			size_t cut = last_nl > start ? last_nl : i;
			out.push_back(s.substr(start, cut - start));
			if (cut == last_nl)
				cut++; /* the newline itself is the seam */
			start = cut;
			i = cut;
			units = 0;
			last_nl = start;
			continue;
		}
		if (s[i] == '\n')
			last_nl = i;
		units += cp_units;
		i += len;
	}
	if (start < s.size())
		out.push_back(s.substr(start));
	if (out.empty())
		out.push_back(std::string());
	return out;
}

/* The FWD_* bit an attachment is filtered by, from its MIME type. */
static unsigned attachment_type(const gwdiscord::Attachment &a)
{
	if (a.content_type.compare(0, 6, "image/") == 0)
		return FWD_PHOTO;
	if (a.content_type.compare(0, 6, "video/") == 0)
		return FWD_VIDEO;
	return FWD_DOCUMENT; /* including one Discord sent no type for */
}

/*
 * Render the parts of `m` a route forwarding `types` (FWD_* bits) should get,
 * or "" when none of them is enabled and the route gets nothing at all.
 */
std::string DiscordD::Impl::render(const gwdiscord::Message &m,
				   unsigned types) const
{
	/*
	 * Plain text: the author line then the content. Telegram entity
	 * formatting is deliberately not attempted yet -- Discord markdown
	 * would have to be translated, and getting that wrong is worse than
	 * sending it verbatim.
	 *
	 * The author is identified by username#discriminator, not by the
	 * display name: the display name is free text a user can set to
	 * anything (and is often long), whereas username#discriminator is the
	 * stable handle that actually identifies the account. Discord migrated
	 * most accounts to a "0" discriminator, which is kept rather than
	 * hidden so the rendering is uniform.
	 */
	std::string who = m.author.username;
	if (who.empty())
		who = "unknown";
	if (!m.author.discriminator.empty())
		who += "#" + m.author.discriminator;

	std::string body;
	if ((types & FWD_TEXT) && !m.content.empty())
		body = m.content;

	/* Media goes across as a link line each; the flags pick the lines. */
	for (const auto &a : m.attachments) {
		if (!(types & attachment_type(a)))
			continue;
		if (!body.empty() && body.back() != '\n')
			body += "\n";
		body += "[" + a.filename + "] " + a.url;
	}
	for (const auto &st : m.stickers) {
		if (!(types & FWD_STICKER))
			continue;
		if (!body.empty() && body.back() != '\n')
			body += "\n";
		body += "[sticker: " + st.name + "] " + st.url();
	}

	/*
	 * A message with nothing we render at all (an embed, a poll) is still
	 * announced, as a text-less text message. That is different from a
	 * message whose every part was filtered out, which must stay silent.
	 */
	if (body.empty()) {
		if (!(types & FWD_TEXT) || !m.content.empty() ||
		    !m.attachments.empty() || !m.stickers.empty())
			return std::string();
		body = "(no text)";
	}
	return who + ":\n" + body;
}

void DiscordD::Impl::on_message(const gwdiscord::Message &m, bool edited)
{
	auto routes = routes_for(m.channel_id);
	if (routes.empty())
		return; /* not a configured channel: nothing to log or send */

	/*
	 * MESSAGE_UPDATE does not mean "a user edited this". Discord fires it
	 * for its own metadata mutations too -- a link embed resolving, a
	 * flags change -- and those carry edited_timestamp: null, since the
	 * user never touched the message. Treating them as edits stamped
	 * edited_at on messages that were never edited and pushed a no-op
	 * editText per forwarded copy. Such an event may still refresh the
	 * archive (below), but it is not an edit and must not forward.
	 *
	 * Partial update payloads can also omit content entirely; those are
	 * dropped whole rather than overwriting recorded text with "".
	 */
	const bool metadata_only = edited && m.edited_timestamp.empty();
	if (metadata_only) {
		if (m.content.empty())
			return;
		edited = false;
	}

	/* Log first, so the archive is complete even if forwarding fails. */
	try {
		db->upsertGuild(m.guild_id, "");
		db->upsertChannel(m.channel_id, m.guild_id, "", 0);
		db->upsertUser(m.author.id, m.author.username,
			       m.author.global_name, m.author.discriminator,
			       m.author.avatar, m.author.bot);
		db->upsertMessage(m.id, m.channel_id, m.guild_id, m.author.id,
				  m.webhook_id, m.content,
				  m.reference ? m.reference->message_id : 0,
				  gwdiscord::snowflake_created_ms(m.id), edited);
		for (const auto &a : m.attachments) {
			db->upsertAttachment(a.id, m.id, a.filename,
					     a.content_type, a.size, a.url,
					     a.width, a.height);
		}
	} catch (const std::exception &e) {
		log(LOG_ERROR, std::string("logging failed: ") + e.what());
	}

	/*
	 * A metadata-only update is archived above and goes no further: it is
	 * neither an edit to apply nor a message to send -- forwarding here
	 * would post a pre-start message to Telegram out of order the moment
	 * one of its links unfurled.
	 */
	if (metadata_only)
		return;

	/*
	 * ECHO SUPPRESSION. Two distinct cases, and both are needed:
	 *
	 *  - webhook_id set: the message was posted by a webhook, which is how
	 *    the Telegram -> Discord forwarder writes into Discord. Sending it
	 *    back would loop forever.
	 *  - the author is one of OUR Telegram bots: not currently reachable,
	 *    since discordd never posts to Discord, but it costs nothing and
	 *    closes the hole if it ever does. A bot's own message carries
	 *    bot=true and webhook_id=0, so the first check does not cover it.
	 */
	if (m.from_webhook()) {
		log(LOG_DEBUG, "skipping webhook message " +
				       std::to_string(m.id) + " (loop guard)");
		return;
	}
	{
		std::lock_guard<std::mutex> lk(routes_mtx);
		for (int64_t uid : our_bot_user_ids) {
			if ((uint64_t)uid == m.author.id) {
				log(LOG_DEBUG, "skipping our own bot's message");
				return;
			}
		}
	}

	if (edited) {
		/*
		 * Apply the edit to what we already sent. One lookup for the
		 * whole event: the per-route filter is applied to the one
		 * result set, instead of re-running the identical query once
		 * per route as the loop below used to.
		 */
		try {
			for (const auto &f : db->getForwarded(m.id)) {
				const Route *route = nullptr;
				for (const auto &r : routes) {
					if (f.route_id == r.id) {
						route = &r;
						break;
					}
				}
				if (!route)
					continue;
				/*
				 * Rendered per route: each has its own content
				 * types. If the edit left nothing this route
				 * forwards, what was sent stays as it is.
				 */
				const std::string text =
					render(m, route->types);
				if (text.empty())
					continue;
				/*
				 * Only the head chunk is recorded, so only it
				 * can be edited; an edit that grew past the
				 * cap updates the head and the tail stays as
				 * originally sent.
				 */
				std::string err;
				if (!tg->editText(f.telegram_bot_id,
						  f.telegram_chat_id,
						  f.telegram_message_id,
						  split_for_telegram(text)[0],
						  &err))
					log(LOG_WARN, "edit failed: " + err);
			}
		} catch (const std::exception &e) {
			log(LOG_ERROR,
			    std::string("edit fan-out failed: ") + e.what());
		}
		return;
	}

	for (const auto &r : routes) {
		try {
			/* Only the content types this route forwards. */
			const std::string text = render(m, r.types);
			if (text.empty()) {
				log(LOG_DEBUG, "nothing in " +
						       std::to_string(m.id) +
						       " for route " +
						       std::to_string(r.id) +
						       " (content types)");
				continue;
			}

			/* A redelivered gateway event must not double-send. */
			if (db->alreadyForwarded(m.id, r.id)) {
				log(LOG_DEBUG, "already forwarded " +
						       std::to_string(m.id));
				continue;
			}

			/*
			 * If this is a reply and the replied-to message has a
			 * Telegram counterpart in the destination chat, thread
			 * the forward onto it. Otherwise send it unthreaded --
			 * better than attaching it to the wrong message.
			 */
			int64_t reply_to = 0;
			if (m.is_reply()) {
				auto t = db->resolveReply(
					m.reference->message_id,
					r.telegram_chat_id);
				if (t) {
					reply_to = t->telegram_message_id;
					log(LOG_DEBUG,
					    "reply maps to telegram message " +
						    std::to_string(reply_to));
				}
			}

			/*
			 * A rendered message can exceed Telegram's limit --
			 * 4000 chars of content plus an attachment line per
			 * file adds up. Send it in chunks rather than letting
			 * the whole forward fail; the first chunk is the one
			 * recorded, so edits and deletes keep pointing at the
			 * head of the message.
			 */
			const auto chunks = split_for_telegram(text);
			std::string err;
			const int64_t sent = tg->sendText(r.telegram_bot_id,
							  r.telegram_chat_id,
							  chunks[0], reply_to,
							  &err);
			if (!sent) {
				log(LOG_WARN, "forward to chat " +
						      std::to_string(r.telegram_chat_id) +
						      " failed: " + err);
				continue;
			}
			db->recordForwarded(m.id, r.id, r.telegram_chat_id,
					    sent);
			for (size_t ci = 1; ci < chunks.size(); ci++) {
				if (!tg->sendText(r.telegram_bot_id,
						  r.telegram_chat_id,
						  chunks[ci], sent, &err))
					log(LOG_WARN, "continuation " +
						std::to_string(ci) +
						" failed: " + err);
			}
			log(LOG_INFO, "forwarded discord " +
					      std::to_string(m.id) +
					      " -> telegram " +
					      std::to_string(r.telegram_chat_id) +
					      "/" + std::to_string(sent));
		} catch (const std::exception &e) {
			log(LOG_ERROR,
			    std::string("forward failed: ") + e.what());
		}
	}
}

void DiscordD::Impl::on_delete(const gwdiscord::MessageDelete &d)
{
	if (routes_for(d.channel_id).empty())
		return;

	try {
		db->markMessageDeleted(d.id);
		/*
		 * Telegram bots may only delete their own messages, and only
		 * within a limited window, so tombstone the text instead --
		 * the same choice the Telegram -> Discord direction makes.
		 */
		for (const auto &f : db->getForwarded(d.id)) {
			std::string err;
			if (!tg->editText(f.telegram_bot_id, f.telegram_chat_id,
					  f.telegram_message_id,
					  "(deleted on Discord)", &err)) {
				log(LOG_WARN, "tombstone failed: " + err);
			}
		}
		log(LOG_INFO, "tombstoned deleted discord message " +
				      std::to_string(d.id));
	} catch (const std::exception &e) {
		log(LOG_ERROR, std::string("delete handling failed: ") +
				       e.what());
	}
}

DiscordD::DiscordD(Config cfg) : impl_(new Impl())
{
	impl_->cfg = std::move(cfg);
}

DiscordD::~DiscordD(void)
{
	stop();
	if (impl_->refresh_thr.joinable())
		impl_->refresh_thr.join();
}

void DiscordD::stop(void)
{
	impl_->stopping = true;
	impl_->wake_cv.notify_all();
	if (impl_->gw)
		impl_->gw->stop();
}

int DiscordD::run(void)
{
	Impl &m = *impl_;

	try {
		m.db = std::make_unique<DB>(m.cfg.db);
		m.db->ping();
	} catch (const std::exception &e) {
		m.log(LOG_ERROR, std::string("database unavailable: ") +
					 e.what());
		return 1;
	}

	m.tg = std::make_unique<TelegramSender>(
		m.cfg.api_id, m.cfg.api_hash, m.cfg.data_dir,
		[&m](int lvl, const std::string &msg) {
			m.log(lvl, "telegram: " + msg);
		});
	m.tg->start();

	m.reload_routes();
	m.refresh_thr = std::thread([&m] { m.refresh_loop(); });

	gwdiscord::GatewayConfig gcfg;
	gcfg.token = m.cfg.discord_bot_token;
	gcfg.intents = gwdiscord::intents::MESSAGE_LOGGING;

	m.gw = std::make_unique<gwdiscord::Gateway>(
		gcfg, gwdiscord::beast_transport(),
		[&m](gwdiscord::LogLevel lvl, const std::string &msg) {
			int l = LOG_INFO;
			switch (lvl) {
			case gwdiscord::LogLevel::Error: l = LOG_ERROR; break;
			case gwdiscord::LogLevel::Warn:  l = LOG_WARN;  break;
			case gwdiscord::LogLevel::Info:  l = LOG_INFO;  break;
			case gwdiscord::LogLevel::Debug: l = LOG_DEBUG; break;
			}
			m.log(l, "gateway: " + msg);
		});

	m.gw->on_ready([&m](const gwdiscord::Ready &r) {
		m.log(LOG_INFO, "connected to Discord as " + r.user.username);
	});
	m.gw->on_message_create(
		[&m](const gwdiscord::Message &msg) { m.on_message(msg, false); });
	m.gw->on_message_update(
		[&m](const gwdiscord::Message &msg) { m.on_message(msg, true); });
	m.gw->on_message_delete(
		[&m](const gwdiscord::MessageDelete &d) { m.on_delete(d); });

	const gwdiscord::StopReason why = m.gw->run();

	m.stopping = true;
	m.wake_cv.notify_all();
	if (m.refresh_thr.joinable())
		m.refresh_thr.join();
	m.tg->stop();

	switch (why) {
	case gwdiscord::StopReason::Requested:
		m.log(LOG_INFO, "shutting down");
		return 0;
	case gwdiscord::StopReason::AuthFailed:
		m.log(LOG_ERROR, "Discord rejected the bot token");
		return 2;
	case gwdiscord::StopReason::FatalClose:
		m.log(LOG_ERROR,
		      "Discord closed the connection with a non-retryable code");
		return 3;
	case gwdiscord::StopReason::Exhausted:
		m.log(LOG_ERROR, "gave up reconnecting to Discord");
		return 4;
	}
	return 0;
}

} /* namespace discordd */

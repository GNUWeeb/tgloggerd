// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "DiscordBots.hpp"

#include <algorithm>
#include <atomic>
#include <thread>

namespace discordd {

enum { LOG_ERROR = 0, LOG_WARN = 1, LOG_INFO = 2, LOG_DEBUG = 3 };

const char *bot_status_name(BotStatus st)
{
	switch (st) {
	case BotStatus::Connecting:	return "connecting";
	case BotStatus::Connected:	return "connected";
	case BotStatus::IntentMissing:	return "intent_missing";
	case BotStatus::TokenRefused:	return "token_refused";
	case BotStatus::Disabled:	return "disabled";
	}
	return "connecting";
}

BotReport report_for_stop(const DiscordBotRow &row, gwdiscord::StopReason why,
			  const gwdiscord::CloseInfo &close)
{
	BotReport r;
	r.id = row.id;
	r.token = row.token;

	std::string code = std::to_string(close.code);
	if (!close.reason.empty())
		code += ": " + close.reason;

	switch (why) {
	case gwdiscord::StopReason::AuthFailed:
		r.status = BotStatus::TokenRefused;
		r.disable = true;
		r.detail = "Discord refused the token (close code " + code +
			   "). Replace the token, then enable the bot.";
		break;
	case gwdiscord::StopReason::FatalClose:
		r.disable = true;
		if (close.code == 4014) {
			r.status = BotStatus::IntentMissing;
			r.detail = "The Message Content intent is not enabled "
				   "for this bot (close code " + code + "). "
				   "Enable it in the Discord Developer Portal, "
				   "then enable the bot.";
		} else {
			/*
			 * 4010-4013: Discord refused the session as discordd
			 * asked for it. Retrying cannot help either, and the
			 * status that says "refused, do not retry" is this.
			 */
			r.status = BotStatus::TokenRefused;
			r.detail = "Discord refused the session (close code " +
				   code + ").";
		}
		break;
	case gwdiscord::StopReason::Exhausted:
		/* Not a refusal: discordd starts it again on a later sync. */
		r.status = BotStatus::Connecting;
		r.detail = "Gave up reconnecting to Discord; trying again.";
		break;
	case gwdiscord::StopReason::Requested:
		r.status = BotStatus::Connecting;
		break;
	}
	return r;
}

struct DiscordBots::Bot {
	DiscordBotRow				row;
	std::unique_ptr<gwdiscord::Gateway>	gw;
	std::thread				thr;
	std::atomic<bool>			stop_requested{false};
	std::atomic<uint64_t>			user_id{0};

	/*
	 * Set by the bot's thread before `done`, read by sync() after it:
	 * why the gateway stopped by itself, whether that reached the
	 * database, and when.
	 */
	BotReport		final;
	bool			reported = false;
	Clock::time_point	finished_at;
	std::atomic<bool>	done{false};
};

DiscordBots::DiscordBots(gwdiscord::GatewayConfig base, Transports tp,
			 Reporter report, Handlers h, Log log)
	: base_(std::move(base)), tp_(std::move(tp)),
	  report_(std::move(report)), h_(std::move(h)), log_(std::move(log))
{
}

DiscordBots::~DiscordBots(void)
{
	stop_all();
}

void DiscordBots::run_bot(const std::shared_ptr<Bot> &b)
{
	const gwdiscord::StopReason why = b->gw->run();
	if (b->stop_requested.load()) {
		b->done = true;
		return;
	}

	BotReport r = report_for_stop(b->row, why, b->gw->fatal_close());
	const bool ok = report_(r);
	if (log_) {
		log_(r.disable ? LOG_ERROR : LOG_WARN,
		     "discord bot " + std::to_string(b->row.id) + ": " +
			     bot_status_name(r.status) + ": " + r.detail +
			     (r.disable ? " Disabled; not retried." : ""));
	}
	b->final = std::move(r);
	b->reported = ok;
	b->finished_at = Clock::now();
	b->done = true;
}

void DiscordBots::start(const DiscordBotRow &row)
{
	auto b = std::make_shared<Bot>();
	b->row = row;

	BotReport hello;
	hello.id = row.id;
	hello.token = row.token;
	hello.status = BotStatus::Connecting;
	if (!report_(hello) && log_)
		log_(LOG_WARN, "discord bot " + std::to_string(row.id) +
				       ": could not record its status");

	gwdiscord::GatewayConfig cfg = base_;
	cfg.token = row.token;
	const std::string tag = "discord bot " + std::to_string(row.id) + ": ";
	Log log = log_;
	b->gw = std::make_unique<gwdiscord::Gateway>(
		cfg, tp_(row),
		[log, tag](gwdiscord::LogLevel lvl, const std::string &msg) {
			if (!log)
				return;
			int l = LOG_INFO;
			switch (lvl) {
			case gwdiscord::LogLevel::Error: l = LOG_ERROR; break;
			case gwdiscord::LogLevel::Warn:  l = LOG_WARN;  break;
			case gwdiscord::LogLevel::Info:  l = LOG_INFO;  break;
			case gwdiscord::LogLevel::Debug: l = LOG_DEBUG; break;
			}
			log(l, tag + "gateway: " + msg);
		});

	Bot *raw = b.get();
	const uint64_t id = row.id;
	b->gw->on_ready([this, raw, tag](const gwdiscord::Ready &rd) {
		raw->user_id = rd.user.id;
		BotReport r;
		r.id = raw->row.id;
		r.token = raw->row.token;
		r.status = BotStatus::Connected;
		r.has_identity = true;
		r.user_id = rd.user.id;
		r.username = rd.user.username;
		if (log_)
			log_(LOG_INFO, tag + "connected as " + rd.user.username);
		if (!report_(r) && log_)
			log_(LOG_WARN, tag + "could not record its status");
	});
	b->gw->on_message_create([this, id](const gwdiscord::Message &m) {
		if (h_.message)
			h_.message(id, m, false);
	});
	b->gw->on_message_update([this, id](const gwdiscord::Message &m) {
		if (h_.message)
			h_.message(id, m, true);
	});
	b->gw->on_message_delete([this, id](const gwdiscord::MessageDelete &d) {
		if (h_.deleted)
			h_.deleted(id, d);
	});

	{
		std::lock_guard<std::mutex> lk(mtx_);
		bots_[row.id] = b;
	}
	b->thr = std::thread([this, b] { run_bot(b); });
}

void DiscordBots::stop_bot(const std::shared_ptr<Bot> &b)
{
	b->stop_requested = true;
	if (b->gw)
		b->gw->stop();
	if (b->thr.joinable())
		b->thr.join();
}

void DiscordBots::sync(const std::vector<DiscordBotRow> &enabled,
		       Clock::time_point loaded_at)
{
	std::map<uint64_t, const DiscordBotRow *> want;
	for (const auto &r : enabled)
		want[r.id] = &r;

	std::vector<std::shared_ptr<Bot>> have;
	{
		std::lock_guard<std::mutex> lk(mtx_);
		for (const auto &kv : bots_)
			have.push_back(kv.second);
	}

	/*
	 * Bots are stopped and their threads joined without mtx_ held: a
	 * handler running on such a thread may be asking user_ids().
	 */
	std::vector<std::shared_ptr<Bot>> gone;
	for (const auto &b : have) {
		auto it = want.find(b->row.id);
		const DiscordBotRow *row = it == want.end() ? nullptr : it->second;

		if (!b->done.load()) {
			if (row && row->token == b->row.token)
				continue; /* running as it should */
			if (log_)
				log_(LOG_INFO, "discord bot " +
						       std::to_string(b->row.id) +
						       (row ? ": token replaced, "
							      "restarting"
							    : ": disabled or removed, "
							      "stopping"));
			gone.push_back(b);
			continue;
		}

		/* It stopped by itself. */
		if (b->thr.joinable())
			b->thr.join();
		if (!b->reported) {
			/*
			 * Its row still says enabled: starting it again
			 * would repeat the refusal. Keep it stopped until
			 * the report is stored.
			 */
			b->reported = report_(b->final);
			if (b->reported)
				b->finished_at = Clock::now();
			continue;
		}
		/*
		 * The rows were read before the report was written, so a
		 * refused bot still looks enabled in them. Wait for a load
		 * that saw the report: in it the bot is disabled, unless it
		 * was enabled again since, which is one more try.
		 */
		if (b->finished_at >= loaded_at)
			continue;
		std::lock_guard<std::mutex> lk(mtx_);
		bots_.erase(b->row.id);
	}
	for (const auto &b : gone) {
		{
			std::lock_guard<std::mutex> lk(mtx_);
			auto it = bots_.find(b->row.id);
			if (it != bots_.end() && it->second == b)
				bots_.erase(it);
		}
		stop_bot(b);
	}

	for (const auto &r : enabled) {
		{
			std::lock_guard<std::mutex> lk(mtx_);
			if (bots_.count(r.id))
				continue;
		}
		start(r);
	}
}

void DiscordBots::stop_all(void)
{
	std::map<uint64_t, std::shared_ptr<Bot>> all;
	{
		std::lock_guard<std::mutex> lk(mtx_);
		all.swap(bots_);
	}
	for (const auto &kv : all)
		stop_bot(kv.second);
}

std::vector<uint64_t> DiscordBots::user_ids(void) const
{
	std::vector<uint64_t> out;
	std::lock_guard<std::mutex> lk(mtx_);
	for (const auto &kv : bots_) {
		const uint64_t u = kv.second->user_id.load();
		if (u)
			out.push_back(u);
	}
	return out;
}

std::vector<uint64_t> DiscordBots::running(void) const
{
	std::vector<uint64_t> out;
	std::lock_guard<std::mutex> lk(mtx_);
	for (const auto &kv : bots_) {
		if (!kv.second->done.load())
			out.push_back(kv.first);
	}
	return out;
}

} /* namespace discordd */

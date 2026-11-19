// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "EmojiApi.hpp"

#include "ChatRef.hpp"
#include "EmojiExtract.hpp"

#include <td/telegram/td_api.h>

#include <algorithm>
#include <cctype>
#include <cstdint>
#include <limits>
#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <utility>

namespace tgloggerd {

namespace {

namespace td_api = td::td_api;
using namespace std::chrono_literals;
using Clock = std::chrono::steady_clock;

/* getCustomEmojiStickers takes at most this many ids. */
constexpr size_t kMaxEmojiIds = 200;

/* At most this many effects in one request: one getMessageEffect each. */
constexpr size_t kMaxEffectIds = 50;

/* Per TDLib request, as the chat endpoints. */
constexpr std::chrono::milliseconds kRequestTimeout = 15s;

/* All the getMessageEffect calls of one request together. */
constexpr std::chrono::milliseconds kEffectsBudget = 20s;

/* How long an id Telegram does not know is answered as missing without
 * asking again. */
constexpr std::chrono::seconds kMissTtl = 1h;

/* The most such ids remembered; past it the expired ones go, then all. */
constexpr size_t kMaxMisses = 100000;

/* The ids Telegram answered without, remembered for kMissTtl. */
class MissCache {
public:
	bool fresh(int64_t id, Clock::time_point now)
	{
		std::lock_guard<std::mutex> lk(mtx_);
		auto it = at_.find(id);
		return it != at_.end() && now - it->second < kMissTtl;
	}

	void add(int64_t id, Clock::time_point now)
	{
		std::lock_guard<std::mutex> lk(mtx_);
		if (at_.size() >= kMaxMisses) {
			for (auto it = at_.begin(); it != at_.end();)
				it = now - it->second >= kMissTtl ? at_.erase(it)
								  : std::next(it);
			if (at_.size() >= kMaxMisses)
				at_.clear();
		}
		at_[id] = now;
	}

private:
	std::mutex					mtx_;
	std::unordered_map<int64_t, Clock::time_point>	at_;
};

/*
 * The ids of {"ids": [...]}: each a nonzero integer, as a JSON number or
 * a decimal string (ids do not fit a JavaScript number). Duplicates
 * count once; the order is kept.
 */
std::vector<int64_t> parseIds(const nlohmann::json &b, size_t max)
{
	if (!b.is_object() || !b.contains("ids") || !b["ids"].is_array())
		throw InternalApi::Error::badRequest("want {\"ids\": [ids]}");
	std::vector<int64_t> out;
	std::unordered_set<int64_t> seen;
	for (const auto &v : b["ids"]) {
		int64_t id = 0;
		bool ok = false;
		if (v.is_number_unsigned()) {
			uint64_t u = v.get<uint64_t>();
			ok = u <= (uint64_t)std::numeric_limits<int64_t>::max();
			id = (int64_t)u;
		} else if (v.is_number_integer()) {
			id = v.get<int64_t>();
			ok = true;
		} else if (v.is_string()) {
			const std::string &s = v.get_ref<const std::string &>();
			size_t used = 0;
			try {
				id = std::stoll(s, &used);
				ok = used == s.size() && !s.empty() &&
				     (std::isdigit((unsigned char)s[0]) || s[0] == '-');
			} catch (const std::exception &) {
				ok = false;
			}
		}
		if (!ok || id == 0)
			throw InternalApi::Error::badRequest(
				"ids must be nonzero 64-bit integers");
		if (seen.insert(id).second)
			out.push_back(id);
	}
	if (out.size() > max)
		throw InternalApi::Error::badRequest(
			"at most " + std::to_string(max) + " ids per request");
	return out;
}

/* How a TDLib call ended, for the answer's outcome. */
struct Outcome {
	std::string	outcome = "ok";
	std::string	message;
	int		retry_after = 0;
};

Outcome outcomeOf(const td_api::error &e)
{
	ChatOutcome c = classifyChatError(e.code_, e.message_);
	Outcome o;
	o.message = e.message_;
	if (c.outcome == "flood_wait" || c.outcome == "unavailable" ||
	    c.outcome == "timeout")
		o.outcome = c.outcome;
	else
		o.outcome = "error";
	o.retry_after = c.retry_after;
	return o;
}

nlohmann::json fileJson(uint64_t id)
{
	return id ? nlohmann::json(id) : nlohmann::json(nullptr);
}

nlohmann::json textJson(const std::string &s)
{
	return s.empty() ? nlohmann::json(nullptr) : nlohmann::json(s);
}

nlohmann::json emojiJson(const models::CustomEmoji &e)
{
	return {
		{ "custom_emoji_id", std::to_string(e.custom_emoji_id) },
		{ "emoji", e.emoji },
		{ "set_id", e.set_id ? nlohmann::json(std::to_string(*e.set_id))
				     : nlohmann::json(nullptr) },
		{ "format", textJson(e.format) },
		{ "needs_repainting", e.needs_repainting },
		{ "file_id", fileJson(e.file_id) },
	};
}

nlohmann::json effectJson(const models::MessageEffect &e)
{
	return {
		{ "effect_id", std::to_string(e.effect_id) },
		{ "emoji", e.emoji },
		{ "is_premium", e.is_premium },
		{ "type", textJson(e.effect_type) },
		{ "static_icon_file_id", fileJson(e.static_icon_file_id) },
		{ "select_animation_file_id", fileJson(e.select_animation_file_id) },
		{ "effect_animation_file_id", fileJson(e.effect_animation_file_id) },
		{ "sticker_file_id", fileJson(e.sticker_file_id) },
		{ "sticker_format", textJson(e.sticker_format) },
	};
}

nlohmann::json answer(const Outcome &o, size_t asked, const char *key,
		      nlohmann::json rows, const std::vector<int64_t> &missing)
{
	nlohmann::json miss = nlohmann::json::array();
	for (int64_t id : missing)
		miss.push_back(std::to_string(id));
	nlohmann::json b = {
		{ "outcome", o.outcome },
		{ "message", o.message },
		{ "asked", asked },
		{ key, std::move(rows) },
		{ "missing", std::move(miss) },
	};
	if (o.outcome == "flood_wait")
		b["retry_after"] = o.retry_after;
	return b;
}

/* POST /v1/custom-emoji */
nlohmann::json customEmoji(const EmojiBackend &be, MissCache &misses,
			   const std::vector<int64_t> &ids)
{
	const auto now = Clock::now();
	std::unordered_map<int64_t, models::CustomEmoji> rows;
	for (auto &e : be.load_emoji(ids))
		rows[e.custom_emoji_id] = std::move(e);

	std::vector<int64_t> want;
	std::unordered_set<int64_t> missing;
	for (int64_t id : ids) {
		if (rows.count(id))
			continue;
		if (misses.fresh(id, now))
			missing.insert(id);
		else
			want.push_back(id);
	}

	Outcome o;
	if (!want.empty()) {
		TDLib::TdObject obj = be.request(
			td_api::make_object<td_api::getCustomEmojiStickers>(
				std::vector<int64_t>(want)),
			kRequestTimeout);
		if (obj && obj->get_id() == td_api::error::ID) {
			o = outcomeOf(static_cast<const td_api::error &>(*obj));
		} else if (!obj || obj->get_id() != td_api::stickers::ID) {
			o.outcome = "error";
			o.message = "unexpected answer from TDLib";
		} else {
			const std::unordered_set<int64_t> wanted(want.begin(),
								 want.end());
			auto &list = static_cast<const td_api::stickers &>(*obj);
			for (const auto &s : list.stickers_) {
				if (!s)
					continue;
				const int64_t id = sticker_custom_emoji_id(*s);
				if (!wanted.count(id) || rows.count(id))
					continue;
				models::CustomEmoji e = custom_emoji_of(*s);
				if (auto mf = sticker_file_meta(s->sticker_.get(),
								e.format))
					e.file_id = be.record_file(*mf);
				be.save_emoji(e);
				rows[id] = std::move(e);
			}
			/* Telegram leaves out the ids it does not know. */
			for (int64_t id : want)
				if (!rows.count(id)) {
					misses.add(id, now);
					missing.insert(id);
				}
		}
	}

	nlohmann::json out = nlohmann::json::array();
	std::vector<int64_t> missing_ordered;
	for (int64_t id : ids) {
		auto it = rows.find(id);
		if (it != rows.end())
			out.push_back(emojiJson(it->second));
		else if (missing.count(id))
			missing_ordered.push_back(id);
	}
	return answer(o, want.size(), "emoji", std::move(out), missing_ordered);
}

/* POST /v1/message-effects */
nlohmann::json messageEffects(const EmojiBackend &be, MissCache &misses,
			      const std::vector<int64_t> &ids)
{
	using Slot = EffectFile::Slot;
	const auto now = Clock::now();
	const auto deadline = now + kEffectsBudget;
	std::unordered_map<int64_t, models::MessageEffect> rows;
	for (auto &e : be.load_effects(ids))
		rows[e.effect_id] = std::move(e);

	std::vector<int64_t> want;
	std::unordered_set<int64_t> missing;
	for (int64_t id : ids) {
		if (rows.count(id))
			continue;
		if (misses.fresh(id, now))
			missing.insert(id);
		else
			want.push_back(id);
	}

	/* One call per effect, stopping at the first failure: a flood
	 * wait or TDLib not ready would fail the rest as well. */
	Outcome o;
	size_t asked = 0;
	for (int64_t id : want) {
		auto left = std::chrono::duration_cast<std::chrono::milliseconds>(
			deadline - Clock::now());
		if (left <= 0ms) {
			o.outcome = "timeout";
			o.message = "Telegram did not answer in time";
			break;
		}
		asked++;
		TDLib::TdObject obj = be.request(
			td_api::make_object<td_api::getMessageEffect>(id),
			std::min(left, kRequestTimeout));
		if (obj && obj->get_id() == td_api::error::ID) {
			auto &err = static_cast<const td_api::error &>(*obj);
			if (err.code_ == 404) {
				/* Not an effect Telegram has. */
				misses.add(id, now);
				missing.insert(id);
				continue;
			}
			o = outcomeOf(err);
			break;
		}
		if (!obj || obj->get_id() != td_api::messageEffect::ID) {
			o.outcome = "error";
			o.message = "unexpected answer from TDLib";
			break;
		}
		auto &eff = static_cast<const td_api::messageEffect &>(*obj);
		models::MessageEffect e = message_effect_of(eff);
		e.effect_id = id;
		for (const auto &x : message_effect_files(eff)) {
			auto mf = sticker_file_meta(x.file, x.ext);
			if (!mf)
				continue;
			const uint64_t fid = be.record_file(*mf);
			switch (x.slot) {
			case Slot::StaticIcon:
				e.static_icon_file_id = fid;
				break;
			case Slot::SelectAnimation:
				e.select_animation_file_id = fid;
				break;
			case Slot::EffectAnimation:
				e.effect_animation_file_id = fid;
				break;
			case Slot::Sticker:
				e.sticker_file_id = fid;
				break;
			}
		}
		be.save_effect(e);
		rows[id] = std::move(e);
	}

	nlohmann::json out = nlohmann::json::array();
	std::vector<int64_t> missing_ordered;
	for (int64_t id : ids) {
		auto it = rows.find(id);
		if (it != rows.end())
			out.push_back(effectJson(it->second));
		else if (missing.count(id))
			missing_ordered.push_back(id);
	}
	return answer(o, asked, "effects", std::move(out), missing_ordered);
}

} /* namespace */

void registerEmojiApi(InternalApi &api, EmojiBackend backend)
{
	auto be = std::make_shared<const EmojiBackend>(std::move(backend));
	auto emoji_misses = std::make_shared<MissCache>();
	auto effect_misses = std::make_shared<MissCache>();

	api.route("POST", "/v1/custom-emoji",
		  [be, emoji_misses](const InternalApi::Request &req) {
		std::vector<int64_t> ids = parseIds(req.body, kMaxEmojiIds);
		InternalApi::Response r;
		r.body = customEmoji(*be, *emoji_misses, ids);
		return r;
	});

	api.route("POST", "/v1/message-effects",
		  [be, effect_misses](const InternalApi::Request &req) {
		std::vector<int64_t> ids = parseIds(req.body, kMaxEffectIds);
		InternalApi::Response r;
		r.body = messageEffects(*be, *effect_misses, ids);
		return r;
	});
}

} /* namespace tgloggerd */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ChatRecheck.hpp"

#include <algorithm>
#include <unordered_set>

namespace tgloggerd {
namespace recheck {

const char *admitReason(Admit a)
{
	switch (a) {
	case Admit::Recent:
		return "recent";
	case Admit::Busy:
		return "busy";
	case Admit::AlreadyQueued:
		return "queued";
	case Admit::Queued:
		break;
	}
	return "";
}

static Clock::duration intervalOf(int64_t around)
{
	return around == 0 ? Clock::duration(kNewestInterval)
			   : Clock::duration(kOlderInterval);
}

void Queue::note(const Key &k, Clock::time_point now)
{
	last_[k] = now;
}

/* Forget the stretches whose interval has passed, at most once a minute,
 * so the map holds only what can still answer "recent". */
void Queue::prune(Clock::time_point now)
{
	if (pruned_ != Clock::time_point{} && now - pruned_ < kNewestInterval)
		return;
	pruned_ = now;
	for (auto it = last_.begin(); it != last_.end();)
		it = now - it->second >= intervalOf(it->first.second)
			     ? last_.erase(it) : std::next(it);
}

Admit Queue::push(const Item &it, Clock::time_point now)
{
	const Key k{ it.chat_id, it.around };
	if (waiting_.count(k))
		return Admit::AlreadyQueued;
	prune(now);
	auto l = last_.find(k);
	if (l != last_.end() && now - l->second < intervalOf(it.around))
		return Admit::Recent;
	if (size() >= max_)
		return Admit::Busy;
	Item q = it;
	q.queued_at = now;
	(it.background ? background_ : asked_).push_back(q);
	waiting_.insert(k);
	note(k, now);
	return Admit::Queued;
}

std::optional<Item> Queue::pop(Clock::time_point now, bool background_ok)
{
	if (asked_.empty() && !background_ok)
		return std::nullopt;
	std::deque<Item> &q = !asked_.empty() ? asked_ : background_;
	if (q.empty())
		return std::nullopt;
	Item it = q.front();
	q.pop_front();
	const Key k{ it.chat_id, it.around };
	waiting_.erase(k);
	note(k, now);
	return it;
}

bool repairMayRun(size_t rechecks_queued, int rechecks_inflight,
		  Clock::duration since_repair_sent)
{
	if (rechecks_queued == 0 && rechecks_inflight <= 0)
		return true;
	return since_repair_sent >= kRepairMaxPause;
}

int repairAllowance(int budget, int tick_used, int repair_used)
{
	return std::max(0, std::min(budget - tick_used,
				    kRepairPerSecond - repair_used));
}

Shape shapeOf(int64_t around)
{
	if (around == 0)
		return Shape{ 0, 0, kNewestLimit };
	/* TDLib addresses a server message as id << 20. */
	return Shape{ around << 20, kOlderOffset, kOlderLimit };
}

std::optional<Span> spanOf(const Page &p)
{
	if (p.ids.empty())
		return std::nullopt;
	auto [lo, hi] = std::minmax_element(p.ids.begin(), p.ids.end());
	if (*lo == *hi)
		return std::nullopt;
	return Span{ *lo, *hi };
}

static Verdict nothing(const char *why)
{
	Verdict v;
	v.why = why;
	return v;
}

bool isShort(const Page &p, bool older_exists)
{
	/* The older messages asked for: the whole newest page or a later
	 * page, or N and those below it; the few newer ones around N are
	 * not judged. */
	size_t want_older = kNewestLimit;
	size_t older = p.total;
	if (!p.cont && p.around != 0) {
		const Shape sh = shapeOf(p.around);
		want_older = (size_t)(sh.limit + sh.offset);
		older = 0;
		for (int64_t id : p.ids)
			older += id <= p.around;
	}
	return older < want_older && older_exists;
}

bool askAgain(int pages_used, bool short_answer, size_t new_messages)
{
	return short_answer && pages_used < kMaxPages && new_messages > 0;
}

Verdict selectDeleted(const Page &p, const Stored &s)
{
	auto span = spanOf(p);
	if (!span)
		return nothing("fewer than two messages returned");

	if (isShort(p, s.older_exists))
		return nothing("short answer above stored older messages");

	std::unordered_set<int64_t> got(p.ids.begin(), p.ids.end());
	Verdict v;
	v.concluded = true;
	for (int64_t id : s.between)
		if (id > span->lo && id < span->hi && !got.count(id))
			v.deleted.push_back(id);
	std::sort(v.deleted.begin(), v.deleted.end());
	v.deleted.erase(std::unique(v.deleted.begin(), v.deleted.end()),
			v.deleted.end());
	return v;
}

std::vector<int64_t> catchUpChats(
	const std::unordered_map<int64_t, int64_t> &last_message_date,
	int64_t now)
{
	std::vector<std::pair<int64_t, int64_t>> recent;	/* (date, chat) */
	for (const auto &[chat, date] : last_message_date)
		if (date > 0 && now - date <= kCatchUpWindowSecs)
			recent.emplace_back(date, chat);
	std::sort(recent.begin(), recent.end(),
		  [](const auto &a, const auto &b) {
		return a.first != b.first ? a.first > b.first
					  : a.second < b.second;
	});
	std::vector<int64_t> out;
	out.reserve(recent.size());
	for (const auto &r : recent)
		out.push_back(r.second);
	return out;
}

std::vector<int64_t> gapChats(
	const std::vector<int64_t> &chats,
	const std::unordered_map<int64_t, int64_t> &tdlib_last,
	const std::unordered_map<int64_t, int64_t> &stored_max)
{
	std::vector<int64_t> out;
	for (int64_t c : chats) {
		auto t = tdlib_last.find(c);
		if (t == tdlib_last.end() || t->second <= 0)
			continue; /* TDLib named no last message: nothing to compare */
		auto s = stored_max.find(c);
		const int64_t have = s == stored_max.end() ? 0 : s->second;
		if (have < t->second)
			out.push_back(c);
	}
	return out;
}

} /* namespace recheck */
} /* namespace tgloggerd */

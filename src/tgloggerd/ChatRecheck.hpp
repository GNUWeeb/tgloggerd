// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__CHAT_RECHECK_HPP
#define TGLOGGERD__CHAT_RECHECK_HPP

/*
 * The parts of a chat recheck (POST /v1/chats/{chat_id}/recheck,
 * docs/internal-api.md) that need neither TDLib nor the database: the
 * queue with its limits, the choice of the messages a recheck may mark
 * deleted, and the chats the restart catch-up rechecks. Plain data in,
 * plain data out, so each is tested on its own.
 */

#include <chrono>
#include <cstddef>
#include <cstdint>
#include <deque>
#include <map>
#include <optional>
#include <set>
#include <unordered_map>
#include <utility>
#include <vector>

namespace tgloggerd {
namespace recheck {

using Clock = std::chrono::steady_clock;

/* The newest stretch of a chat is rechecked at most this often... */
constexpr std::chrono::seconds kNewestInterval{60};
/* ...and an older stretch (chat, around) at most this often. */
constexpr std::chrono::seconds kOlderInterval{24 * 3600};
/* Rechecks waiting at most; past it a request is answered "busy". */
constexpr size_t kQueueMax = 1000;

/*
 * The getChatHistory of a recheck, one per recheck: from (a TDLib
 * message id), offset and limit. The newest stretch is the newest 50
 * messages. Around a message N, which the web sends as the NEWER edge
 * of what it shows (an older page's "before", or a message page's own
 * id), it is mostly older: N and 49 older, and up to 10 newer.
 */
struct Shape {
	int64_t	from = 0;
	int32_t	offset = 0;
	int32_t	limit = 0;
};

constexpr int32_t kNewestLimit = 50;
constexpr int32_t kOlderLimit = 60;
constexpr int32_t kOlderOffset = -10;

/* The shape for @around (a server message id, 0 = the newest). */
Shape shapeOf(int64_t around);

/* The largest server message id that still fits a TDLib id (id << 20). */
constexpr int64_t kMaxServerId = INT64_MAX >> 20;

/* The restart catch-up rechecks chats whose last message is this recent. */
constexpr int64_t kCatchUpWindowSecs = 48 * 3600;

/* A recheck that marks more than this many messages is logged as a warning. */
constexpr size_t kWarnMarked = 10;

/* A queued recheck. around is a server message id; 0 = the newest. */
struct Item {
	int64_t	chat_id = 0;
	int64_t	around = 0;
	/* Queued by the daemon itself (the restart catch-up), not asked for:
	 * waits behind every recheck that was asked for. */
	bool	background = false;
};

/* What became of a request for a recheck. */
enum class Admit {
	Queued,		/* queued now */
	Recent,		/* that stretch was rechecked or queued too recently */
	Busy,		/* the queue is full */
	AlreadyQueued,	/* that exact recheck is waiting already */
};

/* The "reason" of a 200 answer ("recent", "busy", "queued"); "" for
 * Queued, which is the 202. */
const char *admitReason(Admit a);

/*
 * The recheck queue and its limits. Not thread-safe: the daemon holds
 * it under the TDLib refetch lock. Times are passed in, so tests run on
 * a clock of their own.
 *
 * A stretch counts as rechecked when it is queued and again when it is
 * taken off the queue to run, so a recheck that waited is not repeated
 * just after it ran. Rechecks asked for run before background ones;
 * each kind in the order it was queued. Both count against kQueueMax.
 */
class Queue {
public:
	explicit Queue(size_t max = kQueueMax) : max_(max) {}

	Admit push(const Item &it, Clock::time_point now);
	std::optional<Item> pop(Clock::time_point now);

	size_t size(void) const { return asked_.size() + background_.size(); }
	bool empty(void) const { return size() == 0; }

private:
	using Key = std::pair<int64_t, int64_t>;	/* (chat_id, around) */

	void note(const Key &k, Clock::time_point now);
	void prune(Clock::time_point now);

	size_t				max_;
	std::deque<Item>		asked_;
	std::deque<Item>		background_;
	std::set<Key>			waiting_;
	std::map<Key, Clock::time_point> last_;
	Clock::time_point		pruned_{};
};

/*
 * One recheck's answer, in server message ids: the stretch asked for
 * (around; its shape is shapeOf(around)) and what came back. total
 * counts every message returned, ids only those with a server id.
 */
struct Page {
	int64_t			around = 0;
	size_t			total = 0;
	std::vector<int64_t>	ids;
};

/* The range of stored ids to look at: strictly between lo and hi. */
struct Span {
	int64_t	lo = 0;
	int64_t	hi = 0;
};

/*
 * The span of an answer: its lowest and highest id. None for an answer
 * of fewer than two distinct ids, from which nothing is concluded.
 */
std::optional<Span> spanOf(const Page &p);

/*
 * What the database holds for the chat around that span: the live
 * (not deleted) stored ids strictly between lo and hi, and whether a
 * live stored message lies below lo (so the oldest returned is not the
 * chat's first message).
 */
struct Stored {
	std::vector<int64_t>	between;
	bool			older_exists = false;
};

struct Verdict {
	/* The stored ids to mark deleted, ascending. */
	std::vector<int64_t>	deleted;
	/* Whether the answer was complete enough to conclude anything; when
	 * false, deleted is empty and why says what was missing. */
	bool			concluded = false;
	const char		*why = "";
};

/*
 * The stored messages a recheck marks deleted: the live ones strictly
 * between the lowest and highest id returned that were not returned.
 * Nothing is concluded from an answer of fewer than two messages, nor
 * from a short one while the stretch should have more: fewer older
 * messages than asked (all of the newest stretch's 50; around N, the 50
 * at and below N) while an older message is stored (TDLib can answer
 * short while it loads). Few newer messages around N is normal near
 * the newest end. TDLib's answer is the only evidence; nothing outside
 * it is touched.
 */
Verdict selectDeleted(const Page &p, const Stored &s);

/*
 * The chats the restart catch-up rechecks: those whose last message
 * (Unix date) is within kCatchUpWindowSecs of now, newest first.
 */
std::vector<int64_t> catchUpChats(
	const std::unordered_map<int64_t, int64_t> &last_message_date,
	int64_t now);

} /* namespace recheck */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__CHAT_RECHECK_HPP */

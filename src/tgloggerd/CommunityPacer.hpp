// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__COMMUNITY_PACER_HPP
#define TGLOGGERD__COMMUNITY_PACER_HPP

#include <cstdlib>
#include <ctime>
#include <string>

namespace tgloggerd {

/*
 * The pace of getCommunityFullInfo, for the whole account (one pacer per
 * daemon, not per chat): one ask at a time, at most kCommunityAsksPerMinute a
 * minute, and none while Telegram's flood wait lasts. Telegram answers a
 * burst of these with 420/429, and TDLib then sits out the wait inside
 * every other request of the account too.
 */
constexpr int kCommunityAsksPerMinute = 3;
constexpr time_t kCommunityAskGap = 60 / kCommunityAsksPerMinute;

struct CommunityFullInfoPacer {
	bool	inflight = false;
	time_t	next_at = 0;		/* no ask before this */
	time_t	paused_until = 0;	/* a flood wait's end */

	bool mayAsk(time_t now) const
	{
		return !inflight && now >= next_at && now >= paused_until;
	}
	void asked(time_t now)
	{
		inflight = true;
		next_at = now + kCommunityAskGap;
	}
	void answered(void)
	{
		inflight = false;
	}
	/* A 420/429: every ask waits @secs, the community asked included. */
	void floodWait(time_t now, long secs)
	{
		inflight = false;
		paused_until = now + (secs > 0 ? secs : 1);
	}
};

/*
 * The seconds a flood-wait error asks for: TDLib's "Too Many Requests:
 * retry after N" (429) or "FLOOD_WAIT_N" (420), @dflt when the message
 * names none.
 */
inline long retryAfterSecs(const std::string &message, long dflt)
{
	size_t i = message.find_last_not_of("0123456789");
	if (i == std::string::npos || i + 1 >= message.size())
		return dflt;
	long v = std::atol(message.c_str() + i + 1);
	return v > 0 ? v : dflt;
}

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__COMMUNITY_PACER_HPP */

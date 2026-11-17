// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "EventApi.hpp"

#include <cerrno>
#include <cstdlib>

namespace tgloggerd {

namespace {

/* Events per pull; the server pulls again as soon as they are written. */
constexpr size_t kBatch = 256;

void append_frame(std::string &out, uint64_t seq, const std::string &json)
{
	out += "id: ";
	out += std::to_string(seq);
	out += "\ndata: ";
	out += json;
	out += "\n\n";
}

/* hello and resync: where the subscriber stands, as a seq. */
void append_marker(std::string &out, const char *kind, uint64_t seq)
{
	append_frame(out, seq, "{\"seq\":" + std::to_string(seq) +
			       ",\"kind\":\"" + kind + "\"}");
}

/*
 * The since parameter: false when absent; decimal digits that fit 64
 * bits; anything else is a 400.
 */
bool parse_since(const std::string &query, uint64_t &since)
{
	size_t i = 0;
	bool found = false;
	while (i <= query.size()) {
		size_t amp = query.find('&', i);
		if (amp == std::string::npos)
			amp = query.size();
		std::string kv = query.substr(i, amp - i);
		i = amp + 1;
		size_t eq = kv.find('=');
		if (kv.substr(0, eq) != "since")
			continue;
		std::string v = eq == std::string::npos ? "" : kv.substr(eq + 1);
		bool digits = !v.empty() && v.size() <= 20;
		for (char c : v)
			digits = digits && c >= '0' && c <= '9';
		errno = 0;
		unsigned long long n = digits ? strtoull(v.c_str(), nullptr, 10)
					      : 0;
		if (!digits || errno)
			throw InternalApi::Error::badRequest(
				"since must be a decimal seq");
		since = (uint64_t)n;
		found = true;
	}
	return found;
}

class EventFeed : public InternalApi::Stream {
public:
	EventFeed(EventBus &bus, uint64_t sub, uint64_t cursor,
		  std::string first)
		: bus_(bus), sub_(sub), cursor_(cursor),
		  first_(std::move(first)) {}

	~EventFeed(void) override
	{
		/* No wakeup after this returns (EventBus::unsubscribe). */
		bus_.unsubscribe(sub_);
	}

	bool pull(std::string &out) override
	{
		if (!first_.empty()) {
			out += first_;
			first_.clear();
		}
		std::vector<EventBus::Event> evs;
		uint64_t last = 0;
		if (bus_.readSince(cursor_, evs, kBatch, &last) ==
		    EventBus::ReadResult::Gap) {
			/*
			 * Fell out of the ring while the write of an earlier
			 * batch was slow: what was skipped cannot be sent,
			 * so the subscriber resynchronises and goes on from
			 * the newest event.
			 */
			cursor_ = last;
			append_marker(out, "resync", last);
			return true;
		}
		for (const auto &e : evs) {
			append_frame(out, e.seq, e.json);
			cursor_ = e.seq;
		}
		return true;
	}

	std::string keepalive(void) override
	{
		return ": keepalive\n\n";
	}

private:
	EventBus	&bus_;
	uint64_t	sub_;
	uint64_t	cursor_;
	std::string	first_;
};

} /* namespace */

void registerEventApi(InternalApi &api, EventBus &bus)
{
	EventBus *b = &bus;
	api.streamRoute("/v1/events", "text/event-stream",
		[b](const InternalApi::Request &req, InternalApi::Wake wake)
			-> std::unique_ptr<InternalApi::Stream> {
		uint64_t since = 0;
		bool has_since = parse_since(req.query, since);

		uint64_t sub = b->subscribe(std::move(wake));
		if (!sub)
			throw InternalApi::Error::unavailable(
				"too many event subscribers", 5);

		/*
		 * Subscribed before the cursor is taken, so an event
		 * published in between wakes the stream rather than being
		 * missed; the first pull reads from the cursor anyway.
		 */
		std::vector<EventBus::Event> none;
		uint64_t last = 0;
		std::string first;
		uint64_t cursor;
		if (!has_since) {
			cursor = b->lastSeq();
			append_marker(first, "hello", cursor);
		} else if (b->readSince(since, none, 0, &last) ==
			   EventBus::ReadResult::Ok) {
			cursor = since;
			append_marker(first, "hello", cursor);
		} else {
			cursor = last;
			append_marker(first, "resync", cursor);
		}
		try {
			return std::make_unique<EventFeed>(*b, sub, cursor,
							   std::move(first));
		} catch (...) {
			b->unsubscribe(sub);
			throw;
		}
	});
}

} /* namespace tgloggerd */

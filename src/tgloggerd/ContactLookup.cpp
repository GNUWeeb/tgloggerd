// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ContactLookup.hpp"

#include <algorithm>
#include <exception>
#include <utility>

namespace tgloggerd {

using Kind = ContactError::Kind;
using std::chrono::steady_clock;

ContactLookup::ContactLookup(ContactTelegram &tg, ContactJournal &journal,
			     StoreOwnName store, Config cfg, log_hd_t *l)
	: tg_(tg), journal_(journal), store_(std::move(store)), cfg_(cfg),
	  l_(l)
{
}

ContactLookup::~ContactLookup(void)
{
	stop();
}

bool ContactLookup::pause(std::chrono::milliseconds d)
{
	std::unique_lock<std::mutex> lk(stop_mu_);
	return !stop_cv_.wait_for(lk, d, [this] { return stopping_; });
}

void ContactLookup::stop(void)
{
	{
		std::lock_guard<std::mutex> lk(stop_mu_);
		stopping_ = true;
	}
	stop_cv_.notify_all();
	if (restore_thread_.joinable())
		restore_thread_.join();
}

void ContactLookup::readd(const ContactRecord &c)
{
	/*
	 * The one step that must not be given up lightly: until it succeeds
	 * the account is missing a contact. A short flood wait is sat out;
	 * anything else leaves the row for restorePending().
	 */
	for (int i = 1;; i++) {
		try {
			tg_.addContact(c);
			return;
		} catch (const ContactError &e) {
			bool again = e.kind() == Kind::FloodWait &&
				     i < cfg_.readd_attempts &&
				     std::chrono::seconds(e.retryAfter()) <=
					     cfg_.readd_max_wait;
			pr_warn(l_, "contacts: re-adding user %lld failed"
				" (attempt %d): %s", (long long)c.user_id, i,
				e.what());
			if (!again ||
			    !pause(std::chrono::seconds(e.retryAfter() + 1)))
				throw;
		}
	}
}

size_t ContactLookup::restore_locked(int *retry_after, bool *not_ready)
{
	std::vector<ContactRecord> rows = journal_.pending();
	size_t left = 0;
	for (const auto &c : rows) {
		try {
			readd(c);
		} catch (const ContactError &e) {
			left++;
			if (e.kind() == Kind::NotReady)
				*not_ready = true;
			*retry_after = std::max(*retry_after, e.retryAfter());
			try {
				journal_.noteFailure(c.user_id, e.what());
			} catch (const std::exception &de) {
				pr_error(l_, "contacts: cannot note the failure"
					 " for user %lld: %s",
					 (long long)c.user_id, de.what());
			}
			continue;
		}
		pr_info(l_, "contacts: restored contact %lld left by an"
			" interrupted lookup", (long long)c.user_id);
		journal_.erase(c.user_id);
	}
	return left;
}

size_t ContactLookup::restorePending(void)
{
	std::lock_guard<std::mutex> lk(mu_);
	int ra = 0;
	bool nr = false;
	return restore_locked(&ra, &nr);
}

void ContactLookup::restore_loop(void)
{
	std::chrono::seconds backoff = cfg_.restore_retry;
	for (;;) {
		size_t left;
		int ra = 0;
		bool nr = false;
		try {
			std::lock_guard<std::mutex> lk(mu_);
			left = restore_locked(&ra, &nr);
		} catch (const std::exception &e) {
			/* The journal itself failed (database down). */
			pr_error(l_, "contacts: reading the lookup journal"
				 " failed: %s", e.what());
			left = 1;
		}
		if (left == 0)
			return;

		/*
		 * Waiting for TDLib to authorize is the common case at start
		 * and is retried quickly; other failures back off up to ten
		 * minutes, or as long as Telegram asks.
		 */
		std::chrono::seconds wait = cfg_.restore_retry;
		if (!nr) {
			wait = std::max(backoff, std::chrono::seconds(ra + 1));
			backoff = std::min(backoff * 2,
					   std::chrono::seconds(600));
			pr_warn(l_, "contacts: %zu contact(s) still to be"
				" added back; retrying in %llds", left,
				(long long)wait.count());
		}
		if (!pause(wait))
			return;
	}
}

void ContactLookup::startRestore(void)
{
	restore_thread_ = std::thread([this] { restore_loop(); });
}

ContactLookup::Result ContactLookup::lookup(int64_t user_id)
{
	std::unique_lock<std::mutex> lk(mu_, std::try_to_lock);
	if (!lk.owns_lock())
		throw ContactError(Kind::Busy, "another lookup is running", 5);
	{
		std::lock_guard<std::mutex> sl(stop_mu_);
		if (stopping_)
			throw ContactError(Kind::NotReady,
					   "the daemon is shutting down", 5);
	}

	auto now = steady_clock::now();
	if (last_lookup_ && now - *last_lookup_ < cfg_.min_interval) {
		auto left = std::chrono::duration_cast<std::chrono::seconds>(
			cfg_.min_interval - (now - *last_lookup_));
		throw ContactError(Kind::RateLimited,
				   "one lookup per " +
				   std::to_string(cfg_.min_interval.count()) +
				   " seconds", (int)left.count() + 1);
	}

	/* A contact still missing from an earlier cycle goes back first. */
	int ra = 0;
	bool nr = false;
	if (restore_locked(&ra, &nr) != 0)
		throw ContactError(nr ? Kind::NotReady : Kind::RestorePending,
				   "a contact removed by an earlier lookup is"
				   " not added back yet", std::max(ra, 60));

	ContactUserState st = tg_.getUser(user_id);
	if (!st.is_contact)
		throw ContactError(Kind::NotContact, "not a contact");
	ContactTelegram::FullInfo fi = tg_.getFullInfo(user_id);

	ContactRecord c;
	c.user_id = user_id;
	c.phone_number = st.phone_number;
	c.first_name = st.first_name;
	c.last_name = st.last_name;
	c.note_json = fi.note_json;
	/*
	 * TDLib asks for the exception when the account's privacy settings
	 * do not show its number to this user; without one being needed,
	 * the number is shared, by the settings or by an exception that the
	 * re-add must keep.
	 */
	c.share_phone_number = !fi.need_phone_number_privacy_exception;

	/* Journal first: from here on the contact can always be put back. */
	journal_.insert(c);
	last_lookup_ = now;
	pr_info(l_, "contacts: looking up the own name of contact %lld",
		(long long)user_id);

	try {
		tg_.removeContact(user_id);
	} catch (const ContactError &e) {
		/*
		 * Refused (a flood wait, most likely). If the contact is still
		 * there nothing changed and the journal row goes; if it cannot
		 * be told, it stays and is re-added like any other.
		 */
		bool still = false;
		try {
			still = tg_.getUser(user_id).is_contact;
		} catch (const ContactError &) {
		}
		if (still) {
			journal_.erase(user_id);
			throw;
		}
		pr_warn(l_, "contacts: removing %lld reported %s, but it is"
			" not a contact now", (long long)user_id, e.what());
	}
	try {
		journal_.markRemoved(user_id);
	} catch (const std::exception &e) {
		/* Bookkeeping only; the row is there and is what counts. */
		pr_warn(l_, "contacts: cannot mark %lld removed: %s",
			(long long)user_id, e.what());
	}

	/*
	 * TDLib applies the users that come with the removal and reports
	 * them; poll its view of the user until it is no contact any more.
	 */
	std::optional<models::Name> own;
	auto deadline = steady_clock::now() + cfg_.name_wait;
	for (;;) {
		try {
			ContactUserState s = tg_.getUser(user_id);
			if (!s.is_contact) {
				own = models::Name{ s.first_name, s.last_name };
				break;
			}
		} catch (const ContactError &e) {
			if (e.kind() == Kind::NotReady)
				break;
		}
		if (steady_clock::now() >= deadline || !pause(cfg_.name_poll))
			break;
	}

	try {
		readd(c);
	} catch (const ContactError &e) {
		try {
			journal_.noteFailure(user_id, e.what());
		} catch (const std::exception &) {
		}
		pr_error(l_, "contacts: contact %lld is removed and not added"
			 " back yet: %s", (long long)user_id, e.what());
		throw ContactError(Kind::RestorePending,
				   std::string("the contact is removed and not"
					       " added back yet: ") + e.what(),
				   std::max(e.retryAfter(), 60));
	}

	std::string store_err;
	if (own) {
		try {
			store_(user_id, *own);
		} catch (const std::exception &e) {
			store_err = e.what();
		}
	}
	journal_.erase(user_id);

	if (!own)
		throw ContactError(Kind::Timeout, "the contact is added back,"
				   " but TDLib did not report the own name");
	if (!store_err.empty())
		throw ContactError(Kind::Internal, "the contact is added back,"
				   " but storing the name failed: " +
				   store_err);

	pr_info(l_, "contacts: contact %lld is added back; own name looked"
		" up", (long long)user_id);

	Result r;
	r.user_id = user_id;
	r.own_name = *own;
	r.contact_name = { c.first_name, c.last_name };
	return r;
}

} /* namespace tgloggerd */

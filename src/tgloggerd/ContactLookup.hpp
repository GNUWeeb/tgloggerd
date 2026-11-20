// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__CONTACT_LOOKUP_HPP
#define TGLOGGERD__CONTACT_LOOKUP_HPP

#include <tgloggerd/models/ContactName.hpp>
#include "helpers/log.h"

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <functional>
#include <mutex>
#include <optional>
#include <stdexcept>
#include <string>
#include <thread>
#include <vector>

namespace tgloggerd {

/*
 * "Look up real name": read a contact's own name by removing the contact,
 * reading the name TDLib then reports, and adding the contact back exactly
 * as it was (docs/internal-api.md, POST /v1/users/{id}/lookup-name).
 *
 * Telegram gives an account no field with a contact's own name; it only
 * shows it while the user is not a contact. The cycle is journalled in
 * telegram_contact_lookups before anything changes, and a row is deleted
 * only after the contact was added back, so a daemon that dies in between
 * re-adds the contact on its next start.
 *
 * This class holds the cycle and the rules (one lookup at a time, a minimum
 * interval, no retry of a refused lookup). Telegram and the journal are
 * behind the two interfaces below, so the logic runs without TDLib or a
 * database in a test.
 */

/* A contact as the account has it: one journal row. */
struct ContactRecord {
	int64_t		user_id = 0;
	std::string	phone_number;
	std::string	first_name;
	std::string	last_name;
	/* userFullInfo.note as {"text": ..., "entities": [...]}; none if unset. */
	std::optional<std::string>	note_json;
	bool		share_phone_number = false;
};

/* What a lookup needs from a td_api::user. */
struct ContactUserState {
	bool		is_contact = false;
	std::string	first_name;
	std::string	last_name;
	std::string	phone_number;
};

/* A refused or failed step, with what to answer. */
class ContactError : public std::runtime_error {
public:
	enum class Kind {
		NotFound,	/* TDLib does not know the user */
		NotContact,	/* nothing to look up */
		Unsupported,	/* the contact cannot be restored exactly */
		RateLimited,	/* our own limit; retry_after set */
		FloodWait,	/* Telegram's limit; retry_after set */
		Busy,		/* another lookup is running */
		NotReady,	/* TDLib not authorized, or shutting down */
		RestorePending,	/* a removed contact is not added back yet */
		Timeout,	/* TDLib did not answer, or no own name came */
		Failed,		/* any other Telegram error */
		Internal,	/* our side failed after the contact came back */
	};

	ContactError(Kind k, const std::string &msg, int retry_after = 0)
		: std::runtime_error(msg), kind_(k), retry_after_(retry_after) {}

	Kind kind(void) const noexcept { return kind_; }
	int retryAfter(void) const noexcept { return retry_after_; }

private:
	Kind	kind_;
	int	retry_after_;
};

/* Telegram, over TDLib::request. Every call throws ContactError. */
class ContactTelegram {
public:
	struct FullInfo {
		std::optional<std::string>	note_json;
		/* userFullInfo.need_phone_number_privacy_exception */
		bool	need_phone_number_privacy_exception = false;
	};

	virtual ~ContactTelegram(void) = default;
	virtual ContactUserState getUser(int64_t user_id) = 0;
	/* Throws Unsupported for a note it could not put back as it is. */
	virtual FullInfo getFullInfo(int64_t user_id) = 0;
	virtual void removeContact(int64_t user_id) = 0;
	virtual void addContact(const ContactRecord &c) = 0;
};

/* telegram_contact_lookups. Every call may throw (a database error). */
class ContactJournal {
public:
	virtual ~ContactJournal(void) = default;
	virtual void insert(const ContactRecord &c) = 0;
	virtual void markRemoved(int64_t user_id) = 0;
	virtual void noteFailure(int64_t user_id, const std::string &err) = 0;
	virtual void erase(int64_t user_id) = 0;
	virtual std::vector<ContactRecord> pending(void) = 0;
};

class ContactLookup {
public:
	struct Config {
		/* Least time between two lookups (a refused one does not count). */
		std::chrono::seconds		min_interval{300};
		/* How long to wait for the own name after the removal. */
		std::chrono::milliseconds	name_wait{10000};
		std::chrono::milliseconds	name_poll{250};
		/* A re-add hit by a flood wait this short is tried again. */
		std::chrono::seconds		readd_max_wait{30};
		int				readd_attempts = 3;
		/* Boot restore: pause between rounds while TDLib is not ready. */
		std::chrono::seconds		restore_retry{5};
	};

	struct Result {
		int64_t		user_id = 0;
		models::Name	own_name;
		models::Name	contact_name;
	};

	/* Stores the own name and the check time (DB::recordOwnNameLookup). */
	using StoreOwnName = std::function<void(int64_t, const models::Name &)>;

	ContactLookup(ContactTelegram &tg, ContactJournal &journal,
		      StoreOwnName store, Config cfg, log_hd_t *l);
	~ContactLookup(void);

	ContactLookup(const ContactLookup &) = delete;
	ContactLookup &operator=(const ContactLookup &) = delete;

	/*
	 * One lookup for one user. Throws ContactError; whatever it throws,
	 * the contact is either untouched, added back, or left in the
	 * journal for restorePending() (then RestorePending is thrown).
	 */
	Result lookup(int64_t user_id);

	/*
	 * Add back every contact left in the journal. Returns how many are
	 * still left; failures are logged and noted on their row.
	 */
	size_t restorePending(void);

	/*
	 * Run restorePending() on a thread until the journal is empty,
	 * waiting while TDLib is not ready and for flood waits. Call once,
	 * at start, before anything else uses the contacts.
	 */
	void startRestore(void);

	/*
	 * Stop the restore thread and cut short the waits of a running
	 * lookup (it still tries its re-add once). Call before the API and
	 * TDLib stop. Idempotent.
	 */
	void stop(void);

private:
	/* restorePending() with mu_ held; *retry_after gets the longest wait. */
	size_t restore_locked(int *retry_after, bool *not_ready);
	void readd(const ContactRecord &c);
	/* Sleep unless stopping; false if stopping. */
	bool pause(std::chrono::milliseconds d);
	void restore_loop(void);

	ContactTelegram		&tg_;
	ContactJournal		&journal_;
	StoreOwnName		store_;
	Config			cfg_;
	log_hd_t		*l_;

	/* Held for a whole lookup or restore round: one at a time. */
	std::mutex		mu_;
	std::optional<std::chrono::steady_clock::time_point>	last_lookup_;

	std::mutex		stop_mu_;
	std::condition_variable	stop_cv_;
	bool			stopping_ = false;
	std::thread		restore_thread_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__CONTACT_LOOKUP_HPP */

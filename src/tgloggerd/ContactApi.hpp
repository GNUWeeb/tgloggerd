// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__CONTACT_API_HPP
#define TGLOGGERD__CONTACT_API_HPP

#include "ContactLookup.hpp"
#include "InternalApi.hpp"
#include "TDLib.hpp"
#include "DB.hpp"

#include <memory>

namespace tgloggerd {

/*
 * ContactLookup wired to the daemon: Telegram over TDLib::request, the
 * journal over DB. Owns the three, so the daemon holds one object.
 */
class Contacts {
public:
	/* tdlib and db must outlive this object. */
	Contacts(TDLib &tdlib, DB &db, ContactLookup::StoreOwnName store,
		 ContactLookup::Config cfg, log_hd_t *l);
	~Contacts(void);

	ContactLookup &lookup(void) { return *lookup_; }

private:
	std::unique_ptr<ContactTelegram>	tg_;
	std::unique_ptr<ContactJournal>		journal_;
	std::unique_ptr<ContactLookup>		lookup_;
};

/*
 * Register POST /v1/users/{id}/lookup-name (docs/internal-api.md).
 * contacts must outlive the API.
 */
void registerContactApi(InternalApi &api, Contacts &contacts);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__CONTACT_API_HPP */

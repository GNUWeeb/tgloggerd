// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__CONTACT_NAME_HPP
#define TGLOGGERD__MODELS__CONTACT_NAME_HPP

#include <string>
#include <vector>
#include <utility>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * A user's own name versus the name the logging account saved them under.
 *
 * TDLib reports a contact's first/last name as the account saved them.
 * telegram_users.first_name/last_name keep meaning the user's own name:
 * while the user is a contact they hold the last own name seen before,
 * and the contact name is stored apart. Pure logic, no SQL, so it can be
 * tested on its own.
 */

using Name = std::pair<std::string, std::string>; /* first, last */

/* The name columns of a telegram_users row, as stored. */
struct StoredUserName {
	Name			name;
	/* NULL: not seen since migration 000033. */
	std::optional<bool>	is_contact;
	bool			own_name_known = true;
};

/* What TDLib reports now. */
struct ReportedUserName {
	Name	name;
	bool	is_contact = false;
};

/* The name columns to write, and the history rows to add. */
struct UserNamePlan {
	Name			name;		/* first_name, last_name */
	bool			own_name_known = true;
	/* contact_first_name/last_name; nullopt (NULL) if not a contact. */
	std::optional<Name>	contact_name;
	/* telegram_user_hist_name rows to insert, in order. */
	std::vector<Name>	hist;
};

/*
 * Plan the name columns for a user report.
 *
 * old is the stored row, or nullptr for a user seen for the first time.
 * hist_own is the own-name backfill: the newest telegram_user_hist_name
 * row whose name differs from the reported (contact) name. It is only
 * used, and only needs to be looked up, when needsOwnNameBackfill() says
 * so.
 */
UserNamePlan planUserName(const StoredUserName *old,
			  const ReportedUserName &now,
			  const std::optional<Name> &hist_own);

/*
 * True on the first report since migration 000033 of a user who is a
 * contact and whose stored name is the contact name: the name the old
 * logger stored was the account's, and the own name, if any was ever
 * seen, is in the name history.
 */
bool needsOwnNameBackfill(const StoredUserName *old,
			  const ReportedUserName &now);

/* "first last", or whichever of the two is set. */
std::string displayName(const Name &n);

/*
 * The name to show for a user in a message mirrored elsewhere (the
 * Discord forwarder). reported is the name TDLib has for them, which
 * for a contact is the account's contact name. stored is their
 * telegram_users row, or nullptr if there is none.
 *
 * A contact whose own name is known is shown under the stored own name.
 * Everyone else keeps the reported name: for a non-contact it is their
 * own and the freshest, and for a contact with no known own name the
 * stored name is only the contact name standing in for it.
 */
std::string mirroredSenderName(const StoredUserName *stored,
			       const std::string &reported);

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__CONTACT_NAME_HPP */

// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ContactName.hpp"

namespace tgloggerd {
namespace models {

bool needsOwnNameBackfill(const StoredUserName *old,
			  const ReportedUserName &now)
{
	return old && !old->is_contact.has_value() && now.is_contact &&
	       old->name == now.name;
}

UserNamePlan planUserName(const StoredUserName *old,
			  const ReportedUserName &now,
			  const std::optional<Name> &hist_own)
{
	UserNamePlan p;

	if (now.is_contact)
		p.contact_name = now.name;

	if (!old) {
		/*
		 * First seen. A contact's reported name is the account's, so
		 * no own name is known yet and none goes into the history:
		 * the history holds own names only.
		 */
		p.name = now.name;
		p.own_name_known = !now.is_contact;
		if (!now.is_contact)
			p.hist.push_back(now.name);
		return p;
	}

	if (!now.is_contact) {
		/*
		 * The reported name is the user's own. As before the contact
		 * split: on a change, the old own name goes into the history.
		 * An own name seen for the first time (it was a contact all
		 * along) is recorded as the initial one, like a new user's.
		 */
		p.name = now.name;
		p.own_name_known = true;
		if (!old->own_name_known)
			p.hist.push_back(now.name);
		else if (old->name != now.name)
			p.hist.push_back(old->name);
		return p;
	}

	/* A contact: the own name stays as stored, renames are the account's. */
	p.name = old->name;
	p.own_name_known = old->own_name_known;

	if (needsOwnNameBackfill(old, now)) {
		/*
		 * Stored before the split, so the stored name is the contact
		 * name. The newest history row with another name is what the
		 * user was called before the account saved them; without one,
		 * no own name was ever seen.
		 */
		if (hist_own) {
			p.name = *hist_own;
			p.own_name_known = true;
		} else {
			p.own_name_known = false;
		}
	}
	/*
	 * A stored name that differs from the contact name on the first
	 * report since the split was seen while the user was not a contact
	 * (they became one while the daemon was down), so it is kept as the
	 * own name.
	 */
	return p;
}

} /* namespace models */
} /* namespace tgloggerd */

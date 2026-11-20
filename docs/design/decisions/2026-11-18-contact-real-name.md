# Decision log: 2026-11-18, Contact names

Decided while implementing `contact-real-name` (the item is done; see
git history), for the contact-name split (migration 000033,
`src/tgloggerd/models/ContactName.cpp`) and the "Look up real name"
action (`docs/internal-api.md`, `POST /v1/users/{id}/lookup-name`):

- The backfill of current contacts runs in the daemon, not in the
  migration: only TDLib knows who is a contact. On the first report of
  a user since the migration (`is_contact` still NULL), a contact whose
  stored name equals the contact name takes the newest name history row
  with another name as their own name; without one, `own_name_known`
  is 0. The daemon asks TDLib for the contact list once per start, so
  every contact is reported soon after deploy.
- Reason and limit: the history has no record of when a user became a
  contact, so this picks the name they had before the last name change
  the logger saw. For a user who renamed themselves and was then saved
  under their current name, it picks the older name. "Look up real
  name" corrects such a case.
- History holds own names only: a user first seen as a contact gets no
  history row until an own name is seen, and then that name starts the
  history.
- The lookup is limited to one at a time and one per
  `TG_CONTACT_LOOKUP_INTERVAL` seconds (default 300, at least 60),
  counted from lookups that changed a contact.
- `share_phone_number` on the re-add is
  `!userFullInfo.need_phone_number_privacy_exception`: TDLib needs the
  exception exactly when the account's number is not shown to the user.
- A re-add hit by a flood wait of up to 30 seconds is retried, up to 3
  times; a longer one leaves the journal row, retried at the next
  lookup request and at start (backing off up to 10 minutes).

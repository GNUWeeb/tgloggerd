# contact-real-name

Log a user's own name rather than the name the logging account saved
them under as a contact, and keep the contact name apart.

## Why

TDLib reports a contact's first and last name as the account saved
them, not as the user set them. User 742278412 is logged as "Michael
William Jonathan", the account's contact name for them; their own name
was "Moe" until the account saved them as a contact (2026-11-16), as
`telegram_user_hist_name` shows. Every page, search and MCP answer
then shows a name the user never chose, and the name history records
the account's address book instead of the user's renames.

## Scope

- Know which users are contacts: TDLib's `user.is_contact` (and
  `is_mutual_contact`), stored per user.
- Keep the user's own name apart from the contact name:
  - while a user is not a contact, the name TDLib reports is their own
    name, as today;
  - while a user is a contact, keep the last own name seen before they
    became one, store the contact name separately, and do not record
    contact renames in the name history as the user's renames;
  - when a user stops being a contact, TDLib reports their own name
    again and logging resumes as before.
- Backfill: for users who are contacts now, take their own name from
  the newest `telegram_user_hist_name` row written before they became
  a contact, where there is one.
- The web shows the own name, with the contact name as a secondary
  label; search matches both. MCP answers carry the own name.
- An explicit admin action, "Look up real name", on a contact's user
  page (decided with the project owner on 2026-11-18): tgloggerd
  removes the contact, records the own name TDLib then reports, and
  adds the contact back as it was. It runs once per click, for that
  one user, never automatically or in bulk. The result is stored in
  the own-name columns with a "last checked" time.
- Not in scope: any other change to the account's contacts.

## Design notes

- Telegram gives an account no field with a contact's own name; the
  name in `user` is the contact name. What the logger can know is what
  it saw while the user was not a contact. A contact who renames
  themselves while saved is not seen; a user who was already a contact
  before logging began has no known own name, and keeps showing the
  contact name, marked as such.
- Possible extension: a second TDLib session that does not have the
  user as a contact sees their own name. The Telegram bots discordd
  forwards with are such sessions, but a bot can only look up users it
  has met (shares a chat with), so this covers some contacts, not all.
  Decide before building it.
- Schema change: an additive migration on `telegram_users` (contact
  flag, contact name columns) with the web synced in the same series.
- The lookup cycle, as a curated operation on the internal API:
  1. Journal first: a row holding the contact as the account has it
     (contact name, phone number if known, the contact's private note
     from `userFullInfo.note`, and whether the account shared its
     phone number with them), written before anything changes.
  2. `removeContacts([user_id])`; wait for the `updateUser` that
     carries the user's own name, with a timeout.
  3. `addContact(user_id, importedContact{phone, first, last, note},
     share_phone_number)` restores the contact exactly as journalled.
  4. Store the own name and the check time; delete the journal row
     only after the re-add succeeded.
  On startup, any journal row left over (a crash between 2 and 3) is
  re-added before anything else, so a contact is never lost.
- Side effects to state on the page before the admin confirms: the
  user can briefly see a change (mutual-contact status, the account's
  phone number), per-contact settings TDLib does not expose (such as
  a custom photo set for them) are not restored, and contact changes
  are rate-limited by Telegram, so the action refuses to run more
  often than a modest limit (a flood wait is reported, not retried).

## Depends on

Nothing for the passive part; the lookup action needs the internal
API, which is merged (`TDLib::request` and `InternalApi` routes).

## Done when

- "Look up real name" on a contact's page shows their own name and
  the check time, and the contact is still in the account afterwards
  with the same name, phone number and note; a daemon killed between
  removal and re-add restores the contact on its next start.
- User 742278412 shows as "Moe" with "Michael William Jonathan" as the
  contact name; a contact rename in the account does not change the
  logged name or add a name history row; removing the contact makes
  the logger follow the user's own name again.

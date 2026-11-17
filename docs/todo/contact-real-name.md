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
- Not in scope: changing the account's contacts.

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

## Depends on

Nothing.

## Done when

- User 742278412 shows as "Moe" with "Michael William Jonathan" as the
  contact name; a contact rename in the account does not change the
  logged name or add a name history row; removing the contact makes
  the logger follow the user's own name again.

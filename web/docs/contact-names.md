# Contact names and "Look up real name"

Telegram shows the logging account a contact's name as the account saved
them, not as the user set it. The daemon keeps the two apart
(`telegram_users`, migration 000033):

- `first_name`/`last_name`: the user's own name. While the user is a
  contact, the last own name seen before they became one; for a user who
  was a contact before logging began and was never seen otherwise, the
  contact name stands in for it and `own_name_known` is 0.
- `contact_first_name`/`contact_last_name`: the name the account saved them
  under; NULL when they are not a contact. Contact renames do not go into
  `telegram_user_hist_name`.
- `is_contact`, `is_mutual_contact`; `is_contact` is NULL for a user the
  daemon has not seen since the migration (read as not a contact).
- `own_name_checked_at`: the last "Look up real name".

Every page and MCP answer that shows a user's name shows the own name. The
user page adds the contact name as a secondary label, a contact badge, and
a note when no own name is known. Search matches both names
([search-api.md](search-api.md): `any_name`, `contact_first_name`,
`contact_last_name`, `is_contact`); the forwarding chat picker matches
either name.

## Look up real name

An admin action on a contact's user page. The daemon removes the contact,
reads the own name Telegram then reports, and adds the contact back exactly
as it was; the full cycle, journal and limits are in
[../../docs/internal-api.md](../../docs/internal-api.md)
(`POST /v1/users/{id}/lookup-name`).

| Route | Filters | Does |
|---|---|---|
| `GET /users/{id}/lookup-name` | AuthFilter, AdminFilter | Confirmation page: what the cycle does and its side effects, with the form. |
| `POST /users/{id}/lookup-name` | AuthFilter, AdminFilter | Form field `csrf` (the session token). Calls the daemon; on success redirects to `/users/{id}?looked_up=1`, otherwise shows the daemon's answer on the confirmation page. |

- The button shows only to admins, and only for a user who is a contact.
  The POST refuses a user the database does not record as a contact before
  asking the daemon, which checks again.
- Each POST that reaches the daemon is written to `web_audit` as
  `contact_lookup`, with `user <id>: ok` or the daemon's error code.
- While a journal row in `telegram_contact_lookups` is left for a user (a
  contact removed and not added back yet), the user page and the
  confirmation page say so.
- The web reaches the daemon at `WEB_TGLOGGERD_API` (in the deployment
  `http://10.0.88.5:8091`) with the bearer key derived from `WEB_APP_KEY`
  through the shared client (`src/daemon/Api.cpp`, `lookupRealName`).
  Empty: the page explains that the action is not available. The request
  waits up to 120 seconds.

## The JSON API (Go service)

    POST /api/users/{id}/lookup-name

The same action as a JSON operation (`webapi/openapi.yaml`, tag
`contacts`), with the session's CSRF token in `X-CSRF-Token` and no
body. It is for superadmins only (the `lookup_name` ability); to
anyone else, and for a user the viewer may not see or the logger never
saw, it does not exist (404). A user the database does not record as
a contact is 409 `not_a_contact` before the daemon is asked. Success is
the daemon's answer (`user_id`, the own and the contact names,
`checked_at`); the daemon's errors keep their code and status (409,
429 and 503 with `Retry-After`, 504), anything else is 502 `daemon`.
Every request that reaches the daemon writes the same `contact_lookup`
row as above.

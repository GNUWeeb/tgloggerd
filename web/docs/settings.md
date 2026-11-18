# Account settings

Every account has its own settings (docs/design/web-accounts.md
section 7), kept by the Go service (`webapi/`) in the web database
(web migration 000005) and described in webapi/openapi.yaml (tag
account). Every write carries the session's CSRF token.

## The pages

The three settings pages are served by the Go service, each with its
loader (`/api/page/settings...`):

- `/settings` (route settings): the account (login name, display name
  or else the login name, role, email address, not verified until
  web-email) and the security notices it is sent, through the text
  lookup.
- `/settings/preferences` (settings_preferences): the stored
  preferences, the time zones offered, the landing pages the menu
  shows plus the chats of the pinned groups, the pinned groups the
  scope sees with their photo links, and the saved searches. GET
  /api/account/preferences answers the same data.
- `/settings/sessions` (settings_sessions): see Sessions.

## Preferences

    GET /api/account/preferences    the preferences page's data
    PUT /api/account/preferences    replace every preference; 200 {prefs}
    PATCH /api/account/preferences  change only the keys given; 200 {prefs}

| Key | Values | Site default |
|---|---|---|
| timezone | an IANA zone name (`Asia/Jakarta`, `UTC`) | WEBAPI_DEFAULT_TIMEZONE, Asia/Jakarta |
| date_format | `24h`, `12h`, `iso` | WEBAPI_DEFAULT_DATE_FORMAT, 24h |
| lang | `en`, `id` | WEBAPI_DEFAULT_LANG, en |
| theme | `system`, `light`, `dark` | WEBAPI_DEFAULT_THEME, system |
| sidebar | `open`, `collapsed` | open |
| rows_per_page | 25, 50, 100 | 25 |
| landing | a path on this site, at most 255 bytes | `/` |
| chat_auto_load, chat_live | true, false | true |
| chat_media | `auto`, `click` | auto |
| chat_show_deleted, chat_show_edited, chat_show_service | true, false | true |

- A key left out or null is the site default, and stays unset: a
  change of the site default reaches every account that did not choose
  for itself. Each key set is one row of web_user_settings, holding its
  JSON value.
- A key that is not in the table, or a value it does not take, refuses
  the whole body: 400 bad_request with the key in `field`.
- PATCH changes only the keys its body names (null puts one back to
  the site default) and keeps the others as stored, so a control that
  saves one key, such as the theme toggle or the menu's collapse,
  cannot undo another page's save made at the same time. It refuses
  what PUT refuses and answers every preference as now stored.
- The site defaults of the first four keys are configuration
  (decisions/2026-11-18-web-rollout.md), checked at start; the others
  are fixed.
- `id` is accepted but has no texts yet, so it is shown in English
  until a catalogue for it exists (see Texts).

### Where they apply

POST /api/login and GET /api/session answer the account's `landing`
page, where a sign-in without a next page goes.

Every page's data carries the settings in effect, the account's own
or the site's, in `viewer` (`timezone`, `date_format`, `lang`,
`theme`, `sidebar`, `rows_per_page`, `chat`), for a signed-out visitor
too. Dates stay UTC in the database and in every JSON answer; the
pages show them in the viewer's zone and format, the same on the
server's render and in the browser. The document's `<html>` carries `lang` and, unless the
theme follows the system, `data-theme`.

## Pinned groups and saved searches

    PUT    /api/account/pinned/{group_id}      pin a group; 204
    DELETE /api/account/pinned/{group_id}      unpin it; 204
    POST   /api/account/saved-searches         {title, url}; 201 {id}
    DELETE /api/account/saved-searches/{id}    204

- Only a group the request scope sees can be pinned; any other id
  answers 404, as a group that does not exist. `viewer.pinned` lists
  the pins the scope sees, in the order pinned, so a group the
  account loses access to drops out of its menu, while the pin stays
  and comes back with the access. Unpinning a hidden group answers
  404 too.
- A saved search is a title (trimmed, 1 to 128 characters) and the
  search page's path and query string on this site (one leading
  slash, no backslash, no control character, at most 2048 bytes);
  another account's id answers 404.
- An account keeps at most 50 pins and 100 saved searches; one more
  answers 400 too_many.

## Sessions

    GET    /api/account/sessions                    {sessions}
    DELETE /api/account/sessions/{id}               sign one out; 204
    POST   /api/account/sessions/sign-out-others    200 {csrf}

The sessions page (`/settings/sessions`) shows the same list. Each
session is a row of web_sessions, keyed by the SHA-256 of its cookie
(decisions/2026-11-18-sessions-list.md):

- `device` is a short description of the User-Agent it signed in
  with ("Firefox on Linux"), `ip` the address of its last use, `place`
  empty (no location source yet), `started` and `last` UTC times. The
  current session comes first, the others by last use.
- A session signed in on the Go service is listed at once. One signed
  in on the Drogon web is listed once it reaches the Go service (any
  page or API request); the last use is recorded there, once a minute
  at most.
- Signing one out ends its row: both services refuse its cookie from
  the next request (the Drogon AuthFilter reads the row in the same
  query as the epoch). Signing this browser's own session out clears
  its cookie, as a logout does; POST /api/logout ends the row too.
- "Sign out all others" bumps the account's session epoch, as a
  password change does, so every other session ends on both services,
  listed or not. This session gets a new cookie that keeps its start
  and device, and a new CSRF token, which the answer carries; the
  client must send it from then on.
- Rows of an older epoch and expired rows are deleted when the account
  signs in or lists its sessions. Audit rows: `session_end` (detail
  `session <id>`) and `sessions_signout_others`.

## Texts

User-facing texts go through a lookup by language
(`webapi/internal/i18n`), so other languages can be added without
code changes: each language is one JSON file,
`webapi/internal/i18n/catalog/<lang>.json`, of key to text, embedded
at build time. A key missing in a language falls back to English;
English has every key (a test checks every key the code looks up).
Adding Indonesian is adding `id.json`. The texts follow the account's
language, else the site's. The settings operations and pages use it;
the older messages of signing in and of the accounts administration
are still written in English in the code and move to the lookup when
they are next changed.

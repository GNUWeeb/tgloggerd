# web-fetch-permission

Make fetching an unstored file from Telegram an explicit action:
superadmins and admins may, members only with `fetch_files`.

## Why

web-fetch-unstored-media lets the web ask the daemon to fetch files
the store does not hold. Each fetch costs Telegram traffic and disk,
and the project owner wants it to be a deliberate action ("Fetch from
Telegram", approved in the mockup's file preview) of accounts allowed
to do it, not something every page view triggers.

## Scope

- A "Fetch from Telegram" action for unstored files (`on_disk = 0`),
  in the web API used by the file preview, allowed for superadmins
  and admins and for members holding `fetch_files`:
  it asks the daemon to fetch and store the file, then the file can be
  viewed and downloaded like a stored one (`view_files`).
- Fetches triggered by plain page views or anonymous `/files/<token>`
  requests are limited accordingly (small files only, or none, as
  agreed when implementing).
- `web/docs/media.md` updated.

## Design notes

- Builds on the fetch path of web-fetch-unstored-media and the roles
  and grants of web-roles and web-member-access
  (docs/design/web-accounts.md); agree the exact rule for page views
  and tokens with the project owner before building.
- The fetch decision is one function, `media::mayFetch()` in
  `web/src/media/Fetch.cpp`: `/files/<token>` asks it before calling
  the daemon, and the chat pages set `fetchable` from it
  (`media::markFetchable()`), so gating it there gates both. Today it
  allows any signed-in request and, without a session, files under
  `TG_MAX_STORE_FILE_SIZE` (web/docs/media.md).

## Depends on

- `web-member-access`
- `go-port-media`

## Done when

- A member with `fetch_files` fetches an unstored file from its
  preview and then views it; a member without it gets 403; the
  agreed rule for page views and tokens holds.

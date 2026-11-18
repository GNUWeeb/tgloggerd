# web-user-settings

Make every per-account setting take effect where it applies. The
settings themselves are built: the preferences, pinned groups, saved
searches and the sessions page are stored and served by the Go
service (web/docs/settings.md), and every page's viewer carries the
settings in effect.

## Why

A setting that is stored but not applied does nothing for the
account that chose it. The React pages so far use the timezone, the
date format, the theme and the chat's live switch from the viewer;
the rest is saved but not yet followed.

## Scope

- The list page loaders (users, groups, files, group and private
  messages) take `viewer.rows_per_page` as the page size when the
  request names none.
- The React application (frontend session, web/app):
  - goes to `landing` from the sign-in answer when no next page was
    asked for;
  - follows `viewer.rows_per_page` and the chat switches
    (`chat.auto_load`, `chat.media`, `chat.show_deleted`,
    `chat.show_edited`, `chat.show_service`);
  - after "Sign out all others", sends the CSRF token the answer
    carries (setSessionCsrf), since the session's cookie was reissued;
  - leaves out the session's `place` while it is empty.
- Not in scope: TOTP and per-user MCP tokens (later items), an
  Indonesian catalogue (adding `webapi/internal/i18n/catalog/id.json`
  needs no code change when it is wanted).

## Design notes

- The database stores UTC; the pages convert only when rendering,
  with the viewer's timezone and format, the same on the server's
  render and in the browser.
- The sessions design is in
  design/decisions/2026-11-18-sessions-list.md.

## Depends on

Nothing.

## Done when

- Two accounts with different timezones see the same message at their
  own local times on the chat and message pages.
- Every listed setting, changed on the preferences page, changes what
  the next page shows.

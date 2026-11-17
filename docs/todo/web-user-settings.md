# web-user-settings

Settings per account, starting with the timezone used to show dates
and times.

## Why

Every account sees dates in the server's zone and shares one set of
display defaults (docs/design/web-accounts.md section 7).

## Scope

- One key-value table of known keys with validation, a settings page
  and API: timezone, date/time format, UI language, theme, rows per
  page, landing page, chat view defaults (auto-load, live, media
  loading, deleted/edited/service messages), pinned groups, saved
  searches; a "my sessions" page with sign-out.
- Dates rendered in the account's timezone and format, server side
  and in the JSON the pages render.
- Not in scope: TOTP and per-user MCP tokens (later items).

## Design notes

- The database stores UTC; convert only when rendering.

## Depends on

- `web-roles`

## Done when

- Two accounts with different timezones see the same message at their
  own local times; every listed setting persists and takes effect.

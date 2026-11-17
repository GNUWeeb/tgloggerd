# web-accounts

Several web accounts with roles and permissions, managed by an
administrator from the web.

## Why

The web has a single admin login. Others who should see the archive
must share it, and nothing limits what they see or do. The redesigned
web UI (mockup approved by the project owner on 2026-11-18, pages
`/settings/accounts`) needs real accounts behind it.

## Scope

- Accounts administration, for an admin or a member with
  `manage_accounts`: create, edit and disable accounts; reset a
  password; sign an account out everywhere (bump its `session_epoch`).
  A new account gets a temporary password, optionally with a forced
  change at its first sign-in.
- Roles: `admin` (everything; permissions and scopes are ignored) and
  `member`. The existing `viewer` role in `web_users.role` is migrated
  to `member` with the permissions it has today.
- Member permissions (flags): `view_files` (open or download stored
  media), `fetch_files` (have the daemon fetch unstored media from
  Telegram), `view_private` (private messages), `manage_integrations`
  (platform forwarding and MCP admin), `manage_accounts`.
- Enforcement: every route checks the permission it needs (in
  `AuthFilter` and the admin filters), refusing with 403; pages a
  member cannot use are left out of the navigation.
- An accounts JSON API (CSRF on every write), audited as other admin
  actions are.
- A web DB migration (`web/migrations/000004_*`): additive ALTERs and
  new tables only; the web DB is never dropped (its admin is not
  seeded again).
- `web/docs/` describes accounts, roles, permissions and the API, in
  the same commits.
- Not in scope: access scopes (web-access-scopes); the fetch action
  itself (web-fetch-permission, on the fetch path that is already
  there).

## Design notes

- Today: `web_users` (role `admin` or `viewer`, `session_epoch` from
  web migration 000002), signed session cookies checked by
  `AuthFilter` against the account's epoch, `AdminFilter` for admin
  pages; see `web/docs/` and `web/src/auth/`.
- Keep the permission set a small fixed list (columns or a flags
  column); finer-grained rules may come later.
- The frontend session owns the UI; this item delivers the backend,
  the API contract in `web/docs/`, and whatever of the current pages
  must change so nothing breaks before the React UI replaces them.

## Depends on

Nothing.

## Done when

- An admin creates a member with a temporary password, the member is
  forced to change it at first sign-in, sees only what their
  permissions allow (403 otherwise), and is signed out everywhere when
  the admin asks; a disabled account cannot sign in.

# web-roles

Several web accounts with three roles (superadmin, admin, member),
managed from the web.

## Why

The web has one shared admin login. The project owner wants a
multi-user dashboard (docs/design/web-accounts.md).

## Scope

- Web DB migration (`web/migrations/000004_*`, additive; the web DB
  is never dropped): role superadmin/admin/member, disabled flag,
  forced password change, created_by. Today's `admin` becomes
  superadmin, `viewer` becomes member.
- Accounts administration and its JSON API (CSRF on writes): create
  (with an email address; the set-password link comes with
  web-email), edit, disable,
  reset password, sign out everywhere (session_epoch).
- Who may manage whom (web-accounts.md section 2); the last active
  superadmin cannot be disabled or demoted; every change audited.
- The request scope object of web-accounts.md section 9, with roles
  only for now (later items add federations, restrictions, grants).
- `web/docs/` in the same commits.
- Not in scope: federations, restrictions, member grants, ownership.

## Design notes

- Built in the Go service (docs/design/web-go.md). Today's model:
  `web_users` (role admin/viewer, session_epoch), AuthFilter and
  AdminFilter in `web/src/auth/`. While Drogon still serves pages, its
  AdminFilter must treat superadmin and admin as today's admin.

## Depends on

Nothing.

## Done when

- A superadmin creates an admin and a member, each signs in (forced
  password change when set) and sees what its role allows; the last
  superadmin cannot be disabled; sign-out-everywhere works.

# web-email

Give every web account an email address, and send set-password links,
password resets and security notices by email.

## Why

Accounts need a way to receive their first sign-in, reset a forgotten
password and hear about changes to their account
(docs/design/web-accounts.md section 8).

## Scope

- A required, unique, verified email address per account (web DB
  migration, additive); accounts without one add and verify it right
  after their next sign-in.
- Outbound mail through an SMTP relay configured in the environment
  (host, port, STARTTLS or TLS, user, password, sender), sent
  asynchronously with retries, logged without links; `.env.example`
  documents it.
- One-time links (stored as hashes, single-use, expiring): set the
  first password, reset a forgotten password, verify an address.
- Security notices: password changed or reset, address changed (also
  to the old address), sign-in from a new device, account disabled,
  role changed.
- Account creation by an admin sends the set-password link instead of
  a temporary password: `POST /api/accounts` and
  `POST /api/accounts/{id}/password-reset` answer `temporary_password`
  until then (decisions/2026-11-18-temporary-passwords.md); that path
  goes, and `resend-invite` (planned in webapi/openapi.yaml) is served.
- `web_users.email` exists since web migration 000004 (unique,
  NULL-able, recorded at creation but unverified); this item adds the
  verification and requires it. The gate is in place: set
  `scope.Viewer.EmailVerified` from the account in
  `Server.requestScope`, and `RequireSession` answers 403
  `email_required` for routes that do not pass `scope.EmailRequired`.
- `web/docs/` describes the flows; the OpenAPI description covers the
  endpoints.

## Design notes

- Built in the Go service. Mail templates in plain text plus simple
  HTML, in the account's language setting when it exists.
- Links: `<site>/password/set#token=<token>` (invitations and resets)
  and `<site>/account/email/verify#token=<token>`. The token is in the
  fragment, so it never reaches the server's or nginx's logs or a
  Referer header; the page posts it (`POST /api/password/set`,
  `POST /api/account/email/verify`) only when the person acts, so a
  mail scanner opening the link does not use it up. The pages' GETs
  must consume nothing.
- The pages `password_forgot`, `password_set` and `email_verify` are
  served to signed-out and signed-in visitors alike.
- Gate for accounts with no verified address: for every page path
  but those three, `GET /api/page` answers the `email_required` route
  (props: the pending address, if any, and the page asked for) until
  an address is verified; the account's other API operations answer
  403 `email_required`, except session, logout, the account's email
  operations and the link operations. One check in the request scope,
  not one per handler.
- Error codes the pages read: `bad_link` and `weak_password` (set,
  verify), `rate_limited` and `csrf` (forgot); any other error shows
  its message. `POST /api/password/forgot` carries the pre-login CSRF
  token, as a sign-in does.
- All of it is described in webapi/openapi.yaml (planned until built).

## Depends on

Nothing.

## Done when

- A new account receives a set-password link and signs in with the
  password it chose; a forgotten password is reset by email; each
  security notice arrives; an unverified address receives no reset.

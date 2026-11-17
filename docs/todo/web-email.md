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
  a temporary password.
- `web/docs/` describes the flows; the OpenAPI description covers the
  endpoints.

## Design notes

- Built in the Go service. Mail templates in plain text plus simple
  HTML, in the account's language setting when it exists.

## Depends on

- `web-roles`

## Done when

- A new account receives a set-password link and signs in with the
  password it chose; a forgotten password is reset by email; each
  security notice arrives; an unverified address receives no reset.

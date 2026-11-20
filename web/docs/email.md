# Account email

A web account may have an email address, used for set-password links,
password resets and security notices
([../../docs/design/web-accounts.md](../../docs/design/web-accounts.md)
section 8, decisions/2026-11-18-web-email.md). Email is optional, for
each account and for the whole service
([../../docs/design/decisions/2026-11-18-email-optional.md](../../docs/design/decisions/2026-11-18-email-optional.md)):
an account needs a username and a password, no account is asked to
add an address, and links and notices go out only when the account
has an address and mail is set up. Without them nothing is sent and
nothing waits for mail. The Go service (`webapi/`) does all of it; the
operations are in [../../webapi/openapi.yaml](../../webapi/openapi.yaml)
(see Gates and Email links in its description).

## Addresses

- `web_users.email` is the account's address and
  `web_users.email_verified_at` says when it was verified (web
  migration 000007). Only a verified address receives resets and
  notices.
- An address is verified by its verification link, or by setting the
  password from a set-password link sent to it (it received the link).
- An account changes its own address with
  `POST /api/account/email {email, password}`: the current password
  confirms it. With mail, a verification link goes to the new address
  (`{"verification_sent": true}`) and the old address stays the
  account's until the link is used; then both are told (the old one
  only if it was verified). A newer request replaces the earlier link.
  Without mail, the address is recorded at once, unverified
  (`{"verification_sent": false}`). An empty address removes the
  account's. An address another account has answers 409
  `email_taken`.
- An admin may correct the address of an invited account only (one
  that has not set its own password); with mail, the invitation then
  goes to the new address and the earlier one stops working. A
  superadmin may change any account's address, for an account that
  lost its mailbox: the change is audited, the old address is told,
  and the new one is unverified and, with mail, gets a verification
  link.

## No email gate

No account is asked for an address: there is no gate on a missing or
unverified one, in the API or on the pages. The only gate is the
forced password change of an account whose password was set for it
(accounts.md).

## Links

    <site>/password/set#token=<token>           set the password (invitation, reset)
    <site>/account/email/verify#token=<token>   verify an address

`<site>` is `WEB_PUBLIC_URL` (else `TG_DISCORD_PUBLIC_URL`). The token
is 32 random bytes in base64url; `web_email_links` keeps only its
SHA-256, with the account, the purpose (`password` or `verify`), the
address it went to, its expiry and when it was used.

- The token is in the URL's fragment, which the browser never sends:
  it reaches no server or nginx log and no Referer header. The pages
  (`password_set`, `email_verify`, and the form `password_forgot`)
  change nothing on GET; they post the token, to
  `POST /api/password/set {token, new_password}` or
  `POST /api/account/email/verify {token}`, only when the person acts,
  so a mail scanner that opens the link uses nothing up.
- A link works once. Using it and checking it are one transaction
  (`SELECT ... FOR UPDATE`), so two requests with the same token
  cannot both pass. Sending a new link of the same purpose to an
  account ends its earlier ones; so does using one.
- Links expire: invitations and an admin's resets after
  `WEBAPI_INVITE_LINK_TTL`, resets asked for on the forgotten-password
  form after `WEBAPI_RESET_LINK_TTL`, verifications after
  `WEBAPI_VERIFY_LINK_TTL` (seconds). A disabled account's links do
  not work.
- Setting a password from a link ends every session of the account,
  ends a forced change and verifies the address the link went to. The
  password is checked first (at least 8 bytes, 400 `weak_password`),
  so a short one leaves the link working. Any link that does not work
  answers 400 `bad_link`.

## New accounts and resets

- `POST /api/accounts` gives the account the first password its
  creator typed, or a generated temporary one answered once; the
  account changes it at its first sign-in (accounts.md). With
  `"invite": true` it gets no password and the invitation goes to its
  address instead, which needs an address (409 `no_email`) and mail
  (409 `mail_off`).
- `POST /api/accounts/{id}/resend-invite` sends an invited account a
  new invitation (an account reset by link gets the reset text again).
  Earlier links stop working. 409 `mail_off` without mail.
- `POST /api/accounts/{id}/password-reset` ends the sessions. With a
  verified address and mail it removes the password and sends a
  set-password link (`{"link_sent": true}`); otherwise it gives a
  temporary password, answered once (`{"link_sent": false,
  "temporary_password": ...}`).

## Forgotten passwords

`POST /api/password/forgot {username}` (the `password_forgot` page)
takes a username, or an address when it has an @, and carries the
pre-login CSRF token of `GET /api/login`, whose cookie is set for this
path as well. It answers 202 whether or not such an account exists,
and whether or not mail is set up: a reset link goes out only to an
active account's verified address while mail is on, and the password
stays until the link is used.

- A client address may ask 5 times an hour; the 6th answers 429
  `rate_limited`, which says nothing about any account.
- An account gets at most 3 links an hour; past that the request is
  answered the same and audited (`password_forgot_ratelimited`), but
  nothing is sent, since a 429 would tell that the account exists.
- Each request is audited: `password_forgot` (with the account),
  `password_forgot_unknown` (with the name asked for).

## Security notices

Always sent to the verified address while mail is set up, never to an
unverified one:

| Notice | When |
|---|---|
| password changed | `POST /api/account/password`; a password set from a reset link (not an account's first password) |
| address changed | the new address verified: to the new address and to the old one; a superadmin changed it: to the old one |
| new device | a sign-in from a device (browser and system, as the sessions page names it) the account had not used, when it had used another before |
| disabled | the account was disabled |
| role changed | the account's role was changed |

Known devices are kept in `web_known_devices` by the SHA-256 of their
description; they outlive the session records.

## Texts

Every message is plain text plus a simple HTML version of the same
text, in the account's language and time zone (its preferences, else
the site's). The texts are in the i18n catalogue
(`webapi/internal/i18n/catalog/<lang>.json`): `mail.<kind>.subject`,
`mail.<kind>.body` and `mail.footer`, with `{name}`, `{username}`,
`{site}`, `{time}`, `{link}`, `{expires}` and the message's own values.
A language without a text falls back to English.

## Sending

Mail goes through one SMTP relay (`webapi/internal/mail`). Sending
never blocks a request: a message is queued in memory and delivered by
a worker; a temporary failure (a 4xx answer, a connection that failed)
is retried after 30 s, 2, 10 and 30 minutes, then 1, 2, 4 and twice 8
hours, a permanent one (5xx) is not. Every delivery and failure is
logged with the message's kind, recipient and subject, never its body,
so no link reaches a log.

The queue is not kept in the database, because the links it carries
are stored nowhere else in plain text. Messages still queued when the
service stops are lost (the log counts them); the links can be sent
again (resend-invite, the forgotten-password form, a new address
request).

    WEBAPI_SMTP_HOST       the relay; unset turns mail off (nothing is
                           sent; first passwords and resets are
                           passwords answered once)
    WEBAPI_SMTP_PORT       default 587, or 465 with tls
    WEBAPI_SMTP_SECURITY   starttls (default; refused when the relay
                           does not offer it), tls, or none (a relay on
                           the same host only)
    WEBAPI_SMTP_USER, WEBAPI_SMTP_PASSWORD
                           AUTH PLAIN, sent only over TLS (or to
                           localhost); unset sends without
    WEBAPI_SMTP_FROM       the sender, "Name <address>" or an address;
                           required with a relay
    WEB_PUBLIC_URL         the site the links point to (falls back to
                           TG_DISCORD_PUBLIC_URL); required with a relay

The tests send to `mailtest`, an SMTP server on a loopback port the
system picks, and read what it caught; no real mail is ever sent by
them.

# Decision log: 2026-11-18, Email is optional

Decided with the project owner on 2026-11-18, for
[../web-accounts.md](../web-accounts.md) section 8; it replaces parts
of [2026-11-18-web-email.md](2026-11-18-web-email.md) and
[2026-11-18-temporary-passwords.md](2026-11-18-temporary-passwords.md):

- Email is optional, for each account and for the whole service. An
  account needs at least a username and a password; an address is
  extra.
- Accounts without an address are never asked to add one: there is
  no gate on a missing or unverified address.
- When an account is created, its creator either types its first
  password or leaves it empty to have the server generate a one-time
  temporary password, shown once. Either way the account changes it
  at its first sign-in. When the account has an address and mail is
  configured, the creator may send a set-password link instead.
- A password reset sends a link when the account has a verified
  address and mail is configured; otherwise it gives a one-time
  temporary password, shown once.
- Links, address verification and security notices work as section 8
  describes whenever an address and mail are both there; without
  them, nothing is sent and nothing waits for mail.

# Decision log: 2026-11-18, First passwords before email

Decided with the project owner on 2026-11-18, for
[../web-accounts.md](../web-accounts.md) sections 2 and 8:

- Until set-password links exist (web-email), a new account's first
  password is a random one-time temporary password the server
  generates when an account is created or reset, shown once to the
  account's creator and never stored in plain text or logged.
- The account must change it at its first sign-in before it can use
  the dashboard or the API.
- When web-email lands, creation and resets switch to set-password
  links and the temporary password path is removed.

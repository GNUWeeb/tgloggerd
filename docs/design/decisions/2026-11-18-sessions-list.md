# Decision log: 2026-11-18, Listing and ending sessions

Taken while building `web-user-settings`, within
[2026-11-18-web-accounts-details.md](2026-11-18-web-accounts-details.md)
("sessions get a server-side table, session id in the cookie ...; the
Drogon AuthFilter checks the same table"), choosing the smallest
design that works while the Drogon web still reads the session cookie.

- **A session is its cookie.** The cookie format stays
  `<uid>\n<exp>\n<epoch>\n<role>\n<username>`, signed as before; no
  field is added, since the Drogon web parses exactly five. The
  session's id in `web_sessions` is the SHA-256 of the cookie value,
  which both services compute without a new secret. The value is
  canonical (a token opens in one spelling only), so a copied cookie
  cannot be respelled to dodge its row. The API names a session by
  its row id, never by the hash.
- **Each cookie must be distinct.** Two sign-ins of one account in
  the same second, or any two when sessions never expire (expiry 0),
  would make the same cookie. The Go service records the session at
  sign-in and, when the hash is taken, takes one second off the
  expiry and tries again; with sessions that never expire it uses the
  end of the year 9999 instead of 0. The Drogon login page, while it
  serves, keeps its own expiry and gives a never-expiring session a
  random expiry in the last years of 9999 instead of 0, so its
  sessions are told apart too.
- **Recorded where seen.** The Go service records a session at its
  own sign-in (device from the User-Agent, address), or the first
  time a valid cookie reaches it (one signed in by the Drogon web or
  before this change; the start is the expiry minus the session
  timeout). It records the last use and address at most once a minute
  per session. The Drogon web writes nothing: a session used only
  there is listed once it reaches the Go service, and is ended by
  "sign out all others" in any case.
- **Ending.** Signing one session out sets `revoked_at` on its row;
  both services refuse a cookie whose row is ended (the Drogon
  AuthFilter in the same query as the epoch). A sign-out
  (POST /api/logout) ends the session's row as well, recording an
  ended row when it had none, so a copy of the cookie stops working.
  "Sign out all others" bumps the account's session_epoch, as a
  password change does, so every other session ends on both services
  whether or not it was ever recorded; this session gets a new cookie
  that keeps its start and device, and the answer carries the new
  CSRF token.
- **Pruning.** Rows of an older epoch are dead through the epoch and
  expired rows through their expiry; both are deleted when the
  account signs in or lists its sessions. An ended row stays until
  then, since deleting it would let its cookie be recorded afresh.
- **No location.** The sessions page shows the address; `place` is
  empty until a geolocation source exists.

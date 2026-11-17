# go-web-auth

Authentication in the Go service that is interchangeable with the
Drogon web's: sessions, passwords, CSRF, file tokens and the daemon
client.

## Why

During the migration both services serve the same users; signing in
on one must sign in on both, and links minted by one must work on the
other (docs/design/web-go.md section 3).

## Scope

- Session cookies: the same signing and contents as
  `web/src/auth/Session.cpp` and `Token.cpp`, the `session_epoch`
  check of `AuthFilter`, the same cookie names and flags.
- Login, logout, the login rate limit and audit rows, as the Drogon
  web does them; password verification and hashing compatible with
  the stored libsodium `crypto_pwhash_str` (Argon2id) strings.
- CSRF tokens as `web/src/auth/Csrf.cpp` makes them.
- File tokens as `web/src/auth/FileToken.cpp` (keyed BLAKE2b and
  XChaCha20 from `WEB_APP_KEY`).
- The daemon's internal API client: bearer derivation, timeouts,
  error mapping, as `web/src/daemon/Api.cpp`.
- Tests: cookies, CSRF and file tokens minted by the C++ code verify in
  Go and the other way round (fixtures generated from the C++ code).

## Design notes

- Read the C++ sources named above; reproduce byte for byte, do not
  redesign. A new scheme can come later, once Drogon is gone.

## Depends on

Nothing.

## Done when

- A user signed in on the Drogon web is signed in on the Go service
  and the other way round; sign-out-everywhere ends both; a Go-minted
  file token opens in Drogon and the other way round.

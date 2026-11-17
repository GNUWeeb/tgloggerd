# Decision log: 2026-11-18, Password hashing

Decided with the project owner on 2026-11-18, for
[../web-accounts.md](../web-accounts.md) section 2:

- Account passwords are hashed with Argon2id, in the string format of
  libsodium's crypto_pwhash_str, by the Drogon web and the Go service
  alike. bcrypt is not used anywhere.
- Passwords longer than 72 characters are allowed and used in full;
  there is no upper limit below the size limit of the request.

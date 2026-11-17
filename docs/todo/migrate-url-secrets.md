# migrate-url-secrets

Keep database passwords out of logs and working with any character in
`run.sh`'s migrations, and keep the MySQL root password out of
processes that do not need it.

## Why

`run.sh` builds the daemon's and the web's migration URLs with their
database passwords inside: `migrate` prints the whole URL when it
cannot connect, and a password containing URL-special characters
(`@`, `/`, `:`) breaks the URL. Every service also receives
`MYSQL_ROOT_PASSWORD` through the shared environment file, though only
the grants step needs it.

## Scope

- Percent-encode the passwords in every migration URL and mask them in
  any output, as the web grants step does.
- Remove `MYSQL_ROOT_PASSWORD` from the environment of the processes
  `run.sh` starts once the grants step is done.

## Depends on

Nothing.

## Done when

- A failed migration connection prints no password; a password with
  URL-special characters migrates; the running daemon, web and discordd
  have no `MYSQL_ROOT_PASSWORD` in their environment.

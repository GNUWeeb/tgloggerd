# web-users-init-grants

Make the MySQL init script work on a fresh data directory, so a new
deployment gets its web database and users.

## Why

`docker/mysql/init/10-web-users.sh` grants table-level privileges on
logger tables (for example `telegram_discord_webhooks`) when MySQL is
first initialised. Those tables do not exist yet at that point (the
daemon's migrations create them later), and MySQL 9.7 refuses a
table-level grant on a missing table (ERROR 1146), so the script stops:
a fresh deployment gets no web database and no web users. Its comment
says such grants need no table, which is wrong. Existing deployments
are not affected.

## Scope

- Create the web database and users at init time, and apply the
  table-level grants once the tables exist (for example from the
  daemon's start after migrations, or a grants step run by run.sh),
  idempotently.
- Correct the comment; document the procedure in `web/README.web`.

## Depends on

Nothing.

## Done when

- A fresh MySQL data directory plus a first start of the stack ends
  with working web users and every grant the web needs, and a second
  start changes nothing.

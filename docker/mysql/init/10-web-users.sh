#!/bin/bash
#
# MySQL first-initialization hook: create the web interface's own database and
# its two restricted users from the credentials in env/ (env/README).
#
# The official MySQL image runs every executable file in
# /docker-entrypoint-initdb.d exactly once -- only while the data directory is
# empty, and after it has created MYSQL_DATABASE / MYSQL_USER. On later starts
# the data directory already exists, so this does not run again and the
# accounts keep whatever they were first initialized with.
#
# Only grants that need no logger table are given here; the table-level ones
# follow the daemon's migrations (docker/mysql/web-grants.sql, run.sh).
set -euo pipefail

: "${MYSQL_ROOT_PASSWORD:?}" "${MYSQL_DATABASE:?}"
: "${WEB_DB_RO_USER:?}" "${WEB_DB_RO_PASSWORD:?}"
: "${WEB_DB_APP_USER:?}" "${WEB_DB_APP_PASSWORD:?}" "${WEB_DB_APP_NAME:?}"

# The values go into SQL as they are written in the env files, so each is
# quoted for where it goes: a string literal (user names, passwords) doubles
# its quotes and backslashes, a backquoted name (databases) doubles its
# backquotes. Unquoted, a password holding a quote or a backslash would
# break the statement or, worse, end the literal early.
lit() {
	local s=${1//\\/\\\\}
	printf "'%s'" "${s//\'/\'\'}"
}
ident() { printf '`%s`' "${1//\`/\`\`}"; }

app_db=$(ident "$WEB_DB_APP_NAME")
logger_db=$(ident "$MYSQL_DATABASE")
ro_user=$(lit "$WEB_DB_RO_USER")
ro_pass=$(lit "$WEB_DB_RO_PASSWORD")
app_user=$(lit "$WEB_DB_APP_USER")
app_pass=$(lit "$WEB_DB_APP_PASSWORD")

# The root password reaches the client through its environment, not its
# command line, where any process in the container could read it.
MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --protocol=socket -uroot <<SQL
CREATE DATABASE IF NOT EXISTS ${app_db}
  CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Read-only browsing over the logger's schema.
CREATE USER IF NOT EXISTS ${ro_user}@'%' IDENTIFIED BY ${ro_pass};
GRANT SELECT ON ${logger_db}.* TO ${ro_user}@'%';

-- The table-level write grants on logger tables cannot be given here: those
-- tables do not exist until the daemon's migrations run, and MySQL refuses a
-- table-level GRANT on a missing table. run.sh applies them after the
-- migrations, from docker/mysql/web-grants.sql.

-- Read-write over the web app's own schema (accounts, audit). ALL is needed to
-- run the web migrations; at run time the app performs only DML.
CREATE USER IF NOT EXISTS ${app_user}@'%' IDENTIFIED BY ${app_pass};
GRANT ALL PRIVILEGES ON ${app_db}.* TO ${app_user}@'%';

FLUSH PRIVILEGES;
SQL

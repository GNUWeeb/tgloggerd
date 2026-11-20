#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Test the MySQL first-initialization hook (docker/mysql/init) against a
# throwaway server: credentials holding quotes, backslashes, backquotes,
# `$` and SQL must create exactly the users and databases they name, and
# sign in with exactly the passwords given.
#
# The server runs with no network and no volume, under a name of its own,
# and is removed when the test ends, so a deployment is never touched.
# The MYSQL_* values stay plain: the image's own entrypoint does not
# quote them (env/mysql-init.env.example says so).
#
# Usage: scripts/test-mysql-init.sh   (needs Docker)
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
NAME="tgloggerd-test-mysql-init-$$"
IMAGE=mysql:9.7.0
WORK=$(mktemp -d "${TMPDIR:-$HOME/tmp}/test-mysql-init.XXXXXX")
cleanup() {
	docker rm -f "$NAME" >/dev/null 2>&1 || true
	rm -rf "${WORK:?}"
}
trap cleanup EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# Written byte for byte: docker's --env-file takes the rest of the line.
cat >"$WORK/env" <<'EOF'
MYSQL_ROOT_PASSWORD=test-root
MYSQL_DATABASE=tgloggerd
MYSQL_USER=tgloggerd
MYSQL_PASSWORD=test-logger
WEB_DB_RO_USER=web'ro
WEB_DB_RO_PASSWORD=ro'p\a"ss$HOME`id`;--
WEB_DB_APP_USER=web_app
WEB_DB_APP_PASSWORD=ap\'p'; DROP USER root; --
WEB_DB_APP_NAME=tg`web
EOF
val() { sed -n "s/^$1=//p" "$WORK/env"; }

docker run -d --name "$NAME" --network none --env-file "$WORK/env" \
	-v "$ROOT/docker/mysql/init:/docker-entrypoint-initdb.d:ro" "$IMAGE" >/dev/null
if ! timeout 300 bash -c "until docker logs '$NAME' 2>&1 | grep -q 'ready for connections.*port: 3306'; do
	docker inspect -f '{{.State.Running}}' '$NAME' | grep -qx true || exit 1; sleep 2; done"; then
	docker logs "$NAME" 2>&1 | tail -20 >&2
	fail "the server did not come up (the init hook failed?)"
fi

# q USER PASSWORD SQL: the answer, signed in over TCP as USER.
q() {
	docker exec -e MYSQL_PWD="$2" "$NAME" \
		mysql --protocol=tcp -h127.0.0.1 -u"$1" -N -e "$3"
}

ro_user=$(val WEB_DB_RO_USER)
[ "$(q "$ro_user" "$(val WEB_DB_RO_PASSWORD)" 'SELECT CURRENT_USER()')" = "$ro_user@%" ] ||
	fail "the read-only user does not sign in with its password"
q "$ro_user" "$(val WEB_DB_RO_PASSWORD)" 'SHOW GRANTS' | grep -qF 'GRANT SELECT ON `tgloggerd`.*' ||
	fail "the read-only user lacks SELECT on the logger's database"
[ "$(q web_app "$(val WEB_DB_APP_PASSWORD)" 'SELECT CURRENT_USER()')" = "web_app@%" ] ||
	fail "the app user does not sign in with its password"
q web_app "$(val WEB_DB_APP_PASSWORD)" 'SHOW GRANTS' | grep -qF 'GRANT ALL PRIVILEGES ON `tg``web`.*' ||
	fail "the app user lacks its own database"
if q web_app wrong 'SELECT 1' >/dev/null 2>&1; then
	fail "a wrong password signs in"
fi
q root test-root "SELECT COUNT(*) FROM mysql.user WHERE user = 'root'" | grep -qx '[12]' ||
	fail "root is gone: a password ran as SQL"
q root test-root 'SHOW DATABASES' | grep -qxF 'tg`web' ||
	fail "the web's database was not created under its name"
echo "OK: the init hook quotes every value"

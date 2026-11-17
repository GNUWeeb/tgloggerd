#!/usr/bin/env bash
#
# Unified container entrypoint for tgloggerd.
#
# It initializes the vendored git submodules the requested component needs,
# builds it (incrementally, into a bind-mounted build/ directory), applies the
# database migrations, and then execs the binary. All configuration comes from
# the environment -- see .env.example.
#
# Usage: run.sh <daemon|web|discord|webapi>
#
#   daemon  Build and run the logger daemon (src/, build/). On first login it
#           prompts on stdin for the phone number, code and 2FA password, so
#           run it interactively the first time: `docker compose run --rm
#           tgloggerd`.
#   web     Build and run the web interface (web/, web/build/).
#   discord Build and run discordd, the Discord daemon: logs Discord
#           messages and forwards them to Telegram. Shares build/ with
#           the daemon so TDLib is compiled only once.
#   webapi  Build and run the Go web service (webapi/, webapi/build/), which
#           serves the JSON API under /api/ beside the web interface.
#
set -euo pipefail

MODE="${1:-daemon}"

# This script lives at the repository root, which is the bind mount; run
# everything from there so the relative paths below (build/, migrations/, ...)
# and the *_DIR environment variables resolve consistently for both modes.
cd "$(dirname "$(readlink -f "$0")")"

# The container may run as a uid with no /etc/passwd entry (see TG_UID in the
# compose file), in which case HOME defaults to "/" and is not writable. Point
# it somewhere writable so `git config --global` below can store its config.
if [ ! -w "${HOME:-/}" ]; then
	export HOME=/tmp
fi

log() { printf '\n\033[1;32m=== %s ===\033[0m\n' "$*"; }

NPROC="$(nproc)"

# Compile without starving the rest of the host. nice needs no privileges;
# swallow a failure just in case it is unavailable.
build_cmd() { nice -n 19 bash -c "$1" || bash -c "$1"; }

# The daemon and discordd containers build in the SAME tree (build/, so TDLib
# is compiled once). Two cmake processes configuring one tree concurrently
# corrupt its cache -- seen in practice as compiler-identification files
# written with an empty COMPILER_ID, after which every later configure
# trusts the cache, skips re-identification, and dies on TDLib's "Compiler
# isn't supported" check. Both containers being (re)created by one
# `docker compose up` is exactly when they configure at the same time, so
# one lock, held for the whole configure+build: whoever comes second waits,
# then reuses everything the first one compiled.
build_locked() {
	(
		exec 9> .git/tgld-build.lock
		flock 9
		# The cache inspection runs under the lock too: checking another
		# container's half-written identification files would itself
		# misread them as corrupt.
		prepare_build_dir "$1"
		build_cmd "$2"
	)
}

# cmake bakes absolute paths into <dir>/CMakeCache.txt. If the directory was
# configured for a different path -- e.g. a native `cmake -B build` run on the
# host before switching to this container, where the same files are mounted at
# a different location -- cmake refuses to reuse the cache. Detect that and
# start clean so the build is not wedged.
prepare_build_dir() {
	local dir="$1" cache="$1/CMakeCache.txt"
	[ -f "$cache" ] || return 0
	if ! grep -qxF "CMAKE_CACHEFILE_DIR:INTERNAL=${PWD}/${dir}" "$cache"; then
		log "Discarding stale CMake cache in ${dir}/ (configured for another path)"
		rm -rf "$dir"
		return 0
	fi
	# Recover from the corruption described above build_locked: an
	# identification file recording an empty compiler id wedges every
	# subsequent configure. Dropping only CMakeFiles/<cmake-version>/
	# makes cmake re-identify the toolchain and reconfigure while the
	# compiled objects stay valid, so recovery is an incremental build,
	# not a from-scratch TDLib compile.
	local idfile
	for idfile in "$dir"/CMakeFiles/*/CMakeCXXCompiler.cmake; do
		[ -f "$idfile" ] || continue
		if grep -q 'COMPILER_ID ""' "$idfile"; then
			log "Discarding corrupted compiler identification in ${idfile%/*}/"
			rm -rf "${idfile%/*}"
		fi
	done
}

# Populate the submodules a component builds from. A fresh checkout leaves them
# empty, so cmake would fail with "does not contain a CMakeLists.txt file".
# Serialize with a lock on the shared .git so the daemon and web containers,
# which may both start cold, do not race on the git index.
ensure_submodules() {
	local need=0 p
	for p in "$@"; do
		[ -e "$p/CMakeLists.txt" ] || need=1
	done
	[ "$need" -eq 0 ] && return 0

	# This container runs as root while the bind-mounted tree is owned by the
	# host user; without this git refuses to operate ("dubious ownership").
	git config --global --add safe.directory '*' 2>/dev/null || true

	log "Initializing git submodules: $*"
	flock .git/tgld-submodule.lock \
		git submodule update --init --recursive "$@"
}

# Block until the MySQL server accepts TCP connections. migrate and the app
# both retry, but waiting here keeps the logs clean and the migration ordering
# deterministic.
wait_for_mysql() {
	local host="$1" port="$2" i
	log "Waiting for MySQL at ${host}:${port}"
	for i in $(seq 1 60); do
		if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
			exec 3>&- 3<&-
			return 0
		fi
		sleep 2
	done
	echo "run.sh: MySQL at ${host}:${port} did not come up" >&2
	return 1
}

# Percent-encode $1 for the userinfo part of a URL, so a password holding
# '@', '/', ':' or the like cannot break the migrate URL it goes into.
url_encode() {
	local LC_ALL=C s="$1" out="" c i
	for ((i = 0; i < ${#s}; i++)); do
		c="${s:i:1}"
		case "$c" in
		[A-Za-z0-9._~-]) out+="$c" ;;
		*) out+="$(printf '%%%02X' "'$c")" ;;
		esac
	done
	printf '%s' "$out"
}

# Copy stdin to stdout with every occurrence of each non-empty argument
# replaced by "***".
mask() {
	local line a
	while IFS= read -r line || [ -n "$line" ]; do
		for a in "$@"; do
			[ -n "$a" ] && line="${line//"$a"/***}"
		done
		printf '%s\n' "$line"
	done
}

# Run migrate against MySQL: run_migrate <user> <password> <host> <port>
# <database> <query> <migrate args...>. <query> is the URL's query string
# without the '?', or empty. The password is percent-encoded into the URL,
# so '@', '/', ':' and the like cannot break it, and masked, raw and
# encoded, in migrate's output, which prints the whole URL when it cannot
# connect. Returns migrate's exit status.
run_migrate() {
	local user="$1" pw="$2" host="$3" port="$4" db="$5" query="$6" enc url
	shift 6
	enc="$(url_encode "$pw")"
	url="mysql://${user}:${enc}@tcp(${host}:${port})/${db}${query:+?${query}}"
	migrate -database "$url" "$@" 2>&1 | mask "$enc" "$pw"
}

# Give the web's read-only user its table-level write grants on logger tables
# (docker/mysql/web-grants.sql). They cannot be given when MySQL initializes
# its data directory, because the tables only exist once the daemon's
# migrations have run, so this follows every `migrate up`. GRANT only adds
# privileges, so a repeat run changes nothing.
#
# It needs MySQL root (the logger's user has no GRANT OPTION), with the
# MYSQL_ROOT_PASSWORD the mysql service was initialized with, over TCP: the
# image creates root@'%' unless MYSQL_ROOT_HOST says otherwise. The image has
# no mysql client, so migrate runs the rendered file, recording its version in
# a table of the web database; resetting that version first makes every run
# apply the file in full. A failure only warns: the daemon does not need these
# grants, only the web's admin pages do.
apply_web_grants() {
	if [ -z "${MYSQL_ROOT_PASSWORD:-}" ] || [ -z "${WEB_DB_RO_USER:-}" ] ||
		[ -z "${WEB_DB_APP_NAME:-}" ]; then
		log "Skipping the web table grants (MYSQL_ROOT_PASSWORD, WEB_DB_RO_USER or WEB_DB_APP_NAME is not set)"
		return 0
	fi
	# The names are substituted into SQL and into a sed expression.
	local n
	for n in "$TG_DB_NAME" "$WEB_DB_RO_USER" "$WEB_DB_APP_NAME"; do
		if ! [[ "$n" =~ ^[A-Za-z0-9_]+$ ]]; then
			echo "run.sh: WARNING: not applying the web table grants: '$n' is not a plain name" >&2
			return 0
		fi
	done

	log "Applying the web table grants"
	local dir rc=0
	dir="$(mktemp -d)"
	sed -e "s/@TG_DB_NAME@/${TG_DB_NAME}/g" \
		-e "s/@WEB_DB_RO_USER@/${WEB_DB_RO_USER}/g" \
		docker/mysql/web-grants.sql >"${dir}/1_web_grants.up.sql"
	local db=(root "$MYSQL_ROOT_PASSWORD" "$TG_DB_HOST" "$TG_DB_PORT"
		"$WEB_DB_APP_NAME" x-migrations-table=web_grants_version)
	{ run_migrate "${db[@]}" -path "$dir" force -- -1 &&
		run_migrate "${db[@]}" -path "$dir" up; } || rc=$?
	rm -rf "${dir:?}"
	if [ "$rc" -ne 0 ]; then
		echo "run.sh: WARNING: the web table grants were not applied; the web cannot change integrations, routes, bots or the MCP allowlist (see web/README.web)" >&2
	fi
}

# docker-compose.yml loads the whole .env into every service, so each one
# starts with every secret of the deployment. Each mode below unsets the ones
# its component does not use, before building it, so neither the build nor
# the process run.sh execs holds them. The mysql service's own passwords
# (MYSQL_ROOT_PASSWORD, MYSQL_PASSWORD) go everywhere, MYSQL_ROOT_PASSWORD
# only after the daemon's grants step has used it. This covers the processes
# run.sh starts; `docker exec` into a container still gets the whole .env.
case "$MODE" in
daemon)
	: "${TG_DB_HOST:?TG_DB_HOST is not set}" "${TG_DB_PORT:?TG_DB_PORT is not set}"
	: "${TG_DB_USER:?}" "${TG_DB_PASSWORD:?}" "${TG_DB_NAME:?}"
	unset MYSQL_PASSWORD WEB_DB_RO_PASSWORD WEB_DB_APP_PASSWORD DISCORD_BOT_TOKEN

	ensure_submodules submodules/td submodules/mysql-connector-cpp

	log "Building tgloggerd (this compiles TDLib on the first run and is slow)"
	# --target tgloggerd, not everything: build/ also holds discordd, which
	# this container neither runs nor needs, and building it here would
	# make the daemon depend on discordd's own dependencies.
	build_locked build "cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build --target tgloggerd -j${NPROC}"

	wait_for_mysql "$TG_DB_HOST" "$TG_DB_PORT"
	log "Applying daemon migrations"
	run_migrate "$TG_DB_USER" "$TG_DB_PASSWORD" "$TG_DB_HOST" "$TG_DB_PORT" \
		"$TG_DB_NAME" "" -path migrations up
	apply_web_grants
	unset MYSQL_ROOT_PASSWORD

	log "Starting tgloggerd"
	exec build/tgloggerd
	;;

web)
	: "${WEB_DB_HOST:?WEB_DB_HOST is not set}" "${WEB_DB_PORT:?WEB_DB_PORT is not set}"
	: "${WEB_DB_APP_USER:?}" "${WEB_DB_APP_PASSWORD:?}" "${WEB_DB_APP_NAME:?}"
	unset MYSQL_ROOT_PASSWORD MYSQL_PASSWORD TG_DB_PASSWORD DISCORD_BOT_TOKEN

	ensure_submodules submodules/drogon submodules/mariadb-connector-c

	log "Building tgloggerd_web (this compiles Drogon on the first run and is slow)"
	prepare_build_dir web/build
	build_cmd "cmake -S web -B web/build -DCMAKE_BUILD_TYPE=Release && cmake --build web/build -j${NPROC}"

	wait_for_mysql "$WEB_DB_HOST" "$WEB_DB_PORT"
	# web_app owns tgloggerd_web, so it can run the web migrations; at run time
	# it performs only DML.
	log "Applying web migrations"
	run_migrate "$WEB_DB_APP_USER" "$WEB_DB_APP_PASSWORD" "$WEB_DB_HOST" \
		"$WEB_DB_PORT" "$WEB_DB_APP_NAME" "" -path web/migrations up

	log "Starting tgloggerd_web"
	exec web/build/tgloggerd_web
	;;

discord)
	: "${TG_DB_HOST:?TG_DB_HOST is not set}" "${TG_DB_PORT:?TG_DB_PORT is not set}"
	: "${TG_DB_USER:?}" "${TG_DB_PASSWORD:?}" "${TG_DB_NAME:?}"
	: "${DISCORD_BOT_TOKEN:?DISCORD_BOT_TOKEN is not set}"
	# discordd mints no /files/ links, so it needs no WEB_APP_KEY.
	unset MYSQL_ROOT_PASSWORD MYSQL_PASSWORD WEB_DB_RO_PASSWORD \
		WEB_DB_APP_PASSWORD WEB_APP_KEY

	# discordd links TDLib (it sends to Telegram as a bot) and the JDBC
	# connector, so it needs the same submodules as the daemon.
	ensure_submodules submodules/td submodules/mysql-connector-cpp

	log "Building discordd (shares build/ with tgloggerd, so TDLib is compiled once)"
	# The configure line must match the daemon's exactly: both containers
	# share build/, and a differing cache would make them reconfigure in a
	# loop against each other.
	build_locked build "cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build --target discordd -j${NPROC}"

	wait_for_mysql "$TG_DB_HOST" "$TG_DB_PORT"
	# The daemon container owns migrations; discordd only reads the schema
	# it produces. Waiting for the table it needs avoids a startup race on
	# a cold deployment where both containers come up together.
	log "Starting discordd"
	exec build/discordd
	;;

webapi)
	# WEB_APP_KEY stays: the service is taking over the web's features, and
	# the web signs its sessions with that key.
	unset MYSQL_ROOT_PASSWORD MYSQL_PASSWORD TG_DB_PASSWORD DISCORD_BOT_TOKEN

	# Only vendor/ is used: GOPROXY=off makes a missing module an error
	# instead of a download. The build cache lives in the bind-mounted
	# webapi/build/, so a restart rebuilds incrementally.
	export GOFLAGS=-mod=vendor GOPROXY=off GOCACHE="${PWD}/webapi/build/cache"

	log "Building webapi"
	build_cmd "cd webapi && go build -o build/webapi ./cmd/webapi"

	# No migrations: the web service owns the web schema and the daemon
	# the logger's. The pools connect lazily, so MySQL need not be up
	# yet; /api/health reports it until it is.
	log "Starting webapi"
	exec webapi/build/webapi
	;;

*)
	echo "Usage: run.sh <daemon|web|discord|webapi>" >&2
	exit 2
	;;
esac

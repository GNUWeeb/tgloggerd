#!/bin/bash
# Run the web's checks with nothing but Docker on the host:
#
#   - the Go service (webapi/): gofmt, go vet, and go test with -race
#     against the test MySQL of docker-compose.test.yml, every test in a
#     database of its own (webapi/internal/testdb);
#   - the React application (web/app/): npm ci, check:api, typecheck
#     and its tests.
#
# Usage: scripts/test-web.sh [--go-only | --app-only] [go test args...]
#
# Extra arguments go to go test, before the packages (./... by default
# when none of them names a package), e.g. -run TestMedia -count=1.
#
# The test MySQL is started when it is not running and stopped and
# removed again at the end; one that was already running is used and
# left running, so several runs can share it. COMPOSE_PROJECT_NAME picks
# another copy of the test project. The Go container sees only the
# project's internal network, so the tests run offline; the module
# cache and build cache live in $TGD_TEST_GO_CACHE (default
# ~/.cache/tgd-webapi-go).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
GO_IMAGE=golang:1.27.4
PROJECT=${COMPOSE_PROJECT_NAME:-tgloggerd-test}
CACHE=${TGD_TEST_GO_CACHE:-$HOME/.cache/tgd-webapi-go}
COMPOSE=(docker compose -p "$PROJECT" -f "$ROOT/docker-compose.test.yml")

go=1
app=1
case ${1:-} in
--go-only) app=0; shift ;;
--app-only) go=0; shift ;;
-h | --help) sed -n '2,21p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
esac

log() { printf '== %s\n' "$*"; }

started=0
cleanup() {
	if [ "$started" = 1 ]; then
		log "Stopping the test MySQL"
		"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

run_go() {
	if [ -z "$("${COMPOSE[@]}" ps -q --status running mysql 2>/dev/null)" ]; then
		log "Starting the test MySQL ($PROJECT)"
		started=1
		"${COMPOSE[@]}" up -d --wait --quiet-pull mysql
	else
		log "Using the running test MySQL ($PROJECT)"
	fi

	mkdir -p "$CACHE"
	log "webapi: gofmt, go vet, go test -race"
	docker run --rm --init --network "${PROJECT}_default" \
		-u "$(id -u):$(id -g)" \
		-v "$ROOT:/src" -v "$CACHE:/cache" -w /src/webapi \
		-e GOFLAGS=-mod=vendor -e GOPROXY=off -e HOME=/cache \
		-e GOCACHE=/cache/build -e GOMODCACHE=/cache/mod \
		-e 'WEBAPI_TEST_MYSQL_DSN=root:test@tcp(mysql:3306)/' \
		"$GO_IMAGE" bash -c '
			set -euo pipefail
			bad=$(gofmt -l . | grep -v "^vendor/" || true)
			if [ -n "$bad" ]; then
				echo "gofmt: these files need formatting:" >&2
				echo "$bad" >&2
				exit 1
			fi
			go vet ./...
			pkgs=(./...)
			for a in "$@"; do
				case $a in ./* | tgloggerd/*) pkgs=() ;; esac
			done
			go test -race "$@" "${pkgs[@]}"
		' test-web "$@"
}

run_app() {
	log "web/app: npm ci, check:api, typecheck, tests"
	cd "$ROOT/web/app"
	./dev npm ci --prefer-offline --no-audit --no-fund
	./dev npm run check:api
	./dev npm run typecheck
	./dev npm test
}

if [ "$go" = 1 ]; then
	run_go "$@"
fi
if [ "$app" = 1 ]; then
	run_app
fi
log "All checks passed"

#!/bin/bash
# Run the web's checks with nothing but Docker on the host:
#
#   - the Go service (webapi/): gofmt, go vet, and go test with -race
#     against the test MySQL of docker-compose.test.yml, every test in a
#     database of its own (webapi/internal/testdb);
#   - the React application (web/app/): npm ci, check:api, typecheck
#     and its tests; with --e2e also its browser suite (web/app/e2e/,
#     in the Playwright image, a stand-in per worker);
#   - with --search-engine, the Go tests also get the test OpenSearch
#     of docker-compose.test.yml (WEBAPI_TEST_SEARCH_ENGINE_URL), so the
#     search engine's tests run instead of skipping.
#
# Usage: scripts/test-web.sh [--go-only | --app-only] [--e2e] [--search-engine]
#                            [go test args...]
#
# Extra arguments go to go test, before the packages (./... by default
# when none of them names a package), e.g. -run TestMedia -count=1.
#
# The test MySQL (and OpenSearch) is started when it is not running
# and stopped and removed again at the end; one that was already
# running is used and left running, so several runs can share it.
# COMPOSE_PROJECT_NAME picks another copy of the test project. The Go
# container sees only the project's internal network, so the tests run
# offline; the module cache and build cache live in $TGD_TEST_GO_CACHE
# (default ~/.cache/tgd-webapi-go).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
GO_IMAGE=golang:1.27.4
PROJECT=${COMPOSE_PROJECT_NAME:-tgloggerd-test}
CACHE=${TGD_TEST_GO_CACHE:-$HOME/.cache/tgd-webapi-go}
COMPOSE=(docker compose -p "$PROJECT" -f "$ROOT/docker-compose.test.yml")

go=1
app=1
e2e=0
engine=0
while :; do
	case ${1:-} in
	--go-only) app=0; shift ;;
	--app-only) go=0; shift ;;
	--e2e) e2e=1; shift ;;
	--search-engine) engine=1; shift ;;
	-h | --help) sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) break ;;
	esac
done

log() { printf '== %s\n' "$*"; }

started=0
started_engine=0
cleanup() {
	if [ "$started" = 1 ]; then
		log "Stopping the test MySQL"
		"${COMPOSE[@]}" --profile search down -v --remove-orphans >/dev/null 2>&1 || true
	elif [ "$started_engine" = 1 ]; then
		log "Stopping the test OpenSearch"
		"${COMPOSE[@]}" --profile search rm -f -s -v opensearch >/dev/null 2>&1 || true
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
	engine_url=
	if [ "$engine" = 1 ]; then
		if [ -z "$("${COMPOSE[@]}" --profile search ps -q --status running opensearch 2>/dev/null)" ]; then
			log "Starting the test OpenSearch ($PROJECT)"
			started_engine=1
			"${COMPOSE[@]}" --profile search up -d --wait --quiet-pull opensearch
		else
			log "Using the running test OpenSearch ($PROJECT)"
		fi
		engine_url=http://opensearch:9200
	fi

	mkdir -p "$CACHE"
	log "webapi: gofmt, go vet, go test -race"
	docker run --rm --init --network "${PROJECT}_default" \
		-u "$(id -u):$(id -g)" \
		-v "$ROOT:/src" -v "$CACHE:/cache" -w /src/webapi \
		-e GOFLAGS=-mod=vendor -e GOPROXY=off -e HOME=/cache \
		-e GOCACHE=/cache/build -e GOMODCACHE=/cache/mod \
		-e 'WEBAPI_TEST_MYSQL_DSN=root:test@tcp(mysql:3306)/' \
		-e "WEBAPI_TEST_SEARCH_ENGINE_URL=$engine_url" \
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
	if [ "$e2e" = 1 ]; then
		log "web/app: browser suite"
		./dev e2e
	fi
}

if [ "$go" = 1 ]; then
	run_go "$@"
fi
if [ "$app" = 1 ]; then
	run_app
fi
log "All checks passed"

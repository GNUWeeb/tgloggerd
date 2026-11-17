#!/usr/bin/env bash
#
# End-to-end test of the render service as docker-compose.yml deploys it:
# builds web/app with the render-build service, starts the render and
# webapi services, and checks that the webapi service renders /login
# through the render service (data-ssr="1"), serves the client bundle the
# same build made, and falls back to browser rendering (data-ssr="0") once
# the render service is stopped.
#
# It runs on a copy of the tree (the compose file, run.sh, webapi/ and
# web/app/ as they are in the working tree), under a compose project, a
# network and a socket volume of its own, so it never touches a running
# deployment or the checkout, and several runs can go at once. Everything it
# starts is removed on exit. It needs Docker, the tgloggerd:latest image
# (for the webapi service; it is never built here) and the npm registry
# (npm ci). Set TMPDIR to choose where the copy goes.
#
# Usage: scripts/test-render-service.sh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/tgld-render-test.XXXXXX")
PROJECT="tgld-render-test-$(basename "$WORK" | tr 'A-Z' 'a-z' | tr -dc 'a-z0-9' | tail -c 12)"
export TG_UID=$(id -u) TG_GID=$(id -g)

compose() {
	docker compose -p "$PROJECT" --project-directory "$WORK" \
		-f "$WORK/docker-compose.yml" -f "$WORK/test.override.yml" "$@"
}

cleanup() {
	compose down -v --remove-orphans --timeout 5 >/dev/null 2>&1 || true
	rm -rf "${WORK:?}"
}
trap cleanup EXIT

fail() {
	echo "FAIL: $*" >&2
	compose logs --no-color render webapi 2>&1 | tail -n 40 >&2 || true
	exit 1
}

echo "== copying the tree to $WORK (project $PROJECT)"
tar -C "$ROOT" -cf - --exclude=webapi/build --exclude=web/app/node_modules \
	--exclude=web/app/dist docker-compose.yml run.sh webapi web/app |
	tar -C "$WORK" -xf -

# The deployment's settings come from .env, which the test does not have
# and must not read: the webapi service gets a throwaway key instead, and
# no database (the pools connect lazily, and the signed-out sign-in page
# needs none). Container names, the fixed tgnet addresses and its bridge
# name are dropped, so nothing collides with a running deployment.
key=$(head -c 32 /dev/urandom | base64)
cat >"$WORK/test.override.yml" <<EOF
services:
  render:
    container_name: !reset null
  webapi:
    container_name: !reset null
    env_file: !reset []
    depends_on: !reset []
    environment:
      WEB_APP_KEY: "$key"
      WEB_DB_HOST: 127.0.0.1
      WEBAPI_LISTEN_ADDR: 0.0.0.0
      WEBAPI_LISTEN_PORT: "8081"
    networks: !reset {}
    ports: ["127.0.0.1::8081"]
networks:
  tgnet:
    ipam: !reset {}
    driver_opts: !reset {}
EOF

echo "== render-build"
compose run --rm render-build >"$WORK/build.log" 2>&1 ||
	{ tail -n 30 "$WORK/build.log" >&2; fail "render-build"; }
build=$(sed -n 's/^{"build":"\([^"]*\)"}$/\1/p' "$WORK/web/app/dist/client/build.json")
[ -n "$build" ] || fail "no build id in dist/client/build.json"
grep -q "BUILD = \"$build\"" "$WORK/web/app/dist/server/entry-server.js" ||
	fail "the render entry does not carry build $build"
echo "build $build"

echo "== starting render and webapi"
compose up -d --no-build render webapi >/dev/null 2>&1 || fail "compose up"

render_id=$(compose ps -q render)
timeout 120 bash -c "until [ \"\$(docker inspect -f '{{.State.Health.Status}}' $render_id)\" = healthy ]; do sleep 1; done" ||
	fail "render did not become healthy"
cfg=$(docker inspect -f '{{.HostConfig.NetworkMode}} {{.HostConfig.ReadonlyRootfs}} {{.Config.User}}' "$render_id")
[ "$cfg" = "none true $TG_UID:$TG_GID" ] ||
	fail "render is not network none, read-only, as $TG_UID:$TG_GID: $cfg"
docker inspect -f '{{json .Config.Env}}' "$render_id" | grep -q WEB_APP_KEY &&
	fail "render has the webapi's key in its environment"

addr=$(compose port webapi 8081) || fail "webapi has no published port"
# The webapi service compiles itself on start (run.sh webapi). Any HTTP
# answer will do: /api/health is 503 here, with no database.
timeout 600 bash -c "until [ \"\$(curl -s -o /dev/null -w '%{http_code}' http://$addr/api/health)\" != 000 ]; do sleep 2; done" ||
	fail "webapi did not answer"

page() { curl -fsS "http://$addr/login" || fail "GET /login"; }

echo "== rendered by the render service"
doc=$(page)
grep -q 'data-ssr="1"' <<<"$doc" || fail "/login was not rendered on the server"
grep -q '<form' <<<"$doc" || fail "/login has no rendered form"
entry=$(grep -o '/app/assets/entry-client-[A-Za-z0-9_-]*\.js' <<<"$doc" | head -n 1)
[ -n "$entry" ] || fail "/login links no client entry"
curl -fsS -o /dev/null "http://$addr$entry" || fail "GET $entry"
compose logs --no-color render | grep -q '"route":"login"' ||
	fail "the render service logged no render of login"
echo "ok: data-ssr=1, $entry served"

echo "== render service stopped"
compose stop render >/dev/null 2>&1
doc=$(page)
grep -q 'data-ssr="0"' <<<"$doc" || fail "/login did not fall back"
grep -q '<div id="root" data-ssr="0"></div>' <<<"$doc" || fail "fallback has markup"
grep -q "$entry" <<<"$doc" || fail "fallback links no client entry"
echo "ok: data-ssr=0"

echo "PASS"

#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Builds fixgen against the Drogon web's auth code and runs it.
#
#   run.sh gen      rewrite ../drogon.json from the Drogon code
#   run.sh verify   have the Go tests write the values they make, and
#                   check them with the Drogon code
#
# Run from anywhere; needs Docker and a built web/build tree (the
# project image's normal build). Uses the tgloggerd image for C++ and
# the pinned Go image for the Go side, both without network.
set -euo pipefail

GO_IMAGE=golang:1.27.4
IMAGE=${TGLOGGERD_IMAGE:-tgloggerd:latest}
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../../.." && pwd)
REL=${HERE#"$ROOT"/}
OUT=web/build/fixgen
U="$(id -u):$(id -g)"

build_and_run() {
	docker run --rm -i --network none --entrypoint bash --user "$U" \
		-e HOME=/tmp -v "$ROOT":/workspace -w /workspace "$IMAGE" \
		-c '
set -e
W=/workspace/web B=/workspace/web/build O=/workspace/'"$OUT"'
INC="-I$W/src -I$W/../src -I$W/third_party
 -I/workspace/submodules/drogon/trantor -I/workspace/submodules/drogon/lib/inc
 -I$B/drogon/lib/inc -I/workspace/submodules/drogon/orm_lib/inc
 -I/workspace/submodules/drogon/nosql_lib/redis/inc -I$B/drogon
 -I$B/drogon/exports -I$B/drogon/trantor/exports -isystem /usr/include/jsoncpp"
mkdir -p "$O"
for f in auth/Token auth/Session auth/Csrf auth/FileToken auth/Password \
	 daemon/Api /workspace/'"$REL"'/fixgen; do
	case $f in /*) s=$f.cpp ;; *) s=$W/src/$f.cpp ;; esac
	c++ -O1 -std=gnu++20 -fPIE -Wall -Wextra $INC -c "$s" \
		-o "$O/$(basename "$f").o" >&2
done
c++ "$O"/*.o -o "$O/fixgen" $B/drogon/libdrogon.a -lsodium \
	$B/drogon/trantor/libtrantor.a -lssl -lcrypto -lpthread -ljsoncpp \
	-luuid -lbrotlidec -lbrotlienc -lbrotlicommon \
	$B/mariadb-connector-c/libmariadb/libmariadbclient.a -lz -lm -ldl >&2
exec "$O/fixgen" "$@"' fixgen "$@"
}

case ${1:-} in
gen)
	build_and_run gen > "$HERE/../drogon.json.new"
	mv "$HERE/../drogon.json.new" "$HERE/../drogon.json"
	echo "wrote $REL/../drogon.json" >&2
	;;
verify)
	mkdir -p "$ROOT/$OUT"
	docker run --rm --network none --user "$U" -e HOME=/tmp \
		-e GOFLAGS=-mod=vendor -e GOPROXY=off \
		-e WEBAPI_GO_MINTED=/workspace/$OUT/go-minted.json \
		-v "$ROOT":/workspace -w /workspace/webapi "$GO_IMAGE" \
		go test -count=1 -run TestGoMinted ./internal/auth
	build_and_run verify < "$ROOT/$OUT/go-minted.json"
	;;
*)
	echo "usage: $0 gen|verify" >&2
	exit 2
	;;
esac

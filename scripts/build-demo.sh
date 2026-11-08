#!/usr/bin/env bash
# Builds the EasyLPAC demo for this machine's platform, on a simulated
# card: the static libeuicc, then the app bundle with MyGo, without a dmg.
# Extra arguments go to `mygo build`.
#
#   ./scripts/build-demo.sh           # demo build, build/demo/<os>-<arch>/
#
# A demo build compiles in internal/demo (build tag "demo"): no card or
# SM-DP+ is used and settings are not saved. Release builds
# (scripts/build-app.sh) never contain it, and it cannot be switched on at
# run time.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f "third_party/libeuicc/$(go env GOOS)-$(go env GOARCH)/libeuicc.a" ]; then
	./scripts/build-libeuicc.sh
fi
# zig compiles the C code, as in CI.
export CC="$ROOT/scripts/zcc"
GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=demo" CGO_ENABLED=1 \
	go run github.com/egoist/mygo/cmd/mygo@v0.8.3 build -o build/demo --skip-dmg "$@"

#!/usr/bin/env bash
# Builds the EasyLPAC app for this machine's platform: the static
# libeuicc, then the app bundle and installer with MyGo. Extra arguments go
# to `mygo build`. The eUICC data embedded is the one committed in
# internal/registry/data: builds never download it (to update it, run
# scripts/update-euicc-data.sh and commit the result).
#
#   ./scripts/build-app.sh            # release build, build/<os>-<arch>/
#
# Release builds never contain the simulated card; build it with
# scripts/build-demo.sh.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

for a in "$@"; do
	if [ "$a" = --demo ]; then
		echo "build-app.sh: --demo moved to scripts/build-demo.sh" >&2
		exit 2
	fi
done

if [ ! -f "third_party/libeuicc/$(go env GOOS)-$(go env GOARCH)/libeuicc.a" ]; then
	./scripts/build-libeuicc.sh
fi
# zig compiles the C code, as in CI.
export CC="$ROOT/scripts/zcc"
CGO_ENABLED=1 go run github.com/egoist/mygo/cmd/mygo@v0.8.3 build "$@"

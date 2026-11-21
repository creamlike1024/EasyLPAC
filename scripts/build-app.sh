#!/usr/bin/env bash
# Builds the EasyLPAC app for this machine's platform: the static
# libeuicc, then the app bundle and installer with MyGo. Extra arguments go
# to `mygo build`. The eUICC data embedded is the one committed in
# internal/registry/data: builds never download it (to update it, run
# scripts/update-euicc-data.sh and commit the result). The version is the
# one scripts/version.sh gives from the git tags (EASYLPAC_VERSION sets it).
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
# mygo_nowebview: every window is native UI, so Windows builds leave
# WebView2 out (the tag does nothing on other platforms).
# The version comes from git (scripts/version.sh), not from mygo.json.
GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=mygo_nowebview" CGO_ENABLED=1 \
	./scripts/with-version.sh go run github.com/egoist/mygo/cmd/mygo@v0.10.0 build "$@"

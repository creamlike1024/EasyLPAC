#!/bin/sh
# Build and run the C-side libeuicc/shim contract harnesses.
# Complements `go test ./internal/euicc/` (Go-side lifecycle, which
# cannot use cgo in test files) with pure-C checks of the callback and
# HTTP contracts against the real static library.
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
LPAC="$ROOT/third_party/lpac"
LIB="$ROOT/third_party/libeuicc/$(go env GOOS)-$(go env GOARCH)"

if [ ! -f "$LIB/libeuicc.a" ]; then
    "$ROOT/scripts/build-libeuicc.sh"
fi

OUT=$(mktemp -d)
trap 'rm -rf "$OUT"' EXIT

"$ROOT/scripts/zcc" -I"$LPAC" "$ROOT/scripts/test-http-shim.c" \
    -L"$LIB" -leuicc -lcjsonext -lcjson -o "$OUT/test-http-shim"
"$OUT/test-http-shim"
"$OUT/test-http-shim"

echo "C harnesses passed."

#!/bin/sh
# Builds the static libeuicc (+cJSON) from the lpac submodule for
# one target, with zig as the C compiler (scripts/zcc), so that any host
# builds for every platform.
#
# Usage: scripts/build-libeuicc.sh [GOOS/GOARCH]    (default: the host's)
#
# Output: third_party/libeuicc/<GOOS>-<GOARCH>/{libeuicc.a,libcjson.a,libcjsonext.a},
# which internal/euicc links for that target. Everything is written
# outside the submodule, which stays as checked out.
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
SRC="$ROOT/third_party/lpac"
PLATFORM="${1:-$(go env GOOS)/$(go env GOARCH)}"
GOOS_T="${PLATFORM%/*}"
GOARCH_T="${PLATFORM#*/}"
NAME="$GOOS_T-$GOARCH_T"
OUT="$ROOT/third_party/libeuicc/$NAME"
BUILD="$OUT/cmake"
if [ ! -f "$SRC/CMakeLists.txt" ]; then
	echo "build-libeuicc: third_party/lpac is empty; run: git submodule update --init" >&2
	exit 1
fi

case "$GOOS_T" in
darwin) SYSTEM=Darwin ;;
linux) SYSTEM=Linux ;;
windows) SYSTEM=Windows ;;
*) echo "build-libeuicc: unsupported GOOS $GOOS_T" >&2; exit 1 ;;
esac

# The compiler wrapper picks the zig target from GOOS/GOARCH.
export GOOS="$GOOS_T" GOARCH="$GOARCH_T"
cmake -S "$SRC" -B "$BUILD" \
	-DLPAC_BUILD_LIBEUICC_ONLY=ON \
	-DLPAC_DYNAMIC_LIBEUICC=OFF \
	-DBUILD_TESTING=OFF \
	-DCMAKE_BUILD_TYPE="${CMAKE_BUILD_TYPE:-Release}" \
	-DCMAKE_SYSTEM_NAME="$SYSTEM" \
	-DCMAKE_C_COMPILER="$ROOT/scripts/zcc" \
	-DCMAKE_AR="$ROOT/scripts/zar" \
	-DCMAKE_RANLIB="$ROOT/scripts/zranlib"
cmake --build "$BUILD" --parallel

mkdir -p "$OUT"
cp "$BUILD/euicc/libeuicc.a" "$OUT/libeuicc.a"
cp "$BUILD/_deps/cjson-build/libcjson.a" "$OUT/libcjson.a"
rm -f "$OUT/libcjsonext.a"
zig ar rcs "$OUT/libcjsonext.a" "$(find "$BUILD/cjson-ext" -name 'cJSON_ex.c.o' -o -name 'cJSON_ex.c.obj' | head -n 1)"
echo "libeuicc for $PLATFORM:"
ls -l "$OUT"

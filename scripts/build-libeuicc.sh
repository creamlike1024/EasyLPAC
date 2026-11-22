#!/bin/sh
# Builds the static libeuicc (+cJSON) from the lpac submodule for
# one target, with zig as the C compiler (scripts/zcc), so that any host
# builds for every platform. Android is built with the NDK's clang, which
# mygo links Go with: the NDK of ANDROID_NDK_HOME, or the SDK's newest.
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
android) SYSTEM=Android ;;
*) echo "build-libeuicc: unsupported GOOS $GOOS_T" >&2; exit 1 ;;
esac

# The compiler wrapper picks the zig target from GOOS/GOARCH.
# LTO and PIC are set rather than probed by lpac, whose probes link test
# programs that zig cannot (LTO on windows-gnu, -pie/-no_pie on macOS)
# and fill the log with their errors. LTO is off on every target; the
# objects are position-independent, as the Go programs that link them
# may be PIE (always so on macOS; Windows has no PIC flag).
export GOOS="$GOOS_T" GOARCH="$GOARCH_T"
if [ "$GOOS_T" = android ]; then
	case "$GOARCH_T" in
	arm64) ABI=arm64-v8a ;;
	arm) ABI=armeabi-v7a ;;
	amd64) ABI=x86_64 ;;
	*) echo "build-libeuicc: unsupported Android GOARCH $GOARCH_T" >&2; exit 1 ;;
	esac
	NDK="${ANDROID_NDK_HOME:-}"
	if [ -z "$NDK" ]; then
		for sdk in "${ANDROID_HOME:-}" "$HOME/Library/Android/sdk" "$HOME/Android/Sdk"; do
			if [ -n "$sdk" ] && [ -d "$sdk/ndk" ]; then
				NDK="$sdk/ndk/$(ls "$sdk/ndk" | sort -V | tail -n 1)"
				break
			fi
		done
	fi
	if [ ! -f "$NDK/build/cmake/android.toolchain.cmake" ]; then
		echo "build-libeuicc: no Android NDK: install one, or set ANDROID_NDK_HOME" >&2
		exit 1
	fi
	for AR in "$NDK"/toolchains/llvm/prebuilt/*/bin/llvm-ar; do break; done
	# API 26 is mygo.json's android.minSdk.
	set -- -DCMAKE_TOOLCHAIN_FILE="$NDK/build/cmake/android.toolchain.cmake" \
		-DANDROID_ABI="$ABI" -DANDROID_PLATFORM=android-26
else
	AR="$ROOT/scripts/zar"
	set -- -DCMAKE_SYSTEM_NAME="$SYSTEM" \
		-DCMAKE_C_COMPILER="$ROOT/scripts/zcc" \
		-DCMAKE_AR="$AR" \
		-DCMAKE_RANLIB="$ROOT/scripts/zranlib"
fi
cmake -S "$SRC" -B "$BUILD" \
	-DLPAC_BUILD_LIBEUICC_ONLY=ON \
	-DLPAC_DYNAMIC_LIBEUICC=OFF \
	-DBUILD_TESTING=OFF \
	-DCMAKE_INTERPROCEDURAL_OPTIMIZATION=OFF \
	-DCMAKE_POSITION_INDEPENDENT_CODE=ON \
	-DCMAKE_BUILD_TYPE="${CMAKE_BUILD_TYPE:-Release}" \
	"$@"
cmake --build "$BUILD" --parallel

mkdir -p "$OUT"
cp "$BUILD/euicc/libeuicc.a" "$OUT/libeuicc.a"
cp "$BUILD/_deps/cjson-build/libcjson.a" "$OUT/libcjson.a"
rm -f "$OUT/libcjsonext.a"
"$AR" rcs "$OUT/libcjsonext.a" "$(find "$BUILD/cjson-ext" -name 'cJSON_ex.c.o' -o -name 'cJSON_ex.c.obj' | head -n 1)"
echo "libeuicc for $PLATFORM:"
ls -l "$OUT"

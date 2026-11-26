#!/usr/bin/env bash
# Builds the Android APK and AAB (arm64): libeuicc with the NDK, then the
# app with MyGo and Gradle. Extra arguments go to `mygo build`.
#
#   ./scripts/build-android.sh        # build/android/
#
# Needs the Android SDK (Platform 36, Build-Tools 35.0.0, an NDK) and a
# JDK 17, which MyGo's Gradle build asks for: Android Studio's own JDK is
# too new for its Gradle (8.14, which runs on Java 24 at most). The JDK is
# JAVA_HOME if that is a 17, or else the system's or Homebrew's.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

is_jdk17() {
	[ -x "$1/bin/java" ] || return 1
	local out
	out="$("$1/bin/java" -version 2>&1)" || return 1
	[[ "$out" == *'version "17'* ]]
}

JDK=
for h in "${JAVA_HOME:-}" \
	/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
	/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
	/Library/Java/JavaVirtualMachines/*17*/Contents/Home \
	/usr/lib/jvm/*17*; do
	if [ -n "$h" ] && is_jdk17 "$h"; then
		JDK="$h"
		break
	fi
done
if [ -z "$JDK" ]; then
	echo "build-android.sh: no JDK 17: install one, or set JAVA_HOME to it" >&2
	exit 1
fi
export JAVA_HOME="$JDK"

if [ ! -f third_party/libeuicc/android-arm64/libeuicc.a ]; then
	./scripts/build-libeuicc.sh android/arm64
fi
EASYLPAC_ANDROID=1 \
	./scripts/with-version.sh go run github.com/egoist/mygo/cmd/mygo@v0.10.0 build -platform android/arm64 "$@"

#!/bin/sh
# Runs a command with the version of scripts/version.sh in mygo.json, then
# puts mygo.json back as it was, also when the command fails or is
# interrupted. mygo build links that version into the app and names the
# packages after it; the committed mygo.json keeps "0.0.0-dev".
#
#   scripts/with-version.sh go run github.com/egoist/mygo/cmd/mygo@v0.10.0 build
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
CONFIG="$ROOT/mygo.json"
VERSION=$("$ROOT/scripts/version.sh")

BACKUP=$(mktemp "${TMPDIR:-/tmp}/easylpac-mygo.json.XXXXXX")
cp "$CONFIG" "$BACKUP"
restore() { cp "$BACKUP" "$CONFIG" && rm -f "$BACKUP"; }
trap restore EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# The one top-level "version" line of mygo.json, two spaces in.
sed -E 's/^(  "version": )"[^"]*"/\1"'"$VERSION"'"/' "$BACKUP" >"$CONFIG"
if ! grep -qxF "  \"version\": \"$VERSION\"," "$CONFIG"; then
	echo "with-version.sh: no top-level \"version\" in mygo.json to set" >&2
	exit 1
fi
echo "with-version.sh: version $VERSION" >&2

# Android's versionCode, which mygo would take wrongly from a snapshot
# (0.9.2.r12.g... as 900): scripts/build-android.sh asks for it.
if [ -n "${EASYLPAC_ANDROID:-}" ]; then
	CODE=$("$ROOT/scripts/android-version-code.sh" "$VERSION")
	VERSIONED=$(sed -E "s/^(    \"versionCode\": )[0-9]+/\\1$CODE/" "$CONFIG")
	printf '%s\n' "$VERSIONED" >"$CONFIG"
	if ! grep -Eq "^    \"versionCode\": $CODE,?\$" "$CONFIG"; then
		echo "with-version.sh: no android.versionCode in mygo.json to set" >&2
		exit 1
	fi
	echo "with-version.sh: versionCode $CODE" >&2

	# The upload key that signs release builds, when given: its keystore
	# and alias here, its passwords in MYGO_ANDROID_KEYSTORE_PASSWORD and
	# MYGO_ANDROID_KEY_PASSWORD, which mygo reads. Without it mygo signs
	# with the debug key.
	if [ -n "${EASYLPAC_ANDROID_KEYSTORE:-}" ]; then
		ALIAS="${EASYLPAC_ANDROID_KEY_ALIAS:?with-version.sh: EASYLPAC_ANDROID_KEY_ALIAS is needed with EASYLPAC_ANDROID_KEYSTORE}"
		case "$EASYLPAC_ANDROID_KEYSTORE$ALIAS" in
		*'"'* | *'\'*)
			echo "with-version.sh: no quotes or backslashes in the keystore's path or alias" >&2
			exit 1
			;;
		esac
		if [ ! -f "$EASYLPAC_ANDROID_KEYSTORE" ]; then
			echo "with-version.sh: no keystore at $EASYLPAC_ANDROID_KEYSTORE" >&2
			exit 1
		fi
		# After the versionCode line, the last of "android".
		SIGNED=$(awk -v ks="$EASYLPAC_ANDROID_KEYSTORE" -v alias="$ALIAS" '
			/^    "versionCode": [0-9]+$/ {
				print $0 ","
				print "    \"keystore\": \"" ks "\","
				print "    \"keyAlias\": \"" alias "\""
				done = 1
				next
			}
			{ print }
			END { if (!done) exit 1 }
		' "$CONFIG") || {
			echo "with-version.sh: android.versionCode is not the last of \"android\" in mygo.json" >&2
			exit 1
		}
		printf '%s\n' "$SIGNED" >"$CONFIG"
		echo "with-version.sh: signed with the key $ALIAS of $EASYLPAC_ANDROID_KEYSTORE" >&2
	fi
fi

"$@"

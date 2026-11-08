#!/usr/bin/env bash
# Updates the eUICC reference data committed in internal/registry/data
# from the eUICC Manual, with the download date (UTC) as the data version.
# Run it by hand, review the diff and commit it: builds (local and CI)
# only use the committed data and never download it, and the app never
# loads it online.
#
#   EUM: https://euicc-manual.osmocom.org/docs/pki/eum/ (References: accredited.json)
#   CI:  https://euicc-manual.osmocom.org/docs/pki/ci/  (References: manifest.json)
#   CI certificates: https://euicc-manual.osmocom.org/docs/pki/ci/files/<id>.txt and bundle*.pem
#   SM-DP+ error codes: https://euicc-manual.osmocom.org/docs/rsp/sm-dp-plus/ (error-codes.json)
set -euo pipefail

BASE="${EUICC_MANUAL_BASE:-https://euicc-manual.osmocom.org}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/internal/registry/data"
TMP="$(mktemp -d)"
BACKUP="$(mktemp -d)"
trap 'rm -rf "$TMP" "$BACKUP"' EXIT

curl -fsSL --retry 3 -o "$TMP/accredited.json" "$BASE/docs/pki/eum/accredited.json"
curl -fsSL --retry 3 -o "$TMP/manifest.json" "$BASE/docs/pki/ci/manifest.json"
curl -fsSL --retry 3 -o "$TMP/error-codes.json" "$BASE/docs/rsp/sm-dp-plus/error-codes.json"
# CI certificates (References: files/<key id>.txt, the first six hex
# digits of each key ID in manifest.json; some have none) and the bundles
# that say which are live, test and mainland China CIs.
mkdir -p "$TMP/ci"
for id in $(grep -o '"key-id": *"[0-9a-f]*"' "$TMP/manifest.json" | grep -o '[0-9a-f]*"$' | tr -d '"'); do
	code=$(curl -sSL --retry 3 -o "$TMP/ci/$id.txt" -w '%{http_code}' "$BASE/docs/pki/ci/files/$id.txt" || true)
	if [ "$code" != 200 ]; then
		rm -f "$TMP/ci/$id.txt"
		echo "update-euicc-data: no certificate published for CI $id ($code)" >&2
	fi
done
for b in bundle bundle-tests bundle-china; do
	curl -fsSL --retry 3 -o "$TMP/ci/$b.pem" "$BASE/docs/pki/ci/$b.pem"
done
date -u +%Y-%m-%d > "$TMP/VERSION"

FILES="accredited.json manifest.json error-codes.json VERSION"
for f in $FILES; do cp "$DEST/$f" "$BACKUP/" 2>/dev/null || true; done
[ -d "$DEST/ci" ] && cp -R "$DEST/ci" "$BACKUP/ci"
for f in $FILES; do cp "$TMP/$f" "$DEST/"; done
rm -rf "$DEST/ci" && cp -R "$TMP/ci" "$DEST/ci"

# The registry's tests parse the embedded files; keep the old data if the
# new files do not have the expected shape.
if ! (cd "$ROOT" && go test -count=1 ./internal/registry >/dev/null); then
	echo "update-euicc-data: downloaded data failed validation; keeping the previous data" >&2
	for f in $FILES; do cp "$BACKUP/$f" "$DEST/" 2>/dev/null || true; done
	[ -d "$BACKUP/ci" ] && rm -rf "$DEST/ci" && cp -R "$BACKUP/ci" "$DEST/ci"
	exit 1
fi
echo "eUICC data $(cat "$DEST/VERSION"): $(wc -c <"$DEST/accredited.json") B EUM, $(wc -c <"$DEST/manifest.json") B CI, $(wc -c <"$DEST/error-codes.json") B SM-DP+ error codes, $(ls "$DEST/ci" | grep -c '\.txt$') CI certificates"

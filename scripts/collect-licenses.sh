#!/bin/sh
# Collects what a release ships besides the apps: the licenses of
# EasyLPAC and of the statically linked libraries, and the complete
# source (with the vendored libeuicc and the scripts that rebuild it), as
# LGPL-2.1 asks of statically linked libeuicc.
#
# Usage: scripts/collect-licenses.sh <dir>
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
DIR="${1:?usage: collect-licenses.sh <dir>}"
mkdir -p "$DIR"
cp "$ROOT/LICENSE" "$DIR/LICENSE"
cp "$ROOT/third_party/lpac/euicc/LICENSE" "$DIR/LICENSE-libeuicc"
cp "$ROOT/third_party/lpac/LICENSES/MIT.txt" "$DIR/LICENSE-cjson-ext"
CJSON=$(find "$ROOT"/third_party/libeuicc -path '*/cjson/LICENSE' | head -n 1)
cp "$CJSON" "$DIR/LICENSE-cjson"
cat >"$DIR/LICENSE-euicc-manual-data" <<NOTICE
eUICC reference data bundled in EasyLPAC

EasyLPAC embeds, unmodified, the following files from the eUICC Manual
(https://euicc-manual.osmocom.org, source: https://gitea.osmocom.org/sim-card/euicc-manual),
downloaded on $(cat "$ROOT/internal/registry/data/VERSION"):

  docs/pki/eum/accredited.json
  docs/pki/ci/manifest.json
  docs/pki/ci/bundle.pem, bundle-tests.pem, bundle-china.pem
  docs/pki/ci/files/<key id>.txt
  docs/rsp/sm-dp-plus/error-codes.json

The eUICC Manual is licensed under the Creative Commons
Attribution-NonCommercial-ShareAlike 4.0 International License
(CC BY-NC-SA 4.0): https://creativecommons.org/licenses/by-nc-sa/4.0/
This license applies to the files above, not to EasyLPAC's own code
(see LICENSE).
NOTICE
# git archive leaves submodules out: add lpac's tree (libeuicc's source,
# which the LGPL asks for) under third_party/lpac/.
WORK=$(mktemp -d)
git -C "$ROOT" archive --format=tar --prefix=EasyLPAC/ -o "$WORK/app.tar" HEAD
git -C "$ROOT/third_party/lpac" archive --format=tar --prefix=EasyLPAC/third_party/lpac/ -o "$WORK/lpac.tar" HEAD
tar -C "$WORK" -xf "$WORK/app.tar"
tar -C "$WORK" -xf "$WORK/lpac.tar"
tar -C "$WORK" -czf "$DIR/EasyLPAC-source.tar.gz" EasyLPAC
rm -rf "$WORK"
ls -l "$DIR"

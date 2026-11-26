#!/bin/sh
# Prints Android's versionCode of a version of scripts/version.sh, which
# grows as the version does:
#
#   (major*10000 + minor*100 + patch) * 10000 + slot
#
# and the slot, of 10000 for each x.y.z:
#
#   alpha.N, beta.N, rc.N    0, 2000, 4000 + N*100 + snapshot (N up to 19)
#   x.y.z itself             6000
#   x.y.z.rN.g...            6000 + N
#
# Snapshots past a slot's room (99 commits after a pre-release, 3999 after
# a release) share its last code, which still installs over itself.
set -eu

printf '%s\n' "${1:?usage: android-version-code.sh <version>}" | awk '
function fail(why) {
	printf "android-version-code.sh: %s: %s\n", orig, why > "/dev/stderr"
	exit 1
}
{
	orig = v = $0
	sub(/\.dirty$/, "", v)
	r = 0
	if (match(v, /\.r[0-9]+\.g[0-9a-f]+$/)) {
		s = substr(v, RSTART + 2)
		sub(/\..*/, "", s)
		r = s + 0
		v = substr(v, 1, RSTART - 1)
	}
	pre = ""
	if (i = index(v, "-")) {
		pre = substr(v, i + 1)
		v = substr(v, 1, i - 1)
	}
	if (v !~ /^[0-9]+\.[0-9]+\.[0-9]+$/) fail("not x.y.z[-pre][.rN.gHASH]")
	split(v, p, ".")
	if (p[1] > 20 || p[2] > 99 || p[3] > 99) fail("too large for a versionCode")
	if (pre == "") {
		slot = 6000 + (r > 3999 ? 3999 : r)
	} else {
		if (pre ~ /^alpha(\.?[0-9]+)?$/) slot = 0
		else if (pre ~ /^beta(\.?[0-9]+)?$/) slot = 2000
		else if (pre ~ /^rc(\.?[0-9]+)?$/) slot = 4000
		else fail("pre-releases are alpha.N, beta.N or rc.N")
		n = pre
		gsub(/[^0-9]/, "", n)
		if (n + 0 > 19) fail("pre-release number over 19")
		slot += (n + 0) * 100 + (r > 99 ? 99 : r)
	}
	code = (p[1] * 10000 + p[2] * 100 + p[3]) * 10000 + slot
	if (code > 2100000000) fail("too large for a versionCode")
	printf "%d\n", (code > 0 ? code : 1)
}'

#!/bin/sh
# Prints the version of this checkout, from its git tags:
#
#   on a release tag         0.9.3
#   N commits after it       0.9.3.r12.ga4e79e0
#   uncommitted changes      0.9.3.r12.ga4e79e0.dirty (0.9.3.r0.g….dirty on the tag)
#
# Release tags are x.y.z, optionally with a pre-release (0.10.0-rc.1);
# older tags of another form (0.8.1.1) are skipped. A snapshot sorts after
# the tag it follows and before the next release, for dpkg, rpm and
# pacman alike, and is the pkgver of an Arch -git package as it is. It
# holds no "+": mygo turns that into "~" for dpkg, which sorts the
# snapshot before its tag.
#
# EASYLPAC_VERSION, when set, is printed instead, checked for these forms.
# Needs the history and the tags: in CI, check out with fetch-depth: 0.
set -eu

semver='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
snapshot='(\.r[0-9]+\.g[0-9a-f]+)?(\.dirty)?$'

if [ -n "${EASYLPAC_VERSION:-}" ]; then
	if ! printf '%s\n' "$EASYLPAC_VERSION" | grep -Eq "${semver%\$}$snapshot"; then
		echo "version.sh: EASYLPAC_VERSION=$EASYLPAC_VERSION is not x.y.z[-pre][.rN.gHASH]" >&2
		exit 1
	fi
	printf '%s\n' "$EASYLPAC_VERSION"
	exit 0
fi

cd "$(dirname -- "$0")/.."

# Every tag not of the release form is excluded from describe.
set --
for t in $(git tag -l); do
	printf '%s\n' "$t" | grep -Eq "$semver" || set -- "$@" --exclude "$t"
done

if ! desc=$(git describe --tags --long --abbrev=7 --dirty=-dirty --match '[0-9]*' "$@" 2>/dev/null); then
	echo "version.sh: no release tag (x.y.z) is reachable from HEAD;" \
		"a shallow clone has none: fetch the full history and tags" >&2
	exit 1
fi

# <tag>-<count>-g<hash>[-dirty], where the tag itself may hold "-".
dirty=
case "$desc" in *-dirty)
	dirty=1
	desc=${desc%-dirty}
	;;
esac
hash=${desc##*-g}
desc=${desc%-g*}
count=${desc##*-}
tag=${desc%-*}

if [ "$count" = 0 ]; then
	# Of several release tags on HEAD (0.10.0-rc.1, then 0.10.0 on the
	# same commit), the newest: a pre-release sorts before its release.
	for t in $(git -c versionsort.suffix=- tag --points-at HEAD --sort=-v:refname); do
		if printf '%s\n' "$t" | grep -Eq "$semver"; then
			tag=$t
			break
		fi
	done
	v=$tag
	[ -z "$dirty" ] || v="$tag.r0.g$hash.dirty"
else
	v="$tag.r$count.g$hash"
	[ -z "$dirty" ] || v="$v.dirty"
fi
printf '%s\n' "$v"

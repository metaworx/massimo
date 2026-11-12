#!/usr/bin/env bash
# Prints the tag `make release <flavour>` cuts next, from the repository's
# tags on stdin (git tag -l): the version tags alone count, vX.Y.Z and
# vX.Y.Z-<label>.N. The version is the newest one any version tag has,
# or the next patch, minor or major of it with BUMP; a pre-release
# counts its label's number up at that version, and stable is the version
# itself. It refuses what would sort before a release already out: a
# pre-release of a version released as stable (BUMP starts the next), one
# of a flavour less stable than one already cut at that version, and a
# stable release that exists. packaging/README.md, "Versions"; the
# Makefile's release.
set -euo pipefail

usage() {
	echo "usage: git tag -l | next-version.sh <alpha|beta|rc|stable> [patch|minor|major]" >&2
	exit 2
}
case "$#" in 1 | 2) ;; *) usage ;; esac
flavour=$1
bump=${2:-}
# The numbered flavours, the least stable first (internal/update.Flavours).
order="alpha beta rc stable"
rank() { # <flavour>: its place in order, nothing for another name
	local i=0 f
	for f in $order; do
		[ "$f" = "$1" ] && { echo "$i"; return; }
		i=$((i + 1))
	done
}
[ -n "$(rank "$flavour")" ] || usage
case "$bump" in "" | patch | minor | major) ;; *) usage ;; esac

tags=$(grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z][a-z0-9]*\.[0-9]+)?$' || true)
[ -n "$tags" ] || {
	echo "next-version.sh: no version tag to count from" >&2
	exit 1
}
base=$(printf '%s\n' "$tags" | sed -E 's/^v([0-9]+\.[0-9]+\.[0-9]+).*/\1/' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n 1)
IFS=. read -r x y z <<<"$base"
case "$bump" in
patch) z=$((z + 1)) ;;
minor) y=$((y + 1)) z=0 ;;
major) x=$((x + 1)) y=0 z=0 ;;
esac
next=$x.$y.$z
if printf '%s\n' "$tags" | grep -qx "v$next"; then
	echo "next-version.sh: v$next is released as stable; BUMP=patch, minor or major starts the next version" >&2
	exit 1
fi
if [ "$flavour" = stable ]; then
	printf 'v%s\n' "$next"
	exit 0
fi
quoted=${next//./\\.}
for label in $(printf '%s\n' "$tags" | sed -nE "s/^v$quoted-([a-z][a-z0-9]*)\.[0-9]+$/\1/p" | sort -u); do
	r=$(rank "$label")
	if [ -n "$r" ] && [ "$r" -gt "$(rank "$flavour")" ]; then
		echo "next-version.sh: v$next has a $label release already, which a $flavour one would sort before; BUMP starts the next version" >&2
		exit 1
	fi
done
n=$(printf '%s\n' "$tags" | sed -nE "s/^v$quoted-$flavour\.([0-9]+)$/\1/p" | sort -n | tail -n 1)
printf 'v%s-%s.%s\n' "$next" "$flavour" "$((${n:-0} + 1))"

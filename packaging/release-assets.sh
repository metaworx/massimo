#!/usr/bin/env bash
# Checks that a GitHub release holds every file named, under the file's
# own name (release.yml, after each upload). GitHub renames an asset whose
# name it does not keep - the ~ of a Debian version became a dot - and
# SHA256SUMS and the signed manifest would then name files the release
# lacks; a rolling release would delete the renamed ones as not its own.
# Exits 1 and names each file missing; the release may hold more.
set -euo pipefail

usage() {
	echo "usage: $0 <release tag> <file>..." >&2
	exit 2
}

[ "$#" -ge 2 ] || usage
release="$1"
shift

assets="$(gh release view "$release" --json assets --jq '.assets[].name')"
missing=0
for f in "$@"; do
	name="$(basename "$f")"
	if ! grep -qxF -- "$name" <<<"$assets"; then
		echo "::error::release ${release} has no asset ${name}; GitHub may have renamed it"
		missing=1
	fi
done
[ "$missing" -eq 0 ] || exit 1
echo "release-assets: ${release} holds all $# files under their own names"

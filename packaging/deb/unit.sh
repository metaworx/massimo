#!/usr/bin/env bash
# Derives the Debian package's unit from the embedded one, at build time
# (`make packages`): the package installs the binary as /usr/bin/massimo,
# not /usr/local/bin, and updates come through apt, so the update helper's
# request flag goes. Both substitutions must match, or the unit changed in
# a way this script does not know.
set -euo pipefail

usage() {
	echo "usage: $0 <packaging/systemd/massimo.service> <out>" >&2
	exit 2
}

[ $# -eq 2 ] || usage
src=$1
out=$2

grep -q '/usr/local/bin/massimo' "$src" || {
	echo "$0: $src: no /usr/local/bin/massimo to replace" >&2
	exit 1
}
grep -q -- ' -update-request /var/lib/massimo/update-request' "$src" || {
	echo "$0: $src: no -update-request flag to drop" >&2
	exit 1
}
mkdir -p "$(dirname "$out")"
{
	printf '%s\n' \
		'# Installed by the massimo package: the binary is /usr/bin/massimo and' \
		'# updates come through apt, so there is no update helper here. Derived' \
		'# from packaging/systemd/massimo.service by packaging/deb/unit.sh at' \
		'# build time; change that file, not this one.' \
		''
	sed -e 's#/usr/local/bin/massimo#/usr/bin/massimo#g' \
		-e 's# -update-request /var/lib/massimo/update-request##' "$src"
} >"$out"

#!/usr/bin/env bash
# Derives the Debian package's unit from the embedded one, at build time
# (`make packages`): the package installs the binary as /usr/bin/massimo,
# not /usr/local/bin. The update helper's request flag stays: the
# package's own helper (packaging/deb/massimo-update.service) runs apt
# when the web UI asks. The substitution must match, and the flag be
# there, or the unit changed in a way this script does not know.
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
	echo "$0: $src: no -update-request flag for the package's update helper" >&2
	exit 1
}
mkdir -p "$(dirname "$out")"
{
	printf '%s\n' \
		'# Installed by the massimo package: the binary is /usr/bin/massimo, and' \
		'# the update helper the request names runs apt (massimo-update.service' \
		'# of the package). Derived from packaging/systemd/massimo.service by' \
		'# packaging/deb/unit.sh at build time; change that file, not this one.' \
		''
	sed -e 's#/usr/local/bin/massimo#/usr/bin/massimo#g' "$src"
} >"$out"

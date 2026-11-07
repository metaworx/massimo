#!/usr/bin/env bash
# The apt repository built from dist/ with a throwaway key, verified, and
# installed from in a Debian container (`make apt-test`, CI's "Debian
# package" job): proves the site's shape, the signature and the keyring
# package without any key of the project's. Needs gpg, apt-ftparchive and
# docker on the host, and the packages of `make packages` in dist/.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
image=${1:-debian:trixie-slim}
work=$(mktemp -d)
trap 'rm -rf "${work:?}"' EXIT

export GNUPGHOME=$work/gnupg
mkdir -m 700 "$GNUPGHOME"
gpg --batch --quiet --quick-gen-key --passphrase '' 'Massimo CI <ci@example.invalid>' rsa3072 sign 1d
key=$(gpg --list-keys --with-colons | awk -F: '/^fpr/ { print $10; exit }')

"$root/packaging/apt/publish.sh" --out "$work/site" --key "$key" --from "$root/dist" --channel alpha

# The suite verifies against the exported key, as apt will check it.
gpg --export "$key" >"$work/keyring.gpg"
gpgv --keyring "$work/keyring.gpg" "$work/site/apt/dists/alpha/InRelease" 2>/dev/null || {
	echo "FAIL: InRelease does not verify" >&2
	exit 1
}
for f in apt/massimo.asc apt/massimo-archive-keyring.deb apt/massimo.sources index.html apt/dists/alpha/Release.gpg \
	apt/dists/alpha/main/binary-amd64/Packages.gz apt/dists/alpha/main/binary-armhf/Packages; do
	[ -s "$work/site/$f" ] || {
		echo "FAIL: $f missing" >&2
		exit 1
	}
done
grep -q '^Package: massimo-archive-keyring$' "$work/site/apt/dists/alpha/main/binary-arm64/Packages" || {
	echo "FAIL: the keyring package is not listed for arm64" >&2
	exit 1
}

# A rotated key makes a newer keyring package, whatever its expiry: here
# one without any, after the first that expires tomorrow.
keyring_version() { # <site>
	local debs=("$1"/apt/pool/alpha/massimo-archive-keyring_*_all.deb)
	dpkg-deb -f "${debs[0]}" Version
}
sleep 1
gpg --batch --quiet --quick-gen-key --passphrase '' 'Massimo CI rotated <ci@example.invalid>' rsa3072 sign never
rotated=$(gpg --list-keys --with-colons | awk -F: '/^fpr/ { f = $10 } END { print f }')
[ "$rotated" != "$key" ] || {
	echo "FAIL: no second key" >&2
	exit 1
}
"$root/packaging/apt/publish.sh" --out "$work/rotated" --key "$rotated" --from "$root/dist" --channel alpha >/dev/null
dpkg --compare-versions "$(keyring_version "$work/rotated")" gt "$(keyring_version "$work/site")" || {
	echo "FAIL: the rotated key's keyring package $(keyring_version "$work/rotated") is not newer than $(keyring_version "$work/site")" >&2
	exit 1
}

# A container installs from it over file:// as packaging/README.md says:
# the keyring package first, then the snippet, whose Signed-By must name
# the file that package owns, so that a new keyring package is what apt
# verifies with; Debian's own sources stay for the dependencies (adduser
# is not in the slim images), and apt-cache policy shows the package came
# from the suite.
chmod -R a+rX "$work/site"
docker run --rm -v "$work/site/apt:/repo:ro" "$image" bash -euo pipefail -c '
	export DEBIAN_FRONTEND=noninteractive
	apt-get -qq install -y /repo/massimo-archive-keyring.deb >/dev/null
	sed "s#^URIs: .*#URIs: file:///repo#" /repo/massimo.sources >/etc/apt/sources.list.d/massimo.sources
	signed_by=$(sed -n "s/^Signed-By: *//p" /etc/apt/sources.list.d/massimo.sources)
	[ "$(dpkg -S "$signed_by" 2>/dev/null)" = "massimo-archive-keyring: $signed_by" ] ||
		{ echo "FAIL: Signed-By names $signed_by, which the keyring package does not own" >&2; exit 1; }
	apt-get -qq update
	apt-get -qq install -y massimo >/dev/null
	grep -q "alpha/main" <<<"$(apt-cache policy massimo)" || { echo "FAIL: not from the suite alpha" >&2; exit 1; }
	grep -q "alpha/main" <<<"$(apt-cache policy massimo-archive-keyring)" ||
		{ echo "FAIL: the keyring package is not in the suite alpha" >&2; exit 1; }
	grep -q "^massimo " <<<"$(massimo version)" || { echo "FAIL: massimo version" >&2; exit 1; }
	[ "$(head -n 1 /usr/share/massimo/packaged)" = apt ] || { echo "FAIL: marker" >&2; exit 1; }
	echo "apt test: ok"
'

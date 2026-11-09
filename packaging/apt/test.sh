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

# The package's update helper (packaging/README.md, "Debian package"): a
# newer package made from the amd64 one, a second site that has it, and a
# box on the older one switched over to that site; then the request and
# the helper's command as its unit runs it, but without -health (nothing
# serves here), while another process holds apt's lists lock for 8 s. The
# helper fetches the massimo source alone, waits for the lock, and apt
# installs the newer package.
debs=("$root"/dist/massimo_*_amd64.deb)
old=$(dpkg-deb -f "${debs[0]}" Version)
new=$old+t1
dpkg --compare-versions "$new" gt "$old" || {
	echo "FAIL: $new does not sort after $old" >&2
	exit 1
}
mkdir -p "$work/from"
dpkg-deb -R "${debs[0]}" "$work/newer"
sed -i "s/^Version: .*/Version: $new/" "$work/newer/DEBIAN/control"
dpkg-deb --root-owner-group -b "$work/newer" "$work/from/massimo_${new}_amd64.deb" >/dev/null
cp "${debs[0]}" "$work/from/"
"$root/packaging/apt/publish.sh" --out "$work/upgrade" --key "$key" --from "$work/from" --channel alpha >/dev/null
chmod -R a+rX "$work/upgrade"
docker run --rm -v "$work/site/apt:/site1:ro" -v "$work/upgrade/apt:/site2:ro" -e OLD="$old" -e NEW="$new" "$image" bash -euo pipefail -c '
	fail() { echo "FAIL: $*" >&2; exit 1; }
	version() { dpkg-query -W --showformat="\${Version}" massimo; }
	export DEBIAN_FRONTEND=noninteractive
	ln -s /site1 /repo
	apt-get -qq install -y /repo/massimo-archive-keyring.deb >/dev/null
	sed "s#^URIs: .*#URIs: file:///repo#" /repo/massimo.sources >/etc/apt/sources.list.d/massimo.sources
	apt-get -qq update
	apt-get -qq install -y massimo >/dev/null
	[ "$(version)" = "$OLD" ] || fail "installed $(version), not $OLD"
	[ -L /etc/systemd/system/paths.target.wants/massimo-update.path ] || fail "the update helper is not enabled"
	ln -sfn /site2 /repo
	mkdir -p /var/lib/massimo
	echo alpha >/var/lib/massimo/update-request
	perl -e '"'"'use Fcntl; open(my $f, ">>", "/var/lib/apt/lists/lock") or die; my $l = pack("ssx4qqix4", F_WRLCK, 0, 0, 0, 0); fcntl($f, F_SETLK, $l) or die "lock: $!"; sleep 8'"'"' &
	lock=$!
	sleep 1
	helper=$(sed -n "s/^ExecStart=//p" /usr/lib/systemd/system/massimo-update.service | sed "s/ -health .*//")
	start=$SECONDS
	$helper
	wait "$lock" || fail "the lists lock was not held"
	[ $((SECONDS - start)) -ge 5 ] || fail "the helper did not wait for the lists lock"
	[ "$(version)" = "$NEW" ] || fail "the helper left $(version), not $NEW"
	grep -q "\"state\": \"done\"" /var/lib/massimo-update/status.json || fail "status: $(cat /var/lib/massimo-update/status.json)"
	[ ! -e /var/lib/massimo/update-request ] || fail "the request was not taken"
	echo "apt update helper test: ok"
'

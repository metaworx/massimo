#!/usr/bin/env bash
# The apt repository built from dist/ with a throwaway key, verified, and
# installed from in a Debian container (`make apt-test`, CI's "Debian
# package" job): proves the repository's shape, the signature and the
# keyring package without any key of the project's, and the site that
# packaging/site/build.sh makes of it, from the repository and from its
# index asset. Needs gpg, apt-ftparchive, curl and docker on the host, and
# the packages of `make packages` in dist/.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
image=${1:-debian:trixie-slim}
work=$(mktemp -d)
trap 'rm -rf "${work:?}"' EXIT

export GNUPGHOME=$work/gnupg
mkdir -m 700 "$GNUPGHOME"
gpg --batch --quiet --quick-gen-key --passphrase '' 'Massimo CI <ci@example.invalid>' rsa3072 sign 1d
key=$(gpg --list-keys --with-colons | awk -F: '/^fpr/ { print $10; exit }')

"$root/packaging/apt/publish.sh" --out "$work/site" --key "$key" --from "$root/dist" --channel alpha --index-asset "$work/index.tar.gz"

# The suite verifies against the exported key, as apt will check it.
gpg --export "$key" >"$work/keyring.gpg"
gpgv --keyring "$work/keyring.gpg" "$work/site/apt/dists/alpha/InRelease" 2>/dev/null || {
	echo "FAIL: InRelease does not verify" >&2
	exit 1
}
for f in apt/massimo.asc apt/massimo-archive-keyring.deb apt/massimo.sources \
	apt/dists/alpha/Release.gpg apt/dists/alpha/main/binary-amd64/Packages.gz apt/dists/alpha/main/binary-armhf/Packages; do
	[ -s "$work/site/$f" ] || {
		echo "FAIL: $f missing" >&2
		exit 1
	}
done

# The index asset: the signed suites as the site has them, and pool.tsv
# naming every package of the pool with its sum; a --from directory's
# packages are in the tarball itself.
mkdir "$work/index"
tar -C "$work/index" -xzf "$work/index.tar.gz"
for f in apt/dists/alpha/InRelease apt/massimo.asc apt/massimo.sources apt/massimo-archive-keyring.deb apt/pool.tsv; do
	[ -s "$work/index/$f" ] || {
		echo "FAIL: the index asset lacks $f" >&2
		exit 1
	}
done
cmp -s "$work/index/apt/dists/alpha/InRelease" "$work/site/apt/dists/alpha/InRelease" || {
	echo "FAIL: the index asset's InRelease is not the site's" >&2
	exit 1
}
pool=$(cd "$work/site/apt" && find pool -name '*.deb' | sort)
if [ "$(cut -f1 "$work/index/apt/pool.tsv" | sort)" != "$pool" ]; then
	echo "FAIL: pool.tsv does not list the pool" >&2
	exit 1
fi
while IFS=$'\t' read -r path sum url; do
	if [ "$url" != included ] || [ "$(sha256sum "$work/index/apt/$path" | cut -d' ' -f1)" != "$sum" ]; then
		echo "FAIL: pool.tsv's $path ($url) is not the package in the tarball" >&2
		exit 1
	fi
done <"$work/index/apt/pool.tsv"

# The site, built from the repository and again from its index asset
# alone, as a run of main builds it: the same suites and pool, and the
# pages of packaging/site, the index with the suites in place of its
# placeholder, the privacy policy and the stylesheet as they are.
"$root/packaging/site/build.sh" --out "$work/pages" --apt "$work/site/apt" --no-docs
"$root/packaging/site/build.sh" --out "$work/pages-index" --apt-index "$work/index.tar.gz" --no-docs
for d in pages pages-index; do
	cmp -s "$work/site/apt/dists/alpha/InRelease" "$work/$d/apt/dists/alpha/InRelease" || {
		echo "FAIL: $d's InRelease is not the repository's" >&2
		exit 1
	}
	if [ "$(cd "$work/$d/apt" && find pool -name '*.deb' | sort)" != "$pool" ]; then
		echo "FAIL: $d's pool is not the repository's" >&2
		exit 1
	fi
	for f in site.css docs.js; do
		cmp -s "$root/packaging/site/$f" "$work/$d/$f" || {
			echo "FAIL: $d's $f is not packaging/site's" >&2
			exit 1
		}
	done
done
index=$(<"$work/pages/index.html")
if grep -qF '{{' <<<"$index"; then
	echo "FAIL: index.html keeps a placeholder" >&2
	exit 1
fi
grep -qF 'One suite per update channel: alpha.' <<<"$index" || {
	echo "FAIL: index.html does not name the suite alpha" >&2
	exit 1
}
cmp -s "$root/packaging/site/privacy.html" "$work/pages/privacy.html" || {
	echo "FAIL: privacy.html is not packaging/site's" >&2
	exit 1
}
grep -qx 'URIs: https://massimo.metaworx.ch/apt' "$work/site/apt/massimo.sources" || {
	echo "FAIL: massimo.sources does not name https://massimo.metaworx.ch/apt" >&2
	exit 1
}
grep -q '^Package: massimo-archive-keyring$' "$work/site/apt/dists/alpha/main/binary-arm64/Packages" || {
	echo "FAIL: the keyring package is not listed for arm64" >&2
	exit 1
}

# Only a version tag of the full shape is a release: publish.sh's pool and
# build.sh's newest index, from a gh that lists v0.1.0-alpha.1 and v1.2.3
# among tags that only look like one, take those two alone. v1.2.3 comes
# last, as an older release of another channel does.
mkdir "$work/bin"
cat >"$work/bin/gh" <<'GH'
#!/usr/bin/env bash
# gh for the version-tag checks: the releases of $STUB_TAGS, newest first,
# each with one package ($STUB_DEB) and the index asset ($STUB_INDEX).
set -euo pipefail
printf '%s\n' "$*" >>"$STUB_LOG"
cmd="$1 $2" tag=${3:-} jq="" dir="" pattern=""
while [ $# -gt 0 ]; do
	case "$1" in
	--jq) jq=$2 ;;
	--dir) dir=$2 ;;
	--pattern) pattern=$2 ;;
	esac
	shift
done
case "$cmd" in
"release list") jq -nc '$ARGS.positional | map({tagName: .})' --args $STUB_TAGS | jq -r "$jq" ;;
"release view") jq -nc --arg deb "$(basename "$STUB_DEB")" '{assets: [{name: $deb}, {name: "massimo-apt-index.tar.gz"}]}' | jq -r "$jq" ;;
"release download")
	case "$pattern" in
	*.deb) cp "$STUB_DEB" "$dir/" ;;
	massimo-apt-index.tar.gz) cp "$STUB_INDEX" "$dir/massimo-apt-index.tar.gz" ;;
	esac
	;;
*) echo "gh stub: $cmd" >&2; exit 1 ;;
esac
GH
chmod +x "$work/bin/gh"
stub_deb=$(find "$root/dist" -maxdepth 1 -name 'massimo_*_amd64.deb' | head -1)
export STUB_LOG=$work/gh.log STUB_DEB=$stub_deb STUB_INDEX=$work/index.tar.gz
export STUB_TAGS='v1.2 verification-test v1..2-alpha.1 v1.2.3-rc release/devel v0.1.0-alpha.1 v1.2.3'
PATH=$work/bin:$PATH "$root/packaging/apt/publish.sh" --out "$work/tags" --key "$key" >/dev/null || {
	echo "FAIL: publish.sh stops on releases of several channels" >&2
	exit 1
}
PATH=$work/bin:$PATH "$root/packaging/site/build.sh" --out "$work/pages-newest" --apt-index newest --no-docs >/dev/null
suites=$(cd "$work/tags/apt/dists" && echo *)
if [ "$suites" != "alpha stable" ]; then
	echo "FAIL: publish.sh made the suites $suites, not alpha stable" >&2
	exit 1
fi
if grep -E '^release (view|download) ' "$STUB_LOG" | grep -Ev '^release [a-z]+ (v0\.1\.0-alpha\.1|v1\.2\.3) '; then
	echo "FAIL: publish.sh or build.sh took a release whose tag is no version" >&2
	exit 1
fi
unset STUB_LOG STUB_DEB STUB_INDEX STUB_TAGS

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

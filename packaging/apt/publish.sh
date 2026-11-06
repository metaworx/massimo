#!/usr/bin/env bash
# Builds the apt repository site (packaging/README.md, "The apt
# repository"): the Debian packages of the newest releases of every
# update channel, one suite per channel, indexed with apt-ftparchive and
# signed with the repository's OpenPGP key. The site has no state of its
# own: the release workflow rebuilds it from GitHub's releases at every
# release and deploys it to the project's GitHub Pages; the test feeds it
# a directory of packages and a throwaway key instead.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: publish.sh --out <dir> --key <fingerprint> [--releases <n>] [--repo <owner/name>]
       publish.sh --out <dir> --key <fingerprint> --from <dir> --channel <name>

  --out <dir>        the site: <dir>/apt holds the repository, <dir>/index.html the page
  --key <fpr>        the OpenPGP key in gpg's keyring that signs the suites; its
                     passphrase, if any, in APT_KEY_PASSPHRASE
  --releases <n>     the newest releases kept per channel (default 10)
  --repo <o/n>       the GitHub repository the releases come from (default metaworx/massimo)
  --from <dir>       take every .deb in <dir> instead of GitHub's releases …
  --channel <name>   … as the one suite <name>
  --base-url <url>   what the sources snippet names (default https://metaworx.github.io/massimo/apt)

NFPM names the nfpm command for the keyring package (default: go run at the Makefile's version).
EOF
	exit 2
}

root=$(cd "$(dirname "$0")/../.." && pwd)
out=""
key=""
releases=10
repo=metaworx/massimo
from=""
channel=""
base_url=https://metaworx.github.io/massimo/apt
while [ $# -gt 0 ]; do
	case "$1" in
	--out) out=$2; shift 2 ;;
	--key) key=$2; shift 2 ;;
	--releases) releases=$2; shift 2 ;;
	--repo) repo=$2; shift 2 ;;
	--from) from=$2; shift 2 ;;
	--channel) channel=$2; shift 2 ;;
	--base-url) base_url=$2; shift 2 ;;
	-h | --help) usage ;;
	*) echo "publish.sh: unknown argument $1" >&2; usage ;;
	esac
done
if [ -z "$out" ] || [ -z "$key" ]; then
	usage
fi
if [ -n "$from" ] && [ -z "$channel" ] || [ -z "$from" ] && [ -n "$channel" ]; then
	echo "publish.sh: --from and --channel go together" >&2
	usage
fi
nfpm_version=$(sed -n 's/^NFPM_VERSION *= *//p' "$root/Makefile")
NFPM=${NFPM:-go run github.com/goreleaser/nfpm/v2/cmd/nfpm@$nfpm_version}

apt=$out/apt
mkdir -p "$apt/pool" "$apt/dists"

# gpg signs with the key; a passphrase comes from the environment, never
# from the command line.
sign() { # <in> <out> <--clearsign|--detach-sign>
	local in=$1 output=$2 mode=$3
	if [ -n "${APT_KEY_PASSPHRASE:-}" ]; then
		gpg --batch --yes --local-user "$key" --pinentry-mode loopback --passphrase-fd 3 \
			"$mode" --armor --output "$output" "$in" 3<<<"$APT_KEY_PASSPHRASE"
	else
		gpg --batch --yes --local-user "$key" "$mode" --armor --output "$output" "$in"
	fi
}

# The channel of a release tag: the pre-release identifier of its name
# (v0.7.0-alpha.1 is on alpha), else stable.
channel_of() { # <tag>
	local c
	c=$(printf '%s\n' "$1" | sed -n 's/^v[0-9][0-9.]*-\([a-z][a-z0-9]*\)\.[0-9][0-9]*$/\1/p')
	printf '%s\n' "${c:-stable}"
}

# 1. The pool: the packages per channel.
if [ -n "$from" ]; then
	mkdir -p "$apt/pool/$channel"
	cp "$from"/*.deb "$apt/pool/$channel/"
else
	tags=$(gh release list --repo "$repo" --limit 200 --exclude-drafts --json tagName --jq '.[].tagName')
	channels=""
	while read -r tag; do
		[ -n "$tag" ] || continue
		channels="$channels $(channel_of "$tag")"
	done <<<"$tags"
	for ch in $(printf '%s\n' "$channels" | tr ' ' '\n' | sort -u); do
		[ -n "$ch" ] || continue
		newest=$(while read -r tag; do
			[ "$(channel_of "$tag")" = "$ch" ] && printf '%s\n' "$tag"
		done <<<"$tags" | sort -V | tail -n "$releases")
		mkdir -p "$apt/pool/$ch"
		while read -r tag; do
			[ -n "$tag" ] || continue
			debs=$(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name | select(endswith(".deb"))')
			[ -n "$debs" ] || continue
			gh release download "$tag" --repo "$repo" --pattern '*.deb' --dir "$apt/pool/$ch" --clobber
		done <<<"$newest"
	done
fi

# 2. The key, and the keyring package in every suite's pool: its version
# is the key's expiry date (its creation date for a key without one), so
# that an extended or a rotated key is a newer package and reaches the
# boxes through apt.
gpg --armor --export "$key" >"$apt/massimo.asc"
keyring=$(mktemp)
trap 'rm -f "${keyring:?}"' EXIT
gpg --export "$key" >"$keyring"
stamp=$(gpg --list-keys --with-colons "$key" | awk -F: '/^pub/ { print ($7 != "" ? $7 : $6); exit }')
keyring_version="1.0.$(date -u -d "@$stamp" +%Y%m%d)"
for pool in "$apt"/pool/*/; do
	VERSION=$keyring_version KEYRING=$keyring MAINTAINER=$(sed -n 's/^maintainer: *//p' "$root/packaging/nfpm.yaml") \
		$NFPM package -f "$root/packaging/apt/keyring.nfpm.yaml" -p deb \
		-t "$pool/massimo-archive-keyring_${keyring_version}_all.deb" >/dev/null
done

# 3. The suites: Packages per architecture (a package for every
# architecture is listed in each), Release, InRelease.
archs="amd64 arm64 armhf"
for pool in "$apt"/pool/*/; do
	ch=$(basename "$pool")
	dist=$apt/dists/$ch
	rm -rf "${dist:?}"
	for a in $archs; do
		mkdir -p "$dist/main/binary-$a"
	done
	# From the repository's root, so that Filename: is relative to it.
	(cd "$apt" && apt-ftparchive packages "pool/$ch") >"$dist/all.packages"
	awk -v dist="$dist" -v archs="$archs" '
		BEGIN { RS = ""; FS = "\n"; n = split(archs, as, " ") }
		{
			arch = ""
			for (i = 1; i <= NF; i++) if ($i ~ /^Architecture: /) arch = substr($i, 15)
			if (arch == "all") { for (i = 1; i <= n; i++) print $0 "\n" > (dist "/main/binary-" as[i] "/Packages") }
			else print $0 "\n" > (dist "/main/binary-" arch "/Packages")
		}' "$dist/all.packages"
	rm -f "$dist/all.packages"
	for a in $archs; do
		touch "$dist/main/binary-$a/Packages"
		gzip -9 -n -k -f "$dist/main/binary-$a/Packages"
	done
	(cd "$dist" && apt-ftparchive \
		-o "APT::FTPArchive::Release::Origin=Massimo" \
		-o "APT::FTPArchive::Release::Label=Massimo" \
		-o "APT::FTPArchive::Release::Suite=$ch" \
		-o "APT::FTPArchive::Release::Codename=$ch" \
		-o "APT::FTPArchive::Release::Architectures=$archs" \
		-o "APT::FTPArchive::Release::Components=main" \
		-o "APT::FTPArchive::Release::Description=Massimo, the update channel $ch" \
		release .) >"$dist/Release"
	sign "$dist/Release" "$dist/InRelease" --clearsign
	sign "$dist/Release" "$dist/Release.gpg" --detach-sign
done

# 4. The snippet and the page.
suites=$(for pool in "$apt"/pool/*/; do basename "$pool"; done | tr '\n' ' ')
cat >"$apt/massimo.sources" <<EOF
# The Massimo apt repository (https://github.com/metaworx/massimo,
# packaging/README.md, "The apt repository"): one suite per update
# channel (${suites% }); the key goes to /etc/apt/keyrings/massimo.asc.
Types: deb
URIs: $base_url
Suites: ${suites%% *}
Components: main
Signed-By: /etc/apt/keyrings/massimo.asc
EOF
cat >"$out/index.html" <<EOF
<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Massimo apt repository</title></head>
<body><h1>Massimo apt repository</h1>
<p>The Debian packages of <a href="https://github.com/metaworx/massimo">Massimo</a>, one suite per update channel: ${suites% }.
See <a href="https://github.com/metaworx/massimo/blob/main/packaging/README.md#the-apt-repository">packaging/README.md</a>
for the sources snippet; the key is <a href="apt/massimo.asc">apt/massimo.asc</a>.</p>
</body></html>
EOF
echo "publish.sh: $out holds the suites ${suites% }"

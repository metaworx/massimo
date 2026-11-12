#!/usr/bin/env bash
# Builds the project's apt repository (packaging/README.md, "The apt
# repository"): under <out>/apt the Debian packages of the newest releases
# of every update channel, one suite per channel, indexed with
# apt-ftparchive and signed with the repository's OpenPGP key. It has no
# state of its own: the release workflow rebuilds it from GitHub's
# releases at every release, and packaging/site/build.sh puts it on the
# project's site, https://massimo.metaworx.ch, with the pages and the
# docs; the test feeds it a directory of packages and a throwaway key
# instead.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: publish.sh --out <dir> --key <fingerprint> [--releases <n>] [--repo <owner/name>] [--index-asset <file>]
       publish.sh --out <dir> --key <fingerprint> --from <dir> --channel <name> [--index-asset <file>]

  --out <dir>        <dir>/apt holds the repository (packaging/site/build.sh --apt takes it)
  --key <fpr>        the OpenPGP key in gpg's keyring that signs the suites; its
                     passphrase, if any, in APT_KEY_PASSPHRASE
  --releases <n>     the newest releases kept per channel (default 10)
  --repo <o/n>       the GitHub repository the releases come from (default metaworx/massimo)
  --from <dir>       take every .deb in <dir> instead of GitHub's releases …
  --channel <name>   … as the one suite <name>
  --base-url <url>   what the sources snippet names (default https://massimo.metaworx.ch/apt)
  --index-asset <f>  also write the signed index as a tarball, for a site rebuilt without the key:
                     apt/dists, massimo.asc, massimo.sources, the keyring package and apt/pool.tsv,
                     one line per package of the pool - its path under apt/, its SHA-256 and the
                     download URL of the release file it is, or "included" for one in the tarball

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
index_asset=""
base_url=https://massimo.metaworx.ch/apt
while [ $# -gt 0 ]; do
	case "$1" in
	--out) out=$2; shift 2 ;;
	--key) key=$2; shift 2 ;;
	--releases) releases=$2; shift 2 ;;
	--repo) repo=$2; shift 2 ;;
	--from) from=$2; shift 2 ;;
	--channel) channel=$2; shift 2 ;;
	--base-url) base_url=$2; shift 2 ;;
	--index-asset) index_asset=$2; shift 2 ;;
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
# Where each package of the pool came from, by its path under apt/: the
# download URL of its release file (--index-asset).
declare -A urls=()

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
	c=$(printf '%s\n' "$1" | sed -nE 's/^v[0-9]+\.[0-9]+\.[0-9]+-([a-z][a-z0-9]*)\.[0-9]+$/\1/p')
	printf '%s\n' "${c:-stable}"
}

# 1. The pool: the packages per channel.
if [ -n "$from" ]; then
	mkdir -p "$apt/pool/$channel"
	cp "$from"/*.deb "$apt/pool/$channel/"
else
	# The numbered releases alone: a rolling build's release (release/devel)
	# gets no suite of its own until its packages' versions sort.
	tags=$(gh release list --repo "$repo" --limit 200 --exclude-drafts --json tagName --jq '.[].tagName | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+(-[a-z][a-z0-9]*\\.[0-9]+)?$"))')
	channels=""
	while read -r tag; do
		[ -n "$tag" ] || continue
		channels="$channels $(channel_of "$tag")"
	done <<<"$tags"
	for ch in $(printf '%s\n' "$channels" | tr ' ' '\n' | sort -u); do
		[ -n "$ch" ] || continue
		newest=$(while read -r tag; do
			# An if, not &&: a last tag of another channel would end the
			# loop, and with pipefail the script, with that test's status.
			if [ "$(channel_of "$tag")" = "$ch" ]; then
				printf '%s\n' "$tag"
			fi
		done <<<"$tags" | sort -V | tail -n "$releases")
		mkdir -p "$apt/pool/$ch"
		while read -r tag; do
			[ -n "$tag" ] || continue
			debs=$(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name | select(endswith(".deb"))')
			[ -n "$debs" ] || continue
			gh release download "$tag" --repo "$repo" --pattern '*.deb' --dir "$apt/pool/$ch" --clobber
			while read -r deb; do
				urls["pool/$ch/$deb"]="https://github.com/$repo/releases/download/$tag/$deb"
			done <<<"$debs"
		done <<<"$newest"
	done
fi

# 2. The key, and the keyring package in every suite's pool: its version
# is the time of this publish (UTC, to the second), which only goes up,
# so that every publish, an extended key's or a new key's, is a newer
# package. The key's own dates are not: a new key may expire sooner than
# the old one, or never.
gpg --armor --export "$key" >"$apt/massimo.asc"
keyring=$(mktemp)
trap 'rm -f "${keyring:?}"' EXIT
gpg --export "$key" >"$keyring"
keyring_version="1.0.$(date -u +%Y%m%d%H%M%S)"
for pool in "$apt"/pool/*/; do
	VERSION=$keyring_version KEYRING=$keyring MAINTAINER=$(sed -n 's/^maintainer: *//p' "$root/packaging/nfpm.yaml") \
		$NFPM package -f "$root/packaging/apt/keyring.nfpm.yaml" -p deb \
		-t "$pool/massimo-archive-keyring_${keyring_version}_all.deb" >/dev/null
done
# The same package under a name that does not change, for a box's first
# install, before apt knows the repository (packaging/README.md).
pools=("$apt"/pool/*/)
cp "${pools[0]}massimo-archive-keyring_${keyring_version}_all.deb" "$apt/massimo-archive-keyring.deb"

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

# 4. The snippet. Signed-By names the keyring package's file, so that the
# key apt verifies with is the one the newest keyring package carries; the
# box installs that package first. The pages beside the repository are
# packaging/site/build.sh's, which takes this apt/ as it is.
suites=$(for pool in "$apt"/pool/*/; do basename "$pool"; done | tr '\n' ' ')
cat >"$apt/massimo.sources" <<EOF
# The Massimo apt repository (https://massimo.metaworx.ch,
# packaging/README.md, "The apt repository"): one suite per update
# channel (${suites% }); the key comes with the package
# massimo-archive-keyring, installed first from
# $base_url/massimo-archive-keyring.deb.
Types: deb
URIs: $base_url
Suites: ${suites%% *}
Components: main
Signed-By: /usr/share/keyrings/massimo-archive-keyring.gpg
EOF

# 5. The signed index on its own (--index-asset): what a site rebuilt from
# main needs to serve the repository without the key, the pool fetched
# again by pool.tsv and checked by its sums. A package not from a release
# (the keyring package, a --from directory's) is in the tarball itself.
if [ -n "$index_asset" ]; then
	stage=$(mktemp -d)
	trap 'rm -f "${keyring:?}"; rm -rf "${stage:?}"' EXIT
	mkdir -p "$stage/apt"
	cp -R "$apt/dists" "$apt/massimo.asc" "$apt/massimo.sources" "$apt/massimo-archive-keyring.deb" "$stage/apt/"
	: >"$stage/apt/pool.tsv"
	for deb in "$apt"/pool/*/*.deb; do
		rel=${deb#"$apt"/}
		url=${urls[$rel]:-included}
		printf '%s\t%s\t%s\n' "$rel" "$(sha256sum "$deb" | cut -d' ' -f1)" "$url" >>"$stage/apt/pool.tsv"
		if [ "$url" = included ]; then
			mkdir -p "$stage/apt/$(dirname "$rel")"
			cp "$deb" "$stage/apt/$rel"
		fi
	done
	tar -C "$stage" --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner -cf - apt | gzip -9 -n >"$index_asset"
	echo "publish.sh: $index_asset holds the signed index and pool.tsv"
fi
echo "publish.sh: $out holds the suites ${suites% }"

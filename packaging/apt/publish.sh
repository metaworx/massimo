#!/usr/bin/env bash
# Builds the project's apt repository (packaging/README.md, "The apt
# repository"): under <out>/apt the Debian packages of the newest releases
# of every update channel, one suite per channel holding its own and every
# more stable channel's packages, as a box on it takes them, indexed with
# apt-ftparchive and signed with the repository's OpenPGP key. It has no
# state of its own: the release workflow rebuilds it from GitHub's
# releases at every release, and packaging/site/build.sh puts it on the
# project's site, https://massimo.metaworx.ch, with the pages and the
# docs; the test feeds it a directory of packages and a throwaway key
# instead.
#
# Two keys sign the suites, each in the one run that may use it: the
# numbered suites (alpha and up) the apt key of the version tags' runs, a
# rolling suite (devel, nightly) the rolling apt key of its own run on
# main. Their public halves are committed, archive-key.asc and
# rolling-archive-key.asc, and the keyring package carries both, one file
# each; a box's source names the file of its kind of suite, so that the
# rolling key signs nothing a box on alpha accepts. publish.sh refuses to
# sign with a key that is not the committed one for the suites it signs.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: publish.sh --out <dir> --key <fingerprint> [--releases <n>] [--repo <owner/name>] [--index-asset <file>]
       publish.sh --out <dir> --key <fingerprint> --rolling <flavour> [--releases <n>] [--repo <owner/name>] [--index-asset <file>]
       publish.sh --out <dir> --key <fingerprint> --from <dir> --channel <name> [--index-asset <file>]

  --out <dir>        <dir>/apt holds the repository (packaging/site/build.sh --apt takes it)
  --key <fpr>        the OpenPGP key in gpg's keyring that signs the suites; its
                     passphrase, if any, in APT_KEY_PASSPHRASE
  --releases <n>     the newest releases kept per channel (default 10)
  --repo <o/n>       the GitHub repository the releases come from (default metaworx/massimo)
  --rolling <f>      the one rolling suite <f>, devel or nightly: the release release/<f>, and the
                     newest numbered releases of every channel
  --from <dir>       take every .deb in <dir> instead of GitHub's releases …
  --channel <name>   … as the one suite <name>
  --archive-key <f>  the public key of the numbered suites (default packaging/apt/archive-key.asc)
  --rolling-archive-key <f>
                     that of the rolling suites (default packaging/apt/rolling-archive-key.asc)
  --base-url <url>   what the sources snippets name (default https://massimo.metaworx.ch/apt)
  --index-asset <f>  also write the signed index as a tarball, for a site rebuilt without the key:
                     apt/dists, the public keys, the snippet, the keyring package and apt/pool.tsv,
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
rolling=""
from=""
channel=""
index_asset=""
archive_key=$root/packaging/apt/archive-key.asc
rolling_archive_key=$root/packaging/apt/rolling-archive-key.asc
base_url=https://massimo.metaworx.ch/apt
while [ $# -gt 0 ]; do
	case "$1" in
	--out) out=$2; shift 2 ;;
	--key) key=$2; shift 2 ;;
	--releases) releases=$2; shift 2 ;;
	--repo) repo=$2; shift 2 ;;
	--rolling) rolling=$2; shift 2 ;;
	--from) from=$2; shift 2 ;;
	--channel) channel=$2; shift 2 ;;
	--archive-key) archive_key=$2; shift 2 ;;
	--rolling-archive-key) rolling_archive_key=$2; shift 2 ;;
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
if [ -n "$rolling" ] && [ -n "$from" ]; then
	echo "publish.sh: --rolling or --from, not both" >&2
	usage
fi

# The rolling flavours, built from main and signed by the rolling key;
# every other channel is numbered.
is_rolling() { # <channel>
	case "$1" in devel | nightly) return 0 ;; *) return 1 ;; esac
}
# The flavours, the least stable first, as update.Flavours ranks them: a
# suite takes its own packages and those of every flavour after it.
flavours="devel nightly alpha beta stable"
rank() { # <channel>: its place in flavours, nothing for another name
	local i=0 f
	for f in $flavours; do
		[ "$f" = "$1" ] && { echo "$i"; return; }
		i=$((i + 1))
	done
}
if [ -n "$rolling" ] && ! is_rolling "$rolling"; then
	echo "publish.sh: --rolling $rolling: devel or nightly" >&2
	exit 2
fi

# The kind of suites this run signs, and the committed key it must sign
# them with.
kind=numbered
if [ -n "$rolling" ] || { [ -n "$from" ] && is_rolling "$channel"; }; then
	kind=rolling
fi
fpr_of_file() { # <armored public key>
	gpg --show-keys --with-colons "$1" 2>/dev/null | awk -F: '/^fpr/ { print $10; exit }'
}
for f in "$archive_key" "$rolling_archive_key"; do
	[ -n "$(fpr_of_file "$f")" ] || {
		echo "publish.sh: $f holds no public key" >&2
		exit 1
	}
done
want=$archive_key
[ "$kind" = numbered ] || want=$rolling_archive_key
signer=$(gpg --list-keys --with-colons "$key" 2>/dev/null | awk -F: '/^fpr/ { print $10; exit }')
if [ -z "$signer" ] || [ "$signer" != "$(fpr_of_file "$want")" ]; then
	echo "publish.sh: the key ${signer:-$key} is not the $kind suites' key of $want ($(fpr_of_file "$want")); nothing signed" >&2
	exit 1
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

# fetch puts the packages of a release into pool/<dir> and notes each
# one's download URL, as GitHub gives it.
fetch() { # <tag> <dir>
	local tag=$1 dir=$2 assets name url
	assets=$(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[] | select(.name | endswith(".deb")) | [.name, .url] | @tsv')
	[ -n "$assets" ] || return 0
	mkdir -p "$apt/pool/$dir"
	gh release download "$tag" --repo "$repo" --pattern '*.deb' --dir "$apt/pool/$dir" --clobber
	while IFS=$'\t' read -r name url; do
		urls["pool/$dir/$name"]=$url
	done <<<"$assets"
}

# 1. The pool: the packages per channel, and the suites.
suites=""
if [ -n "$from" ]; then
	mkdir -p "$apt/pool/$channel"
	cp "$from"/*.deb "$apt/pool/$channel/"
	suites=$channel
else
	# The numbered releases, the newest of each channel, into
	# pool/<channel>: the numbered suites, and a rolling suite's more
	# stable packages. The rolling builds' releases (release/devel) are
	# suites of their own, signed by their own runs.
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
		while read -r tag; do
			[ -n "$tag" ] || continue
			fetch "$tag" "$ch"
		done <<<"$newest"
		[ ! -d "$apt/pool/$ch" ] || suites="$suites $ch"
	done
	suites=${suites# }
	if [ -n "$rolling" ]; then
		# The rolling build's own packages, replaced in place at each
		# build; the numbered releases as they are at this build.
		fetch "release/$rolling" "$rolling"
		[ -d "$apt/pool/$rolling" ] || {
			echo "publish.sh: release/$rolling has no packages" >&2
			exit 1
		}
		suites=$rolling
	fi
fi
[ -n "$suites" ] || {
	echo "publish.sh: no packages, no suite" >&2
	exit 1
}

# 2. The public keys, and the keyring package with both, in pool/keyring:
# its version is the time of this publish (UTC, to the second), which only
# goes up, so that every publish, an extended key's or a new key's, is a
# newer package; both kinds of run make one, with the same files. The
# keys' own dates are not: a new key may expire sooner than the old one,
# or never.
cp "$archive_key" "$apt/massimo.asc"
cp "$rolling_archive_key" "$apt/massimo-rolling.asc"
keys=$(mktemp -d)
trap 'rm -rf "${keys:?}"' EXIT
gpg --dearmor <"$archive_key" >"$keys/numbered.gpg"
gpg --dearmor <"$rolling_archive_key" >"$keys/rolling.gpg"
keyring_version="1.0.$(date -u +%Y%m%d%H%M%S)"
mkdir -p "$apt/pool/keyring"
VERSION=$keyring_version KEYRING=$keys/numbered.gpg ROLLING_KEYRING=$keys/rolling.gpg \
	MAINTAINER=$(sed -n 's/^maintainer: *//p' "$root/packaging/nfpm.yaml") \
	$NFPM package -f "$root/packaging/apt/keyring.nfpm.yaml" -p deb \
	-t "$apt/pool/keyring/massimo-archive-keyring_${keyring_version}_all.deb" >/dev/null
# The same package under a name that does not change, for a box's first
# install, before apt knows the repository (packaging/README.md).
cp "$apt/pool/keyring/massimo-archive-keyring_${keyring_version}_all.deb" "$apt/massimo-archive-keyring.deb"

# The pools of a suite: its own, every more stable channel's, and the
# keyring's. A channel the ranks do not know keeps to its own.
suite_pools() { # <suite>
	local r p pr
	echo "pool/$1"
	r=$(rank "$1")
	for p in "$apt"/pool/*/; do
		p=$(basename "$p")
		pr=$(rank "$p")
		if [ -n "$r" ] && [ -n "$pr" ] && [ "$pr" -gt "$r" ]; then
			echo "pool/$p"
		fi
	done
	echo pool/keyring
}

# 3. The suites: Packages per architecture (a package for every
# architecture is listed in each) from the suite's pools, Release,
# InRelease.
archs="amd64 arm64 armhf"
for ch in $suites; do
	dist=$apt/dists/$ch
	rm -rf "${dist:?}"
	for a in $archs; do
		mkdir -p "$dist/main/binary-$a"
	done
	# From the repository's root, so that Filename: is relative to it.
	for dir in $(suite_pools "$ch"); do
		(cd "$apt" && apt-ftparchive packages "$dir")
	done >"$dist/all.packages"
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

# 4. The snippet: massimo.sources for the numbered suites, naming the
# first of them; massimo-<flavour>.sources for a rolling one. Signed-By
# names the keyring package's file of the suite's kind, so that the key
# apt verifies with is the one the newest keyring package carries; the
# box installs that package first. The pages beside the repository are
# packaging/site/build.sh's, which takes this apt/ as it is, or assembles
# it from the runs' indexes.
snippet() { # <file> <suite> <keyring file> <what the repository holds>
	cat >"$1" <<EOF
# The Massimo apt repository (https://massimo.metaworx.ch,
# packaging/README.md, "The apt repository"): $4; the key comes with the
# package massimo-archive-keyring, installed first from
# $base_url/massimo-archive-keyring.deb.
Types: deb
URIs: $base_url
Suites: $2
Components: main
Signed-By: /usr/share/keyrings/$3
EOF
}
if [ "$kind" = numbered ]; then
	sources=massimo.sources
	snippet "$apt/$sources" "${suites%% *}" massimo-archive-keyring.gpg "one suite per update channel (${suites// /, })"
else
	sources=massimo-$suites.sources
	snippet "$apt/$sources" "$suites" massimo-rolling-archive-keyring.gpg "the rolling suite $suites, signed with the rolling key"
fi

# 5. The signed index on its own (--index-asset): what a site rebuilt from
# main needs to serve the repository without the key, the pool fetched
# again by pool.tsv and checked by its sums. A package not from a release
# (the keyring package, a --from directory's) is in the tarball itself.
if [ -n "$index_asset" ]; then
	stage=$(mktemp -d)
	trap 'rm -rf "${keys:?}" "${stage:?}"' EXIT
	mkdir -p "$stage/apt"
	cp -R "$apt/dists" "$apt/massimo.asc" "$apt/massimo-rolling.asc" "$apt/$sources" \
		"$apt/massimo-archive-keyring.deb" "$stage/apt/"
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
echo "publish.sh: $out holds the suites ${suites// /, }"

#!/usr/bin/env bash
# Renders the Homebrew formula of a release from massimo.rb.in
# (packaging/README.md, "Homebrew"): the formula is named after the update
# channel, massimo-alpha for alpha, massimo for stable, and prints its name.
# The release workflow runs it for the tag just published and pushes the
# result to the tap; for a rolling build (--commit, --version) the rolling
# job renders massimo-devel or massimo-nightly from the build's commit,
# versioned as git describe names it, which brew sorts after the tag it
# follows, and pushes it to the rolling tap. CI's macOS job renders it for
# a tarball of the checkout and builds it. The formula conflicts with the tap's other
# formulae (--conflicts; all install bin/massimo), and only with those:
# brew warns of a conflicts_with naming a formula it cannot find, and in
# developer mode refuses the install.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: render.sh --tag <vX.Y.Z[-channel.N]> --out <file> [--url <url>] [--sha256 <hex>] [--channel <name>] [--conflicts <names>]
       render.sh --commit <sha> --version <vX.Y.Z[-label.N]-<n>-g<hash>> --channel devel|nightly --out <file> [...]

  --tag <tag>        the release tag; the version and, unless --channel says, the channel come from it
  --commit <sha>     a rolling build's commit, whose archive on GitHub is the source …
  --version <v>      … and its version, as git describe gives it (the rolling release's)
  --out <file>       the formula to write; its name is printed on stdout
  --url <url>        the source tarball (default: the tag's archive on GitHub)
  --sha256 <hex>     the tarball's SHA-256 (default: downloaded and computed)
  --channel <name>   the update channel, alpha or stable (default: from the tag)
  --conflicts <names> the tap's formulae, comma-separated, to conflict with; its own name is dropped (default: none)
EOF
	exit 2
}

here=$(cd "$(dirname "$0")" && pwd)
# sha256sum on Linux, shasum on macOS.
checksum() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum; else shasum -a 256; fi
}
tag=""
commit=""
rolling_version=""
out=""
url=""
sha256=""
channel=""
others=""
while [ $# -gt 0 ]; do
	case "$1" in
	--tag) tag=$2; shift 2 ;;
	--commit) commit=$2; shift 2 ;;
	--version) rolling_version=$2; shift 2 ;;
	--out) out=$2; shift 2 ;;
	--url) url=$2; shift 2 ;;
	--sha256) sha256=$2; shift 2 ;;
	--channel) channel=$2; shift 2 ;;
	--conflicts) others=$2; shift 2 ;;
	-h | --help) usage ;;
	*) echo "render.sh: unknown argument $1" >&2; usage ;;
	esac
done
if [ -z "$out" ]; then
	usage
fi
if [ -n "$commit" ]; then
	# A rolling build: its commit, the version git describe gives it, and
	# the rolling flavour it is released as.
	if [ -n "$tag" ] || [ -z "$rolling_version" ]; then
		usage
	fi
	case "$channel" in devel | nightly) ;; *) echo "render.sh: a rolling build's channel is devel or nightly, not ${channel:-none}" >&2; exit 1 ;; esac
	if ! printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$'; then
		echo "render.sh: $commit is not a full commit hash" >&2
		exit 1
	fi
	if ! printf '%s\n' "$rolling_version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z][a-z0-9]*\.[0-9]+)?-[0-9]+-g[0-9a-f]+$'; then
		echo "render.sh: $rolling_version is not a version git describe gives a commit after a tag" >&2
		exit 1
	fi
	version=${rolling_version#v}
	[ -n "$url" ] || url="https://github.com/metaworx/massimo/archive/$commit.tar.gz"
else
	if [ -z "$tag" ] || [ -n "$rolling_version" ]; then
		usage
	fi
	# A version tag has the full shape, vX.Y.Z or vX.Y.Z-<label>.N: a label
	# names the channel, and only a tag without one is stable.
	if ! printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z][a-z0-9]*\.[0-9]+)?$'; then
		echo "render.sh: $tag is not a version tag" >&2
		exit 1
	fi
	if [ -z "$channel" ]; then
		channel=$(printf '%s\n' "$tag" | sed -nE 's/^v[0-9]+\.[0-9]+\.[0-9]+-([a-z][a-z0-9]*)\.[0-9]+$/\1/p')
		channel=${channel:-stable}
	fi
	version=${tag#v}
fi
if [ "$channel" = stable ]; then
	name=massimo
else
	name=massimo-$channel
fi
# The conflicts line: the other formulae of --conflicts, or no line.
conflicts=""
for other in $(printf '%s\n' "$others" | tr ',' ' '); do
	[ "$other" = "$name" ] && continue
	conflicts="$conflicts${conflicts:+, }\"$other\""
done
[ -z "$conflicts" ] || conflicts="conflicts_with $conflicts, because: \"both install bin/massimo\""
# The class: the name in PascalCase, massimo-alpha → MassimoAlpha, by awk,
# since macOS's sed has neither \U nor \| (CI's macOS job renders it there).
class=$(printf '%s\n' "$name" | awk -F- -v OFS= '{ for (i = 1; i <= NF; i++) $i = toupper(substr($i, 1, 1)) substr($i, 2); print }')
[ -n "$url" ] || url="https://github.com/metaworx/massimo/archive/refs/tags/$tag.tar.gz"
if [ -z "$sha256" ]; then
	sha256=$(curl -fsSL "$url" | checksum | cut -d' ' -f1)
fi

# Without conflicts the placeholder's line goes, with the blank line after it.
if [ -n "$conflicts" ]; then
	conflicts_expr="s|@CONFLICTS@|$conflicts|g"
else
	conflicts_expr='/^[[:space:]]*@CONFLICTS@[[:space:]]*$/{N;d;}'
fi
sed -e "s|@CLASS@|$class|g" -e "s|@NAME@|$name|g" -e "s|@CHANNEL@|$channel|g" \
	-e "s|@VERSION@|$version|g" -e "s|@URL@|$url|g" -e "s|@SHA256@|$sha256|g" \
	-e "$conflicts_expr" "$here/massimo.rb.in" >"$out"
printf '%s\n' "$name"

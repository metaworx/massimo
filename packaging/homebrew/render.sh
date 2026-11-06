#!/usr/bin/env bash
# Renders the Homebrew formula of a release from massimo.rb.in
# (packaging/README.md, "Homebrew"): the formula is named after the update
# channel, massimo-alpha for alpha, massimo for stable, and prints its name.
# The release workflow runs it for the tag just published and pushes the
# result to the tap; CI's macOS job renders it for a tarball of the
# checkout and builds it.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: render.sh --tag <vX.Y.Z[-channel.N]> --out <file> [--url <url>] [--sha256 <hex>] [--channel <name>]

  --tag <tag>        the release tag; the version and, unless --channel says, the channel come from it
  --out <file>       the formula to write; its name is printed on stdout
  --url <url>        the source tarball (default: the tag's archive on GitHub)
  --sha256 <hex>     the tarball's SHA-256 (default: downloaded and computed)
  --channel <name>   the update channel, alpha or stable (default: from the tag)
EOF
	exit 2
}

here=$(cd "$(dirname "$0")" && pwd)
# sha256sum on Linux, shasum on macOS.
checksum() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum; else shasum -a 256; fi
}
tag=""
out=""
url=""
sha256=""
channel=""
while [ $# -gt 0 ]; do
	case "$1" in
	--tag) tag=$2; shift 2 ;;
	--out) out=$2; shift 2 ;;
	--url) url=$2; shift 2 ;;
	--sha256) sha256=$2; shift 2 ;;
	--channel) channel=$2; shift 2 ;;
	-h | --help) usage ;;
	*) echo "render.sh: unknown argument $1" >&2; usage ;;
	esac
done
if [ -z "$tag" ] || [ -z "$out" ]; then
	usage
fi
case "$tag" in
v[0-9]*) ;;
*) echo "render.sh: $tag is not a version tag" >&2; exit 1 ;;
esac

if [ -z "$channel" ]; then
	channel=$(printf '%s\n' "$tag" | sed -n 's/^v[0-9][0-9.]*-\([a-z][a-z0-9]*\)\.[0-9][0-9]*$/\1/p')
	channel=${channel:-stable}
fi
version=${tag#v}
if [ "$channel" = stable ]; then
	name=massimo
	conflicts='conflicts_with "massimo-alpha", "massimo-beta", because: "both install bin/massimo"'
else
	name=massimo-$channel
	conflicts='conflicts_with "massimo", because: "both install bin/massimo"'
fi
# The class: the name in PascalCase, massimo-alpha → MassimoAlpha, by awk,
# since macOS's sed has neither \U nor \| (CI's macOS job renders it there).
class=$(printf '%s\n' "$name" | awk -F- -v OFS= '{ for (i = 1; i <= NF; i++) $i = toupper(substr($i, 1, 1)) substr($i, 2); print }')
[ -n "$url" ] || url="https://github.com/metaworx/massimo/archive/refs/tags/$tag.tar.gz"
if [ -z "$sha256" ]; then
	sha256=$(curl -fsSL "$url" | checksum | cut -d' ' -f1)
fi

sed -e "s|@CLASS@|$class|g" -e "s|@NAME@|$name|g" -e "s|@CHANNEL@|$channel|g" \
	-e "s|@VERSION@|$version|g" -e "s|@URL@|$url|g" -e "s|@SHA256@|$sha256|g" \
	-e "s|@CONFLICTS@|$conflicts|g" "$here/massimo.rb.in" >"$out"
printf '%s\n' "$name"

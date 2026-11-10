#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# Builds the project's site, https://massimo.metaworx.ch (packaging/README.md,
# "The site"), without a key: the apt repository under apt/, the pages of
# packaging/site at the root, and the documentation of every channel under
# docs/<channel>/, made by site.py with the guidelines' md_html.py.
# site.yml runs it for every run of release.yml; make site and
# packaging/apt/test.sh run it too. The site has no state: every run
# builds all of it again.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: build.sh --out <dir> (--apt <dir> | --apt-index <file|newest> | --no-apt)
                [--ref <channel>=<git ref>... | --no-docs] [--strict <channel>] [--repo <owner/name>]

  --out <dir>          the site; it must not exist yet
  --apt <dir>          the apt repository as publish.sh built it, copied as it is
  --apt-index <file>   the signed index of a release (massimo-apt-index.tar.gz),
                       its pool fetched by pool.tsv and every package checked by its sum;
                       "newest" takes it from the newest v* release carrying one
  --no-apt             no apt repository: the pages and docs alone, never to be deployed
  --ref <ch>=<ref>     the docs of channel <ch> from <ref>, repeatable; default every
                       release/<channel> branch or tag of origin there is, and main=origin/main
  --no-docs            no documentation: the apt repository and the pages alone (packaging/apt/test.sh)
  --strict <ch>        fail where a document under docs/ of <ch> is missing from
                       docs/fragments/_documents.md (default main)
  --repo <o/n>         the GitHub repository (default metaworx/massimo)

Exit 0 built, 1 failed, 2 usage, 3 no release carries an apt index (nothing written).
EOF
	exit 2
}

root=$(cd "$(dirname "$0")/../.." && pwd)
out=""
apt_dir=""
apt_index=""
no_apt=0
no_docs=0
refs=()
strict=main
repo=metaworx/massimo
while [ $# -gt 0 ]; do
	case "$1" in
	--out) out=$2; shift 2 ;;
	--apt) apt_dir=$2; shift 2 ;;
	--apt-index) apt_index=$2; shift 2 ;;
	--no-apt) no_apt=1; shift ;;
	--ref) refs+=("$2"); shift 2 ;;
	--no-docs) no_docs=1; shift ;;
	--strict) strict=$2; shift 2 ;;
	--repo) repo=$2; shift 2 ;;
	-h | --help) usage ;;
	*) echo "build.sh: unknown argument $1" >&2; usage ;;
	esac
done
modes=$(( (${#apt_dir} > 0) + (${#apt_index} > 0) + no_apt ))
if [ -z "$out" ] || [ "$modes" -ne 1 ] || { [ "$no_docs" = 1 ] && [ ${#refs[@]} -gt 0 ]; }; then
	usage
fi
if [ -e "$out" ]; then
	echo "build.sh: $out exists" >&2
	exit 2
fi
md_html=$root/GUIDELINES/shared/tools/md_html.py
[ "$no_docs" = 1 ] || [ -f "$md_html" ] || {
	echo "build.sh: no $md_html (the guidelines submodule)" >&2
	exit 1
}
work=$(mktemp -d)
trap 'rm -rf "${work:?}"' EXIT
mkdir -p "$work/site"
site=$work/site

# 1. The apt repository.
if [ -n "$apt_dir" ]; then
	cp -R "$apt_dir" "$site/apt"
elif [ -n "$apt_index" ]; then
	if [ "$apt_index" = newest ]; then
		apt_index=""
		for tag in $(gh release list --repo "$repo" --limit 100 --exclude-drafts --json tagName --jq '.[].tagName | select(test("^v[0-9]"))'); do
			if gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name' | grep -qx massimo-apt-index.tar.gz; then
				gh release download "$tag" --repo "$repo" --pattern massimo-apt-index.tar.gz --dir "$work"
				apt_index=$work/massimo-apt-index.tar.gz
				echo "build.sh: the apt index of $tag"
				break
			fi
		done
		if [ -z "$apt_index" ]; then
			echo "build.sh: no release carries massimo-apt-index.tar.gz: no site without the apt repository"
			exit 3
		fi
	fi
	tar -C "$site" -xzf "$apt_index" apt
	[ -s "$site/apt/pool.tsv" ] || {
		echo "build.sh: $apt_index has no apt/pool.tsv" >&2
		exit 1
	}
	while IFS=$'\t' read -r path sum url; do
		case "$path" in
		pool/*.deb) ;;
		*) echo "build.sh: pool.tsv names $path, outside the pool" >&2; exit 1 ;;
		esac
		case "$path" in *..*) echo "build.sh: pool.tsv names $path" >&2; exit 1 ;; esac
		if [ "$url" != included ]; then
			mkdir -p "$site/apt/$(dirname "$path")"
			curl -fsSL --retry 3 -o "$site/apt/$path" "$url"
		fi
		[ "$(sha256sum "$site/apt/$path" | cut -d' ' -f1)" = "$sum" ] || {
			echo "build.sh: $path is not the package the signed index names" >&2
			exit 1
		}
	done <"$site/apt/pool.tsv"
	rm "$site/apt/pool.tsv"
fi

# 2. The pages, the suites in place of the home's placeholder.
suites=none
if [ -d "$site/apt/dists" ]; then
	suites=$(find "$site/apt/dists" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort | paste -sd, - | sed 's/,/, /g')
fi
cp "$root"/packaging/site/*.html "$root"/packaging/site/*.css "$root"/packaging/site/*.js "$site/"
sed "s/{{SUITES}}/$suites/g" "$root/packaging/site/index.html" >"$site/index.html"

# 3. The docs of every channel: its commit's tree, as git archive has it.
if [ "$no_docs" = 0 ] && [ ${#refs[@]} -eq 0 ]; then
	for ch in stable beta alpha nightly devel; do
		for ref in "refs/remotes/origin/release/$ch" "refs/tags/release/$ch"; do
			if git -C "$root" rev-parse --verify --quiet "$ref^{commit}" >/dev/null; then
				refs+=("$ch=$ref")
				break
			fi
		done
	done
	if git -C "$root" rev-parse --verify --quiet "refs/remotes/origin/main^{commit}" >/dev/null; then
		refs+=("main=refs/remotes/origin/main")
	else
		refs+=("main=HEAD")
	fi
fi
for spec in ${refs[@]+"${refs[@]}"}; do
	ch=${spec%%=*}
	ref=${spec#*=}
	case "$ch" in
	[a-z]*) ;;
	*) echo "build.sh: --ref $spec: a channel is a lower-case name" >&2; exit 2 ;;
	esac
	case "$ch" in *[!a-z0-9]*) echo "build.sh: --ref $spec: a channel is a lower-case name" >&2; exit 2 ;; esac
	commit=$(git -C "$root" rev-parse --verify "$ref^{commit}")
	version=$(git -C "$root" describe --tags --match 'v[0-9]*' "$commit" 2>/dev/null || git -C "$root" rev-parse --short "$commit")
	date=$(git -C "$root" show -s --format=%cs "$commit")
	pin=$(git -C "$root" rev-parse "$commit:GUIDELINES/shared")
	tree=$work/tree-$ch
	mkdir -p "$tree"
	git -C "$root" archive "$commit" | tar -C "$tree" -xf -
	args=(--tree "$tree" --out "$site" --channel "$ch" --commit "$commit" --version "$version"
		--date "$date" --pin "$pin" --repo "$repo" --md-html "$md_html")
	[ "$ch" = "$strict" ] && args+=(--strict)
	python3 "$root/packaging/site/site.py" docs "${args[@]}"
	rm -rf "${tree:?}"
done
[ "$no_docs" = 1 ] || python3 "$root/packaging/site/site.py" index --out "$site"

# 4. Every link of every page, then the site where it was asked for.
python3 "$root/packaging/site/site.py" check --out "$site"
mkdir -p "$(dirname "$out")"
mv "$site" "$out"
echo "build.sh: $out holds the site (suites: $suites)"

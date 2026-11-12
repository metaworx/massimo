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
                [--rolling-index <flavour>=<file>... | --no-rolling]
                [--ref <channel>=<git ref>... | --no-docs] [--strict <channel>] [--repo <owner/name>]

  --out <dir>          the site; it must not exist yet
  --apt <dir>          the numbered suites as publish.sh built them, copied as they are
  --apt-index <file>   the signed index of a release (massimo-apt-index.tar.gz),
                       its pool fetched by pool.tsv and every package checked by its sum;
                       "newest" takes it from the newest version tag's release with one
  --rolling-index <f>=<file>
                       the signed index of the rolling suite <f>, repeatable; default the one
                       of each rolling release there is, release/devel and release/nightly.
                       A rolling index that is missing or does not check out leaves its
                       suite out, with a warning: a failed rolling build takes no site offline
  --no-rolling         no rolling suites
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
no_rolling=0
rolling_indexes=()
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
	--rolling-index) rolling_indexes+=("$2"); shift 2 ;;
	--no-rolling) no_rolling=1; shift ;;
	--ref) refs+=("$2"); shift 2 ;;
	--no-docs) no_docs=1; shift ;;
	--strict) strict=$2; shift 2 ;;
	--repo) repo=$2; shift 2 ;;
	-h | --help) usage ;;
	*) echo "build.sh: unknown argument $1" >&2; usage ;;
	esac
done
modes=$(( (${#apt_dir} > 0) + (${#apt_index} > 0) + no_apt ))
if [ -z "$out" ] || [ "$modes" -ne 1 ] || { [ "$no_docs" = 1 ] && [ ${#refs[@]} -gt 0 ]; } ||
	{ [ "$no_rolling" = 1 ] && [ ${#rolling_indexes[@]} -gt 0 ]; }; then
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

# unpack_index puts an index's apt/ under <dir>, its pool fetched by
# pool.tsv and every package checked by its sum; it fails, saying why, on
# anything that does not check out.
unpack_index() { # <index tarball> <dir>
	local index=$1 dir=$2 path sum url
	mkdir -p "$dir"
	tar -C "$dir" -xzf "$index" apt || return 1
	[ -s "$dir/apt/pool.tsv" ] || {
		echo "build.sh: $index has no apt/pool.tsv" >&2
		return 1
	}
	while IFS=$'\t' read -r path sum url; do
		case "$path" in
		pool/*.deb) ;;
		*) echo "build.sh: pool.tsv names $path, outside the pool" >&2; return 1 ;;
		esac
		case "$path" in *..*) echo "build.sh: pool.tsv names $path" >&2; return 1 ;; esac
		if [ "$url" != included ]; then
			mkdir -p "$dir/apt/$(dirname "$path")"
			curl -fsSL --retry 3 -o "$dir/apt/$path" "$url" || {
				echo "build.sh: $path not fetched from $url" >&2
				return 1
			}
		fi
		[ "$(sha256sum "$dir/apt/$path" | cut -d' ' -f1)" = "$sum" ] || {
			echo "build.sh: $path is not the package the signed index names" >&2
			return 1
		}
	done <"$dir/apt/pool.tsv"
	rm "$dir/apt/pool.tsv"
}

# release_index downloads the index a release carries into <dir>, and
# prints its path; it prints nothing where the release has none.
release_index() { # <tag> <dir>
	if gh release view "$1" --repo "$repo" --json assets --jq '.assets[].name' 2>/dev/null | grep -qx massimo-apt-index.tar.gz; then
		mkdir -p "$2"
		gh release download "$1" --repo "$repo" --pattern massimo-apt-index.tar.gz --dir "$2" &&
			printf '%s\n' "$2/massimo-apt-index.tar.gz"
	fi
}

# 1. The apt repository: the numbered suites, from --apt's directory or a
# version tag's index, then each rolling suite from its own build's index,
# added beside them. The packages they share are the same files.
if [ -n "$apt_dir" ]; then
	cp -R "$apt_dir" "$site/apt"
elif [ -n "$apt_index" ]; then
	if [ "$apt_index" = newest ]; then
		apt_index=""
		for tag in $(gh release list --repo "$repo" --limit 100 --exclude-drafts --json tagName --jq '.[].tagName | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+(-[a-z][a-z0-9]*\\.[0-9]+)?$"))'); do
			apt_index=$(release_index "$tag" "$work/numbered-index")
			if [ -n "$apt_index" ]; then
				echo "build.sh: the apt index of $tag"
				break
			fi
		done
		if [ -z "$apt_index" ]; then
			echo "build.sh: no release carries massimo-apt-index.tar.gz: no site without the apt repository"
			exit 3
		fi
	fi
	unpack_index "$apt_index" "$work/numbered" || exit 1
	mv "$work/numbered/apt" "$site/apt"
fi
if [ "$no_apt" = 0 ] && [ "$no_rolling" = 0 ]; then
	if [ ${#rolling_indexes[@]} -eq 0 ]; then
		for f in devel nightly; do
			rolling_indexes+=("$f=$(release_index "release/$f" "$work/rolling-index-$f" || true)")
		done
	fi
	for spec in "${rolling_indexes[@]}"; do
		f=${spec%%=*} index=${spec#*=}
		case "$f" in devel | nightly) ;; *) echo "build.sh: --rolling-index $spec: devel or nightly" >&2; exit 2 ;; esac
		if [ -z "$index" ] || [ ! -s "$index" ]; then
			echo "::warning::build.sh: no apt index of release/$f: the site has no suite $f"
			continue
		fi
		if ! unpack_index "$index" "$work/rolling-$f" || [ "$(cd "$work/rolling-$f/apt/dists" 2>/dev/null && echo *)" != "$f" ]; then
			echo "::warning::build.sh: the apt index of release/$f does not check out: the site has no suite $f"
			continue
		fi
		# Every file the site lacks: the suite, its snippet and its pool;
		# the keys, the keyring package and the numbered releases' packages
		# it shares with the numbered index stay as they are.
		(cd "$work/rolling-$f/apt" && find . -type f) | while read -r path; do
			if [ ! -e "$site/apt/$path" ]; then
				mkdir -p "$(dirname "$site/apt/$path")"
				cp "$work/rolling-$f/apt/$path" "$site/apt/$path"
			fi
		done
		echo "build.sh: the rolling suite $f"
	done
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
	version=$(git -C "$root" describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' "$commit" 2>/dev/null || git -C "$root" rev-parse --short "$commit")
	date=$(git -C "$root" show -s --format=%cs "$commit")
	pin=$(git -C "$root" rev-parse "$commit:GUIDELINES/shared")
	tree=$work/tree-$ch
	mkdir -p "$tree"
	git -C "$root" archive "$commit" | tar -C "$tree" -xf -
	args=(--tree "$tree" --out "$site" --channel "$ch" --commit "$commit" --version "$version"
		--date "$date" --pin "$pin" --repo "$repo" --md-html "$md_html" --suites "$suites")
	[ "$ch" = "$strict" ] && args+=(--strict)
	python3 "$root/packaging/site/site.py" docs "${args[@]}"
	rm -rf "${tree:?}"
done
[ "$no_docs" = 1 ] || python3 "$root/packaging/site/site.py" index --out "$site"

# 4. Every link of every page, where the docs are there for the home's to
# point at, then the site where it was asked for.
[ "$no_docs" = 1 ] || python3 "$root/packaging/site/site.py" check --out "$site"
mkdir -p "$(dirname "$out")"
mv "$site" "$out"
echo "build.sh: $out holds the site (suites: $suites)"

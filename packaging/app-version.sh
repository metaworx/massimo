#!/usr/bin/env bash
# The Android app's version from git's version tags (AP MobileApp, Block 4,
# D12-A and D21-A), the one place its formula lives: app/version.json holds
# a release's, which make release writes before the cut and changelog.sh
# cut commits in the release commit; build.yml asks for a nightly build's;
# the job Android app checks the file.
#
# The versionName is the tag without its v, or git describe's for a build
# between tags. The versionCode is MMmmPPbbbb: the major (at most 20, since
# Play's largest code is 2100000000), the minor and the patch (at most 99
# each), and the commits since the version's first tag (at most 9999). The
# labels have no digits of their own: v0.16.0-alpha.1 is 16000000, a build
# 227 commits after it 16000227, a v0.16.0 cut there 16000229, and
# v0.17.0-alpha.1 is 17000000 again. A shallow clone is refused: the count
# needs the whole history and its tags.
#
#   app-version.sh describe [<commit>]   "<versionName> <versionCode>" of a
#                                        build of the commit (HEAD)
#   app-version.sh release <tag> [<n>]   the versionCode of the release <tag>,
#                                        its commit <n> commits after HEAD (1)
#   app-version.sh write <tag> [<n>]     app/version.json for that release
#   app-version.sh check [<tag>]         app/version.json names a version
#                                        whose tag exists, with that tag's
#                                        versionCode; with <tag>, that one
#
# Written for bash 3.2 too, macOS's.
set -euo pipefail

TAG_RE='^v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-z][a-z0-9]*\.[0-9]+)?$'

usage() {
	echo "usage: app-version.sh describe [<commit>] | release <tag> [<n>] | write <tag> [<n>] | check [<tag>]" >&2
	exit 2
}

die() {
	echo "app-version: $*" >&2
	exit 1
}

is_tag() { [[ "$1" =~ $TAG_RE ]]; }

# code <X.Y.Z> <count>: the versionCode, or a refusal where a part is too big.
code() {
	local major minor patch
	IFS=. read -r major minor patch <<<"$1"
	major=$((10#$major)) minor=$((10#$minor)) patch=$((10#$patch))
	[ "$major" -le 20 ] || die "$1: the major is at most 20, as Play's largest versionCode allows"
	[ "$minor" -le 99 ] || die "$1: the minor is at most 99"
	[ "$patch" -le 99 ] || die "$1: the patch is at most 99"
	[ "$2" -le 9999 ] || die "$1: $2 commits since the version's first tag; at most 9999"
	echo $((major * 100000000 + minor * 1000000 + patch * 10000 + $2))
}

# first <X.Y.Z> <commit>: the version's oldest tag in the commit's history,
# or nothing where it has none.
first() {
	local re="^v${1//./\\.}(-[a-z][a-z0-9]*\\.[0-9]+)?$" t n best="" least=""
	while IFS= read -r t; do
		[[ "$t" =~ $re ]] || continue
		n="$(git rev-list --count "refs/tags/$t")"
		if [ -z "$least" ] || [ "$n" -lt "$least" ]; then
			least="$n" best="$t"
		fi
	done < <(git tag -l --merged "$2" "v$1" "v$1-*")
	printf '%s' "$best"
}

# since <X.Y.Z> <commit>: the commits since the version's first tag, which
# the commit's history must hold.
since() {
	local f
	f="$(first "$1" "$2")"
	[ -n "$f" ] || die "no tag of $1 in the history of $2"
	git rev-list --count "refs/tags/$f..$2"
}

xyz() { # <tag>: its X.Y.Z
	[[ "$1" =~ $TAG_RE ]]
	printf '%s.%s.%s' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
}

describe() { # <commit>
	local tag d v n c
	tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' "$1" 2>/dev/null)" ||
		die "no version tag in the history of $1"
	is_tag "$tag" || die "$tag, the newest tag of $1, is not a version tag (vX.Y.Z or vX.Y.Z-<label>.N)"
	d="$(git describe --tags --match "$tag" "$1")"
	v="$(xyz "$tag")"
	n="$(since "$v" "$1")"
	c="$(code "$v" "$n")"
	echo "${d#v} $c"
}

release() { # <tag> <n>
	local v f n
	is_tag "$1" || die "$1 is not a version tag (vX.Y.Z or vX.Y.Z-<label>.N)"
	case "$2" in '' | *[!0-9]*) usage ;; esac
	! git rev-parse -q --verify "refs/tags/$1" >/dev/null || die "$1 exists already: check reads its file"
	v="$(xyz "$1")"
	f="$(first "$v" HEAD)"
	n=0 # without a tag of the version, the release's becomes its first
	if [ -n "$f" ]; then
		n="$(git rev-list --count "refs/tags/$f..HEAD")"
		n=$((n + $2))
	fi
	code "$v" "$n"
}

# The file's two values, as write lays them out and changelog.sh cut keeps
# them; any other file is refused rather than guessed at.
field() { # <file> <name> <pattern of the value>
	local got
	got="$(sed -nE "s/^[[:space:]]*\"$2\"[[:space:]]*:[[:space:]]*($3)[[:space:]]*,?[[:space:]]*$/\1/p" "$1")"
	if [ -z "$got" ] || [ "$(printf '%s\n' "$got" | wc -l)" -ne 1 ]; then
		die "$1 does not hold one \"$2\" as app-version.sh write lays it out"
	fi
	printf '%s' "${got//\"/}"
}

check() { # [<tag>]
	local f="app/version.json" version vcode tag v n want
	[ -f "$f" ] || die "$f is missing"
	version="$(field "$f" version '"[^"]*"')"
	vcode="$(field "$f" versionCode '[0-9]+')"
	tag="v$version"
	is_tag "$tag" || die "$f names $version, not a released version"
	[ -z "${1:-}" ] || [ "$1" = "$tag" ] || die "$f names $version, not $1"
	git rev-parse -q --verify "refs/tags/$tag" >/dev/null || die "$f names $version, and there is no tag $tag"
	v="$(xyz "$tag")"
	n="$(since "$v" "refs/tags/$tag")"
	want="$(code "$v" "$n")"
	[ "$vcode" = "$want" ] || die "$f gives $version the versionCode $vcode; its tag's is $want"
	echo "app-version: $f is $version, $vcode"
}

[ "$#" -ge 1 ] || usage
cd "$(git rev-parse --show-toplevel)"
[ "$(git rev-parse --is-shallow-repository)" = false ] ||
	die "a shallow clone: the versionCode counts commits since a tag (fetch-depth: 0)"
cmd="$1"
shift
case "$cmd:$#" in
describe:0) describe HEAD ;;
describe:1) describe "$1" ;;
release:1 | release:2) release "$1" "${2:-1}" ;;
write:1 | write:2)
	vcode="$(release "$1" "${2:-1}")"
	printf '{\n  "version": "%s",\n  "versionCode": %s\n}\n' "${1#v}" "$vcode" >app/version.json.tmp
	mv app/version.json.tmp app/version.json
	echo "app-version: app/version.json is ${1#v}, $vcode"
	;;
check:0 | check:1) check "${1:-}" ;;
*) usage ;;
esac

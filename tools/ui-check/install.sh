#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# install.sh - TypeScript and Biome for make ui-check, installed once per
# lock file for every checkout of the project (AP CleanRuns, D1-A)
#
# Prints the directory that holds tsc and biome, a node_modules/.bin. An
# install is keyed by this directory's package-lock.json and the
# platform, <the lock's SHA-256, its first 16 digits>-<Node's
# platform-arch>, with -musl for a Linux without glibc, so that a Mac,
# WSL and an Alpine container never share Biome's binary, and kept in
# UI_CHECK_CACHE, else in the main checkout's GUIDELINES/temp/ui-check/:
# the checkout of the common directory git names, which a worktree and a
# copy exported into one name alike. A key not installed yet is installed
# by npm ci, which runs no package's scripts, into a directory of its own
# beside it, .new.XXXXXX, then renamed into place; where another run
# renamed its own first, this run's copy is removed, so that no run uses
# a half install. Each use touches the key's directory, and the next run
# removes an install unused for 30 days and a .new.* a stopped run left a
# day ago, each known by its package.json, so that nothing else in
# UI_CHECK_CACHE is touched; that housekeeping never fails the check.
# Without git, or where that directory cannot be written, npm ci installs
# into tools/ui-check/node_modules, as it did before.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
lock="$here/package-lock.json"

say() { echo "ui-check: $*" >&2; }

# npm_ci dir: the locked TypeScript and Biome into dir/node_modules; npm's
# own lines go to stderr, so that stdout holds the directory alone.
npm_ci() {
	npm ci --prefix "$1" --ignore-scripts --no-audit --no-fund --loglevel=error --no-update-notifier >&2
}

# installed dir: dir holds both programs.
installed() {
	[ -x "$1/node_modules/.bin/tsc" ] && [ -x "$1/node_modules/.bin/biome" ]
}

# here_install why: the install into this checkout, as before.
here_install() {
	say "TypeScript and Biome into $here/node_modules ($1)"
	npm_ci "$here"
	echo "$here/node_modules/.bin"
	exit 0
}

# prune age pattern: the entries of base whose names match pattern, older
# than age minutes, that hold this directory's package.json: an install
# or a copy of one; -mmin, since BSD's find rounds -mtime's days up.
prune() {
	find "$base" -mindepth 1 -maxdepth 1 -type d -name "$2" ! -name "$key" -mmin +"$1" \
		-exec sh -c 'grep -qs "\"name\": \"massimo-ui-check\"" "$1/package.json" && rm -rf "$1"' sh {} \; \
		2>/dev/null || :
}

if command -v sha256sum >/dev/null 2>&1; then
	sum=$(sha256sum <"$lock")
else
	sum=$(shasum -a 256 <"$lock")
fi
platform=$(node -p 'process.platform + "-" + process.arch + (process.platform === "linux" && !process.report.getReport().header.glibcVersionRuntime ? "-musl" : "")')
key="$(printf '%s\n' "$sum" | cut -c1-16)-$platform"

base=${UI_CHECK_CACHE:-}
if [ -z "$base" ] && common=$(git -C "$here" rev-parse --path-format=absolute --git-common-dir 2>/dev/null); then
	main=$(dirname "$common")
	if [ -d "$main/GUIDELINES" ]; then base="$main/GUIDELINES/temp/ui-check"; fi
fi
[ -n "$base" ] || here_install "no shared directory"
if ! mkdir -p "$base" 2>/dev/null || [ ! -w "$base" ]; then
	here_install "$base is not writable"
fi

dir="$base/$key"
if ! installed "$dir"; then
	new=$(mktemp -d "$base/.new.XXXXXX" 2>/dev/null) || here_install "nothing can be made in $base"
	cp "$here/package.json" "$lock" "$new/"
	say "installing TypeScript and Biome into $dir"
	npm_ci "$new" || { rm -rf "$new"; exit 1; }
	# rename(2) refuses a directory that is there and not empty: another
	# run's, renamed first, which this run then uses.
	if ! node -e 'require("fs").renameSync(process.argv[1], process.argv[2])' "$new" "$dir" 2>/dev/null; then
		rm -rf "$new"
		installed "$dir" || { say "$dir holds no install and cannot be replaced; remove it"; exit 1; }
	fi
fi
touch -c "$dir" 2>/dev/null || :
say "TypeScript and Biome of $dir"
prune 43200 '[0-9a-f]*-*'
prune 1440 '.new.*'
echo "$dir/node_modules/.bin"

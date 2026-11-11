#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# run.sh - each fuzz target of the packages given, for FUZZTIME
#
# make fuzz (GUIDELINES/project/CI.md, section 3): the Fuzz* targets of
# each package one after the other, since go test -fuzz takes one at a
# time, each for FUZZTIME (10 s); it stops at the first that fails. A
# crash leaves its input under the package's testdata/fuzz/, which
# go test then replays as a seed.
#
# Go 1.26's fuzzing coordinator now and then reports its own -fuzztime
# deadline as the target's failure: "context deadline exceeded" and
# nothing else, no input written (golang/go#75804, fixed in Go 1.27). On
# a toolchain without that fix, such a run passes, with a warning: the
# target ran its time, and a fixed toolchain returns no error there.
#
# usage: FUZZTIME=10s tools/fuzz/run.sh <package>...

set -euo pipefail

fuzztime=${FUZZTIME:-10s}
export MASSIMO_ENV_FILE=/dev/null # the checkout's .env is real hardware's

# harness_fixed: the toolchain's coordinator drops its own deadline, the
# condition Go 1.27 gave CoordinateFuzzing's stop (golang/go#75804).
harness_fixed() {
	grep -qF 'err == ctx.Err() || err == fuzzCtx.Err()' \
		"$(go env GOROOT)/src/internal/fuzz/fuzz.go" 2>/dev/null
}

# deadline_only <log> <target>: the target's failure says only that the
# deadline passed. A crash adds its message and "Failing input written
# to", a hung worker its own line, so neither matches.
deadline_only() {
	awk -v t="$2" '
		index($0, "--- FAIL: " t " (") == 1 { f = 1; next }
		f && /^    / { msgs = msgs $0 "\n"; next }
		{ f = 0 }
		END { exit !(msgs == "    context deadline exceeded\n") }
	' "$1"
}

log=$(mktemp)
trap 'rm -f "$log"' EXIT

for pkg in "$@"; do
	list=$(go test -list '^Fuzz' "$pkg")
	for t in $(grep '^Fuzz' <<<"$list" || true); do
		echo "fuzz $pkg $t ($fuzztime)"
		if go test -run '^$' -fuzz "^$t\$" -fuzztime "$fuzztime" "$pkg" 2>&1 | tee "$log"; then
			continue
		fi
		if ! harness_fixed && deadline_only "$log" "$t"; then
			msg="$t ran its $fuzztime; its failure is Go's own deadline (golang/go#75804), not the target's"
			if [[ ${GITHUB_ACTIONS:-} == true ]]; then
				echo "::warning title=make fuzz::$msg"
			else
				echo "fuzz: $msg"
			fi
			continue
		fi
		exit 1
	done
done

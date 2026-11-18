#!/usr/bin/env bash
# The description of the Tested status that ci.yml's All tests passed
# writes once it has passed (GUIDELINES/project/CI.md, section 2): the
# fingerprint of the code that ran, then whether the UI tests, LAN, Fuzz,
# the Android app, macOS and Windows passed for it, in this run or in the
# one it carries on (What to test's *_done), as ui=, lan=, fuzz=, app=,
# mac= and win=, which What to test and the nightly build's What to ship
# read:
#   - true: it passed, here or before;
#   - unneeded (lan, app, mac, win): What to test answered false for it,
#     and it did not pass; a push carries it from its pull request's
#     record;
#   - false: neither, as for a job that failed or did not run. A macOS or
#     Windows job that no runner took on a pull request or a push to main
#     counts as not run (AP TestSpeed, D5-A, D16-A): the merge's push runs
#     it, and the nightly build runs it before it ships the code.
# The UI tests passed where their matrix job did (screenshots, both
# widths); macOS and Windows where platforms.yml's output says so, as the
# call's result is that of both jobs together.
#
# Needs NEEDS, the job's needs as JSON, and FP, the fingerprint. Prints
# the description, with a warning annotation where it is longer than the
# 140 characters GitHub keeps: the fingerprint's 68 and the words' 72 at
# their longest.
set -euo pipefail

needs="${NEEDS:?NEEDS is not set}"
fp="${FP:?FP is not set}"
desc=$(jq -r --arg fp "$fp" '
	def word($done; $passed; $answer):
		if $done == "true" or $passed then "true" elif $answer == "false" then "unneeded" else "false" end;
	(.changes.outputs // {}) as $a
	| (.platforms.outputs // {}) as $p
	| "\($fp)"
		+ " ui=\(word($a.ui_done; .screenshots.result == "success"; null))"
		+ " lan=\(word($a.lan_done; .lan.result == "success"; $a.lan))"
		+ " fuzz=\(word($a.fuzz_done; .fuzz.result == "success"; null))"
		+ " app=\(word($a.app_done; .android.result == "success"; $a.app))"
		+ " mac=\(word($a.macos_done; $p.macos == "true"; $a.macos))"
		+ " win=\(word($a.windows_done; $p.windows == "true"; $a.windows))"' <<<"$needs")
[ "${#desc}" -le 140 ] || echo "::warning::the Tested record is ${#desc} characters, GitHub takes 140: ${desc}" >&2
echo "$desc"

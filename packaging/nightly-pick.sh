#!/usr/bin/env bash
# What the nightly build ships, and whether nightly.yml runs its macOS and
# Windows jobs first: nightly.yml's What to ship (GUIDELINES/project/CI.md,
# sections 2 and 6).
#
# The commit, the output sha: none, and no site (site=false), where
# main's head (GITHUB_SHA) is a release commit, its version tag at it:
# that tag's run builds it and deploys the site at the same commit, and
# Pages keeps the first deploy of a commit (D55-A). Otherwise the newest
# of main's first parents, at most NIGHTLY_WALK (100), down to the commit
# release/nightly carries, whose run of ci.yml (its push, or its tag's)
# had the job All tests passed succeed, so that a red last commit does
# not cost a night whose second-last was green (the owner's rule); none
# where no such commit is newer than the nightly build's.
#
# The macOS and Windows jobs, the outputs macos and windows (AP
# TestSpeed, D15-B): a push to main counts such a job that no runner took
# as not run, and its Tested record says mac=false or win=false (D16-A),
# so the nightly build runs the job before it ships the code. For each
# job the records of the commit and of its first parents, newest first,
# decide:
#   - true, or a record without the word, written before AP TestSpeed,
#     when every run ran both jobs: it passed for this code;
#   - false: it did not run for this code, and runs;
#   - unneeded: the commit's pull request did not touch the job's paths,
#     so its code for the job is its parent's: on to the parent;
#   - no record: a release commit whose tag's run passed passed, as a tag
#     runs both jobs and fails without them; one whose code is its
#     parent's (fingerprint.sh same: documents alone, which write no
#     record) goes on to the parent; any other commit's job runs;
#   - the commit release/nightly carries, or one before it: it passed,
#     since the nightly build shipped it under this rule, or when a push
#     to main still failed without both jobs;
#   - the walk's end (NIGHTLY_WALK commits, or the history's): it runs.
# PLATFORMS=both runs both whatever the records say (nightly.yml's input
# platforms). The output test is the commit the jobs check out: the one
# to ship, or main's head where there is none and PLATFORMS=both.
#
# Needs a full history with the version tags, the guidelines
# (fingerprint.sh, or FPSH), GH_TOKEN with actions: read and statuses:
# read, GITHUB_REPOSITORY, GITHUB_SHA and GITHUB_OUTPUT; GITHUB_STEP_SUMMARY
# where GitHub sets it. NIGHTLY_MAIN (origin/main) is the branch walked.
set -euo pipefail

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
head="${GITHUB_SHA:?GITHUB_SHA is not set}"
output="${GITHUB_OUTPUT:?GITHUB_OUTPUT is not set}"
summary="${GITHUB_STEP_SUMMARY:-/dev/null}"
walk="${NIGHTLY_WALK:-100}"
main="${NIGHTLY_MAIN:-origin/main}"
fpsh="${FPSH:-GUIDELINES/shared/tools/fingerprint.sh}"
force="${PLATFORMS:-}"
case "$walk" in '' | *[!0-9]*) walk=100 ;; esac

# Each reason, for the log and the summary.
why=()
say() { echo "$1"; why+=("$1"); }

# version_tag <commit>: the first version tag at the commit, or nothing.
version_tag() {
	local at
	at="$(git tag --points-at "$1" --list 'v[0-9]*.[0-9]*.[0-9]*')"
	echo "${at%%$'\n'*}"
}

# passed <commit>: whether a run of ci.yml for the commit (its push to
# main, or a tag's) had its job "All tests passed" succeed.
passed() {
	local id ok
	for id in $(gh api "repos/${repo}/actions/workflows/ci.yml/runs?head_sha=$1&event=push&per_page=20" --jq '.workflow_runs[].id' || true); do
		ok="$(gh api "repos/${repo}/actions/runs/${id}/jobs?per_page=100" \
			--jq '.jobs[] | select(.name == "All tests passed" and .conclusion == "success") | .id' || true)"
		[ -z "$ok" ] || return 0
	done
	return 1
}

# record <commit>: the description of its Tested status, green and written
# by ci.yml's runs; nothing where there is none.
record() {
	gh api "repos/${repo}/commits/${1}/statuses?per_page=100" \
		--jq '[.[] | select(.context == "Tested")][0] | select(.state == "success" and .creator.login == "github-actions[bot]") | .description' 2>/dev/null || true
}

# runs <word> <job>: whether the job runs before the nightly build ships
# $pick, by the records (above); says why.
runs() {
	local word="$1" job="$2" c="$pick" n=0 desc w state tag parent same
	while :; do
		if [ -n "$cur" ] && git merge-base --is-ancestor "$c" "$cur"; then
			say "${job}: the nightly build carries ${c:0:12}'s code for it: not run."
			return 1
		fi
		desc="$(record "$c")"
		if [ -n "$desc" ]; then
			state=""
			for w in $desc; do
				case "$w" in "${word}="*) state="${w#*=}" ;; esac
			done
			case "$state" in
			true)
				say "${job} passed for ${c:0:12}'s code (${word}=true): not run."
				return 1
				;;
			"")
				say "${job}: ${c:0:12}'s record is older than ${word}=, from when every run ran it: not run."
				return 1
				;;
			unneeded) ;;
			*)
				say "${job} did not run for ${c:0:12}'s code (${word}=${state}): it runs."
				return 0
				;;
			esac
		else
			tag="$(version_tag "$c")"
			if [ -n "$tag" ] && passed "$c"; then
				say "${job}: ${c:0:12} is ${tag}, whose run passed, as a tag's does with both jobs alone: not run."
				return 1
			fi
			parent="$(git rev-parse --verify --quiet "${c}^1^{commit}" || true)"
			same=""
			[ -z "$parent" ] || same="$("$fpsh" -q same "$parent" "$c" 2>/dev/null || true)"
			if [ "$same" != true ]; then
				say "${job}: ${c:0:12} has no record and code of its own: it runs."
				return 0
			fi
		fi
		n=$((n + 1))
		c="$(git rev-parse --verify --quiet "${c}^1^{commit}" || true)"
		if [ -z "$c" ] || [ "$n" -ge "$walk" ]; then
			say "${job}: no record within ${n} commit(s) says it passed: it runs."
			return 0
		fi
	done
}

sha="" test="" site=true macos=false windows=false cur="" pick=""
at="$(version_tag "$head")"
if [ -n "$at" ]; then
	say "main's head is the release commit of ${at}: its own run builds and deploys it; no nightly build, no site."
	site=false
else
	git fetch --quiet --force origin "+refs/tags/release/nightly:refs/tags/release/nightly" 2>/dev/null || true
	cur="$(git rev-parse --verify --quiet "refs/tags/release/nightly^{commit}" || true)"
	commits="$(git rev-list --first-parent --max-count="$walk" "$main")"
	for c in $commits; do
		if [ -n "$cur" ] && git merge-base --is-ancestor "$c" "$cur"; then
			break
		fi
		if passed "$c"; then
			pick="$c"
			break
		fi
	done
	if [ -n "$pick" ]; then
		say "the newest commit of main whose suite passed, after ${cur:0:12}: ${pick:0:12}."
		sha="$pick" test="$pick"
		if [ "$force" = both ]; then
			say "PLATFORMS=both: both jobs run, whatever the records say."
			macos=true windows=true
		else
			if runs mac "macOS LaunchDaemon"; then macos=true; fi
			if runs win "Windows service"; then windows=true; fi
		fi
	else
		say "no commit of main after ${cur:-the start} has passed its suite: no nightly build."
	fi
fi
if [ -z "$pick" ] && [ "$force" = both ]; then
	say "PLATFORMS=both and nothing to ship: both jobs run on main's head, ${head:0:12}."
	test="$head" macos=true windows=true
fi

echo "sha=${sha} test=${test} site=${site} macos=${macos} windows=${windows}"
{
	echo "### What to ship"
	echo
	if [ "${#why[@]}" -gt 0 ]; then
		printf -- '- %s\n' "${why[@]}"
		echo
	fi
	echo "| Answer | Value |"
	echo "|--------|-------|"
	echo "| sha | \`${sha:-none}\` |"
	echo "| test | \`${test:-none}\` |"
	echo "| site | \`${site}\` |"
	echo "| macos | \`${macos}\` |"
	echo "| windows | \`${windows}\` |"
	echo
} >>"$summary"
{
	echo "sha=${sha}"
	echo "test=${test}"
	echo "site=${site}"
	echo "macos=${macos}"
	echo "windows=${windows}"
} >>"$output"

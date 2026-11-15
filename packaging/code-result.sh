#!/usr/bin/env bash
# The verdict on the code of a run that tests none (ci.yml's Code result,
# GUIDELINES/project/CI.md section 2): where What to test answers
# code=false, the change is documents alone and the code is that of the
# commit before it, so the last run that tested that code holds the verdict.
# Nothing is run again: a known red is carried, never retried.
#
# It walks main's first parents from HEAD^1 (a push's commits before it, a
# pull request's base) while a commit holds HEAD's code: no path differs but
# those [ci] untested and [ci] site name in GUIDELINES/config.ini, as What to
# test's site check compares. For each such commit, newest first, it reads
# ci.yml's push runs on it, a tag's among them (this run left out), and of
# each run the latest attempt's code jobs: all but What to test, Guidelines,
# Site, Code result, All tests passed, Deploy site and the called Build and
# Release.
#   - a run still going: it waits for it (CODE_DEADLINE seconds, 3600);
#   - a cancelled run (main's group keeps one run waiting, so a push
#     replaces the run before it that has not started), a run in which Code
#     result ran (a verdict carried, so the walk goes on to the run that made
#     it, whose latest attempt holds a re-run's result), and a run whose code
#     jobs were all skipped: no verdict;
#   - a code job that failed, timed out or was cancelled: red;
#   - else green: the code jobs passed.
# Where no run on a commit gives a verdict and none carried one, its record
# does, read where What to test reads it: the Tested status on the commit,
# then on a merge's second parent, the pull request's head. Green, written
# by this workflow and opening with HEAD's fingerprint: green. Else the walk
# goes on.
# The walk ends red at a commit with other code, after CODE_WALK commits
# (100) or where the history ends: no run tested this code.
#
# Red exits 1 on a push and 0 with a warning on a pull request, which adds
# no code of its own; green exits 0. The summary names the run either way.
# Needs a full history, the guidelines (fingerprint.sh), GH_TOKEN with actions: read and statuses: read, and
# GITHUB_REPOSITORY; GITHUB_RUN_ID, GITHUB_EVENT_NAME and
# GITHUB_STEP_SUMMARY where GitHub sets them. CODE_HEAD (HEAD) names the
# commit to judge, for a test by hand.
set -euo pipefail

head="$(git rev-parse --verify "${CODE_HEAD:-HEAD}^{commit}")"
deadline="${CODE_DEADLINE:-3600}"
walk="${CODE_WALK:-100}"
event="${GITHUB_EVENT_NAME:-push}"
repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
self="${GITHUB_RUN_ID:-}"
cfg=GUIDELINES/config.ini
fpsh=GUIDELINES/shared/tools/fingerprint.sh
end=$((SECONDS + deadline))

# The verdict, said once: on stdout, in the summary, and as the exit code.
verdict() { # <green|red> <message>
	echo "$2"
	if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
		{
			if [ "$1" = green ]; then echo "### The code passed"; else echo "### The code did not pass"; fi
			echo
			echo "$2"
		} >>"$GITHUB_STEP_SUMMARY"
	fi
	[ "$1" = green ] && exit 0
	if [ "$event" = pull_request ]; then
		echo "::warning::${2} The pull request changes documents alone, so it fails nothing here; main stays red until code fixes it."
		exit 0
	fi
	echo "::error::${2}"
	exit 1
}

# The paths that are not code, as exclusions; a value with pathspec magic
# cannot be negated by a prefix, so the code cannot be compared.
skip=()
while IFS= read -r p; do
	[ -n "$p" ] || continue
	case "$p" in :*) verdict red "${cfg} has a [ci] value with pathspec magic (${p}): the code cannot be compared." ;; esac
	skip+=(":!${p}")
done < <(git config -f "$cfg" --get-all ci.untested || true; git config -f "$cfg" --get-all ci.site || true)

# The latest attempt's jobs of a run, one "<conclusion>\t<name>" a line.
jobs_of() { # <run id>
	gh api --paginate "repos/${repo}/actions/runs/${1}/jobs?filter=latest&per_page=100" \
		--jq '.jobs[] | "\(.conclusion // .status)\t\(.name)"'
}

# Whether a job is one of the code's: every job but the gate, the checks of
# documents, this one, the verdict and what follows it.
is_code_job() { # <name>
	case "$1" in
	"What to test" | Guidelines | Site | "Code result" | "All tests passed" | "Deploy site") return 1 ;;
	Build | "Build / "* | Release | "Release / "*) return 1 ;;
	esac
	return 0
}

# The run that wrote a commit's Tested status, where it is green, written by
# this workflow and for HEAD's code: the fingerprint opens its description.
tested_by() { # <sha>
	local line
	[ -n "$fp" ] || return 1
	line="$(gh api "repos/${repo}/commits/${1}/statuses?per_page=100" \
		--jq '[.[] | select(.context == "Tested")][0] | select(.state == "success" and .creator.login == "github-actions[bot]") | "\(.description) \(.target_url)"' 2>/dev/null || true)"
	[ "${line%% *}" = "$fp" ] || return 1
	echo "${line##* }"
}

# HEAD's code, as What to test fingerprints it; none, and no record counts.
fp="$("$fpsh" -q fingerprint "$head" 2>/dev/null)" || fp=""
[ -n "$fp" ] || echo "code-result: no fingerprint of ${head:0:12}; the Tested records are not read"

commit="$(git rev-parse --verify --quiet "${head}^1^{commit}")" ||
	verdict red "${head:0:12} has no parent: no run tested its code."
n=0
while :; do
	git diff --quiet "$commit" "$head" -- . "${skip[@]}" ||
		verdict red "No run tested this code: the walk reached ${commit:0:12}, which holds other code, without a verdict."
	carried_any=false
	runs="$(gh api "repos/${repo}/actions/workflows/ci.yml/runs?head_sha=${commit}&event=push&per_page=100" \
		--jq '.workflow_runs[] | "\(.id)\t\(.html_url)"')" ||
		verdict red "The runs on ${commit:0:12} could not be read."
	while IFS=$'\t' read -r id url; do
		{ [ -n "$id" ] && [ "$id" != "$self" ]; } || continue
		while :; do
			info="$(gh api "repos/${repo}/actions/runs/${id}" --jq '"\(.status) \(.conclusion // "-")"')" ||
				info="unreadable -"
			status="${info% *}" conclusion="${info#* }"
			[ "$status" != completed ] || break
			[ "$SECONDS" -lt "$end" ] ||
				verdict red "Run ${url} on ${commit:0:12} did not finish within ${deadline} s."
			echo "code-result: run ${id} on ${commit:0:12} is ${status}; waiting"
			sleep 20
		done
		[ "$conclusion" != cancelled ] || { echo "code-result: run ${id} was cancelled; no verdict"; continue; }
		jobs="$(jobs_of "$id")" || verdict red "The jobs of run ${url} could not be read."
		carried=false passed=false bad=""
		while IFS=$'\t' read -r c name; do
			[ -n "$name" ] || continue
			if [ "$name" = "Code result" ]; then
				[ "$c" = skipped ] || carried=true
				continue
			fi
			is_code_job "$name" || continue
			case "$c" in
			success) passed=true ;;
			skipped | neutral) ;;
			*) bad="${bad}${bad:+, }${name} (${c})" ;;
			esac
		done <<<"$jobs"
		if [ "$carried" = true ]; then
			echo "code-result: run ${id} carried a verdict; on to the run that made it"
			carried_any=true
			continue
		fi
		[ -z "$bad" ] || verdict red "The code failed in run ${url} on ${commit:0:12}: ${bad}."
		[ "$passed" = true ] && verdict green "The code passed in run ${url} on ${commit:0:12}."
		echo "code-result: run ${id} tested no code; no verdict"
	done <<<"$runs"
	if [ "$carried_any" = false ]; then
		for sha in "$commit" $(git rev-parse --verify --quiet "${commit}^2^{commit}" || true); do
			by="$(tested_by "$sha")" &&
				verdict green "The code passed earlier: ${commit:0:12}'s runs gave no verdict, and the Tested status on ${sha:0:12}, written by ${by}, holds this code's fingerprint."
		done
		echo "code-result: no Tested record on ${commit:0:12} for this code"
	fi
	n=$((n + 1))
	[ "$n" -lt "$walk" ] || verdict red "No run tested this code within ${walk} commits."
	commit="$(git rev-parse --verify --quiet "${commit}^1^{commit}")" ||
		verdict red "No run tested this code: the history ends."
done

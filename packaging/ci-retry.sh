#!/usr/bin/env bash
# Retry (retry.yml): a macOS or Windows job that no runner took, run again
# in a tag's run of ci.yml or in a nightly run, at most RETRIES (2) times,
# RETRY_WAIT seconds (1200, about 20 minutes) after the attempt it missed
# ended (AP TestSpeed, D16-A, D15-B; GUIDELINES/project/CI.md, section 2).
# GitHub's macOS runners are few, and a job that waits for one about a
# quarter of an hour ends cancelled with no runner and no step; a pull
# request and a push to main count it as not run, a tag's release and the
# nightly build need it.
#
# It re-runs only a run of this repository, its head this repository's
# too, that is one of the two: ci.yml's push of a version tag
# (vX.Y.Z or vX.Y.Z-<label>), the tag at the run's commit and no branch of
# its name, its verdict job All tests passed; or nightly.yml's run on main
# from the schedule or by hand, its verdict job Platforms passed. It reads
# that from the run itself, which must match the event's HEAD_BRANCH and
# HEAD_SHA, at the start and again before each re-run, and refuses any
# other run with an error.
#
# It is started for RUN_ID's first attempt, once that has ended. Where
# every job of the attempt passed or was skipped, but for jobs of
# RETRY_JOBS that no runner took (cancelled with no runner and no step,
# as ci-report.sh tells them) and the verdict job, which failed for them,
# it waits RETRY_WAIT seconds and asks GitHub to re-run the run's failed
# jobs: those, and every job that depends on them, the release among them.
# It then waits for the new attempt's jobs of RETRY_JOBS to end: where a
# runner took each, it is done, whatever they gave, since the run's own
# verdict decides; where one was not taken again, it waits for the attempt
# to end and does the same once more. Nothing is run again where another
# job failed or was cancelled, where the verdict did not fail (as in a run
# someone cancelled), or where the run is at another attempt than the one
# expected (someone ran it again). After RETRIES re-runs without a runner
# it fails with an error: the tag's release fails, or the night is
# skipped. A re-run that leaves out a job it was to take fails with an
# error naming it, as GitHub's documentation names failed jobs alone, and
# a run that cannot be read before a re-run fails too.
#
# Needs GH_TOKEN with actions: write, GITHUB_REPOSITORY, RUN_ID,
# HEAD_BRANCH and HEAD_SHA (the event's), and RETRY_JOBS (a JSON array of
# job names, as the API gives them); RETRIES, RETRY_WAIT, RETRY_POLL (60 s
# between looks) and RETRY_DEADLINE (3600 s for each attempt to end, so
# that two of them and both waits end within the job's time limit) are
# optional; GITHUB_STEP_SUMMARY where GitHub sets it.
set -euo pipefail

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
run="${RUN_ID:?RUN_ID is not set}"
head_branch="${HEAD_BRANCH:?HEAD_BRANCH is not set}"
head_sha="${HEAD_SHA:?HEAD_SHA is not set}"
names="${RETRY_JOBS:?RETRY_JOBS is not set}"
retries="${RETRIES:-2}"
wait="${RETRY_WAIT:-1200}"
poll="${RETRY_POLL:-60}"
deadline="${RETRY_DEADLINE:-3600}"
summary="${GITHUB_STEP_SUMMARY:-/dev/null}"
for n in "$retries" "$wait" "$poll" "$deadline"; do
	case "$n" in '' | *[!0-9]*) echo "::error::RETRIES, RETRY_WAIT, RETRY_POLL and RETRY_DEADLINE are numbers" >&2; exit 2 ;; esac
done
jq -e 'type == "array" and all(.[]; type == "string")' <<<"$names" >/dev/null ||
	{ echo "::error::RETRY_JOBS is no JSON array of names: ${names}" >&2; exit 2; }

# What happened, in the log and the summary; an error ends the run.
note() { # <message>
	echo "$1"
	printf -- '- %s\n' "$1" >>"$summary"
}
fail() { # <message>
	echo "::error::$1"
	printf -- '- **Error:** %s\n' "$1" >>"$summary"
	exit 1
}

# read_run: the run's latest attempt, status and what it is, into r_*;
# false where the run cannot be read.
read_run() {
	local line
	line="$(gh api "repos/${repo}/actions/runs/${run}" --jq '[(.run_attempt // 1), .status, .event, .path,
		.repository.full_name, .head_repository.full_name, .head_branch, .head_sha] | map(tostring) | join("\t")')" || return 1
	IFS=$'\t' read -r r_attempt r_status r_event r_path r_repo r_head_repo r_branch r_sha <<<"$line" || return 1
	case "$r_attempt" in '' | *[!0-9]*) return 1 ;; esac
}

# read_run_or_fail: read_run, three tries RETRY_POLL apart, else an error.
read_run_or_fail() {
	local i
	for i in 1 2 3; do
		read_run && return 0
		[ "$i" = 3 ] || sleep "$poll"
	done
	fail "Run ${run} could not be read: nothing is run again."
}

# tag_commit <name>: the commit the tag names, its annotations peeled.
tag_commit() {
	local obj type sha i
	obj="$(gh api "repos/${repo}/git/ref/tags/$1" --jq '"\(.object.type) \(.object.sha)"')" || return 1
	type="${obj%% *}" sha="${obj#* }"
	for i in 1 2 3 4 5; do
		[ "$type" = tag ] || break
		obj="$(gh api "repos/${repo}/git/tags/${sha}" --jq '"\(.object.type) \(.object.sha)"')" || return 1
		type="${obj%% *}" sha="${obj#* }"
	done
	[ "$type" = commit ] || return 1
	echo "$sha"
}

# branch_absent <name>: whether GitHub answers that no branch has the name;
# false where it has one, or could not say.
branch_absent() {
	local out
	if out="$(gh api "repos/${repo}/git/ref/heads/$1" 2>&1)"; then
		return 1
	fi
	case "$out" in *"HTTP 404"*) return 0 ;; esac
	return 1
}

# check_run: refuse, with an error, a run Retry may not re-run (above);
# set the verdict job, follower.
check_run() {
	if [ "$r_repo" != "$repo" ] || [ "$r_head_repo" != "$repo" ]; then
		fail "Run ${run} is of ${r_repo}, its head of ${r_head_repo}, not ${repo}: Retry leaves it."
	fi
	if [ "$r_branch" != "$head_branch" ] || [ "$r_sha" != "$head_sha" ]; then
		fail "Run ${run} is of ${r_branch} at ${r_sha}, not the event's ${head_branch} at ${head_sha}: Retry leaves it."
	fi
	case "$r_path" in
	.github/workflows/ci.yml)
		[ "$r_event" = push ] || fail "Run ${run} of ci.yml is a ${r_event}, not a tag's push: Retry leaves it."
		[[ "$r_branch" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.+)?$ ]] ||
			fail "Run ${run} of ci.yml is of ${r_branch}, which is no version tag: Retry leaves it."
		local sha
		sha="$(tag_commit "$r_branch")" || fail "The tag ${r_branch} could not be read: Retry leaves run ${run}."
		[ "$sha" = "$r_sha" ] || fail "The tag ${r_branch} names ${sha}, not run ${run}'s ${r_sha}: Retry leaves it."
		branch_absent "$r_branch" ||
			fail "A branch is named ${r_branch} too, or GitHub could not say: Retry leaves run ${run}."
		follower="All tests passed"
		;;
	.github/workflows/nightly.yml)
		case "$r_event" in
		schedule | workflow_dispatch) ;;
		*) fail "Run ${run} of nightly.yml is a ${r_event}, not the schedule's or one by hand: Retry leaves it." ;;
		esac
		[ "$r_branch" = main ] || fail "Run ${run} of nightly.yml is on ${r_branch}, not main: Retry leaves it."
		follower="Platforms passed"
		;;
	*) fail "Run ${run} is of ${r_path}, neither ci.yml nor nightly.yml: Retry leaves it." ;;
	esac
}

# The latest attempt's jobs as a JSON array of {name, attempt, state}: the
# state "pending" before a job ends, "not taken" for one cancelled with no
# runner and no step, "passed", "skipped", or its conclusion.
jobs() {
	gh api --paginate "repos/${repo}/actions/runs/${run}/jobs?filter=latest&per_page=100" --jq '.jobs[]' |
		jq -s 'map({name, attempt: (.run_attempt // 1), state: (
			if .status != "completed" then "pending"
			elif .conclusion == "cancelled" and (.runner_name // "") == "" and (.steps // [] | length) == 0 then "not taken"
			elif .conclusion == "success" or .conclusion == "neutral" then "passed"
			else .conclusion // "no conclusion" end)})'
}

follower=""
read_run_or_fail
check_run
note "Retry: run ${run} (${r_path}, ${r_event} of ${r_branch}), the jobs ${names}, at most ${retries} re-run(s), ${wait} s after an attempt."
attempt=1 round=0 rerun='[]' js='[]'
while :; do
	# Wait for the attempt: the jobs re-run in it ended, and where one of
	# them was not taken again, the attempt itself. A run that cannot be
	# read is looked at again until the deadline.
	end=$((SECONDS + deadline))
	while :; do
		if read_run; then
			if [ "$r_attempt" -gt "$attempt" ]; then
				note "Run ${run} is at attempt ${r_attempt}, after ${attempt}: someone ran it again, and Retry leaves it."
				exit 0
			fi
			if [ "$r_attempt" = "$attempt" ] && js="$(jobs)"; then
				if [ "$round" -gt 0 ] && jq -e --argjson want "$rerun" --argjson at "$attempt" '
					[.[] | select(.name | IN($want[]))] as $re
					| ($re | length) == ($want | length)
						and all($re[]; .attempt == $at and (.state | IN("pending", "not taken") | not))' <<<"$js" >/dev/null; then
					note "A runner took them in attempt ${attempt}: $(jq -r --argjson want "$rerun" '[.[] | select(.name | IN($want[])) | "\(.name) \(.state)"] | join(", ")' <<<"$js"). The run's own verdict decides."
					exit 0
				fi
				[ "$r_status" != completed ] || break
			fi
		fi
		[ "$SECONDS" -lt "$end" ] || fail "Attempt ${attempt} of run ${run} did not end within ${deadline} s."
		sleep "$poll"
	done

	if [ "$round" -gt 0 ]; then
		left="$(jq -r --argjson want "$rerun" --argjson at "$attempt" '
			[$want[] as $n | select(all(.[]; .name != $n or .attempt < $at)) | $n] | join(", ")' <<<"$js")"
		[ -z "$left" ] || fail "The re-run of attempt ${attempt} left out ${left}, which no runner took: run it again by hand."
	fi
	unrun="$(jq -c --argjson names "$names" '[.[] | select((.name | IN($names[])) and .state == "not taken") | .name]' <<<"$js")"
	if [ "$unrun" = '[]' ]; then
		note "A runner took every job of ${names} in attempt ${attempt}: nothing to run again."
		exit 0
	fi
	list="$(jq -r 'join(", ")' <<<"$unrun")"
	bad="$(jq -r --argjson unrun "$unrun" --arg follower "$follower" '
		[.[] | select((.state | IN("passed", "skipped") | not) and (.name | IN($unrun[]) | not) and .name != $follower)
			| "\(.name) (\(.state))"] | join(", ")' <<<"$js")"
	if [ -n "$bad" ]; then
		note "No runner took ${list} in attempt ${attempt}, but ${bad} did not pass either: nothing is run again."
		exit 0
	fi
	verdict="$(jq -r --arg follower "$follower" '[.[] | select(.name == $follower) | .state][0] // "missing"' <<<"$js")"
	if [ "$verdict" != failure ]; then
		note "No runner took ${list} in attempt ${attempt}, and ${follower} is ${verdict}, not failed (a run someone cancelled?): nothing is run again."
		exit 0
	fi
	if [ "$round" -ge "$retries" ]; then
		fail "No runner took ${list} in ${attempt} attempts of run ${run}: the run fails for want of a runner, a tag's release with it, or the night is skipped."
	fi
	round=$((round + 1))
	note "No runner took ${list} in attempt ${attempt}: re-run ${round} of ${retries} in ${wait} s."
	sleep "$wait"
	read_run_or_fail
	check_run
	if [ "$r_attempt" != "$attempt" ] || [ "$r_status" != completed ]; then
		note "Run ${run} is now ${r_status} at attempt ${r_attempt}: someone ran it again, and Retry leaves it."
		exit 0
	fi
	gh api -X POST "repos/${repo}/actions/runs/${run}/rerun-failed-jobs" >/dev/null ||
		fail "GitHub refused the re-run of run ${run}'s failed jobs."
	note "Asked GitHub to re-run run ${run}'s failed jobs: attempt $((attempt + 1))."
	rerun="$unrun" attempt=$((attempt + 1))
done

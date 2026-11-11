#!/usr/bin/env bash
# Waits for a job of the same workflow run to succeed (release.yml: apt-build
# and the Homebrew job wait for "Sign and publish"). They start beside it,
# so that one approval of the signing environment lets all of them run;
# GitHub asks again for a job that reaches the environment later.
# The job is found by its name, or as "<caller> / <name>" where a workflow
# calls this one, in every attempt of the run: a re-run of a failed
# follower finds the attempt in which the job succeeded.
# Exits 0 once it has succeeded, 1 once it has ended otherwise in this
# attempt, or after the deadline (seconds, 1800 by default).
# Needs GH_TOKEN with actions: read, GITHUB_REPOSITORY, GITHUB_RUN_ID and
# GITHUB_RUN_ATTEMPT, which GitHub sets.
set -euo pipefail

usage() {
	echo "usage: $0 <job name> [<deadline seconds>]" >&2
	exit 2
}

case "$#" in 1 | 2) ;; *) usage ;; esac
job="$1"
deadline="${2:-1800}"
case "$deadline" in '' | *[!0-9]*) usage ;; esac
attempt="${GITHUB_RUN_ATTEMPT:-1}"
end=$((SECONDS + deadline))

while :; do
	# One line per job: attempt, status, conclusion, name.
	jobs="$(gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/jobs?filter=all&per_page=100" \
		--jq '.jobs[] | "\(.run_attempt)\t\(.status)\t\(.conclusion // "")\t\(.name)"')"
	seen=""
	while IFS=$'\t' read -r at status conclusion name; do
		[ "$name" = "$job" ] || [ "${name% / "$job"}" != "$name" ] || continue
		if [ "$conclusion" = success ]; then
			echo "wait-for-job: ${name} succeeded (attempt ${at})"
			exit 0
		fi
		if [ "$at" = "$attempt" ]; then
			seen="${status}${conclusion:+ ($conclusion)}"
			if [ "$status" = completed ]; then
				echo "::error::${name} ended ${conclusion:-without a conclusion}; nothing follows it."
				exit 1
			fi
		fi
	done <<<"$jobs"
	if [ "$SECONDS" -ge "$end" ]; then
		echo "::error::${job} did not succeed within ${deadline} s (${seen:-not seen in this run})."
		exit 1
	fi
	echo "wait-for-job: ${job} is ${seen:-not listed yet}; waiting"
	sleep 15
done

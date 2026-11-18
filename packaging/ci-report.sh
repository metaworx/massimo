#!/usr/bin/env bash
# The report of ci.yml's All tests passed (GUIDELINES/project/CI.md,
# section 2): which job did not pass, and how, and which passed only on a
# re-run (AP TestInfrastructure, D10-A). The verdict is the job's next
# step's, from its needs; this script fails nothing.
#
# The names come from the run's jobs through the API, matrix jobs
# expanded, "cancelled before a runner took it" for a cancelled job with
# no runner and no step; NEEDS's ids stand in where the API tells nothing,
# or not what failed. Left out: this job (SELF) and the jobs of the
# workflows the run calls (CALLED), which are not among its needs.
#   - The summary lists every job, those that did not pass first, with an
#     error annotation for each that did not pass.
#   - From the second attempt on, a job that did not pass in an earlier
#     attempt and passes now counts as passed, and is named: in a line of
#     the summary with the attempts it did not pass in and the one it passed
#     in, and with a warning annotation.
#   - On a pull request (PR set) one comment, starting with
#     <!-- ci-report -->, is the report: written at the pull request's first
#     failing run and edited by every run after it, a passing one included,
#     the jobs that needed a re-run named in it. An edit notifies no one,
#     so a job whose last run before this attempt did not pass, and which
#     passes now, gets a comment of its own, starting with
#     <!-- ci-rerun -->, which notifies the pull request's author: once
#     for each re-run that made a job pass.
#   - A push (no PR) gets the summary and the annotations alone.
# A comment that cannot be written, as with a read-only token, Dependabot's
# or a fork's, leaves a warning, in the summary too.
#
# Needs GH_TOKEN with actions: read, and pull-requests: write for the
# comments; GITHUB_REPOSITORY, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT,
# GITHUB_SERVER_URL and GITHUB_STEP_SUMMARY, which GitHub sets; NEEDS, the
# job's needs as JSON; SHA, the commit tested; SELF and CALLED as above.
# Exits 0 but where one of them is missing or NEEDS is no JSON.
set -euo pipefail

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
run="${GITHUB_RUN_ID:?GITHUB_RUN_ID is not set}"
attempt="${GITHUB_RUN_ATTEMPT:-1}"
summary="${GITHUB_STEP_SUMMARY:?GITHUB_STEP_SUMMARY is not set}"
needs="${NEEDS:?NEEDS is not set}"
self="${SELF:-All tests passed}"
called="${CALLED:-^$}"
pr="${PR:-}"
sha="${SHA:-}"
run_url="${GITHUB_SERVER_URL:-https://github.com}/${repo}/actions/runs/${run}"
case "$attempt" in '' | *[!0-9]*) attempt=1 ;; esac

# A warning in the log, as an annotation, and in the summary.
warn() { # <message>
	echo "::warning::$1"
	printf -- '- **Warning:** %s\n' "$1" >>"$summary"
}

# How a job of the API ended, in the report's words.
jq_result='def result:
	if .status != "completed" then .status | gsub("_"; " ")
	elif .conclusion == "cancelled" and (.runner_name // "") == "" and (.steps // [] | length) == 0
		then "cancelled before a runner took it"
	elif .conclusion == "success" then "passed"
	elif .conclusion == "failure" then "failed"
	else .conclusion // "no conclusion" | gsub("_"; " ") end;'

# The jobs of a run's attempts, one JSON object a line: filter=latest gives
# each job's last run, filter=all its runs in every attempt.
api_jobs() { # <latest|all>
	gh api --paginate "repos/${repo}/actions/runs/${run}/jobs?filter=${1}&per_page=100" --jq '.jobs[]'
}

verdict=$(jq -r 'if all(.[]; .result == "success" or .result == "skipped") then "passed" else "failed" end' <<<"$needs")
jobs=$(api_jobs latest |
	jq -s --arg self "$self" --arg called "$called" "${jq_result}"'
		map(select(.name != $self and (.name | test($called) | not)) | {name, result: result})') || jobs='[]'
if [ "$jobs" = '[]' ] || { [ "$verdict" = failed ] && jq -e 'all(.[]; .result == "passed" or .result == "skipped")' <<<"$jobs" >/dev/null; }; then
	# The API told nothing, or not what failed: the needs' ids.
	jobs=$(jq 'to_entries | map({name: .key, result: (.value.result | if . == "success" then "passed" elif . == "failure" then "failed" else . end)})' <<<"$needs")
fi
jobs=$(jq 'sort_by((.result == "passed" or .result == "skipped"), (.name | ascii_downcase))' <<<"$jobs")
bad=$(jq '[.[] | select(.result != "passed" and .result != "skipped")]' <<<"$jobs")
n=$(jq length <<<"$bad")

# The jobs that pass now and did not in an earlier attempt: each with the
# attempts it did not pass in, the attempt it passed in after them, and
# whether its last run before this attempt did not pass, which this
# attempt's re-run then made pass. A job a re-run did not take is carried
# into the next attempt with its start, so a start counts once.
reruns='[]'
reruns_failed=false
if [ "$attempt" -gt 1 ]; then
	if all=$(api_jobs all | jq -s .); then
		reruns=$(jq --argjson jobs "$jobs" --argjson attempt "$attempt" --arg self "$self" --arg called "$called" "${jq_result}"'
			def bad: .conclusion != "success" and .conclusion != "skipped" and .conclusion != "neutral";
			($jobs | map(select(.result == "passed") | {key: .name, value: true}) | from_entries) as $passed
			| map(select(.name != $self and (.name | test($called) | not) and .status == "completed"
				and (.run_attempt // 1) <= $attempt and $passed[.name]))
			| group_by(.name)
			| map(sort_by(.run_attempt // 1)
				| map(select((.run_attempt // 1) < $attempt)) as $earlier
				| ([$earlier | unique_by(.started_at // (.id | tostring))[] | select(bad)
					| {attempt: (.run_attempt // 1), result: result}] | sort_by(.attempt)) as $before
				| {name: .[0].name, before: $before,
					passed: ([.[] | select(.conclusion == "success" and (.run_attempt // 1) > ($before | last | .attempt // 0))
						| .run_attempt // 1] | min // $attempt),
					last: (($earlier | last // {conclusion: "success"}) | bad)})
			| map(select(.before | length > 0))
			| sort_by(.name | ascii_downcase)' <<<"$all") || { reruns='[]'; reruns_failed=true; }
	else
		reruns_failed=true
	fi
fi
r=$(jq length <<<"$reruns")
notify=$(jq '[.[] | select(.last)]' <<<"$reruns")
# What a re-run job did: "attempt 1 failed, passed in attempt 2".
jq_before='def before: (.before | map("attempt \(.attempt) \(.result)") | join(", ")) + ", passed in attempt \(.passed)";'

if [ "$verdict" = passed ]; then
	head="Every test job passed or was skipped"
	[ "$r" -eq 0 ] || head="${head}, ${r} only on a re-run"
else
	head="$n test job(s) did not pass"
fi
{
	echo "### ${head}"
	echo
	echo "| Job | Result |"
	echo "|-----|--------|"
	jq -r '.[] | "| \(.name) | \(.result) |"' <<<"$jobs"
	if [ "$r" -gt 0 ]; then
		echo
		echo "Passed only on a re-run, and counted as passed:"
		echo
		jq -r "${jq_before}"'.[] | "- \(.name): \(before)"' <<<"$reruns"
	fi
	echo
} >>"$summary"
jq -r '.[] | "::error::\(.name): \(.result)"' <<<"$bad"
jq -r "${jq_before}"'.[] | "::warning::\(.name) passed only on a re-run: \(before)"' <<<"$reruns"
[ "$reruns_failed" = false ] || warn "the earlier attempts' jobs could not be read: a job that passed only on a re-run is not named."

[ -n "$pr" ] || exit 0

# The table of the re-run jobs, for both comments.
rerun_table() {
	echo "| Job | Before | Now |"
	echo "|-----|--------|-----|"
	jq -r '.[] |
		"| \(.name) | \(.before | map("attempt \(.attempt) \(.result)") | join(", ")) | passed in attempt \(.passed) |"' <<<"$1"
}

marker='<!-- ci-report -->'
comment="$(mktemp)"
trap 'rm -f "$comment"' EXIT
{
	echo "$marker"
	echo "**${head}** on ${sha:0:7}, [run ${run} attempt ${attempt}](${run_url})."
	if [ "$n" -gt 0 ]; then
		echo
		echo "| Job | Result |"
		echo "|-----|--------|"
		jq -r '.[] | "| \(.name) | \(.result) |"' <<<"$bad"
	fi
	if [ "$r" -gt 0 ]; then
		echo
		echo "Passed only on a re-run, and counted as passed:"
		echo
		rerun_table "$reruns"
	fi
} >"$comment"
id=$(gh api --paginate "repos/${repo}/issues/${pr}/comments" \
	--jq '.[] | select(.user.login == "github-actions[bot]" and (.body | startswith("<!-- ci-report -->"))) | .id' | tail -n 1) || id=''
if [ -n "$id" ]; then
	gh api -X PATCH "repos/${repo}/issues/comments/${id}" -F body=@"$comment" >/dev/null ||
		warn "the pull request's report was not updated (a read-only token?)."
elif [ "$verdict" = failed ]; then
	gh api -X POST "repos/${repo}/issues/${pr}/comments" -F body=@"$comment" >/dev/null ||
		warn "no report on the pull request (a read-only token?)."
fi

# The comment that notifies: the jobs this attempt's re-run made pass.
k=$(jq length <<<"$notify")
[ "$k" -gt 0 ] || exit 0
{
	echo '<!-- ci-rerun -->'
	echo "**${k} job(s) passed only on a re-run** on ${sha:0:7}, [run ${run} attempt ${attempt}](${run_url}):"
	echo "they count as passed, and what kept them from passing before is worth a look."
	echo
	rerun_table "$notify"
} >"$comment"
gh api -X POST "repos/${repo}/issues/${pr}/comments" -F body=@"$comment" >/dev/null ||
	warn "the pull request was not told of the re-run (a read-only token?)."
exit 0

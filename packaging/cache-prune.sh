#!/usr/bin/env bash
# CodeQL's overlay databases of older commits, deleted from the
# repository's Actions caches (AP TestSpeed, D10-A; GUIDELINES/project/
# CI.md, section 3). Code scanning's default setup saves one for each
# language at every commit of main it analyses, about 90 MB a commit, and
# restores only the newest whose key has its prefix; the older ones fill
# the repository's 10 GB, past which GitHub evicts the least recently used
# caches, the Go jobs' among them. For each of the keys' stems (the key
# without its commit, run and attempt: CodeQL's hash, the language and the
# CLI's version) it keeps the entries of the KEEP newest commits (3) and
# deletes the rest of main's. An entry of another ref, a key of another
# form and every other cache are left alone, whatever the API lists; the
# summary counts main's entries with the prefix whose key has another
# form, with a warning where there are any, as CodeQL changing its keys'
# form would leave every entry alone.
# The deletes come PRUNE_PAUSE seconds apart (1), as GitHub's secondary
# rate limits refuse many writes in a row. A delete that finds the entry
# gone (404) counts as done. A refusal for the rate (429, or a 403 that
# names a rate limit) ends the run with a notice, and the next run goes
# on; any other 403 ends it with a warning, and the script fails. Any
# other failure is a warning, and the script fails after trying the rest.
# DRY_RUN=1 names the entries without deleting them. The summary
# (GITHUB_STEP_SUMMARY; stderr where it is not set) says what went.
# usage: GH_TOKEN=… GITHUB_REPOSITORY=owner/repo packaging/cache-prune.sh
set -euo pipefail

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY names the repository}"
keep="${KEEP:-3}"
pause="${PRUNE_PAUSE:-1}"
summary="${GITHUB_STEP_SUMMARY:-/dev/stderr}"
prefix=codeql-overlay-base-database-
case "$keep" in '' | *[!0-9]* | 0*)
	echo "KEEP must be a number of commits, 1 or more: ${keep}" >&2
	exit 2
	;;
esac

# main's entries under the prefix, newest first; the API matches the key
# as a prefix.
listed="$(gh api --paginate \
	"repos/${repo}/actions/caches?per_page=100&ref=refs/heads/main&key=${prefix}&sort=created_at&direction=desc" \
	--jq '.actions_caches[] | [.id, .key, .ref, .created_at, .size_in_bytes] | @tsv')"

# main's entries under the prefix, as objects with the key's stem and
# commit, null where the key has another form. A jq program: its $names
# are jq's.
# shellcheck disable=SC2016
entries='[inputs | select(length > 0) | split("\t")
	| {id: .[0], key: .[1], ref: .[2], created: .[3], size: (.[4] | tonumber? // 0)}
	| select(.ref == "refs/heads/main" and (.key | startswith($prefix)) and (.id | test("^[0-9]+$")))
	| . + ((.key | capture("^(?<stem>.+)-(?<sha>[0-9a-f]{40})-[0-9]+-[0-9]+$")) // {stem: null})]'
odd="$(jq -R -n --arg prefix "$prefix" "${entries}"' | map(select(.stem == null)) | length' <<<"$listed")"

# The entries to delete, the oldest first: per stem, those of every
# commit but the KEEP newest, a commit's age its newest entry's (a re-run
# adds one).
doomed="$(jq -R -n -r --arg prefix "$prefix" --argjson keep "$keep" "${entries}"'
	| [map(select(.stem != null))
		| group_by(.stem)[]
		| ([group_by(.sha)[] | {sha: .[0].sha, newest: (map(.created) | max)}]
			| sort_by(.newest) | reverse | .[:$keep] | map(.sha)) as $kept
		| .[] | select(.sha as $s | $kept | index($s) | not)]
	| sort_by(.created, (.id | tonumber))[]
	| [.id, .key, .size] | @tsv' <<<"$listed")"

deleted=0 gone=0 failed=0 bytes=0 first=true stopped=""
while IFS=$'\t' read -r id key size; do
	[ -n "$id" ] || continue
	# Never another cache, whatever the filter above let through.
	case "$key" in "$prefix"*) ;; *) continue ;; esac
	if [ "${DRY_RUN:-}" = 1 ]; then
		echo "would delete ${key} (${size} bytes)"
		deleted=$((deleted + 1)) bytes=$((bytes + size))
		continue
	fi
	[ "$first" = true ] || sleep "$pause"
	first=false
	if err="$(gh api -X DELETE "repos/${repo}/actions/caches/${id}" 2>&1 >/dev/null)"; then
		echo "deleted ${key} (${size} bytes)"
		deleted=$((deleted + 1)) bytes=$((bytes + size))
	elif [[ "$err" == *"HTTP 404"* ]]; then
		echo "already gone: ${key}"
		gone=$((gone + 1))
	elif [[ "$err" == *"HTTP 429"* || ("$err" == *"HTTP 403"* && "${err,,}" == *"rate limit"*) ]]; then
		stopped=rate
		echo "::notice::GitHub's rate limit refused a delete: the next run deletes the rest (${err})"
		break
	elif [[ "$err" == *"HTTP 403"* ]]; then
		stopped=refused
		failed=$((failed + 1))
		echo "::warning::GitHub refused to delete ${key}: ${err}"
		break
	else
		echo "::warning::could not delete ${key}: ${err}"
		failed=$((failed + 1))
	fi
done <<<"$doomed"

[ "$odd" -eq 0 ] ||
	echo "::warning::${odd} of main's caches under ${prefix} have a key of another form, and were left alone: has CodeQL changed its keys?"
{
	echo "### CodeQL's caches"
	echo
	verb=deleted
	[ "${DRY_RUN:-}" != 1 ] || verb="to delete (dry run)"
	echo "- ${deleted} overlay database(s) of older commits ${verb}, $((bytes / 1000000)) MB; the ${keep} newest commits' kept for each language"
	[ "$gone" -eq 0 ] || echo "- ${gone} already gone"
	[ "$failed" -eq 0 ] || echo "- ${failed} could not be deleted: see the warnings"
	[ "$stopped" != rate ] || echo "- stopped by GitHub's rate limit: the next run deletes the rest"
	[ "$stopped" != refused ] || echo "- stopped at a refusal (403) that names no rate limit"
	echo "- ${odd} with a key of another form, left alone"
} >>"$summary"
[ "$failed" -eq 0 ]

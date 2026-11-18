#!/usr/bin/env bash
# The tests CI's Go test jobs skipped, and why (GUIDELINES/project/CI.md,
# section 3; AP TestInfrastructure, D9-A): it reads `go test -json` on
# stdin, as `go test -json ./...` or `make race TESTFLAGS=-json` write it,
# and writes the log while the tests run: each package's line, a failed
# test's output as `go test -v` writes it, the build's errors, and any line
# that is not go test's JSON as it came. Passed and skipped tests say
# nothing, as without -v.
# Once stdin ends, it adds to the summary (GITHUB_STEP_SUMMARY; stderr
# where it is not set) how many named tests skipped, each with its
# package and its reason, the test's own lines, so that a skip where the
# platform has what the test needs shows; then the run's 20 slowest tests
# and its 10 slowest packages, from go test's times (AP TestSpeed, Block
# 3), so that a run shows where its time went.
# It fails nothing and always exits 0: the job's outcome is go test's,
# which the step keeps with `set -o pipefail`. A line it cannot read goes
# to the log as it came; should the reading itself die, the whole stream
# does at its end, as `go test -v` writes it, since the lines it had taken
# are lost to it. Either way the summary says the skips were not read, and
# names no times.
# usage: go test -json ./... | packaging/go-test-report.sh [<title>]
set -uo pipefail

title="${1:-go test}"
summary="${GITHUB_STEP_SUMMARY:-/dev/stderr}"
# How much of a test's output is held until it ends: its first lines
# (GO_TEST_REPORT_FIRST) and its last (GO_TEST_REPORT_LAST), with the
# number left out between them. Held whole, a test that writes tens of
# thousands of lines slowed the reading down until it could hold go test
# up behind the pipe.
first="${GO_TEST_REPORT_FIRST:-1000}"
last="${GO_TEST_REPORT_LAST:-200}"
raw="$(mktemp)"
errs="$(mktemp)"
died=false
trap 'rm -f "$raw" "$errs"' EXIT

# A line of the stream as an event; a line that is not one, as raw output.
jq_event='def event: (fromjson? | objects | select((.Output // "" | type) == "string")) // {Action: "raw", Output: (. + "\n")};
	def framing: test("^=== (RUN|PAUSE|CONT|NAME) ");'

# Each test's output is held until it ends, as -v writes it: dropped
# where it passed or skipped, written where it failed. A package's own
# lines are written as they come, but the PASS of a package that passed
# and its last lines (ok, FAIL), which wait for the package's end: then
# the output of its tests that never ended, as one that ran into go test's
# -timeout, comes first, its goroutines among it, and then those lines.
# What is still held at the end of the stream is written then. The held
# output is a new object whenever it is empty: jq's objects keep a deleted
# key's slot, and one object for every test of a run grew slower with
# each. A line whose reading fails is written as it came, and noted on
# stderr, which goes to $errs. tee goes on into $raw where jq died.
tee --output-error=warn-nopipe "$raw" | jq -R -n -j --unbuffered --argjson first "$first" --argjson last "$last" "${jq_event}"'
	def held: (.head | join(""))
		+ (if .cut > 0 then "go-test-report: \(.cut) line(s) of this test left out\n" else "" end)
		+ (.tail | join(""));
	def hold($k; $o): .buf[$k] |= ((. // {head: [], tail: [], cut: 0})
		| if (.head | length) < $first then .head += [$o]
		else .tail += [$o] | if (.tail | length) > $last then .tail |= .[1:] | .cut += 1 else . end end);
	def fresh: if .buf == {} then .buf = {} else . end;
	def drop($k): del(.buf[$k]) | fresh;
	def take($k): .out = (.buf[$k] // null | if . then held else "" end) | drop($k);
	def flush($p): (.buf | to_entries | map(select(.key | startswith($p + "\t")))) as $held
		| .out = ([$held[].value | held] | add // "") + (.ends[$p] // "")
		| .buf |= with_entries(select(.key | startswith($p + "\t") | not)) | fresh
		| del(.ends[$p]);
	def handle($e):
		if $e.Action == "raw" or $e.Action == "build-output" then .out = ($e.Output // "")
		elif ($e.Test // "") == "" then
			"\($e.Package // "")" as $p
			| if $e.Action == "output" then
				($e.Output // "") as $o
				| if $o == "PASS\n" then .
				elif ($o | test("^(FAIL\\s|ok  \\t)")) then .ends[$p] += $o
				else .out = $o end
			elif $e.Action == "pass" or $e.Action == "fail" or $e.Action == "skip" then flush($p)
			else . end
		else "\($e.Package // "")\t\($e.Test)" as $k
			| if $e.Action == "output" then hold($k; $e.Output // "")
			elif $e.Action == "fail" then take($k)
			elif $e.Action == "pass" or $e.Action == "skip" then drop($k)
			else . end
		end;
	foreach ((inputs | {line: .}), {eof: true}) as $in ({buf: {}, ends: {}, out: null};
		. as $state
		| try (.out = null
			| if $in.eof then .out = ([.buf[] | held] | add // "") + ([.ends[]] | add // "") | .buf = {} | .ends = {}
			else handle($in.line | event) end)
		catch (. as $error | $state | .out = (if $in.eof then "" else $in.line + "\n" end)
			| ("\($error): \($in.line // "the end of the stream")\n" | stderr) as $_ | .);
		.out // empty)' 2>"$errs"
reading=("${PIPESTATUS[@]}")
[ "${reading[1]}" = 0 ] || died=true
if [ "$died" = true ]; then
	echo "go-test-report: the stream could not be read; all of it as go test -v writes it:"
	jq -R -j '. as $line | ([fromjson? | objects] | first) as $e
		| if $e == null then $line + "\n" else ($e.Output | strings) // empty end' "$raw" 2>/dev/null || cat "$raw"
fi
if [ -s "$errs" ]; then
	echo "go-test-report: what could not be read:"
	cat "$errs"
fi

# The named tests that skipped, in the order they did: package, test, and
# the test's own lines but the framing and its --- SKIP line, as one
# reason. Only the skipped tests' output is gathered.
skips=""
if [ "$died" = false ] && [ ! -s "$errs" ]; then
	skips="$(jq -R -s "${jq_event}"'
		[split("\n")[] | select(length > 0) | event | select((.Test // "") != "")
			| . + {k: "\(.Package // "")\t\(.Test)"}] as $ev
		| ([$ev[] | select(.Action == "skip") | {key: .k, value: true}] | from_entries) as $skipped
		| (reduce ($ev[] | select(.Action == "output" and $skipped[.k])) as $e ({}; .[$e.k] += ($e.Output // ""))) as $out
		| [$ev[] | select(.Action == "skip") | {package: (.Package // ""), test: .Test,
			reason: (($out[.k] // "") | split("\n")
				| map(sub("^\\s+"; "") | select(length > 0 and (framing | not) and (startswith("--- SKIP") | not)))
				| join(" / "))}]' "$raw" 2>/dev/null)" || skips=""
fi

# The run's times, from the Elapsed of go test's pass and fail events: the
# 20 slowest tests, top-level ones alone, since a test's time holds its
# subtests' and a subtest would only follow its parent in the list; and
# the 10 slowest packages, each its test binary's own run, with how many
# top-level tests ended in it. A parent whose subtests run in parallel
# reports only its own function's time, so they would drop out of the
# list; no test here runs its subtests in parallel. A package whose
# result came from go test's cache, its `ok` line saying (cached), is
# left out of both and counted: its tests' times are those of the run
# that was cached. Each event is cut to its fields before the reduce, so
# that the reduce holds no `//`, whose fork point made it copy the list
# at every test's end: 100,000 tests took two minutes.
times=""
if [ -n "$skips" ]; then
	times="$(jq -R -n "${jq_event}"'
		reduce (inputs | event | select(.Action == "pass" or .Action == "fail" or .Action == "skip"
			or (.Action == "output" and (.Test // "") == ""))
			| {a: .Action, p: "\(.Package // "")", t: (.Test // ""), o: (.Output // ""),
				s: (if (.Elapsed | type) == "number" and .Action != "skip" then .Elapsed else null end)}) as $e
			({tests: [], packages: {}, cached: {}};
			if $e.t == "" then
				if $e.a == "output" then
					if ($e.o | test("^ok  \t.*\t\\(cached\\)")) then .cached[$e.p] = true else . end
				elif $e.s != null then .packages[$e.p].s = $e.s
				else . end
			elif $e.t | contains("/") then .
			else .packages[$e.p].n += 1
				| if $e.s != null then .tests[.tests | length] = {package: $e.p, test: $e.t, s: $e.s} else . end
			end)
		| .cached as $c
		| {tests: ([.tests[] | select($c[.package] | not)] | sort_by(-.s) | .[:20]),
			packages: ([.packages | to_entries[] | select(($c[.key] | not) and .value.s != null)
				| {package: .key, n: (.value.n // 0), s: .value.s}] | sort_by(-.s) | .[:10]),
			cached: ($c | length)}' "$raw" 2>/dev/null)" || times=""
fi
# Seconds as go test writes a test's, to two places.
jq_secs='def secs: . * 100 | round
	| "\(. / 100 | floor).\(. % 100 | tostring | if length < 2 then "0" + . else . end)";'

{
	if [ -z "$skips" ]; then
		echo "### Skipped tests, ${title}: not read"
		echo
		echo "The output of go test -json could not be read whole; the job's log has what came, and the summary names no test's time."
	else
		count="$(jq length <<<"$skips")"
		echo "### Skipped tests, ${title}: ${count}"
		if [ "$count" -gt 0 ]; then
			echo
			echo "| Package | Test | Why |"
			echo "|---------|------|-----|"
			jq -r '.[:200][] | "| \(.package) | \(.test) | \(.reason | if length > 200 then .[:200] + "..." else . end | gsub("\\|"; "\\|")) |"' <<<"$skips"
			[ "$count" -le 200 ] || echo "| | and $((count - 200)) more | |"
		fi
	fi
	echo
	if [ -n "$skips" ] && [ -z "$times" ]; then
		echo "### Slowest tests, ${title}: not read"
		echo
	elif [ -n "$times" ]; then
		tests="$(jq '.tests | length' <<<"$times")"
		cached="$(jq '.cached' <<<"$times")"
		if [ "$tests" -gt 0 ] || [ "$cached" -gt 0 ]; then
			echo "### Slowest tests, ${title}"
			echo
		fi
		if [ "$cached" -gt 0 ]; then
			echo "${cached} package(s) came from go test's cache and are left out: their times are an earlier run's."
			echo
		fi
		if [ "$tests" -gt 0 ]; then
			echo "| Package | Test | Seconds |"
			echo "|---------|------|--------:|"
			jq -r "${jq_secs}"'.tests[] | "| \(.package) | \(.test) | \(.s | secs) |"' <<<"$times"
			echo
		fi
		if [ "$(jq '.packages | length' <<<"$times")" -gt 0 ]; then
			echo "### Slowest packages, ${title}"
			echo
			echo "| Package | Tests | Seconds |"
			echo "|---------|------:|--------:|"
			jq -r "${jq_secs}"'.packages[] | "| \(.package) | \(.n) | \(.s | secs) |"' <<<"$times"
			echo
		fi
	fi
} >>"$summary"
exit 0

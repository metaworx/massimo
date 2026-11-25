#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The web UI smoke test (make ui-test): builds massimo, starts a server
# set for each of UI_TEST_WORKERS pytest-xdist workers (default 4), and
# runs the pytest suite next to this script (conftest.py, test_*.py) in
# them, each worker against a set of its own and a file's tests together
# in one worker (--dist loadfile); UI_TEST_WORKERS=1 runs it in one
# process against one set. The arguments after "smoke" go to pytest, as
# do those in PYTEST_ADDOPTS (PYTEST_ADDOPTS='-k cubes' make ui-test).
# Of a failed test it keeps the screenshot under tools/ui-test/results/,
# and of a run whose tests failed each set's serve.log and fakecube.log
# under results/logs/set<n>/, which CI uploads with the screenshots, so
# that a failure shows what the servers did meanwhile (AP ServerTraffic,
# D4-A).
# The first pass records no trace, whose DOM snapshots make every step
# slower, and no video, which lengthens every test's tear-down (AP
# TestSpeed, D8-A); when tests fail, they run once more with both, the
# screenshot, the video and the trace of one that fails again under
# results/retry/, and the run fails either way. The first pass ends
# with its 20 slowest phases, each a test's set-up, call or tear-down
# (--durations=20, before PYTEST_ADDOPTS, which can ask for more), so
# that a run shows where its time went (AP TestSpeed, Block 3); the
# retry, whose few tests carry the video and the trace, lists none of
# its own.
# `run.sh screenshots` (make ui-screenshots) starts one set and runs
# screenshots.py instead, which takes the screenshot set into
# docs/screenshots/.
# Without Playwright (and, for the suite, pytest and pytest-playwright)
# it says so and exits 0, so `make ui-test` never fails on a machine that
# cannot run it; UI_TEST_REQUIRED=1, as CI sets it, makes that exit 1.
# Without pytest-xdist the suite runs in one process against one set,
# and with UI_TEST_REQUIRED=1 and more than one worker it exits 1.
#
# A set is the fake Cube with the greeting next to this script and the
# server against it, both on loopback ports of their own and in an empty
# directory (no .env, no MASSIMO_* variables but a test API token, so the
# page goes through the token and the stream tickets, and the fake Cube's
# address and name, which serve seeds into its store, massimo.db in that
# directory), and a second server. The suite gets set 0's page address
# in UI_TEST_URL, the token in UI_TEST_TOKEN and the server's log in
# UI_TEST_SERVE_LOG; worker gw<n> takes set n's from UI_TEST_URL_<n>,
# UI_TEST_DOWN_URL_<n> and UI_TEST_SERVE_LOG_<n> (conftest.py). A set has
# started when each of its programs says it listens on its own port and
# the server is connected to its fake Cube; every program started must
# still run then, since a server of another run on one of the ports would
# answer the same token. For the suite, each set's Cube then gets a
# backup taken by hand, so that its server takes no automatic one while
# the tests run (backed_up); its poll of the fake Cube stays at 10 s.
#
# The fake Cube has a device that joins pairing mode two seconds after
# n:, a copy of the radiator thermostat 0a010d as 0abcde (TEQ0000001),
# and joins again after a t: deleted it, so that every case can pair it.
# It has every memory slot free (-free-slots 50), so that its S replies
# say no radiogram pending: by default they say one, and the server then
# reconnects to read its budget two minutes after every write, whose
# burst of events and reads would fall into whichever test runs then
# (AP ServerTraffic, D1-A). A test that wants a reconnect plants one
# (POST …/refresh).
#
# The fake Cube also answers UDP discovery, as a Cube that is not
# configured (KEQ0000000), on a loopback port the server asks instead of
# broadcasting; the server's own discovery socket is on a free loopback
# port (-discover-listen 127.0.0.1:0), so that no set needs UDP 23272,
# which another program may hold.
#
# The second server, for the page of a Cube that is not connected, serves
# a Cube at a closed loopback port (127.0.0.1:9), Cellar Cube
# (KEQ0000200), from a directory of its own, with the same token; the
# suite gets its address in UI_TEST_DOWN_URL.
#
# Environment: UI_TEST_PYTHON (default tools/ui-test/.venv/bin/python
# when present, else python3), UI_TEST_REQUIRED, UI_TEST_WORKERS (4),
# UI_TEST_CASES (the cases to run, such as 390-en,390-de; all four), and
# set 0's ports: UI_TEST_CUBE_PORT (62919), UI_TEST_API_PORT (8119),
# UI_TEST_DISCOVERY_PORT (23919), UI_TEST_DOWN_PORT (the second server's,
# UI_TEST_API_PORT + 1); set n takes the fake Cube's two ports + n and
# the servers' two + 2n.
set -eu

mode=${1:-smoke}
[ $# -gt 0 ] && shift
case "$mode" in
	smoke) needs='import playwright.sync_api, pytest, pytest_playwright'; what="pytest, pytest-playwright and Playwright" ;;
	screenshots) needs='import playwright.sync_api'; what="Playwright" ;;
	*) echo "usage: run.sh [smoke [pytest arguments…]|screenshots]" >&2; exit 2 ;;
esac
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
py=${UI_TEST_PYTHON:-}
if [ -z "$py" ]; then
	if [ -x "$here/.venv/bin/python" ]; then py="$here/.venv/bin/python"; else py=python3; fi
fi
if ! "$py" -c "$needs" 2>/dev/null; then
	if [ "${UI_TEST_REQUIRED:-}" = 1 ]; then
		echo "ui-test: $mode cannot run, not installed for $py: $what for Python (see tools/ui-test/README.md)" >&2
		exit 1
	fi
	echo "ui-test: $mode skipped, not installed for $py: $what for Python (see tools/ui-test/README.md)"
	exit 0
fi

workers=${UI_TEST_WORKERS:-4}
case "$workers" in
	'' | *[!0-9]* | 0*) echo "ui-test: UI_TEST_WORKERS is a number from 1, not '$workers'" >&2; exit 2 ;;
esac
if [ "$mode" = screenshots ]; then
	workers=1
elif [ "$workers" -gt 1 ] && ! "$py" -c 'import xdist' 2>/dev/null; then
	if [ "${UI_TEST_REQUIRED:-}" = 1 ]; then
		echo "ui-test: $workers workers need pytest-xdist, not installed for $py (see tools/ui-test/README.md)" >&2
		exit 1
	fi
	echo "ui-test: pytest-xdist is not installed for $py, so the suite runs in one process (see tools/ui-test/README.md)"
	workers=1
fi

cube_port=${UI_TEST_CUBE_PORT:-62919}
api_port=${UI_TEST_API_PORT:-8119}
discovery_port=${UI_TEST_DISCOVERY_PORT:-23919}
down_port=${UI_TEST_DOWN_PORT:-$((api_port + 1))}
token=ui-test-token
work=$(mktemp -d)
pids=""
# The trap below runs cleanup, which shellcheck takes for unreachable.
# shellcheck disable=SC2317
cleanup() {
	for p in $pids; do kill "$p" 2>/dev/null || true; done
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

# The page's parts first (make ui): the server embeds the bundle.
(cd "$root" && go -C tools/uibundle run . && go build -o "$work/massimo" ./cmd/massimo)

# start_set n: set n's fake Cube and its two servers, in $work/set<n>.
start_set() {
	d="$work/set$1"
	cube="127.0.0.1:$((cube_port + $1))"
	disc="127.0.0.1:$((discovery_port + $1))"
	mkdir -p "$d/down"
	cd "$d"
	env -i PATH="$PATH" HOME="$HOME" "$work/massimo" fakecube -greeting "$here/greeting.txt" \
		-listen "$cube" -discovery "$disc" -free-slots 50 \
		-pair 0a010d,0abcde,TEQ0000001 > fakecube.log 2>&1 &
	pids="$pids $!"
	env -i PATH="$PATH" HOME="$HOME" MASSIMO_API_TOKEN="$token" \
		MASSIMO_CUBE_KEQ0000100_ADDRESS="$cube" MASSIMO_CUBE_KEQ0000100_NAME="Fake Cube" \
		"$work/massimo" serve -proxy off \
		-listen "127.0.0.1:$((api_port + 2 * $1))" -update-status "$d/no-update-status.json" \
		-discover-target "$disc" -discover-listen 127.0.0.1:0 -discover-interval 0 > serve.log 2>&1 &
	pids="$pids $!"
	cd "$d/down"
	env -i PATH="$PATH" HOME="$HOME" MASSIMO_API_TOKEN="$token" \
		MASSIMO_CUBE_KEQ0000200_ADDRESS="127.0.0.1:9" MASSIMO_CUBE_KEQ0000200_NAME="Cellar Cube" \
		"$work/massimo" serve -proxy off \
		-listen "127.0.0.1:$((down_port + 2 * $1))" -update-status "$d/no-update-status.json" \
		-discover-target "127.0.0.1:9" -discover-listen 127.0.0.1:0 -discover-interval 0 > ../serve-down.log 2>&1 &
	pids="$pids $!"
}

# listening log line what: waits up to 10 s for the line in which a
# program says it listens on its own port.
listening() {
	i=0
	until grep -qF "$2" "$1" 2>/dev/null; do
		i=$((i + 1))
		if [ $i -gt 50 ]; then
			echo "ui-test: $3 did not start (its port in use?); its log:" >&2
			cat "$1" >&2
			exit 1
		fi
		sleep 0.2
	done
}

# ready_set n: set n's programs listen on their own ports, and its server
# is connected to its fake Cube.
ready_set() {
	d="$work/set$1"
	api="127.0.0.1:$((api_port + 2 * $1))"
	listening "$d/fakecube.log" "fake cube listening on 127.0.0.1:$((cube_port + $1)) " "the fake Cube of set $1"
	listening "$d/fakecube.log" "answering discovery on 127.0.0.1:$((discovery_port + $1)) " "the fake Cube's discovery of set $1"
	listening "$d/serve.log" "msg=\"api listening\" source=system addr=$api " "the server of set $1"
	listening "$d/serve-down.log" "msg=\"api listening\" source=system addr=127.0.0.1:$((down_port + 2 * $1)) " "the second server of set $1"
	i=0
	until curl -sf -H "Authorization: Bearer $token" "http://$api/api/v1/cubes" 2>/dev/null | grep -q '"connected": *true'; do
		i=$((i + 1))
		if [ $i -gt 50 ]; then
			echo "ui-test: the server of set $1 did not connect to its fake Cube; its log:" >&2
			cat "$d/serve.log" >&2
			exit 1
		fi
		sleep 0.2
	done
}

n=0
while [ $n -lt "$workers" ]; do start_set $n; n=$((n + 1)); done
n=0
while [ $n -lt "$workers" ]; do ready_set $n; n=$((n + 1)); done
for p in $pids; do
	if ! kill -0 "$p" 2>/dev/null; then
		echo "ui-test: a fake Cube or a server this run started has stopped; the logs' ends:" >&2
		tail -n 5 "$work"/set*/*.log >&2
		exit 1
	fi
done

# An operator and a guest with passwords in every set, for the sign-in
# and the preferences tests: user add opens the store beside the running
# server.
n=0
while [ $n -lt "$workers" ]; do
	d="$work/set$n"
	cd "$d"
	printf 'op-password\n' | env -i PATH="$PATH" HOME="$HOME" "$work/massimo" -data "$d" user add operator -role operator -password-stdin >/dev/null 2>>serve.log
	printf 'visit-pass\n' | env -i PATH="$PATH" HOME="$HOME" "$work/massimo" -data "$d" user add guest1 -role guest -password-stdin >/dev/null 2>>serve.log
	n=$((n + 1))
done

if [ "$mode" = screenshots ]; then
	"$py" "$here/screenshots.py" "http://127.0.0.1:$api_port/" "$token"
	exit
fi

# backed_up n: set n's Cube has a backup taken by hand, so that its
# server takes none by itself during the run. A Cube without a backup
# gets one a minute after the start, which no setting turns off
# (internal/server/backups.go); with one, the Cube is clean, and an
# automatic backup waits for a change of its configuration and then
# backups.auto_min, 60 minutes, longer than a run. The server answers
# 503 until it has the Cube's greeting, so it is asked again for up to
# 10 s.
backed_up() {
	api="127.0.0.1:$((api_port + 2 * $1))"
	i=0
	until curl -sf -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{}' \
		"http://$api/api/v1/cubes/KEQ0000100/backups" >/dev/null 2>&1; do
		i=$((i + 1))
		if [ $i -gt 50 ]; then
			echo "ui-test: the server of set $1 took no backup of its Cube; its log:" >&2
			cat "$work/set$1/serve.log" >&2
			exit 1
		fi
		sleep 0.2
	done
}

n=0
while [ $n -lt "$workers" ]; do backed_up $n; n=$((n + 1)); done
cd "$here"
n=0
while [ $n -lt "$workers" ]; do
	export "UI_TEST_URL_$n=http://127.0.0.1:$((api_port + 2 * n))/" \
		"UI_TEST_DOWN_URL_$n=http://127.0.0.1:$((down_port + 2 * n))/" \
		"UI_TEST_SERVE_LOG_$n=$work/set$n/serve.log"
	n=$((n + 1))
done
if [ "$workers" -gt 1 ]; then
	set -- -n "$workers" --dist loadfile "$@"
fi
export UI_TEST_URL="http://127.0.0.1:$api_port/" UI_TEST_DOWN_URL="http://127.0.0.1:$down_port/" UI_TEST_TOKEN="$token" \
	UI_TEST_SERVE_LOG="$work/set0/serve.log"

# keep_logs: each set's serve.log and fakecube.log copied into
# results/logs/set<n>/, for a run whose tests failed, after the retry, so
# that they hold both passes, and before the cleanup removes them with
# the work directory.
keep_logs() {
	n=0
	while [ $n -lt "$workers" ]; do
		mkdir -p "$here/results/logs/set$n"
		cp "$work/set$n/serve.log" "$work/set$n/fakecube.log" "$here/results/logs/set$n/" || true
		n=$((n + 1))
	done
	echo "ui-test: the servers' logs are in tools/ui-test/results/logs/" >&2
}

# A run's logs go before it starts: the first pass empties results/ only
# once a test has started, and a passing run leaves none.
rm -rf "$here/results/logs"
rc=0
PYTEST_ADDOPTS="--durations=20 ${PYTEST_ADDOPTS:-}" "$py" -m pytest --output "$here/results" "$@" || rc=$?
# Only failed tests (exit 1) run again: not an interrupted or empty run.
[ $rc -eq 1 ] || exit $rc
echo "ui-test: tests failed; they run once more, with the video and the trace (tools/ui-test/results/retry/)" >&2
if "$py" -m pytest --last-failed --last-failed-no-failures none --video retain-on-failure --tracing retain-on-failure \
	--output "$here/results/retry" "$@"; then
	echo "ui-test: the failed tests passed the second time, and the run fails all the same: they are flaky" >&2
else
	echo "ui-test: tests failed again; their videos and traces are in tools/ui-test/results/retry/" >&2
fi
keep_logs
exit 1

#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The web UI smoke test (make ui-test): builds massimo, starts the fake
# Cube with the greeting next to this script and the server against it,
# both on loopback ports of their own and in an empty directory (no .env,
# no MASSIMO_* variables but a test API token, so the page goes through
# the token and the stream tickets, and the fake Cube's address and name,
# which serve seeds into its store, massimo.db in that directory), then
# runs the pytest suite next to this script (conftest.py, test_*.py) with
# the page's address in UI_TEST_URL, the token in UI_TEST_TOKEN and the
# server's log in UI_TEST_SERVE_LOG; the
# arguments after "smoke" go to pytest, as do those in PYTEST_ADDOPTS
# (PYTEST_ADDOPTS='-k cubes' make ui-test). Of a failed test it keeps
# the trace, the screenshot and the video under tools/ui-test/results/.
# `run.sh screenshots` (make ui-screenshots) runs screenshots.py instead,
# which takes the screenshot set into docs/screenshots/.
# Without Playwright (and, for the suite, pytest and pytest-playwright)
# it says so and exits 0, so `make ui-test` never fails on a machine that
# cannot run it; UI_TEST_REQUIRED=1, as CI sets it, makes that exit 1.
#
# The fake Cube has a device that joins pairing mode two seconds after
# n:, a copy of the radiator thermostat 0a010d as 0abcde (TEQ0000001),
# and joins again after a t: deleted it, so that every case can pair it.
#
# The fake Cube also answers UDP discovery, as a Cube that is not
# configured (KEQ0000000), on a loopback port the server asks instead of
# broadcasting; the server's own discovery socket is UDP 23272 as always,
# so a search answers 503 where another program holds that port, and
# the suite then skips the tests that need an answer.
#
# A second server, for the page of a Cube that is not connected, serves
# a Cube at a closed loopback port (127.0.0.1:9), Cellar Cube
# (KEQ0000200), from a directory of its own, with the same token; the
# suite gets its address in UI_TEST_DOWN_URL.
#
# Environment: UI_TEST_PYTHON (default tools/ui-test/.venv/bin/python
# when present, else python3), UI_TEST_REQUIRED, UI_TEST_CUBE_PORT
# (62919), UI_TEST_API_PORT (8119), UI_TEST_DISCOVERY_PORT (23919),
# UI_TEST_DOWN_PORT (the second server's, UI_TEST_API_PORT + 1).
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

cube_port=${UI_TEST_CUBE_PORT:-62919}
api_port=${UI_TEST_API_PORT:-8119}
discovery_port=${UI_TEST_DISCOVERY_PORT:-23919}
down_port=${UI_TEST_DOWN_PORT:-$((api_port + 1))}
token=ui-test-token
work=$(mktemp -d)
pids=""
cleanup() {
	for p in $pids; do kill "$p" 2>/dev/null || true; done
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

(cd "$root" && go build -o "$work/massimo" ./cmd/massimo)
cd "$work"
env -i PATH="$PATH" HOME="$HOME" ./massimo fakecube -greeting "$here/greeting.txt" \
	-listen "127.0.0.1:$cube_port" -discovery "127.0.0.1:$discovery_port" \
	-pair 0a010d,0abcde,TEQ0000001 > fakecube.log 2>&1 &
pids="$pids $!"
env -i PATH="$PATH" HOME="$HOME" MASSIMO_API_TOKEN="$token" \
	MASSIMO_CUBE_KEQ0000100_ADDRESS="127.0.0.1:$cube_port" MASSIMO_CUBE_KEQ0000100_NAME="Fake Cube" \
	./massimo serve -proxy off \
	-listen "127.0.0.1:$api_port" -update-status "$work/no-update-status.json" \
	-discover-target "127.0.0.1:$discovery_port" -discover-interval 0 > serve.log 2>&1 &
pids="$pids $!"

url="http://127.0.0.1:$api_port/"
i=0
until curl -sf -H "Authorization: Bearer $token" "${url}api/v1/cubes" 2>/dev/null | grep -q '"connected": *true'; do
	i=$((i + 1))
	if [ $i -gt 50 ]; then
		echo "ui-test: the server did not connect to the fake Cube; its log:" >&2
		cat serve.log >&2
		exit 1
	fi
	sleep 0.2
done

mkdir "$work/down"
cd "$work/down"
env -i PATH="$PATH" HOME="$HOME" MASSIMO_API_TOKEN="$token" \
	MASSIMO_CUBE_KEQ0000200_ADDRESS="127.0.0.1:9" MASSIMO_CUBE_KEQ0000200_NAME="Cellar Cube" \
	../massimo serve -proxy off \
	-listen "127.0.0.1:$down_port" -update-status "$work/no-update-status.json" \
	-discover-target "127.0.0.1:9" -discover-interval 0 > ../serve-down.log 2>&1 &
pids="$pids $!"
cd "$work"
down_url="http://127.0.0.1:$down_port/"
i=0
until curl -sf -H "Authorization: Bearer $token" "${down_url}api/v1/cubes" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ $i -gt 50 ]; then
		echo "ui-test: the second server did not answer; its log:" >&2
		cat serve-down.log >&2
		exit 1
	fi
	sleep 0.2
done

# An operator and a guest with passwords, for the sign-in and the
# preferences tests: user add opens the store beside the running server.
printf 'op-pass\n' | env -i PATH="$PATH" HOME="$HOME" ./massimo -data "$work" user add operator -role operator -password-stdin >/dev/null 2>>serve.log
printf 'guest-pass\n' | env -i PATH="$PATH" HOME="$HOME" ./massimo -data "$work" user add guest1 -role guest -password-stdin >/dev/null 2>>serve.log

if [ "$mode" = screenshots ]; then
	"$py" "$here/screenshots.py" "$url" "$token"
	exit
fi
cd "$here"
UI_TEST_URL="$url" UI_TEST_DOWN_URL="$down_url" UI_TEST_TOKEN="$token" UI_TEST_SERVE_LOG="$work/serve.log" "$py" -m pytest --output "$here/results" "$@"

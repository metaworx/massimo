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
# runs smoke.py with Playwright; `run.sh screenshots` (make ui-screenshots)
# runs screenshots.py instead, which takes the screenshot set into
# docs/screenshots/.
# Without Playwright it says so and exits 0, so `make ui-test` never
# fails on a machine that cannot run it.
#
# The fake Cube also answers UDP discovery, as a Cube that is not
# configured (KEQ0000000), on a loopback port the server asks instead of
# broadcasting; the server's own discovery socket is UDP 23272 as always,
# so a search answers 503 where another program holds that port, and
# smoke.py then skips the checks that need an answer.
#
# Environment: UI_TEST_PYTHON (default tools/ui-test/.venv/bin/python
# when present, else python3), UI_TEST_CUBE_PORT (62919),
# UI_TEST_API_PORT (8119), UI_TEST_DISCOVERY_PORT (23919).
set -eu

case "${1:-smoke}" in
	smoke | screenshots) script=${1:-smoke} ;;
	*) echo "usage: run.sh [smoke|screenshots]" >&2; exit 2 ;;
esac
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
py=${UI_TEST_PYTHON:-}
if [ -z "$py" ]; then
	if [ -x "$here/.venv/bin/python" ]; then py="$here/.venv/bin/python"; else py=python3; fi
fi
if ! "$py" -c 'import playwright.sync_api' 2>/dev/null; then
	echo "ui-test: $script skipped, Playwright for Python is not installed for $py (see tools/ui-test/README.md)"
	exit 0
fi

cube_port=${UI_TEST_CUBE_PORT:-62919}
api_port=${UI_TEST_API_PORT:-8119}
discovery_port=${UI_TEST_DISCOVERY_PORT:-23919}
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
	-listen "127.0.0.1:$cube_port" -discovery "127.0.0.1:$discovery_port" > fakecube.log 2>&1 &
pids="$pids $!"
env -i PATH="$PATH" HOME="$HOME" MASSIMO_API_TOKEN="$token" \
	MASSIMO_CUBE_KEQ0537249_ADDRESS="127.0.0.1:$cube_port" MASSIMO_CUBE_KEQ0537249_NAME="Fake Cube" \
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

"$py" "$here/$script.py" "$url" "$token"

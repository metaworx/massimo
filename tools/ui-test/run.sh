#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The web UI smoke test (make ui-test): builds massimo, starts the fake
# Cube with the greeting next to this script and the server against it,
# both on loopback ports of their own and in an empty directory (no .env,
# no MASSIMO_* variables), then runs smoke.py with Playwright. Without
# Playwright it says so and exits 0, so `make ui-test` never fails on a
# machine that cannot run it.
#
# Environment: UI_TEST_PYTHON (default tools/ui-test/.venv/bin/python
# when present, else python3), UI_TEST_CUBE_PORT (62919),
# UI_TEST_API_PORT (8119).
set -eu

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
py=${UI_TEST_PYTHON:-}
if [ -z "$py" ]; then
	if [ -x "$here/.venv/bin/python" ]; then py="$here/.venv/bin/python"; else py=python3; fi
fi
if ! "$py" -c 'import playwright.sync_api' 2>/dev/null; then
	echo "ui-test: skipped, Playwright for Python is not installed for $py (see tools/ui-test/README.md)"
	exit 0
fi

cube_port=${UI_TEST_CUBE_PORT:-62919}
api_port=${UI_TEST_API_PORT:-8119}
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
	-listen "127.0.0.1:$cube_port" > fakecube.log 2>&1 &
pids="$pids $!"
env -i PATH="$PATH" HOME="$HOME" ./massimo serve -addr "127.0.0.1:$cube_port" -proxy off \
	-listen "127.0.0.1:$api_port" -update-status "$work/no-update-status.json" > serve.log 2>&1 &
pids="$pids $!"

url="http://127.0.0.1:$api_port/"
i=0
until curl -sf "${url}api/v1/cubes" 2>/dev/null | grep -q '"connected": *true'; do
	i=$((i + 1))
	if [ $i -gt 50 ]; then
		echo "ui-test: the server did not connect to the fake Cube; its log:" >&2
		cat serve.log >&2
		exit 1
	fi
	sleep 0.2
done

"$py" "$here/smoke.py" "$url"

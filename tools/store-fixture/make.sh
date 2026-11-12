#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# make.sh [--ref <commit>] [--flavour <flavour>] <tag> [<out>]: the store
# a released massimo writes, as a text dump for TestMigratesReleasedStores
# (internal/store/releases_test.go; docs/store.md, "Migrations").
# make release runs it before the release commit, with --ref HEAD for the
# tag it is about to make: the release commit changes only the changelog,
# so HEAD is the release's code, built under the tag's name. --flavour is
# the build's flavour, by default the tag's own (none: stable).
#
# It builds the tag (or the ref) in a temporary worktree and runs its own
# fakecube, with today's tools/ui-test/greeting.txt (its serials and
# radio addresses are placeholders), and its serve against it, in an
# empty directory on loopback ports of their own: no .env, no MASSIMO_*
# variables but a made-up API token and the fake Cube's address, the
# flags of serve's that this release knows (from its serve -h). Once the
# Cube's devices are known it sends the same requests to every release:
# the settings, a device's setpoint and a room's, a room renamed; from
# v0.8.0 a user and a key (massimo user add, user token) and a request
# with the key; from v0.11.0 a backup by hand. A request the release does
# not take is named on stderr and left out. Then serve stops, the same
# binary's `store backup` makes the copy, and sqlite3 dumps it to <out>
# (default internal/store/testdata/releases/<tag>.sql) under a header
# naming the tag, its commit and the date.
#
# Needs git, go, curl, jq, python3 (for free ports) and sqlite3. Nothing
# leaves the machine: every program listens on 127.0.0.1.
set -euo pipefail

usage="usage: make.sh [--ref <commit>] [--flavour <flavour>] <tag> [<out>]"
ref=""
flavour=""
while [ $# -gt 0 ]; do
	case $1 in
	--ref) ref=${2:?$usage}; shift 2 ;;
	--flavour) flavour=${2:?$usage}; shift 2 ;;
	-*) echo "$usage" >&2; exit 2 ;;
	*) break ;;
	esac
done
tag=${1:?$usage}
root=$(git rev-parse --show-toplevel)
out=${2:-$root/internal/store/testdata/releases/$tag.sql}
ref=${ref:-$tag}
commit=$(git -C "$root" rev-parse --short "$ref^{commit}")
if [ -z "$flavour" ]; then
	flavour=$(printf '%s\n' "$tag" | sed -nE 's/^v[0-9]+\.[0-9]+\.[0-9]+-([a-z][a-z0-9]*)\.[0-9]+$/\1/p')
	flavour=${flavour:-stable}
fi
work=$(mktemp -d)
pids=""

cleanup() {
	for p in $pids; do kill "$p" 2>/dev/null || true; done
	wait 2>/dev/null || true
	git -C "$root" worktree remove --force "$work/src" 2>/dev/null || true
	rm -rf "$work"
}
trap cleanup EXIT

say() { printf 'store-fixture %s: %s\n' "$tag" "$*" >&2; }

# Three free loopback ports: the fake Cube, its discovery, the API.
read -r cube_port disc_port api_port < <(python3 -c '
import socket
s = [socket.socket() for _ in range(3)]
for x in s: x.bind(("127.0.0.1", 0))
print(*[x.getsockname()[1] for x in s])
for x in s: x.close()')

git -C "$root" -c submodule.recurse=false worktree add -q --detach "$work/src" "$commit"
(cd "$work/src" && CGO_ENABLED=0 go build -ldflags "-X main.version=$tag -X main.flavour=$flavour" -o "$work/massimo" ./cmd/massimo)
git -C "$root" worktree remove --force "$work/src"
m=$work/massimo
data=$work/data
mkdir "$data"
token=fixture-token-not-a-secret
serial=KEQ0000100
api=http://127.0.0.1:$api_port/api/v1

run() { (cd "$data" && env -i PATH="$PATH" HOME="$work" "$@"); }

knows() { "$m" serve -h 2>&1 | grep -q -- "^ *-$1[ =]"; }
flags=(-proxy off -listen "127.0.0.1:$api_port")
knows discover-target && flags+=(-discover-target "127.0.0.1:$disc_port")
knows discover-interval && flags+=(-discover-interval 0)
knows discover-listen && flags+=(-discover-listen 127.0.0.1:0)
knows update-status && flags+=(-update-status "$work/no-update-status.json")

run "$m" fakecube -greeting "$root/tools/ui-test/greeting.txt" \
	-listen "127.0.0.1:$cube_port" -discovery "127.0.0.1:$disc_port" > "$work/fakecube.log" 2>&1 &
pids="$pids $!"
(cd "$data" && env -i PATH="$PATH" HOME="$work" MASSIMO_API_TOKEN="$token" \
	MASSIMO_CUBE_${serial}_ADDRESS="127.0.0.1:$cube_port" MASSIMO_CUBE_${serial}_NAME="Fake Cube" \
	"$m" serve "${flags[@]}") > "$work/serve.log" 2>&1 &
serve=$!
pids="$pids $serve"

# call method path [body]: the request with the bootstrap token; prints
# the answer, names a refused one on stderr and returns 1.
call() {
	local code
	code=$(curl -s -o "$work/answer" -w '%{http_code}' -X "$1" \
		-H "Authorization: Bearer ${key:-$token}" -H 'Content-Type: application/json' \
		${3:+--data "$3"} "$api$2") || code=000
	if [ "${code#2}" = "$code" ]; then
		say "$1 $2 not taken ($code)"
		return 1
	fi
	cat "$work/answer"
}

cubejson=""
for _ in $(seq 100); do
	if cubejson=$(call GET "/cubes/$serial" 2>/dev/null) && jq -e '.. | objects | select(has("rf_address"))' <<<"$cubejson" >/dev/null 2>&1; then
		break
	fi
	cubejson=""
	sleep 0.2
done
if [ -z "$cubejson" ]; then
	say "the Cube's devices never showed; serve's log:"
	cat "$work/serve.log" >&2
	exit 1
fi
rf=$(jq -r '[.. | objects | select(has("rf_address")) | select((.type // "") | test("thermostat"))] | .[0].rf_address // empty' <<<"$cubejson")
[ -n "$rf" ] || rf=$(jq -r '[.. | objects | select(has("rf_address"))] | .[1].rf_address' <<<"$cubejson")
room=$(jq -r '[.. | objects | select(has("room_id")) | .room_id | select(. > 0)] | .[0]' <<<"$cubejson")
say "serve $commit on the fake Cube: device $rf, room $room"

call PUT /settings '{"stale_after": "36h"}' >/dev/null || true
call PUT "/cubes/$serial/devices/$rf/setpoint" '{"mode": "manual", "temp": 21.5}' >/dev/null || true
call PUT "/cubes/$serial/rooms/$room/setpoint" '{"mode": "manual", "temp": 20}' >/dev/null || true
call PUT "/cubes/$serial/rooms/$room" '{"name": "Fixture room"}' >/dev/null || true
call POST "/cubes/$serial/backups" '{}' >/dev/null || true

if "$m" user -h 2>&1 | grep -q add; then
	if printf 'fixture-pass-1\n' | run "$m" user add anna -role viewer -password-stdin > /dev/null 2>> "$work/user.log"; then
		if key=$(run "$m" user token anna -label fixture 2>> "$work/user.log" | grep -oE '[A-Za-z0-9_.-]{20,}' | tail -1) && [ -n "$key" ]; then
			call GET "/cubes/$serial" > /dev/null || true
		else
			say "user token not taken"
		fi
		key=""
	else
		say "user add not taken: $(tail -1 "$work/user.log")"
	fi
else
	say "no user commands in this release"
fi

sleep 2 # the writer's batch
kill -TERM "$serve"
for _ in $(seq 50); do kill -0 "$serve" 2>/dev/null || break; sleep 0.2; done
if kill -0 "$serve" 2>/dev/null; then
	say "serve did not stop"
	exit 1
fi

if ! run "$m" store backup "$work/copy.db" > "$work/backup.log" 2>&1; then
	say "store backup failed:"
	cat "$work/backup.log" >&2
	exit 1
fi
mkdir -p "$(dirname "$out")"
{
	printf -- '-- The store that massimo %s (%s) wrote against the fake Cube,\n' "$tag" "$commit"
	printf -- '-- made by tools/store-fixture/make.sh on %s: a text dump\n' "$(date -u +%Y-%m-%d)"
	printf -- '-- (sqlite3 .dump) that TestMigratesReleasedStores loads and migrates.\n'
	sqlite3 "$work/copy.db" .dump
} > "$out.tmp"
mv "$out.tmp" "$out"
say "$(grep -c '^INSERT' "$out") rows in $out"

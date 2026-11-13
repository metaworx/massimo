#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# run.sh - discovery, serve's mDNS announcement and massimo discover on a
# LAN in Docker, against avahi
#
# make lan-test (README.md): builds massimo, brings up the hosts of
# compose.yaml on a bridge network of their own, a fake Cube with a MAC
# of eQ-3's prefix, a searcher, a serve with no Cube, and a client with
# avahi, and checks with no proxy on the LAN
#   1. that a search of the searcher's finds the Cube, its address, its
#      MAC, not configured, and logs it found;
#   2. that, the Cube's cable pulled, the next search lists nothing and
#      logs it gone;
#   3. that, the cable back, a search finds it again and logs it found;
# then brings up the box, a serve beside avahi-daemon whose proxy answers
# discovery for the Cube and which polls it every 5 s, and checks
#   4. that, while the box's proxy answers for the Cube too, a search
#      lists the Cube once, at its own address, the box among its proxies;
#   5. that serve announces itself beside avahi, UDP 5353 shared;
#   6. that avahi-browse resolves the service, its node, version, TLS
#      and fingerprint, on the box and on the client, to the box;
#   7. that massimo discover on the client lists the server, and the two
#      answers of the Cube's serial: the box's marked proxy, the Cube's not;
#   8. that, the Cube's cable pulled again, the box's serve loses it and
#      its proxy stops answering for it: a search lists the serial from
#      nowhere;
#   9. that the server's goodbye takes it out of the client's avahi;
# then, the cable back, brings up the follower, a serve with the Cube by
# its serial alone, which polls it every 5 s, and checks
#  10. that, the Cube moved to another address, the follower loses it,
#      searches, and connects to it at the new address.
# The hosts are taken down whatever happened, their logs printed first
# when a check failed. Needs Docker with Compose v2; exit 1 on a failed
# check, 2 without Docker.
set -euo pipefail
readonly VERSION=1.5.0
readonly SERIAL=KEQ0000100
readonly CUBE_MAC=00:1a:22:00:00:01
readonly BUILD=v0.0.0-lan-test

case "${1:-}" in
	-h | --help)
		sed -n '4,36p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	--version)
		echo "run.sh $VERSION"
		exit 0
		;;
	"") ;;
	*)
		echo "run.sh: no arguments, see --help" >&2
		exit 2
		;;
esac

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
if ! docker compose version >/dev/null 2>&1; then
	echo "lan-test: needs Docker with Compose v2 (tools/lan-test/README.md)" >&2
	exit 2
fi

work="$(mktemp -d)"
export LAN_TEST_WORK="$work"
export COMPOSE_PROJECT_NAME="massimo-lan-$$"
compose() { docker compose -f "$here/compose.yaml" "$@"; }

finish() {
	local rc=$?
	if [ "$rc" != 0 ]; then
		for s in cube searcher box client follower; do
			echo "--- $s" >&2
			compose logs --no-color "$s" >&2 2>&1 || true
		done
	fi
	compose down --volumes --remove-orphans --timeout 2 >/dev/null 2>&1 || true
	rm -rf -- "${work:?}"
	exit "$rc"
}
trap finish EXIT

ok() { echo "ok    $*"; }
fail() {
	echo "FAIL  $*" >&2
	exit 1
}
# within tries a check until it passes, for up to $1 seconds
within() {
	local end=$((SECONDS + $1))
	shift
	until "$@"; do
		[ "$SECONDS" -lt "$end" ] || return 1
		sleep 0.5
	done
}

(cd "$root" && CGO_ENABLED=0 go build -ldflags "-X main.version=$BUILD" -o "$work/massimo" ./cmd/massimo)
cp "$root/tools/ui-test/greeting.txt" "$work/"
compose build --quiet box >/dev/null 2>&1 || compose build box
# The box comes up at check 4: until then no proxy answers for the Cube.
compose up --detach --quiet-pull cube searcher client >/dev/null 2>&1 || compose up --detach cube searcher client
ip_of() { docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$(compose ps -q "$1")"; }
cube_ip="$(ip_of cube)"

# search runs one search of the searcher's, through its API, and sets
# answers to one line per found Cube: the serial, the address, the MAC,
# whether it is configured, and its proxies (- for none).
answers=""
search() {
	answers="$(compose exec -T searcher curl -sf -X POST http://127.0.0.1:8099/api/v1/cubes/discover | python3 -I -c '
import json, sys
for f in json.load(sys.stdin).get("found") or []:
    print(f.get("serial"), f.get("address"), (f.get("mac") or "-").lower(), str(f.get("configured")).lower(), ",".join(f.get("proxies") or []) or "-")
' 2>/dev/null)"
}
cube_listed() { grep -q "^$SERIAL $cube_ip $CUBE_MAC false " <<<"$answers"; }
finds_cube() { search && cube_listed; }
# has reports whether its input holds a line matching the pattern. It
# reads all of it: grep -q stops at the first match, and the log still
# being written into the pipe then dies of SIGPIPE, which pipefail makes
# the pipe's failure although the line was there.
has() { grep -- "$1" >/dev/null; }
searcher_log() { compose logs --no-color --no-log-prefix searcher 2>&1; }
gone_lines() { searcher_log | grep -c -- "msg=\"cube gone from the LAN\" source=cube kind=cube_gone serial=$SERIAL ip=$cube_ip " || true; }
found_lines() { searcher_log | grep -c -- "msg=\"cube found on the LAN\" source=cube kind=cube_found serial=$SERIAL ip=$cube_ip mac=$CUBE_MAC" || true; }
logged_found() { [ "$(found_lines)" -ge "$1" ]; }

# 1. A search finds the Cube, with no proxy on the LAN.
within 30 finds_cube || fail "the searcher's search does not find $SERIAL at $cube_ip ($CUBE_MAC), not configured: ${answers:-nothing}"
within 5 logged_found 1 || fail "the searcher does not log $SERIAL found at $cube_ip"
ok "a search finds $SERIAL at $cube_ip, its MAC $CUBE_MAC, not configured, and the searcher logs it found"

# 2. The cable pulled: the next search lists nothing.
compose exec -T cube ip link set eth0 down
search || fail "the searcher's search failed"
if grep -q "^$SERIAL " <<<"$answers"; then
	fail "the first search after the Cube's cable was pulled still lists it: $answers"
fi
gone_logged() { [ "$(gone_lines)" -ge 1 ]; }
within 5 gone_logged || fail "the searcher does not log $SERIAL gone"
ok "the Cube's cable pulled, the next search lists nothing, and the searcher logs it gone"

# 3. The cable back: found again.
compose exec -T cube ip link set eth0 up
within 30 finds_cube || fail "the searcher's search does not find $SERIAL again: ${answers:-nothing}"
within 5 logged_found 2 || fail "the searcher does not log $SERIAL found again"
ok "the cable back, a search finds $SERIAL again, and the searcher logs it found once more"

# 4. The box's proxy answers too: one serial from two addresses, which
# the searcher lists as one Cube at its own address, the box among its
# proxies. The client's massimo discover shows the box's answer first.
compose up --detach box >/dev/null 2>&1 || compose up --detach box
box_ip="$(ip_of box)"
box_answers() { compose exec -T client /m/massimo discover -json -wait 2s 2>/dev/null | grep -q "\"ip\": \"$box_ip\""; }
within 30 box_answers || fail "the box's proxy does not answer the client's discover for $SERIAL"
once() { search && cube_listed && [ "$(grep -c "^$SERIAL " <<<"$answers")" -eq 1 ] && grep -qE "^$SERIAL $cube_ip [^ ]+ false ([^ ]*,)?$box_ip(,|$)" <<<"$answers"; }
within 10 once || fail "the searcher's search does not list $SERIAL once, at $cube_ip, with the box's $box_ip among its proxies: ${answers:-nothing}"
ok "with the box's proxy up, a search lists $SERIAL once, at the Cube's $cube_ip, with the box's $box_ip among its proxies"

# 5. serve announces itself beside avahi-daemon.
box_log() { compose logs --no-color box 2>&1; }
announced() { box_log | has 'msg="announced on the LAN"'; }
within 30 announced || fail "the box's serve did not announce itself"
if box_log | has 'not announced'; then
	fail "the box's log: $(box_log | grep 'not announced')"
fi
node="$(box_log | sed -n 's/.*msg="api listening".* node=\([0-9a-f-]*\).*/\1/p' | head -n1)"
[ -n "$node" ] || fail "no node in the box's log"
ok "serve announces itself beside avahi-daemon on the box, UDP 5353 shared (node $node)"

# 6. avahi-browse resolves it, on the box and on the client.
resolved() { compose exec -T "$1" avahi-browse -rpt _massimo._tcp 2>/dev/null | grep '^=' | grep ';IPv4;' | head -n1; }
seen() { [ -n "$(resolved "$1")" ]; }
for host in box client; do
	within 30 seen "$host" || fail "avahi-browse on the $host does not resolve _massimo._tcp"
	line="$(resolved "$host")"
	for want in "\"node=$node\"" "\"v=$BUILD\"" '"tls=1"' '"fp='; do
		case "$line" in *"$want"*) ;; *) fail "avahi-browse on the $host: no $want in $line" ;; esac
	done
	case "$line" in *";$box_ip;8099;"*) ;; *) fail "avahi-browse on the $host: not $box_ip:8099 in $line" ;; esac
	ok "avahi-browse on the $host resolves the server at $box_ip:8099 with its node, version, TLS and fingerprint"
done

# 7. massimo discover on the client: the server, and the proxy marked.
json="$(compose exec -T client /m/massimo discover -json -wait 4s 2>/dev/null)"
python3 -I - "$json" "$node" "$box_ip" "$cube_ip" "$SERIAL" "$CUBE_MAC" <<'EOF' || fail "massimo discover on the client: $json"
import json, sys
text, node, box, cube, serial, mac = sys.argv[1:7]
d = json.loads(text[text.index("{"):])
if not any(s.get("node") == node for s in d.get("servers") or []):
    sys.exit(f"no server of node {node}")
answers = {c.get("ip"): c for c in d.get("cubes") or [] if c.get("serial") == serial}
if answers.get(box, {}).get("proxy") is not True:
    sys.exit(f"the box's answer at {box} is not marked proxy: {answers}")
c = answers.get(cube, {})
if c.get("proxy") is not False or (c.get("mac") or "").lower() != mac:
    sys.exit(f"the Cube's answer at {cube} is marked proxy, or not of MAC {mac}: {answers}")
EOF
ok "massimo discover on the client lists the server, the box's answer marked proxy and the Cube's ($CUBE_MAC) not"

# 8. The cable pulled with the box up: the box's serve takes the Cube
# for lost after two unanswered polls, 5 s apart, and 10 s, and its
# proxy answers no search for it.
compose exec -T cube ip link set eth0 down
unlisted() { search && ! grep -q "^$SERIAL " <<<"$answers"; }
within 45 unlisted || fail "the Cube's cable pulled, a search still lists $SERIAL: $answers"
box_log | has 'msg="cube connection lost"' || fail "the box's serve does not log the Cube lost"
ok "the Cube's cable pulled again, the box's serve loses it, and a search no longer lists $SERIAL from its proxy"

# 9. The goodbye takes the server out of the client's avahi.
compose exec -T box pkill -TERM -f '/m/massimo -data' || fail "serve not stopped on the box"
gone() { [ -z "$(resolved client)" ]; }
within 30 gone || fail "the client's avahi still lists the server after its goodbye"
ok "the server's goodbye takes it out of the client's avahi"

# 10. The follower, a serve with the Cube by its serial alone, finds it,
# and follows it to another address on the bridge: it takes the Cube for
# lost by its polls, searches at the loss, and connects at the new one.
compose exec -T cube ip link set eth0 up
compose up --detach follower >/dev/null 2>&1 || compose up --detach follower
# follower_at prints the follower's Cube: its address and whether it is
# connected.
follower_at() {
	compose exec -T follower curl -sf http://127.0.0.1:8099/api/v1/cubes 2>/dev/null | python3 -I -c '
import json, sys
for c in json.load(sys.stdin) or []:
    if c.get("configured"):
        print(c.get("address"), str(c.get("connected")).lower())
' 2>/dev/null
}
connected_at() { [ "$(follower_at)" = "$1:62910 true" ]; }
within 30 connected_at "$cube_ip" || fail "the follower is not connected to $SERIAL at $cube_ip: $(follower_at)"
# Another address in the bridge's subnet, its last but one.
subnet="$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}}{{end}}' "${COMPOSE_PROJECT_NAME}_lan")"
moved_ip="$(python3 -I -c 'import ipaddress, sys; print(ipaddress.ip_network(sys.argv[1]).broadcast_address - 1)' "$subnet")"
compose exec -T cube sh -c "ip addr flush dev eth0 && ip addr add $moved_ip/${subnet#*/} brd + dev eth0"
within 90 connected_at "$moved_ip" || fail "the follower did not follow $SERIAL to $moved_ip: $(follower_at)"
follower_log() { compose logs --no-color --no-log-prefix follower 2>&1; }
follower_log | has 'msg="cube connection lost"' || fail "the follower does not log the Cube lost"
follower_log | has "msg=\"cube address from discovery\".* from=$cube_ip to=$moved_ip" || fail "the follower does not log the Cube's new address"
ok "the Cube moved to $moved_ip, the follower loses it, searches, and connects at the new address"

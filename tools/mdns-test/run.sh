#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# run.sh - serve's mDNS announcement and massimo discover against avahi
#
# make mdns-test (README.md): builds massimo, brings up the three hosts of
# compose.yaml on a bridge network of their own, a fake Cube with a MAC
# of eQ-3's prefix, a box running serve beside avahi-daemon and a client
# with avahi of its own, and checks
#   1. that serve announces itself beside avahi, UDP 5353 shared;
#   2. that avahi-browse resolves the service, its node, version,
#      TLS and fingerprint, on the box and on the client, to the box;
#   3. that massimo discover on the client lists the server, and the two
#      answers of the Cube's serial: the box's marked proxy, the Cube's not;
#   4. that the server's goodbye takes it out of the client's avahi.
# The hosts are taken down whatever happened, their logs printed first
# when a check failed. Needs Docker with Compose v2; exit 1 on a failed
# check, 2 without Docker.
set -euo pipefail
readonly VERSION=1.0.0
readonly SERIAL=KEQ0000100
readonly CUBE_MAC=00:1a:22:00:00:01
readonly BUILD=v0.0.0-mdns-test

case "${1:-}" in
	-h | --help)
		sed -n '4,20p' "$0" | sed 's/^# \{0,1\}//'
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
	echo "mdns-test: needs Docker with Compose v2 (tools/mdns-test/README.md)" >&2
	exit 2
fi

work="$(mktemp -d)"
export MDNS_TEST_WORK="$work"
export COMPOSE_PROJECT_NAME="massimo-mdns-$$"
compose() { docker compose -f "$here/compose.yaml" "$@"; }

finish() {
	local rc=$?
	if [ "$rc" != 0 ]; then
		for s in box cube client; do
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
# within tries a check, up to 30 s
within() {
	for _ in $(seq 60); do
		if "$@"; then return 0; fi
		sleep 0.5
	done
	return 1
}

(cd "$root" && CGO_ENABLED=0 go build -ldflags "-X main.version=$BUILD" -o "$work/massimo" ./cmd/massimo)
cp "$root/tools/ui-test/greeting.txt" "$work/"
compose up --build --detach --quiet-pull >/dev/null 2>&1 || compose up --build --detach
ip_of() { docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$(compose ps -q "$1")"; }
box_ip="$(ip_of box)"
cube_ip="$(ip_of cube)"

# 1. serve announces itself beside avahi-daemon.
box_log() { compose logs --no-color box 2>&1; }
announced() { box_log | grep -q 'msg="announced on the LAN"'; }
within announced || fail "the box's serve did not announce itself"
if box_log | grep -q 'not announced'; then
	fail "the box's log: $(box_log | grep 'not announced')"
fi
node="$(box_log | sed -n 's/.*msg="api listening".* node=\([0-9a-f-]*\).*/\1/p' | head -n1)"
[ -n "$node" ] || fail "no node in the box's log"
ok "serve announces itself beside avahi-daemon on the box, UDP 5353 shared (node $node)"

# 2. avahi-browse resolves it, on the box and on the client.
resolved() { compose exec -T "$1" avahi-browse -rpt _massimo._tcp 2>/dev/null | grep '^=' | grep ';IPv4;' | head -n1; }
seen() { [ -n "$(resolved "$1")" ]; }
for host in box client; do
	within seen "$host" || fail "avahi-browse on the $host does not resolve _massimo._tcp"
	line="$(resolved "$host")"
	for want in "\"node=$node\"" "\"v=$BUILD\"" '"tls=1"' '"fp='; do
		case "$line" in *"$want"*) ;; *) fail "avahi-browse on the $host: no $want in $line" ;; esac
	done
	case "$line" in *";$box_ip;8099;"*) ;; *) fail "avahi-browse on the $host: not $box_ip:8099 in $line" ;; esac
	ok "avahi-browse on the $host resolves the server at $box_ip:8099 with its node, version, TLS and fingerprint"
done

# 3. massimo discover on the client: the server, and the proxy marked.
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

# 4. The goodbye takes the server out of the client's avahi.
compose exec -T box pkill -TERM -f '/m/massimo -data' || fail "serve not stopped on the box"
gone() { [ -z "$(resolved client)" ]; }
within gone || fail "the client's avahi still lists the server after its goodbye"
ok "the server's goodbye takes it out of the client's avahi"

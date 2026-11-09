#!/usr/bin/env bash
# massimo install on the template's settings, as a new Mac gets it: CI's
# macOS job runs it before its own install, on a runner whose sudo asks
# no password; on another Mac run it where sudo is at hand and massimo is
# not installed. The service is left for its owner, with the message;
# its owner line, run as printed from a directory the service user cannot
# enter, creates the owner; install again restarts a loaded job and
# leaves a booted-out one; the web UI's way; a removal that leaves
# nothing. It stops at once on a Mac with massimo installed, and removes
# what it installed when it ends, also on a failure.
#
# usage: template-test.sh <massimo binary>
set -uo pipefail

bin=${1:?usage: template-test.sh <massimo binary>}
case "$bin" in /*) ;; *) bin=$PWD/$bin ;; esac
label=ch.metaworx.massimo
plist=/Library/LaunchDaemons/$label.plist
work=$(mktemp -d)
out=$work/out
failn=0

if [ -e "$plist" ] || [ -e /usr/local/bin/massimo ] || [ -e /etc/massimo ]; then
	echo "massimo is installed on this Mac: stopping without a change" >&2
	exit 1
fi
trap 'sudo "$bin" install -remove -purge >/dev/null 2>&1; rm -rf "$work"' EXIT

ok() { echo "  ok   $*"; }
bad() {
	echo "  FAIL $*"
	failn=$((failn + 1))
}
check() {
	local what=$1
	shift
	if "$@"; then ok "$what"; else bad "$what"; fi
}
run() {
	echo "+ $*"
	"$@" >"$out" 2>&1
	local rc=$?
	sed 's/^/    | /' "$out"
	return $rc
}
inst() {
	run sudo "$bin" install
	cp "$out" "$work/install-out"
}
message() { grep -q '^not started' "$work/install-out"; }
no_message() { ! message; }
loaded() { sudo launchctl print "system/$1" >/dev/null 2>&1; }
not_loaded() { ! loaded "$1"; }
pid() { sudo launchctl print "system/$label" 2>/dev/null | awk '$1 == "pid" { print $3 }'; }
up() {
	for _ in $(seq 1 60); do
		curl -fsS -m 1 -o "$work/hz" http://127.0.0.1:8099/healthz 2>/dev/null && return 0
		sleep 0.5
	done
	return 1
}
down() {
	for _ in $(seq 1 40); do
		curl -s -m 1 -o /dev/null http://127.0.0.1:8099/healthz || return 0
		sleep 0.5
	done
	return 1
}
code() { curl -s -m 3 -o /dev/null -w '%{http_code}' "$@"; }
started() { up && sed -n 's/.*"started_at": *"\([^"]*\)".*/\1/p' "$work/hz"; }
mode() { stat -f '%Lp %Su:%Sg' "$1" 2>/dev/null; }
bootout() {
	run sudo launchctl bootout "system/$label"
	for _ in $(seq 1 40); do
		loaded "$label" || break
		sleep 0.25
	done
	down
}
line() { sed -n 's/^create the owner first: //p' "$work/install-out" | tr -d '\r' | head -1; }
envset() { sudo sh -c "{ $1; } >/etc/massimo/env"; }
api=http://127.0.0.1:8099/api/v1

echo "1. A fresh install, on the template"
inst
check "the message is shown" message
check "not loaded" not_loaded "$label"
check "the update helper is loaded, to run on a request" loaded ch.metaworx.massimo-update
check "/var/lib/massimo is $(mode /var/lib/massimo)" [ "$(mode /var/lib/massimo)" = "750 _massimo:_massimo" ]
check "nothing answers on 8099" down
check "the owner line: $(line)" [ "$(line)" = "sudo -u _massimo /usr/local/bin/massimo user add <name> -role owner" ]

echo "2. The owner first, the message's line as printed, from a directory _massimo cannot enter"
home=$work/home
mkdir -m 750 "$home"
owner_line=$(line)
(cd "$home" && printf 'test-password-1\n' | run eval "${owner_line/<name>/anna} -password-stdin")
check "it creates the owner" grep -q 'added with the role owner' "$out"
run sudo launchctl bootstrap system "$plist"
check "loaded" loaded "$label"
up
check "the API asks for a sign-in ($(code "$api/cubes"))" [ "$(code "$api/cubes")" = 401 ]

echo "3. Loaded on the template: install again restarts it"
p1=$(pid)
s1=$(started)
sleep 1.1
inst
s2=$(started)
check "restarted (PID $p1 -> $(pid)), no message" [ -n "$(pid)" ] && [ "$(pid)" != "$p1" ] && [ "$s1" != "$s2" ] && no_message

echo "4. Booted out on the template: left stopped"
bootout
inst
check "not loaded, the message" not_loaded "$label" && message
check "nothing answers on 8099" down

echo "5. An older template's comments: still the template"
sudo cp -p /etc/massimo/env /etc/massimo/env.saved
envset "echo '# A comment of an older template.'; cat /etc/massimo/env.saved; echo"
inst
check "not loaded, the message" not_loaded "$label" && message

echo "6. A setting of its own: started"
envset "cat /etc/massimo/env.saved; echo MASSIMO_ANNOUNCE=off"
inst
check "loaded, no message" loaded "$label" && no_message
check "it answers" up
bootout
envset "cat /etc/massimo/env.saved"
sudo rm /etc/massimo/env.saved

echo "7. The web UI's way: install, start, the first owner from the form's route, install again"
run sudo "$bin" install -remove -purge
inst
check "the message is shown" message
run sudo launchctl bootstrap system "$plist"
up
check "an open server answers the Mac itself ($(code "$api/cubes"))" [ "$(code "$api/cubes")" = 200 ]
c=$(code -X POST -H 'Content-Type: application/json' -d '{"name":"anna","role":"owner","password":"test-password-1"}' "$api/users")
check "the first owner from the form's route ($c)" [ "$c" = 201 ]
check "then closed ($(code "$api/cubes"))" [ "$(code "$api/cubes")" = 401 ]
p1=$(pid)
inst
up
check "install again restarts it (PID $p1 -> $(pid)), no message" [ "$(pid)" != "$p1" ] && no_message

echo "8. Remove and purge: nothing left"
run sudo "$bin" install -remove -purge
for l in ch.metaworx.massimo ch.metaworx.massimo-update ch.metaworx.massimo-reboot; do
	check "$l not loaded, its plist gone" [ ! -e "/Library/LaunchDaemons/$l.plist" ] && not_loaded "$l"
done
for p in /usr/local/bin/massimo /etc/massimo /var/lib/massimo /var/lib/massimo-update /var/log/massimo; do
	check "$p is gone" [ ! -e "$p" ]
done
check "the user _massimo is gone" sh -c '! dscl . -read /Users/_massimo >/dev/null 2>&1'
check "no firewall entry left" sh -c '! /usr/libexec/ApplicationFirewall/socketfilterfw --listapps 2>/dev/null | grep -q /usr/local/bin/massimo'
check "nothing answers on 8099" down

if [ "$failn" -gt 0 ]; then
	echo "template test: $failn check(s) failed" >&2
	sudo tail -20 /var/log/massimo/serve.log 2>/dev/null
	exit 1
fi
echo "template test: ok"

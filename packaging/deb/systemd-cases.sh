#!/usr/bin/env bash
# One case of systemd-test.sh, run as root inside its container, systemd
# the first process: /dist holds the packages and the linux-amd64 binary
# that `make packages` built, /old/old.deb the v0.12.0-alpha.1 package,
# the last whose account was massimo. Prints a line per check and exits
# non-zero when one fails.
#
# usage: systemd-cases.sh fresh|upgrade|person|takenover|install
set -uo pipefail
export DEBIAN_FRONTEND=noninteractive

new=$(ls -t /dist/massimo_*_amd64.deb | head -1)
old=/old/old.deb
failn=0
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

# inst <args>: apt-get install, its output in /tmp/out without the
# carriage returns apt writes. Without --allow-downgrades: every case
# goes from v0.12.0-alpha.1 up, and a build between tags sorts after the
# tag it follows (the Makefile's DEBVER), so apt refusing a downgrade here
# means a package that sorts wrong.
inst() {
	apt-get -qq install -y "$@" 2>&1 | tr -d '\r' >/tmp/out
	[ "${PIPESTATUS[0]}" -eq 0 ] || {
		echo "  apt failed:"
		sed 's/^/    | /' /tmp/out
	}
}
purge() { apt-get -qq purge -y massimo >/dev/null 2>&1; }
state() { systemctl is-active massimo.service 2>/dev/null; }
pid() { systemctl show -p MainPID --value massimo.service; }
procuser() { ps -o user= -p "$(pid)" 2>/dev/null | tr -d ' '; }
up() {
	for _ in $(seq 1 60); do
		curl -fsS -o /tmp/hz http://127.0.0.1:8099/healthz 2>/dev/null && return 0
		sleep 0.5
	done
	return 1
}
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
version() { up && sed -n 's/.*"version": *"massimo \([^ "]*\).*/\1/p' /tmp/hz; }
mode() { stat -c '%a %U:%G' "$1" 2>/dev/null; }
api=http://127.0.0.1:8099/api/v1
first_owner() {
	code -X POST -H 'Content-Type: application/json' \
		-d '{"name":"anna","role":"owner","password":"test-password-1"}' "$api/users"
}
login() {
	code -X POST -H 'Content-Type: application/json' \
		-d '{"user":"anna","password":"test-password-1"}' "$api/login"
}
message() { grep -q 'massimo: not started' /tmp/out; }
no_message() { ! message; }
person() { useradd -m -c 'Massimo Rossi' -s /bin/bash massimo; }

case "${1:-}" in
fresh)
	inst "$new"
	check "the account is _massimo" sh -c 'getent passwd _massimo >/dev/null'
	check "no account massimo" sh -c '! getent passwd massimo >/dev/null'
	check "not started ($(state))" [ "$(state)" = inactive ]
	line=$(sed -n 's/^massimo: create the owner first: //p' /tmp/out)
	check "the owner line: $line" [ "$line" = "sudo -u _massimo /usr/bin/massimo user add <name> -role owner" ]
	check "/var/lib/massimo is $(mode /var/lib/massimo)" [ "$(mode /var/lib/massimo)" = "750 _massimo:_massimo" ]
	# The line as printed, from a home directory the service user cannot
	# enter, as its owner would run it.
	useradd -m -s /bin/bash owner && chmod 700 /home/owner
	run=${line/<name>/anna}
	(cd /home/owner && printf 'test-password-1\n' | eval "$run -password-stdin") >/tmp/ua 2>&1
	check "it creates the owner from /home/owner" grep -q 'added with the role owner' /tmp/ua
	systemctl start massimo.service
	up
	check "started as $(procuser)" [ "$(state)" = active -a "$(procuser)" = _massimo ]
	check "the API asks for a sign-in ($(code "$api/cubes"))" [ "$(code "$api/cubes")" = 401 ]
	check "/var/lib/massimo after the start: $(mode /var/lib/massimo)" [ "$(mode /var/lib/massimo)" = "750 _massimo:_massimo" ]
	# The log: the admin group's to read, in JSON.
	check "/var/log/massimo after the start: $(mode /var/log/massimo)" [ "$(mode /var/log/massimo)" = "2750 _massimo:adm" ]
	check "serve.log is $(mode /var/log/massimo/serve.log)" [ "$(mode /var/log/massimo/serve.log)" = "640 _massimo:adm" ]
	check "serve.log is JSON" sh -c 'head -n 1 /var/log/massimo/serve.log | grep -q "^{"'
	p1=$(pid)
	inst --reinstall "$new"
	up
	check "running, reinstalled: restarted (PID $p1 -> $(pid)), no message" [ "$(pid)" != "$p1" -a "$(state)" = active ] && no_message
	check "/var/lib/massimo after the restart: $(mode /var/lib/massimo)" [ "$(mode /var/lib/massimo)" = "750 _massimo:_massimo" ]
	systemctl stop massimo.service
	inst --reinstall "$new"
	check "stopped, reinstalled: left stopped ($(state)), the message" [ "$(state)" = inactive ] && message
	purge
	check "the purge takes the account and the files" sh -c '! getent passwd _massimo >/dev/null && [ ! -e /etc/massimo ] && [ ! -e /var/lib/massimo ]'
	;;
upgrade)
	inst "$old"
	uid=$(id -u massimo)
	systemctl start massimo.service
	up
	check "v0.12.0-alpha.1 runs as $(procuser), the owner made ($(first_owner))" [ "$(procuser)" = massimo ]
	v1=$(version)
	p1=$(pid)
	inst "$new"
	up
	v2=$(version)
	check "renamed in place: _massimo has the ID $uid" [ "$(id -u _massimo 2>/dev/null)" = "$uid" ]
	check "no account and no group massimo" sh -c '! getent passwd massimo >/dev/null && ! getent group massimo >/dev/null'
	check "restarted on the new binary as $(procuser): $v1 -> $v2" [ "$(pid)" != "$p1" -a "$(procuser)" = _massimo -a "$v2" != "$v1" ]
	check "the store and the settings are _massimo's" [ "$(stat -c %U /var/lib/massimo/massimo.db)" = _massimo -a "$(stat -c %U /etc/massimo/env)" = _massimo ]
	check "the owner signs in ($(login)), the API closed otherwise" [ "$(login)" = 200 -a "$(code "$api/cubes")" = 401 ]
	check "/var/lib/massimo is $(mode /var/lib/massimo)" [ "$(mode /var/lib/massimo)" = "750 _massimo:_massimo" ]
	;;
person)
	person
	before=$(getent passwd massimo)
	inst "$new"
	check "the person is untouched" [ "$(getent passwd massimo)" = "$before" ]
	check "_massimo beside the person" sh -c 'getent passwd _massimo >/dev/null'
	check "the settings are _massimo's" [ "$(stat -c %U /etc/massimo/env)" = _massimo ]
	systemctl start massimo.service
	up
	check "the service runs as $(procuser)" [ "$(procuser)" = _massimo ]
	purge
	check "the purge keeps the person and takes _massimo" sh -c "[ \"\$(getent passwd massimo)\" = '$before' ] && ! getent passwd _massimo >/dev/null"
	;;
takenover)
	person
	before=$(getent passwd massimo)
	inst "$old"
	systemctl start massimo.service
	up
	check "v0.12.0-alpha.1 took the person over ($(procuser))" [ "$(procuser)" = massimo ]
	inst "$new"
	chown=$(sed -n 's/^massimo: hand the service its files: //p' /tmp/out)
	check "the upgrade names the chown" [ "$chown" = "sudo chown -R _massimo:_massimo /etc/massimo /var/lib/massimo /var/log/massimo" ]
	eval "${chown#sudo }"
	systemctl restart massimo.service
	up
	check "after the chown as printed and a restart: $(state), as $(procuser)" [ "$(state)" = active -a "$(procuser)" = _massimo ]
	check "the person is untouched" [ "$(getent passwd massimo)" = "$before" ]
	;;
install)
	mkdir -p /tmp/oldx && dpkg-deb -x "$old" /tmp/oldx
	cp /tmp/oldx/usr/bin/massimo /tmp/massimo-old
	cp /dist/massimo-linux-amd64 /tmp/massimo-new
	cd /tmp || exit 1
	MASSIMO_ENV_FILE=/dev/null ./massimo-old install >/tmp/out 2>&1 || { echo "  v0.12 install failed"; cat /tmp/out; }
	uid=$(id -u massimo)
	systemctl start massimo.service
	up
	check "v0.12.0-alpha.1's hand install runs as $(procuser), the owner made ($(first_owner))" [ "$(procuser)" = massimo ]
	p1=$(pid)
	MASSIMO_ENV_FILE=/dev/null ./massimo-new install >/tmp/out 2>&1 || { echo "  install failed"; sed 's/^/    | /' /tmp/out; }
	up
	check "renamed in place: _massimo has the ID $uid" [ "$(id -u _massimo 2>/dev/null)" = "$uid" ]
	check "no account massimo" sh -c '! getent passwd massimo >/dev/null'
	check "restarted as $(procuser) (PID $p1 -> $(pid))" [ "$(pid)" != "$p1" -a "$(procuser)" = _massimo ]
	check "the owner signs in ($(login))" [ "$(login)" = 200 ]
	MASSIMO_ENV_FILE=/dev/null ./massimo-new install -remove -purge >/tmp/out 2>&1
	check "-remove -purge takes _massimo and the files" sh -c '! getent passwd _massimo >/dev/null && [ ! -e /var/lib/massimo ]'
	person
	before=$(getent passwd massimo)
	MASSIMO_ENV_FILE=/dev/null ./massimo-new install >/tmp/out 2>&1
	check "beside a person: the person untouched, _massimo made" [ "$(getent passwd massimo)" = "$before" ] && getent passwd _massimo >/dev/null
	MASSIMO_ENV_FILE=/dev/null ./massimo-new install -remove -purge >/tmp/out 2>&1
	check "-remove -purge keeps the person" [ "$(getent passwd massimo)" = "$before" ]
	;;
*)
	echo "usage: $0 fresh|upgrade|person|takenover|install" >&2
	exit 2
	;;
esac
[ "$failn" -eq 0 ]

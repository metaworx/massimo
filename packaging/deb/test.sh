#!/usr/bin/env bash
# The Debian package installed, checked and purged in a Debian container,
# as root: `make deb-test` and CI's "Debian package" job run it with the
# packages mounted at /dist. There is no systemd in a container, so the
# enable goes through deb-systemd-helper's symlinks and nothing starts;
# the start on a real box is the owner's check.
set -euo pipefail

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

debs=(/dist/massimo_*_amd64.deb)
deb=${debs[0]}
[ -f "$deb" ] || fail "no /dist/massimo_*_amd64.deb"
# The version in the file name is the package's own, as dpkg reads it.
name=${deb##*/}
name=${name#massimo_}
[ "$(dpkg-deb -f "$deb" Version)" = "${name%_amd64.deb}" ] ||
	fail "$deb holds the version $(dpkg-deb -f "$deb" Version)"
# The DNS-01 hooks' tools, suggested and not installed.
[ "$(dpkg-deb -f "$deb" Suggests)" = "curl, jq" ] ||
	fail "$deb suggests $(dpkg-deb -f "$deb" Suggests)"
export DEBIAN_FRONTEND=noninteractive
apt-get -qq update
apt-get -qq install -y "$deb" >/dev/null

getent passwd massimo >/dev/null || fail "no user massimo"
getent group massimo >/dev/null || fail "no group massimo"
[ "$(stat -c %a:%U:%G /etc/massimo)" = 750:massimo:massimo ] || fail "/etc/massimo is $(stat -c %a:%U:%G /etc/massimo)"
[ "$(stat -c %a:%U:%G /etc/massimo/env)" = 600:massimo:massimo ] || fail "/etc/massimo/env is $(stat -c %a:%U:%G /etc/massimo/env)"
cmp -s /etc/massimo/env /usr/share/massimo/env.template || fail "/etc/massimo/env is not the template"
[ "$(stat -c %a:%U:%G /var/lib/massimo)" = 750:massimo:massimo ] || fail "/var/lib/massimo is $(stat -c %a:%U:%G /var/lib/massimo)"
unit=/usr/lib/systemd/system/massimo.service
grep -q '^ExecStart=/usr/bin/massimo ' "$unit" || fail "the unit does not run /usr/bin/massimo"
grep '^ExecStart=' "$unit" | grep -q -- ' -update-request /var/lib/massimo/update-request' ||
	fail "the unit lacks -update-request for the update helper"
[ -L /etc/systemd/system/multi-user.target.wants/massimo.service ] || fail "the unit is not enabled"
for u in massimo-update.path massimo-update.service massimo-reboot.path massimo-reboot.service; do
	[ -f "/usr/lib/systemd/system/$u" ] || fail "no $u"
done
# The update helper runs apt, and is enabled; the reboot helper is not.
grep -q '^ExecStart=/usr/bin/massimo update -request /var/lib/massimo/update-request ' /usr/lib/systemd/system/massimo-update.service ||
	fail "the update helper does not run /usr/bin/massimo update"
if grep -q '^ProtectSystem' /usr/lib/systemd/system/massimo-update.service; then fail "the update helper cannot run dpkg"; fi
[ -L /etc/systemd/system/paths.target.wants/massimo-update.path ] || fail "the update helper is not enabled"
[ ! -e /etc/systemd/system/paths.target.wants/massimo-reboot.path ] || fail "the reboot helper is enabled"
[ "$(head -n 1 /usr/share/massimo/packaged)" = apt ] || fail "the marker does not say apt"
[ -f /usr/share/doc/massimo/copyright ] || fail "no copyright file"
# The DNS-01 hooks, for -acme-dns-hook; the purge below takes them along.
for hook in infomaniak cloudflare; do
	f=/usr/share/massimo/acme-dns-hooks/$hook.sh
	[ "$(stat -c %a:%U:%G "$f")" = 755:root:root ] || fail "$f is $(stat -c %a:%U:%G "$f")"
	sh -n "$f" || fail "$f does not parse"
	# The slim images have neither curl nor jq: the hook names the first.
	if ! command -v curl >/dev/null; then
		out=$("$f" set _acme-challenge.massimo.example.net x 2>&1) && fail "$f ran without curl"
		grep -q "$hook: curl is not installed" <<<"$out" || fail "$f without curl: $out"
	fi
done
grep -q '^massimo ' <<<"$(massimo version)" || fail "massimo version"

# On a packaged box massimo install only configures, and refuses to remove.
out=$(massimo install -dry-run 2>&1) || fail "install -dry-run failed: $out"
grep -q 'managed by apt' <<<"$out" || fail "install -dry-run does not say managed by apt: $out"
if grep -q '/usr/local/bin' <<<"$out"; then fail "install -dry-run would touch /usr/local/bin: $out"; fi
out=$(massimo install -reboot-helper -dry-run 2>&1) || fail "install -reboot-helper -dry-run failed: $out"
grep -q 'would run systemctl enable --now massimo-reboot.path' <<<"$out" || fail "install -reboot-helper -dry-run: $out"
out=$(massimo install -no-update-helper -dry-run 2>&1) || fail "install -no-update-helper -dry-run failed: $out"
grep -q 'would run systemctl disable --now massimo-update.path' <<<"$out" || fail "install -no-update-helper -dry-run: $out"
if massimo install -remove 2>/tmp/err; then fail "install -remove did not refuse"; fi
grep -q 'apt remove massimo' /tmp/err || fail "install -remove: $(cat /tmp/err)"
# And the updater run by hand leaves the binary to apt, before any
# network; the helper's run through apt is packaging/apt/test.sh's.
if massimo update 2>/tmp/err; then fail "update did not refuse"; fi
grep -q 'managed by apt' /tmp/err || fail "update: $(cat /tmp/err)"

# When the service starts: postinst asks systemd only where it runs, so
# stand-ins for systemctl and deb-systemd-invoke log what it asks, and
# systemctl answers is-active with the state in /tmp/state.
mkdir -p /run/systemd/system
for cmd in systemctl deb-systemd-invoke; do
	printf '%s\n' '#!/bin/sh' \
		"echo \"$cmd \$*\" >>/tmp/calls" \
		'[ "$1" = is-active ] || exit 0' \
		'state=$(cat /tmp/state 2>/dev/null || echo inactive)' \
		'echo "$state"' \
		'[ "$state" = active ]' >"/usr/local/sbin/$cmd"
	chmod 755 "/usr/local/sbin/$cmd"
done
# reinstall <state>: the package again, with the service in that state;
# apt gives dpkg a PATH of its own, here with the stand-ins first.
reinstall() {
	echo "$1" >/tmp/state
	: >/tmp/calls
	apt-get -qq -o DPkg::Path=/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin install -y --reinstall "$deb" >/tmp/out 2>&1 ||
		fail "reinstall: $(cat /tmp/out)"
}
restarted() { grep -q '^deb-systemd-invoke restart massimo.service$' /tmp/calls; }
# Never ran on the template: left for the owner, who is told how.
reinstall inactive
if restarted; then fail "started on the template: $(cat /tmp/calls)"; fi
grep -q 'user add <name> -role owner' /tmp/out || fail "the message does not name user add: $(cat /tmp/out)"
grep -q 'sudo systemctl start massimo' /tmp/out || fail "the message does not name the start: $(cat /tmp/out)"
# Started by its owner on the template and set up through the web UI:
# restarted with the new binary, and no message.
reinstall active
restarted || fail "a running service was not restarted: $(cat /tmp/calls)"
if grep -q 'not started' /tmp/out; then fail "the message for a running service: $(cat /tmp/out)"; fi
reinstall activating
restarted || fail "a service in its restart pause was not restarted: $(cat /tmp/calls)"
# An older template, its comments changed: still the template.
cp /etc/massimo/env /tmp/env
sed -i '1i # A comment of an older template.' /etc/massimo/env
echo >>/etc/massimo/env
reinstall inactive
if restarted; then fail "started on an older template: $(cat /tmp/calls)"; fi
# A setting of its own: started.
cp /tmp/env /etc/massimo/env
echo 'MASSIMO_ANNOUNCE=off' >>/etc/massimo/env
reinstall inactive
restarted || fail "filled-in settings were not started: $(cat /tmp/calls)"
cp /tmp/env /etc/massimo/env
rm -r /run/systemd/system /usr/local/sbin/systemctl /usr/local/sbin/deb-systemd-invoke

# The settings survive a reinstall and an upgrade.
echo 'MASSIMO_API_TOKEN=test' >>/etc/massimo/env
apt-get -qq install -y --reinstall "$deb" >/dev/null
grep -q '^MASSIMO_API_TOKEN=test$' /etc/massimo/env || fail "the settings were replaced on reinstall"
# The reboot helper, once the owner enabled it (the link that
# systemctl enable makes), stays enabled.
mkdir -p /etc/systemd/system/paths.target.wants
ln -s /usr/lib/systemd/system/massimo-reboot.path /etc/systemd/system/paths.target.wants/massimo-reboot.path
apt-get -qq install -y --reinstall "$deb" >/dev/null
[ -L /etc/systemd/system/paths.target.wants/massimo-reboot.path ] || fail "a reinstall disabled the reboot helper"
# The update helper, once the owner turned it off (the link that
# systemctl disable removes), stays off.
rm /etc/systemd/system/paths.target.wants/massimo-update.path
apt-get -qq install -y --reinstall "$deb" >/dev/null
[ ! -e /etc/systemd/system/paths.target.wants/massimo-update.path ] || fail "a reinstall enabled the update helper again"
mkdir -p /var/lib/massimo-update
echo '{}' >/var/lib/massimo-update/status.json

apt-get -qq purge -y massimo >/dev/null
if getent passwd massimo >/dev/null; then fail "the user is left after purge"; fi
[ ! -e /etc/massimo ] || fail "/etc/massimo is left after purge"
[ ! -e /var/lib/massimo ] || fail "/var/lib/massimo is left after purge"
[ ! -e /usr/share/massimo ] || fail "/usr/share/massimo is left after purge"
[ ! -e /usr/bin/massimo ] || fail "/usr/bin/massimo is left after purge"
[ ! -e /usr/lib/systemd/system/massimo-reboot.path ] || fail "the reboot helper is left after purge"
[ ! -e /usr/lib/systemd/system/massimo-update.path ] || fail "the update helper is left after purge"
[ ! -e /var/lib/massimo-update ] || fail "/var/lib/massimo-update is left after purge"
[ ! -L /etc/systemd/system/paths.target.wants/massimo-reboot.path ] || fail "the reboot helper's link is left after purge"

# A hand install blocks the package.
mkdir -p /etc/systemd/system
touch /etc/systemd/system/massimo.service
if apt-get -qq install -y "$deb" >/tmp/out 2>&1; then fail "installed over a hand install"; fi
grep -q 'massimo install -remove' /tmp/out || fail "the guard's message: $(cat /tmp/out)"
rm /etc/systemd/system/massimo.service
echo "deb test: ok"

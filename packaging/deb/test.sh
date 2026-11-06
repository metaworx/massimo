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
export DEBIAN_FRONTEND=noninteractive
apt-get -qq update
apt-get -qq install -y "$deb" >/dev/null

getent passwd massimo >/dev/null || fail "no user massimo"
getent group massimo >/dev/null || fail "no group massimo"
[ "$(stat -c %a:%U:%G /etc/massimo)" = 750:massimo:massimo ] || fail "/etc/massimo is $(stat -c %a:%U:%G /etc/massimo)"
[ "$(stat -c %a:%U:%G /etc/massimo/env)" = 600:massimo:massimo ] || fail "/etc/massimo/env is $(stat -c %a:%U:%G /etc/massimo/env)"
cmp -s /etc/massimo/env /usr/share/massimo/env.template || fail "/etc/massimo/env is not the template"
unit=/usr/lib/systemd/system/massimo.service
grep -q '^ExecStart=/usr/bin/massimo ' "$unit" || fail "the unit does not run /usr/bin/massimo"
if grep '^ExecStart=' "$unit" | grep -q -- '-update-request'; then fail "the unit still has -update-request"; fi
[ -L /etc/systemd/system/multi-user.target.wants/massimo.service ] || fail "the unit is not enabled"
[ "$(head -n 1 /usr/share/massimo/packaged)" = apt ] || fail "the marker does not say apt"
[ -f /usr/share/doc/massimo/copyright ] || fail "no copyright file"
grep -q '^massimo ' <<<"$(massimo version)" || fail "massimo version"

# On a packaged box massimo install only configures, and refuses to remove.
out=$(massimo install -dry-run 2>&1) || fail "install -dry-run failed: $out"
grep -q 'managed by apt' <<<"$out" || fail "install -dry-run does not say managed by apt: $out"
if grep -q '/usr/local/bin' <<<"$out"; then fail "install -dry-run would touch /usr/local/bin: $out"; fi
if massimo install -remove 2>/tmp/err; then fail "install -remove did not refuse"; fi
grep -q 'apt remove massimo' /tmp/err || fail "install -remove: $(cat /tmp/err)"
# And the updater leaves the binary to apt, before any network.
if massimo update 2>/tmp/err; then fail "update did not refuse"; fi
grep -q 'managed by apt' /tmp/err || fail "update: $(cat /tmp/err)"

# The settings survive a reinstall and an upgrade.
echo 'MASSIMO_API_TOKEN=test' >>/etc/massimo/env
apt-get -qq install -y --reinstall "$deb" >/dev/null
grep -q '^MASSIMO_API_TOKEN=test$' /etc/massimo/env || fail "the settings were replaced on reinstall"

apt-get -qq purge -y massimo >/dev/null
if getent passwd massimo >/dev/null; then fail "the user is left after purge"; fi
[ ! -e /etc/massimo ] || fail "/etc/massimo is left after purge"
[ ! -e /usr/share/massimo ] || fail "/usr/share/massimo is left after purge"
[ ! -e /usr/bin/massimo ] || fail "/usr/bin/massimo is left after purge"

# A hand install blocks the package.
mkdir -p /etc/systemd/system
touch /etc/systemd/system/massimo.service
if apt-get -qq install -y "$deb" >/tmp/out 2>&1; then fail "installed over a hand install"; fi
grep -q 'massimo install -remove' /tmp/out || fail "the guard's message: $(cat /tmp/out)"
rm /etc/systemd/system/massimo.service
echo "deb test: ok"

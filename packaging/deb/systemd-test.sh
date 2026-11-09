#!/usr/bin/env bash
# The Debian package under a real systemd: `make deb-systemd-test` and
# CI's "Debian package" job run it on the host, with the packages that
# `make packages` left in dist/. test.sh's container has no systemd, so a
# start, a restart and a rename of a running service's account happen
# only here: each case of systemd-cases.sh runs in a container of its
# own, systemd its first process, without network, so that a running
# serve reaches neither the LAN nor GitHub and no case sees what another
# left behind.
#
# usage: systemd-test.sh [<image>] [<case>...]
#
# The upgrade cases start from v0.12.0-alpha.1, the last release whose
# account was massimo; OLD_DEB names its amd64 package instead of the
# download, for a run without the network.
set -euo pipefail

image=${1:-debian:trixie-slim}
shift || true
cases=("$@")
[ ${#cases[@]} -gt 0 ] || cases=(fresh upgrade person takenover install)

root=$(cd "$(dirname "$0")/../.." && pwd)
dist=$root/dist
ls "$dist"/massimo_*_amd64.deb >/dev/null 2>&1 || {
	echo "systemd-test: no $dist/massimo_*_amd64.deb; run make packages first" >&2
	exit 1
}
[ -x "$dist/massimo-linux-amd64" ] || {
	echo "systemd-test: no $dist/massimo-linux-amd64; run make packages first" >&2
	exit 1
}

old_tag=v0.12.0-alpha.1
old_name=massimo_0.12.0.alpha.1_amd64.deb
work=$(mktemp -d)
names=()
cleanup() {
	for n in "${names[@]}"; do docker rm -f "$n" >/dev/null 2>&1 || true; done
	rm -rf "$work"
}
trap cleanup EXIT

# The old package: a file named, else the release's, through gh where it
# is signed in (CI gives it the job's token), else from the download page.
if [ -n "${OLD_DEB:-}" ]; then
	cp "$OLD_DEB" "$work/old.deb"
elif command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
	gh release download "$old_tag" -R metaworx/massimo -p "$old_name" -D "$work"
	mv "$work/$old_name" "$work/old.deb"
else
	curl -fsSL --retry 3 -o "$work/old.deb" \
		"https://github.com/metaworx/massimo/releases/download/$old_tag/$old_name"
fi

# systemd as the first process, and no policy-rc.d: Debian's images ship
# one that exits 101, so that deb-systemd-invoke refuses every start and
# stop, which a real box does not.
tag="massimo-systemd-test:${image//[:\/]/-}"
docker build -q -t "$tag" - >/dev/null <<DOCKERFILE
FROM $image
RUN apt-get update \\
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\
	systemd systemd-sysv dbus adduser init-system-helpers logrotate curl procps sudo ca-certificates \\
 && rm -rf /var/lib/apt/lists/* /usr/sbin/policy-rc.d
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
DOCKERFILE

failed=()
for c in "${cases[@]}"; do
	name="massimo-systemd-$c-$$"
	names+=("$name")
	docker run -d --name "$name" --hostname massimo-test --network none \
		--privileged --cgroupns=private --tmpfs /run --tmpfs /run/lock \
		-v "$dist:/dist:ro" -v "$work:/old:ro" \
		-v "$root/packaging/deb/systemd-cases.sh:/cases.sh:ro" "$tag" >/dev/null
	state=
	for _ in $(seq 1 60); do
		state=$(docker exec "$name" systemctl is-system-running 2>/dev/null || true)
		case "$state" in running | degraded) break ;; esac
		sleep 0.5
	done
	case "$state" in
	running | degraded) ;;
	*)
		echo "FAIL $c: systemd did not come up ($state)"
		failed+=("$c")
		continue
		;;
	esac
	echo "== $c ($image)"
	if ! docker exec "$name" bash /cases.sh "$c"; then
		failed+=("$c")
		echo "--- the journal of massimo.service"
		docker exec "$name" journalctl -u massimo.service --no-pager -o cat 2>/dev/null | tail -20 || true
	fi
	docker rm -f "$name" >/dev/null
done

if [ ${#failed[@]} -gt 0 ]; then
	echo "systemd-test: failed on $image: ${failed[*]}" >&2
	exit 1
fi
echo "systemd test: ok ($image)"

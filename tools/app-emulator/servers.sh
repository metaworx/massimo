# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The emulator suite's two servers on the host (tools/app-emulator/README.md,
# "The servers"), sourced by run.sh: massimo serve built from this tree,
# each in a temporary directory of its own (its store and its self-signed
# certificate there, nothing of the checkout's .env: MASSIMO_ENV_FILE is
# /dev/null), against a fake Cube of its own with the UI suite's greeting
# (tools/ui-test/greeting.txt, the serial KEQ0000100), over HTTPS with the
# certificate it makes at its start, on a loopback port that the emulator
# reaches through adb reverse; no proxy, no announcement, no search for
# Cubes but at the fake one. The owner's key is the run's own, the
# servers' MASSIMO_API_TOKEN, and the user signs in with a password.
#
# A library: it sets no shell option. Its functions read the caller's
# work (a temporary directory), out (the results' directory), root (the
# checkout), key, user and password, and each server's ports as
# api_<s>, cube_<s> and disc_<s>, s being a or b; they add each program
# they start to the caller's array pids, which shellcheck cannot see
# assigned here.
# shellcheck shell=bash disable=SC2154

# servers_build builds massimo from the checkout into work, the web UI's
# parts first, as make ui bundles them (the server embeds them).
servers_build() {
	(cd "$root" && go -C tools/uibundle run . && go build -o "$work/massimo" ./cmd/massimo)
}

# servers_start s starts server s and its fake Cube; their logs go to out.
servers_start() {
	local s=$1 d="$work/$1" api cube disc
	api=$(servers_port api "$s")
	cube=$(servers_port cube "$s")
	disc=$(servers_port disc "$s")
	mkdir -p "$d"
	(cd "$d" && exec env -i PATH="$PATH" HOME="$HOME" "$work/massimo" fakecube -greeting "$root/tools/ui-test/greeting.txt" \
		-listen "127.0.0.1:$cube" -discovery "127.0.0.1:$disc") >"$out/fakecube-$s.log" 2>&1 &
	pids+=("$!")
	(cd "$d" && exec env -i PATH="$PATH" HOME="$HOME" MASSIMO_ENV_FILE=/dev/null MASSIMO_API_TOKEN="$key" \
		MASSIMO_CUBE_KEQ0000100_ADDRESS="127.0.0.1:$cube" MASSIMO_CUBE_KEQ0000100_NAME="Fake Cube" \
		"$work/massimo" -data "$d" serve -proxy off -announce=off -tls self-signed -listen "127.0.0.1:$api" \
		-update-status "$d/no-update-status.json" -discover-target "127.0.0.1:$disc" -discover-listen 127.0.0.1:0 \
		-discover-interval 0) >"$out/serve-$s.log" 2>&1 &
	pids+=("$!")
}

# servers_ready s waits until server s and its fake Cube listen on their
# own ports and the server is connected to its Cube, then adds the user;
# every program started must still run then, since a server of another
# run on the port would answer the same way. It prints the certificate's
# fingerprint, which the server's log names as it makes it.
servers_ready() {
	local s=$1 api cube fp body seen i
	api=$(servers_port api "$s")
	cube=$(servers_port cube "$s")
	# called as $(servers_ready s) || …, where errexit is off: each failure returns
	servers_listening "$out/fakecube-$s.log" "fake cube listening on 127.0.0.1:$cube " "the fake Cube of server $s" || return 1
	servers_listening "$out/serve-$s.log" "msg=\"api listening\" source=system addr=127.0.0.1:$api " "server $s" || return 1
	i=0
	while :; do
		# a loopback server this run made, its certificate checked below by its fingerprint
		body=$(curl -sfk -H "Authorization: Bearer $key" "https://127.0.0.1:$api/api/v1/cubes" 2>/dev/null) || body=""
		if grep -qE '"connected": *true' <<<"$body"; then break; fi
		i=$((i + 1))
		if [ "$i" -gt 100 ]; then
			echo "app-emulator: server $s did not connect to its fake Cube; its log: $out/serve-$s.log" >&2
			return 1
		fi
		sleep 0.2
	done
	for p in "${pids[@]}"; do
		if ! kill -0 "$p" 2>/dev/null; then
			echo "app-emulator: a fake Cube or a server this run started has stopped (a port in use?); the logs are in $out" >&2
			return 1
		fi
	done
	if ! printf '%s\n' "$password" | env -i PATH="$PATH" HOME="$HOME" MASSIMO_ENV_FILE=/dev/null \
		"$work/massimo" -data "$work/$s" user add "$user" -role operator -password-stdin >/dev/null 2>>"$out/serve-$s.log"; then
		echo "app-emulator: the user was not added at server $s; its log: $out/serve-$s.log" >&2
		return 1
	fi
	fp=$(sed -n '/msg="tls: self-signed certificate made"/{s/.* fingerprint=\(SHA256:[0-9A-F:]*\).*/\1/p;q;}' "$out/serve-$s.log")
	if [ "${#fp}" -ne 102 ]; then
		echo "app-emulator: server $s's log names no certificate it made; its log: $out/serve-$s.log" >&2
		return 1
	fi
	# the certificate that answers is the one the log names
	seen=$(openssl s_client -connect "127.0.0.1:$api" </dev/null 2>/dev/null | openssl x509 -noout -fingerprint -sha256 2>/dev/null) || seen=""
	if [ "SHA256:${seen#*=}" != "$fp" ]; then
		echo "app-emulator: server $s answers with another certificate than its log names; another program on port $api?" >&2
		return 1
	fi
	echo "$fp"
}

# servers_port kind s prints server s's port of kind api, cube or disc.
servers_port() {
	local v="${1}_${2}"
	echo "${!v}"
}

# servers_listening log text what waits up to 20 s for the line in which a
# program says it listens on its own port.
servers_listening() {
	local i=0
	until grep -qF "$2" "$1" 2>/dev/null; do
		i=$((i + 1))
		if [ "$i" -gt 100 ]; then
			echo "app-emulator: $3 did not start (its port in use?); its log: $1" >&2
			return 1
		fi
		sleep 0.2
	done
}

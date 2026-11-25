# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The emulator suite's slots (tools/app-emulator/README.md, "The ports"),
# sourced by run.sh, so that runs of two checkouts at once each take ports
# of their own without a setting (AP EmulatorRuns, D2-A). Slot k, 0 to 7,
# gives the emulator's console port 5584 - 2k, counting down so that adb
# finds the emulator by itself after its server restarts (it looks at
# 5555 to 5585 alone), the servers' 9443 + 2k and 9444 + 2k, their fake
# Cubes' 62941 + 2k and 62942 + 2k, and the fake Cubes' discovery ports
# 23941 + 2k and 23942 + 2k; a port variable given (APP_EMU_PORT,
# APP_EMU_API_PORT_A, …) replaces that port alone.
#
# A slot is held by a flock of its lock file,
# massimo-app-emulator-slot.<k>.lock in APP_EMU_LOCK_DIR (default /tmp,
# not TMPDIR, so that runs with other TMPDIRs see one another's), made
# where missing, readable by everyone, and left in place; it is opened
# for reading, so that one another user made serves too. A slot whose
# lock path is a link or no regular file, a FIFO among them, whose open
# would hang, is passed over; so is one whose lock is free where adb
# lists its emulator, one that a killed run left and that holds no lock,
# or where one of its ports listens, the console's pair (the console
# port + 1, adb's) among them: ss reads the sockets without connecting
# to them, since a connection to a server of another program would put
# an error in its log.
#
# A checkout's run also locks the suite's directory (slot_checkout): its
# results/ and app/android's build are the checkout's, which a second run
# of the same checkout would take from under the first. Both locks are
# then handed to one holder (slot_hold), a process outside the run's
# children that keeps them until the run has ended and been reaped,
# however it ends, a signal to its whole process group included, and
# lets them go within about a second, so that nothing else the run
# starts holds either: not an adb server that a later adb call starts
# again, nor a loop of the run's that a kill -9 of it leaves running.
#
# A library: it sets no shell option. slot_take reads the caller's adb,
# which is not assigned here, and sets slot, slot_lock (its lock file),
# port, serial (emulator-<port>), api_a, api_b, cube_a, cube_b, disc_a
# and disc_b, which run.sh and servers.sh read, not this file. The locks
# are on fd 8 (the checkout) and fd 9 (the slot) until slot_hold.
# shellcheck shell=bash disable=SC2034,SC2154

# slot_checkout dir locks dir, the suite's directory, on fd 8, and fails
# where another run holds it, saying so.
slot_checkout() {
	if ! { exec 8<"$1"; } 2>/dev/null; then
		echo "app-emulator: $1 cannot be opened, whose lock says that a run of this checkout is going" >&2
		return 1
	fi
	if ! flock -n 8; then
		exec 8<&-
		echo "app-emulator: a run of this checkout is going (it holds $1): its results/ and app/android's build are this checkout's, so wait for it, or run from another checkout" >&2
		return 1
	fi
}

# slot_take takes the first free slot: it locks the slot's file on fd 9,
# which stays open and locked, sets the slot's ports and returns 0; each
# slot passed over is said, with why. It returns 1, holding no slot's
# lock, where none is free, where a port variable is no port, and where
# ss cannot say what listens, the last two said. adb is asked before any
# slot's lock is taken, and without the checkout's, so that an adb server
# it starts inherits neither.
slot_take() {
	local dir=${APP_EMU_LOCK_DIR:-/tmp} devices k f p busy listening why
	local -a ports names
	devices=$("$adb" devices </dev/null 8<&- 9<&- 2>/dev/null) || devices=""
	names=(APP_EMU_PORT APP_EMU_API_PORT_A APP_EMU_API_PORT_B APP_EMU_CUBE_PORT_A APP_EMU_CUBE_PORT_B
		APP_EMU_DISCOVERY_PORT_A APP_EMU_DISCOVERY_PORT_B)
	for k in 0 1 2 3 4 5 6 7; do
		ports=("${APP_EMU_PORT:-$((5584 - 2 * k))}" "${APP_EMU_API_PORT_A:-$((9443 + 2 * k))}" "${APP_EMU_API_PORT_B:-$((9444 + 2 * k))}"
			"${APP_EMU_CUBE_PORT_A:-$((62941 + 2 * k))}" "${APP_EMU_CUBE_PORT_B:-$((62942 + 2 * k))}"
			"${APP_EMU_DISCOVERY_PORT_A:-$((23941 + 2 * k))}" "${APP_EMU_DISCOVERY_PORT_B:-$((23942 + 2 * k))}")
		for p in "${!ports[@]}"; do
			if ! [[ ${ports[p]} =~ ^[1-9][0-9]{0,4}$ ]] || [ "${ports[p]}" -gt 65535 ]; then
				echo "app-emulator: ${names[p]} is a port, not '${ports[p]}'" >&2
				return 1
			fi
		done
		f="$dir/massimo-app-emulator-slot.$k.lock"
		# a link or a FIFO is no lock of the suite's, and a FIFO would
		# hang the open
		if [ -L "$f" ] || { [ -e "$f" ] && [ ! -f "$f" ]; }; then
			echo "app-emulator: slot $k passed over: its lock file $f is no regular file"
			continue
		fi
		[ -e "$f" ] || (umask 022 && : >"$f") 2>/dev/null || true
		if ! { exec 9<"$f"; } 2>/dev/null; then
			echo "app-emulator: slot $k passed over: its lock file $f cannot be opened"
			continue
		fi
		if ! flock -n 9; then
			exec 9<&-
			echo "app-emulator: slot $k passed over: another run holds it"
			continue
		fi
		busy=""
		if grep -q "^emulator-${ports[0]}[[:space:]]" <<<"$devices"; then
			busy="emulator-${ports[0]} runs"
		else
			for p in "${ports[@]}" $((ports[0] + 1)); do
				# its output alone: a warning on its standard error is no
				# socket
				if ! listening=$(ss -Hltun "sport = :$p" 2>/dev/null); then
					exec 9<&-
					why=$(ss -Hltun "sport = :$p" 2>&1 >/dev/null) || true
					echo "app-emulator: ss cannot say which ports listen: ${why:-it failed}" >&2
					return 1
				fi
				if [ -n "$listening" ]; then
					busy="port $p in use"
					break
				fi
			done
		fi
		if [ -n "$busy" ]; then
			exec 9<&-
			echo "app-emulator: slot $k passed over: $busy"
			continue
		fi
		slot=$k slot_lock=$f
		port=${ports[0]} api_a=${ports[1]} api_b=${ports[2]}
		cube_a=${ports[3]} cube_b=${ports[4]} disc_a=${ports[5]} disc_b=${ports[6]}
		serial="emulator-$port"
		return 0
	done
	return 1
}

# slot_hold hands the locks on fd 8 and 9 to a holder and closes them in
# the caller. The holder is tail, waiting for the caller's end
# (--pid=$$, which it looks at every second), started by a subshell that
# ends at once: it is no child of the caller, whose wait does not wait
# for it. It ignores INT, TERM, HUP and QUIT (a signal ignored before
# exec stays ignored after it), so that a signal to the whole process
# group, as Ctrl-C or a timeout sends it, does not end it before the
# caller: it keeps the locks until the caller has ended and been reaped,
# however it ends, a group signal and a kill -9 included, and so through
# the caller's cleanup, which still writes the checkout's results/.
slot_hold() {
	(trap '' INT TERM HUP QUIT; exec tail --pid="$$" -f /dev/null </dev/null >/dev/null 2>&1 &)
	exec 8<&- 9<&-
}

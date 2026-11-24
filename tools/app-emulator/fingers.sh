#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The emulator suite's helper on the host (tools/app-emulator/README.md,
# "The fingerprint"): a test inside the emulator cannot reach the
# emulator's console, which alone touches its fingerprint sensor, so it
# asks for each touch with a line in the log under the suite's tag,
# MassimoEmu, "touch <n>", n rising (Rig.touch in the app's androidTest);
# this reads the log and answers each such line once, with a touch of the
# enrolled finger and its lift (adb emu finger touch, finger remove). A
# touch or a lift that fails is said and the helper goes on, the test
# asking again where its prompt stays. It runs until it is stopped or the
# emulator is gone: a log that ends while adb still sees the emulator
# (adb get-state) is read again, its older lines skipped as below.
#
# usage: fingers.sh emulator-<port>
#
# It talks to the one device it is given, and only to an emulator: a
# serial that is not emulator-<port> is refused before adb is called.
# ADB names adb (default: adb on the PATH); APP_EMU_FINGER the finger's id
# (default 1, the one the enrollment's touches made).
set -euo pipefail

serial=${1:-}
if ! [[ $serial =~ ^emulator-[0-9]+$ ]]; then
	echo "fingers.sh: '$serial' is not an emulator's serial (emulator-<port>); the helper touches no other device" >&2
	exit 2
fi
adb=${ADB:-adb}
finger=${APP_EMU_FINGER:-1}
if ! [[ $finger =~ ^[0-9]+$ ]]; then
	echo "fingers.sh: APP_EMU_FINGER is a finger's number, not '$finger'" >&2
	exit 2
fi

# answer reads the suite's lines of the log on its input and answers each
# ask once; last, the newest ask answered, outlives it.
last=0
answer() {
	local line n
	while IFS= read -r line; do
		line=${line%$'\r'}
		[[ $line =~ ^touch\ ([0-9]{1,18})$ ]] || continue
		n=${BASH_REMATCH[1]}
		# A line read again is not a second touch: n rises from one ask to
		# the next, across the suite's processes.
		((10#$n > last)) || continue
		last=$((10#$n))
		# the lift follows a failed touch too, so that no finger stays down
		"$adb" -s "$serial" emu finger touch "$finger" </dev/null >/dev/null || echo "fingers.sh: the touch for $n failed" >&2
		sleep 0.3
		"$adb" -s "$serial" emu finger remove </dev/null >/dev/null || echo "fingers.sh: the lift after $n failed" >&2
		echo "fingers.sh: touched for $n"
	done
}

while :; do
	answer < <("$adb" -s "$serial" logcat -v raw -s MassimoEmu:I </dev/null)
	if [ "$(timeout 10 "$adb" -s "$serial" get-state </dev/null 2>/dev/null)" != device ]; then
		echo "fingers.sh: the emulator is gone"
		exit 0
	fi
	echo "fingers.sh: the log ended while the emulator runs; reading it again" >&2
	sleep 1
done

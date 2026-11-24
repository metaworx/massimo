#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The Android app's flows on an emulator (make app-emulator-test;
# tools/app-emulator/README.md): the app's instrumented tests
# (app/android/app/src/androidTest/) on an emulator of the pinned system
# image (pins.sh), against two servers on the host built from this tree
# (servers.sh), which the emulator reaches at 127.0.0.1 through adb
# reverse, as the phone rig reaches one; the fingerprint simulated, each
# touch a test asks for in the log answered by the helper (fingers.sh).
#
# In order: the SDK's adb, emulator and image (sdk.sh check) and KVM,
# which the emulator needs, each said where it is missing before anything
# starts; the AVD massimo-test, made where missing and its settings
# written from the pins at each run; the emulator, without a window,
# audio or snapshots, its data wiped, so that every run starts as a new
# phone, the device's log kept from the moment adb sees it; while it
# boots, the servers and the app's two APKs built; once it has booted,
# its animations and autofill off, the screen kept on and the PIN set,
# which a fingerprint needs; adb reverse for both servers' ports, the
# owner's key written to a file on the emulator, the helper started; the
# finger enrolled by EnrollTest alone, the run stopped where that fails;
# then Gradle's :app:connectedFossDebugAndroidTest on that one emulator
# (ANDROID_SERIAL), the servers, the user, the PIN and the key's file as
# the instrumentation's arguments; after each of the two Gradle runs, what
# it ran, counted from its JUnit results against the tests the sources
# declare (tally/); then everything stopped, and what the run leaves kept
# in results/ next to this script (the emulator's log with the kernel's,
# the device's, the helper's, the servers', the screen of a failed test,
# Gradle's report and results).
#
# Gradle passes a run whose tests were skipped, or that ran fewer than
# declared: strict (APP_EMU_STRICT=1, or CI=true as GitHub sets it), the
# run fails there too.
#
# A run stopped by a signal (INT, TERM), as a CI runner stops a step at
# its time limit, keeps them too: Gradle is waited for in the background,
# so that the trap runs at once and not once Gradle ends, and the cleanup
# takes the screens and the report home before it stops the emulator.
#
# Every adb command names the emulator by its serial, emulator-<port>:
# no command goes to another device, and a phone adb knows is left alone.
#
# Environment: ANDROID_HOME (the SDK; adb from its platform-tools);
# APP_EMU_SDK (the SDK holding the emulator and the image, default
# ANDROID_HOME); ANDROID_AVD_HOME (default ~/.android/avd); APP_EMU_PORT
# (the emulator's console port, even, 5554 to 5682, default 5584);
# APP_EMU_BOOT_TIMEOUT (seconds the boot may take once the builds are
# done, default 300); APP_EMU_STRICT (1: a test skipped or not run fails
# the run, as under CI=true; 0 or empty: it is said only); APP_EMU_CLASS (a test class,
# or a class#method, as the instrumentation's -e class takes it);
# APP_EMU_API_PORT_A and _B (the servers' ports, 9443 and 9444),
# APP_EMU_CUBE_PORT_A and _B (their fake Cubes', 62941 and 62942) and
# APP_EMU_DISCOVERY_PORT_A and _B (23941 and 23942). JAVA_HOME as Gradle
# wants it (app/README.md).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
# shellcheck source=pins.sh
. "$here/pins.sh"
# shellcheck source=servers.sh
. "$here/servers.sh"

die() {
	echo "app-emulator: $*" >&2
	exit 1
}

if [ $# -ne 0 ]; then
	echo "usage: run.sh (its settings are environment variables: tools/app-emulator/README.md)" >&2
	exit 2
fi

out="$here/results"
port=${APP_EMU_PORT:-5584}
boot_s=${APP_EMU_BOOT_TIMEOUT:-300}
api_a=${APP_EMU_API_PORT_A:-9443}
api_b=${APP_EMU_API_PORT_B:-9444}
# The fake Cubes' ports, which servers.sh reads by their names.
# shellcheck disable=SC2034
cube_a=${APP_EMU_CUBE_PORT_A:-62941} cube_b=${APP_EMU_CUBE_PORT_B:-62942}
# shellcheck disable=SC2034
disc_a=${APP_EMU_DISCOVERY_PORT_A:-23941} disc_b=${APP_EMU_DISCOVERY_PORT_B:-23942}
avd=massimo-test
avd_home=${ANDROID_AVD_HOME:-$HOME/.android/avd}
user=tester
password=tester-pass-0000
pin=1111
# on the emulator: the screens of failed tests (Rig.SHOTS in the app's
# androidTest), and the owner's key (Rig.auditKey)
shots=/data/local/tmp/massimo-emu
key_file=/data/local/tmp/massimo-emu-key
build="$root/app/android/app/build"

if ! [[ $port =~ ^[0-9]+$ ]] || [ "$port" -lt 5554 ] || [ "$port" -gt 5682 ] || [ $((port % 2)) -ne 0 ]; then
	die "APP_EMU_PORT is an emulator's console port, even, from 5554 to 5682, not '$port'"
fi
if ! [[ $boot_s =~ ^[0-9]+$ ]]; then
	die "APP_EMU_BOOT_TIMEOUT is a number of seconds, not '$boot_s'"
fi
case "${APP_EMU_STRICT:-}" in
1) strict=1 ;;
0) strict=0 ;;
"") strict=0; [ "${CI:-}" != true ] || strict=1 ;;
*) die "APP_EMU_STRICT is 1 or 0, not '$APP_EMU_STRICT'" ;;
esac
serial="emulator-$port"

# --- what the run needs, said before anything starts ---------------------

[ -n "${ANDROID_HOME:-}" ] || die "ANDROID_HOME names no Android SDK (app/README.md, \"The toolchain in WSL\")"
adb="$ANDROID_HOME/platform-tools/adb"
[ -x "$adb" ] || die "the SDK has no adb ($adb): sdkmanager platform-tools"
"$here/sdk.sh" check
sdk=${APP_EMU_SDK:-$ANDROID_HOME}
emulator="$sdk/emulator/emulator"
if [ ! -e /dev/kvm ]; then
	die "no KVM here: /dev/kvm is missing, and the emulator runs an x86_64 image with hardware acceleration alone (tools/app-emulator/README.md, \"What it needs\"); nothing was started"
fi
if [ ! -r /dev/kvm ] || [ ! -w /dev/kvm ]; then
	die "KVM is not accessible: /dev/kvm is $(stat -c '%U:%G, mode %a' /dev/kvm), which this user may not open; it needs the group $(stat -c %G /dev/kvm), or a rule of its own for the device (tools/app-emulator/README.md, \"What it needs\"); nothing was started"
fi
if ! accel=$("$emulator" -accel-check 2>&1); then
	die "the emulator can use no acceleration: $(tr '\n' ' ' <<<"$accel")"
fi
for c in go curl openssl; do
	command -v "$c" >/dev/null || die "$c is not on the PATH"
done
devices=$("$adb" devices </dev/null 2>/dev/null) || devices=""
if grep -q "^${serial}[[:space:]]" <<<"$devices"; then
	die "$serial runs already, another run's emulator or one on its port: APP_EMU_PORT takes another"
fi

# --- the run ------------------------------------------------------------------

rm -rf "$out"
mkdir -p "$out"
# an earlier run's report is not this one's
rm -rf "$build/reports/androidTests/connected" "$build/outputs/androidTest-results/connected"
work=$(mktemp -d)
key=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
pids=()
emu_pid=""
step_pid=""

# reports dir moves the report and the results of Gradle's last connected
# run into dir.
reports() {
	mkdir -p "$1"
	[ ! -d "$build/reports/androidTests/connected" ] || mv "$build/reports/androidTests/connected" "$1/report"
	[ ! -d "$build/outputs/androidTest-results/connected" ] || mv "$build/outputs/androidTest-results/connected" "$1/test-results"
}

# The traps below run cleanup, which shellcheck takes for unreachable.
# shellcheck disable=SC2317
cleanup() {
	set +e
	# a second signal, which a runner sends seconds after its first, does
	# not cut it short; each step is bounded instead
	trap '' INT TERM
	[ -z "$step_pid" ] || kill "$step_pid" 2>/dev/null
	for p in "${pids[@]}"; do kill "$p" 2>/dev/null; done
	if [ -n "$emu_pid" ] && kill -0 "$emu_pid" 2>/dev/null; then
		timeout 10 "$adb" -s "$serial" pull "$shots" "$out/shots" </dev/null >/dev/null 2>&1
	fi
	reports "$out"
	if [ -n "$emu_pid" ] && kill -0 "$emu_pid" 2>/dev/null; then
		timeout 10 "$adb" -s "$serial" emu kill </dev/null >/dev/null 2>&1
		for _ in $(seq 30); do
			kill -0 "$emu_pid" 2>/dev/null || break
			sleep 1
		done
	fi
	# the run's own children, whose numbers no other process takes before
	# they are waited for
	for p in "$emu_pid" "$step_pid" "${pids[@]}"; do [ -z "$p" ] || kill -9 "$p" 2>/dev/null; done
	wait 2>/dev/null
	rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# adbs runs adb on the run's emulator alone.
adbs() {
	"$adb" -s "$serial" "$@" </dev/null
}

# booted prints the emulator's sys.boot_completed, "" while adb cannot
# ask it; an adb that hangs is given 10 s.
booted() {
	timeout 10 "$adb" -s "$serial" shell getprop sys.boot_completed </dev/null 2>/dev/null | tr -d '\r' || true
}

# step runs a long step of the run, Gradle's, in the background and waits
# for it: a shell runs its trap only once its foreground command has
# ended, while wait gives way to a signal at once. The command is the
# step's process itself (env -C, which execs it), so that cleanup stops it.
step() {
	"$@" &
	step_pid=$!
	local rc=0
	wait "$step_pid" || rc=$?
	step_pid=""
	return "$rc"
}

# devlog prints the device's log from the moment adb sees the emulator,
# its boot included; a logcat that ends while adb still sees the emulator
# is started again, the lines it had given repeated. Stopped, it stops its
# adb too.
devlog() {
	local c=""
	trap '[ -z "$c" ] || kill "$c" 2>/dev/null; exit 0' TERM
	"$adb" -s "$serial" wait-for-device </dev/null &
	c=$!
	wait "$c" || return 0
	while :; do
		"$adb" -s "$serial" logcat -v threadtime </dev/null &
		c=$!
		wait "$c" || true
		[ "$(timeout 10 "$adb" -s "$serial" get-state </dev/null 2>/dev/null)" = device ] || return 0
		echo "app-emulator: the device's log ended while the emulator runs; it is read again from its start"
		sleep 1
	done
}

echo "app-emulator: the AVD $avd in $avd_home"
mkdir -p "$avd_home/$avd.avd"
cat >"$avd_home/$avd.ini" <<EOF
avd.ini.encoding=UTF-8
path=$avd_home/$avd.avd
target=android-$APP_EMU_API
EOF
cat >"$avd_home/$avd.avd/config.ini" <<EOF
avd.ini.encoding=UTF-8
AvdId=$avd
avd.ini.displayname=Massimo test
PlayStore.enabled=no
abi.type=$APP_EMU_ABI
hw.cpu.arch=$APP_EMU_ABI
hw.cpu.ncore=2
hw.ramSize=2G
vm.heapSize=228M
disk.dataPartition.size=4G
hw.lcd.width=1080
hw.lcd.height=2400
hw.lcd.density=420
hw.initialOrientation=portrait
hw.keyboard=yes
hw.mainKeys=no
hw.gpu.enabled=yes
hw.gpu.mode=swiftshader_indirect
hw.audioInput=no
hw.audioOutput=no
hw.camera.back=none
hw.camera.front=none
hw.gps=no
hw.sdCard=no
showDeviceFrame=no
fastboot.forceColdBoot=yes
image.sysdir.1=system-images/android-$APP_EMU_API/$APP_EMU_TAG/$APP_EMU_ABI/
tag.id=$APP_EMU_TAG
tag.display=Google APIs
target=android-$APP_EMU_API
EOF

echo "app-emulator: the emulator $serial"
ANDROID_HOME="$sdk" ANDROID_SDK_ROOT="$sdk" ANDROID_AVD_HOME="$avd_home" "$emulator" -avd "$avd" -port "$port" \
	-no-window -no-audio -no-snapshot -no-boot-anim -no-metrics -wipe-data -accel on -gpu swiftshader_indirect \
	-camera-back none -camera-front none -skip-adb-auth -show-kernel </dev/null >"$out/emulator.log" 2>&1 &
emu_pid=$!
started=$SECONDS
devlog >"$out/logcat.txt" 2>&1 &
pids+=("$!")

echo "app-emulator: the servers and the app, while it boots"
servers_build
go -C "$root" build -o "$work/tally" ./tools/app-emulator/tally
servers_start a
servers_start b
fp_a=$(servers_ready a) || exit 1
fp_b=$(servers_ready b) || exit 1
step env -C "$root/app/android" ./gradlew --no-daemon --console=plain -q :app:assembleFossDebug :app:assembleFossDebugAndroidTest

# the boot's time counted from here, so that a slow first build, which
# fetches Gradle's libraries, takes none of it
built=$SECONDS
echo "app-emulator: built $((built - started)) s after the emulator's start; its boot may take $boot_s s more"
until [ "$(booted)" = 1 ]; do
	kill -0 "$emu_pid" 2>/dev/null || die "the emulator stopped as it booted; its log: $out/emulator.log, the device's: $out/logcat.txt"
	[ $((SECONDS - built)) -lt "$boot_s" ] || die "the emulator did not boot within $boot_s s of the builds' end, $((SECONDS - started)) s after its start; its log: $out/emulator.log, the device's: $out/logcat.txt"
	sleep 3
done
until [ -n "$(timeout 10 "$adb" -s "$serial" shell pm path android </dev/null 2>/dev/null | tr -d '\r' || true)" ]; do
	[ $((SECONDS - built)) -lt "$boot_s" ] || die "the emulator's package manager did not start within $boot_s s of the builds' end; the device's log: $out/logcat.txt"
	sleep 2
done
echo "app-emulator: booted $((SECONDS - started)) s after its start"

for s in window_animation_scale transition_animation_scale animator_duration_scale; do
	adbs shell settings put global "$s" 0 || echo "app-emulator: warning: the animations stay on ($s)" >&2
done
adbs shell settings put system screen_off_timeout 2147483647 || echo "app-emulator: warning: the screen's timeout stays" >&2
adbs shell svc power stayon true || echo "app-emulator: warning: the screen may turn off" >&2
adbs shell settings put secure autofill_service null || echo "app-emulator: warning: autofill stays on" >&2
adbs shell input keyevent KEYCODE_WAKEUP || true
adbs shell wm dismiss-keyguard || true
adbs shell locksettings set-pin "$pin" >/dev/null || die "the emulator's PIN was not set"
adbs shell locksettings verify --old "$pin" >/dev/null || die "the emulator's PIN does not verify"
adbs reverse "tcp:$api_a" "tcp:$api_a" >/dev/null || die "adb reverse failed for port $api_a"
adbs reverse "tcp:$api_b" "tcp:$api_b" >/dev/null || die "adb reverse failed for port $api_b"
# the owner's key as a file on the emulator, which the tests read
# (Rig.auditKey), so that it is on no command line
(umask 077 && printf '%s\n' "$key" >"$work/audit-key")
adbs push "$work/audit-key" "$key_file" >/dev/null || die "the owner's key was not written to the emulator ($key_file)"
ADB="$adb" "$here/fingers.sh" "$serial" >"$out/fingers.log" 2>&1 &
pids+=("$!")

arg=-Pandroid.testInstrumentationRunnerArguments
given=(
	"$arg.massimoUrlA=https://127.0.0.1:$api_a/" "$arg.massimoFpA=$fp_a"
	"$arg.massimoUrlB=https://127.0.0.1:$api_b/" "$arg.massimoFpB=$fp_b"
	"$arg.massimoUser=$user" "$arg.massimoPassword=$password" "$arg.massimoPin=$pin" "$arg.massimoAuditKeyFile=$key_file"
)

# connected arg... runs Gradle's connected tests on the run's emulator
# alone, with the suite's arguments and those given.
connected() {
	step env -C "$root/app/android" ANDROID_SERIAL="$serial" ./gradlew --no-daemon --console=plain :app:connectedFossDebugAndroidTest "${given[@]}" "$@"
}

# tally what dir [class] says what the Gradle run named what ran, from its
# results moved into dir, against the tests the sources declare, narrowed
# by the class filter; strict, it fails where a test was skipped, failed
# or not run.
tally() {
	local a=(-what "$1" -tests "$root/app/android/app/src/androidTest/kotlin")
	[ -z "${3:-}" ] || a+=(-class "$3")
	[ "$strict" = 0 ] || a+=(-strict)
	"$work/tally" "${a[@]}" "$2"
}

# The finger once, before the suite: a finger not enrolled is this one
# failure, and not a wait in each case.
echo "app-emulator: the finger, enrolled by EnrollTest alone"
enroll=ch.metaworx.massimo.EnrollTest
if ! connected "$arg.class=$enroll"; then
	reports "$out/enroll"
	tally "the enrollment" "$out/enroll/test-results" "$enroll" || true
	die "no finger was enrolled, which every case needs, so the suite did not start; Settings' screen is kept in $out/shots, the steps in $out/logcat.txt (MassimoEmu), Gradle's report in $out/enroll/report"
fi
reports "$out/enroll"
tally "the enrollment" "$out/enroll/test-results" "$enroll" || die "the enrollment did not run as it should (above), so the suite did not start"

echo "app-emulator: the suite"
suite=()
[ -z "${APP_EMU_CLASS:-}" ] || suite+=("$arg.class=$APP_EMU_CLASS")
rc=0
connected "${suite[@]}" || rc=$?
reports "$out"
counted=0
tally "the suite" "$out/test-results" "${APP_EMU_CLASS:-}" || counted=$?
if [ "$rc" -ne 0 ]; then
	echo "app-emulator: the suite failed; its report, the logs and the screens of failed tests are kept in $out" >&2
	exit "$rc"
fi
if [ "$counted" -ne 0 ]; then
	die "Gradle passed the suite, but not every test the sources declare ran and passed (above): a test skipped or left out passes there"
fi
echo "app-emulator: the suite passed"

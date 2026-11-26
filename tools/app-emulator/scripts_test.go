// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package appemulator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// bash is the shell the scripts run in; the tests are skipped where there
// is none, and on Windows, where the suite does not run.
func bash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the emulator suite runs on Linux")
	}
	b, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	return b
}

// fakeADB is adb as fingers.sh calls it: every call's arguments are
// logged to $FAKE_ADB/args; `-s <serial> logcat …` prints $FAKE_ADB/log
// at its first call, $FAKE_ADB/log.<n> at its n-th, nothing where there is
// none; `-s <serial> get-state` prints the first line of $FAKE_ADB/states
// and takes it away, and fails where none is left; `-s <serial> emu …`
// logs the serial and the console's command to $FAKE_ADB/calls, and fails
// where $FAKE_ADB/fail lists that command; anything else fails.
const fakeADB = `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$FAKE_ADB/args"
[ "$1" = -s ] || exit 3
serial=$2
shift 2
case "$1" in
logcat)
	n=$(($(cat "$FAKE_ADB/logcats" 2>/dev/null || echo 0) + 1))
	echo "$n" >"$FAKE_ADB/logcats"
	f="$FAKE_ADB/log"
	[ "$n" = 1 ] || f="$FAKE_ADB/log.$n"
	[ ! -f "$f" ] || cat "$f"
	;;
get-state)
	[ -s "$FAKE_ADB/states" ] || exit 1
	head -n 1 "$FAKE_ADB/states"
	sed -i 1d "$FAKE_ADB/states"
	;;
emu)
	shift
	printf '%s %s\n' "$serial" "$*" >>"$FAKE_ADB/calls"
	! grep -qxF "$*" "$FAKE_ADB/fail" 2>/dev/null || exit 1
	;;
*) exit 4 ;;
esac
`

// fingers runs fingers.sh for serial against the fake adb, the log
// holding lines, with the extra environment env; it returns the console's
// commands, every adb call's arguments, the script's output and its
// exit code.
func fingers(t *testing.T, serial string, lines []string, env ...string) (calls, args []string, out string, code int) {
	t.Helper()
	return fingersIn(t, t.TempDir(), serial, lines, env...)
}

// fingersIn is fingers with the fake adb's directory dir, which may hold
// its other files already (log.<n>, states, fail).
func fingersIn(t *testing.T, dir, serial string, lines []string, env ...string) (calls, args []string, out string, code int) {
	t.Helper()
	sh := bash(t)
	adb := filepath.Join(dir, "adb")
	if err := os.WriteFile(adb, []byte(fakeADB), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "log"), lines...)
	cmd := exec.Command(sh, "fingers.sh", serial)
	cmd.Env = append(os.Environ(), append([]string{"ADB=" + adb, "FAKE_ADB=" + dir}, env...)...)
	b, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		t.Fatalf("fingers.sh: %v", err)
	}
	read := func(name string) []string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	}
	return read("calls"), read("args"), string(b), code
}

// TestFingersAnswersEachAskOnce: each line "touch <n>" of the suite's log,
// n higher than the last answered, is one touch of finger 1 and its lift
// on the emulator it was given; a line read again, an older one, the
// suite's other lines and anything that is not the line exactly are not.
func TestFingersAnswersEachAskOnce(t *testing.T) {
	calls, args, out, code := fingers(t, "emulator-5584", []string{
		"enrolling a finger",
		"touch 1000",
		"touch 1000",
		"touch 999",
		"touch 2500\r",
		"touch 3000 again",
		"retouch 3500",
		"touch -4000",
		"touch 4000",
	})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	pair := []string{"emulator-5584 finger touch 1", "emulator-5584 finger remove"}
	want := append(append(append([]string{}, pair...), pair...), pair...)
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("the console's commands:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "-s emulator-5584 ") {
			t.Errorf("an adb call without the emulator's serial: %q", a)
		}
	}
	for _, n := range []string{"1000", "2500", "4000"} {
		if !strings.Contains(out, "touched for "+n) {
			t.Errorf("the output does not name the touch for %s:\n%s", n, out)
		}
	}
}

// write writes lines into the file path, each ended by a newline.
func write(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFingersGoesOnAfterAFailedTouch: a touch the console refuses is said,
// its lift still sent, and the next ask answered.
func TestFingersGoesOnAfterAFailedTouch(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "fail"), "finger touch 1")
	calls, _, out, code := fingersIn(t, dir, "emulator-5584", []string{"touch 10", "touch 20"})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	pair := []string{"emulator-5584 finger touch 1", "emulator-5584 finger remove"}
	if want := append(append([]string{}, pair...), pair...); strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("the console's commands:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
	for _, s := range []string{"the touch for 10 failed", "the touch for 20 failed"} {
		if !strings.Contains(out, s) {
			t.Errorf("the output does not say %q:\n%s", s, out)
		}
	}
}

// TestFingersReadsTheLogAgainWhileTheEmulatorRuns: a log that ends while
// adb still sees the emulator is read again, the asks answered before
// not answered twice; once adb no longer sees it, the helper ends.
func TestFingersReadsTheLogAgainWhileTheEmulatorRuns(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "log.2"), "touch 10", "touch 20", "touch 30")
	write(t, filepath.Join(dir, "states"), "device")
	calls, args, out, code := fingersIn(t, dir, "emulator-5584", []string{"touch 10", "touch 20"})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if n := strings.Count(strings.Join(calls, "\n"), "finger touch 1"); n != 3 {
		t.Errorf("%d touches, want 3 (10, 20, 30):\n%s", n, strings.Join(calls, "\n"))
	}
	if n := strings.Count(strings.Join(args, "\n"), " logcat "); n != 2 {
		t.Errorf("the log read %d times, want 2:\n%s", n, strings.Join(args, "\n"))
	}
	for _, s := range []string{"reading it again", "touched for 30", "the emulator is gone"} {
		if !strings.Contains(out, s) {
			t.Errorf("the output does not say %q:\n%s", s, out)
		}
	}
}

// TestFingersTakesTheFingerGiven: APP_EMU_FINGER chooses the finger; one
// that is not a number is refused before adb is called.
func TestFingersTakesTheFingerGiven(t *testing.T) {
	calls, _, out, code := fingers(t, "emulator-5554", []string{"touch 5"}, "APP_EMU_FINGER=2")
	if code != 0 || len(calls) != 2 || calls[0] != "emulator-5554 finger touch 2" {
		t.Errorf("exit %d, calls %q:\n%s", code, calls, out)
	}
	calls, args, out, code := fingers(t, "emulator-5554", []string{"touch 5"}, "APP_EMU_FINGER=1; reboot")
	if code != 2 || calls != nil || args != nil {
		t.Errorf("a finger that is no number: exit %d, calls %q, adb %q:\n%s", code, calls, args, out)
	}
}

// TestFingersRefusesAnotherDevice: a serial that is not an emulator's,
// a phone's over USB or over the network among them, is refused before
// adb is called, so that the helper never touches a phone adb knows.
func TestFingersRefusesAnotherDevice(t *testing.T) {
	for _, serial := range []string{"", "0123456789ABCDEF", "192.168.1.30:41000", "emulator-", "emulator-55x4", "emulator-5584 -d", "Emulator-5584"} {
		calls, args, out, code := fingers(t, serial, []string{"touch 5"})
		if code != 2 || calls != nil || args != nil {
			t.Errorf("serial %q: exit %d, calls %q, adb %q:\n%s", serial, code, calls, args, out)
		}
		if !strings.Contains(out, "not an emulator's serial") {
			t.Errorf("serial %q: the refusal does not say why:\n%s", serial, out)
		}
	}
}

// pins reads pins.sh's assignments.
func pins(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile("pins.sh")
	if err != nil {
		t.Fatal(err)
	}
	p := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^(APP_EMU_[A-Z0-9_]+)=(\S+)$`).FindAllStringSubmatch(string(b), -1) {
		p[m[1]] = m[2]
	}
	return p
}

// TestPinsHoldTogether: the image's archive is the one of its API level,
// ABI, tag and revision, as Google's repository names it, and each
// archive has its size and SHA-1, so that a pin moved in part fails here
// and not on a runner.
func TestPinsHoldTogether(t *testing.T) {
	p := pins(t)
	rev, err := strconv.Atoi(p["APP_EMU_IMAGE_REVISION"])
	if err != nil {
		t.Fatalf("APP_EMU_IMAGE_REVISION: %v", err)
	}
	want := fmt.Sprintf("https://dl.google.com/android/repository/sys-img/%s/%s-%s_r%02d.zip", p["APP_EMU_TAG"], p["APP_EMU_ABI"], p["APP_EMU_API"], rev)
	if p["APP_EMU_IMAGE_URL"] != want {
		t.Errorf("APP_EMU_IMAGE_URL is %q, the pinned image's archive %q", p["APP_EMU_IMAGE_URL"], want)
	}
	if !regexp.MustCompile(`^https://dl\.google\.com/android/repository/emulator-linux_x64-[0-9]+\.zip$`).MatchString(p["APP_EMU_EMULATOR_URL"]) {
		t.Errorf("APP_EMU_EMULATOR_URL is %q, not an emulator's archive for Linux", p["APP_EMU_EMULATOR_URL"])
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(p["APP_EMU_EMULATOR_REVISION"]) {
		t.Errorf("APP_EMU_EMULATOR_REVISION is %q", p["APP_EMU_EMULATOR_REVISION"])
	}
	for _, k := range []string{"APP_EMU_IMAGE", "APP_EMU_EMULATOR"} {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(p[k+"_SHA1"]) {
			t.Errorf("%s_SHA1 is %q, not a SHA-1", k, p[k+"_SHA1"])
		}
		if n, err := strconv.ParseInt(p[k+"_SIZE"], 10, 64); err != nil || n < 1<<20 {
			t.Errorf("%s_SIZE is %q", k, p[k+"_SIZE"])
		}
	}
}

// sdk makes an SDK under a temporary directory holding the emulator and
// the image of the revisions given, "" for a package missing.
func sdk(t *testing.T, emulator, image string) string {
	t.Helper()
	p := pins(t)
	root := t.TempDir()
	put := func(dir, rev string, files ...string) {
		if rev == "" {
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "source.properties"), []byte("Pkg.Desc=x\nPkg.Revision="+rev+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	put(filepath.Join(root, "emulator"), emulator, "emulator")
	put(filepath.Join(root, "system-images", "android-"+p["APP_EMU_API"], p["APP_EMU_TAG"], p["APP_EMU_ABI"]), image, "system.img")
	return root
}

// TestSDKCheck: sdk.sh check passes the pinned packages, warns of another
// revision and goes on, and fails a missing one with the sdkmanager line
// that installs it.
func TestSDKCheck(t *testing.T) {
	sh := bash(t)
	p := pins(t)
	image := "system-images;android-" + p["APP_EMU_API"] + ";" + p["APP_EMU_TAG"] + ";" + p["APP_EMU_ABI"]
	for _, tc := range []struct {
		name            string
		emulator, image string
		code            int
		says, lacks     string
	}{
		{"pinned", p["APP_EMU_EMULATOR_REVISION"], p["APP_EMU_IMAGE_REVISION"], 0, "app-emulator: the emulator " + p["APP_EMU_EMULATOR_REVISION"], "warning"},
		{"another emulator", "36.1.9", p["APP_EMU_IMAGE_REVISION"], 0, "warning: the emulator is 36.1.9, the pin " + p["APP_EMU_EMULATOR_REVISION"], "no emulator"},
		{"another image", p["APP_EMU_EMULATOR_REVISION"], "3", 0, "warning: the system image is revision 3", "no system image"},
		{"no image", p["APP_EMU_EMULATOR_REVISION"], "", 1, `sdkmanager "` + image + `"`, "app-emulator: the emulator "},
		{"no emulator", "", p["APP_EMU_IMAGE_REVISION"], 1, "the SDK has no emulator", "app-emulator: the emulator "},
	} {
		cmd := exec.Command(sh, "sdk.sh", "check")
		cmd.Env = append(os.Environ(), "APP_EMU_SDK="+sdk(t, tc.emulator, tc.image), "ANDROID_HOME=")
		var b bytes.Buffer
		cmd.Stdout, cmd.Stderr = &b, &b
		code := 0
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("%s: %v", tc.name, err)
			}
			code = exit.ExitCode()
		}
		if code != tc.code || !strings.Contains(b.String(), tc.says) || strings.Contains(b.String(), tc.lacks) {
			t.Errorf("%s: exit %d, want %d, saying %q and not %q:\n%s", tc.name, code, tc.code, tc.says, tc.lacks, b.String())
		}
	}
}

// TestSDKFetch: sdk.sh fetch asks for the SDK's licence accepted and
// accepts none itself; with it, an SDK that holds the pins fetches
// nothing, and one apart from ANDROID_HOME gets the SDK's platform-tools,
// platforms and licences linked into it.
func TestSDKFetch(t *testing.T) {
	sh := bash(t)
	p := pins(t)
	home := t.TempDir()
	for _, d := range []string{"platform-tools", "platforms", "licenses"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fetch := func(root string) (string, int) {
		cmd := exec.Command(sh, "sdk.sh", "fetch")
		// no network: a fetch that would download fails at once
		cmd.Env = append(os.Environ(), "APP_EMU_SDK="+root, "ANDROID_HOME="+home, "https_proxy=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9")
		b, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return string(b), exit.ExitCode()
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(b), 0
	}
	root := sdk(t, p["APP_EMU_EMULATOR_REVISION"], p["APP_EMU_IMAGE_REVISION"])
	if out, code := fetch(root); code != 1 || !strings.Contains(out, "licence is not accepted") {
		t.Errorf("without the licence: exit %d:\n%s", code, out)
	}
	if err := os.WriteFile(filepath.Join(home, "licenses", "android-sdk-license"), []byte("\n0000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := fetch(root)
	if code != 0 || strings.Contains(out, "fetching") {
		t.Errorf("the pins held already: exit %d:\n%s", code, out)
	}
	for _, d := range []string{"platform-tools", "platforms", "licenses"} {
		if target, err := os.Readlink(filepath.Join(root, d)); err != nil || target != filepath.Join(home, d) {
			t.Errorf("%s not linked to the SDK's: %q, %v", d, target, err)
		}
	}
	// a download that fails leaves nothing of its own in the SDK
	root = sdk(t, "", p["APP_EMU_IMAGE_REVISION"])
	out, code = fetch(root)
	if code != 1 || !strings.Contains(out, "was not fetched") {
		t.Errorf("a failed download: exit %d:\n%s", code, out)
	}
	left, err := filepath.Glob(filepath.Join(root, ".app-emulator.*"))
	if err != nil || len(left) != 0 {
		t.Errorf("a failed download left %q (%v)", left, err)
	}
}

// fakeSlotADB is adb as slot_take asks it: `devices` prints the list's
// head and the lines of $FAKE_ADB/devices, and writes the descriptors it
// was started with to $FAKE_ADB/adb-fds where /proc/self/fd shows them;
// anything else fails.
const fakeSlotADB = `#!/usr/bin/env bash
[ "$1" = devices ] || exit 4
[ ! -d /proc/self/fd ] || ls /proc/self/fd >"$FAKE_ADB/adb-fds"
echo "List of devices attached"
[ ! -f "$FAKE_ADB/devices" ] || cat "$FAKE_ADB/devices"
`

// fakeSS is ss as slot_take asks it, and only so, ss -Hltun "sport = :<p>":
// it prints a listening socket where $FAKE_SS/ports lists p and nothing
// where it does not, a warning on its standard error where $FAKE_SS/warn
// is there; it fails where $FAKE_SS/fail is there, and where it is asked
// anything else, so that a query without UDP or not of the listening
// sockets fails a test.
const fakeSS = `#!/usr/bin/env bash
if [ $# -ne 2 ] || [ "$1" != -Hltun ] || ! [[ $2 =~ ^sport\ =\ :([0-9]+)$ ]]; then
	echo "fake ss: asked ss $*" >&2
	exit 2
fi
p=${BASH_REMATCH[1]}
if [ -e "$FAKE_SS/fail" ]; then
	echo "Cannot open netlink socket: Protocol not supported" >&2
	exit 1
fi
[ ! -e "$FAKE_SS/warn" ] || echo "ss: warning: some sockets were not read" >&2
if grep -qxF "$p" "$FAKE_SS/ports" 2>/dev/null; then
	echo "tcp   LISTEN 0      4096   127.0.0.1:$p 0.0.0.0:*"
fi
`

// slotSetup is what slots.sh finds: the emulators adb lists, the ports
// ss says listen, whether ss warns or fails, the environment, and a
// script run before the test's in the same shell, which may hold a lock.
type slotSetup struct {
	devices, listening []string
	ssWarns, ssFails   bool
	env                []string
	before             string
}

// slotDirs are the directories of a slot script's run: the fakes', the
// lock files' (APP_EMU_LOCK_DIR) and the checkout's (CHECKOUT).
type slotDirs struct {
	fakes, locks, checkout string
}

// slotEnv writes the fakes of the setup into a directory of their own
// and returns bash, the environment a slot script runs in, the lock files
// in a directory of their own, and the directories.
func slotEnv(t *testing.T, s slotSetup) (sh string, env []string, dirs slotDirs) {
	t.Helper()
	sh = bash(t)
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip("no flock")
	}
	dirs = slotDirs{t.TempDir(), t.TempDir(), t.TempDir()}
	for name, body := range map[string]string{"adb": fakeSlotADB, "ss": fakeSS} {
		if err := os.WriteFile(filepath.Join(dirs.fakes, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if s.devices != nil {
		write(t, filepath.Join(dirs.fakes, "devices"), s.devices...)
	}
	if s.listening != nil {
		write(t, filepath.Join(dirs.fakes, "ports"), s.listening...)
	}
	if s.ssWarns {
		write(t, filepath.Join(dirs.fakes, "warn"))
	}
	if s.ssFails {
		write(t, filepath.Join(dirs.fakes, "fail"))
	}
	env = append(os.Environ(),
		"PATH="+dirs.fakes+string(os.PathListSeparator)+os.Getenv("PATH"), "FAKE_ADB="+dirs.fakes, "FAKE_SS="+dirs.fakes,
		"FLOCK="+flock, "APP_EMU_LOCK_DIR="+dirs.locks, "CHECKOUT="+dirs.checkout,
		"APP_EMU_PORT=", "APP_EMU_API_PORT_A=", "APP_EMU_API_PORT_B=", "APP_EMU_CUBE_PORT_A=",
		"APP_EMU_CUBE_PORT_B=", "APP_EMU_DISCOVERY_PORT_A=", "APP_EMU_DISCOVERY_PORT_B=")
	return sh, append(env, s.env...), dirs
}

// slotPrelude opens every slot script: set -euo pipefail, as run.sh
// runs, slots.sh sourced and the fake adb as adb.
const slotPrelude = "set -euo pipefail\n. ./slots.sh\nadb=\"$FAKE_ADB/adb\"\n"

// slotScript runs body after the prelude and the setup's script, in
// bash against the fake adb and the fake ss; it returns the output, the
// exit code and the directories. A run that does not end within 30 s
// fails.
func slotScript(t *testing.T, s slotSetup, body string) (out string, code int, dirs slotDirs) {
	t.Helper()
	sh, env, dirs := slotEnv(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, sh, "-c", slotPrelude+s.before+"\n"+body)
	cmd.Env = env
	b, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("the slot script did not end within 30 s:\n%s", b)
	}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		t.Fatalf("the slot script: %v", err)
	}
	return string(b), code, dirs
}

// slotTakeBody runs slot_take and prints the slot's variables on a line
// "taken …" where it took one, then each slot's lock as "held <k>",
// "free <k>" or, where its path is no regular file, "other <k>", and
// exits with slot_take's code.
const slotTakeBody = `rc=0
slot_take || rc=$?
if [ "$rc" -eq 0 ]; then
	echo "taken slot=$slot port=$port serial=$serial api_a=$api_a api_b=$api_b cube_a=$cube_a cube_b=$cube_b disc_a=$disc_a disc_b=$disc_b"
fi
for k in 0 1 2 3 4 5 6 7; do
	f="$APP_EMU_LOCK_DIR/massimo-app-emulator-slot.$k.lock"
	if [ -L "$f" ] || { [ -e "$f" ] && [ ! -f "$f" ]; }; then
		echo "other $k"
	elif "$FLOCK" -n "$f" true; then
		echo "free $k"
	else
		echo "held $k"
	fi
done
exit "$rc"
`

// slotTake runs slot_take (slotTakeBody) on the setup.
func slotTake(t *testing.T, s slotSetup) (out string, code int) {
	t.Helper()
	out, code, _ = slotScript(t, s, slotTakeBody)
	return out, code
}

// locks is the lock lines slotTakeBody prints where the slots held are
// held, the slots other are no regular file and the rest are free.
func locks(held, other []int) []string {
	var lines []string
	for k := 0; k < 8; k++ {
		state := "free"
		switch {
		case slices.Contains(held, k):
			state = "held"
		case slices.Contains(other, k):
			state = "other"
		}
		lines = append(lines, fmt.Sprintf("%s %d", state, k))
	}
	return lines
}

// lockSlot0 is a script that holds slot 0's lock in the shell itself, on
// another open of its file than slot_take's.
const lockSlot0 = `f="$APP_EMU_LOCK_DIR/massimo-app-emulator-slot.0.lock"
: >"$f"
exec 7<"$f"
flock -n 7
`

// slotLine is the line slotTakeBody prints for slot k with the default
// ports, the console's given where console is not 0.
func slotLine(k, console int) string {
	if console == 0 {
		console = 5584 - 2*k
	}
	return fmt.Sprintf("taken slot=%d port=%d serial=emulator-%d api_a=%d api_b=%d cube_a=%d cube_b=%d disc_a=%d disc_b=%d",
		k, console, console, 9443+2*k, 9444+2*k, 62941+2*k, 62942+2*k, 23941+2*k, 23942+2*k)
}

// TestSlotTakesTheFirstFree: slot_take takes the first slot whose lock
// is free, whose emulator adb does not list and none of whose ports
// listens, the console port's pair among them, holds its lock and sets
// its seven ports: the console's 2k below slot 0's, where adb finds an
// emulator after its server restarts, the others 2k above; a slot passed
// over is said with why, and its lock is let go; a warning of ss's is no
// port in use (AP EmulatorRuns, D2-A).
func TestSlotTakesTheFirstFree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup slotSetup
		says  []string
		held  []int
	}{
		{"nothing taken", slotSetup{devices: []string{"0123456789ABCDEF\tdevice"}}, []string{slotLine(0, 0)}, []int{0}},
		{"slot 0's lock held", slotSetup{before: lockSlot0}, []string{"slot 0 passed over: another run holds it", slotLine(1, 0)}, []int{0, 1}},
		{"slot 0's emulator runs", slotSetup{devices: []string{"emulator-5584\tdevice"}}, []string{"slot 0 passed over: emulator-5584 runs", slotLine(1, 0)}, []int{1}},
		{"a fake Cube's port listens", slotSetup{listening: []string{"62941"}}, []string{"slot 0 passed over: port 62941 in use", slotLine(1, 0)}, []int{1}},
		{"the console's pair listens", slotSetup{listening: []string{"5585"}}, []string{"slot 0 passed over: port 5585 in use", slotLine(1, 0)}, []int{1}},
		{"ss warns", slotSetup{ssWarns: true}, []string{slotLine(0, 0)}, []int{0}},
		{"a server's port given", slotSetup{env: []string{"APP_EMU_API_PORT_A=9999"}},
			[]string{"taken slot=0 port=5584 serial=emulator-5584 api_a=9999 api_b=9444 cube_a=62941 cube_b=62942 disc_a=23941 disc_b=23942"}, []int{0}},
		{"the console's port given", slotSetup{env: []string{"APP_EMU_PORT=5560"}, listening: []string{"9443"}},
			[]string{"slot 0 passed over: port 9443 in use", slotLine(1, 5560)}, []int{1}},
	} {
		out, code := slotTake(t, tc.setup)
		if code != 0 {
			t.Errorf("%s: exit %d:\n%s", tc.name, code, out)
			continue
		}
		for _, s := range append(tc.says, locks(tc.held, nil)...) {
			if !strings.Contains(out, s+"\n") {
				t.Errorf("%s: the output does not say %q:\n%s", tc.name, s, out)
			}
		}
		if n := strings.Count(out, "passed over"); n != len(tc.says)-1 {
			t.Errorf("%s: %d slots passed over, want %d:\n%s", tc.name, n, len(tc.says)-1, out)
		}
	}
}

// TestSlotPortGiven: slot_port_given takes a console port given that
// adb finds by itself after its server restarts, an even one from 5554
// to 5584, and an APP_EMU_PORT unset or empty; it refuses any other,
// saying so (AP EmulatorRuns, D6-B).
func TestSlotPortGiven(t *testing.T) {
	const body = `rc=0
slot_port_given || rc=$?
echo "rc=$rc"
`
	for _, tc := range []struct {
		name, before string
		env          []string
		taken        bool
	}{
		{"5554", "", []string{"APP_EMU_PORT=5554"}, true},
		{"5570", "", []string{"APP_EMU_PORT=5570"}, true},
		{"5584", "", []string{"APP_EMU_PORT=5584"}, true},
		{"empty", "", nil, true},
		{"unset", "unset APP_EMU_PORT", nil, true},
		{"5553", "", []string{"APP_EMU_PORT=5553"}, false},
		{"5583", "", []string{"APP_EMU_PORT=5583"}, false},
		{"5586", "", []string{"APP_EMU_PORT=5586"}, false},
		{"5682", "", []string{"APP_EMU_PORT=5682"}, false},
		{"5552", "", []string{"APP_EMU_PORT=5552"}, false},
		{"a word", "", []string{"APP_EMU_PORT=console"}, false},
		{"a leading zero", "", []string{"APP_EMU_PORT=05560"}, false},
	} {
		out, code, _ := slotScript(t, slotSetup{env: tc.env, before: tc.before}, body)
		if code != 0 {
			t.Errorf("%s: exit %d:\n%s", tc.name, code, out)
			continue
		}
		if tc.taken {
			if out != "rc=0\n" {
				t.Errorf("%s: not taken, or said something:\n%s", tc.name, out)
			}
			continue
		}
		value := strings.TrimPrefix(tc.env[0], "APP_EMU_PORT=")
		want := "app-emulator: APP_EMU_PORT is an emulator's console port, even, from 5554 to 5584, " +
			"the ports adb finds an emulator on by itself after its server restarts, not '" + value + "'\nrc=1\n"
		if out != want {
			t.Errorf("%s: says\n%s\nwant\n%s", tc.name, out, want)
		}
	}
}

// TestSlotFailsWhereNoneIsFree: with all eight slots taken, by a lock,
// an emulator or a port in use, or with a port given that listens,
// slot_take fails, every slot said with why, and holds no lock; so it
// does where ss cannot say what listens, and for a port variable that is
// no port, one of twenty digits among them.
func TestSlotFailsWhereNoneIsFree(t *testing.T) {
	all := slotSetup{
		before:    lockSlot0,
		devices:   []string{"emulator-5582\tdevice"},
		listening: []string{"9447", "9450", "62949", "62952", "23953", "5571"},
	}
	for _, tc := range []struct {
		name  string
		setup slotSetup
		says  []string
		held  []int
	}{
		{"all eight taken", all, []string{
			"slot 0 passed over: another run holds it", "slot 1 passed over: emulator-5582 runs",
			"slot 2 passed over: port 9447 in use", "slot 3 passed over: port 9450 in use",
			"slot 4 passed over: port 62949 in use", "slot 5 passed over: port 62952 in use",
			"slot 6 passed over: port 23953 in use", "slot 7 passed over: port 5571 in use",
		}, []int{0}},
		{"a given port listens", slotSetup{env: []string{"APP_EMU_API_PORT_A=9999"}, listening: []string{"9999"}}, []string{
			"slot 0 passed over: port 9999 in use", "slot 7 passed over: port 9999 in use",
		}, nil},
		{"ss fails", slotSetup{ssFails: true}, []string{"ss cannot say which ports listen: Cannot open netlink socket: Protocol not supported"}, nil},
		{"no port", slotSetup{env: []string{"APP_EMU_CUBE_PORT_B=62942x"}}, []string{"APP_EMU_CUBE_PORT_B is a port, not '62942x'"}, nil},
		{"twenty digits", slotSetup{env: []string{"APP_EMU_API_PORT_B=12345678901234567890"}},
			[]string{"APP_EMU_API_PORT_B is a port, not '12345678901234567890'"}, nil},
		{"above 65535", slotSetup{env: []string{"APP_EMU_DISCOVERY_PORT_A=65536"}}, []string{"APP_EMU_DISCOVERY_PORT_A is a port, not '65536'"}, nil},
	} {
		out, code := slotTake(t, tc.setup)
		if code == 0 {
			t.Errorf("%s: exit 0:\n%s", tc.name, out)
		}
		for _, s := range append(tc.says, locks(tc.held, nil)...) {
			if !strings.Contains(out, s+"\n") {
				t.Errorf("%s: the output does not say %q:\n%s", tc.name, s, out)
			}
		}
		if strings.Contains(out, "taken ") {
			t.Errorf("%s: a slot taken:\n%s", tc.name, out)
		}
	}
}

// TestSlotPassesOverALockPathItCannotUse: a slot whose lock path is a
// FIFO, which would hang the open, or a link, whose target it does not
// make, or a file this user cannot open, another user's with no read for
// others, is passed over and said; a lock file it makes is readable by
// everyone, whatever the umask.
func TestSlotPassesOverALockPathItCannotUse(t *testing.T) {
	f := `f="$APP_EMU_LOCK_DIR/massimo-app-emulator-slot.0.lock"` + "\n"
	for _, tc := range []struct {
		name, before, says string
		// the locks after it: a file flock cannot open is said held
		held, other []int
	}{
		{"a FIFO", f + `mkfifo "$f"`, "slot 0 passed over: its lock file %s is no regular file", []int{1}, []int{0}},
		{"a link", f + `ln -s "$APP_EMU_LOCK_DIR/target" "$f"`, "slot 0 passed over: its lock file %s is no regular file", []int{1}, []int{0}},
		{"no read", f + `: >"$f"` + "\n" + `chmod 000 "$f"`, "slot 0 passed over: its lock file %s cannot be opened", []int{0, 1}, nil},
	} {
		if tc.name == "no read" && os.Geteuid() == 0 {
			continue // root opens any file
		}
		out, code, dirs := slotScript(t, slotSetup{before: tc.before}, slotTakeBody)
		says := fmt.Sprintf(tc.says, filepath.Join(dirs.locks, "massimo-app-emulator-slot.0.lock"))
		if code != 0 || !strings.Contains(out, says+"\n") || !strings.Contains(out, slotLine(1, 0)+"\n") {
			t.Errorf("%s: exit %d, want slot 1 and %q:\n%s", tc.name, code, says, out)
		}
		for _, s := range locks(tc.held, tc.other) {
			if !strings.Contains(out, s+"\n") {
				t.Errorf("%s: the output does not say %q:\n%s", tc.name, s, out)
			}
		}
		if exists(filepath.Join(dirs.locks, "target")) {
			t.Errorf("%s: the link's target was made", tc.name)
		}
	}
	out, code, dirs := slotScript(t, slotSetup{before: "umask 077"}, slotTakeBody)
	fi, err := os.Stat(filepath.Join(dirs.locks, "massimo-app-emulator-slot.0.lock"))
	if code != 0 || err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("under umask 077: exit %d, the lock file %v (%v), want mode 644:\n%s", code, fi, err, out)
	}
}

// exists says whether path is there, a link that leads nowhere among it.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// slotFree waits up to 5 s for path's lock to be free, as a fresh flock
// -n finds it, and says whether it is.
func slotFree(t *testing.T, path string) bool {
	t.Helper()
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip("no flock")
	}
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if exec.Command(flock, "-n", path, "true").Run() == nil {
			return true
		}
		if time.Now().After(end) {
			return false
		}
	}
}

// TestSlotCheckoutStopsASecondRun: slot_checkout locks the suite's
// directory, which another process then cannot lock; where another
// process holds it, slot_checkout fails and says that a run of this
// checkout is going.
func TestSlotCheckoutStopsASecondRun(t *testing.T) {
	out, code, _ := slotScript(t, slotSetup{}, `slot_checkout "$CHECKOUT"
if "$FLOCK" -n "$CHECKOUT" true; then echo "checkout free"; else echo "checkout held"; fi
`)
	if code != 0 || !strings.Contains(out, "checkout held\n") {
		t.Errorf("one run: exit %d:\n%s", code, out)
	}
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip("no flock")
	}
	// the other process holds the lock while its cat reads, and ends
	// with it once its input is closed; -o keeps the lock from cat
	checkout := t.TempDir()
	holder := exec.Command(flock, "-o", checkout, "cat")
	input, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = input.Close()
		_ = holder.Wait()
	}()
	for end := time.Now().Add(5 * time.Second); exec.Command(flock, "-n", checkout, "true").Run() == nil; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the other process did not lock the checkout")
		}
	}
	out, code, _ = slotScript(t, slotSetup{env: []string{"CHECKOUT=" + checkout}}, `slot_checkout "$CHECKOUT"`)
	if want := "a run of this checkout is going (it holds " + checkout + ")"; code == 0 || !strings.Contains(out, want) {
		t.Errorf("a second run: exit %d, want it to say %q:\n%s", code, want, out)
	}
}

// lockHeld says whether path's lock is held now, as a fresh flock -n
// finds it.
func lockHeld(t *testing.T, path string) bool {
	t.Helper()
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip("no flock")
	}
	return exec.Command(flock, "-n", path, "true").Run() != nil
}

// TestSlotHoldHandsTheLocksToOneHolder: after slot_hold the shell holds
// neither lock, so that no child it starts later has fd 8 or 9, while
// the holder keeps both, its fds 0, 1 and 2 on /dev/null, so that it
// keeps no output open, and the shell's wait does not wait for it; it
// lets them go within a few seconds of the shell's end; adb is asked
// without the checkout's lock.
func TestSlotHoldHandsTheLocksToOneHolder(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("no /proc/self/fd")
	}
	out, code, dirs := slotScript(t, slotSetup{}, `slot_checkout "$CHECKOUT"
slot_take
slot_hold
echo "child fds: $(ls /proc/self/fd | tr '\n' ' ')"
if "$FLOCK" -n "$slot_lock" true; then echo "slot free"; else echo "slot held"; fi
if "$FLOCK" -n "$CHECKOUT" true; then echo "checkout free"; else echo "checkout held"; fi
holder=""
for _ in $(seq 20); do
	for c in /proc/[0-9]*/cmdline; do
		if [ "$(tr '\0' ' ' <"$c" 2>/dev/null)" = "tail --pid=$$ -f /dev/null " ]; then holder=${c%/cmdline}; fi
	done
	[ -z "$holder" ] || break
	sleep 0.1
done
echo "holder fds: $(readlink "$holder/fd/0" "$holder/fd/1" "$holder/fd/2" | tr '\n' ' ')" || true
echo "holder has: $(ls "$holder/fd" | tr '\n' ' ')" || true
wait
echo "waited"
`)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, s := range []string{"slot held\n", "checkout held\n", "waited\n", "holder fds: /dev/null /dev/null /dev/null \n"} {
		if !strings.Contains(out, s) {
			t.Errorf("the output does not say %q:\n%s", s, out)
		}
	}
	if fds := lineFields(out, "child fds: "); len(fds) == 0 || slices.Contains(fds, "8") || slices.Contains(fds, "9") {
		t.Errorf("a child after slot_hold has the fds %q, want neither 8 nor 9:\n%s", fds, out)
	}
	if fds := lineFields(out, "holder has: "); !slices.Contains(fds, "8") || !slices.Contains(fds, "9") {
		t.Errorf("the holder has the fds %q, want 8 and 9:\n%s", fds, out)
	}
	b, err := os.ReadFile(filepath.Join(dirs.fakes, "adb-fds"))
	if err != nil || slices.Contains(strings.Fields(string(b)), "8") || slices.Contains(strings.Fields(string(b)), "9") {
		t.Errorf("adb was asked with fds %q (%v)", b, err)
	}
	if !slotFree(t, filepath.Join(dirs.locks, "massimo-app-emulator-slot.0.lock")) {
		t.Error("the slot's lock is held 5 s after the shell's end")
	}
	if !slotFree(t, dirs.checkout) {
		t.Error("the checkout's lock is held 5 s after the shell's end")
	}
}

// lineFields is the fields of the output's line that opens with prefix,
// after it.
func lineFields(out, prefix string) []string {
	_, after, ok := strings.Cut(out, prefix)
	if !ok {
		return nil
	}
	line, _, _ := strings.Cut(after, "\n")
	return strings.Fields(line)
}

// slotWaits ends a slot script that holds both locks: it writes the
// shell's PID and the slot's lock file to $FAKE_ADB/ready, with 1 where
// the shell's INT trap is set (0 where INT was ignored as the shell
// started, which no trap can change), then waits up to 20 s in read, a
// builtin, so that no child of the shell's is left after it.
const slotWaits = `mkfifo "$FAKE_ADB/fifo"
exec 7<>"$FAKE_ADB/fifo"
echo "$$ $slot_lock $(trap -p INT | grep -c 'exit 130' || true)" >"$FAKE_ADB/ready.tmp"
mv "$FAKE_ADB/ready.tmp" "$FAKE_ADB/ready"
read -r -t 20 -u 7 || true
echo "not stopped"
`

// slotStart starts the prelude and body in bash under wrap (a command
// such as setsid -w, or none), and waits up to 10 s for the line the
// body writes to $FAKE_ADB/ready (slotWaits), whose fields it returns
// with the running command, its output and the directories; the command
// is killed at the test's end where it still runs.
func slotStart(t *testing.T, body string, wrap ...string) (cmd *exec.Cmd, ready []string, out *bytes.Buffer, dirs slotDirs) {
	t.Helper()
	sh, env, dirs := slotEnv(t, slotSetup{})
	args := append(append([]string{}, wrap...), sh, "-c", slotPrelude+body)
	cmd = exec.Command(args[0], args[1:]...)
	cmd.Env = env
	out = &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if b, err := os.ReadFile(filepath.Join(dirs.fakes, "ready")); err == nil {
			if ready = strings.Fields(string(b)); len(ready) == 3 {
				return cmd, ready, out, dirs
			}
			t.Fatalf("the script's ready line is %q", b)
		}
		if time.Now().After(end) {
			t.Fatal("the slot script did not get ready within 10 s")
		}
	}
}

// TestSlotHoldOutlivesASignalToTheGroup: a signal to the shell's whole
// process group, INT as Ctrl-C sends it or TERM as a timeout does, does
// not end the holder before the shell: the locks stay held while the
// shell's trap still cleans up, and are let go within a few seconds of
// its end.
func TestSlotHoldOutlivesASignalToTheGroup(t *testing.T) {
	setsid, err := exec.LookPath("setsid")
	if err != nil {
		t.Skip("no setsid")
	}
	kill, err := exec.LookPath("kill")
	if err != nil {
		t.Skip("no kill")
	}
	for _, tc := range []struct {
		sig  string
		code int
	}{{"INT", 130}, {"TERM", 143}} {
		// setsid -w: the shell leads a process group of its own, its PID
		// the group's, which setsid waits for
		cmd, ready, out, dirs := slotStart(t, `slot_checkout "$CHECKOUT"
slot_take >/dev/null
slot_hold
trap 'sleep 3; exit 130' INT
trap 'sleep 3; exit 143' TERM
`+slotWaits, setsid, "-w")
		group, lock := "-"+ready[0], ready[1]
		if tc.sig == "INT" && ready[2] != "1" {
			t.Log("INT was ignored as the shell started, as under a shell's background job: its case is left out")
			_ = exec.Command(kill, "-s", "KILL", "--", group).Run()
			_ = cmd.Wait()
			continue
		}
		if err := exec.Command(kill, "-s", tc.sig, "--", group).Run(); err != nil {
			t.Fatalf("%s to the group: %v", tc.sig, err)
		}
		// inside the trap's 3 s
		time.Sleep(time.Second)
		if !lockHeld(t, lock) || !lockHeld(t, dirs.checkout) {
			t.Errorf("%s: a lock was let go while the shell's trap still ran", tc.sig)
		}
		var exit *exec.ExitError
		if err := cmd.Wait(); !errors.As(err, &exit) || exit.ExitCode() != tc.code {
			t.Errorf("%s: the shell ended with %v, want exit %d from its trap:\n%s", tc.sig, err, tc.code, out)
		}
		if !slotFree(t, lock) || !slotFree(t, dirs.checkout) {
			t.Errorf("%s: a lock is held 5 s after the shell's end", tc.sig)
		}
	}
}

// TestSlotHoldEndsAfterAKill: after a kill -9 of the shell, once it is
// reaped, the holder lets both locks go within a few seconds.
func TestSlotHoldEndsAfterAKill(t *testing.T) {
	cmd, ready, _, dirs := slotStart(t, `slot_checkout "$CHECKOUT"
slot_take >/dev/null
slot_hold
`+slotWaits)
	if !lockHeld(t, ready[1]) || !lockHeld(t, dirs.checkout) {
		t.Fatal("the locks are not held before the kill")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if !slotFree(t, ready[1]) || !slotFree(t, dirs.checkout) {
		t.Error("a lock is held 5 s after the kill -9")
	}
}

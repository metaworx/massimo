// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package appemulator

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
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

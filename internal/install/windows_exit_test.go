// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package install

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A re-install stops the service and replaces massimo.exe. The service
// reports stopped just before its process exits, and until it has, a
// rename over its image is refused with access denied: CI's Windows
// service job failed so at the second massimo install. stopService
// itself needs the service manager and an administrator, and that job
// is its test; these test its parts with a child process holding its
// image, as the service's process does.

// childLife is the environment variable that makes the test binary the
// child: it says up, lives that many milliseconds and exits.
const childLife = "MASSIMO_TEST_CHILD_MS"

func TestHelperChild(t *testing.T) {
	ms, err := strconv.Atoi(os.Getenv(childLife))
	if err != nil {
		t.Skip("only as the child of the tests in this file")
	}
	os.Stdout.WriteString("up\n")
	time.Sleep(time.Duration(ms) * time.Millisecond)
	os.Exit(0)
}

// startChild runs a copy of the test binary for life and returns once it
// runs; the copy is the image it holds.
func startChild(t *testing.T, life time.Duration) (*exec.Cmd, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "child.exe")
	if err := os.WriteFile(exe, b, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestHelperChild$")
	cmd.Env = append(os.Environ(), childLife+"="+strconv.Itoa(int(life/time.Millisecond)))
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // before t.TempDir removes the image
		cmd.Process.Kill()
		cmd.Wait()
	})
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "up\n" {
		t.Fatalf("the child said %q, %v", line, err)
	}
	return cmd, exe
}

// refusedWhileRunning checks the condition the tests rest on: a plain
// rename over the running child's image is refused.
func refusedWhileRunning(t *testing.T, exe string) {
	t.Helper()
	tmp := exe + ".plain"
	if err := os.WriteFile(tmp, []byte("plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, exe); err == nil || !renameHeld(err) {
		t.Fatalf("a rename over a running image: %v, want access denied", err)
	}
}

func TestWriteAtomicOverAnExitingImage(t *testing.T) {
	_, exe := startChild(t, 500*time.Millisecond)
	refusedWhileRunning(t, exe)
	start := time.Now()
	if err := writeAtomic(exe, []byte("new binary"), 0o755); err != nil {
		t.Fatalf("writeAtomic over the image of a process about to exit: %v", err)
	}
	t.Logf("replaced after %s", time.Since(start).Round(time.Millisecond))
	if b, err := os.ReadFile(exe); err != nil || string(b) != "new binary" {
		t.Fatalf("the target holds %q, %v", b, err)
	}
}

func TestWriteAtomicGivesUpOnAHeldImage(t *testing.T) {
	wait := renameHeldWait
	renameHeldWait = 300 * time.Millisecond
	defer func() { renameHeldWait = wait }()
	_, exe := startChild(t, time.Minute)
	refusedWhileRunning(t, exe)
	start := time.Now()
	err := writeAtomic(exe, []byte("new binary"), 0o755)
	if err == nil || !renameHeld(err) {
		t.Fatalf("writeAtomic over a running image: %v, want access denied", err)
	}
	if d := time.Since(start); d < renameHeldWait {
		t.Errorf("gave up after %s, before renameHeldWait (%s)", d, renameHeldWait)
	}
	if b, err := os.ReadFile(exe); err != nil || !strings.HasPrefix(string(b), "MZ") {
		t.Errorf("the image changed: %.8q, %v", b, err)
	}
	tmp := filepath.Join(filepath.Dir(exe), "."+filepath.Base(exe)+".tmp")
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the temp file is left: %v", err)
	}
}

func TestWaitExitReturnsOnceTheProcessExited(t *testing.T) {
	if openProcess(0) != 0 {
		t.Fatal("openProcess(0) opened something")
	}
	cmd, _ := startChild(t, 300*time.Millisecond)
	pid := uint32(cmd.Process.Pid)
	h := openProcess(pid)
	if h == 0 {
		t.Fatal("cannot open the child")
	}
	defer windows.CloseHandle(h)
	// openProcess asks for nothing but waiting; the exit code needs more.
	q, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(q)
	if err := waitExit(h, pid, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(q, &code); err != nil {
		t.Fatal(err)
	}
	if code != 0 { // STILL_ACTIVE, 259, while it runs
		t.Fatalf("the child's exit code is %d after waitExit", code)
	}
}

func TestWaitExitTimesOut(t *testing.T) {
	cmd, _ := startChild(t, time.Minute)
	pid := uint32(cmd.Process.Pid)
	h := openProcess(pid)
	if h == 0 {
		t.Fatal("cannot open the child")
	}
	defer windows.CloseHandle(h)
	err := waitExit(h, pid, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not exit within") {
		t.Fatalf("waitExit on a running process: %v", err)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// fakeApt stands in for apt-get and dpkg-query: it records each command
// line and answers from the script, one reply per apt-get call in turn;
// dpkg-query answers with before until apt-get install ran, then after.
type fakeApt struct {
	before, after string
	updates       []aptReply // apt-get update, in turn; the last one repeats
	install       aptReply
	calls         []string
	installed     bool
}

type aptReply struct {
	out string
	err error
}

func (f *fakeApt) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	switch {
	case name == "dpkg-query":
		if f.installed {
			return []byte(f.after), nil
		}
		return []byte(f.before), nil
	case name == "apt-get" && args[0] == "update":
		r := aptReply{}
		if len(f.updates) > 0 {
			r = f.updates[0]
			if len(f.updates) > 1 {
				f.updates = f.updates[1:]
			}
		}
		return []byte(r.out), r.err
	case name == "apt-get" && args[0] == "install":
		if f.install.err == nil {
			f.installed = true
		}
		return []byte(f.install.out), f.install.err
	}
	return nil, fmt.Errorf("unexpected %s", name)
}

// aptHelper sets up a box apt owns massimo on, with f standing in for
// apt; it returns the request and status files.
func aptHelper(t *testing.T, f *fakeApt) (request, status string) {
	t.Helper()
	oldPackaged, oldRun, oldSource, oldVersion := packaged, aptRun, aptSourcePath, version
	oldWait, oldEvery, oldHealth := aptLockWait, aptLockEvery, healthEvery
	t.Cleanup(func() {
		packaged, aptRun, aptSourcePath, version = oldPackaged, oldRun, oldSource, oldVersion
		aptLockWait, aptLockEvery, healthEvery = oldWait, oldEvery, oldHealth
	})
	packaged = func() (install.Manager, bool) {
		return install.Manager{Name: "apt", Command: "sudo apt update && sudo apt install massimo"}, true
	}
	aptRun = f.run
	dir := t.TempDir()
	aptSourcePath = filepath.Join(dir, "massimo.sources")
	if err := os.WriteFile(aptSourcePath, []byte("Types: deb\nSuites: alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	version = "v" + strings.ReplaceAll(f.before, "~", "-")
	aptLockEvery, healthEvery = time.Millisecond, 10*time.Millisecond
	request, status = filepath.Join(dir, "update-request"), filepath.Join(dir, "status.json")
	// The channel of the request is the server's; apt follows its suite.
	if err := update.WriteRequest(request, "main"); err != nil {
		t.Fatal(err)
	}
	return request, status
}

// healthServer answers /healthz with the version given.
func healthServer(t *testing.T, tag string) string {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ok":true,"version":"massimo %s (go)"}`, tag)
	}))
	t.Cleanup(ts.Close)
	return ts.URL + "/healthz"
}

func readStatus(t *testing.T, path string) update.Status {
	t.Helper()
	st, err := update.ReadStatus(path)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// TestAptUpdateRunsTheFixedCommands: the helper runs exactly the two apt
// commands, whatever the request says, and reports the version apt
// installed once the server runs it.
func TestAptUpdateRunsTheFixedCommands(t *testing.T) {
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1"}
	req, status := aptHelper(t, f)
	health := healthServer(t, "v0.13.0-alpha.1")
	if err := runUpdate([]string{"-request", req, "-status", status, "-health", health}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"dpkg-query -W --showformat=${Version} massimo",
		"apt-get update -q -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/massimo.sources -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0",
		"apt-get install -y -q --only-upgrade -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold -o DPkg::Lock::Timeout=120 massimo",
		"dpkg-query -W --showformat=${Version} massimo",
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("ran\n%s\nwant\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
	st := readStatus(t, status)
	if st.State != "done" || st.From != "v0.12.0-alpha.1" || st.To != "v0.13.0-alpha.1" {
		t.Fatalf("status %+v", st)
	}
	if _, err := os.Stat(req); !os.IsNotExist(err) {
		t.Fatalf("the request was not taken: %v", err)
	}
}

// TestAptUpdateNothingNewer: the GitHub release can be out before the
// apt repository has it.
func TestAptUpdateNothingNewer(t *testing.T) {
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.12.0~alpha.1"}
	req, status := aptHelper(t, f)
	if err := runUpdate([]string{"-request", req, "-status", status, "-health", healthServer(t, "v0.12.0-alpha.1")}); err != nil {
		t.Fatal(err)
	}
	if st := readStatus(t, status); st.State != "up-to-date" || !strings.Contains(st.Message, "apt has nothing newer") {
		t.Fatalf("status %+v", st)
	}
}

// TestAptUpdateWaitsForTheListsLock: apt-get update does not wait for
// the lock itself, so the helper tries again.
func TestAptUpdateWaitsForTheListsLock(t *testing.T) {
	locked := aptReply{"E: Could not get lock /var/lib/apt/lists/lock. It is held by process 4242 (apt-get)\nE: Unable to lock directory /var/lib/apt/lists/", errors.New("exit status 100")}
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1", updates: []aptReply{locked, locked, {}}}
	req, status := aptHelper(t, f)
	if err := runUpdate([]string{"-request", req, "-status", status}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join(f.calls, "\n"), "apt-get update"); n != 3 {
		t.Fatalf("apt-get update ran %d times, want 3", n)
	}
	if st := readStatus(t, status); st.State != "done" || st.To != "v0.13.0-alpha.1" {
		t.Fatalf("status %+v", st)
	}
}

// TestAptUpdateBusy: a lock that stays taken is reported, and nothing is
// installed.
func TestAptUpdateBusy(t *testing.T) {
	locked := aptReply{"E: Could not get lock /var/lib/apt/lists/lock. It is held by process 4242 (apt-get)", errors.New("exit status 100")}
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1", updates: []aptReply{locked}}
	req, status := aptHelper(t, f)
	aptLockWait = 20 * time.Millisecond
	err := runUpdate([]string{"-request", req, "-status", status})
	if !errors.Is(err, errAptBusy) {
		t.Fatalf("got %v", err)
	}
	if f.installed || strings.Contains(strings.Join(f.calls, "\n"), "apt-get install") {
		t.Fatal("installed while apt was busy")
	}
	if st := readStatus(t, status); st.State != "failed" || !strings.Contains(st.Message, "apt is busy") {
		t.Fatalf("status %+v", st)
	}
	// The dpkg lock, once apt-get install's own wait ran out, reads the same.
	f = &fakeApt{before: "0.12.0~alpha.1", install: aptReply{"E: Unable to acquire the dpkg frontend lock (/var/lib/dpkg/lock-frontend), is another process using it?", errors.New("exit status 100")}}
	req, status = aptHelper(t, f)
	if err := runUpdate([]string{"-request", req, "-status", status}); !errors.Is(err, errAptBusy) {
		t.Fatalf("install: got %v", err)
	}
}

// TestAptUpdateFailures: apt's own error lines reach the status, and a
// source apt could not fetch is not "nothing newer".
func TestAptUpdateFailures(t *testing.T) {
	for _, c := range []struct {
		name string
		f    *fakeApt
		want string
	}{
		{"install", &fakeApt{before: "0.12.0~alpha.1", install: aptReply{"Reading package lists...\nE: Sub-process /usr/bin/dpkg returned an error code (1)", errors.New("exit status 100")}},
			"apt-get install: exit status 100: E: Sub-process /usr/bin/dpkg returned an error code (1)"},
		{"fetch", &fakeApt{before: "0.12.0~alpha.1", updates: []aptReply{{"W: Failed to fetch https://massimo.example.invalid/apt/dists/alpha/InRelease  Could not resolve host", nil}}},
			"apt-get update: the source could not be fetched: W: Failed to fetch"},
		{"not installed", &fakeApt{before: ""}, "the package massimo is not installed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req, status := aptHelper(t, c.f)
			err := runUpdate([]string{"-request", req, "-status", status})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
			if st := readStatus(t, status); st.State != "failed" || !strings.Contains(st.Message, c.want) {
				t.Fatalf("status %+v", st)
			}
		})
	}
}

// TestAptUpdateHealthFails: apt installed it, the server does not run
// it; nothing is downgraded, and the status names the way back.
func TestAptUpdateHealthFails(t *testing.T) {
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1"}
	req, status := aptHelper(t, f)
	health := healthServer(t, "v0.12.0-alpha.1")
	err := runUpdate([]string{"-request", req, "-status", status, "-health", health, "-health-timeout", "50ms"})
	if err == nil {
		t.Fatal("a server on the old version passed")
	}
	st := readStatus(t, status)
	if st.State != "failed" || st.To != "v0.13.0-alpha.1" ||
		!strings.Contains(st.Message, "apt installed v0.13.0-alpha.1") || !strings.Contains(st.Message, "sudo apt install massimo=0.12.0~alpha.1") {
		t.Fatalf("status %+v", st)
	}
	if strings.Count(strings.Join(f.calls, "\n"), "apt-get install") != 1 {
		t.Fatalf("apt ran again: %v", f.calls)
	}
}

// TestAptUpdateNeedsTheSource: without the massimo source apt is not
// asked at all.
func TestAptUpdateNeedsTheSource(t *testing.T) {
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1"}
	req, status := aptHelper(t, f)
	os.Remove(aptSourcePath)
	err := runUpdate([]string{"-request", req, "-status", status})
	if err == nil || !strings.Contains(err.Error(), install.AptSource) {
		t.Fatalf("got %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran %v", f.calls)
	}
	if st := readStatus(t, status); st.State != "failed" || !strings.Contains(st.Message, install.AptSource) {
		t.Fatalf("status %+v", st)
	}
}

// TestAptUpdateOnlyAsTheHelper: run by hand, update still leaves the
// install to apt and runs nothing.
func TestAptUpdateOnlyAsTheHelper(t *testing.T) {
	f := &fakeApt{before: "0.12.0~alpha.1", after: "0.13.0~alpha.1"}
	_, status := aptHelper(t, f)
	err := runUpdate([]string{"-yes", "-status", status})
	if err == nil || !strings.Contains(err.Error(), "managed by apt") {
		t.Fatalf("got %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran %v", f.calls)
	}
}

func TestDebTag(t *testing.T) {
	for v, want := range map[string]string{"0.13.0~alpha.1": "v0.13.0-alpha.1", "1.0.0": "v1.0.0", "0.13.0~alpha.1~rc~2": "v0.13.0-alpha.1-rc-2"} {
		if got := debTag(v); got != want {
			t.Errorf("debTag(%q) = %q, want %q", v, got, want)
		}
	}
}

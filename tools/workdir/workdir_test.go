// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package workdir

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// shells are the shells the library runs in, sh first, as the UI suite's
// run.sh is, then bash, as the emulator suite's is, where there is one;
// the tests are skipped where there is no sh, and on Windows, where the
// suites do not run.
func shells(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the suites' scripts run on Linux and macOS")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	found := []string{sh}
	if bash, err := exec.LookPath("bash"); err == nil {
		found = append(found, bash)
	}
	return found
}

// library is workdir.sh's absolute path, which a script sources from any
// directory.
func library(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("workdir.sh")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// run runs script in shell after the library is sourced, under set -eu
// as the suites' scripts run (set -euo pipefail in bash), with TMPDIR
// set to tmp and the extra environment env; it returns the script's
// standard output, its standard error and its exit code.
func run(t *testing.T, shell, tmp, script string, env ...string) (stdout, stderr string, code int) {
	t.Helper()
	opts := "set -eu"
	if filepath.Base(shell) == "bash" {
		opts = "set -euo pipefail"
	}
	cmd := exec.Command(shell, "-c", opts+`; . "$WORKDIR_LIB"; `+script)
	// KEEP= first, so that the caller's KEEP=1 is not the test's
	cmd.Env = append(os.Environ(), append([]string{"KEEP=", "TMPDIR=" + tmp, "WORKDIR_LIB=" + library(t)}, env...)...)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("%s: %v", shell, err)
		}
		code = exit.ExitCode()
	}
	return out.String(), errs.String(), code
}

// deadPID is the PID of a process that has ended and been waited for,
// which no process holds then.
func deadPID(t *testing.T, shell string) int {
	t.Helper()
	cmd := exec.Command(shell, "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// plant is a directory planted under tmp: its name, the lines of its pid
// (none: no pid), whether it has kept, and its age.
type plant struct {
	name string
	pids []string
	kept bool
	age  time.Duration
}

// planted makes each directory under tmp, then gives it its age: the
// files written into a directory change its time, so the times come last.
func planted(t *testing.T, tmp string, plants ...plant) {
	t.Helper()
	for _, p := range plants {
		d := filepath.Join(tmp, p.name)
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if p.pids != nil {
			if err := os.WriteFile(filepath.Join(d, "pid"), []byte(strings.Join(p.pids, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if p.kept {
			if err := os.WriteFile(filepath.Join(d, "kept"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, p := range plants {
		aged(t, filepath.Join(tmp, p.name), p.age)
	}
}

// aged sets path's times to age ago.
func aged(t *testing.T, path string, age time.Duration) {
	t.Helper()
	at := time.Now().Add(-age)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// exists says whether path is there.
func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

// TestSweepRemovesTheGoneRunsAlone: workdir_sweep removes the user's
// directories of its name whose run is gone and that are older than a
// day, a directory without pid among them, and leaves one with a PID
// that runs, whichever of its lines it is, one that KEEP=1 kept, one an
// hour old, a file of the name and a directory of another name; lines of
// pid that are no PID, 0 among them, are passed over.
func TestSweepRemovesTheGoneRunsAlone(t *testing.T) {
	for _, shell := range shells(t) {
		tmp := t.TempDir()
		dead := strconv.Itoa(deadPID(t, shell))
		live := strconv.Itoa(os.Getpid())
		day2 := 48 * time.Hour
		planted(t, tmp,
			plant{name: "suite.dead", pids: []string{dead}, age: day2},
			plant{name: "suite.live", pids: []string{live}, age: day2},
			plant{name: "suite.both", pids: []string{dead, live}, age: day2},
			plant{name: "suite.kept", pids: []string{dead}, kept: true, age: day2},
			plant{name: "suite.young", pids: []string{dead}, age: time.Hour},
			plant{name: "suite.nopid", age: day2},
			plant{name: "suite.junk", pids: []string{"x", "0", "", "-1", "0" + live, live + " " + live}, age: day2},
			plant{name: "other.dead", pids: []string{dead}, age: day2},
		)
		file := filepath.Join(tmp, "suite.file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		aged(t, file, day2)
		if out, errs, code := run(t, shell, tmp, "workdir_sweep suite"); code != 0 || out != "" || errs != "" {
			t.Fatalf("%s: exit %d, output %q, errors %q", shell, code, out, errs)
		}
		for name, stays := range map[string]bool{
			"suite.dead": false, "suite.live": true, "suite.both": true, "suite.kept": true,
			"suite.young": true, "suite.nopid": false, "suite.junk": false, "other.dead": true, "suite.file": true,
		} {
			if got := exists(t, filepath.Join(tmp, name)); got != stays {
				t.Errorf("%s: %s there after the sweep: %v, want %v", shell, name, got, stays)
			}
		}
	}
}

// TestSweepRefusesANameThatIsNone: a name that is empty, or holds
// anything but letters, digits, - and _, sweeps nothing and says why,
// so that a sweep's pattern never reaches beyond its suite's directories.
func TestSweepRefusesANameThatIsNone(t *testing.T) {
	for _, shell := range shells(t) {
		tmp := t.TempDir()
		planted(t, tmp, plant{name: ".hidden", age: 48 * time.Hour}, plant{name: "a.b.dead", age: 48 * time.Hour})
		for _, name := range []string{"", "a.b", "*", "../x"} {
			_, errs, code := run(t, shell, tmp, `workdir_sweep "$NAME"`, "NAME="+name)
			if code == 0 || !strings.Contains(errs, "is not a work directory's name") {
				t.Errorf("%s: name %q: exit %d, errors %q", shell, name, code, errs)
			}
		}
		for _, name := range []string{".hidden", "a.b.dead"} {
			if !exists(t, filepath.Join(tmp, name)) {
				t.Errorf("%s: %s swept by a name that is none", shell, name)
			}
		}
	}
}

// TestMakeSweepsAndMakesTheRunsDirectory: workdir_make sweeps first,
// then makes <name>.XXXXXX under TMPDIR with the caller's PID in pid,
// kept only under KEEP=1, and prints its path.
func TestMakeSweepsAndMakesTheRunsDirectory(t *testing.T) {
	re := regexp.MustCompile(`^suite\.[A-Za-z0-9]{6}$`)
	for _, shell := range shells(t) {
		for _, keep := range []string{"", "1"} {
			tmp := t.TempDir()
			planted(t, tmp, plant{name: "suite.old", pids: []string{strconv.Itoa(deadPID(t, shell))}, age: 48 * time.Hour})
			out, errs, code := run(t, shell, tmp, `work=$(workdir_make suite); printf '%s\n%s\n' "$work" "$$"`, "KEEP="+keep)
			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if code != 0 || len(lines) != 2 {
				t.Fatalf("%s, KEEP=%s: exit %d, output %q, errors %q", shell, keep, code, out, errs)
			}
			work, pid := lines[0], lines[1]
			if filepath.Dir(work) != tmp || !re.MatchString(filepath.Base(work)) {
				t.Errorf("%s, KEEP=%s: the work directory %q, want suite.XXXXXX under %s", shell, keep, work, tmp)
			}
			if fi, err := os.Stat(work); err != nil || !fi.IsDir() {
				t.Fatalf("%s, KEEP=%s: %s is no directory: %v", shell, keep, work, err)
			}
			if b, err := os.ReadFile(filepath.Join(work, "pid")); err != nil || string(b) != pid+"\n" {
				t.Errorf("%s, KEEP=%s: pid holds %q (%v), want the caller's PID %s", shell, keep, b, err, pid)
			}
			if got := exists(t, filepath.Join(work, "kept")); got != (keep == "1") {
				t.Errorf("%s, KEEP=%s: kept there: %v", shell, keep, got)
			}
			if exists(t, filepath.Join(tmp, "suite.old")) {
				t.Errorf("%s, KEEP=%s: the gone run's directory was not swept", shell, keep)
			}
		}
	}
}

// TestMakeFailsWhereItCannot: workdir_make fails where mktemp cannot
// make the directory, a TMPDIR that is not there, and where the name is
// none, and the caller under set -e stops there, having printed nothing.
func TestMakeFailsWhereItCannot(t *testing.T) {
	for _, shell := range shells(t) {
		tmp := t.TempDir()
		for _, tc := range []struct{ tmp, name string }{{filepath.Join(tmp, "missing"), "suite"}, {tmp, "a/b"}} {
			out, _, code := run(t, shell, tc.tmp, `work=$(workdir_make "$NAME"); echo "made $work"`, "NAME="+tc.name)
			if code == 0 || out != "" {
				t.Errorf("%s: TMPDIR %s, name %q: exit %d, output %q", shell, tc.tmp, tc.name, code, out)
			}
		}
		if left, err := filepath.Glob(filepath.Join(tmp, "*")); err != nil || len(left) != 0 {
			t.Errorf("%s: a failed make left %q (%v)", shell, left, err)
		}
	}
}

// TestPidAddsALine: workdir_pid adds its PID to pid, after the caller's.
func TestPidAddsALine(t *testing.T) {
	for _, shell := range shells(t) {
		tmp := t.TempDir()
		out, errs, code := run(t, shell, tmp, `work=$(workdir_make suite); workdir_pid "$work" 4242; cat "$work/pid"; echo "$$"`)
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if code != 0 || len(lines) != 3 || lines[0] != lines[2] || lines[1] != "4242" {
			t.Errorf("%s: exit %d, pid and the caller's PID %q, errors %q", shell, code, out, errs)
		}
	}
}

// TestDoneRemovesOrKeeps: workdir_done removes the directory, saying
// nothing; under KEEP=1 it keeps it and says so with its path, the line
// opened by the caller's name.
func TestDoneRemovesOrKeeps(t *testing.T) {
	for _, shell := range shells(t) {
		for _, keep := range []string{"", "1"} {
			tmp := t.TempDir()
			work := filepath.Join(tmp, "suite.abcdef")
			planted(t, tmp, plant{name: "suite.abcdef", pids: []string{"1"}})
			out, errs, code := run(t, shell, tmp, `workdir_done "$WORK" who`, "WORK="+work, "KEEP="+keep)
			want := ""
			if keep == "1" {
				want = "who: KEEP=1, the work directory is kept: " + work + "\n"
			}
			if code != 0 || out != "" || errs != want {
				t.Errorf("%s, KEEP=%s: exit %d, output %q, errors %q, want %q", shell, keep, code, out, errs, want)
			}
			if got := exists(t, work); got != (keep == "1") {
				t.Errorf("%s, KEEP=%s: the directory there after done: %v", shell, keep, got)
			}
		}
	}
}

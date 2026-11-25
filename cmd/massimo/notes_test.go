// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The notes on the settings file, the lines a read set aside, as the
// start tells them (AP StartConfiguration, D3-B, D33-A): on stderr at
// each command's start but version's and help's, in config after the
// settings, and in serve's log after its server_started. The reload's
// are reload_test.go's.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/testkit"
)

// skipBuilt skips a test that runs the built massimo where it cannot.
func skipBuilt(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the built binary has no .exe name")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
}

// Every command but version and help says each note on stderr at its
// start, after the version line, "warning: <file>: <note>"; config
// lists them after the settings too. A note names a line by its number
// and a secret given twice by its name, never a value.
func TestCommandsTellTheSettingsNotes(t *testing.T) {
	t.Parallel()
	skipBuilt(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "env")
	other := filepath.Join(dir, "other.env")
	values := []string{"placeholder-token-1", "placeholder-token-2", "placeholder-value-3", other,
		"00112233445566778899aabbccddeeff", "ffeeddccbbaa99887766554433221100"}
	const cubeKey = "MASSIMO_CUBE_KEQ0000000_KEY"
	text := tokenVar + "=" + values[0] + "\n" + env.HSTS + "=30\n" + tokenVar + "=" + values[1] + "\n" +
		"=" + values[2] + "\n" + env.FileVar + "=" + values[3] + "\n" + cubeKey + "=" + values[4] + "\n" + cubeKey + "=" + values[5] + "\n"
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	notes := []string{
		tokenVar + " is given on lines 1 and 3; the first wins",
		"line 4 has no name before its =, ignored",
		"line 5 sets " + env.FileVar + ", ignored: name the file with -env",
		cubeKey + " is given on lines 6 and 7; the first wins",
	}
	run := func(args ...string) (stdout, stderr string) {
		t.Helper()
		cmd := exec.Command(builtBinary(t), append([]string{"-env", file, "-data", dir}, args...)...)
		cmd.Env = guardEnv()
		var o, e strings.Builder
		cmd.Stdout, cmd.Stderr = &o, &e
		if err := cmd.Run(); err != nil {
			t.Fatalf("massimo %s: %v\n%s", strings.Join(args, " "), err, e.String())
		}
		for _, v := range values {
			if strings.Contains(o.String()+e.String(), v) {
				t.Fatalf("massimo %s: a value of the file, %q, in its output:\n%s\n%s", strings.Join(args, " "), v, o.String(), e.String())
			}
		}
		return o.String(), e.String()
	}
	var warnings, listed strings.Builder
	for _, n := range notes {
		warnings.WriteString("warning: " + file + ": " + n + "\n")
		listed.WriteString("  " + n + "\n")
	}
	stdout, stderr := run("config")
	version, rest, _ := strings.Cut(stderr, "\n")
	if !strings.HasPrefix(version, "massimo ") || rest != warnings.String() {
		t.Fatalf("config: stderr\n%s\nwant the version line, then\n%s", stderr, warnings.String())
	}
	// stdout stays data only: the notes listed, never the warnings.
	if strings.Contains(stdout, "warning:") {
		t.Fatalf("config: a warning on stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "\nthe settings file, lines set aside:\n"+listed.String()) {
		t.Fatalf("config: the notes not listed after the settings:\n%s", stdout)
	}
	if i, j := strings.Index(stdout, "\nvariables:\n"), strings.Index(stdout, "lines set aside"); i < 0 || j < i {
		t.Fatalf("config: the notes before the settings:\n%s", stdout)
	}
	for _, args := range [][]string{{"version"}, {"help"}} {
		stdout, stderr := run(args...)
		if strings.Contains(stderr, "warning:") {
			t.Fatalf("massimo %s: a note on stderr:\n%s", args[0], stderr)
		}
		// version's stdout is the version line alone, as config's
		// stderr opens with it; help writes nothing there.
		if want := map[string]string{"version": version + "\n", "help": ""}[args[0]]; stdout != want {
			t.Fatalf("massimo %s: stdout\n%q, want\n%q", args[0], stdout, want)
		}
	}
}

// serve says the notes the start found in its log, a settings_read at
// warn, why "start", after its server_started, which the log's sign
// shows; GET /api/v1/restart names no refusal of them. The file fixed
// while serve runs, no value changed, is a settings_read at info, which
// settles the warning; the next start over the fixed file writes none.
func TestServeLogsTheStartsNotes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no SIGTERM to send")
	}
	skipBuilt(t)
	file := filepath.Join(t.TempDir(), "env")
	data := t.TempDir()
	note := env.HSTS + " is given on lines 1 and 2; the first wins"
	if err := os.WriteFile(file, []byte(env.HSTS+"=30\n"+env.HSTS+"=60\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	type line struct {
		ID      int64          `json:"id"`
		Kind    string         `json:"kind"`
		Level   string         `json:"level"`
		Error   string         `json:"error"`
		Summary map[string]any `json:"summary"`
	}
	getJSON := func(s *builtServe, path string, v any) {
		t.Helper()
		code, body := get(t, "http://"+s.addr+path, "")
		if code != 200 || json.Unmarshal([]byte(body), v) != nil {
			t.Fatalf("GET %s: %d %s", path, code, body)
		}
	}
	// started waits for the start's server_started, the n'th, and for its
	// notes' line where it has notes, and answers the server's lines of
	// the starts and their notes, oldest first, and the ids of the log's
	// settings_read alerts.
	started := func(s *builtServe, n int, notes bool) ([]line, []int64) {
		t.Helper()
		var ours []line
		testkit.Eventually(t, "the start's lines", func() bool {
			var lines []line
			getJSON(s, "/api/v1/log?source=system&limit=100", &lines) // oldest first
			ours = nil
			starts := 0
			for _, l := range lines {
				if l.Kind == "server_started" || l.Kind == "settings_read" {
					ours = append(ours, l)
					if l.Kind == "server_started" {
						starts++
					}
				}
			}
			return starts == n && (!notes || ours[len(ours)-1].Kind == "settings_read")
		})
		var alerts struct {
			Alerts []struct {
				ID   int64  `json:"id"`
				Kind string `json:"kind"`
			} `json:"alerts"`
		}
		getJSON(s, "/api/v1/log/alerts", &alerts)
		var signs []int64
		for _, a := range alerts.Alerts {
			if a.Kind == "settings_read" {
				signs = append(signs, a.ID)
			}
		}
		return ours, signs
	}

	s := startServe(t, file, data)
	s.log.Has(t, 0, "the note on stderr at the start", testkit.Contains("warning: "+file+": "+note))
	ours, signs := started(s, 1, true)
	last, before := ours[len(ours)-1], ours[len(ours)-2]
	if len(ours) != 2 || before.Kind != "server_started" || last.Level != "warn" || last.Error != "the settings file: "+note || last.Summary["why"] != "start" {
		t.Fatalf("the start's lines: %+v", ours)
	}
	if len(signs) != 1 || signs[0] != last.ID {
		t.Fatalf("the settings_read alerts %v, want the start's %d", signs, last.ID)
	}
	var restart struct {
		Refused []any `json:"refused"`
	}
	getJSON(s, "/api/v1/restart", &restart)
	if restart.Refused != nil {
		t.Fatalf("GET /api/v1/restart: refused %v, want none", restart.Refused)
	}

	// The later line deleted while serve runs, which changes no value:
	// the watcher's read writes its settings_read at info, as the
	// reloader has the start's notes, and it settles their warning.
	if err := os.WriteFile(file, []byte(env.HSTS+"=30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var fixed line
	if !testkit.Within(25*time.Second, func() bool {
		var lines []line
		getJSON(s, "/api/v1/log?kind=settings_read&limit=100", &lines)
		for _, l := range lines {
			if l.ID > last.ID {
				fixed = l
				return true
			}
		}
		return false
	}) {
		t.Fatal("the fixed file: no settings_read from the watcher")
	}
	if fixed.Level != "info" || fixed.Error != "" || fixed.Summary["why"] != "the settings file changed" {
		t.Fatalf("the fixed file's settings_read: %+v", fixed)
	}
	if _, signs := started(s, 1, false); len(signs) != 0 {
		t.Fatalf("the fixed file: the settings_read alerts %v, want none", signs)
	}
	s.stop(t)

	s = startServe(t, file, data)
	ours, signs = started(s, 2, false)
	if last := ours[len(ours)-1]; last.Kind != "server_started" || len(ours) != 4 || len(signs) != 0 {
		t.Fatalf("the fixed file's start: the lines %+v, the settings_read alerts %v", ours, signs)
	}
	s.log.HasNot(t, 0, "a note on stderr at the fixed file's start", testkit.Contains("warning: "))
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package install

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/svc/mgr"

	"go.metaworx.ch/massimo/internal/env"
)

// On Windows, where PowerShell 5.1's > writes UTF-16, a -settings file
// serve's start cannot read, or a path that is not there (refusedFiles),
// is refused with its error before the install does anything, before it
// reaches the service manager: it writes, says and runs nothing, which
// is what the test can see where no massimo service is installed, as on
// CI; also where the settings file is kept and it would not be copied. A
// dry run, which stops no service whatever it reaches, with ProgramFiles
// and ProgramData in a temp dir.
func TestInstallWindowsRefusesSettingsFirst(t *testing.T) {
	for _, f := range refusedFiles(t) {
		for _, kept := range []bool{false, true} {
			dir := t.TempDir()
			t.Setenv("ProgramFiles", dir)
			t.Setenv("ProgramData", dir)
			if kept {
				p := WindowsPaths(dir, dir)
				os.MkdirAll(filepath.Dir(p.Env), 0o700)
				os.WriteFile(p.Env, []byte("MASSIMO_API_TOKEN=mine\r\n"), 0o600)
			}
			before := tree(t, dir)
			rec := &recorder{}
			var out bytes.Buffer
			err := InstallWindows(Options{Settings: f.path, DryRun: true, Out: &out, Run: rec.run})
			what := fmt.Sprintf("%s, kept %v", f.what, kept)
			if !refusedAs(err, f) {
				t.Errorf("%s: %v, want %s", what, err, f.refusal())
			}
			if out.Len() != 0 || len(rec.cmds) != 0 {
				t.Errorf("%s: before the refusal said %q, ran %q", what, out.String(), rec.cmds)
			}
			if after := tree(t, dir); !maps.Equal(after, before) {
				t.Errorf("%s: written before the refusal: %q, before %q", what, after, before)
			}
		}
	}
}

// A settings file a re-install keeps, which serve refuses for a line
// that is not KEY=VALUE, does not stop the install: Windows' install
// reads it leniently, as Linux's does (readSettings), and says the line
// as a warning, before anything else. A dry run, as above.
func TestInstallWindowsKeepsAFileServeRefuses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramData", dir)
	p := WindowsPaths(dir, dir)
	os.MkdirAll(filepath.Dir(p.Env), 0o700)
	os.WriteFile(p.Env, []byte("MASSIMO_API_TOKEN=mine\r\nnot a setting\r\n"), 0o600)
	var out bytes.Buffer
	if err := InstallWindows(Options{DryRun: true, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), p.Env+" exists; kept it\n") || !strings.Contains(out.String(), "would start the service") {
		t.Errorf("the kept file not kept, or the service not started:\n%s", out.String())
	}
	if note := "warning: " + p.Env + ": line 2 is not KEY=VALUE: serve refuses the file until it is fixed\n"; !strings.HasPrefix(out.String(), note) {
		t.Errorf("the line not said first:\n%s", out.String())
	}
}

// A settings file a re-install keeps and cannot read, UTF-16 text, does
// not stop Windows' install either: it is said to be one the next start
// refuses, first, and the install goes on; Windows has no units that
// follow the settings' TLS, so there is nothing of it to keep (AP
// StartConfiguration, D51-A). A dry run, as above.
func TestInstallWindowsKeepsAFileItCannotRead(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramData", dir)
	p := WindowsPaths(dir, dir)
	os.MkdirAll(filepath.Dir(p.Env), 0o700)
	b, _ := os.ReadFile(utf16File(t, binary.LittleEndian, "", "MASSIMO_API_TOKEN=mine\r\nMASSIMO_TLS=self-signed\r\n", true))
	os.WriteFile(p.Env, b, 0o600)
	var out bytes.Buffer
	if err := InstallWindows(Options{DryRun: true, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	if warning := "warning: " + p.Env + ": UTF-16 text, save it as UTF-8: the next start refuses the file until it is fixed\n"; !strings.HasPrefix(out.String(), warning) || !strings.Contains(out.String(), "would start the service") {
		t.Errorf("the file not said first, or the service not started:\n%s", out.String())
	}
}

// -tls over a settings file Windows' install keeps and cannot read,
// UTF-16 text or a line too long, is refused with the parser's error
// before the install does anything, before it reaches the service
// manager, which stops the service. A dry run, as above.
func TestInstallWindowsRefusesTLSOverAFileItCannotRead(t *testing.T) {
	for _, u := range unreadable(t, "MASSIMO_API_TOKEN=mine\r\n") {
		dir := t.TempDir()
		t.Setenv("ProgramFiles", dir)
		t.Setenv("ProgramData", dir)
		p := WindowsPaths(dir, dir)
		os.MkdirAll(filepath.Dir(p.Env), 0o700)
		os.WriteFile(p.Env, u.text, 0o600)
		_, _, readErr := env.ReadFileLenient(p.Env)
		if readErr == nil {
			t.Fatalf("%s: the file read", u.what)
		}
		before := tree(t, dir)
		rec := &recorder{}
		var out bytes.Buffer
		err := InstallWindows(Options{TLS: "self-signed", DryRun: true, Out: &out, Run: rec.run})
		if want := "-tls: " + readErr.Error() + "; nothing was written"; err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %s", u.what, err, want)
		}
		if after := tree(t, dir); !maps.Equal(after, before) || out.Len() != 0 || len(rec.cmds) != 0 {
			t.Errorf("%s: before the refusal wrote, said %q, ran %q", u.what, out.String(), rec.cmds)
		}
	}
}

// On Windows too a -settings file whose values serve's start refuses is
// refused, every refusal named, before the install does anything (AP
// StartConfiguration, D42-B, D52-A); a kept one's are a warning, said
// before the install asks the service manager for the service, which it
// then stops, and the install goes on (D49-A, D50-A). Dry runs, as
// above; the service manager's connection is the test's, which sees
// what was said by then (connectManager).
func TestInstallWindowsChecksTheValues(t *testing.T) {
	settings := settingsFile(t, "MASSIMO_API_TOKEN=t\r\nMASSIMO_LOG_LEVEL=bad\r\nMASSIMO_HSTS=bad\r\n")
	want := "-settings: " + settings + ": serve's start refuses:\n" + `  MASSIMO_HSTS "bad": refused` + "\n" + `  MASSIMO_LOG_LEVEL "bad": refused` + "\nnothing was written"
	for _, kept := range []bool{false, true} {
		dir := t.TempDir()
		t.Setenv("ProgramFiles", dir)
		t.Setenv("ProgramData", dir)
		if kept {
			p := WindowsPaths(dir, dir)
			os.MkdirAll(filepath.Dir(p.Env), 0o700)
			os.WriteFile(p.Env, []byte("MASSIMO_API_TOKEN=mine\r\n"), 0o600)
		}
		before := tree(t, dir)
		rec := &recorder{}
		var out bytes.Buffer
		err := InstallWindows(Options{Settings: settings, DryRun: true, Out: &out, Run: rec.run, StartChecks: (&fakeChecks{}).run})
		if err == nil || err.Error() != want {
			t.Errorf("kept %v: %v, want %s", kept, err, want)
		}
		if after := tree(t, dir); !maps.Equal(after, before) || out.Len() != 0 || len(rec.cmds) != 0 {
			t.Errorf("kept %v: before the refusal wrote, said %q, ran %q", kept, out.String(), rec.cmds)
		}
	}

	dir := t.TempDir()
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramData", dir)
	p := WindowsPaths(dir, dir)
	os.MkdirAll(filepath.Dir(p.Env), 0o700)
	os.WriteFile(p.Env, []byte("MASSIMO_API_TOKEN=mine\r\nMASSIMO_HSTS=bad\r\n"), 0o600)
	var out bytes.Buffer
	saidBefore := "nothing: the service manager not asked"
	connectManager = func() (*mgr.Mgr, error) {
		saidBefore = out.String()
		return nil, errors.New("no service manager in the test")
	}
	t.Cleanup(func() { connectManager = connect })
	if err := InstallWindows(Options{DryRun: true, Out: &out, Run: (&recorder{}).run, StartChecks: (&fakeChecks{}).run}); err != nil {
		t.Fatal(err)
	}
	warning := "warning: " + p.Env + `: the next start refuses: MASSIMO_HSTS "bad": refused` + "\n"
	if saidBefore != warning {
		t.Errorf("said before the service manager was asked: %q, want %q", saidBefore, warning)
	}
	if !strings.Contains(out.String(), "would start the service") {
		t.Errorf("the service not started:\n%s", out.String())
	}
}

// On Windows, whose service manager picks the service's working
// directory, a relative path of the settings is not checked: the
// refusals of the check that reads it are left to the start, with a
// warning that the TLS settings are not checked; the others stand.
func TestInstallWindowsLeavesARelativePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramData", dir)
	settings := settingsFile(t, "MASSIMO_API_TOKEN=t\r\nMASSIMO_TLS_CERT=bad\r\nMASSIMO_HSTS=bad\r\n")
	var out bytes.Buffer
	err := InstallWindows(Options{Settings: settings, DryRun: true, Out: &out, Run: (&recorder{}).run, StartChecks: (&fakeChecks{}).run})
	if want := "-settings: " + settings + ": serve's start refuses:\n" + `  MASSIMO_HSTS "bad": refused` + "\nnothing was written"; err == nil || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
	if warning := "warning: " + settings + ": MASSIMO_TLS_CERT=bad is a relative path, which the service resolves from its own working directory: the TLS settings are not checked\n"; out.String() != warning {
		t.Errorf("said %q, want %q", out.String(), warning)
	}
}

// The settings saved as UTF-8 are copied as they are, the byte order
// mark too.
func TestWinSettingsCopiesUTF8(t *testing.T) {
	dir := t.TempDir()
	p := WindowsPaths(dir, dir)
	settings := utf16File(t, binary.LittleEndian, "\xEF\xBB\xBFMASSIMO_API_TOKEN=t\r\n", "", false)
	in := newInstaller(Options{Settings: settings, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	if startable, err := in.winSettings(p); err != nil || !startable {
		t.Fatalf("startable %v, %v", startable, err)
	}
	if b, _ := os.ReadFile(p.Env); string(b) != "\xEF\xBB\xBFMASSIMO_API_TOKEN=t\r\n" {
		t.Errorf("the copy: %q", b)
	}
}

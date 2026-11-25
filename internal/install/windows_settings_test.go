// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package install

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On Windows, where PowerShell 5.1's > writes UTF-16, a -settings file
// serve's start cannot read (refusedFiles) is refused with the parser's
// error before the install does anything, before it reaches the service
// manager: it writes, says and runs nothing, which is what the test can
// see where no massimo service is installed, as on CI; also where the
// settings file is kept and it would not be copied. A dry run, which
// stops no service whatever it reaches, with ProgramFiles and
// ProgramData in a temp dir.
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
// reads it only as text, for the template's lines (onlyTemplate) and
// for -tls's (setEnvLines), never with the parser. A dry run, as above.
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

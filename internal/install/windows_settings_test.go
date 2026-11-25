// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package install

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

// On Windows, where PowerShell 5.1's > writes UTF-16, a -settings file
// of it is refused with the parser's text before the install does
// anything: it writes, says and runs nothing, which is what the test can
// see where no massimo service is installed, as on CI. A dry run, which
// stops no service whatever it reaches, with ProgramFiles and
// ProgramData in a temp dir.
func TestInstallWindowsRefusesUTF16First(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramData", dir)
	settings := utf16File(t, "", "MASSIMO_API_TOKEN=t\r\n", true)
	before := tree(t, dir)
	rec := &recorder{}
	var out bytes.Buffer
	err := InstallWindows(Options{Settings: settings, DryRun: true, Out: &out, Run: rec.run})
	if want := settings + ": UTF-16 text, save it as UTF-8"; !errors.Is(err, env.ErrUTF16) || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
	if out.Len() != 0 || len(rec.cmds) != 0 {
		t.Errorf("before the refusal said %q, ran %q", out.String(), rec.cmds)
	}
	if after := tree(t, dir); len(after) != len(before) {
		t.Errorf("written before the refusal: %q", after)
	}
}

// The settings saved as UTF-8 are copied as they are, the byte order
// mark too.
func TestWinSettingsCopiesUTF8(t *testing.T) {
	dir := t.TempDir()
	p := WindowsPaths(dir, dir)
	settings := utf16File(t, "\xEF\xBB\xBFMASSIMO_API_TOKEN=t\r\n", "", false)
	in := newInstaller(Options{Settings: settings, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	if startable, err := in.winSettings(p); err != nil || !startable {
		t.Fatalf("startable %v, %v", startable, err)
	}
	if b, _ := os.ReadFile(p.Env); string(b) != "\xEF\xBB\xBFMASSIMO_API_TOKEN=t\r\n" {
		t.Errorf("the copy: %q", b)
	}
}

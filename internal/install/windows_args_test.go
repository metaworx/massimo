// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"strings"
	"testing"
)

func TestWindowsLayout(t *testing.T) {
	p := WindowsPaths("", `D:\Data\`)
	if p.Bin != `C:\Program Files\Massimo\massimo.exe` || p.Env != `D:\Data\Massimo\env` || p.Log != `D:\Data\Massimo\serve.log` {
		t.Fatalf("%+v", p)
	}
	args := ServiceArgs(p, "")
	if strings.Join(args, " ") != `-env D:\Data\Massimo\env -data D:\Data\Massimo serve -listen :8099 -log D:\Data\Massimo\serve.log` {
		t.Fatalf("%q", args)
	}
	cmdline := `"C:\Program Files\Massimo\massimo.exe" -env "C:\ProgramData\Massimo\env" serve -listen 192.168.1.200:80 -log x`
	if got := ListenFromCommandLine(cmdline); got != "192.168.1.200:80" {
		t.Fatalf("listen %q", got)
	}
	if got := ListenFromCommandLine(`massimo.exe serve`); got != "" {
		t.Fatalf("listen %q", got)
	}
	tmpl := windowsEnvTemplate(p)
	if !strings.Contains(tmpl, `MASSIMO_LOG=D:\Data\Massimo\serve.log`) || strings.Contains(tmpl, "/var/log") || strings.Contains(tmpl, "user massimo") {
		t.Fatal(tmpl)
	}
}

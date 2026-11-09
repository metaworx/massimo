// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"runtime"
	"testing"
)

func TestServiceCommand(t *testing.T) {
	for _, c := range []struct{ name, got, want string }{
		{"sudo", sudoCommand("/usr/bin/massimo", "store", "info"), "sudo -u _massimo /usr/bin/massimo store info"},
		{"sudo, a space in the path", sudoCommand("/Users/anna/My Tools/massimo", "tls", "show"), "sudo -u _massimo '/Users/anna/My Tools/massimo' tls show"},
		{"sudo, a quote in the path", sudoCommand("/opt/it's/massimo"), `sudo -u _massimo '/opt/it'\''s/massimo'`},
		{"windows", winCommand(`C:\Program Files\Massimo\massimo.exe`, "-data", `"C:\ProgramData\Massimo"`, "user", "list"),
			`& "C:\Program Files\Massimo\massimo.exe" -data "C:\ProgramData\Massimo" user list`},
		{"the owner line", ownerLine(BinPath), "sudo -u _massimo /usr/local/bin/massimo -env /etc/massimo/env -data /var/lib/massimo user add <name> -role owner"},
	} {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.name, c.got, c.want)
		}
	}
	want := sudoCommand("/bin/m", "x")
	if runtime.GOOS == "windows" {
		want = winCommand("/bin/m", "x")
	}
	if got := ServiceCommand("/bin/m", "x"); got != want {
		t.Errorf("ServiceCommand on %s: %q, want %q", runtime.GOOS, got, want)
	}
}

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
	if p.Request != `D:\Data\Massimo\update-request` || p.Status != `D:\Data\Massimo\update-status.json` || p.UpdateLog != `D:\Data\Massimo\update.log` {
		t.Fatalf("%+v", p)
	}
	args := ServiceArgs(p, "", false)
	if strings.Join(args, " ") != `-env D:\Data\Massimo\env -data D:\Data\Massimo serve -listen :8099 -log D:\Data\Massimo\serve.log -update-request D:\Data\Massimo\update-request -update-status D:\Data\Massimo\update-status.json` {
		t.Fatalf("%q", args)
	}
	args = ServiceArgs(p, "127.0.0.1:18099", true)
	if strings.Join(args, " ") != `-env D:\Data\Massimo\env -data D:\Data\Massimo serve -listen 127.0.0.1:18099 -log D:\Data\Massimo\serve.log` {
		t.Fatalf("without the helper: %q", args)
	}
	if u, err := HealthURL(":8099", false); err != nil || u != "http://127.0.0.1:8099/healthz" {
		t.Fatalf("health %q %v", u, err)
	}
	if u, err := HealthURL("192.168.1.5:8099", false); err != nil || u != "http://192.168.1.5:8099/healthz" {
		t.Fatalf("health %q %v", u, err)
	}
	if u, err := HealthURL("[::]:443", true); err != nil || u != "https://[::1]:443/healthz" {
		t.Fatalf("health over TLS %q %v", u, err)
	}
	if _, err := HealthURL("8099", false); err == nil {
		t.Fatal("health of a bare port")
	}
	// An address with no port the helper can ask, port 0 or a service
	// name, names serve's setting (AP StartConfiguration, D21-A).
	for _, bad := range []string{"127.0.0.1:0", ":http"} {
		want := `-listen or MASSIMO_LISTEN: the update helper cannot ask "` + bad + `" for /healthz: bad port`
		if _, err := HealthURL(bad, false); err == nil || err.Error() != want {
			t.Errorf("health of %s: %v, want %s", bad, err, want)
		}
	}
	cmdline := `"C:\Program Files\Massimo\massimo.exe" -env "C:\ProgramData\Massimo\env" serve -listen 192.168.1.200:80 -log x`
	if got := ListenFromCommandLine(cmdline); got != "192.168.1.200:80" {
		t.Fatalf("listen %q", got)
	}
	if got := ListenFromCommandLine(`massimo.exe serve`); got != "" {
		t.Fatalf("listen %q", got)
	}
	tmpl := windowsEnvTemplate(p)
	if !strings.Contains(tmpl, `MASSIMO_LOG=D:\Data\Massimo\serve.log`) || strings.Contains(tmpl, "/var/log") || strings.Contains(tmpl, "user "+User) {
		t.Fatal(tmpl)
	}
}

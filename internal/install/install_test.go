// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type recorder struct{ cmds []string }

func (r *recorder) run(name string, args ...string) error {
	r.cmds = append(r.cmds, name+" "+strings.Join(args, " "))
	return nil
}

func (r *recorder) has(prefix string) bool {
	for _, c := range r.cmds {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func setup(t *testing.T) (root, bin string) {
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, LogrotateDir), 0o755)
	bin = filepath.Join(t.TempDir(), "massimo-linux-armv6")
	os.WriteFile(bin, []byte("binary v1"), 0o755)
	return root, bin
}

func read(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallFresh(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, BinPath)) != "binary v1" {
		t.Fatal("binary")
	}
	env := filepath.Join(root, EnvPath)
	if !strings.Contains(read(t, env), "MASSIMO_CUBE_<SN>_ADDRESS") {
		t.Fatal("env template")
	}
	if runtime.GOOS != "windows" { // no Unix modes there
		if fi, _ := os.Stat(filepath.Join(root, BinPath)); fi.Mode().Perm() != 0o755 {
			t.Fatalf("binary mode %v", fi.Mode())
		}
		if fi, _ := os.Stat(env); fi.Mode().Perm() != 0o600 {
			t.Fatalf("env mode %v", fi.Mode())
		}
	}
	unit := read(t, filepath.Join(root, UnitDir, "massimo.service"))
	if !strings.Contains(unit, updateFlag) {
		t.Fatal("massimo.service lacks the update request flag")
	}
	// The store lands in the state directory, whatever the working
	// directory.
	if !strings.Contains(unit, " -data /var/lib/massimo serve ") || !strings.Contains(unit, "StateDirectory=massimo") {
		t.Fatal("massimo.service does not pass -data /var/lib/massimo")
	}
	for _, f := range []string{UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service", LogrotateDir + "/massimo"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Error(err)
		}
	}
	for _, c := range []string{"useradd --system", "chown massimo:massimo " + filepath.Join(root, EnvPath), "systemctl daemon-reload", "systemctl enable massimo.service", "systemctl enable --now massimo-update.path"} {
		if !rec.has(c) {
			t.Errorf("no %q in %q", c, rec.cmds)
		}
	}
	// A fresh template holds no Cubes: enabled, not started.
	if rec.has("systemctl restart") || !strings.Contains(out.String(), "not started") {
		t.Fatalf("started with the template: %q\n%s", rec.cmds, out.String())
	}
}

func TestInstallAgainKeepsSettingsAndEdits(t *testing.T) {
	root, bin := setup(t)
	if err := Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(root, EnvPath)
	os.WriteFile(env, []byte("MASSIMO_CUBE_KEQ0000001_ADDRESS=10.0.0.1\n"), 0o600)
	unit := filepath.Join(root, UnitDir, "massimo.service")
	os.WriteFile(unit, []byte("edited by hand\n"), 0o644)
	os.WriteFile(bin, []byte("binary v2"), 0o755)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("OTHER=1\n"), 0o600)

	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, BinPath)) != "binary v2" {
		t.Fatal("binary not updated")
	}
	if !strings.Contains(read(t, env), "10.0.0.1") {
		t.Fatal("existing settings overwritten")
	}
	if read(t, unit+".bak") != "edited by hand\n" || !strings.Contains(read(t, unit), "ExecStart=") {
		t.Fatal("edited unit not kept as .bak")
	}
	if !rec.has("systemctl restart massimo.service") {
		t.Fatalf("not restarted: %q", rec.cmds)
	}
	if !strings.Contains(out.String(), "massimo-update.path is current") {
		t.Fatalf("unchanged unit not reported:\n%s", out.String())
	}
}

func TestInstallWithoutHelperAndWithSettings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_CUBE_KEQ0000001_ADDRESS=10.0.0.1\n"), 0o600)
	rec := &recorder{}
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, NoUpdateHelper: true, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, filepath.Join(root, UnitDir, "massimo.service")), updateFlag) {
		t.Fatal("update flag left in without the helper")
	}
	if _, err := os.Stat(filepath.Join(root, UnitDir, "massimo-update.path")); err == nil {
		t.Fatal("helper installed")
	}
	if !strings.Contains(read(t, filepath.Join(root, EnvPath)), "10.0.0.1") || !rec.has("systemctl restart massimo.service") || rec.has("systemctl enable --now massimo-update.path") {
		t.Fatalf("settings or start: %q", rec.cmds)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, DryRun: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if len(rec.cmds) != 0 {
		t.Fatalf("ran %q", rec.cmds)
	}
	for _, p := range []string{BinPath, EnvPath, UnitDir + "/massimo.service"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s written", p)
		}
	}
	if !strings.Contains(out.String(), "would write "+UnitDir+"/massimo.service") {
		t.Fatal(out.String())
	}
}

func TestRemove(t *testing.T) {
	root, bin := setup(t)
	Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	rec := &recorder{}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{BinPath, UnitDir + "/massimo.service", UnitDir + "/massimo-update.path", LogrotateDir + "/massimo"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s still there", p)
		}
	}
	if _, err := os.Stat(filepath.Join(root, EnvPath)); err != nil {
		t.Fatal("settings removed without -purge")
	}
	if !rec.has("systemctl disable --now massimo-update.path massimo.service") || rec.has("userdel") {
		t.Fatalf("%q", rec.cmds)
	}
	rec = &recorder{}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, EnvDir)); err == nil || !rec.has("userdel massimo") {
		t.Fatalf("purge: %v %q", err, rec.cmds)
	}
}

func TestSettingsWarnings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte(`MASSIMO_CUBE_JEQ0000001_ADDRESS=192.168.1.118
MASSIMO_CUBE_JEQ0000001_PROXY=192.168.1.50
MASSIMO_CUBE_JEQ0000001_PROXY_SERIAL=JEQ0000009
export MASSIMO_CUBE_KEQ0000001_PROXY="192.168.1.49:62910"
# MASSIMO_CUBE_KEQ0000002_PROXY=192.168.1.51
MASSIMO_CUBE_KEQ0000003_PROXY=:62910
`), 0o600)
	var out bytes.Buffer
	local := func() ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.48"), net.ParseIP("192.168.1.50")}, nil
	}
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run, LocalAddrs: local}); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "no MASSIMO_API_TOKEN") {
		t.Errorf("no token warning:\n%s", o)
	}
	if !strings.Contains(o, "MASSIMO_CUBE_KEQ0000001_PROXY=192.168.1.49:62910 is not an address") {
		t.Errorf("no warning for .49:\n%s", o)
	}
	for _, quiet := range []string{"192.168.1.50 is not", "KEQ0000002", "KEQ0000003", "PROXY_SERIAL"} {
		if strings.Contains(o, quiet) {
			t.Errorf("unexpected warning about %s:\n%s", quiet, o)
		}
	}
}

func TestListenDropIns(t *testing.T) {
	for _, c := range []struct {
		addr, listen, health string
		tls, cap             bool
	}{
		{":80", "MASSIMO_LISTEN=:80", "http://127.0.0.1:80/healthz", false, true},
		{"0.0.0.0:8080", "MASSIMO_LISTEN=0.0.0.0:8080", "http://127.0.0.1:8080/healthz", false, false},
		{"192.168.1.200:80", "MASSIMO_LISTEN=192.168.1.200:80", "http://192.168.1.200:80/healthz", false, true},
		{"[::]:443", "MASSIMO_LISTEN=[::]:443", "http://[::1]:443/healthz", false, true},
		{"[::]:443", "MASSIMO_LISTEN=[::]:443", "https://[::1]:443/healthz", true, true},
		{"192.168.1.200:8443", "MASSIMO_LISTEN=192.168.1.200:8443", "https://192.168.1.200:8443/healthz", true, false},
	} {
		serve, health, err := listenDropIns(c.addr, c.tls)
		if err != nil || !strings.Contains(serve, c.listen) || !strings.Contains(health, c.health) || strings.Contains(serve, "CAP_NET_BIND_SERVICE") != c.cap {
			t.Errorf("%s: %v\n%s\n%s", c.addr, err, serve, health)
		}
		if listenFromDropIn([]byte(serve)) != c.addr {
			t.Errorf("%s: read back %q", c.addr, listenFromDropIn([]byte(serve)))
		}
	}
	for _, bad := range []string{"80", ":http", ":0", ":70000", "host"} {
		if _, _, err := listenDropIns(bad, false); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSetEnvLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, []byte("# settings\nexport MASSIMO_TLS=off\nMASSIMO_API_TOKEN=t\n"), 0o640)
	if err := setEnvLines(p, [][2]string{{"MASSIMO_TLS", "files"}, {"MASSIMO_TLS_CERT", "/etc/massimo/cert.pem"}}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "# settings\nMASSIMO_TLS=files\nMASSIMO_API_TOKEN=t\nMASSIMO_TLS_CERT=/etc/massimo/cert.pem\n" {
		t.Fatalf("%q", got)
	}
	if fi, _ := os.Stat(p); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", fi.Mode())
	}
	missing := filepath.Join(t.TempDir(), "new")
	if err := setEnvLines(missing, [][2]string{{"A", "1"}}); err != nil || read(t, missing) != "A=1\n" {
		t.Fatalf("new file: %v %q", err, read(t, missing))
	}
}

func TestInstallTLSFiles(t *testing.T) {
	root, bin := setup(t)
	src := t.TempDir()
	cert, key := filepath.Join(src, "fullchain.pem"), filepath.Join(src, "privkey.pem")
	os.WriteFile(cert, []byte("CERT"), 0o644)
	os.WriteFile(key, []byte("KEY"), 0o600)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, TLSCert: cert, TLSKey: key, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	certDst, keyDst := filepath.Join(root, EnvDir, certName), filepath.Join(root, EnvDir, keyName)
	if read(t, certDst) != "CERT" || read(t, keyDst) != "KEY" {
		t.Fatal("files not copied")
	}
	if fi, _ := os.Stat(keyDst); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", fi.Mode())
	}
	if !rec.has("chown massimo:massimo "+certDst) || !rec.has("chown massimo:massimo "+filepath.Join(root, EnvPath)) {
		t.Fatalf("not owned by the service user: %q", rec.cmds)
	}
	env := read(t, filepath.Join(root, EnvPath))
	for _, want := range []string{"MASSIMO_TLS=files\n", "MASSIMO_TLS_CERT=/etc/massimo/cert.pem\n", "MASSIMO_TLS_KEY=/etc/massimo/key.pem\n"} {
		if strings.Count(env, want) != 1 {
			t.Errorf("settings lack %q:\n%s", want, env)
		}
	}
	if !strings.HasPrefix(env, "# Settings of massimo serve") {
		t.Error("template lost")
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "MASSIMO_HEALTH=https://127.0.0.1:8099/healthz") {
		t.Fatal("the helper does not check https")
	}
	if _, err := os.Stat(filepath.Join(root, ListenDropIn)); err == nil {
		t.Fatal("a listen drop-in without -listen")
	}

	// A later install with -listen keeps TLS from the settings.
	out.Reset()
	if err := Install(Options{Root: root, Binary: bin, Listen: ":443", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "CAP_NET_BIND_SERVICE") ||
		!strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://127.0.0.1:443/healthz") {
		t.Fatalf("drop-ins after -listen:\n%s", out.String())
	}
	if read(t, certDst) != "CERT" || strings.Count(read(t, filepath.Join(root, EnvPath)), "MASSIMO_TLS=files\n") != 1 {
		t.Fatal("TLS not kept")
	}

	// -tls off: the settings say so, the helper goes back to http.
	if err := Install(Options{Root: root, Binary: bin, TLS: "off", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env = read(t, filepath.Join(root, EnvPath))
	if strings.Count(env, "MASSIMO_TLS=off\n") != 1 || strings.Contains(env, "MASSIMO_TLS=files") || !strings.Contains(env, "MASSIMO_TLS_CERT=/etc/massimo/cert.pem") {
		t.Fatalf("settings after -tls off:\n%s", env)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "http://127.0.0.1:443/healthz") {
		t.Fatal("the helper still checks https")
	}

	// -tls self-signed: the settings say so, the server makes the pair.
	if err := Install(Options{Root: root, Binary: bin, TLS: "self-signed", TLSNames: "massimo.example.net", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env = read(t, filepath.Join(root, EnvPath))
	if strings.Count(env, "MASSIMO_TLS=self-signed\n") != 1 || !strings.Contains(env, "MASSIMO_TLS_NAMES=massimo.example.net\n") {
		t.Fatalf("settings after -tls self-signed:\n%s", env)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://127.0.0.1:443/healthz") {
		t.Fatal("the helper does not check https for self-signed")
	}

	// Refused: a missing file, -tls off with files, a mode not there yet.
	for _, o := range []Options{{TLSCert: filepath.Join(src, "none.pem"), TLSKey: key}, {TLS: "off", TLSCert: cert, TLSKey: key}, {TLS: "files"}, {TLS: "self-signed", TLSCert: cert, TLSKey: key}, {TLS: "acme"}, {TLS: "yes"}} {
		o.Root, o.Binary, o.Out, o.Run = root, bin, &bytes.Buffer{}, rec.run
		if err := Install(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}

	// A dry run says what it would do and writes nothing.
	root2, bin2 := setup(t)
	out.Reset()
	if err := Install(Options{Root: root2, Binary: bin2, TLSCert: cert, TLSKey: key, DryRun: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would copy "+cert) || !strings.Contains(out.String(), "would set MASSIMO_TLS=files") {
		t.Fatalf("dry run:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root2, EnvDir, certName)); err == nil {
		t.Fatal("dry run copied")
	}
}

func TestTLSSettingsWarnings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_API_TOKEN=t\nMASSIMO_TLS=files\nMASSIMO_TLS_CERT=/etc/massimo/cert.pem\n"), 0o600)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "warning: MASSIMO_TLS_CERT=/etc/massimo/cert.pem") || !strings.Contains(out.String(), "warning: no MASSIMO_TLS_KEY") {
		t.Fatalf("no warning about the TLS files:\n%s", out.String())
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://") {
		t.Fatal("the settings' TLS not seen by the helper")
	}
}

func TestInstallListenKeptAcrossInstalls(t *testing.T) {
	root, bin := setup(t)
	run := (&recorder{}).run
	if err := Install(Options{Root: root, Binary: bin, Listen: ":80", Out: &bytes.Buffer{}, Run: run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "MASSIMO_LISTEN=:80") ||
		!strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "127.0.0.1:80/healthz") {
		t.Fatal("drop-ins")
	}
	unit := read(t, filepath.Join(root, UnitDir, "massimo.service"))
	if !strings.Contains(unit, "-listen ${MASSIMO_LISTEN}") || !strings.Contains(unit, "Environment=MASSIMO_LISTEN=:8099") {
		t.Fatal("massimo.service does not take the address from MASSIMO_LISTEN")
	}
	if !strings.Contains(read(t, filepath.Join(root, UnitDir, "massimo-update.service")), "-health ${MASSIMO_HEALTH}") {
		t.Fatal("massimo-update.service does not take the URL from MASSIMO_HEALTH")
	}
	// A later install without -listen keeps them.
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "MASSIMO_LISTEN=:80") || !strings.Contains(out.String(), "kept") {
		t.Fatalf("drop-in not kept:\n%s", out.String())
	}
	if err := Install(Options{Root: root, Binary: bin, Listen: "80", Out: &bytes.Buffer{}, Run: run}); err == nil {
		t.Fatal("bad -listen accepted")
	}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: run}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.Dir(ListenDropIn))); err == nil {
		t.Fatal("drop-in directory left behind")
	}
}

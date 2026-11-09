// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const aptMarker = "apt\nsudo apt update && sudo apt install massimo\nsudo apt remove massimo\n"

// packagedRoot is a root the Debian package has been installed into: the
// marker, the template and the settings file left as the template.
func packagedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(p, content string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		if err := os.WriteFile(filepath.Join(root, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(PackagedPath, aptMarker)
	write("/usr/share/massimo/env.template", envTemplate())
	write(EnvPath, envTemplate())
	return root
}

func TestReadMarker(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readMarker(filepath.Join(dir, "none")); ok {
		t.Fatal("a missing marker")
	}
	cases := []struct {
		name, content string
		want          Manager
		ok            bool
	}{
		{"three lines", aptMarker, Manager{Name: "apt", Command: "sudo apt update && sudo apt install massimo", Remove: "sudo apt remove massimo"}, true},
		{"one line", "brew\n", Manager{Name: "brew"}, true},
		{"spaces", "  dnf \n sudo dnf upgrade massimo \n", Manager{Name: "dnf", Command: "sudo dnf upgrade massimo"}, true},
		{"empty", "\n\n", Manager{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "_"))
			os.WriteFile(p, []byte(tc.content), 0o644)
			got, ok := readMarker(p)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %+v %v, want %+v %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestManagerText(t *testing.T) {
	m := Manager{Name: "apt", Command: "sudo apt install massimo", Remove: "sudo apt remove massimo"}
	if m.String() != "managed by apt: sudo apt install massimo" {
		t.Fatal(m.String())
	}
	if m.RemoveHint() != "remove it with: sudo apt remove massimo" {
		t.Fatal(m.RemoveHint())
	}
	if (Manager{Name: "brew"}).String() != "managed by brew" {
		t.Fatal("bare manager")
	}
	if !m.Configurable() || (Manager{Name: "brew"}).Configurable() {
		t.Fatal("configurable")
	}
}

func TestPackagedBesideBinary(t *testing.T) {
	prefix := t.TempDir()
	exe := filepath.Join(prefix, "bin", "massimo")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("binary"), 0o755)
	if _, ok := packagedIn(t.TempDir(), exe); ok {
		t.Fatal("no marker anywhere")
	}
	os.MkdirAll(filepath.Join(prefix, "share", "massimo"), 0o755)
	os.WriteFile(filepath.Join(prefix, "share", "massimo", "packaged"), []byte("brew\nbrew upgrade massimo-alpha\n"), 0o644)
	m, ok := packagedIn(t.TempDir(), exe)
	if !ok || m.Name != "brew" || m.Command != "brew upgrade massimo-alpha" {
		t.Fatalf("beside the binary: %+v %v", m, ok)
	}
	root := packagedRoot(t)
	if m, ok := packagedIn(root, ""); !ok || m.Name != "apt" {
		t.Fatalf("below the root: %+v %v", m, ok)
	}
}

func TestInstallPackagedConfiguresOnly(t *testing.T) {
	root := packagedRoot(t)
	bin := filepath.Join(t.TempDir(), "massimo-linux-arm64")
	os.WriteFile(bin, []byte("binary v2"), 0o755)
	settings := filepath.Join(t.TempDir(), "settings.env")
	os.WriteFile(settings, []byte("MASSIMO_CUBE_KEQ0000000_ADDRESS=192.0.2.10\nMASSIMO_API_TOKEN=t\n"), 0o600)
	rec := &recorder{}
	var out bytes.Buffer
	o := Options{Root: root, Binary: bin, Settings: settings, Listen: ":80", Out: &out, Run: rec.run}
	if err := Install(o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "managed by apt") {
		t.Fatalf("output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, BinPath)); err == nil {
		t.Fatal("the binary was written")
	}
	if _, err := os.Stat(filepath.Join(root, UnitDir, "massimo.service")); err == nil {
		t.Fatal("a unit was written")
	}
	if rec.has("useradd") || rec.has("systemctl enable massimo.service") {
		t.Fatalf("the package's work was done again: %v", rec.cmds)
	}
	if !rec.has("systemctl enable --now massimo-update.path") {
		t.Fatalf("the update helper not enabled: %v", rec.cmds)
	}
	if !strings.Contains(read(t, filepath.Join(root, EnvPath)), "MASSIMO_API_TOKEN=t") {
		t.Fatal("the settings were not copied over the template")
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "MASSIMO_LISTEN=:80") {
		t.Fatal("no listen drop-in")
	}
	if !rec.has("systemctl daemon-reload") || !rec.has("systemctl restart massimo.service") {
		t.Fatalf("not restarted: %v", rec.cmds)
	}

	// -tls goes into the settings as on a hand install, and the package's
	// update helper checks the server over https.
	out.Reset()
	if err := Install(Options{Root: root, TLS: "self-signed", TLSNames: "massimo.example.net", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env := read(t, filepath.Join(root, EnvPath))
	if strings.Count(env, "MASSIMO_TLS=self-signed\n") != 1 || !strings.Contains(env, "MASSIMO_TLS_NAMES=massimo.example.net\n") {
		t.Fatalf("settings after -tls self-signed:\n%s", env)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "MASSIMO_HEALTH=https://127.0.0.1:80/healthz") {
		t.Fatalf("health drop-in: %s", read(t, filepath.Join(root, HealthDropIn)))
	}
	cert, key := filepath.Join(t.TempDir(), "cert.pem"), filepath.Join(t.TempDir(), "key.pem")
	os.WriteFile(cert, []byte("CERT"), 0o644)
	os.WriteFile(key, []byte("KEY"), 0o600)
	if err := Install(Options{Root: root, TLSCert: cert, TLSKey: key, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	certDst := filepath.Join(root, EnvDir, certName)
	if read(t, certDst) != "CERT" || !rec.has("chown _massimo:_massimo "+certDst) || strings.Count(read(t, filepath.Join(root, EnvPath)), "MASSIMO_TLS=files\n") != 1 {
		t.Fatalf("TLS files on a packaged box: %v", rec.cmds)
	}

	// Filled in, the settings are kept whatever -settings names.
	os.WriteFile(filepath.Join(root, EnvPath), []byte("MASSIMO_API_TOKEN=mine\n"), 0o600)
	out.Reset()
	if err := Install(Options{Root: root, Settings: settings, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, EnvPath)) != "MASSIMO_API_TOKEN=mine\n" || !strings.Contains(out.String(), "kept it and did not copy") {
		t.Fatalf("settings replaced: %s", out.String())
	}

	// Still the template, nothing to start.
	root2 := packagedRoot(t)
	rec2 := &recorder{}
	out.Reset()
	if err := Install(Options{Root: root2, Out: &out, Run: rec2.run}); err != nil {
		t.Fatal(err)
	}
	if rec2.has("systemctl restart") || !strings.Contains(out.String(), "not started") || !strings.Contains(out.String(), "create the owner first: sudo -u _massimo /usr/bin/massimo -env /etc/massimo/env -data /var/lib/massimo user add <name> -role owner\n") {
		t.Fatalf("started with the template: %v %s", rec2.cmds, out.String())
	}

	// An older template, its comments changed, is the template still.
	root3 := packagedRoot(t)
	os.WriteFile(filepath.Join(root3, EnvPath), []byte("# an older comment\nMASSIMO_LOG=/var/log/massimo/serve.log\n"), 0o600)
	rec3 := &recorder{}
	out.Reset()
	if err := Install(Options{Root: root3, Out: &out, Run: rec3.run}); err != nil {
		t.Fatal(err)
	}
	if rec3.has("systemctl restart") || !strings.Contains(out.String(), "is still the template") {
		t.Fatalf("an older template: %v %s", rec3.cmds, out.String())
	}

	// Running on the template, set up through the web UI: restarted.
	rec4 := &recorder{}
	out.Reset()
	if err := Install(Options{Root: packagedRoot(t), Out: &out, Run: rec4.run, Output: isActive("active")}); err != nil {
		t.Fatal(err)
	}
	if !rec4.has("systemctl restart massimo.service") || strings.Contains(out.String(), "not started") {
		t.Fatalf("not restarted on the template: %v %s", rec4.cmds, out.String())
	}
}

func TestInstallPackagedBrewRefuses(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, filepath.Dir(PackagedPath)), 0o755)
	os.WriteFile(filepath.Join(root, PackagedPath), []byte("brew\nbrew upgrade massimo-alpha\n"), 0o644)
	err := Install(Options{Root: root, Out: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "managed by brew") {
		t.Fatalf("got %v", err)
	}
	err = InstallDarwin(Options{Root: root, Out: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "Homebrew") {
		t.Fatalf("darwin: got %v", err)
	}
}

func TestRemovePackagedRefuses(t *testing.T) {
	root := packagedRoot(t)
	for _, f := range []func(Options, bool) error{Remove, RemoveDarwin} {
		err := f(Options{Root: root, Out: &bytes.Buffer{}}, true)
		if err == nil || !strings.Contains(err.Error(), "sudo apt remove massimo") {
			t.Fatalf("got %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, EnvPath)); err != nil {
		t.Fatal("the settings went")
	}
}

func TestEnvTemplateEmbedded(t *testing.T) {
	if !strings.Contains(envTemplate(), "MASSIMO_LOG=/var/log/massimo/serve.log") {
		t.Fatal(envTemplate())
	}
}

// TestInstallPackagedUpdateHelper: -no-update-helper turns the package's
// helper off and takes its health drop-in; without it, install turns it
// on again, as on a hand install.
func TestInstallPackagedUpdateHelper(t *testing.T) {
	root := packagedRoot(t)
	rec := &recorder{}
	if err := Install(Options{Root: root, Listen: ":80", Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "MASSIMO_HEALTH=http://127.0.0.1:80/healthz") {
		t.Fatal("no health drop-in for -listen :80")
	}
	rec = &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, NoUpdateHelper: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !rec.has("systemctl disable --now massimo-update.path") || rec.has("systemctl enable --now massimo-update.path") || !gone(t, filepath.Join(root, HealthDropIn)) {
		t.Fatalf("-no-update-helper: %q\n%s", rec.cmds, out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ListenDropIn)); err != nil {
		t.Fatal("the listen drop-in went with the helper's")
	}
	rec = &recorder{}
	if err := Install(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !rec.has("systemctl enable --now massimo-update.path") {
		t.Fatalf("without -no-update-helper: %q", rec.cmds)
	}
}

func TestAptSuite(t *testing.T) {
	dir := t.TempDir()
	for content, want := range map[string]string{
		"# The Massimo apt repository\n# Suites: comment\nTypes: deb\nURIs: https://massimo.example.invalid/apt\nSuites: alpha\nComponents: main\n": "alpha",
		"types: deb\nsuites:   beta stable\n": "beta",
	} {
		p := filepath.Join(dir, "massimo.sources")
		os.WriteFile(p, []byte(content), 0o644)
		if got, err := AptSuite(p); err != nil || got != want {
			t.Errorf("%q: %q, %v; want %q", content, got, err, want)
		}
	}
	p := filepath.Join(dir, "none.sources")
	os.WriteFile(p, []byte("Types: deb\nSuites:\n"), 0o644)
	if _, err := AptSuite(p); err == nil {
		t.Error("an empty Suites field read")
	}
	if _, err := AptSuite(filepath.Join(dir, "missing.sources")); err == nil {
		t.Error("a missing file read")
	}
}

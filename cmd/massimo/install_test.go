// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"maps"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/serve"
)

// installOpts are the options of a hand install under a root of its own,
// with the start's checks (installChecks) and the settings file
// settings, saying to out; it runs no command.
func installOpts(t *testing.T, settings string, out *bytes.Buffer) install.Options {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "massimo")
	if err := os.WriteFile(bin, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, install.LogrotateDir), 0o755)
	return install.Options{Root: root, Binary: bin, Settings: settings, Out: out, Run: func(string, ...string) error { return nil },
		LocalAddrs: func() ([]net.IP, error) { return nil, nil }, StartChecks: installChecks}
}

// writeSettings writes text as a settings file and returns its path.
func writeSettings(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// fileTree is what is under root: each file's content and each
// directory, "dir", by its path.
func fileTree(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			t.Fatal(err)
		case d.IsDir():
			m[p] = "dir"
		default:
			b, _ := os.ReadFile(p)
			m[p] = string(b)
		}
		return nil
	})
	return m
}

// massimo install runs the start's checks on the -settings file's values
// alone (installChecks; AP StartConfiguration, D46-A, D47-A): a bad
// MASSIMO_HSTS and a bad _IGNORE of the file's Cube are refused, both, in
// the start's order and words, a line each, and nothing is written
// (D52-A); install's
// own environment counts for nothing, its bad MASSIMO_HSTS and a Cube's
// bad _IGNORE there, so that a good file installs beside them.
func TestInstallChecksTheFileAlone(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv(env.PerCube(env.Address, "KEQ0000001"), "127.0.0.1:1")
	t.Setenv(env.PerCube(env.Ignore, "KEQ0000001"), "zz")
	t.Setenv(env.HSTS, "year")
	bad := writeSettings(t, "MASSIMO_API_TOKEN=t\nMASSIMO_HSTS=-1\n"+env.PerCube(env.Address, "KEQ0000000")+"=127.0.0.1:2\n"+env.PerCube(env.Ignore, "KEQ0000000")+"=zz\n")
	var out bytes.Buffer
	o := installOpts(t, bad, &out)
	before := fileTree(t, o.Root)
	err := install.Install(o)
	_, hsts := hstsDays("-1")
	head := "-settings: " + bad + ": serve's start refuses:\n  " + hsts.Error() + "\n  " + env.PerCube(env.Ignore, "KEQ0000000") + ": "
	if err == nil || !strings.HasPrefix(err.Error(), head) || !strings.HasSuffix(err.Error(), "\nnothing was written") || strings.Count(err.Error(), "\n") != 3 || strings.Contains(err.Error(), "KEQ0000001") {
		t.Errorf("%v, want %s…\nnothing was written", err, head)
	}
	if after := fileTree(t, o.Root); !maps.Equal(after, before) || out.Len() != 0 {
		t.Errorf("before the refusal wrote, or said %q", out.String())
	}
	if err := install.Install(installOpts(t, writeSettings(t, "MASSIMO_API_TOKEN=t\n"), &bytes.Buffer{})); err != nil {
		t.Errorf("install's own environment read: %v", err)
	}
}

// A -tls-cert and -tls-key pair that does not load is refused as the
// start refuses it, before the install writes anything, where install
// itself checks only that the files are there (D48-A); a pair that
// loads is installed.
func TestInstallRefusesAPairThatDoesNotLoad(t *testing.T) {
	t.Parallel()
	cert, _, _ := testPair(t, t.TempDir())
	_, key, _ := testPair(t, t.TempDir())
	var out bytes.Buffer
	o := installOpts(t, "", &out)
	o.TLSCert, o.TLSKey = cert, key
	before := fileTree(t, o.Root)
	err := install.Install(o)
	loadErr := serve.CheckTLS(serve.TLS{Mode: "files", Cert: cert, Key: key})
	if want := "-tls files: serve's start refuses:\n  " + loadErr.Error() + "\nnothing was written"; err == nil || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
	if after := fileTree(t, o.Root); !maps.Equal(after, before) || out.Len() != 0 {
		t.Errorf("before the refusal wrote, or said %q", out.String())
	}
	o = installOpts(t, "", &bytes.Buffer{})
	o.TLSCert, o.TLSKey, _ = testPair(t, t.TempDir())
	if err := install.Install(o); err != nil {
		t.Errorf("a pair that loads: %v", err)
	}
}

// A settings file a re-install keeps, which names TLS files the start
// cannot load, is warned of in the start's words, its paths under the
// install's root, and the install goes on (D49-A).
func TestInstallKeptFileInTheStartsWords(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	o := installOpts(t, "", &out)
	os.MkdirAll(filepath.Join(o.Root, install.EnvDir), 0o750)
	os.WriteFile(filepath.Join(o.Root, install.EnvPath), []byte("MASSIMO_API_TOKEN=t\nMASSIMO_TLS=files\nMASSIMO_TLS_CERT=/etc/massimo/cert.pem\nMASSIMO_TLS_KEY=/etc/massimo/key.pem\n"), 0o600)
	if err := install.Install(o); err != nil {
		t.Fatal(err)
	}
	refused := serve.CheckTLS(serve.TLS{Mode: "files", Cert: filepath.Join(o.Root, "/etc/massimo/cert.pem"), Key: filepath.Join(o.Root, "/etc/massimo/key.pem")})
	if want := "warning: " + install.EnvPath + ": the next start refuses: " + refused.Error() + "\n"; strings.Count(out.String(), want) != 1 {
		t.Errorf("want %q once:\n%s", want, out.String())
	}
}

// A relative path of the settings file's is checked where the service
// resolves it, at its working directory under the root
// (/var/lib/massimo, the unit's): a pair there is installed, one that
// is not there is refused with its place, and so are the other TLS
// settings beside it, a mode the start does not know.
func TestInstallChecksARelativePathWhereTheServiceRuns(t *testing.T) {
	t.Parallel()
	relative := writeSettings(t, "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=files\nMASSIMO_TLS_CERT=tls/cert.pem\nMASSIMO_TLS_KEY=tls/key.pem\n")
	o := installOpts(t, relative, &bytes.Buffer{})
	dir := filepath.Join(o.Root, install.StateDir, "tls")
	os.MkdirAll(dir, 0o750)
	testPair(t, dir)
	if err := install.Install(o); err != nil {
		t.Errorf("a pair at the service's directory: %v", err)
	}
	o = installOpts(t, relative, &bytes.Buffer{})
	missing := serve.CheckTLS(serve.TLS{Mode: "files", Cert: filepath.Join(o.Root, install.StateDir, "tls/cert.pem"), Key: filepath.Join(o.Root, install.StateDir, "tls/key.pem")})
	if err := install.Install(o); err == nil || !strings.Contains(err.Error(), "\n  "+missing.Error()+"\n") {
		t.Errorf("a pair not there: %v, want %v", err, missing)
	}
	bogus := writeSettings(t, "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=bogus\nMASSIMO_TLS_CERT=tls/cert.pem\n")
	if err := install.Install(installOpts(t, bogus, &bytes.Buffer{})); err == nil || !strings.Contains(err.Error(), `"bogus": want off, files, self-signed or acme`) {
		t.Errorf("a mode beside a relative path: %v", err)
	}
}

// Each refusal of installChecks names the variables of the file its
// check reads, which install's relative paths go by.
func TestInstallChecksNameTheirVariables(t *testing.T) {
	t.Parallel()
	got := installChecks(map[string]string{env.TLS: "bogus", env.HSTS: "-1", tokenVar: "t"})
	if len(got) != 2 || !slices.Equal(got[0].Vars, []string{env.HSTS}) || !slices.Equal(got[1].Vars, []string{env.TLS}) {
		t.Errorf("%+v, want the HSTS check's and the TLS check's", got)
	}
}

// The install command hands the start's checks to install: a dry run of
// a -settings file with a bad MASSIMO_HSTS is refused.
func TestInstallCommandRunsTheStartChecks(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("-root is for Linux and macOS")
	}
	bad := writeSettings(t, "MASSIMO_API_TOKEN=t\nMASSIMO_HSTS=-1\n")
	err := runInstall([]string{"-root", t.TempDir(), "-dry-run", "-settings", bad})
	if _, hsts := hstsDays("-1"); err == nil || !strings.Contains(err.Error(), "serve's start refuses:\n  "+hsts.Error()+"\n") {
		t.Errorf("%v, want the start's %v", err, hsts)
	}
}

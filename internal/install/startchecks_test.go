// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeChecks stands in for serve's start's checks (Options.StartChecks,
// which cmd/massimo hands in with its table): it refuses every variable
// whose value's last element is "bad", in the order of their names, and
// keeps each set of values it was given (seen).
type fakeChecks struct{ seen []map[string]string }

func (f *fakeChecks) run(vars map[string]string) []StartRefusal {
	f.seen = append(f.seen, maps.Clone(vars))
	var out []StartRefusal
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		if filepath.Base(vars[k]) == "bad" {
			out = append(out, StartRefusal{Vars: []string{k}, Err: fmt.Errorf("%s %q: refused", k, vars[k])})
		}
	}
	return out
}

// An installKind is one of install's entry points but Windows', whose
// tests are their own (windows_settings_test.go).
type installKind struct {
	what             string
	packaged, darwin bool
}

var installKinds = []installKind{{"a hand install", false, false}, {"a packaged box", true, false}, {"macOS", false, true}}

// opts are the options of the entry point k under a root of its own,
// saying to out and running through rec, and the install to call.
func (k installKind) opts(t *testing.T, rec *recorder, out *bytes.Buffer) (root string, o Options, install func(Options) error) {
	t.Helper()
	root, bin := setup(t)
	if k.packaged {
		root = packagedRoot(t)
	}
	o, install = Options{Root: root, Binary: bin, Out: out, Run: rec.run}, Install
	if k.darwin {
		o, install = darwinOpts(root, bin, rec, out), InstallDarwin
	}
	return root, o, install
}

// keep writes text as the settings file under root, which the install
// keeps.
func keep(t *testing.T, root, text string) {
	t.Helper()
	os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
	if err := os.WriteFile(filepath.Join(root, EnvPath), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// settingsFile writes text as a -settings file and returns its path.
func settingsFile(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A -settings file whose values serve's start refuses is refused with
// every refusal, one a line, in the start's order, and the word that
// nothing was written, before the install writes, runs or says anything: by a hand
// install, by one on a packaged box and by one on macOS, in a dry run
// too, and where a settings file is kept and it would not be copied (AP
// StartConfiguration, D42-B, D52-A).
func TestInstallRefusesValuesTheStartRefuses(t *testing.T) {
	settings := settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_LOG_LEVEL=bad\nMASSIMO_HSTS=bad\n")
	want := "-settings: " + settings + ": serve's start refuses:\n" + `  MASSIMO_HSTS "bad": refused` + "\n" + `  MASSIMO_LOG_LEVEL "bad": refused` + "\nnothing was written"
	for _, k := range installKinds {
		for _, c := range []struct{ dry, kept bool }{{false, false}, {true, false}, {false, true}} {
			var out bytes.Buffer
			rec := &recorder{}
			root, o, install := k.opts(t, rec, &out)
			if c.kept {
				keep(t, root, "MASSIMO_API_TOKEN=mine\n")
			}
			before := tree(t, root)
			o.Settings, o.DryRun, o.StartChecks = settings, c.dry, (&fakeChecks{}).run
			err := install(o)
			what := fmt.Sprintf("%s, dry run %v, kept %v", k.what, c.dry, c.kept)
			if err == nil || err.Error() != want {
				t.Errorf("%s: %v, want %s", what, err, want)
			}
			if after := tree(t, root); !maps.Equal(after, before) {
				t.Errorf("%s: written before the refusal: %q, before %q", what, after, before)
			}
			if out.Len() != 0 || len(rec.cmds) != 0 {
				t.Errorf("%s: before the refusal said %q, ran %q", what, out.String(), rec.cmds)
			}
		}
	}
}

// A settings file a re-install keeps, whose values serve's start
// refuses, does not stop the install: its refusals are a warning each,
// in the start's order, and the install goes on and starts the service, so
// that an upgrade never fails on it (D49-A). By a hand install, by one
// on a packaged box and by one on macOS, without -settings and with one
// that is not copied over the kept file.
func TestInstallWarnsOfAKeptFileTheStartRefuses(t *testing.T) {
	good := settingsFile(t, "MASSIMO_API_TOKEN=u\n")
	const warning = "warning: " + EnvPath + `: the next start refuses: MASSIMO_HSTS "bad": refused` + "\n" +
		"warning: " + EnvPath + `: the next start refuses: MASSIMO_SIGHUP "bad": refused` + "\n"
	for _, k := range installKinds {
		for _, settings := range []string{"", good} {
			var out bytes.Buffer
			rec := &recorder{}
			root, o, install := k.opts(t, rec, &out)
			keep(t, root, "MASSIMO_API_TOKEN=mine\nMASSIMO_SIGHUP=bad\nMASSIMO_HSTS=bad\n")
			o.Settings, o.StartChecks = settings, (&fakeChecks{}).run
			what := fmt.Sprintf("%s, -settings %q", k.what, settings)
			if err := install(o); err != nil {
				t.Errorf("%s: %v", what, err)
				continue
			}
			s := out.String()
			if n := strings.Count(s, warning); n != 1 {
				t.Errorf("%s: the warning said %d times:\n%s", what, n, s)
			}
			if started := rec.has("systemctl restart massimo.service") || k.darwin && strings.Contains(s, "start "+Label); !started {
				t.Errorf("%s: not started: %q\n%s", what, rec.cmds, s)
			}
		}
	}
}

// On macOS the plist's -log-process-format none fixes the log's format
// on stderr (darwinFixed): the start never reads that line of the
// settings file, so that a bad one is no refusal there, of a -settings
// file or of a kept one, and the start's checks read none; elsewhere it
// is refused.
func TestInstallDarwinFixesTheProcessFormat(t *testing.T) {
	settings := settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_LOG_PROCESS_FORMAT=bad\n")
	for _, k := range installKinds {
		for _, kept := range []bool{false, true} {
			var out bytes.Buffer
			rec, checks := &recorder{}, &fakeChecks{}
			root, o, install := k.opts(t, rec, &out)
			o.StartChecks = checks.run
			if kept {
				keep(t, root, "MASSIMO_API_TOKEN=mine\nMASSIMO_LOG_PROCESS_FORMAT=bad\n")
			} else {
				o.Settings = settings
			}
			err := install(o)
			what := fmt.Sprintf("%s, kept %v", k.what, kept)
			refused := err != nil || strings.Contains(out.String(), "the next start refuses")
			if refused == k.darwin {
				t.Errorf("%s: refused %v (%v):\n%s", what, refused, err, out.String())
			}
			for _, v := range checks.seen {
				if k.darwin && v["MASSIMO_LOG_PROCESS_FORMAT"] != "none" {
					t.Errorf("%s: the checks read MASSIMO_LOG_PROCESS_FORMAT=%q", what, v["MASSIMO_LOG_PROCESS_FORMAT"])
				}
			}
		}
	}
}

// The start's checks read a settings file's values as the next start
// reads them after this install: -tls's variables in place of the
// file's, as the install writes them, so that -tls off over a file's
// MASSIMO_TLS_HTTP is no refusal; -tls-cert and -tls-key at their
// sources, which are not copied yet; a path of the file's own under
// Root, where the service finds it, a relative one at the service's
// working directory, the unit's and the plist's (serviceStart); and
// -acme-dns-hook's hook under Root, which the settings name as given.
func TestInstallChecksTheValuesTheNextStartReads(t *testing.T) {
	check := func(o Options) (*fakeChecks, error) {
		t.Helper()
		root, bin := setup(t)
		checks := &fakeChecks{}
		o.Root, o.Binary, o.Out, o.Run, o.StartChecks = root, bin, &bytes.Buffer{}, (&recorder{}).run, checks.run
		return checks, Install(o)
	}
	tlsHTTP := settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_TLS_HTTP=bad\n")
	if _, err := check(Options{Settings: tlsHTTP, TLS: "off"}); err != nil {
		t.Errorf("-tls off over MASSIMO_TLS_HTTP: %v", err)
	}
	if _, err := check(Options{Settings: tlsHTTP}); err == nil {
		t.Error("MASSIMO_TLS_HTTP without -tls not refused")
	}

	src := t.TempDir()
	cert, key := filepath.Join(src, "c.pem"), filepath.Join(src, "k.pem")
	os.WriteFile(cert, []byte("CERT"), 0o644)
	os.WriteFile(key, []byte("KEY"), 0o600)
	checks, err := check(Options{Settings: settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=self-signed\n"), TLSCert: cert, TLSKey: key})
	if err != nil || len(checks.seen) == 0 {
		t.Fatalf("-tls-cert and -tls-key: %v, checked %d times", err, len(checks.seen))
	}
	for _, v := range checks.seen {
		if v[tlsVar] != "files" || v[tlsCertVar] != cert || v[tlsKeyVar] != key {
			t.Errorf("the checks read %s=%q %s=%q %s=%q, want files at the sources", tlsVar, v[tlsVar], tlsCertVar, v[tlsCertVar], tlsKeyVar, v[tlsKeyVar])
		}
	}

	for _, k := range installKinds {
		for _, c := range []struct{ what, value, at string }{
			{"an absolute path", "/etc/ssl/bad", "/etc/ssl/bad"},
			{"a relative path", "keys/bad", StateDir + "/keys/bad"},
		} {
			root, o, install := k.opts(t, &recorder{}, &bytes.Buffer{})
			o.Settings, o.StartChecks = settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_TLS_KEY="+c.value+"\n"), (&fakeChecks{}).run
			err := install(o)
			if want := fmt.Sprintf("%s %q: refused", tlsKeyVar, filepath.Join(root, c.at)); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s, %s: %v, want %s", k.what, c.what, err, want)
			}
		}
	}

	root, bin := setup(t)
	hook := "/usr/local/libexec/massimo-dns-hook"
	os.MkdirAll(filepath.Dir(filepath.Join(root, hook)), 0o755)
	os.WriteFile(filepath.Join(root, hook), []byte("#!/bin/sh\n"), 0o755)
	checks = &fakeChecks{}
	err = Install(Options{Root: root, Binary: bin, TLS: "acme", TLSNames: "massimo.example.net", ACMEDNSHook: hook, Out: &bytes.Buffer{}, Run: (&recorder{}).run, StartChecks: checks.run})
	if err != nil || len(checks.seen) == 0 {
		t.Fatalf("-acme-dns-hook under the root: %v, checked %d times", err, len(checks.seen))
	}
	for _, v := range checks.seen {
		if v[acmeDNSHook] != filepath.Join(root, hook) {
			t.Errorf("the checks read %s=%q, want it under the root", acmeDNSHook, v[acmeDNSHook])
		}
	}
}

// The service's working directory is the unit's and the plist's, where
// a relative path of its settings resolves.
func TestServiceWorkingDirectories(t *testing.T) {
	if got := systemdStart().workDir; got != StateDir {
		t.Errorf("massimo.service's WorkingDirectory %q, want %s", got, StateDir)
	}
	if got := launchdStart().workDir; got != StateDir {
		t.Errorf("the LaunchDaemon's WorkingDirectory %q, want %s", got, StateDir)
	}
}

// The -tls options are checked as the start checks the variables they
// set, before the install writes, runs or says anything, by each entry
// point: -tls acme's names, -tls-cert and -tls-key's pair; beside a
// -settings file one refusal names both, with every refusal of either.
func TestInstallRefusesTLSOptionsTheStartRefuses(t *testing.T) {
	settings := settingsFile(t, "MASSIMO_API_TOKEN=t\nMASSIMO_HSTS=bad\n")
	for _, k := range installKinds {
		for _, c := range []struct{ settings, want string }{
			{"", "-tls acme: serve's start refuses:\n" + `  MASSIMO_TLS_NAMES "bad": refused` + "\nnothing was written"},
			{settings, "-settings: " + settings + ", with -tls acme: serve's start refuses:\n" + `  MASSIMO_HSTS "bad": refused` + "\n" + `  MASSIMO_TLS_NAMES "bad": refused` + "\nnothing was written"},
		} {
			var out bytes.Buffer
			rec := &recorder{}
			root, o, install := k.opts(t, rec, &out)
			before := tree(t, root)
			o.Settings, o.TLS, o.TLSNames, o.StartChecks = c.settings, "acme", "bad", (&fakeChecks{}).run
			if err := install(o); err == nil || err.Error() != c.want {
				t.Errorf("%s, -settings %q: %v, want %s", k.what, c.settings, err, c.want)
			}
			if after := tree(t, root); !maps.Equal(after, before) || out.Len() != 0 || len(rec.cmds) != 0 {
				t.Errorf("%s, -settings %q: before the refusal wrote, said %q, ran %q", k.what, c.settings, out.String(), rec.cmds)
			}
		}
	}
}

// -tls-http names itself in its error, not -listen.
func TestInstallTLSHTTPNamesItself(t *testing.T) {
	root, bin := setup(t)
	err := Install(Options{Root: root, Binary: bin, TLS: "acme", TLSNames: "massimo.example.net", TLSHTTP: ":80x", Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	if want := `-tls-http ":80x": bad port`; err == nil || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
}

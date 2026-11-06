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

func darwinOpts(root, bin string, rec *recorder, out *bytes.Buffer) Options {
	return Options{Root: root, Binary: bin, Out: out, Run: rec.run,
		Output: func(name string, args ...string) ([]byte, error) {
			if name == dscl && len(args) == 4 && args[1] == "-list" {
				if args[2] == "/Users" {
					return []byte("root 0\n_oahd 441\nurs 501\n"), nil
				}
				return []byte("wheel 0\ncom.apple.access_remote_ae 400\n"), nil
			}
			return nil, nil
		}}
}

func TestInstallDarwin(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_API_TOKEN=t\nMASSIMO_LOG=/tmp/x.log\n"), 0o600)
	rec := &recorder{}
	var out bytes.Buffer
	o := darwinOpts(root, bin, rec, &out)
	o.Settings = settings
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, BinPath)) != "binary v1" || !strings.HasPrefix(read(t, filepath.Join(root, EnvPath)), "MASSIMO_API_TOKEN=t\n") {
		t.Fatal("binary or settings")
	}
	if !strings.Contains(out.String(), "warning: MASSIMO_LOG") {
		t.Error("no warning about MASSIMO_LOG")
	}
	if strings.Contains(read(t, filepath.Join(root, PlistPath(Label))), "<string>-log</string>") {
		t.Error("-log in the service plist")
	}
	serve := read(t, filepath.Join(root, PlistPath(Label)))
	for _, want := range []string{"<string>-listen</string>\n\t\t<string>:8099</string>", "<string>-update-request</string>", "<string>_massimo</string>", "<string>-data</string>\n\t\t<string>/var/lib/massimo</string>"} {
		if !strings.Contains(serve, want) {
			t.Errorf("service plist lacks %q", want)
		}
	}
	helper := read(t, filepath.Join(root, PlistPath(HelperLabel)))
	if !strings.Contains(helper, "<string>http://127.0.0.1:8099/healthz</string>") || strings.Contains(helper, healthMark) {
		t.Errorf("helper plist: %s", helper)
	}
	for _, c := range []string{
		dscl + " . -create /Groups/_massimo PrimaryGroupID 401", // 400 is a group's
		dscl + " . -create /Users/_massimo UniqueID 401",
		dscl + " . -create /Users/_massimo IsHidden 1",
		"chown _massimo:_massimo " + filepath.Join(root, EnvPath),
		"chown _massimo:_massimo " + filepath.Join(root, StateDir),
		"chown root:wheel " + filepath.Join(root, HelperStateDir),
		launchctl + " bootstrap system " + PlistPath(HelperLabel),
		launchctl + " bootstrap system " + PlistPath(Label),
	} {
		if !rec.has(c) {
			t.Errorf("no %q in %q", c, rec.cmds)
		}
	}

	// Again with another address, then once more without: it is kept, and
	// the helper asks the server there.
	o.Listen = ":80"
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	o.Listen, out = "", bytes.Buffer{}
	o.Out = &out
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "listen address :80 kept") || !strings.Contains(out.String(), PlistPath(Label)+" is current") {
		t.Errorf("reinstall: %s", out.String())
	}
	if !strings.Contains(read(t, filepath.Join(root, PlistPath(HelperLabel))), "http://127.0.0.1:80/healthz") {
		t.Error("helper health URL not updated")
	}
}

func TestInstallDarwinNoHelperAndTemplate(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	o := darwinOpts(root, bin, rec, &out)
	o.NoUpdateHelper = true
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, filepath.Join(root, PlistPath(Label))), "<string>-update-request</string>") {
		t.Error("-update-request without the helper")
	}
	if _, err := os.Stat(filepath.Join(root, PlistPath(HelperLabel))); err == nil {
		t.Error("helper plist written")
	}
	if env := read(t, filepath.Join(root, EnvPath)); !strings.Contains(env, "by the user _massimo.") || strings.Contains(env, "\nMASSIMO_LOG=") {
		t.Errorf("env template: %s", env)
	}
	if _, err := os.Stat(filepath.Join(root, DarwinLog)); err != nil || !rec.has("chown _massimo:_massimo "+filepath.Join(root, DarwinLog)) {
		t.Errorf("log file: %v", err)
	}
	// Only the template: not started.
	if rec.has(launchctl+" bootstrap system "+PlistPath(Label)) || !strings.Contains(out.String(), "not started") {
		t.Errorf("started without settings: %s", out.String())
	}
}

func TestRemoveDarwin(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	o := darwinOpts(root, bin, rec, &out)
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, BinPath+".prev"), nil, 0o755)
	if err := RemoveDarwin(o, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{BinPath, BinPath + ".prev", PlistPath(Label), PlistPath(HelperLabel), EnvDir, StateDir, LogDir, HelperStateDir} {
		if _, err := os.Lstat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s left", p)
		}
	}
	if !rec.has(dscl+" . -delete /Users/_massimo") || !rec.has(dscl+" . -delete /Groups/_massimo") {
		t.Errorf("user not removed: %q", rec.cmds)
	}
}

func TestLaunchdPlists(t *testing.T) {
	serve, helper, err := launchdPlists("[::]:8100", false)
	if err != nil {
		t.Fatal(err)
	}
	if ListenFromPlist(serve) != "[::]:8100" || !bytes.Contains(helper, []byte("http://[::1]:8100/healthz")) {
		t.Fatalf("%s\n%s", serve, helper)
	}
	serve, _, _ = launchdPlists("a&b:81", false)
	if !bytes.Contains(serve, []byte("a&amp;b:81")) || ListenFromPlist(serve) != "a&b:81" {
		t.Error("not escaped")
	}
	for _, bad := range []string{"8099", ":0", ":x"} {
		if _, _, err := launchdPlists(bad, false); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if ListenFromPlist([]byte("<plist/>")) != "" {
		t.Error("listen from nothing")
	}
}

func TestCheckDarwinListen(t *testing.T) {
	for addr, warn := range map[string]bool{":80": false, "0.0.0.0:80": false, "192.168.1.5:80": true, "192.168.1.5:8099": false} {
		var out bytes.Buffer
		in := newInstaller(Options{Out: &out})
		in.checkDarwinListen(addr)
		if (out.Len() > 0) != warn {
			t.Errorf("%s: %q", addr, out.String())
		}
	}
}

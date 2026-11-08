// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/packaging"
)

// writeRequest puts a reboot request below root, as one left from
// before.
func writeRequest(t *testing.T, root string) string {
	t.Helper()
	p := filepath.Join(root, RebootRequest)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte("massimo: reboot requested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func gone(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Lstat(p)
	return os.IsNotExist(err)
}

// The reboot helper comes only with -reboot-helper, stays through later
// installs, goes with -no-reboot-helper and with -remove; enabling it
// takes a request left from before away first.
func TestInstallRebootHelper(t *testing.T) {
	root, bin := setup(t)
	units := []string{UnitDir + "/massimo-reboot.path", UnitDir + "/massimo-reboot.service"}
	rec := &recorder{}
	if err := Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !gone(t, filepath.Join(root, units[0])) || rec.has("systemctl enable --now massimo-reboot.path") {
		t.Fatalf("the helper without -reboot-helper: %q", rec.cmds)
	}

	req := writeRequest(t, root)
	rec = &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, RebootHelper: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		embedded, _ := fs.ReadFile(packaging.Files, "systemd/"+filepath.Base(u))
		if read(t, filepath.Join(root, u)) != string(embedded) {
			t.Errorf("%s is not the embedded unit", u)
		}
	}
	if !rec.has("systemctl enable --now massimo-reboot.path") || !gone(t, req) || !strings.Contains(out.String(), "MASSIMO_REBOOT=on is in "+EnvPath) {
		t.Fatalf("with -reboot-helper: %q, request gone %v\n%s", rec.cmds, gone(t, req), out.String())
	}

	rec, out = &recorder{}, bytes.Buffer{}
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if gone(t, filepath.Join(root, units[0])) || !rec.has("systemctl enable --now massimo-reboot.path") || strings.Contains(out.String(), "MASSIMO_REBOOT") {
		t.Fatalf("installed again: %q\n%s", rec.cmds, out.String())
	}

	rec = &recorder{}
	if err := Install(Options{Root: root, Binary: bin, NoRebootHelper: true, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !gone(t, filepath.Join(root, units[0])) || !gone(t, filepath.Join(root, units[1])) || !rec.has("systemctl disable --now massimo-reboot.path") ||
		rec.has("systemctl enable --now massimo-reboot.path") {
		t.Fatalf("with -no-reboot-helper: %q", rec.cmds)
	}

	Install(Options{Root: root, Binary: bin, RebootHelper: true, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	rec = &recorder{}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}, false); err != nil {
		t.Fatal(err)
	}
	if !gone(t, filepath.Join(root, units[0])) || !gone(t, filepath.Join(root, units[1])) || !rec.has("systemctl disable --now massimo-update.path massimo-reboot.path massimo.service") {
		t.Fatalf("remove: %q", rec.cmds)
	}
}

// On a box with the Debian package the units are the package's: install
// enables or disables the path unit and writes none.
func TestInstallPackagedRebootHelper(t *testing.T) {
	root := packagedRoot(t)
	req := writeRequest(t, root)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, RebootHelper: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !rec.has("systemctl enable --now massimo-reboot.path") || !gone(t, req) || !gone(t, filepath.Join(root, UnitDir, "massimo-reboot.path")) {
		t.Fatalf("-reboot-helper: %q\n%s", rec.cmds, out.String())
	}
	rec = &recorder{}
	if err := Install(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if rec.has("systemctl enable --now massimo-reboot.path") || rec.has("systemctl disable --now massimo-reboot.path") {
		t.Fatalf("without a flag: %q", rec.cmds)
	}
	rec = &recorder{}
	if err := Install(Options{Root: root, NoRebootHelper: true, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !rec.has("systemctl disable --now massimo-reboot.path") {
		t.Fatalf("-no-reboot-helper: %q", rec.cmds)
	}
}

// On macOS the reboot helper is a LaunchDaemon of its own, with the same
// rules as on Linux.
func TestInstallDarwinRebootHelper(t *testing.T) {
	root, bin := setup(t)
	plist := filepath.Join(root, PlistPath(RebootLabel))
	rec := &recorder{}
	var out bytes.Buffer
	o := darwinOpts(root, bin, rec, &out)
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	if !gone(t, plist) {
		t.Fatal("the helper without -reboot-helper")
	}
	req := writeRequest(t, root)
	o.RebootHelper = true
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	embedded, _ := readPlist(RebootLabel)
	if read(t, plist) != string(embedded) || !rec.has(launchctl+" bootstrap system "+PlistPath(RebootLabel)) || !gone(t, req) {
		t.Fatalf("-reboot-helper: %q", rec.cmds)
	}
	o.RebootHelper = false
	if err := InstallDarwin(o); err != nil || gone(t, plist) {
		t.Fatalf("installed again: %v", err)
	}
	o.NoRebootHelper = true
	if err := InstallDarwin(o); err != nil || !gone(t, plist) {
		t.Fatalf("-no-reboot-helper: %v", err)
	}
	o.NoRebootHelper, o.RebootHelper = false, true
	InstallDarwin(o)
	if err := RemoveDarwin(o, false); err != nil || !gone(t, plist) {
		t.Fatalf("remove: %v", err)
	}
}

// The helpers watch the file serve writes, remove it before anything
// else, and read nothing the server's user can write.
func TestRebootHelpersWatchTheRequest(t *testing.T) {
	path, _ := fs.ReadFile(packaging.Files, "systemd/massimo-reboot.path")
	unit, _ := fs.ReadFile(packaging.Files, "systemd/massimo-reboot.service")
	plist, _ := readPlist(RebootLabel)
	if !strings.Contains(string(path), "\nPathExists="+RebootRequest+"\n") || !strings.Contains(string(path), "\nWantedBy=paths.target\n") ||
		filepath.Base(RebootPathWant) != "massimo-reboot.path" || filepath.Base(filepath.Dir(RebootPathWant)) != "paths.target.wants" {
		t.Errorf("the path unit:\n%s", path)
	}
	pre := strings.Index(string(unit), "\nExecStartPre=/bin/rm -f "+RebootRequest+"\n")
	start := strings.Index(string(unit), "\nExecStart=/bin/systemctl reboot\n")
	if pre < 0 || start < pre || strings.Contains(string(unit), "EnvironmentFile") {
		t.Errorf("the service:\n%s", unit)
	}
	script := "r=" + RebootRequest + "; [ -e \"$r\" ] || [ -L \"$r\" ] || exit 0; /bin/rm -f \"$r\" || exit 1; exec /sbin/shutdown -r now"
	if !strings.Contains(string(plist), "<string>"+RebootRequest+"</string>") || !strings.Contains(string(plist), script) {
		t.Errorf("the LaunchDaemon:\n%s", plist)
	}
}
